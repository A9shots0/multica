package agentqualification

import (
	"context"
	"testing"
)

func TestTopologyExperimentV2PinsOnlyTwoFreshMatchedR03Pairs(t *testing.T) {
	definition, err := CurrentTopologyExperimentV2()
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.ExecutionOrder) != 4 || definition.ExecutionOrder[0].ArmID != "direct-v2" || definition.ExecutionOrder[2].ArmID != "reviewed-v2" {
		t.Fatalf("topology v2 is not counterbalanced: %+v", definition.ExecutionOrder)
	}
	for _, observation := range definition.ExecutionOrder {
		if observation.CaseID != "R03" || observation.ObservationID == "r03-p1-direct" || observation.ObservationID == "r03-p1-reviewed" || observation.ObservationID == "r03-p2-direct" || observation.ObservationID == "r03-p2-reviewed" {
			t.Fatalf("topology v2 reused historical evidence identity: %+v", observation)
		}
	}
	if definition.MaxTotalTokens != 390000 || definition.MaxIncrementalSpendMicros != 1800000 || definition.TransportRetries != 0 || definition.ProviderFallbacks || definition.MaxCorrections != 1 {
		t.Fatalf("topology v2 budget or retry policy drifted: %+v", definition)
	}
}

func TestTopologyV2RunnerCannotCallProviderBeforeReview(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV2()
	provider := &topologyCapturingProvider{}
	_, err := RunTopologyCandidateV2(context.Background(), definition, RealisticCaseWorkspace{}, "run", []byte("task"), 1, 10, 5, provider, nil)
	if err == nil || len(provider.requests) != 0 {
		t.Fatalf("pending topology v2 reached provider: calls=%d err=%v", len(provider.requests), err)
	}
}

func TestTopologyExperimentV2UsesExactMatchedClaudeConfiguration(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV2()
	v1, _ := CurrentTopologyExperiment()
	if definition.Executor != v1.Executor || definition.Reviewer != v1.Reviewer || definition.Executor.Model != "openrouter/anthropic/claude-sonnet-5" || definition.Executor.Effort != "medium" || definition.Executor.ServingProvider != "anthropic" {
		t.Fatalf("topology v2 changed the tested configuration: %+v", definition)
	}
}
