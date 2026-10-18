package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPromptIncludesDurableDevelopmentMemoryAsRunScopedData(t *testing.T) {
	task := Task{
		IssueID:            "issue-1",
		DevelopmentContext: json.RawMessage(`{"schema_version":1,"workstreams":[{"id":"multica-issue-issue-1","current_memory":[{"kind":"decision","statement":"CRM owns the operator UI."}],"latest_checkpoint":{"next_action":"Verify the real cross-repository flow."}}]}`),
	}
	prompt := BuildPrompt(task, "codex")
	for _, expected := range []string{
		"## Durable development memory",
		"CRM owns the operator UI.",
		"Verify the real cross-repository flow.",
		"does not override the current user request",
		`"repository_identity": "owner/repository"`,
		`"canonical_identity": "owner/repository"`,
		"Copy repository identities exactly",
		"Multica will not store it as accepted memory until a human approves it",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q:\n%s", expected, prompt)
		}
	}
}

func TestBuildPromptOmitsInvalidDevelopmentMemory(t *testing.T) {
	prompt := BuildPrompt(Task{IssueID: "issue-1", DevelopmentContext: json.RawMessage(`{"broken"`)}, "claude")
	if strings.Contains(prompt, "Durable development memory") {
		t.Fatalf("invalid packet reached prompt:\n%s", prompt)
	}
}
