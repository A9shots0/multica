package migrations

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const issueCreationPolicyMigrationTestSchema = "issue_creation_policy_migration_test"

func TestIssueCreationPolicyMigrationRejectsAgentRowsAtDatabaseBoundary(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("integration test requires Postgres at DATABASE_URL")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	dropTestSchema := func(ctx context.Context) error {
		_, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+issueCreationPolicyMigrationTestSchema+" CASCADE")
		return err
	}
	t.Cleanup(func() {
		defer pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := dropTestSchema(cleanupCtx); err != nil {
			t.Errorf("drop isolated migration schema: %v", err)
		}
	})

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire Postgres connection: %v", err)
	}
	defer conn.Release()

	if err := dropTestSchema(ctx); err != nil {
		t.Fatalf("reset isolated migration schema: %v", err)
	}
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+issueCreationPolicyMigrationTestSchema); err != nil {
		t.Fatalf("create isolated migration schema: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, issueCreationPolicyMigrationTestSchema); err != nil {
		t.Fatalf("set isolated migration search path: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		CREATE TABLE workspace (
			id UUID PRIMARY KEY,
			settings JSONB NOT NULL DEFAULT '{}'::jsonb
		);
		CREATE TABLE issue (
			id UUID PRIMARY KEY,
			workspace_id UUID NOT NULL,
			creator_type TEXT NOT NULL
		);
		INSERT INTO workspace (id, settings) VALUES
			('00000000-0000-4000-8000-000000000001', '{"require_member_issue_creation":true}'),
			('00000000-0000-4000-8000-000000000002', '{}');
	`); err != nil {
		t.Fatalf("create pre-policy tables: %v", err)
	}

	applyMigrationFile(t, ctx, conn.Conn(), "576_require_member_issue_creation.up.sql")

	_, err = conn.Exec(ctx, `
		INSERT INTO issue (id, workspace_id, creator_type)
		VALUES ('00000000-0000-4000-8000-000000000010', '00000000-0000-4000-8000-000000000001', 'agent')
	`)
	assertIssueCreationPolicyViolation(t, err)

	if _, err := conn.Exec(ctx, `
		INSERT INTO issue (id, workspace_id, creator_type)
		VALUES ('00000000-0000-4000-8000-000000000011', '00000000-0000-4000-8000-000000000001', 'member')
	`); err != nil {
		t.Fatalf("member issue rejected in protected workspace: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO issue (id, workspace_id, creator_type)
		VALUES ('00000000-0000-4000-8000-000000000012', '00000000-0000-4000-8000-000000000002', 'agent')
	`); err != nil {
		t.Fatalf("agent issue rejected in unprotected workspace: %v", err)
	}

	// Enabling the policy owns the workspace row lock before a concurrent
	// insert can evaluate it. Once the toggle commits, the waiting insert must
	// observe the enabled value and fail rather than slipping through.
	toggleTx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin policy toggle: %v", err)
	}
	if _, err := toggleTx.Exec(ctx, `UPDATE workspace SET settings = '{"require_member_issue_creation":true}' WHERE id = '00000000-0000-4000-8000-000000000002'`); err != nil {
		t.Fatalf("stage policy toggle: %v", err)
	}
	insertConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire concurrent insert connection: %v", err)
	}
	defer insertConn.Release()
	if _, err := insertConn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, issueCreationPolicyMigrationTestSchema); err != nil {
		t.Fatalf("set concurrent insert search path: %v", err)
	}
	insertResult := make(chan error, 1)
	go func() {
		_, insertErr := insertConn.Exec(context.Background(), `
			INSERT INTO issue (id, workspace_id, creator_type)
			VALUES ('00000000-0000-4000-8000-000000000014', '00000000-0000-4000-8000-000000000002', 'agent')
		`)
		insertResult <- insertErr
	}()
	select {
	case early := <-insertResult:
		t.Fatalf("concurrent insert returned before policy transaction committed: %v", early)
	case <-time.After(100 * time.Millisecond):
	}
	if err := toggleTx.Commit(ctx); err != nil {
		t.Fatalf("commit policy toggle: %v", err)
	}
	select {
	case insertErr := <-insertResult:
		assertIssueCreationPolicyViolation(t, insertErr)
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent insert did not finish after policy toggle committed")
	}

	applyMigrationFile(t, ctx, conn.Conn(), "576_require_member_issue_creation.down.sql")
	if _, err := conn.Exec(ctx, `
		INSERT INTO issue (id, workspace_id, creator_type)
		VALUES ('00000000-0000-4000-8000-000000000013', '00000000-0000-4000-8000-000000000001', 'agent')
	`); err != nil {
		t.Fatalf("agent issue rejected after rollback: %v", err)
	}
}

func assertIssueCreationPolicyViolation(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" || pgErr.ConstraintName != "issue_agent_creation_policy" {
		t.Fatalf("agent insert error = %v, want SQLSTATE 42501 constraint issue_agent_creation_policy", err)
	}
}
