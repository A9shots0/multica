package agentqualification

import (
	"context"
	"testing"
)

func TestTopologyExperimentV5PinsFreshGPT54R03Replication(t *testing.T) {
	definition, err := CurrentTopologyExperimentV5()
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []TopologyExperimentObservation{{"v5-r03-p1-direct", "R03", 1, "direct-v5"}, {"v5-r03-p1-reviewed", "R03", 1, "reviewed-v5"}, {"v5-r03-p2-reviewed", "R03", 2, "reviewed-v5"}, {"v5-r03-p2-direct", "R03", 2, "direct-v5"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		t.Fatalf("unexpected observation count: %d", len(definition.ExecutionOrder))
	}
	for index := range wantOrder {
		if definition.ExecutionOrder[index] != wantOrder[index] {
			t.Fatalf("counterbalanced order drifted: %+v", definition.ExecutionOrder)
		}
	}
	if definition.Executor.ConfigurationID != "realistic.gpt54.medium.anchor.r04.v8" || definition.Executor.Model != "openrouter/openai/gpt-5.4" || definition.Executor.Effort != "medium" || definition.Executor.ServingProvider != "openai" {
		t.Fatalf("executor drifted: %+v", definition.Executor)
	}
	if definition.Reviewer.ConfigurationID != "realistic.gpt54.medium.reviewer.topology-v5" || definition.Reviewer.Model != definition.Executor.Model || definition.Reviewer.Effort != definition.Executor.Effort || definition.Reviewer.ServingProvider != definition.Executor.ServingProvider {
		t.Fatalf("reviewer is not a fresh matched identity: %+v", definition.Reviewer)
	}
}

func TestTopologyExperimentV5PinsV3GeometryAndStopPolicy(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV5()
	reviewed, _ := topologyArm(definition, "reviewed-v5")
	const execute, correction int64 = 60000, 36000
	worstReviewed := execute + correction + 2*definition.ReviewerTotalTokenReservation
	if worstReviewed != 128000 || reviewed.TotalTokenReservation != 130000 || reviewed.TotalTokenReservation < worstReviewed {
		t.Fatalf("reviewed geometry is invalid: worst=%d arm=%d", worstReviewed, reviewed.TotalTokenReservation)
	}
	if 2*75000+2*reviewed.TotalTokenReservation != definition.MaxTotalTokens || definition.MaxTotalTokens != 410000 || definition.MaxIncrementalSpendMicros != 1800000 {
		t.Fatalf("campaign geometry drifted: %+v", definition)
	}
	if definition.MaxCorrections != 1 || definition.TransportRetries != 0 || definition.ProviderFallbacks || definition.OwnerInterventions != 0 {
		t.Fatalf("bounded stop policy drifted: %+v", definition)
	}
}

func TestTopologyV5ProviderReadyStillRejectsInvalidRunBeforeCall(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV5()
	provider := &topologyCapturingProvider{}
	_, err := RunTopologyCandidateV5(context.Background(), definition, RealisticCaseWorkspace{}, "run", []byte("task"), 1, 10, 5, provider, nil)
	if err == nil || len(provider.requests) != 0 || !TopologyExperimentV5ProviderReady(definition) {
		t.Fatalf("reviewed topology v5 admission or invalid-run gate failed: calls=%d err=%v", len(provider.requests), err)
	}
}

func TestTopologyV5ReviewerBindingAcceptsNamedReviewerAndRejectsWrongReviewer(t *testing.T) {
	definition, err := CurrentTopologyExperimentV5()
	if err != nil {
		t.Fatal(err)
	}
	request := topologyReviewFixture(t)
	plan, err := BuildOpenCodeTopologyReviewPlanForFrozenReviewer(definition.Reviewer, definition.Reviewer, request)
	if err != nil || plan.CandidateID != definition.Reviewer.ConfigurationID || plan.Model != definition.Reviewer.Model {
		t.Fatalf("named topology-v5 reviewer was rejected: plan=%+v err=%v", plan, err)
	}
	wrong := definition.Reviewer
	wrong.ConfigurationID = "wrong-reviewer"
	if _, err := BuildOpenCodeTopologyReviewPlanForFrozenReviewer(definition.Reviewer, wrong, request); err == nil {
		t.Fatal("wrong topology-v5 reviewer was accepted")
	}
}
