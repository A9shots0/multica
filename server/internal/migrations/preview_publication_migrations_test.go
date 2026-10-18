package migrations

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPreviewPublicationMigrationsAreFKFreeAndConcurrent(t *testing.T) {
	table := stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "579_preview_publication.up.sql")))
	if match := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|PRIMARY\s+KEY|CREATE\s+(UNIQUE\s+)?INDEX)\b`).FindString(table); match != "" {
		t.Fatalf("publication table must be relationship- and index-free; found %q", match)
	}
	index := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "580_preview_publication_identity_idx.up.sql"))))
	if strings.Count(index, ";") != 1 || !strings.Contains(index, "CREATE UNIQUE INDEX CONCURRENTLY") {
		t.Fatal("publication identity index must be one concurrent statement")
	}
}
