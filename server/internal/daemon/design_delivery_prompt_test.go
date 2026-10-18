package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/designgate"
)

func TestBuildPromptIncludesExactProductDesignGate(t *testing.T) {
	binding := &designgate.Binding{
		SchemaVersion:      1,
		ProductID:          "crm",
		RepositoryIdentity: "a9shots0/smcrm",
		PackSchemaVersion:  1,
		SourceRevision:     "1111111111111111111111111111111111111111",
		PackSHA256:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ApprovalState:      "approved",
		Pack:               json.RawMessage(`{"schema_version":1,"product":{"id":"crm"}}`),
	}

	prompt := BuildPrompt(Task{IssueID: "issue-1", DesignDelivery: binding}, "claude")
	for _, required := range []string{
		"## Required Product & UI delivery",
		"a9shots0/smcrm",
		binding.SourceRevision,
		binding.PackSHA256,
		"Reuse the real product shell",
		"phone, medium, 1440 desktop and 1920 desktop",
		"three separate child review tasks",
		"three different reviewer agents",
		"not_only_better_than_current",
		"cognitive_load_progressive_disclosure",
		"decision_relevance",
		"brand_product_specificity",
		"<design-review-verdict>",
		"review_task_id",
		"reviewer_agent_id",
		"verdict_sha256",
		"desktop_1920",
		"<design-task-delivery>",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt is missing %q\n%s", required, prompt)
		}
	}
}

func TestBuildPromptOmitsProductDesignGateForOrdinaryTask(t *testing.T) {
	prompt := BuildPrompt(Task{IssueID: "issue-1"}, "claude")
	if strings.Contains(prompt, "Required Product & UI delivery") || strings.Contains(prompt, "<design-task-delivery>") {
		t.Fatalf("ordinary task received the Product & UI completion contract: %s", prompt)
	}
}
