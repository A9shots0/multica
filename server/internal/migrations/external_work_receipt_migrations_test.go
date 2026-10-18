package migrations

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestExternalWorkReceiptTableIsGuardedAndRelationshipFree(t *testing.T) {
	up := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "621_external_work_receipt.up.sql"))
	stripped := stripSQLLineComments(up)
	forbidden := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|PRIMARY\s+KEY|CREATE\s+(UNIQUE\s+)?INDEX)\b`)
	if match := forbidden.FindString(stripped); match != "" {
		t.Fatalf("external receipt records must remain FK-free with separate concurrent indexes; found %q", match)
	}
	for _, required := range []string{
		"idempotency_key", "request_hash", "contract_version", "receipt_revision", "supersedes_receipt_revision",
		"credential_kind", "authority_claim_state", "decision_proposal_state", "raw_receipt",
		"evidence_id", "execution_id", "current_picture_revision",
		"CHECK (receipt_revision >= 1)", "recorded_non_actionable",
		"'personal_access_token', 'plugin_invocation'",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("621 up migration is missing %q", required)
		}
	}
	down := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "621_external_work_receipt.down.sql"))
	if !strings.Contains(down, "refusing destructive rollback") || !strings.Contains(down, "IF EXISTS (SELECT 1 FROM issue_external_work_receipt)") {
		t.Fatal("621 down migration must fail closed while durable external receipts exist")
	}
}

func TestExternalWorkReceiptIndexesAreConcurrentSingleStatements(t *testing.T) {
	for _, version := range []string{"622_external_work_receipt_key_index", "623_external_work_receipt_idempotency_index", "624_external_work_receipt_revision_index"} {
		for _, direction := range []string{"up", "down"} {
			name := version + "." + direction + ".sql"
			sql := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), name))))
			if strings.Count(sql, ";") != 1 || !strings.HasSuffix(sql, ";") {
				t.Fatalf("%s must contain exactly one statement", name)
			}
			if !strings.Contains(strings.ToUpper(sql), "INDEX CONCURRENTLY") {
				t.Fatalf("%s must manage its index concurrently", name)
			}
		}
	}
}
