package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const TopologyExperimentV8SchemaVersion = "silver-mediums.topology-experiment.v8"

//go:embed topology_experiment_v8.json
var topologyExperimentV8JSON []byte

func CurrentTopologyExperimentV8() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(topologyExperimentV8JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode topology v8 experiment: %w", err)
	}
	if err := ValidateTopologyExperimentV8(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateTopologyExperimentV8(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != TopologyExperimentV8SchemaVersion || definition.ExperimentID != "realistic-topology-r04-gpt54-claude-v8" || definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v8 identity or status is invalid")
	}
	if len(definition.Cases) != 1 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R04", CaseVersion: "v1", Repeats: 2}) {
		return fmt.Errorf("topology v8 case set drifted")
	}
	wantArms := []TopologyExperimentArm{{ArmID: "direct-v8", MaxProviderTurns: 15, TotalTokenReservation: 60000, GeneratedTokenCeiling: 12000, CostCeilingMicros: 250000}, {ArmID: "reviewed-v8", Reviewed: true, MaxProviderTurns: 25, TotalTokenReservation: 140000, GeneratedTokenCeiling: 32000, CostCeilingMicros: 600000}}
	if len(definition.Arms) != 2 || definition.Arms[0] != wantArms[0] || definition.Arms[1] != wantArms[1] {
		return fmt.Errorf("topology v8 arms drifted")
	}
	wantOrder := []TopologyExperimentObservation{{"v8-r04-p1-direct", "R04", 1, "direct-v8"}, {"v8-r04-p1-reviewed", "R04", 1, "reviewed-v8"}, {"v8-r04-p2-reviewed", "R04", 2, "reviewed-v8"}, {"v8-r04-p2-direct", "R04", 2, "direct-v8"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("topology v8 observation count drifted")
	}
	for i := range wantOrder {
		if definition.ExecutionOrder[i] != wantOrder[i] {
			return fmt.Errorf("topology v8 counterbalance order drifted")
		}
	}
	wantExecutor := RealisticCandidate{ConfigurationID: "realistic.gpt54.medium.executor.topology-v8", Model: "openrouter/openai/gpt-5.4", Effort: "medium", ServingProvider: "openai"}
	wantReviewer := RealisticCandidate{ConfigurationID: "realistic.claude-sonnet5.medium.reviewer.topology-v8", Model: "openrouter/anthropic/claude-sonnet-5", Effort: "medium", ServingProvider: "anthropic"}
	if definition.Executor != wantExecutor || definition.Reviewer != wantReviewer || definition.Executor.ConfigurationID == definition.Reviewer.ConfigurationID {
		return fmt.Errorf("topology v8 model configuration drifted")
	}
	if definition.ReviewerTotalTokenReservation != 20000 || definition.ReviewerGeneratedTokenCeiling != 5000 || definition.MaxCorrections != 1 || definition.MaxTotalTokens != 400000 || definition.MaxIncrementalSpendMicros != 1700000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("topology v8 budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "budget_breach", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "stage_exhaustion_or_nonsafety_harness_failure_preserves_affected_observation_and_continues_unaffected_observations" || definition.AdvancementPolicy != "testing_only_no_role_or_topology_selection" {
		return fmt.Errorf("topology v8 stop behavior drifted")
	}
	return nil
}

func TopologyExperimentV8ProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateTopologyExperimentV8(definition) == nil && definition.Status == "frozen_reviewed"
}
