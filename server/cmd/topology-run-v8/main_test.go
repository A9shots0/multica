package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestTopologyV8PlanIsProviderFreeAndFrozenReviewed(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	plan, err := topologyV8Plan(root)
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["provider_ready"] != true {
		t.Fatalf("plan did not preserve provider-free reviewed activation: %#v", plan)
	}
}

func TestTopologyV8PendingReviewerCrashIsAmbiguousAndAccounted(t *testing.T) {
	root := t.TempDir()
	runtimeState := topologyV8ObservationRuntime{
		ClaimID:       "claim",
		Controller:    agentqualification.RealisticControllerResult{ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact"},
		StageReceipts: []agentqualification.TopologyStageReceipt{{RunID: "claim:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 17, OutputTokens: 3, CostMicros: 11}},
	}
	if err := persistTopologyV8PendingReview(root, &runtimeState, "claim:review-1", "review_1"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "run-state.json"))
	if err != nil || !strings.Contains(string(raw), `"pending_review"`) {
		t.Fatalf("pending marker was not durable: %s err=%v", raw, err)
	}
	var durable topologyV8ObservationRuntime
	if err := json.Unmarshal(raw, &durable); err != nil {
		t.Fatal(err)
	}
	receipts, exact := reconstructTopologyV8CrashReceipts(root, durable)
	if exact || len(receipts) != 2 || receipts[1].Stage != "review_1" || receipts[1].ProviderCalls != 1 {
		t.Fatalf("missing reviewer receipt was treated as exact/no-call: exact=%v receipts=%+v", exact, receipts)
	}
	receipts = applyTopologyV8AmbiguousReviewCost(receipts, durable.PendingReview, 907)
	result := aggregateTopologyResult(agentqualification.TopologyExperimentResult{AccountingStatus: "ambiguous", StageReceipts: receipts})
	if result.ProviderCalls != 2 || result.TotalTokens != 20 || result.CostMicros != 918 {
		t.Fatalf("ambiguous review accounting was overwritten: %+v", result)
	}
}

func TestTopologyV8CLIUsesSeparateResumeAwareExecuteAndCorrection(t *testing.T) {
	raw, err := os.ReadFile("topology_v8_cli.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Count(s, "RunTopologyCandidateV8(") != 1 || strings.Count(s, "RunTopologyCorrectionV8(") != 1 {
		t.Fatal("v8 execute/correction bindings drifted")
	}
	if !strings.Contains(s, "topologyReviewedExecuteTokens       int64 = 60000") || !strings.Contains(s, "topologyReviewedCorrectionTokens    int64 = 40000") {
		t.Fatal("v8 stage caps drifted")
	}
}

func TestTopologyV8CurrentScreeningMainBindingRejectsDrift(t *testing.T) {
	commandRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(commandRoot); err == nil || !strings.Contains(err.Error(), "screening") {
		t.Fatalf("altered current entrypoint reused historical command authority: %v", err)
	}
	if err := verifyTopologyV8CurrentScreeningMainAt(commandRoot); err == nil {
		t.Fatalf("unqualified current screening entrypoint was accepted: %v", err)
	}
	temporaryRoot := t.TempDir()
	temporaryScreening := filepath.Join(temporaryRoot, "screening-run")
	if err := os.MkdirAll(temporaryScreening, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(commandRoot, "screening-run", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temporaryScreening, "main.go"), append(raw, []byte("\n// drift")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyTopologyV8CurrentScreeningMainAt(temporaryRoot); err == nil || !strings.Contains(err.Error(), "current screening entrypoint drifted") {
		t.Fatalf("screening entrypoint drift was accepted: %v", err)
	}
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
