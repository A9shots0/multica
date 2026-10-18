package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func topologyPaidReviewFinishFixture() []byte {
	return []byte("{\"type\":\"step_finish\",\"part\":{\"cost\":0.0019,\"tokens\":{\"input\":12,\"output\":4,\"reasoning\":2,\"cache\":{\"read\":3,\"write\":1}}}}\n")
}

func assertSealedTopologyReviewFailureAccounting(t *testing.T, transportErr error, rawExport []byte) {
	t.Helper()
	root := t.TempDir()
	stage := agentqualification.TopologyStageReceipt{RunID: "review-failure", Stage: "review_1"}
	sealed, trustworthy, err := sealTopologyReviewTransportFailure(root, realisticOpenCodeRoutingEvidence{}, topologyPaidReviewFinishFixture(), rawExport, stage, time.Now(), transportErr)
	if err != nil {
		t.Fatal(err)
	}
	if !trustworthy || sealed.ProviderCalls != 1 || sealed.InputTokens != 16 || sealed.OutputTokens != 4 || sealed.ReasoningTokens != 2 || sealed.CostMicros != 1900 {
		t.Fatalf("paid reviewer accounting was not preserved: trustworthy=%t stage=%+v", trustworthy, sealed)
	}
	if _, err := os.Stat(filepath.Join(root, "provider-receipt.json")); err != nil {
		t.Fatalf("partial paid receipt was not sealed: %v", err)
	}
	classified := classifyTopologyExecutionError(agentqualification.TopologyExperimentResult{StageReceipts: []agentqualification.TopologyStageReceipt{sealed}}, topologyReviewerTransportError{cause: transportErr, accountingTrustworthy: trustworthy})
	if classified.AccountingStatus != "exact" || classified.ProviderCalls != 1 || classified.TotalTokens != 22 || classified.CostMicros != 1900 {
		t.Fatalf("trustworthy paid failure was marked ambiguous: %+v", classified)
	}
}

func TestTopologyV7ReviewerRunFailureAfterStepFinishSealsPaidAccounting(t *testing.T) {
	assertSealedTopologyReviewFailureAccounting(t, errors.New("reviewer CLI completion failed"), nil)
}

func TestTopologyV7ReviewerExportFailureAfterPaidCompletionSealsPaidAccounting(t *testing.T) {
	assertSealedTopologyReviewFailureAccounting(t, errors.New("reviewer export failed"), []byte("partial export"))
}

func TestTopologyV7ReviewerFailureWithoutTrustworthyUsageRemainsAmbiguous(t *testing.T) {
	root := t.TempDir()
	stage := agentqualification.TopologyStageReceipt{RunID: "review-ambiguous", Stage: "review_1"}
	sealed, trustworthy, err := sealTopologyReviewTransportFailure(root, realisticOpenCodeRoutingEvidence{}, []byte("{\"type\":\"step_start\"}\n"), nil, stage, time.Now(), errors.New("reviewer run failed"))
	if err != nil {
		t.Fatal(err)
	}
	classified := classifyTopologyExecutionError(agentqualification.TopologyExperimentResult{StageReceipts: []agentqualification.TopologyStageReceipt{sealed}}, topologyReviewerTransportError{cause: errors.New("reviewer run failed"), accountingTrustworthy: trustworthy})
	if trustworthy || classified.AccountingStatus != "ambiguous" || classified.ProviderCalls != 1 {
		t.Fatalf("untrusted paid failure was not preserved as ambiguous: %+v", classified)
	}
}

func TestTopologyV7CrashRecoveryReconstructsDurablePaidStageWithoutRetry(t *testing.T) {
	runtimeState := topologyV7ObservationRuntime{
		ClaimID: "claim-crash",
		Controller: agentqualification.RealisticControllerResult{
			ProviderCalls: 1, InputTokens: 20, OutputTokens: 4, ReasoningTokens: 2, CostMicros: 900,
		},
	}
	receipts, exact := reconstructTopologyV7CrashReceipts(t.TempDir(), runtimeState)
	if !exact || len(receipts) != 1 || receipts[0].Stage != "execute" || receipts[0].ProviderCalls != 1 || receipts[0].InputTokens != 20 || receipts[0].OutputTokens != 4 || receipts[0].ReasoningTokens != 2 || receipts[0].CostMicros != 900 {
		t.Fatalf("durable paid stage was not reconstructed exactly: exact=%t receipts=%+v", exact, receipts)
	}
}

func TestTopologyV7PlanIsProviderFreeAndUsesFreshRoot(t *testing.T) {
	result, err := topologyV7Plan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	next, ok := result["next_observation"].(*agentqualification.TopologyExperimentObservation)
	proof, proofOK := result["route_prevalidation"].(agentqualification.TopologyV7Prevalidation)
	if !ok || next == nil || next.ObservationID != "v7-r03-p1-direct" || result["provider_called"] != false || result["provider_ready"] != true || !proofOK || proof.ProviderCalls != 0 {
		t.Fatalf("unexpected reviewed provider-free plan: %+v", result)
	}
}

func TestTopologyV7WrongObservationFailsBeforeClaimOrProvider(t *testing.T) {
	root := t.TempDir()
	_, err := executeTopologyV7Observation(context.Background(), root, "not-the-next-observation")
	if err == nil {
		t.Fatal("wrong topology v7 observation was admitted")
	}
	if _, statErr := os.Stat(filepath.Join(root, "campaign.json")); !os.IsNotExist(statErr) {
		t.Fatal("invalid topology v7 execution wrote a claim")
	}
}

func TestTopologyV7PreclaimFailureCreatesNoClaimOrProviderCall(t *testing.T) {
	definition, _ := agentqualification.CurrentTopologyExperimentV7()
	state := agentqualification.TopologyCampaignState{}
	providerCalls := 0
	next, _ := agentqualification.NextTopologyV7Observation(state, definition)
	got, err := prevalidateAndClaimTopologyV7Observation(state, definition, next.ObservationID, "claim", func(agentqualification.TopologyExperimentDefinition) (agentqualification.TopologyV7Prevalidation, error) {
		return agentqualification.TopologyV7Prevalidation{ProviderCalls: providerCalls}, errors.New("route drift")
	})
	if err == nil || got.ActiveClaim != "" || got.Revision != 0 || providerCalls != 0 {
		t.Fatalf("failed prevalidation mutated state or called provider: state=%+v calls=%d err=%v", got, providerCalls, err)
	}
}

func TestTopologyV7DuplicateExecutorResponseStopsBeforeReviewer(t *testing.T) {
	controller := agentqualification.RealisticControllerResult{
		TerminalState: "failed",
		FailureClass:  "response_or_tool_protocol_failure",
	}
	reviewerCalls := 0
	if !isTopologyV7ExecutorProtocolFailure(controller) {
		reviewerCalls++
	}
	if reviewerCalls != 0 {
		t.Fatal("duplicate or malformed executor response invoked a reviewer")
	}
}

func TestTopologyV7MalformedCorrectionStopsBeforeReviewTwo(t *testing.T) {
	correction := agentqualification.RealisticControllerResult{
		TerminalState: "failed",
		FailureClass:  "response_or_tool_protocol_failure",
	}
	reviewTwoCalls := 0
	if !isTopologyV7ExecutorProtocolFailure(correction) {
		reviewTwoCalls++
	}
	if reviewTwoCalls != 0 {
		t.Fatal("malformed correction invoked review two")
	}
}

func TestTopologyV7ProviderBudgetBreachHaltsCampaign(t *testing.T) {
	definition, _ := agentqualification.CurrentTopologyExperimentV7()
	observation := definition.ExecutionOrder[0]
	hash := strings.Repeat("d", 64)
	result := agentqualification.TopologyExperimentResult{
		ClaimID: "claim-budget", ObservationID: observation.ObservationID, CaseID: observation.CaseID,
		Repeat: observation.Repeat, ArmID: observation.ArmID, InitialTreeSHA256: hash, TaskSHA256: hash,
		FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash, AccountingStatus: "exact",
		StageReceipts: []agentqualification.TopologyStageReceipt{{RunID: "claim-budget:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 10, OutputTokens: 2, CostMicros: 3}},
	}
	result = classifyTopologyExecutionError(result, agentqualification.ErrRealisticBudgetBreach)
	if result.Outcome != "global_halt" || result.FailureClass != "uncontrolled_spending" {
		t.Fatalf("budget breach was misclassified: %+v", result)
	}
	state := agentqualification.TopologyCampaignState{ActiveClaim: "claim-budget"}
	state, err := agentqualification.RecordTopologyV7Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "uncontrolled_spending" {
		t.Fatalf("budget breach did not halt campaign: state=%+v err=%v", state, err)
	}
}

func TestTopologyV7ExecuteStageExhaustionIsLocalAndStopsBeforeReviewer(t *testing.T) {
	hash := strings.Repeat("b", 64)
	result := agentqualification.TopologyExperimentResult{
		ClaimID: "claim-stage", ObservationID: "v7-r03-p1-reviewed", CaseID: "R03", Repeat: 1, ArmID: "reviewed-v7",
		InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts:    []agentqualification.TopologyStageReceipt{{RunID: "claim-stage:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 60001, OutputTokens: 1, CostMicros: 10}},
		AccountingStatus: "exact",
	}
	classified := classifyTopologyExecutionError(result, agentqualification.ErrTopologyStageBudgetExceeded)
	if classified.Outcome != "invalid" || classified.FailureClass != "stage_budget_exhaustion" || classified.ProviderCalls != 1 || classified.TotalTokens != 60002 || classified.CostMicros != 10 || len(classified.StageReceipts) != 1 || classified.StageReceipts[0].Stage != "execute" {
		t.Fatalf("execute exhaustion was not local or admitted a reviewer: %+v", classified)
	}
}

func TestTopologyV7CurrentCommandDriftCannotReuseHistoricalAdmission(t *testing.T) {
	if err := verifyPreservedTopologyCommandChain(); err == nil || !strings.Contains(err.Error(), "screening-run") {
		t.Fatalf("altered screening entrypoint must require fresh qualification: %v", err)
	}
}

func TestTopologyV7RejectsNestedTopologyV2CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v2", "topology_v2_cli.go")
	if err := os.WriteFile(path, []byte("nested v2 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v2") {
		t.Fatalf("nested topology-v2 command drift was accepted: %v", err)
	}
}

func TestTopologyV7RejectsTopologyV4CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v4", "topology_v4_cli.go")
	if err := os.WriteFile(path, []byte("topology v4 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v4") {
		t.Fatalf("topology-v4 command drift was accepted: %v", err)
	}
}

func TestTopologyV7RejectsTopologyV5CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v5", "topology_v5_cli.go")
	if err := os.WriteFile(path, []byte("topology v5 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v5") {
		t.Fatalf("topology-v5 command drift was accepted: %v", err)
	}
}

func TestTopologyV7RejectsTopologyV6CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v6", "topology_v6_cli.go")
	if err := os.WriteFile(path, []byte("topology v6 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v6") {
		t.Fatalf("topology-v6 command drift was accepted: %v", err)
	}
}

func TestTopologyV7ExecutionUsesGenerationBoundRunnerForExecuteAndCorrection(t *testing.T) {
	raw, err := os.ReadFile("topology_v7_cli.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Count(source, "agentqualification.RunTopologyCandidateV7(") != 2 || strings.Contains(source, "agentqualification.RunTopologyCandidateV6(") || strings.Contains(source, "agentqualification.RunRealisticCandidate(") {
		t.Fatal("execute and correction are not both bound to the v7 runner")
	}
}

func TestTopologyV7RejectsNestedTopologyV1CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "screening-run", "main.go")
	if err := os.WriteFile(path, []byte("nested v1 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v1") {
		t.Fatalf("nested topology-v1 command drift was accepted: %v", err)
	}
}

func copyTopologyCommandChain(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate topology-v7 test source")
	}
	sourceRoot := filepath.Join(filepath.Dir(current), "..")
	targetRoot := t.TempDir()
	files := []struct{ directory, name string }{
		{"topology-run-v6", "topology_v6_cli.go"},
		{"topology-run-v6", "main.go"},
		{"topology-run-v5", "topology_v5_cli.go"},
		{"topology-run-v5", "main.go"},
		{"topology-run-v4", "topology_v4_cli.go"},
		{"topology-run-v4", "main.go"},
		{"topology-run-v3", "topology_v3_cli.go"},
		{"topology-run-v3", "main.go"},
		{"topology-run-v2", "topology_v2_cli.go"},
		{"topology-run-v2", "main.go"},
		{"screening-run", "topology_cli.go"},
		{"screening-run", "main.go"},
	}
	for _, file := range files {
		source := filepath.Join(sourceRoot, file.directory, file.name)
		targetDir := filepath.Join(targetRoot, file.directory)
		if err := os.MkdirAll(targetDir, 0o700); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, file.name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return targetRoot
}

// Positive verification uses explicit synthetic byte custody. Unrecovered old
// entrypoint bytes cannot be asserted equivalent to the evolving current main.
func TestExactNestedSourceVerifierAcceptsOnlyCompleteUnchangedBytes(t *testing.T) {
	root := t.TempDir()
	executor := []byte("synthetic frozen executor\n")
	main := []byte("synthetic frozen main\n")
	for _, name := range []string{"screening-run", "topology-run-v2", "topology-run-v3", "topology-run-v4", "topology-run-v5", "topology-run-v6", "topology-run-v7"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for file, raw := range map[string][]byte{"executor.go": executor, "main.go": main} {
			if err := os.WriteFile(filepath.Join(dir, file), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		verify := func() error {
			return verifyTopologyCommandSourcesAt(dir, "executor.go", sha256Hex(executor), sha256Hex(main))
		}
		if err := verify(); err != nil {
			t.Fatal(err)
		}
		for file, raw := range map[string][]byte{"executor.go": executor, "main.go": main} {
			path := filepath.Join(dir, file)
			if err := os.WriteFile(path, []byte("drift"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := verify(); err == nil {
				t.Fatalf("%s/%s mutation accepted", name, file)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := verify(); err == nil {
				t.Fatalf("missing %s/%s accepted", name, file)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := verify(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
