package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestTopologyV5PlanIsProviderFreeAndUsesFreshRoot(t *testing.T) {
	result, err := topologyV5Plan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	next, ok := result["next_observation"].(*agentqualification.TopologyExperimentObservation)
	if !ok || next == nil || next.ObservationID != "v5-r03-p1-direct" || result["provider_called"] != false || result["provider_ready"] != true {
		t.Fatalf("unexpected reviewed plan: %+v", result)
	}
}

func TestTopologyV5WrongObservationFailsBeforeClaimOrProvider(t *testing.T) {
	root := t.TempDir()
	_, err := executeTopologyV5Observation(context.Background(), root, "not-the-next-observation")
	if err == nil {
		t.Fatal("wrong topology v5 observation was admitted")
	}
	if _, statErr := os.Stat(filepath.Join(root, "campaign.json")); !os.IsNotExist(statErr) {
		t.Fatal("invalid topology v5 execution wrote a claim")
	}
}

func TestTopologyV5DuplicateExecutorResponseStopsBeforeReviewer(t *testing.T) {
	controller := agentqualification.RealisticControllerResult{
		TerminalState: "failed",
		FailureClass:  "response_or_tool_protocol_failure",
	}
	reviewerCalls := 0
	if !isTopologyV5ExecutorProtocolFailure(controller) {
		reviewerCalls++
	}
	if reviewerCalls != 0 {
		t.Fatal("duplicate or malformed executor response invoked a reviewer")
	}
}

func TestTopologyV5MalformedCorrectionStopsBeforeReviewTwo(t *testing.T) {
	correction := agentqualification.RealisticControllerResult{
		TerminalState: "failed",
		FailureClass:  "response_or_tool_protocol_failure",
	}
	reviewTwoCalls := 0
	if !isTopologyV5ExecutorProtocolFailure(correction) {
		reviewTwoCalls++
	}
	if reviewTwoCalls != 0 {
		t.Fatal("malformed correction invoked review two")
	}
}

func TestTopologyV5ProviderBudgetBreachHaltsCampaign(t *testing.T) {
	definition, _ := agentqualification.CurrentTopologyExperimentV5()
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
	state, err := agentqualification.RecordTopologyV5Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "uncontrolled_spending" {
		t.Fatalf("budget breach did not halt campaign: state=%+v err=%v", state, err)
	}
}

func TestTopologyV5CurrentCommandDriftCannotReuseHistoricalAdmission(t *testing.T) {
	if err := verifyPreservedTopologyCommandChain(); err == nil || !strings.Contains(err.Error(), "screening-run") {
		t.Fatalf("altered screening entrypoint must require fresh qualification: %v", err)
	}
}

func TestTopologyV5RejectsNestedTopologyV2CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v2", "topology_v2_cli.go")
	if err := os.WriteFile(path, []byte("nested v2 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v2") {
		t.Fatalf("nested topology-v2 command drift was accepted: %v", err)
	}
}

func TestTopologyV5RejectsTopologyV4CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v4", "topology_v4_cli.go")
	if err := os.WriteFile(path, []byte("topology v4 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v4") {
		t.Fatalf("topology-v4 command drift was accepted: %v", err)
	}
}

func TestTopologyV5RejectsNestedTopologyV1CommandDrift(t *testing.T) {
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
		t.Fatal("cannot locate topology-v5 test source")
	}
	sourceRoot := filepath.Join(filepath.Dir(current), "..")
	targetRoot := t.TempDir()
	files := []struct{ directory, name string }{
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
