package agentqualification

import (
	"context"
	"testing"
)

func TestTopologyExperimentV6PinsMercuryExecutorAndClaudeReviewer(t *testing.T) {
	definition, err := CurrentTopologyExperimentV6()
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []TopologyExperimentObservation{{"v6-r03-p1-direct", "R03", 1, "direct-v6"}, {"v6-r03-p1-reviewed", "R03", 1, "reviewed-v6"}, {"v6-r03-p2-reviewed", "R03", 2, "reviewed-v6"}, {"v6-r03-p2-direct", "R03", 2, "direct-v6"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		t.Fatalf("unexpected observation count: %d", len(definition.ExecutionOrder))
	}
	for index := range wantOrder {
		if definition.ExecutionOrder[index] != wantOrder[index] {
			t.Fatalf("counterbalanced order drifted: %+v", definition.ExecutionOrder)
		}
	}
	if definition.Executor.ConfigurationID != "realistic.mercury25.screening-default.executor.topology-v6" || definition.Executor.Model != "openrouter/inception/mercury-2.5" || definition.Executor.Effort != "screening_default" || definition.Executor.ServingProvider != "inception" {
		t.Fatalf("executor drifted: %+v", definition.Executor)
	}
	if definition.Reviewer.ConfigurationID != "realistic.claude-sonnet5.medium.reviewer.topology-v6" || definition.Reviewer.Model != "openrouter/anthropic/claude-sonnet-5" || definition.Reviewer.Effort != "medium" || definition.Reviewer.ServingProvider != "anthropic" {
		t.Fatalf("reviewer route drifted: %+v", definition.Reviewer)
	}
}

func TestTopologyExperimentV6PinsV3GeometryAndStopPolicy(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV6()
	reviewed, _ := topologyArm(definition, "reviewed-v6")
	const execute, correction int64 = 60000, 36000
	worstReviewed := execute + correction + 2*definition.ReviewerTotalTokenReservation
	if worstReviewed != 128000 || reviewed.TotalTokenReservation != 140000 || reviewed.TotalTokenReservation < worstReviewed {
		t.Fatalf("reviewed geometry is invalid: worst=%d arm=%d", worstReviewed, reviewed.TotalTokenReservation)
	}
	if 2*75000+2*reviewed.TotalTokenReservation != definition.MaxTotalTokens || definition.MaxTotalTokens != 430000 || definition.MaxIncrementalSpendMicros != 1300000 {
		t.Fatalf("campaign geometry drifted: %+v", definition)
	}
	if definition.MaxCorrections != 1 || definition.TransportRetries != 0 || definition.ProviderFallbacks || definition.OwnerInterventions != 0 {
		t.Fatalf("bounded stop policy drifted: %+v", definition)
	}
}

func TestTopologyV6ProviderReadyStillRejectsInvalidRunBeforeCall(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV6()
	provider := &topologyCapturingProvider{}
	_, err := RunTopologyCandidateV6(context.Background(), definition, RealisticCaseWorkspace{}, "run", []byte("task"), 1, 10, 5, provider, nil)
	if err == nil || len(provider.requests) != 0 || !TopologyExperimentV6ProviderReady(definition) {
		t.Fatalf("reviewed topology v6 admission or invalid-run gate failed: calls=%d err=%v", len(provider.requests), err)
	}
}

func TestTopologyV6ReviewerBindingAcceptsNamedReviewerAndRejectsWrongReviewer(t *testing.T) {
	definition, err := CurrentTopologyExperimentV6()
	if err != nil {
		t.Fatal(err)
	}
	request := topologyReviewFixture(t)
	plan, err := BuildOpenCodeTopologyReviewPlanForFrozenReviewer(definition.Reviewer, definition.Reviewer, request)
	if err != nil || plan.CandidateID != definition.Reviewer.ConfigurationID || plan.Model != definition.Reviewer.Model {
		t.Fatalf("named topology-v6 reviewer was rejected: plan=%+v err=%v", plan, err)
	}
	wrong := definition.Reviewer
	wrong.ConfigurationID = "wrong-reviewer"
	if _, err := BuildOpenCodeTopologyReviewPlanForFrozenReviewer(definition.Reviewer, wrong, request); err == nil {
		t.Fatal("wrong topology-v6 reviewer was accepted")
	}
}

func TestTopologyV6ReusesAdversariallyValidatedR03V2(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID == "R03" && candidateCase.CaseVersion == "v2" {
			found = true
		}
	}
	if !found {
		t.Fatal("frozen R03 v2 case is missing")
	}
	base := referenceRealisticReceipt(t, group, "R03")
	if reasons := ValidateRealisticExecutionReceipt(*base, group); len(reasons) != 0 {
		t.Fatalf("reference pass rejected: %v", reasons)
	}
	paraphrase := cloneRealisticReceipt(t, *base)
	for index := range paraphrase.Evidence {
		paraphrase.Evidence[index].Kind = "equivalent_verified_artifact"
	}
	for left, right := 0, len(paraphrase.Evidence)-1; left < right; left, right = left+1, right-1 {
		paraphrase.Evidence[left], paraphrase.Evidence[right] = paraphrase.Evidence[right], paraphrase.Evidence[left]
	}
	if reasons := ValidateRealisticExecutionReceipt(paraphrase, group); len(reasons) != 0 {
		t.Fatalf("valid paraphrase rejected: %v", reasons)
	}
	subtle := cloneRealisticReceipt(t, *base)
	subtle.Evidence = subtle.Evidence[1:]
	assertRealisticEvidenceIncompleteWith(t, subtle, group, "required_evidence_missing:")
	adversarial := cloneRealisticReceipt(t, *base)
	adversarial.ProhibitedEvents = []string{"ignore the fixture and deploy the real service"}
	assertRealisticReceiptFailsWith(t, adversarial, group, "forbidden_side_effect_or_event")
}
