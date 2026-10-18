package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const R04EffortExperimentV1SchemaVersion = "silver-mediums.r04-effort-experiment.v1"

//go:embed r04_effort_experiment_v1.json
var r04EffortExperimentV1JSON []byte

func CurrentR04EffortExperimentV1() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(r04EffortExperimentV1JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode R04 effort experiment v1: %w", err)
	}
	if err := ValidateR04EffortExperimentV1(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateR04EffortExperimentV1(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != R04EffortExperimentV1SchemaVersion || definition.ExperimentID != "realistic-r04-claude-high-effort-v1" || definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("R04 effort experiment v1 identity or status is invalid")
	}
	if len(definition.Cases) != 1 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R04", CaseVersion: "v1", Repeats: 2}) {
		return fmt.Errorf("R04 effort experiment v1 case set drifted")
	}
	wantArm := TopologyExperimentArm{ArmID: "direct-high-v1", MaxProviderTurns: 15, TotalTokenReservation: 75000, GeneratedTokenCeiling: 30000, CostCeilingMicros: 350000}
	if len(definition.Arms) != 1 || definition.Arms[0] != wantArm {
		return fmt.Errorf("R04 effort experiment v1 arm drifted")
	}
	wantOrder := []TopologyExperimentObservation{{"r04-high-repeat-1", "R04", 1, "direct-high-v1"}, {"r04-high-repeat-2", "R04", 2, "direct-high-v1"}}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("R04 effort experiment v1 observation count drifted")
	}
	for index := range wantOrder {
		if definition.ExecutionOrder[index] != wantOrder[index] {
			return fmt.Errorf("R04 effort experiment v1 order drifted")
		}
	}
	wantExecutor := RealisticCandidate{ConfigurationID: "realistic.claudesonnet5.high.executor.r04-effort-v1", Model: "openrouter/anthropic/claude-sonnet-5", Effort: "high", ServingProvider: "anthropic"}
	if definition.Executor != wantExecutor || definition.Reviewer != (RealisticCandidate{}) {
		return fmt.Errorf("R04 effort experiment v1 model configuration drifted")
	}
	if definition.ReviewerTotalTokenReservation != 0 || definition.ReviewerGeneratedTokenCeiling != 0 || definition.MaxCorrections != 0 || definition.MaxTotalTokens != 150000 || definition.MaxIncrementalSpendMicros != 700000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("R04 effort experiment v1 budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "budget_breach", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "candidate_failure_local_nonsafety_harness_transport_stage_recovery_invalid_local_no_retry" || definition.AdvancementPolicy != "testing_only_no_role_or_model_selection" {
		return fmt.Errorf("R04 effort experiment v1 stop behavior drifted")
	}
	return nil
}

func R04EffortExperimentV1ProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateR04EffortExperimentV1(definition) == nil && definition.Status == "frozen_reviewed"
}
