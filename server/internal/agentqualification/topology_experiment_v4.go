package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const TopologyExperimentV4SchemaVersion = "silver-mediums.topology-experiment.v4"

//go:embed topology_experiment_v4.json
var topologyExperimentV4JSON []byte

func CurrentTopologyExperimentV4() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(topologyExperimentV4JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode topology v4 experiment: %w", err)
	}
	if err := ValidateTopologyExperimentV4(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateTopologyExperimentV4(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != TopologyExperimentV4SchemaVersion || definition.ExperimentID != "realistic-topology-r03-gpt54-repeat-v4" || definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v4 identity or status is invalid")
	}
	if len(definition.Cases) != 1 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R03", CaseVersion: "v2", Repeats: 2}) {
		return fmt.Errorf("topology v4 case set drifted")
	}
	wantArms := []TopologyExperimentArm{{ArmID: "direct-v4", MaxProviderTurns: 15, TotalTokenReservation: 75000, GeneratedTokenCeiling: 20000, CostCeilingMicros: 300000}, {ArmID: "reviewed-v4", Reviewed: true, MaxProviderTurns: 25, TotalTokenReservation: 130000, GeneratedTokenCeiling: 30000, CostCeilingMicros: 600000}}
	if len(definition.Arms) != len(wantArms) || definition.Arms[0] != wantArms[0] || definition.Arms[1] != wantArms[1] {
		return fmt.Errorf("topology v4 arms drifted")
	}
	wantOrder := []TopologyExperimentObservation{{"v4-r03-p1-direct", "R03", 1, "direct-v4"}, {"v4-r03-p1-reviewed", "R03", 1, "reviewed-v4"}, {"v4-r03-p2-reviewed", "R03", 2, "reviewed-v4"}, {"v4-r03-p2-direct", "R03", 2, "direct-v4"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("topology v4 observation count drifted")
	}
	for i := range wantOrder {
		if definition.ExecutionOrder[i] != wantOrder[i] {
			return fmt.Errorf("topology v4 counterbalance order drifted")
		}
	}
	wantExecutor := RealisticCandidate{ConfigurationID: "realistic.gpt54.medium.anchor.r04.v8", Model: "openrouter/openai/gpt-5.4", Effort: "medium", ServingProvider: "openai"}
	wantReviewer := RealisticCandidate{ConfigurationID: "realistic.gpt54.medium.reviewer.topology-v4", Model: wantExecutor.Model, Effort: wantExecutor.Effort, ServingProvider: wantExecutor.ServingProvider}
	if definition.Executor != wantExecutor || definition.Reviewer != wantReviewer || definition.Executor.ConfigurationID == definition.Reviewer.ConfigurationID {
		return fmt.Errorf("topology v4 model configuration drifted")
	}
	if definition.ReviewerTotalTokenReservation != 16000 || definition.ReviewerGeneratedTokenCeiling != 4000 || definition.MaxCorrections != 1 || definition.MaxTotalTokens != 410000 || definition.MaxIncrementalSpendMicros != 1800000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("topology v4 budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "preserve_affected_observation_continue_unaffected_log_post_batch_correction" || definition.AdvancementPolicy != "testing_only_no_role_or_topology_selection" {
		return fmt.Errorf("topology v4 stop behavior drifted")
	}
	return nil
}

func TopologyExperimentV4ProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateTopologyExperimentV4(definition) == nil && definition.Status == "frozen_reviewed"
}
