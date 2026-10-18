package agentqualification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const permissionReceiptTestDatabaseEnv = "PERMISSION_RECEIPT_TEST_DATABASE_URL"

func TestPermissionReceiptStorePostgresContracts(t *testing.T) {
	databaseURL := os.Getenv(permissionReceiptTestDatabaseEnv)
	if databaseURL == "" {
		t.Skipf("integration test requires an isolated database at %s", permissionReceiptTestDatabaseEnv)
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to permission receipt test database: %v", err)
	}
	const schema = "permission_receipt_contract_test"
	identifier := pgx.Identifier{schema}.Sanitize()
	_, _ = conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+identifier+" CASCADE")
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "SET search_path TO public")
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+identifier+" CASCADE")
		_ = conn.Close(context.Background())
	})

	for version := 544; version <= 557; version++ {
		matches, err := filepath.Glob(filepath.Join("..", "..", "migrations", migrationVersionPattern(version)))
		if err != nil || len(matches) != 1 {
			t.Fatalf("resolve migration %d: matches=%v err=%v", version, matches, err)
		}
		body, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply migration %d: %v", version, err)
		}
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE agent_runtime (id UUID NOT NULL, workspace_id UUID NOT NULL, daemon_id TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO agent_runtime (id, workspace_id, daemon_id) VALUES ($1, $2, $3)`, permissionRuntimeID, permissionWorkspaceID, permissionDaemonID); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 13, 22, 0, 0, 123456000, time.UTC)
	store := NewPermissionReceiptStore(db.New(conn))
	receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(ctx, permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StoreQualifiedRuntimeBinding(ctx, permissionControllerID, receipt, now); err != nil {
		t.Fatalf("Postgres exact runtime replay: %v", err)
	}
	if _, err := store.StoreTaskCapabilityGrant(ctx, permissionControllerID, permissionTaskID, scope, targets, now); err != nil {
		t.Fatal(err)
	}
	pkg, err := store.LoadGuardedExecutionPackage(ctx, permissionLoadIdentity(), envelope, now)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := GuardedExecutionPackageSHA256V1(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(ctx, permissionLoadIdentity(), envelope, strings.Repeat("a", 64), permissionDaemonID, now); !errors.Is(err, ErrGuardedAuthorityUnavailable) {
		t.Fatalf("Postgres wrong-digest consumption error = %v", err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(ctx, permissionLoadIdentity(), envelope, digest, "wrong-daemon", now); !errors.Is(err, ErrGuardedAuthorityUnavailable) {
		t.Fatalf("Postgres wrong-daemon consumption error = %v", err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(ctx, permissionLoadIdentity(), envelope, digest, permissionDaemonID, now); err != nil {
		t.Fatalf("Postgres consume exact authority: %v", err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(ctx, permissionLoadIdentity(), envelope, digest, permissionDaemonID, now); !errors.Is(err, ErrGuardedAuthorityUnavailable) {
		t.Fatalf("Postgres replay consumption error = %v", err)
	}
	conflict := scope
	conflict.Receipt.ID = "capability-conflict"
	if _, err := store.StoreTaskCapabilityGrant(ctx, permissionControllerID, permissionTaskID, conflict, targets, now); err == nil {
		t.Fatal("Postgres task uniqueness accepted a second capability grant")
	}

	for _, statement := range []string{
		`UPDATE qualified_runtime_binding_receipt SET model = 'mutated' WHERE id = 'runtime-binding-1'`,
		`DELETE FROM qualified_runtime_binding_receipt WHERE id = 'runtime-binding-1'`,
		`UPDATE guarded_task_capability_grant SET action = 'mutated' WHERE id = 'capability-1'`,
		`DELETE FROM guarded_task_capability_grant WHERE id = 'capability-1'`,
		`UPDATE guarded_execution_launch_consumption SET consumed_by_daemon_id = 'mutated' WHERE capability_grant_id = 'capability-1'`,
		`DELETE FROM guarded_execution_launch_consumption WHERE capability_grant_id = 'capability-1'`,
	} {
		if _, err := conn.Exec(ctx, statement); err == nil {
			t.Fatalf("append-only storage accepted %q", statement)
		}
	}
	if err := store.RevokeQualifiedRuntimeBinding(ctx, permissionControllerID, "00000000-0000-0000-0000-000000000099", receipt.ID, "wrong workspace", now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("wrong-workspace revocation error = %v", err)
	}
	if err := store.RevokeTaskCapabilityGrant(ctx, permissionControllerID, permissionWorkspaceID, scope.Receipt.ID, "scope withdrawn", now); err != nil {
		t.Fatalf("revoke Postgres capability grant: %v", err)
	}
	for _, statement := range []string{
		`UPDATE guarded_task_capability_revocation SET reason = 'mutated' WHERE grant_id = 'capability-1'`,
		`DELETE FROM guarded_task_capability_revocation WHERE grant_id = 'capability-1'`,
	} {
		if _, err := conn.Exec(ctx, statement); err == nil {
			t.Fatalf("append-only capability revocation accepted %q", statement)
		}
	}

	firstConn := connectPermissionReceiptTestDB(t, databaseURL, schema)
	defer firstConn.Close(ctx)
	secondConn := connectPermissionReceiptTestDB(t, databaseURL, schema)
	defer secondConn.Close(ctx)
	stores := []*PermissionReceiptStore{NewPermissionReceiptStore(db.New(firstConn)), NewPermissionReceiptStore(db.New(secondConn))}
	errs := make(chan error, len(stores))
	var wg sync.WaitGroup
	for _, concurrentStore := range stores {
		wg.Add(1)
		go func(s *PermissionReceiptStore) {
			defer wg.Done()
			errs <- s.RevokeQualifiedRuntimeBinding(ctx, permissionControllerID, permissionWorkspaceID, receipt.ID, "qualification withdrawn", now)
		}(concurrentStore)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Postgres revocation: %v", err)
		}
	}
	if _, err := store.LoadGuardedExecutionPackage(ctx, permissionLoadIdentity(), envelope, now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("revoked Postgres load error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE qualified_runtime_binding_revocation SET reason = 'mutated' WHERE receipt_id = 'runtime-binding-1'`,
		`DELETE FROM qualified_runtime_binding_revocation WHERE receipt_id = 'runtime-binding-1'`,
	} {
		if _, err := conn.Exec(ctx, statement); err == nil {
			t.Fatalf("append-only revocation accepted %q", statement)
		}
	}
}

func migrationVersionPattern(version int) string {
	return fmt.Sprintf("%d_*.up.sql", version)
}

func connectPermissionReceiptTestDB(t *testing.T, databaseURL, schema string) *pgx.Conn {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, schema); err != nil {
		_ = conn.Close(ctx)
		t.Fatal(err)
	}
	return conn
}
