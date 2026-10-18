package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestExternalIncidentMigrationsRollbackFailsClosedWithUsedData(t *testing.T) {
	adminPool := openTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema := fmt.Sprintf("migrate_external_incident_%d_%d", time.Now().UnixNano(), rand.Uint32())
	schemaIdent := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schemaIdent); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schemaIdent+" CASCADE") })
	pool := openTestPoolWithSearchPath(t, schema)
	if _, err := pool.Exec(ctx, `
		CREATE TABLE issue (
			id UUID PRIMARY KEY,
			workspace_id UUID NOT NULL,
			origin_type TEXT,
			origin_id UUID
		);
		ALTER TABLE issue ADD CONSTRAINT issue_origin_type_check
			CHECK (origin_type IS NULL OR origin_type IN ('autopilot', 'quick_create', 'lark_chat', 'slack_chat', 'agent_create', 'dingtalk_chat', 'wecom_chat', 'telegram_chat'));
	`); err != nil {
		t.Fatalf("create issue fixture: %v", err)
	}
	versions := append([]string(nil), externalIncidentMigrationVersions...)
	options := runOptions{
		Direction: "up", Files: realMigrationFiles(t, versions, "up"),
		SchemaMigrationsTable: schema + ".schema_migrations",
		AdvisoryLockKey:       int64(rand.Uint64()&0x7fffffffffffffff) | 1,
		Hooks:                 hooksForDirection("up"),
	}
	if err := runMigrations(ctx, pool, options); err != nil {
		t.Fatalf("apply external incident migrations: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO external_incident_source_binding (
			workspace_id, source_key, creator_agent_id, project_id,
			credential_hash, credential_hint, enabled
		) VALUES (
			'00000000-0000-4000-8000-000000000001', 'intelligence',
			'00000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000003',
			repeat('a', 64), 'mei_...fixture', true
		)
	`); err != nil {
		t.Fatalf("insert used external incident binding: %v", err)
	}
	reversedVersions := append([]string(nil), versions...)
	for left, right := 0, len(reversedVersions)-1; left < right; left, right = left+1, right-1 {
		reversedVersions[left], reversedVersions[right] = reversedVersions[right], reversedVersions[left]
	}
	options.Direction = "down"
	options.Files = realMigrationFiles(t, reversedVersions, "down")
	options.Hooks = hooksForDirection("down")
	err := runMigrations(ctx, pool, options)
	if err == nil || !strings.Contains(err.Error(), "cannot roll back external incident or company signal ingestion") {
		t.Fatalf("used-data rollback error = %v, want fail-closed refusal", err)
	}
	var bindingCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_incident_source_binding`).Scan(&bindingCount); err != nil {
		t.Fatalf("count preserved binding: %v", err)
	}
	if bindingCount != 1 {
		t.Fatalf("bindings after refused rollback = %d, want 1", bindingCount)
	}
	for _, relation := range []string{"external_incident_predecessor_key", "external_incident_source_sequence_key"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+relation).Scan(&exists); err != nil || !exists {
			t.Fatalf("index %s was dropped before rollback refusal (exists=%v, err=%v)", relation, exists, err)
		}
	}
	if _, err := pool.Exec(ctx, `DELETE FROM external_incident_source_binding`); err != nil {
		t.Fatalf("clear used binding: %v", err)
	}
	if err := runMigrations(ctx, pool, options); err != nil {
		t.Fatalf("empty external incident rollback: %v", err)
	}
	var tableExists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".external_incident_occurrence").Scan(&tableExists); err != nil {
		t.Fatalf("inspect rolled back table: %v", err)
	}
	if tableExists {
		t.Fatal("external incident tables remain after empty-data rollback")
	}
}
