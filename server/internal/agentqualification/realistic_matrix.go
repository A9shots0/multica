package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const RealisticRunMatrixSchemaVersion = "silver-mediums.realistic-run-matrix.v1"

type RealisticCandidate struct {
	ConfigurationID string `json:"configuration_id"`
	Model           string `json:"model"`
	Effort          string `json:"effort"`
	ServingProvider string `json:"serving_provider"`
}

type RealisticRunMatrix struct {
	SchemaVersion                       string               `json:"schema_version"`
	MatrixID                            string               `json:"matrix_id"`
	Status                              string               `json:"status"`
	ExecutionOrder                      []string             `json:"execution_order"`
	DeferredCases                       []string             `json:"deferred_cases"`
	Candidates                          []RealisticCandidate `json:"candidates"`
	SelectionReason                     string               `json:"selection_reason"`
	StagePolicy                         string               `json:"stage_policy"`
	ReliabilityPolicy                   string               `json:"reliability_policy"`
	MaxIncrementalSpendMicros           int64                `json:"max_incremental_spend_micros"`
	MaxTotalTokens                      int64                `json:"max_total_tokens"`
	PerObservationGeneratedTokenCeiling int64                `json:"per_observation_generated_token_ceiling"`
	PerObservationTotalTokenReservation int64                `json:"per_observation_total_token_reservation,omitempty"`
	Retries                             int                  `json:"retries"`
	OwnerInterventions                  int                  `json:"owner_interventions"`
	ProviderFallbacks                   bool                 `json:"provider_fallbacks"`
	GlobalHaltReasons                   []string             `json:"global_halt_reasons"`
	CandidateEliminationReasons         []string             `json:"candidate_elimination_reasons"`
	SharedFailurePolicy                 string               `json:"shared_failure_policy"`
	AdvancementPolicy                   string               `json:"advancement_policy"`
}

//go:embed realistic_matrix_v1.json
var realisticMatrixV1JSON []byte

//go:embed realistic_matrix_v2.json
var realisticMatrixV2JSON []byte

//go:embed realistic_matrix_v3.json
var realisticMatrixV3JSON []byte

//go:embed realistic_matrix_v4.json
var realisticMatrixV4JSON []byte

//go:embed realistic_matrix_v5.json
var realisticMatrixV5JSON []byte

//go:embed realistic_matrix_v6.json
var realisticMatrixV6JSON []byte

//go:embed realistic_matrix_v7.json
var realisticMatrixV7JSON []byte

//go:embed realistic_matrix_v8.json
var realisticMatrixV8JSON []byte

//go:embed realistic_matrix_v9.json
var realisticMatrixV9JSON []byte

//go:embed realistic_matrix_v10.json
var realisticMatrixV10JSON []byte

//go:embed realistic_matrix_v11.json
var realisticMatrixV11JSON []byte

//go:embed realistic_matrix_v12.json
var realisticMatrixV12JSON []byte

//go:embed realistic_matrix_v13.json
var realisticMatrixV13JSON []byte

//go:embed realistic_matrix_v14.json
var realisticMatrixV14JSON []byte

func CurrentRealisticRunMatrix() (RealisticRunMatrix, error) {
	return CurrentRealisticRunMatrixByID("realistic-foundation-screen-v1")
}

func CurrentRealisticRunMatrixByID(matrixID string) (RealisticRunMatrix, error) {
	var matrix RealisticRunMatrix
	raw := realisticMatrixV1JSON
	if matrixID == "realistic-foundation-screen-v2" {
		raw = realisticMatrixV2JSON
	} else if matrixID == "realistic-foundation-screen-v3" {
		raw = realisticMatrixV3JSON
	} else if matrixID == "realistic-foundation-screen-v4" {
		raw = realisticMatrixV4JSON
	} else if matrixID == "realistic-foundation-screen-v5" {
		raw = realisticMatrixV5JSON
	} else if matrixID == "realistic-foundation-screen-v6" {
		raw = realisticMatrixV6JSON
	} else if matrixID == "realistic-foundation-screen-v7" {
		raw = realisticMatrixV7JSON
	} else if matrixID == "realistic-foundation-screen-v8" {
		raw = realisticMatrixV8JSON
	} else if matrixID == "realistic-foundation-screen-v9" {
		raw = realisticMatrixV9JSON
	} else if matrixID == "realistic-foundation-screen-v10" {
		raw = realisticMatrixV10JSON
	} else if matrixID == "realistic-foundation-screen-v11" {
		raw = realisticMatrixV11JSON
	} else if matrixID == "realistic-foundation-screen-v12" {
		raw = realisticMatrixV12JSON
	} else if matrixID == "realistic-foundation-screen-v13" {
		raw = realisticMatrixV13JSON
	} else if matrixID == "realistic-foundation-screen-v14" {
		raw = realisticMatrixV14JSON
	} else if matrixID != "realistic-foundation-screen-v1" {
		return matrix, fmt.Errorf("unknown realistic matrix identity")
	}
	if err := json.Unmarshal(raw, &matrix); err != nil {
		return matrix, fmt.Errorf("decode realistic run matrix: %w", err)
	}
	if err := ValidateRealisticRunMatrix(matrix); err != nil {
		return RealisticRunMatrix{}, err
	}
	return matrix, nil
}

func IsFrozenRealisticCandidate(candidate RealisticCandidate) (bool, error) {
	for _, matrixID := range []string{"realistic-foundation-screen-v1", "realistic-foundation-screen-v2", "realistic-foundation-screen-v3", "realistic-foundation-screen-v4", "realistic-foundation-screen-v5", "realistic-foundation-screen-v6", "realistic-foundation-screen-v7", "realistic-foundation-screen-v8", "realistic-foundation-screen-v9", "realistic-foundation-screen-v10", "realistic-foundation-screen-v11", "realistic-foundation-screen-v12", "realistic-foundation-screen-v13", "realistic-foundation-screen-v14"} {
		matrix, err := CurrentRealisticRunMatrixByID(matrixID)
		if err != nil {
			return false, err
		}
		for _, allowed := range matrix.Candidates {
			if allowed == candidate {
				return true, nil
			}
		}
	}
	return false, nil
}

func ValidateRealisticRunMatrix(matrix RealisticRunMatrix) error {
	if matrix.SchemaVersion != RealisticRunMatrixSchemaVersion || matrix.MatrixID != "realistic-foundation-screen-v1" && matrix.MatrixID != "realistic-foundation-screen-v2" && matrix.MatrixID != "realistic-foundation-screen-v3" && matrix.MatrixID != "realistic-foundation-screen-v4" && matrix.MatrixID != "realistic-foundation-screen-v5" && matrix.MatrixID != "realistic-foundation-screen-v6" && matrix.MatrixID != "realistic-foundation-screen-v7" && matrix.MatrixID != "realistic-foundation-screen-v8" && matrix.MatrixID != "realistic-foundation-screen-v9" && matrix.MatrixID != "realistic-foundation-screen-v10" && matrix.MatrixID != "realistic-foundation-screen-v11" && matrix.MatrixID != "realistic-foundation-screen-v12" && matrix.MatrixID != "realistic-foundation-screen-v13" && matrix.MatrixID != "realistic-foundation-screen-v14" {
		return fmt.Errorf("realistic matrix identity is incomplete")
	}
	if matrix.Status != "pending_independent_review" && matrix.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic matrix status is invalid")
	}
	continuation := matrix.MatrixID == "realistic-foundation-screen-v5" || matrix.MatrixID == "realistic-foundation-screen-v6" || matrix.MatrixID == "realistic-foundation-screen-v7"
	r04Continuation := matrix.MatrixID == "realistic-foundation-screen-v8"
	r02Browser := matrix.MatrixID == "realistic-foundation-screen-v9"
	reliabilityRepeat := matrix.MatrixID == "realistic-foundation-screen-v10"
	reliabilityBrowser := matrix.MatrixID == "realistic-foundation-screen-v11"
	lunaCapability := matrix.MatrixID == "realistic-foundation-screen-v12"
	survivorCapability := matrix.MatrixID == "realistic-foundation-screen-v13"
	affectedReliability := matrix.MatrixID == "realistic-foundation-screen-v14"
	if !continuation && !r04Continuation && !r02Browser && !reliabilityRepeat && !reliabilityBrowser && !affectedReliability && (!exactSet(matrix.ExecutionOrder, "R01", "R03", "R04") || len(matrix.ExecutionOrder) != 3 || matrix.ExecutionOrder[0] != "R01" || matrix.ExecutionOrder[2] != "R04" || !exactSet(matrix.DeferredCases, "R02")) {
		return fmt.Errorf("realistic matrix execution order drifted")
	}
	if continuation && (!exactSet(matrix.ExecutionOrder, "R03", "R04") || len(matrix.ExecutionOrder) != 2 || matrix.ExecutionOrder[0] != "R03" || !exactSet(matrix.DeferredCases, "R01", "R02")) {
		return fmt.Errorf("realistic continuation matrix execution order drifted")
	}
	if r04Continuation && (!exactSet(matrix.ExecutionOrder, "R04") || !exactSet(matrix.DeferredCases, "R01", "R02", "R03")) {
		return fmt.Errorf("realistic R04 continuation matrix execution order drifted")
	}
	if r02Browser && (!exactSet(matrix.ExecutionOrder, "R02") || !exactSet(matrix.DeferredCases, "R01", "R03", "R04")) {
		return fmt.Errorf("realistic R02 Browser matrix execution order drifted")
	}
	if reliabilityRepeat && (!exactSet(matrix.ExecutionOrder, "R01", "R03", "R04") || len(matrix.ExecutionOrder) != 3 || matrix.ExecutionOrder[0] != "R01" || matrix.ExecutionOrder[2] != "R04" || !exactSet(matrix.DeferredCases, "R02")) {
		return fmt.Errorf("realistic reliability-repeat matrix execution order drifted")
	}
	if reliabilityBrowser && (!exactSet(matrix.ExecutionOrder, "R02") || !exactSet(matrix.DeferredCases, "R01", "R03", "R04")) {
		return fmt.Errorf("realistic reliability Browser matrix execution order drifted")
	}
	if affectedReliability && (!exactSet(matrix.ExecutionOrder, "R03") || !exactSet(matrix.DeferredCases, "R01", "R02", "R04")) {
		return fmt.Errorf("realistic affected reliability matrix execution order drifted")
	}
	wantCandidates := 4
	if continuation {
		wantCandidates = 3
	}
	if matrix.MatrixID == "realistic-foundation-screen-v7" || matrix.MatrixID == "realistic-foundation-screen-v8" {
		wantCandidates = 2
	}
	if r02Browser || reliabilityRepeat || reliabilityBrowser || lunaCapability || affectedReliability {
		wantCandidates = 1
	}
	if len(matrix.Candidates) != wantCandidates {
		return fmt.Errorf("realistic matrix must contain three inexpensive candidates and one anchor")
	}
	wantModels := []string{"openrouter/qwen/qwen3-coder-flash", "openrouter/z-ai/glm-5.3-flash", "openrouter/inception/mercury-2.5", "openrouter/anthropic/claude-sonnet-5"}
	if matrix.MatrixID == "realistic-foundation-screen-v2" {
		wantModels = []string{"openrouter/mistralai/mistral-small-2603", "openrouter/nvidia/nemotron-3.5-lightning", "openrouter/openai/gpt-oss-20b", "openrouter/openai/gpt-5.4"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v3" {
		wantModels = []string{"openrouter/inclusionai/ling-3.0-flash", "openrouter/minimax/minimax-m2.7", "openrouter/xiaomi/mimo-v2.5", "openrouter/nvidia/nemotron-3-super-120b-a12b:free"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v4" {
		wantModels = []string{"openrouter/inception/mercury-2.5", "openrouter/anthropic/claude-sonnet-5", "openrouter/openai/gpt-5.4", "openrouter/nvidia/nemotron-3-super-120b-a12b:free"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v5" {
		wantModels = []string{"openrouter/anthropic/claude-sonnet-5", "openrouter/openai/gpt-5.4", "openrouter/nvidia/nemotron-3-super-120b-a12b:free"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v6" {
		wantModels = []string{"openrouter/anthropic/claude-sonnet-5", "openrouter/openai/gpt-5.4", "openrouter/nvidia/nemotron-3-super-120b-a12b:free"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v7" {
		wantModels = []string{"openrouter/anthropic/claude-sonnet-5", "openrouter/openai/gpt-5.4"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v8" {
		wantModels = []string{"openrouter/anthropic/claude-sonnet-5", "openrouter/openai/gpt-5.4"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v9" {
		wantModels = []string{"openrouter/anthropic/claude-sonnet-5"}
	} else if matrix.MatrixID == "realistic-foundation-screen-v10" || matrix.MatrixID == "realistic-foundation-screen-v11" {
		wantModels = []string{"openrouter/anthropic/claude-sonnet-5"}
	} else if lunaCapability {
		wantModels = []string{"openrouter/openai/gpt-5.6-luna-pro"}
	} else if survivorCapability {
		wantModels = []string{"openrouter/google/gemini-3.5-flash-lite", "openrouter/nex-agi/nex-n2.5-pro:free", "openrouter/qwen/qwen3-coder-next", "openrouter/upstage/solar-pro4"}
	} else if affectedReliability {
		wantModels = []string{"openrouter/anthropic/claude-sonnet-5"}
	}
	seen := map[string]struct{}{}
	for index, candidate := range matrix.Candidates {
		if candidate.ConfigurationID == "" || candidate.Model != wantModels[index] || candidate.Effort == "" || candidate.ServingProvider == "" {
			return fmt.Errorf("realistic candidate %d drifted", index)
		}
		if _, duplicate := seen[candidate.ConfigurationID]; duplicate {
			return fmt.Errorf("realistic candidate duplicated")
		}
		seen[candidate.ConfigurationID] = struct{}{}
	}
	wantTotalTokens := int64(200000)
	if continuation && matrix.MatrixID != "realistic-foundation-screen-v7" {
		wantTotalTokens = 300000
	}
	if r02Browser {
		wantTotalTokens = 100000
	} else if reliabilityRepeat || lunaCapability {
		wantTotalTokens = 225000
	} else if reliabilityBrowser {
		wantTotalTokens = 75000
	}
	if survivorCapability {
		wantTotalTokens = 900000
	} else if affectedReliability {
		wantTotalTokens = 75000
	}
	wantSpendMicros := int64(5000000)
	if affectedReliability {
		wantSpendMicros = 500000
	}
	if matrix.MaxIncrementalSpendMicros != wantSpendMicros || matrix.MaxTotalTokens != wantTotalTokens || matrix.PerObservationGeneratedTokenCeiling != 20000 || matrix.Retries != 0 || matrix.OwnerInterventions != 0 || matrix.ProviderFallbacks {
		return fmt.Errorf("realistic matrix budget or retry policy drifted")
	}
	if (reliabilityRepeat || reliabilityBrowser || lunaCapability || survivorCapability || affectedReliability) && matrix.PerObservationTotalTokenReservation != 75000 || !reliabilityRepeat && !reliabilityBrowser && !lunaCapability && !survivorCapability && !affectedReliability && matrix.PerObservationTotalTokenReservation != 0 {
		return fmt.Errorf("realistic matrix prelaunch token reservation drifted")
	}
	wantGlobalHalts := []string{"test_invalidity", "evidence_integrity_failure", "unsafe_execution", "provider_transport_failure", "budget_breach"}
	if survivorCapability || affectedReliability {
		wantGlobalHalts = []string{"credential_failure", "evidence_integrity_failure", "unsafe_execution", "budget_breach", "uncontrolled_spending"}
	}
	if !exactSet(matrix.GlobalHaltReasons, wantGlobalHalts...) || !exactSet(matrix.CandidateEliminationReasons, "competence_failure", "hard_disqualifier", "response_or_tool_protocol_failure") {
		return fmt.Errorf("realistic matrix stop behavior drifted")
	}
	wantSharedFailure := "same_case_failure_by_two_unrelated_candidates_pauses_that_case_for_fixture_grader_runner_audit_without_halting_unrelated_valid_cases"
	if continuation || r04Continuation || r02Browser {
		wantSharedFailure = "same_case_failure_by_two_unrelated_candidates_is_logged_for_post_batch_audit_without_pausing_unaffected_candidates"
	} else if reliabilityRepeat || reliabilityBrowser {
		wantSharedFailure = "single_candidate_repeat_failure_is_recorded_without_retesting_or_changing_the_frozen_case"
	} else if lunaCapability {
		wantSharedFailure = "single_candidate_failure_is_recorded_without_retesting_or_changing_the_frozen_case"
	} else if survivorCapability {
		wantSharedFailure = "same_case_failure_by_two_unrelated_candidates_is_logged_for_post_batch_audit_without_pausing_unaffected_candidates"
	} else if affectedReliability {
		wantSharedFailure = "single_affected_reliability_observation_is_recorded_without_retesting_or_changing_the_frozen_case"
	}
	if matrix.SharedFailurePolicy != wantSharedFailure {
		return fmt.Errorf("realistic matrix shared-failure behavior drifted")
	}
	wantStage := "all_candidates_run_R01_once_then_only_valid_passes_continue_R03_R04_in_order"
	if continuation {
		wantStage = "all_candidates_run_R03_once_then_only_valid_passes_continue_R04"
	} else if r04Continuation {
		wantStage = "all_candidates_run_R04_once"
	} else if r02Browser {
		wantStage = "sole_survivor_runs_R02_once_with_trusted_browser_handoff"
	} else if reliabilityRepeat {
		wantStage = "sole_survivor_repeats_R01_R03_R04_once_then_R02_only_if_all_pass"
	} else if reliabilityBrowser {
		wantStage = "sole_survivor_repeats_R02_once_after_v10_passes"
	} else if lunaCapability {
		wantStage = "sole_static_reliability_survivor_runs_R01_then_only_valid_pass_continues_R03_R04"
	} else if affectedReliability {
		wantStage = "sole_existing_survivor_repeats_only_the_invalidated_R03_observation_once"
	}
	wantReliabilityPolicy := "no_repeat_until_one_valid_paid_group_pass_then_repeat_same_frozen_paid_group"
	if lunaCapability {
		wantReliabilityPolicy = "first_realistic_capability_pass_no_retries"
	} else if survivorCapability {
		wantReliabilityPolicy = "first_realistic_capability_pass_no_retries"
	} else if affectedReliability {
		wantReliabilityPolicy = "affected_observation_replacement_only_no_retries"
	}
	if matrix.StagePolicy != wantStage || matrix.ReliabilityPolicy != wantReliabilityPolicy || matrix.AdvancementPolicy != "testing_evidence_only_no_role_assignment_no_production_promotion" {
		return fmt.Errorf("realistic matrix advancement behavior drifted")
	}
	return nil
}
