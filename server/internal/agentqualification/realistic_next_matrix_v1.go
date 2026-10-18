package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const RealisticNextRunMatrixSchemaVersion = "silver-mediums.realistic-next-run-matrix.v1"

type RealisticNextRunMatrix struct {
	SchemaVersion                       string               `json:"schema_version"`
	MatrixID                            string               `json:"matrix_id"`
	Status                              string               `json:"status"`
	ExecutionOrder                      []string             `json:"execution_order"`
	Candidates                          []RealisticCandidate `json:"candidates"`
	SelectionReason                     string               `json:"selection_reason"`
	StagePolicy                         string               `json:"stage_policy"`
	MaxIncrementalSpendMicros           int64                `json:"max_incremental_spend_micros"`
	MaxTotalTokens                      int64                `json:"max_total_tokens"`
	PerObservationTotalTokenReservation int64                `json:"per_observation_total_token_reservation"`
	PerObservationGeneratedTokenCeiling int64                `json:"per_observation_generated_token_ceiling"`
	Retries                             int                  `json:"retries"`
	OwnerInterventions                  int                  `json:"owner_interventions"`
	ProviderFallbacks                   bool                 `json:"provider_fallbacks"`
	GlobalHaltReasons                   []string             `json:"global_halt_reasons"`
	CandidateEliminationReasons         []string             `json:"candidate_elimination_reasons"`
	ObservationLocalInvalidPolicy       string               `json:"observation_local_invalid_policy"`
	SharedFailurePolicy                 string               `json:"shared_failure_policy"`
	AdvancementPolicy                   string               `json:"advancement_policy"`
}

//go:embed realistic_next_matrix_v1.json
var realisticNextMatrixV1JSON []byte

func CurrentRealisticNextRunMatrix() (RealisticNextRunMatrix, error) {
	var matrix RealisticNextRunMatrix
	if err := json.Unmarshal(realisticNextMatrixV1JSON, &matrix); err != nil {
		return matrix, fmt.Errorf("decode realistic next run matrix: %w", err)
	}
	if err := ValidateRealisticNextRunMatrix(matrix); err != nil {
		return RealisticNextRunMatrix{}, err
	}
	return matrix, nil
}

func ValidateRealisticNextRunMatrix(matrix RealisticNextRunMatrix) error {
	if matrix.SchemaVersion != RealisticNextRunMatrixSchemaVersion || matrix.MatrixID != "realistic-continuity-and-rollout-screen-v1" || matrix.Status != "pending_independent_review" && matrix.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic next matrix identity drifted")
	}
	if len(matrix.ExecutionOrder) != 2 || matrix.ExecutionOrder[0] != "R05" || matrix.ExecutionOrder[1] != "R06" {
		return fmt.Errorf("realistic next execution order drifted")
	}
	want := []RealisticCandidate{
		{ConfigurationID: "realistic-next.claudesonnet5.medium.anchor.v1", Model: "openrouter/anthropic/claude-sonnet-5", Effort: "medium", ServingProvider: "anthropic"},
		{ConfigurationID: "realistic-next.gpt54.medium.anchor.v1", Model: "openrouter/openai/gpt-5.4", Effort: "medium", ServingProvider: "openai"},
	}
	if len(matrix.Candidates) != len(want) {
		return fmt.Errorf("realistic next matrix must contain two anchors")
	}
	for index := range want {
		if matrix.Candidates[index] != want[index] {
			return fmt.Errorf("realistic next candidate %d drifted", index)
		}
	}
	if matrix.SelectionReason != "compare_the_two_existing_strong_realistic_anchors_on_the_two_deliberately_uncovered_capabilities_without_expanding_the_model_pool_or_selecting_roles" || matrix.StagePolicy != "all_candidates_run_R05_once_then_only_valid_passes_continue_R06" {
		return fmt.Errorf("realistic next selection or stage policy drifted")
	}
	if matrix.MaxIncrementalSpendMicros != 3000000 || matrix.MaxTotalTokens != 300000 || matrix.PerObservationTotalTokenReservation != 75000 || matrix.PerObservationGeneratedTokenCeiling != 20000 || matrix.Retries != 0 || matrix.OwnerInterventions != 0 || matrix.ProviderFallbacks {
		return fmt.Errorf("realistic next budget or retry policy drifted")
	}
	if !exactSet(matrix.GlobalHaltReasons, "credential_failure", "unsafe_execution", "budget_breach", "uncontrolled_spending", "evidence_or_transport_corruption_making_further_results_unusable") || !exactSet(matrix.CandidateEliminationReasons, "competence_failure", "hard_disqualifier", "response_or_tool_protocol_failure") {
		return fmt.Errorf("realistic next halt behavior drifted")
	}
	if matrix.ObservationLocalInvalidPolicy != "preserve_mark_invalid_log_for_post_batch_correction_and_continue_unaffected_bindings" || matrix.SharedFailurePolicy != "same_case_failure_by_two_unrelated_candidates_triggers_post_batch_test_audit_without_changing_the_frozen_batch_or_halting_unaffected_evidence" || matrix.AdvancementPolicy != "testing_evidence_only_no_role_assignment_no_production_promotion" {
		return fmt.Errorf("realistic next continuation or advancement policy drifted")
	}
	return nil
}
