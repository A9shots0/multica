package migrations

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestChatReplyContextMigrationUsesApplicationRelationships(t *testing.T) {
	up := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "619_chat_reply_context.up.sql"))
	sql := stripSQLLineComments(up)
	for _, column := range []string{"reply_to_message_id", "reply_context"} {
		if !strings.Contains(sql, column) {
			t.Fatalf("chat reply migration missing %s", column)
		}
	}
	forbidden := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|CREATE\s+(UNIQUE\s+)?INDEX)\b`)
	if match := forbidden.FindString(sql); match != "" {
		t.Fatalf("chat reply relationship must remain application-enforced; found %q", match)
	}
}

func TestChatReplyTargetIndexIsConcurrentAndIsolated(t *testing.T) {
	up := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "620_chat_reply_target_index.up.sql"))))
	if strings.Count(up, ";") != 1 || !strings.Contains(strings.ToUpper(up), "CREATE INDEX CONCURRENTLY") {
		t.Fatal("chat reply target index must be one concurrent index statement")
	}
	down := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "620_chat_reply_target_index.down.sql"))))
	if strings.Count(down, ";") != 1 || !strings.Contains(strings.ToUpper(down), "DROP INDEX CONCURRENTLY") {
		t.Fatal("chat reply target down migration must be one concurrent drop statement")
	}
}
