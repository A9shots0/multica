package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const TopologyExperimentV3SchemaVersion = "silver-mediums.topology-experiment.v3"

//go:embed topology_experiment_v3.json
var topologyExperimentV3JSON []byte

func CurrentTopologyExperimentV3() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(topologyExperimentV3JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode topology v3 experiment: %w", err)
	}
	if err := ValidateTopologyExperimentV3(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateTopologyExperimentV3(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != TopologyExperimentV3SchemaVersion || definition.ExperimentID != "realistic-topology-r03-repeat-v3" || definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v3 identity or status is invalid")
	}
	if len(definition.Cases) != 1 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R03", CaseVersion: "v2", Repeats: 2}) {
		return fmt.Errorf("topology v3 case set drifted")
	}
	wantArms := []TopologyExperimentArm{{ArmID: "direct-v3", MaxProviderTurns: 15, TotalTokenReservation: 75000, GeneratedTokenCeiling: 20000, CostCeilingMicros: 300000}, {ArmID: "reviewed-v3", Reviewed: true, MaxProviderTurns: 25, TotalTokenReservation: 130000, GeneratedTokenCeiling: 30000, CostCeilingMicros: 600000}}
	if len(definition.Arms) != len(wantArms) || definition.Arms[0] != wantArms[0] || definition.Arms[1] != wantArms[1] {
		return fmt.Errorf("topology v3 arms drifted")
	}
	wantOrder := []TopologyExperimentObservation{{"v3-r03-p1-direct", "R03", 1, "direct-v3"}, {"v3-r03-p1-reviewed", "R03", 1, "reviewed-v3"}, {"v3-r03-p2-reviewed", "R03", 2, "reviewed-v3"}, {"v3-r03-p2-direct", "R03", 2, "direct-v3"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("topology v3 observation count drifted")
	}
	for i := range wantOrder {
		if definition.ExecutionOrder[i] != wantOrder[i] {
			return fmt.Errorf("topology v3 counterbalance order drifted")
		}
	}
	v2, err := CurrentTopologyExperimentV2()
	if err != nil || definition.Executor != v2.Executor || definition.Reviewer != v2.Reviewer {
		return fmt.Errorf("topology v3 model configuration drifted")
	}
	if definition.ReviewerTotalTokenReservation != 16000 || definition.ReviewerGeneratedTokenCeiling != 4000 || definition.MaxCorrections != 1 || definition.MaxTotalTokens != 410000 || definition.MaxIncrementalSpendMicros != 1800000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("topology v3 budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "preserve_affected_observation_continue_unaffected_log_post_batch_correction" || definition.AdvancementPolicy != "testing_only_no_role_or_topology_selection" {
		return fmt.Errorf("topology v3 stop behavior drifted")
	}
	return nil
}

func TopologyExperimentV3ProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateTopologyExperimentV3(definition) == nil && definition.Status == "frozen_reviewed"
}
