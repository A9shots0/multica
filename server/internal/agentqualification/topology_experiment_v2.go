package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const TopologyExperimentV2SchemaVersion = "silver-mediums.topology-experiment.v2"

//go:embed topology_experiment_v2.json
var topologyExperimentV2JSON []byte

func CurrentTopologyExperimentV2() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(topologyExperimentV2JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode topology v2 experiment: %w", err)
	}
	if err := ValidateTopologyExperimentV2(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateTopologyExperimentV2(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != TopologyExperimentV2SchemaVersion || definition.ExperimentID != "realistic-topology-r03-repeat-v2" || definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v2 identity or status is invalid")
	}
	if len(definition.Cases) != 1 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R03", CaseVersion: "v2", Repeats: 2}) {
		return fmt.Errorf("topology v2 case set drifted")
	}
	wantArms := []TopologyExperimentArm{{ArmID: "direct-v2", MaxProviderTurns: 15, TotalTokenReservation: 75000, GeneratedTokenCeiling: 20000, CostCeilingMicros: 300000}, {ArmID: "reviewed-v2", Reviewed: true, MaxProviderTurns: 25, TotalTokenReservation: 120000, GeneratedTokenCeiling: 30000, CostCeilingMicros: 600000}}
	if len(definition.Arms) != len(wantArms) || definition.Arms[0] != wantArms[0] || definition.Arms[1] != wantArms[1] {
		return fmt.Errorf("topology v2 arms drifted")
	}
	wantOrder := []TopologyExperimentObservation{{"v2-r03-p1-direct", "R03", 1, "direct-v2"}, {"v2-r03-p1-reviewed", "R03", 1, "reviewed-v2"}, {"v2-r03-p2-reviewed", "R03", 2, "reviewed-v2"}, {"v2-r03-p2-direct", "R03", 2, "direct-v2"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("topology v2 observation count drifted")
	}
	for i := range wantOrder {
		if definition.ExecutionOrder[i] != wantOrder[i] {
			return fmt.Errorf("topology v2 counterbalance order drifted")
		}
	}
	v1, err := CurrentTopologyExperiment()
	if err != nil || definition.Executor != v1.Executor || definition.Reviewer != v1.Reviewer {
		return fmt.Errorf("topology v2 model configuration drifted")
	}
	if definition.ReviewerTotalTokenReservation != 12000 || definition.ReviewerGeneratedTokenCeiling != 4000 || definition.MaxCorrections != 1 || definition.MaxTotalTokens != 390000 || definition.MaxIncrementalSpendMicros != 1800000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("topology v2 budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "preserve_affected_observation_continue_unaffected_log_post_batch_correction" || definition.AdvancementPolicy != "testing_only_no_role_or_topology_selection" {
		return fmt.Errorf("topology v2 stop behavior drifted")
	}
	return nil
}

func TopologyExperimentV2ProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateTopologyExperimentV2(definition) == nil && definition.Status == "frozen_reviewed"
}
