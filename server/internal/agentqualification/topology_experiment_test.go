package agentqualification

import "testing"

func TestTopologyExperimentPinsCounterbalancedMatchedPairsAndBudget(t *testing.T) {
	definition, err := CurrentTopologyExperiment()
	if err != nil {
		t.Fatal(err)
	}
	if TopologyExperimentProviderReady(definition) != (definition.Status == "frozen_reviewed") {
		t.Fatal("topology provider readiness does not match the frozen review status")
	}
	if len(definition.ExecutionOrder) != 6 || definition.ExecutionOrder[0].ArmID != "direct-v1" || definition.ExecutionOrder[2].ArmID != "reviewed-v1" || definition.ExecutionOrder[4].ArmID != "reviewed-v1" {
		t.Fatalf("comparison order is not counterbalanced: %+v", definition.ExecutionOrder)
	}
	if definition.MaxTotalTokens != 585000 || definition.MaxIncrementalSpendMicros != 3000000 || definition.MaxCorrections != 1 {
		t.Fatalf("comparison budget or correction ceiling drifted: %+v", definition)
	}
}

func TestTopologyExperimentRejectsUnboundedOrUnfairVariants(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	tests := []struct {
		name   string
		mutate func(*TopologyExperimentDefinition)
	}{
		{"extra correction", func(value *TopologyExperimentDefinition) { value.MaxCorrections = 2 }},
		{"fallbacks", func(value *TopologyExperimentDefinition) { value.ProviderFallbacks = true }},
		{"stronger reviewer", func(value *TopologyExperimentDefinition) { value.Reviewer.Effort = "high" }},
		{"missing repeat", func(value *TopologyExperimentDefinition) { value.ExecutionOrder = value.ExecutionOrder[:5] }},
		{"uncapped spend", func(value *TopologyExperimentDefinition) { value.MaxIncrementalSpendMicros = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := definition
			candidate.ExecutionOrder = append([]TopologyExperimentObservation(nil), definition.ExecutionOrder...)
			test.mutate(&candidate)
			if ValidateTopologyExperiment(candidate) == nil {
				t.Fatal("invalid topology experiment was accepted")
			}
		})
	}
}
