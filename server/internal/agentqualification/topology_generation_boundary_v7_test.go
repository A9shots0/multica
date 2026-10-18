package agentqualification

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type topologyV7UsageProvider struct {
	response RealisticProviderResponse
	calls    int
}

func (provider *topologyV7UsageProvider) Next(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
	provider.calls++
	return provider.response, nil
}

func topologyV7ExecutorRequest() RealisticProviderRequest {
	return RealisticProviderRequest{
		ProviderCallID: "v7-executor-boundary", ProviderTurn: 1,
		CaseID: "R03", CaseVersion: "v2", RunFamilyID: "v7-boundary",
		Attempt: 1, Task: "sealed boundary test", WorkspacePaths: []string{"TASK.md"},
		AllowedActions: []string{"read", "search", "edit", "run_check", "finish"}, MaxSteps: 15,
	}
}

func TestTopologyV7ExecutorPlanRejectsEveryIdentityMutation(t *testing.T) {
	definition, err := CurrentTopologyExperimentV7()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeTopologyActionPlanForFrozenExecutor(definition.Executor, definition.Executor, topologyV7ExecutorRequest())
	if err != nil || plan.Transport != OpenCodeTopologyActionTransportV7 {
		t.Fatalf("exact v7 executor was not admitted: plan=%+v err=%v", plan, err)
	}
	mutations := map[string]func(*RealisticCandidate){
		"configuration id": func(candidate *RealisticCandidate) { candidate.ConfigurationID = "wrong" },
		"model":            func(candidate *RealisticCandidate) { candidate.Model = "openrouter/inception/not-mercury" },
		"effort":           func(candidate *RealisticCandidate) { candidate.Effort = "high" },
		"provider":         func(candidate *RealisticCandidate) { candidate.ServingProvider = "not-inception" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := definition.Executor
			mutate(&candidate)
			if _, err := BuildOpenCodeTopologyActionPlanForFrozenExecutor(definition.Executor, candidate, topologyV7ExecutorRequest()); err == nil {
				t.Fatal("mutated executor was accepted")
			}
			if _, _, err := SealTopologyV7ExecutionReceiptForFrozenExecutor(definition.Executor, candidate, filepath.Join(t.TempDir(), "evidence"), RealisticCaseWorkspace{}, RealisticControllerResult{}); err == nil || !strings.Contains(err.Error(), "not frozen") {
				t.Fatalf("mutated executor reached sealer internals: %v", err)
			}
		})
	}
}

func TestTopologyV7PlanAndSealerAcceptTheSameFreshExecutor(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	if _, err := BuildOpenCodeTopologyActionPlanForFrozenExecutor(definition.Executor, definition.Executor, topologyV7ExecutorRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOpenCodeRealisticActionPlan(definition.Executor, topologyV7ExecutorRequest()); err == nil {
		t.Fatal("historical realistic matrix unexpectedly admitted the fresh v7 executor")
	}

	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("consumer_faulty/normalize.py", []byte("def normalize_role(role: str) -> str:\n    return role\n")); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("EVIDENCE.md", []byte("Current local truth: affected and control checks passed; authority unchanged.\n")); err != nil {
		t.Fatal(err)
	}
	finalHash, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := strings.Repeat("a", 64)
	controller := RealisticControllerResult{
		CaseID: "R01", RunFamilyID: "v7-sealer-boundary", Attempt: 1, TerminalState: "completed",
		ToolEvents: []RealisticToolEvent{
			{Sequence: 1, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 1, StartedAt: now, CompletedAt: now, OutputSHA256: hash},
			{Sequence: 2, Tool: "edit_within_workspace", ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash, TouchedPaths: []string{"consumer_faulty/normalize.py", "EVIDENCE.md"}},
			{Sequence: 3, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash},
		},
		AttemptHistory: []RealisticAttemptCheckpoint{{Attempt: 1, TerminalState: "completed", WorkspaceTreeSHA256: finalHash}},
	}
	applySealableProviderFixture(&controller)
	if _, _, err := SealRealisticExecutionReceipt(filepath.Join(parent, "old-sealer"), workspace, definition.Executor, controller); err == nil {
		t.Fatal("historical sealer unexpectedly admitted the fresh v7 executor")
	}
	if _, _, err := SealTopologyV7ExecutionReceiptForFrozenExecutor(definition.Executor, definition.Executor, filepath.Join(parent, "v7-sealer"), workspace, controller); err != nil {
		t.Fatalf("v7 plan/sealer boundary disagreed on the exact executor: %v", err)
	}
}

func TestTopologyV7ReviewerRouteIsExactAndRejectsInheritedState(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	request := topologyReviewFixture(t)
	plan, err := BuildOpenCodeTopologyV7ReviewPlanForFrozenReviewer(definition.Reviewer, definition.Reviewer, request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Transport != OpenCodeTopologyReviewTransportV7 || plan.Transport == OpenCodeTopologyReviewTransport {
		t.Fatalf("reviewer used a transport alias: %s", plan.Transport)
	}
	if err := ValidateTopologyV7PlanBinding(definition.Reviewer, plan, OpenCodeTopologyReviewTransportV7); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateTopologyV7ResolvedConfig(plan, plan.InlineConfig); err != nil {
		t.Fatal(err)
	}
	for name, replacement := range map[string]string{
		"fallback":     strings.Replace(string(plan.InlineConfig), `"allow_fallbacks":false`, `"allow_fallbacks":true`, 1),
		"order":        strings.Replace(string(plan.InlineConfig), `"order":["anthropic"]`, `"order":["other","anthropic"]`, 1),
		"tools":        strings.Replace(string(plan.InlineConfig), `"tools":{"*":false}`, `"tools":{"*":true}`, 1),
		"instructions": strings.Replace(string(plan.InlineConfig), `"instructions":[]`, `"instructions":["inherited"]`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateTopologyV7ResolvedConfig(plan, []byte(replacement)); err == nil {
				t.Fatal("unsafe reviewer route mutation was accepted")
			}
		})
	}
	if _, err := ValidateTopologyV7ResolvedConfig(plan, append(append([]byte(nil), plan.InlineConfig...), []byte(`{"trailing":true}`)...)); err == nil {
		t.Fatal("trailing resolved configuration was accepted")
	}
	if _, err := ValidateTopologyV7ResolvedConfig(plan, append(append([]byte(nil), plan.InlineConfig...), []byte(`not-json`)...)); err == nil {
		t.Fatal("malformed trailing resolved configuration was accepted")
	}
}

func TestTopologyV7StageBudgetCountsOverCapFinishExactly(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	definition.Status = "frozen_reviewed"
	workspace, err := PrepareRealisticCaseWorkspace(t.TempDir(), "R01")
	if err != nil {
		t.Fatal(err)
	}
	provider := &topologyV7UsageProvider{response: RealisticProviderResponse{
		Action: RealisticCandidateAction{Kind: "finish", Summary: "paid finish"}, RawReceipt: []byte(`{"paid":true}`),
		InputTokens: 8, OutputTokens: 3, ReasoningTokens: 1, CostMicros: 77,
	}}
	result, runErr := RunTopologyCandidateV7(context.Background(), definition, workspace, "v7-overcap-finish", []byte("bounded task"), 2, 10, 5, provider, nil)
	if !errors.Is(runErr, ErrTopologyStageBudgetExceeded) || provider.calls != 1 || result.ProviderCalls != 1 || result.InputTokens != 8 || result.OutputTokens != 3 || result.ReasoningTokens != 1 || result.CostMicros != 77 || result.ProviderAccountingStatus != "exact" {
		t.Fatalf("over-cap finish lost exact accounting or made a phantom call: calls=%d result=%+v err=%v", provider.calls, result, runErr)
	}
}

func TestTopologyV7StageBudgetCountsOverCapNonFinishExactly(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	definition.Status = "frozen_reviewed"
	workspace, err := PrepareRealisticCaseWorkspace(t.TempDir(), "R01")
	if err != nil {
		t.Fatal(err)
	}
	provider := &topologyV7UsageProvider{response: RealisticProviderResponse{
		Action: RealisticCandidateAction{Kind: "read", Path: "TASK.md"}, RawReceipt: []byte(`{"paid":true}`),
		InputTokens: 9, OutputTokens: 2, ReasoningTokens: 1, CostMicros: 88,
	}}
	result, runErr := RunTopologyCandidateV7(context.Background(), definition, workspace, "v7-overcap-read", []byte("bounded task"), 2, 10, 5, provider, nil)
	if !errors.Is(runErr, ErrTopologyStageBudgetExceeded) || provider.calls != 1 || result.ProviderCalls != 1 || result.InputTokens != 9 || result.OutputTokens != 2 || result.ReasoningTokens != 1 || result.CostMicros != 88 || len(result.ToolEvents) != 0 || result.ProviderAccountingStatus != "exact" {
		t.Fatalf("over-cap non-finish lost exact accounting or executed work: calls=%d result=%+v err=%v", provider.calls, result, runErr)
	}
}

func TestTopologyV7ExhaustedStageRejectsBeforeVirtualOrPaidCall(t *testing.T) {
	inner := &topologyCapturingProvider{}
	provider := &topologyV7StageProvider{inner: inner, task: "bounded", maxCalls: 1, calls: 1}
	_, err := provider.Next(context.Background(), RealisticProviderRequest{Task: "unbounded"})
	if !errors.Is(err, ErrTopologyStageBudgetExceeded) || !errors.Is(err, ErrRealisticProviderPreflight) || len(inner.requests) != 0 || provider.calls != 1 {
		t.Fatalf("exhausted stage created a phantom call: inner=%d calls=%d err=%v", len(inner.requests), provider.calls, err)
	}
}

func TestTopologyV7CallCeilingRemovesControllerVirtualCall(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	definition.Status = "frozen_reviewed"
	workspace, err := PrepareRealisticCaseWorkspace(t.TempDir(), "R01")
	if err != nil {
		t.Fatal(err)
	}
	provider := &topologyV7UsageProvider{response: RealisticProviderResponse{
		Action: RealisticCandidateAction{Kind: "read", Path: "TASK.md"}, RawReceipt: []byte(`{"paid":true}`),
		InputTokens: 4, OutputTokens: 2, ReasoningTokens: 1, CostMicros: 91,
	}}
	result, runErr := RunTopologyCandidateV7(context.Background(), definition, workspace, "v7-call-ceiling", []byte("bounded task"), 1, 100, 50, provider, nil)
	if !errors.Is(runErr, ErrTopologyStageBudgetExceeded) || provider.calls != 1 || result.ProviderCalls != 1 || result.InputTokens != 4 || result.OutputTokens != 2 || result.ReasoningTokens != 1 || result.CostMicros != 91 || result.PendingProviderCall != nil || result.ProviderAccountingStatus != "exact" {
		t.Fatalf("call ceiling left a phantom virtual call: calls=%d result=%+v err=%v", provider.calls, result, runErr)
	}
}
