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

func TestTopologyV6ReviewerRunFailureAfterStepFinishSealsPaidAccounting(t *testing.T) {
	assertSealedTopologyReviewFailureAccounting(t, errors.New("reviewer CLI completion failed"), nil)
}

func TestTopologyV6ReviewerExportFailureAfterPaidCompletionSealsPaidAccounting(t *testing.T) {
	assertSealedTopologyReviewFailureAccounting(t, errors.New("reviewer export failed"), []byte("partial export"))
}

func TestTopologyV6ReviewerFailureWithoutTrustworthyUsageRemainsAmbiguous(t *testing.T) {
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

func TestTopologyV6PlanIsProviderFreeAndUsesFreshRoot(t *testing.T) {
	result, err := topologyV6Plan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	next, ok := result["next_observation"].(*agentqualification.TopologyExperimentObservation)
	if !ok || next == nil || next.ObservationID != "v6-r03-p1-direct" || result["provider_called"] != false || result["provider_ready"] != true {
		t.Fatalf("unexpected reviewed plan: %+v", result)
	}
}

func TestTopologyV6WrongObservationFailsBeforeClaimOrProvider(t *testing.T) {
	root := t.TempDir()
	_, err := executeTopologyV6Observation(context.Background(), root, "not-the-next-observation")
	if err == nil {
		t.Fatal("wrong topology v6 observation was admitted")
	}
	if _, statErr := os.Stat(filepath.Join(root, "campaign.json")); !os.IsNotExist(statErr) {
		t.Fatal("invalid topology v6 execution wrote a claim")
	}
}

func TestTopologyV6DuplicateExecutorResponseStopsBeforeReviewer(t *testing.T) {
	controller := agentqualification.RealisticControllerResult{
		TerminalState: "failed",
		FailureClass:  "response_or_tool_protocol_failure",
	}
	reviewerCalls := 0
	if !isTopologyV6ExecutorProtocolFailure(controller) {
		reviewerCalls++
	}
	if reviewerCalls != 0 {
		t.Fatal("duplicate or malformed executor response invoked a reviewer")
	}
}

func TestTopologyV6MalformedCorrectionStopsBeforeReviewTwo(t *testing.T) {
	correction := agentqualification.RealisticControllerResult{
		TerminalState: "failed",
		FailureClass:  "response_or_tool_protocol_failure",
	}
	reviewTwoCalls := 0
	if !isTopologyV6ExecutorProtocolFailure(correction) {
		reviewTwoCalls++
	}
	if reviewTwoCalls != 0 {
		t.Fatal("malformed correction invoked review two")
	}
}

func TestTopologyV6ProviderBudgetBreachHaltsCampaign(t *testing.T) {
	definition, _ := agentqualification.CurrentTopologyExperimentV6()
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
	state, err := agentqualification.RecordTopologyV6Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "uncontrolled_spending" {
		t.Fatalf("budget breach did not halt campaign: state=%+v err=%v", state, err)
	}
}

func TestTopologyV6CurrentCommandDriftCannotReuseHistoricalAdmission(t *testing.T) {
	if err := verifyPreservedTopologyCommandChain(); err == nil || !strings.Contains(err.Error(), "screening-run") {
		t.Fatalf("altered screening entrypoint must require fresh qualification: %v", err)
	}
}

func TestTopologyV6RejectsNestedTopologyV2CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v2", "topology_v2_cli.go")
	if err := os.WriteFile(path, []byte("nested v2 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v2") {
		t.Fatalf("nested topology-v2 command drift was accepted: %v", err)
	}
}

func TestTopologyV6RejectsTopologyV4CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v4", "topology_v4_cli.go")
	if err := os.WriteFile(path, []byte("topology v4 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v4") {
		t.Fatalf("topology-v4 command drift was accepted: %v", err)
	}
}

func TestTopologyV6RejectsTopologyV5CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v5", "topology_v5_cli.go")
	if err := os.WriteFile(path, []byte("topology v5 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v5") {
		t.Fatalf("topology-v5 command drift was accepted: %v", err)
	}
}

func TestTopologyV6RejectsNestedTopologyV1CommandDrift(t *testing.T) {
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
		t.Fatal("cannot locate topology-v6 test source")
	}
	sourceRoot := filepath.Join(filepath.Dir(current), "..")
	targetRoot := t.TempDir()
	files := []struct{ directory, name string }{
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
