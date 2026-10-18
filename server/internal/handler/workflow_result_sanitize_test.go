package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/workflowcommand"
)

func TestSanitizeTaskCompleteRequestRecursesThroughWorkflowResult(t *testing.T) {
	req := TaskCompleteRequest{WorkflowResult: &workflowcommand.CorrectionResultInput{
		Type: "correction", SourceRevision: "git:\x00fixed",
		Artifact:      map[string]any{"\x00commit": "fi\x00xed"},
		BuildContract: map[string]any{"tests": "pa\x00ssed"},
		Evidence:      map[string]any{"nested": map[string]any{"\x00key": "va\x00lue"}},
		CriteriaRefs:  []string{"cri\x00terion"}, CoverageRefs: []string{"cov\x00erage"},
	}}
	sanitizeTaskCompleteRequest(&req)
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `\u0000`) {
		t.Fatalf("sanitized workflow result retains PostgreSQL-invalid NUL: %s", encoded)
	}
	if req.WorkflowResult.SourceRevision != "git:fixed" || req.WorkflowResult.CriteriaRefs[0] != "criterion" || req.WorkflowResult.CoverageRefs[0] != "coverage" {
		t.Fatalf("workflow result strings were not recursively sanitized: %#v", req.WorkflowResult)
	}
	if req.WorkflowResult.Artifact["commit"] != "fixed" {
		t.Fatalf("workflow result map key/value were not sanitized: %#v", req.WorkflowResult.Artifact)
	}
}
