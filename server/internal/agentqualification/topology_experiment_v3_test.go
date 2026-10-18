package agentqualification

import (
	"context"
	"testing"
)

func TestTopologyExperimentV3PinsOnlyTwoFreshMatchedR03Pairs(t *testing.T) {
	definition, err := CurrentTopologyExperimentV3()
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.ExecutionOrder) != 4 || definition.ExecutionOrder[0].ArmID != "direct-v3" || definition.ExecutionOrder[2].ArmID != "reviewed-v3" {
		t.Fatalf("topology v3 is not counterbalanced: %+v", definition.ExecutionOrder)
	}
	for _, observation := range definition.ExecutionOrder {
		if observation.CaseID != "R03" || observation.ObservationID == "r03-p1-direct" || observation.ObservationID == "r03-p1-reviewed" || observation.ObservationID == "r03-p2-direct" || observation.ObservationID == "r03-p2-reviewed" {
			t.Fatalf("topology v3 reused historical evidence identity: %+v", observation)
		}
	}
	if definition.MaxTotalTokens != 410000 || definition.ReviewerTotalTokenReservation != 16000 || definition.MaxIncrementalSpendMicros != 1800000 || definition.TransportRetries != 0 || definition.ProviderFallbacks || definition.MaxCorrections != 1 {
		t.Fatalf("topology v3 budget or retry policy drifted: %+v", definition)
	}
}

func TestTopologyV3RunnerCannotCallProviderBeforeReview(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV3()
	provider := &topologyCapturingProvider{}
	_, err := RunTopologyCandidateV3(context.Background(), definition, RealisticCaseWorkspace{}, "run", []byte("task"), 1, 10, 5, provider, nil)
	if err == nil || len(provider.requests) != 0 {
		t.Fatalf("pending topology v3 reached provider: calls=%d err=%v", len(provider.requests), err)
	}
}

func TestTopologyExperimentV3UsesExactMatchedClaudeConfiguration(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV3()
	v2, _ := CurrentTopologyExperimentV2()
	if definition.Executor != v2.Executor || definition.Reviewer != v2.Reviewer || definition.Executor.Model != "openrouter/anthropic/claude-sonnet-5" || definition.Executor.Effort != "medium" || definition.Executor.ServingProvider != "anthropic" {
		t.Fatalf("topology v3 changed the tested configuration: %+v", definition)
	}
}

func TestTopologyV3ReviewedBudgetGeometryCoversEveryPermittedSubstage(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV3()
	arm, _ := topologyArm(definition, "reviewed-v3")
	const execute, correction int64 = 60000, 36000
	worstCase := execute + correction + 2*definition.ReviewerTotalTokenReservation
	if worstCase != 128000 || arm.TotalTokenReservation != 130000 || arm.TotalTokenReservation < worstCase {
		t.Fatalf("reviewed budget does not cover execute + correction + two reviews: worst=%d arm=%d", worstCase, arm.TotalTokenReservation)
	}
	if 2*75000+2*arm.TotalTokenReservation != definition.MaxTotalTokens {
		t.Fatal("campaign cap does not exactly cover all four reserved arms")
	}
}
