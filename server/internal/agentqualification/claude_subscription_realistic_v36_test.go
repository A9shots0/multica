package agentqualification

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestClaudeSubscriptionRealisticV36IsReviewLockedAndExact(t *testing.T) {
	d, err := ClaudeSubscriptionRealisticV36()
	if err != nil {
		t.Fatal(err)
	}
	if d.ProviderCallsAllowed || d.Status != "prepared_for_independent_review" || d.Topology != "direct_candidate_to_deterministic_controller_no_reviewer_no_correction" {
		t.Fatalf("definition=%+v", d)
	}
	for path, want := range d.FrozenAntecedents {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read antecedent %s: %v", path, err)
		}
		if got := sha256Hex(raw); got != want {
			t.Fatalf("antecedent drift %s: %s", path, got)
		}
	}
}

func TestClaudeSubscriptionRealisticV36PlanIsToolFreeAndControllerOnly(t *testing.T) {
	request := RealisticProviderRequest{ProviderCallID: "f:attempt-1:turn-1", ProviderTurn: 1, CaseID: "R03", CaseVersion: "v2", RunFamilyID: "f", Attempt: 1, Task: "task", WorkspacePaths: []string{"TASK.md"}, AllowedActions: []string{"read", "search", "edit", "run_check", "browser", "checkpoint_and_interrupt", "finish"}, AllowedChecks: []string{"python -m unittest discover -s tests -v", "local-release-simulator verify", "local-release-simulator rollback"}, MaxSteps: 15}
	plan, err := BuildClaudeSubscriptionRealisticV36ActionPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.ArgsTemplate, " ")
	if strings.Contains(joined, "--bare") || !strings.Contains(joined, "--restricted --tools  --strict-mcp-config") || !strings.Contains(joined, "--no-session-persistence") || !strings.Contains(joined, "--max-turns 1") {
		t.Fatalf("unsafe args: %q", joined)
	}
	if strings.Contains(string(plan.Schema), "browser") || strings.Contains(string(plan.Schema), "checkpoint_and_interrupt") {
		t.Fatalf("provider can request harness-only actions: %s", plan.Schema)
	}
	if !strings.Contains(plan.Prompt, "deterministic controller") || strings.Contains(plan.Prompt, "reviewer") {
		t.Fatalf("topology prompt drift: %s", plan.Prompt)
	}
}

func TestClaudeSubscriptionRealisticV36NativeReceiptRoundTrips(t *testing.T) {
	request := RealisticProviderRequest{ProviderCallID: "f:attempt-1:turn-1", ProviderTurn: 1, CaseID: "R04", CaseVersion: "v1", RunFamilyID: "f", Attempt: 2, Task: "task", WorkspacePaths: []string{"TASK.md"}, AllowedActions: []string{"read", "search", "edit", "run_check", "browser", "checkpoint_and_interrupt", "finish"}, AllowedChecks: []string{"python resume.py", "python -m unittest discover -s tests -v"}, MaxSteps: 15}
	plan, err := BuildClaudeSubscriptionRealisticV36ActionPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"type":"result","subtype":"success","is_error":false,"result":"{\"action\":{\"kind\":\"read\",\"path\":\"TASK.md\"}}","structured_output":{"action":{"kind":"read","path":"TASK.md"}},"session_id":"s-v36","total_cost_usd":0.000001,"usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4},"modelUsage":{"claude-sonnet-4-6":{}}}`)
	response, err := ParseClaudeSubscriptionRealisticV36Response(request, plan, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if response.Action.Kind != "read" || response.InputTokens != 15 || response.OutputTokens != 4 || response.CostMicros != 1 || response.ReasoningTokens != 0 {
		t.Fatalf("response=%+v", response)
	}
	replayed, err := ParseClaudeRealisticV36ProviderReceipt(response.RawReceipt)
	if err != nil || replayed.ProviderCallID != request.ProviderCallID {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
}

func TestClaudeSubscriptionRealisticV36R04AttemptOneIsZeroCallHarnessTransition(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(t.TempDir(), "R04", "v36-r04")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	provider := realisticProviderFunc(func(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
		calls++
		return RealisticProviderResponse{}, nil
	})
	first, err := runRealisticCandidateFrozen(context.Background(), workspace, "v36-r04", 1, provider, nil, acceptRealisticCheckpoint)
	if err != nil || calls != 0 || first.TerminalState != "interrupted" || first.ProviderCalls != 0 || len(first.AttemptHistory) != 1 {
		t.Fatalf("first=%+v calls=%d err=%v", first, calls, err)
	}
}

func TestClaudeSubscriptionRealisticV36MalformedActionIsProtocolFailureNotEvidenceLoss(t *testing.T) {
	request := RealisticProviderRequest{ProviderCallID: "f:attempt-1:turn-1", ProviderTurn: 1, CaseID: "R03", CaseVersion: "v2", RunFamilyID: "f", Attempt: 1, Task: "task", WorkspacePaths: []string{"TASK.md"}, AllowedActions: []string{"read", "search", "edit", "run_check", "browser", "checkpoint_and_interrupt", "finish"}, AllowedChecks: []string{"python -m unittest discover -s tests -v", "local-release-simulator verify", "local-release-simulator rollback"}, MaxSteps: 15}
	plan, _ := BuildClaudeSubscriptionRealisticV36ActionPlan(request)
	rawMap := map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": "{}", "structured_output": map[string]any{"action": map[string]any{"kind": "deploy"}}, "session_id": "s", "total_cost_usd": 0, "usage": map[string]any{"input_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 1}, "modelUsage": map[string]any{"claude-sonnet-4-6": map[string]any{}}}
	raw, _ := json.Marshal(rawMap)
	response, err := ParseClaudeSubscriptionRealisticV36Response(request, plan, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if response.ProtocolFailure == "" || len(response.RawReceipt) == 0 {
		t.Fatalf("protocol failure lost receipt: %+v", response)
	}
}
