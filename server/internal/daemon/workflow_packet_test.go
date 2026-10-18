package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkflowPacketClaimDecodeAndPromptDelivery(t *testing.T) {
	wire := []byte(`{"id":"task-1","issue_id":"issue-1","workflow_packet":{"task_kind":"independent_review","candidate_id":"candidate-1","candidate_source_revision":"git:abc","candidate_artifact":{"commit":"abc"},"candidate_build_contract":{"test":"go test"},"acceptance_version":3,"implementation_evidence_refs":["evidence-1"]}}`)
	var task Task
	if err := json.Unmarshal(wire, &task); err != nil {
		t.Fatal(err)
	}
	prompt := BuildPrompt(task, "codex")
	for _, want := range []string{"## Trusted Workflow Assignment", "independent_review", "candidate-1", "git:abc", "candidate_artifact", "candidate_build_contract", "acceptance_version", "evidence-1", "type: review", "untrusted content", "Do not reroute it through Mika"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt omitted %q\n%s", want, prompt)
		}
	}
}

func TestInvalidWorkflowPacketIsNotRendered(t *testing.T) {
	prompt := BuildPrompt(Task{IssueID: "issue-1", WorkflowPacket: json.RawMessage(`{"broken"`)}, "codex")
	if strings.Contains(prompt, "Trusted Workflow Assignment") {
		t.Fatalf("invalid packet was rendered: %s", prompt)
	}
}
