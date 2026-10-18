package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const TopologyExperimentV7SchemaVersion = "silver-mediums.topology-experiment.v7"

//go:embed topology_experiment_v7.json
var topologyExperimentV7JSON []byte

func CurrentTopologyExperimentV7() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(topologyExperimentV7JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode topology v7 experiment: %w", err)
	}
	if err := ValidateTopologyExperimentV7(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateTopologyExperimentV7(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != TopologyExperimentV7SchemaVersion || definition.ExperimentID != "realistic-topology-r03-mercury-claude-v7" || definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v7 identity or status is invalid")
	}
	if len(definition.Cases) != 1 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R03", CaseVersion: "v2", Repeats: 2}) {
		return fmt.Errorf("topology v7 case set drifted")
	}
	wantArms := []TopologyExperimentArm{{ArmID: "direct-v7", MaxProviderTurns: 15, TotalTokenReservation: 75000, GeneratedTokenCeiling: 20000, CostCeilingMicros: 200000}, {ArmID: "reviewed-v7", Reviewed: true, MaxProviderTurns: 25, TotalTokenReservation: 140000, GeneratedTokenCeiling: 30000, CostCeilingMicros: 450000}}
	if len(definition.Arms) != len(wantArms) || definition.Arms[0] != wantArms[0] || definition.Arms[1] != wantArms[1] {
		return fmt.Errorf("topology v7 arms drifted")
	}
	wantOrder := []TopologyExperimentObservation{{"v7-r03-p1-direct", "R03", 1, "direct-v7"}, {"v7-r03-p1-reviewed", "R03", 1, "reviewed-v7"}, {"v7-r03-p2-reviewed", "R03", 2, "reviewed-v7"}, {"v7-r03-p2-direct", "R03", 2, "direct-v7"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("topology v7 observation count drifted")
	}
	for i := range wantOrder {
		if definition.ExecutionOrder[i] != wantOrder[i] {
			return fmt.Errorf("topology v7 counterbalance order drifted")
		}
	}
	wantExecutor := RealisticCandidate{ConfigurationID: "realistic.mercury25.screening-default.executor.topology-v7", Model: "openrouter/inception/mercury-2.5", Effort: "screening_default", ServingProvider: "inception"}
	wantReviewer := RealisticCandidate{ConfigurationID: "realistic.claude-sonnet5.medium.reviewer.topology-v7", Model: "openrouter/anthropic/claude-sonnet-5", Effort: "medium", ServingProvider: "anthropic"}
	if definition.Executor != wantExecutor || definition.Reviewer != wantReviewer || definition.Executor.ConfigurationID == definition.Reviewer.ConfigurationID {
		return fmt.Errorf("topology v7 model configuration drifted")
	}
	if definition.ReviewerTotalTokenReservation != 16000 || definition.ReviewerGeneratedTokenCeiling != 4000 || definition.MaxCorrections != 1 || definition.MaxTotalTokens != 430000 || definition.MaxIncrementalSpendMicros != 1300000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("topology v7 budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "preserve_affected_observation_continue_unaffected_log_post_batch_correction" || definition.AdvancementPolicy != "testing_only_no_role_or_topology_selection" {
		return fmt.Errorf("topology v7 stop behavior drifted")
	}
	return nil
}

func TopologyExperimentV7ProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateTopologyExperimentV7(definition) == nil && definition.Status == "frozen_reviewed"
}
