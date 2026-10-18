package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const TopologyExperimentSchemaVersion = "silver-mediums.topology-experiment.v1"

type TopologyExperimentCase struct {
	CaseID      string `json:"case_id"`
	CaseVersion string `json:"case_version"`
	Repeats     int    `json:"repeats"`
}

type TopologyExperimentArm struct {
	ArmID                 string `json:"arm_id"`
	Reviewed              bool   `json:"reviewed"`
	MaxProviderTurns      int    `json:"max_provider_turns"`
	TotalTokenReservation int64  `json:"total_token_reservation"`
	GeneratedTokenCeiling int64  `json:"generated_token_ceiling"`
	CostCeilingMicros     int64  `json:"cost_ceiling_micros"`
}

type TopologyExperimentObservation struct {
	ObservationID string `json:"observation_id"`
	CaseID        string `json:"case_id"`
	Repeat        int    `json:"repeat"`
	ArmID         string `json:"arm_id"`
}

type TopologyExperimentDefinition struct {
	SchemaVersion                 string                          `json:"schema_version"`
	ExperimentID                  string                          `json:"experiment_id"`
	Status                        string                          `json:"status"`
	Cases                         []TopologyExperimentCase        `json:"cases"`
	Arms                          []TopologyExperimentArm         `json:"arms"`
	ExecutionOrder                []TopologyExperimentObservation `json:"execution_order"`
	Executor                      RealisticCandidate              `json:"executor"`
	Reviewer                      RealisticCandidate              `json:"reviewer"`
	ReviewerTotalTokenReservation int64                           `json:"reviewer_total_token_reservation"`
	ReviewerGeneratedTokenCeiling int64                           `json:"reviewer_generated_token_ceiling"`
	MaxCorrections                int                             `json:"max_corrections"`
	MaxTotalTokens                int64                           `json:"max_total_tokens"`
	MaxIncrementalSpendMicros     int64                           `json:"max_incremental_spend_micros"`
	ProviderFallbacks             bool                            `json:"provider_fallbacks"`
	TransportRetries              int                             `json:"transport_retries"`
	OwnerInterventions            int                             `json:"owner_interventions"`
	GlobalHaltReasons             []string                        `json:"global_halt_reasons"`
	InvalidObservationPolicy      string                          `json:"invalid_observation_policy"`
	AdvancementPolicy             string                          `json:"advancement_policy"`
}

//go:embed topology_experiment_v1.json
var topologyExperimentV1JSON []byte

func CurrentTopologyExperiment() (TopologyExperimentDefinition, error) {
	var definition TopologyExperimentDefinition
	if err := json.Unmarshal(topologyExperimentV1JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode topology experiment: %w", err)
	}
	if err := ValidateTopologyExperiment(definition); err != nil {
		return TopologyExperimentDefinition{}, err
	}
	return definition, nil
}

func ValidateTopologyExperiment(definition TopologyExperimentDefinition) error {
	if definition.SchemaVersion != TopologyExperimentSchemaVersion || definition.ExperimentID != "realistic-topology-comparison-v1" {
		return fmt.Errorf("topology experiment identity is invalid")
	}
	if definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("topology experiment status is invalid")
	}
	if len(definition.Cases) != 2 || definition.Cases[0] != (TopologyExperimentCase{CaseID: "R03", CaseVersion: "v2", Repeats: 2}) || definition.Cases[1] != (TopologyExperimentCase{CaseID: "R01", CaseVersion: "v2", Repeats: 1}) {
		return fmt.Errorf("topology experiment cases drifted")
	}
	if len(definition.Arms) != 2 || definition.Arms[0] != (TopologyExperimentArm{ArmID: "direct-v1", MaxProviderTurns: 15, TotalTokenReservation: 75000, GeneratedTokenCeiling: 20000, CostCeilingMicros: 300000}) || definition.Arms[1] != (TopologyExperimentArm{ArmID: "reviewed-v1", Reviewed: true, MaxProviderTurns: 25, TotalTokenReservation: 120000, GeneratedTokenCeiling: 30000, CostCeilingMicros: 600000}) {
		return fmt.Errorf("topology experiment arms drifted")
	}
	wantOrder := []TopologyExperimentObservation{
		{ObservationID: "r03-p1-direct", CaseID: "R03", Repeat: 1, ArmID: "direct-v1"},
		{ObservationID: "r03-p1-reviewed", CaseID: "R03", Repeat: 1, ArmID: "reviewed-v1"},
		{ObservationID: "r01-p1-reviewed", CaseID: "R01", Repeat: 1, ArmID: "reviewed-v1"},
		{ObservationID: "r01-p1-direct", CaseID: "R01", Repeat: 1, ArmID: "direct-v1"},
		{ObservationID: "r03-p2-reviewed", CaseID: "R03", Repeat: 2, ArmID: "reviewed-v1"},
		{ObservationID: "r03-p2-direct", CaseID: "R03", Repeat: 2, ArmID: "direct-v1"},
	}
	if len(definition.ExecutionOrder) != len(wantOrder) {
		return fmt.Errorf("topology experiment execution count drifted")
	}
	for index := range wantOrder {
		if definition.ExecutionOrder[index] != wantOrder[index] {
			return fmt.Errorf("topology experiment counterbalance order drifted")
		}
	}
	for _, candidate := range []RealisticCandidate{definition.Executor, definition.Reviewer} {
		if candidate.Model != "openrouter/anthropic/claude-sonnet-5" || candidate.Effort != "medium" || candidate.ServingProvider != "anthropic" || candidate.ConfigurationID == "" {
			return fmt.Errorf("topology experiment material configuration drifted")
		}
	}
	if definition.Executor.ConfigurationID == definition.Reviewer.ConfigurationID {
		return fmt.Errorf("executor and reviewer run identities are not independent")
	}
	if definition.ReviewerTotalTokenReservation != 12000 || definition.ReviewerGeneratedTokenCeiling != 4000 || definition.MaxCorrections != 1 || definition.MaxTotalTokens != 585000 || definition.MaxIncrementalSpendMicros != 3000000 || definition.ProviderFallbacks || definition.TransportRetries != 0 || definition.OwnerInterventions != 0 {
		return fmt.Errorf("topology experiment budget or loop policy drifted")
	}
	if !exactSet(definition.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "unsafe_external_action", "evidence_corruption") || definition.InvalidObservationPolicy != "preserve_affected_observation_continue_unaffected_log_post_batch_correction" || definition.AdvancementPolicy != "testing_only_no_role_or_topology_selection" {
		return fmt.Errorf("topology experiment stop behavior drifted")
	}
	return nil
}

func TopologyExperimentProviderReady(definition TopologyExperimentDefinition) bool {
	return ValidateTopologyExperiment(definition) == nil && definition.Status == "frozen_reviewed"
}
