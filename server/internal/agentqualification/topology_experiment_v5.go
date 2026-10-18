package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const TopologyExperimentV5SchemaVersion = "silver-mediums.topology-experiment.v5"

//go:embed topology_experiment_v5.json
var topologyExperimentV5JSON []byte

func CurrentTopologyExperimentV5() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(topologyExperimentV5JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode topology v5 experiment: %w", err)
	}
	if err := ValidateTopologyExperimentV5(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateTopologyExperimentV5(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != TopologyExperimentV5SchemaVersion || definition.ExperimentID != "realistic-topology-r03-gpt54-repeat-v5" || definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v5 identity or status is invalid")
	}
	if len(definition.Cases) != 1 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R03", CaseVersion: "v2", Repeats: 2}) {
		return fmt.Errorf("topology v5 case set drifted")
	}
	wantArms := []TopologyExperimentArm{{ArmID: "direct-v5", MaxProviderTurns: 15, TotalTokenReservation: 75000, GeneratedTokenCeiling: 20000, CostCeilingMicros: 300000}, {ArmID: "reviewed-v5", Reviewed: true, MaxProviderTurns: 25, TotalTokenReservation: 130000, GeneratedTokenCeiling: 30000, CostCeilingMicros: 600000}}
	if len(definition.Arms) != len(wantArms) || definition.Arms[0] != wantArms[0] || definition.Arms[1] != wantArms[1] {
		return fmt.Errorf("topology v5 arms drifted")
	}
	wantOrder := []TopologyExperimentObservation{{"v5-r03-p1-direct", "R03", 1, "direct-v5"}, {"v5-r03-p1-reviewed", "R03", 1, "reviewed-v5"}, {"v5-r03-p2-reviewed", "R03", 2, "reviewed-v5"}, {"v5-r03-p2-direct", "R03", 2, "direct-v5"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("topology v5 observation count drifted")
	}
	for i := range wantOrder {
		if definition.ExecutionOrder[i] != wantOrder[i] {
			return fmt.Errorf("topology v5 counterbalance order drifted")
		}
	}
	wantExecutor := RealisticCandidate{ConfigurationID: "realistic.gpt54.medium.anchor.r04.v8", Model: "openrouter/openai/gpt-5.4", Effort: "medium", ServingProvider: "openai"}
	wantReviewer := RealisticCandidate{ConfigurationID: "realistic.gpt54.medium.reviewer.topology-v5", Model: wantExecutor.Model, Effort: wantExecutor.Effort, ServingProvider: wantExecutor.ServingProvider}
	if definition.Executor != wantExecutor || definition.Reviewer != wantReviewer || definition.Executor.ConfigurationID == definition.Reviewer.ConfigurationID {
		return fmt.Errorf("topology v5 model configuration drifted")
	}
	if definition.ReviewerTotalTokenReservation != 16000 || definition.ReviewerGeneratedTokenCeiling != 4000 || definition.MaxCorrections != 1 || definition.MaxTotalTokens != 410000 || definition.MaxIncrementalSpendMicros != 1800000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("topology v5 budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "preserve_affected_observation_continue_unaffected_log_post_batch_correction" || definition.AdvancementPolicy != "testing_only_no_role_or_topology_selection" {
		return fmt.Errorf("topology v5 stop behavior drifted")
	}
	return nil
}

func TopologyExperimentV5ProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateTopologyExperimentV5(definition) == nil && definition.Status == "frozen_reviewed"
}
