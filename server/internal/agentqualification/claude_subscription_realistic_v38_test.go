package agentqualification

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeSubscriptionRealisticV38IsR04OnlyAndReviewedFrozen(t *testing.T) {
	d, err := ClaudeSubscriptionRealisticV38()
	if err != nil {
		t.Fatal(err)
	}
	if !d.ProviderCallsAllowed || d.Status != "reviewed_frozen" || d.ExecutionOrder[0] != "R04:v1" || d.Caps.MaxProviderInvocations != 5 || d.Caps.MaxActionsPerPhase != 4 || d.Caps.MaxRetries != 0 {
		t.Fatalf("definition=%+v", d)
	}
	if err := RequireClaudeSubscriptionRealisticV38Freeze(); err != nil {
		t.Fatalf("reviewed freeze rejected: %v", err)
	}
}

func TestClaudeSubscriptionRealisticV38FinishCleanupIsCrashRecoverable(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R04", "v38-finish-crash")
	if err != nil {
		t.Fatal(err)
	}
	state, err := InitializeClaudeSubscriptionRealisticV38(workspace, "v38-finish-crash", func(RealisticControllerResult) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	transport, request, _ := BuildClaudeSubscriptionRealisticV38PhaseRequest(workspace, "v38-finish-crash", state)
	plan := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "finish", Summary: "Same-family attempt-2 recovery completed exactly once; superseded status rejected."}}}
	receipt := fixtureClaudeV38Receipt(t, transport, request, plan, 10, 2, 1)
	crash := errors.New("crash after finish action")
	checkpointed, err := ExecuteClaudeSubscriptionRealisticV38Phase(workspace, "v38-finish-crash", plan, receipt, state, func(next ClaudeRealisticV37ExecutionState) error {
		if next.Controller.TerminalState == "completed" && next.PendingPlan != nil {
			return crash
		}
		return nil
	})
	if !errors.Is(err, crash) || checkpointed.PendingPlan == nil || len(checkpointed.Controller.AttemptHistory) != 1 {
		t.Fatalf("finish checkpoint=%+v err=%v", checkpointed, err)
	}
	resumed, err := ExecuteClaudeSubscriptionRealisticV38Phase(workspace, "v38-finish-crash", plan, receipt, checkpointed, func(ClaudeRealisticV37ExecutionState) error { return nil })
	if err != nil || resumed.PendingPlan != nil || resumed.Controller.ProviderCalls != 1 || len(resumed.Controller.AttemptHistory) != 2 || resumed.Controller.AttemptHistory[1].Attempt != 2 {
		t.Fatalf("finish recovery=%+v err=%v", resumed, err)
	}
}

func TestClaudeSubscriptionRealisticV38RawReceiptAndAccountingAreReplayBound(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R04", "v38-replay")
	if err != nil {
		t.Fatal(err)
	}
	state, err := InitializeClaudeSubscriptionRealisticV38(workspace, "v38-replay", func(RealisticControllerResult) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	transport, request, err := BuildClaudeSubscriptionRealisticV38PhaseRequest(workspace, "v38-replay", state)
	if err != nil {
		t.Fatal(err)
	}
	plan := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}}}
	receipt := fixtureClaudeV38Receipt(t, transport, request, plan, 10, 2, 1)
	forged := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "search", Query: "checkpoint"}}}
	if _, err := ExecuteClaudeSubscriptionRealisticV38Phase(workspace, "v38-replay", forged, receipt, state, func(ClaudeRealisticV37ExecutionState) error { return nil }); err == nil || !strings.Contains(err.Error(), "raw plan replay") {
		t.Fatalf("executed plan diverged from raw receipt: %v", err)
	}
	overflow := state
	overflow.Controller.InputTokens = math.MaxInt64
	if _, err := ExecuteClaudeSubscriptionRealisticV38Phase(workspace, "v38-replay", plan, receipt, overflow, func(ClaudeRealisticV37ExecutionState) error { return nil }); err == nil || !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("overflow accepted: %v", err)
	}
}

func TestClaudeSubscriptionRealisticV38ForcedAttemptOneAndPhasedAttemptTwo(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R04", "v38-family")
	if err != nil {
		t.Fatal(err)
	}
	state, err := InitializeClaudeSubscriptionRealisticV38(workspace, "v38-family", func(RealisticControllerResult) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if state.Controller.Attempt != 2 || state.Controller.ProviderCalls != 0 || len(state.Controller.AttemptHistory) != 1 || state.Controller.AttemptHistory[0].TerminalState != "interrupted" {
		t.Fatalf("state=%+v", state)
	}
	transport, request, err := BuildClaudeSubscriptionRealisticV38PhaseRequest(workspace, "v38-family", state)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(transport.ArgsTemplate, " ")
	if strings.Contains(joined, "--resume") || !strings.Contains(joined, "--restricted --tools  --strict-mcp-config") {
		t.Fatalf("args=%s", joined)
	}
	plan := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "run_check", Check: "python resume.py"}, {Kind: "edit", Path: "HANDOFF.md", Contents: "Run family v38-family resumed from durable checkpoint. Legacy completion is superseded; remaining verification completed exactly once.\n"}, {Kind: "run_check", Check: "python -m unittest discover -s tests -v"}, {Kind: "finish", Summary: "Same-family attempt-2 recovery completed exactly once; superseded status rejected."}}}
	candidateRaw, _ := json.Marshal(plan)
	rawMap := map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": string(candidateRaw), "structured_output": plan, "session_id": "v38", "total_cost_usd": 0.000001, "usage": map[string]any{"input_tokens": 10, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 2}, "modelUsage": map[string]any{"claude-sonnet-4-6": map[string]any{}}}
	raw, _ := json.Marshal(rawMap)
	parsed, receipt, err := ParseClaudeSubscriptionRealisticV38Response(request, transport, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	state, err = ExecuteClaudeSubscriptionRealisticV38Phase(workspace, "v38-family", parsed, receipt, state, func(ClaudeRealisticV37ExecutionState) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if state.Controller.TerminalState != "completed" || state.Controller.ProviderCalls != 1 || len(state.Controller.AttemptHistory) != 2 || state.Controller.AttemptHistory[1].Attempt != 2 || state.Controller.AttemptHistory[1].ProviderCalls != 1 {
		t.Fatalf("state=%+v", state)
	}
}

func TestClaudeSubscriptionRealisticV38RejectsUnsafePlansAndStateDrift(t *testing.T) {
	paths := map[string]struct{}{"TASK.md": {}, "HANDOFF.md": {}}
	checks := []string{"python resume.py"}
	for _, plan := range []ClaudeRealisticV37Plan{{Actions: []RealisticCandidateAction{{Kind: "deploy"}}}, {Actions: []RealisticCandidateAction{{Kind: "read", Path: "missing"}}}, {Actions: []RealisticCandidateAction{{Kind: "edit", Path: "FINAL-OUTPUT.json", Contents: "fake"}}}, {Actions: []RealisticCandidateAction{{Kind: "run_check", Check: "curl prod"}}}} {
		if ValidateClaudeSubscriptionRealisticV38Plan(plan, paths, checks, false) == nil {
			t.Fatalf("accepted %+v", plan)
		}
	}
	workspace, _ := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R04", "v38-drift")
	state, _ := InitializeClaudeSubscriptionRealisticV38(workspace, "v38-drift", func(RealisticControllerResult) error { return nil })
	state.Controller.AttemptHistory[0].ProviderCalls = 1
	if _, _, err := BuildClaudeSubscriptionRealisticV38PhaseRequest(workspace, "v38-drift", state); err == nil {
		t.Fatal("forged zero-call attempt accepted")
	}
}

func fixtureClaudeV38Receipt(t *testing.T, transport ClaudeScreeningPlan, request RealisticProviderRequest, candidate ClaudeRealisticV37Plan, input, output, cost int64) []byte {
	t.Helper()
	candidateRaw, _ := json.Marshal(candidate)
	rawMap := map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": string(candidateRaw), "structured_output": candidate, "session_id": request.ProviderCallID, "total_cost_usd": float64(cost) / 1_000_000, "usage": map[string]any{"input_tokens": input, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": output}, "modelUsage": map[string]any{"claude-sonnet-4-6": map[string]any{}}}
	raw, _ := json.Marshal(rawMap)
	parsed, receipt, err := ParseClaudeSubscriptionRealisticV38Response(request, transport, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mustJSONV37(parsed), mustJSONV37(candidate)) {
		t.Fatal("candidate changed")
	}
	return receipt
}
