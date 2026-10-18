package migrations

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGuardedPermissionReceiptTablesAreDurableAndRelationshipFree(t *testing.T) {
	up := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "544_guarded_permission_receipts.up.sql"))
	plain := stripSQLLineComments(up)
	forbidden := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|PRIMARY\s+KEY|CREATE\s+(UNIQUE\s+)?INDEX)\b`)
	if match := forbidden.FindString(plain); match != "" {
		t.Fatalf("guarded receipt tables must use application relationships and separate concurrent indexes; found %q", match)
	}

	for _, table := range []string{
		"qualified_runtime_binding_receipt",
		"qualified_runtime_binding_revocation",
		"guarded_task_capability_grant",
		"guarded_task_capability_revocation",
	} {
		if !strings.Contains(up, "CREATE TABLE "+table+" ") {
			t.Errorf("migration 544 is missing table %s", table)
		}
		if !strings.Contains(up, "BEFORE UPDATE OR DELETE ON "+table) {
			t.Errorf("migration 544 does not make %s append-only", table)
		}
	}

	for _, required := range []string{
		"receipt_sha256 ~ '^[0-9a-f]{64}$'",
		"payload_sha256 ~ '^[0-9a-f]{64}$'",
		"qualification_status = 'qualified_bounded'",
		"source = 'controller_record'",
		"immutable = TRUE",
		"expires_at > issued_at",
		"jsonb_array_length(evidence_receipt_ids) > 0",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("migration 544 is missing guard %q", required)
		}
	}
}

func TestGuardedPermissionReceiptIndexesAreConcurrentSingleStatements(t *testing.T) {
	for _, version := range []string{
		"545_qualified_runtime_binding_receipt_pkey_idx",
		"547_qualified_runtime_binding_revocation_pkey_idx",
		"549_guarded_task_capability_grant_pkey_idx",
		"551_guarded_task_capability_grant_task_idx",
		"552_guarded_task_capability_revocation_pkey_idx",
		"555_guarded_execution_launch_consumption_pkey_idx",
		"557_guarded_execution_launch_consumption_task_idx",
	} {
		up := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), version+".up.sql"))))
		if !strings.Contains(strings.ToUpper(up), "CREATE UNIQUE INDEX CONCURRENTLY") {
			t.Fatalf("%s must create one unique index concurrently", version)
		}
		if strings.Count(up, ";") != 1 || !strings.HasSuffix(up, ";") {
			t.Fatalf("%s must contain exactly one statement", version)
		}

		down := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), version+".down.sql"))))
		if !strings.Contains(strings.ToUpper(down), "DROP INDEX CONCURRENTLY") {
			t.Fatalf("%s down migration must drop its index concurrently", version)
		}
		if strings.Count(down, ";") != 1 || !strings.HasSuffix(down, ";") {
			t.Fatalf("%s down migration must contain exactly one statement", version)
		}
	}
}

func TestGuardedPermissionReceiptPrimaryKeysAttachPreparedIndexes(t *testing.T) {
	for _, tc := range []struct {
		version string
		table   string
		index   string
	}{
		{"546_qualified_runtime_binding_receipt_primary_key", "qualified_runtime_binding_receipt", "qualified_runtime_binding_receipt_pkey_idx"},
		{"548_qualified_runtime_binding_revocation_primary_key", "qualified_runtime_binding_revocation", "qualified_runtime_binding_revocation_pkey_idx"},
		{"550_guarded_task_capability_grant_primary_key", "guarded_task_capability_grant", "guarded_task_capability_grant_pkey_idx"},
		{"553_guarded_task_capability_revocation_primary_key", "guarded_task_capability_revocation", "guarded_task_capability_revocation_pkey_idx"},
		{"556_guarded_execution_launch_consumption_primary_key", "guarded_execution_launch_consumption", "guarded_execution_launch_consumption_pkey_idx"},
	} {
		up := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), tc.version+".up.sql"))))
		want := "ALTER TABLE " + tc.table + " ADD CONSTRAINT " + tc.table + "_pkey PRIMARY KEY USING INDEX " + tc.index + ";"
		if up != want {
			t.Fatalf("%s = %q, want exact prepared-index attachment %q", tc.version, up, want)
		}
	}
}

func TestGuardedExecutionConsumptionIsAppendOnlyAndRelationshipFree(t *testing.T) {
	up := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "554_guarded_execution_launch_consumption.up.sql"))
	plain := stripSQLLineComments(up)
	forbidden := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|PRIMARY\s+KEY|CREATE\s+(UNIQUE\s+)?INDEX)\b`)
	if match := forbidden.FindString(plain); match != "" {
		t.Fatalf("guarded launch consumption must use application relationships and separate concurrent indexes; found %q", match)
	}
	for _, required := range []string{
		"CREATE TABLE guarded_execution_launch_consumption",
		"BEFORE UPDATE OR DELETE ON guarded_execution_launch_consumption",
		"package_sha256 ~ '^[0-9a-f]{64}$'",
		"consumed_by_daemon_id TEXT NOT NULL",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("migration 554 is missing guard %q", required)
		}
	}
}
