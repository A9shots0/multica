package previewpublication

import (
	"os"
	"strings"
	"testing"
)

func TestPublicationQueriesBindAuthorityAndFailClosed(t *testing.T) {
	query, err := os.ReadFile("../../pkg/db/queries/preview_publication.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(query)
	for _, required := range []string{
		"task.context->'preview'->>'candidate_id'",
		"task.context->'preview'->>'source_revision'",
		"task.runtime_id",
		"runtime.daemon_id = @daemon_id",
		"COALESCE(NULLIF(task.durable_work_dir, ''), NULLIF(task.work_dir, '')) IS NOT NULL",
		"NULLIF(task.branch_name, '') IS NOT NULL",
		"task.status IN ('dispatched', 'running', 'completed')",
		"runtime.status = 'online'",
		"candidate.superseded_at IS NULL",
		"rejected.superseded_at IS NULL",
		"ON CONFLICT (workspace_id, issue_id, candidate_id, candidate_revision) DO NOTHING",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("preview publication query missing authority invariant %q", required)
		}
	}
}
