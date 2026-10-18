package migrations

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestOwnerNotificationTablesStayRelationshipAndIndexFree(t *testing.T) {
	body := stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "523_owner_notification_delivery.up.sql")))
	forbidden := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|PRIMARY\s+KEY|CREATE\s+(UNIQUE\s+)?INDEX|UNIQUE\s*\()`)
	if match := forbidden.FindString(body); match != "" {
		t.Fatalf("notification table migration must use application relationships and separate concurrent indexes; found %q", match)
	}
}

func TestOwnerNotificationIndexesAreSingleStatementConcurrentMigrations(t *testing.T) {
	for _, version := range []string{
		"530_owner_notification_event_unique_idx",
		"531_owner_notification_due_idx",
		"532_owner_notification_issue_idx",
		"533_owner_notification_attempt_unique_idx",
		"534_owner_notification_attempt_notification_idx",
		"538_owner_notification_binding_pkey_idx",
		"540_owner_notification_pkey_idx",
		"542_owner_notification_attempt_pkey_idx",
	} {
		body := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), version+".up.sql"))))
		if !strings.Contains(strings.ToUpper(body), " INDEX CONCURRENTLY ") {
			t.Fatalf("%s must create its index concurrently", version)
		}
		if strings.Count(body, ";") != 1 || !strings.HasSuffix(body, ";") {
			t.Fatalf("%s must contain exactly one statement", version)
		}
	}
}
