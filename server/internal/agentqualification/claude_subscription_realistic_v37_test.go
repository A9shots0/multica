package agentqualification

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeSubscriptionRealisticV37DefinitionIsBoundedAndReviewed(t *testing.T) {
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		t.Fatal(err)
	}
	if !d.ProviderCallsAllowed || d.Status != "frozen_reviewed" || d.Topology != "bounded_phased_candidate_plans_to_deterministic_controller_no_reviewer_no_correction" {
		t.Fatalf("definition=%+v", d)
	}
	if d.ExecutionOrder[0] != "R03:v2" || d.Caps.MaxProviderInvocations != 5 || d.Caps.MaxActionsPerPhase != 4 || d.Caps.MaxTotalActions != 20 || d.Caps.MaxRetries != 0 {
		t.Fatalf("case/caps=%+v", d.Caps)
	}
	if err := RequireClaudeSubscriptionRealisticV37Freeze(); err != nil {
		t.Fatalf("reviewed v37 remains locked: %v", err)
	}
}

func TestClaudeSubscriptionRealisticV37PhaseRequestIsCompactRestrictedAndLatestOnly(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R03", "v37-compact")
	if err != nil {
		t.Fatal(err)
	}
	older := RealisticToolResult{Sequence: 1, ActionKind: "read", Success: true, Output: "OLDER_FULL_OUTPUT_MUST_NOT_APPEAR", OutputHash: strings.Repeat("a", 64)}
	latest := RealisticToolResult{Sequence: 2, ActionKind: "read", Success: true, Output: "LATEST_PHASE_ONLY", OutputHash: strings.Repeat("e", 64)}
	olderRaw, _ := json.Marshal([]RealisticToolResult{older})
	latestRaw, _ := json.Marshal([]RealisticToolResult{latest})
	tree, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	state := ClaudeRealisticV37ExecutionState{
		CompletedPhases: 2,
		TotalActions:    2,
		Cumulative: []ClaudeRealisticV37CumulativeEntry{
			{Phase: 1, ActionCount: 1, ToolResultCount: 1, ResultsSHA256: sha256Hex(olderRaw), WorkspaceTreeSHA256: tree},
			{Phase: 2, ActionCount: 1, ToolResultCount: 1, ResultsSHA256: sha256Hex(latestRaw), WorkspaceTreeSHA256: tree},
		},
		PriorPhaseResults: []RealisticToolResult{latest},
		Controller:        RealisticControllerResult{ToolResults: []RealisticToolResult{older, latest}},
	}
	plan, request, err := BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, "v37-compact", state)
	if err != nil {
		t.Fatal(err)
	}
	if request.ProviderCallID != "v37-compact:attempt-1:phase-3" || request.ProviderTurn != 3 || request.MaxSteps != 4 || len(request.PreviousResults) != 1 || request.PreviousResults[0].Output != "LATEST_PHASE_ONLY" {
		t.Fatalf("request=%+v", request)
	}
	if !strings.Contains(request.Task, sha256Hex(olderRaw)) || !strings.Contains(request.Task, tree) || strings.Contains(request.Task, "LATEST_PHASE_ONLY") || strings.Contains(request.Task, "OLDER_FULL_OUTPUT_MUST_NOT_APPEAR") {
		t.Fatalf("task did not keep old context hash-only: %s", request.Task)
	}
	joined := strings.Join(plan.ArgsTemplate, " ")
	if !strings.Contains(joined, "--restricted --tools  --strict-mcp-config") || !strings.Contains(joined, "--no-session-persistence") || strings.Contains(joined, "--resume") || strings.Contains(joined, "--bare") {
		t.Fatalf("unsafe args: %q", joined)
	}
	if !strings.Contains(string(plan.Schema), `"maxItems":4`) || strings.Contains(string(plan.Schema), "browser") || !strings.Contains(plan.Prompt, "full tool outputs only from the immediately prior phase") {
		t.Fatalf("plan boundary drift: %+v", plan)
	}
}

func TestClaudeSubscriptionRealisticV37RejectsInvalidPlans(t *testing.T) {
	paths := map[string]struct{}{"TASK.md": {}, "app/access.py": {}}
	checks := []string{"python -m unittest discover -s tests -v"}
	cases := []ClaudeRealisticV37Plan{
		{Actions: []RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}, {Kind: "read", Path: "TASK.md"}, {Kind: "read", Path: "TASK.md"}, {Kind: "read", Path: "TASK.md"}, {Kind: "read", Path: "TASK.md"}}},
		{Actions: []RealisticCandidateAction{{Kind: "edit", Path: "../outside", Contents: "x"}}},
		{Actions: []RealisticCandidateAction{{Kind: "run_check", Check: "curl production"}}},
		{Actions: []RealisticCandidateAction{{Kind: "deploy"}}},
		{Actions: []RealisticCandidateAction{{Kind: "finish", Summary: "done"}, {Kind: "read", Path: "TASK.md"}}},
	}
	for index, candidate := range cases {
		if err := ValidateClaudeSubscriptionRealisticV37Plan(candidate, paths, checks, 4, false); err == nil {
			t.Fatalf("invalid plan %d accepted: %+v", index, candidate)
		}
	}
	if err := ValidateClaudeSubscriptionRealisticV37Plan(ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}}}, paths, checks, 4, true); err == nil {
		t.Fatal("fifth phase without finish accepted")
	}
	for _, creation := range []string{"release/receipt.json", "EVIDENCE.md"} {
		if err := ValidateClaudeSubscriptionRealisticV37Plan(ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "edit", Path: creation, Contents: "bounded"}}}, paths, checks, 4, false); err != nil {
			t.Fatalf("frozen R03 creation %s rejected: %v", creation, err)
		}
		if err := ValidateClaudeSubscriptionRealisticV37Plan(ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "read", Path: creation}}}, paths, checks, 4, false); err == nil {
			t.Fatalf("nonexistent read %s accepted", creation)
		}
	}
}

func TestClaudeSubscriptionRealisticV37CheckpointResumeDoesNotDuplicatePaidCall(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R03", "v37-resume")
	if err != nil {
		t.Fatal(err)
	}
	_, request, err := BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, "v37-resume", ClaudeRealisticV37ExecutionState{})
	if err != nil {
		t.Fatal(err)
	}
	plan := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}, {Kind: "search", Query: "rollback"}}}
	receipt := claudeRealisticV37FixtureReceipt(t, request, plan, 40, 7, 2)
	errCrash := errors.New("simulated durable-process crash")
	crashed, err := ExecuteClaudeSubscriptionRealisticV37Phase(workspace, "v37-resume", 1, plan, receipt, ClaudeRealisticV37ExecutionState{}, func(state ClaudeRealisticV37ExecutionState) error {
		if state.NextAction == 1 {
			return errCrash
		}
		return nil
	})
	if !errors.Is(err, errCrash) || crashed.Controller.ProviderCalls != 1 || crashed.NextAction != 1 || crashed.PendingPlan == nil {
		t.Fatalf("crash checkpoint=%+v err=%v", crashed, err)
	}
	resumed, err := ExecuteClaudeSubscriptionRealisticV37Phase(workspace, "v37-resume", 1, plan, receipt, crashed, func(ClaudeRealisticV37ExecutionState) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Controller.ProviderCalls != 1 || resumed.CompletedPhases != 1 || len(resumed.Controller.ToolResults) != 2 || resumed.PendingPlan != nil || resumed.Controller.InputTokens != 40 || resumed.Controller.OutputTokens != 7 || resumed.Controller.CostMicros != 2 {
		t.Fatalf("resume duplicated or lost accounting: %+v", resumed)
	}
}

func TestClaudeSubscriptionRealisticV37ReceiptReplayAndAccountingAreExact(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R03", "v37-receipt")
	if err != nil {
		t.Fatal(err)
	}
	_, request, _ := BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, "v37-receipt", ClaudeRealisticV37ExecutionState{})
	plan := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}}}
	receiptRaw := claudeRealisticV37FixtureReceipt(t, request, plan, 23, 5, 3)
	receipt, err := ParseClaudeRealisticV37ProviderReceipt(receiptRaw)
	if err != nil || receipt.InputTokens != 23 || receipt.OutputTokens != 5 || receipt.CostMicros != 3 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	var tampered map[string]any
	if err := json.Unmarshal(receiptRaw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["input_tokens"] = float64(24)
	tamperedRaw, _ := json.Marshal(tampered)
	if _, err := ParseClaudeRealisticV37ProviderReceipt(tamperedRaw); err == nil {
		t.Fatal("forged accounting accepted")
	}
	if _, ok := checkedAddClaudeV37Int(math.MaxInt, 1); ok {
		t.Fatal("integer overflow accepted")
	}
}

func TestClaudeSubscriptionRealisticV37RejectsExecutedPlanDifferentFromRawReceipt(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R03", "v37-plan-bind")
	if err != nil {
		t.Fatal(err)
	}
	_, request, _ := BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, "v37-plan-bind", ClaudeRealisticV37ExecutionState{})
	receiptPlan := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}}}
	receipt := claudeRealisticV37FixtureReceipt(t, request, receiptPlan, 10, 2, 1)
	executedPlan := ClaudeRealisticV37Plan{Actions: []RealisticCandidateAction{{Kind: "search", Query: "rollback"}}}
	if _, err := ExecuteClaudeSubscriptionRealisticV37Phase(workspace, "v37-plan-bind", 1, executedPlan, receipt, ClaudeRealisticV37ExecutionState{}, func(ClaudeRealisticV37ExecutionState) error { return nil }); err == nil {
		t.Fatal("plan differing from raw Claude receipt was executed")
	}
}

func TestClaudeSubscriptionRealisticV37PhaseRequestRejectsStateAndWorkspaceDrift(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(filepath.Clean(t.TempDir()), "R03", "v37-state-bind")
	if err != nil {
		t.Fatal(err)
	}
	result := RealisticToolResult{Sequence: 1, ActionKind: "read", Success: true, Output: "ok", OutputHash: strings.Repeat("a", 64)}
	resultRaw, _ := json.Marshal([]RealisticToolResult{result})
	tree, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	state := ClaudeRealisticV37ExecutionState{CompletedPhases: 1, TotalActions: 1, Controller: RealisticControllerResult{ToolResults: []RealisticToolResult{result}}, PriorPhaseResults: []RealisticToolResult{result}, Cumulative: []ClaudeRealisticV37CumulativeEntry{{Phase: 1, ActionCount: 1, ToolResultCount: 1, ResultsSHA256: sha256Hex(resultRaw), WorkspaceTreeSHA256: tree}}}
	state.PriorPhaseResults[0].Output = "forged"
	if _, _, err := BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, "v37-state-bind", state); err == nil {
		t.Fatal("forged prior-phase full output accepted")
	}
	state.PriorPhaseResults = []RealisticToolResult{result}
	if err := workspace.Edit("app/access.py", []byte("def employee_pages():\n    return []\n")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, "v37-state-bind", state); err == nil {
		t.Fatal("workspace drift accepted")
	}
}

func claudeRealisticV37FixtureReceipt(t *testing.T, request RealisticProviderRequest, candidate ClaudeRealisticV37Plan, input, output, cost int64) []byte {
	t.Helper()
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		t.Fatal(err)
	}
	transportPlan, err := buildClaudeSubscriptionRealisticV37Plan(d, request)
	if err != nil {
		t.Fatal(err)
	}
	candidateRaw, _ := json.Marshal(candidate)
	rawMap := map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"result": string(candidateRaw), "structured_output": candidate,
		"session_id": "fixture-v37", "total_cost_usd": float64(cost) / 1_000_000,
		"usage":      map[string]any{"input_tokens": input, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": output},
		"modelUsage": map[string]any{"claude-sonnet-4-6": map[string]any{}},
	}
	raw, _ := json.Marshal(rawMap)
	gotPlan, receipt, err := ParseClaudeSubscriptionRealisticV37Response(request, transportPlan, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	gotRaw, _ := json.Marshal(gotPlan)
	wantRaw, _ := json.Marshal(candidate)
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("candidate plan changed: got=%s want=%s", gotRaw, wantRaw)
	}
	return receipt
}
