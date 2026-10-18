package main

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestR04EffortV2PlanIsProviderFreeAndReviewed(t *testing.T) {
	plan, err := r04EffortV2Plan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["provider_ready"] != true {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestR04EffortV2ReviewedFreezeIsComplete(t *testing.T) {
	if err := agentqualification.RequireCompleteR04EffortFreezeV2(); err != nil {
		t.Fatalf("reviewed freeze is incomplete: %v", err)
	}
}

func TestR04EffortV2CommandHasOneExecutorCallAndNoReachableReviewedArm(t *testing.T) {
	s := string(r04EffortV2CampaignExecutorSource)
	if strings.Count(s, "RunR04EffortCandidateV2(") != 1 {
		t.Fatal("executor call count drifted")
	}
	if !strings.Contains(s, "if false { // unreachable legacy v9 review path") {
		t.Fatal("legacy review path is not hard-disabled")
	}
}

func TestR04EffortV2DirectCrashAttributesAmbiguousSpendToExecute(t *testing.T) {
	receipts := []agentqualification.TopologyStageReceipt{{RunID: "claim:execute", Stage: "execute", ProviderCalls: 1, CostMicros: 19}}
	receipts = applyR04EffortV2AmbiguousCost(receipts, nil, "claim", 907)
	if len(receipts) != 1 || receipts[0].Stage != "execute" || receipts[0].ProviderCalls != 1 || receipts[0].CostMicros != 926 {
		t.Fatalf("ambiguous direct spend was not retained: %+v", receipts)
	}
	result := aggregateTopologyResult(agentqualification.TopologyExperimentResult{StageReceipts: receipts})
	if result.CostMicros != 926 {
		t.Fatalf("aggregation discarded ambiguous spend: %+v", result)
	}
}

func TestR04EffortV2DirectCrashCreatesExecuteReceiptWhenMissing(t *testing.T) {
	receipts := applyR04EffortV2AmbiguousCost(nil, nil, "claim", 907)
	if len(receipts) != 1 || receipts[0].RunID != "claim:execute" || receipts[0].Stage != "execute" || receipts[0].ProviderCalls != 1 || receipts[0].CostMicros != 907 {
		t.Fatalf("missing direct receipt was not conservatively reconstructed: %+v", receipts)
	}
}
