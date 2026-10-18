package main

import (
	"context"
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestTopologyReviewedStageBudgetsFitFrozenArm(t *testing.T) {
	definition, err := agentqualification.CurrentTopologyExperiment()
	if err != nil {
		t.Fatal(err)
	}
	arm := topologyArmByID(definition, "reviewed-v1")
	if topologyReviewedExecuteTokens+topologyReviewedCorrectionTokens+2*definition.ReviewerTotalTokenReservation != arm.TotalTokenReservation {
		t.Fatal("reviewed token sub-budgets do not exactly fit the frozen arm")
	}
	if topologyReviewedExecuteGenerated+topologyReviewedCorrectionGenerated+2*definition.ReviewerGeneratedTokenCeiling != arm.GeneratedTokenCeiling {
		t.Fatal("reviewed generated-token sub-budgets do not exactly fit the frozen arm")
	}
}

func TestTopologyReviewRequestContainsInspectableControllerEvidence(t *testing.T) {
	workspace, err := agentqualification.PrepareRealisticCaseWorkspace(t.TempDir(), "R03")
	if err != nil {
		t.Fatal(err)
	}
	content := "historical journey check failed with exact local output"
	controller := agentqualification.RealisticControllerResult{TerminalState: "failed", FailureClass: "competence_failure", FinalSummary: "bounded executor result", ToolResults: []agentqualification.RealisticToolResult{{Output: content, OutputHash: sha256Hex([]byte(content))}}}
	observation := agentqualification.TopologyExperimentObservation{ObservationID: "obs", CaseID: "R03", Repeat: 1, ArmID: "reviewed-v1"}
	request, err := topologyReviewRequestFor(observation, workspace, controller, nil, t.TempDir(), "obs:review-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(request.EvidenceItems) != 1 || request.EvidenceItems[0].Content != content || request.EvidenceItems[0].SHA256 != sha256Hex([]byte(content)) {
		t.Fatalf("review request contains opaque rather than inspectable evidence: %+v", request.EvidenceItems)
	}
}

func TestTopologyTransportImperfectionInvalidatesOnlyObservation(t *testing.T) {
	hash := sha256Hex([]byte("hash source"))
	result := agentqualification.TopologyExperimentResult{InitialTreeSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash, AccountingStatus: "exact"}
	classified := classifyTopologyExecutionError(result, errors.New("temporary provider export failure"))
	if classified.Outcome != "invalid" || classified.AccountingStatus != "ambiguous" || classified.PostBatchCorrection == "" {
		t.Fatalf("non-safety imperfection was not isolated: %+v", classified)
	}
}

func TestTopologyStageBudgetExhaustionIsCandidateLocalAndExact(t *testing.T) {
	hash := sha256Hex([]byte("hash source"))
	result := agentqualification.TopologyExperimentResult{
		InitialTreeSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts:    []agentqualification.TopologyStageReceipt{{RunID: "run", Stage: "execute", ProviderCalls: 1, InputTokens: 8, OutputTokens: 3, CostMicros: 2}},
		AccountingStatus: "exact",
	}
	classified := classifyTopologyExecutionError(result, agentqualification.ErrTopologyStageBudgetExceeded)
	if classified.Outcome != "invalid" || classified.FailureClass != "stage_budget_exhaustion" || classified.AccountingStatus != "exact" || classified.ProviderCalls != 1 || classified.TotalTokens != 11 || classified.CostMicros != 2 {
		t.Fatalf("stage exhaustion was not isolated with exact accounting: %+v", classified)
	}
}

func TestTopologyRecoveryWithoutClaimIsNoOp(t *testing.T) {
	result, err := recoverTopologyObservation(context.Background(), t.TempDir())
	if err != nil || result["status"] != "nothing_to_recover" {
		t.Fatalf("empty campaign recovery changed state: result=%+v err=%v", result, err)
	}
}
