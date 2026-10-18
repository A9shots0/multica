package agentqualification

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type topologyV8CostCrossingProvider struct {
	calls int
	cost  int64
}

func TestTopologyV8ExactCallCeilingIsObservationLocalExhaustion(t *testing.T) {
	hash := strings.Repeat("a", 64)
	result := TopologyExperimentResult{
		ClaimID: "claim", ObservationID: "v8-r04-p1-reviewed", CaseID: "R04", Repeat: 1, ArmID: "reviewed-v8",
		Outcome: "invalid", FailureClass: "stage_budget_exhaustion", AccountingStatus: "exact", InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts: []TopologyStageReceipt{
			{RunID: "claim:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 1},
			{RunID: "claim:review-1", Stage: "review_1", ProviderCalls: 1, InputTokens: 1},
			{RunID: "claim:correction", Stage: "correction", ProviderCalls: 8, InputTokens: 8, Correction: true},
		}, ProviderCalls: 10, TotalTokens: 10, CostMicros: 0, PostBatchCorrection: "exact call ceiling stopped the next call",
	}
	definition, _ := CurrentTopologyExperimentV8()
	if err := validateTopologyV8Result(definition, result); err != nil {
		t.Fatalf("exact correction call ceiling was not observation-local exhaustion: %v", err)
	}
}

func TestTopologyV8R04PairIntegrityIgnoresRunBoundInitialTree(t *testing.T) {
	hashA, hashB, taskHash, fixtureHash := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64)
	prior := TopologyExperimentResult{
		ClaimID: "direct-claim", ObservationID: "v8-r04-p1-direct", CaseID: "R04", Repeat: 1, ArmID: "direct-v8", Outcome: "fail", FailureClass: "competence_failure", AccountingStatus: "exact",
		InitialTreeSHA256: hashA, TaskSHA256: taskHash, FixtureSHA256: fixtureHash, FinalTreeSHA256: hashA, HiddenGradeSHA256: hashA,
		StageReceipts: []TopologyStageReceipt{{RunID: "direct-claim:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 1}, {RunID: "direct-claim:hidden-grade", Stage: "hidden_grade"}}, ProviderCalls: 1, TotalTokens: 1,
	}
	preserved := TopologyExperimentResult{
		ClaimID: "reviewed-claim", ObservationID: "v8-r04-p1-reviewed", CaseID: "R04", Repeat: 1, ArmID: "reviewed-v8", Outcome: "invalid", FailureClass: "transport_or_harness_imperfection", AccountingStatus: "exact", PostBatchCorrection: "preserved",
		InitialTreeSHA256: hashB, TaskSHA256: taskHash, FixtureSHA256: fixtureHash, FinalTreeSHA256: hashB, HiddenGradeSHA256: hashB,
		StageReceipts: []TopologyStageReceipt{{RunID: "reviewed-claim:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 1}}, ProviderCalls: 1, TotalTokens: 1,
	}
	definition, _ := CurrentTopologyExperimentV8()
	state := TopologyCampaignState{Observations: []TopologyExperimentResult{prior}, ActiveClaim: preserved.ClaimID, TotalProviderCalls: 1, TotalTokens: 1}
	next, err := RecordTopologyV8Observation(state, definition, preserved)
	if err != nil || next.GlobalHaltReason != "" || len(next.Observations) != 2 {
		t.Fatalf("run-bound initial tree caused a false pair halt: state=%+v err=%v", next, err)
	}
}

func (provider *topologyV8CostCrossingProvider) Next(_ context.Context, _ RealisticProviderRequest) (RealisticProviderResponse, error) {
	provider.calls++
	return RealisticProviderResponse{
		Action: RealisticCandidateAction{Kind: "read", Path: "TASK.md"}, RawReceipt: []byte(`{"provider":"fixture"}`),
		InputTokens: 1, OutputTokens: 1, CostMicros: provider.cost,
	}, nil
}

func TestTopologyV8DefinitionIsExactAndFrozenReviewed(t *testing.T) {
	d, err := CurrentTopologyExperimentV8()
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "frozen_reviewed" || !TopologyExperimentV8ProviderReady(d) {
		t.Fatal("reviewed topology v8 was not atomically activated")
	}
	if d.Executor != (RealisticCandidate{"realistic.gpt54.medium.executor.topology-v8", "openrouter/openai/gpt-5.4", "medium", "openai"}) || d.Reviewer != (RealisticCandidate{"realistic.claude-sonnet5.medium.reviewer.topology-v8", "openrouter/anthropic/claude-sonnet-5", "medium", "anthropic"}) {
		t.Fatal("route tuple drifted")
	}
	want := []TopologyExperimentObservation{{"v8-r04-p1-direct", "R04", 1, "direct-v8"}, {"v8-r04-p1-reviewed", "R04", 1, "reviewed-v8"}, {"v8-r04-p2-reviewed", "R04", 2, "reviewed-v8"}, {"v8-r04-p2-direct", "R04", 2, "direct-v8"}}
	for i := range want {
		if d.ExecutionOrder[i] != want[i] {
			t.Fatal("counterbalance drifted")
		}
	}
	if 2*d.Arms[0].TotalTokenReservation+2*d.Arms[1].TotalTokenReservation != d.MaxTotalTokens || 2*d.Arms[0].CostCeilingMicros+2*d.Arms[1].CostCeilingMicros != d.MaxIncrementalSpendMicros {
		t.Fatal("campaign geometry drifted")
	}
}

func TestTopologyV8R04ForcedResumeAndCorrectionDoNotReplay(t *testing.T) {
	d, _ := CurrentTopologyExperimentV8()
	d.Status = "frozen_reviewed"
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R04")
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "bounded"}}}
	got, err := RunTopologyCandidateV8(context.Background(), d, workspace, "r04-family", []byte("task"), 15, 60000, 12000, provider, acceptRealisticCheckpoint)
	if err != nil || got.Attempt != 2 || got.ProviderCalls != 1 || len(got.AttemptHistory) < 2 || got.AttemptHistory[0].TerminalState != "interrupted" {
		t.Fatalf("forced resume drifted: %+v err=%v", got, err)
	}
	provider = &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "corrected"}}}
	correction, err := RunTopologyCorrectionV8(context.Background(), d, workspace, "correction-family", []byte("correction"), 8, 40000, 10000, provider, acceptRealisticCheckpoint)
	if err != nil || correction.Attempt != 2 || correction.ProviderCalls != 1 {
		t.Fatalf("correction replayed interruption: %+v err=%v", correction, err)
	}
}

func TestTopologyV8StageCapsAndRoutes(t *testing.T) {
	d, _ := CurrentTopologyExperimentV8()
	base := TopologyExperimentResult{Outcome: "pass"}
	base.StageReceipts = []TopologyStageReceipt{{RunID: "x", Stage: "execute", ProviderCalls: 15, InputTokens: 48000, OutputTokens: 6000, ReasoningTokens: 6000, CostMicros: 250000}}
	if err := validateTopologyV8StageCaps(base); err != nil {
		t.Fatal(err)
	}
	base.StageReceipts[0].ReasoningTokens++
	base.Outcome, base.FailureClass = "invalid", "stage_budget_exhaustion"
	if err := validateTopologyV8StageCaps(base); err != nil {
		t.Fatal("exact paid over-cap response was not preserved as local invalid")
	}
	base.Outcome, base.FailureClass = "pass", ""
	if err := validateTopologyV8StageCaps(base); err == nil {
		t.Fatal("completed over-cap stage was accepted")
	}
	proof, err := PrevalidateTopologyV8Routes(d)
	if err != nil {
		t.Fatal(err)
	}
	if proof.ProviderCalls != 0 || proof.ExecutorPlanSHA256 == "" || proof.ReviewerPlanSHA256 == "" {
		t.Fatal("prevalidation was not provider-free")
	}
}

func TestTopologyV8ExecuteAndCorrectionStopBeforeCallAfterCostCrossing(t *testing.T) {
	d, _ := CurrentTopologyExperimentV8()
	d.Status = "frozen_reviewed"
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R04")
	if err != nil {
		t.Fatal(err)
	}

	executeProvider := &topologyV8CostCrossingProvider{cost: 250001}
	execute, executeErr := RunTopologyCandidateV8(context.Background(), d, workspace, "execute-cost-family", []byte("task"), 15, 60000, 12000, executeProvider, acceptRealisticCheckpoint)
	if !errors.Is(executeErr, ErrTopologyStageBudgetExceeded) || executeProvider.calls != 1 || execute.ProviderCalls != 1 || execute.CostMicros != 250001 {
		t.Fatalf("execute escaped cost checkpoint: calls=%d result=%+v err=%v", executeProvider.calls, execute, executeErr)
	}

	correctionProvider := &topologyV8CostCrossingProvider{cost: 200001}
	correction, correctionErr := RunTopologyCorrectionV8(context.Background(), d, workspace, "correction-cost-family", []byte("correction"), 8, 40000, 10000, correctionProvider, acceptRealisticCheckpoint)
	if !errors.Is(correctionErr, ErrTopologyStageBudgetExceeded) || correctionProvider.calls != 1 || correction.ProviderCalls != 1 || correction.CostMicros != 200001 {
		t.Fatalf("correction escaped cost checkpoint: calls=%d result=%+v err=%v", correctionProvider.calls, correction, correctionErr)
	}
}
