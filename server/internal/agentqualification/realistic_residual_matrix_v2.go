package agentqualification

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
)

const RealisticResidualRunMatrixV2SchemaVersion = "silver-mediums.realistic-residual-run-matrix.v2"

type RealisticResidualRunMatrixV2 struct {
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
	ObservationExhaustionPolicy         string               `json:"observation_exhaustion_policy"`
	GlobalHaltReasons                   []string             `json:"global_halt_reasons"`
	CandidateEliminationReasons         []string             `json:"candidate_elimination_reasons"`
	AdvancementPolicy                   string               `json:"advancement_policy"`
}

type RealisticResidualReceiptUsageV2 struct {
	ProviderCalls, InputTokens, OutputTokens, ReasoningTokens int64
	CostMicros                                                int64
}

// RealisticResidualReceiptUsageV2 recomputes usage from immutable raw
// receipts. Callers must not trust convenience token fields returned beside
// those receipts.
func ComputeRealisticResidualReceiptUsageV2(receipts [][]byte) (RealisticResidualReceiptUsageV2, error) {
	accounting, err := recomputeRealisticProviderAccounting(receipts)
	if err != nil {
		return RealisticResidualReceiptUsageV2{}, err
	}
	return RealisticResidualReceiptUsageV2{ProviderCalls: int64(accounting.ProviderCalls), InputTokens: accounting.InputTokens, OutputTokens: accounting.OutputTokens, ReasoningTokens: accounting.ReasoningTokens, CostMicros: accounting.CostMicros}, nil
}

// RealisticResidualBoundedProviderV2 enforces the combined observation
// reservation after every receipted call, before another call can begin.
type RealisticResidualBoundedProviderV2 struct {
	Inner              RealisticProvider
	MaxTotalTokens     int64
	MaxGeneratedTokens int64
	receipts           [][]byte
}

func (provider *RealisticResidualBoundedProviderV2) Next(ctx context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	if provider == nil || provider.Inner == nil || provider.MaxTotalTokens <= 0 || provider.MaxGeneratedTokens <= 0 {
		return RealisticProviderResponse{}, fmt.Errorf("%w: residual bounded provider is incomplete", ErrRealisticProviderPreflight)
	}
	response, err := provider.Inner.Next(ctx, request)
	if err != nil {
		return response, err
	}
	if len(bytes.TrimSpace(response.RawReceipt)) == 0 {
		return response, ErrRealisticEvidenceIntegrity
	}
	provider.receipts = append(provider.receipts, append([]byte(nil), response.RawReceipt...))
	usage, usageErr := ComputeRealisticResidualReceiptUsageV2(provider.receipts)
	if usageErr != nil {
		return response, fmt.Errorf("%w: %v", ErrRealisticEvidenceIntegrity, usageErr)
	}
	if usage.InputTokens+usage.OutputTokens+usage.ReasoningTokens > provider.MaxTotalTokens || usage.OutputTokens+usage.ReasoningTokens > provider.MaxGeneratedTokens {
		response.ProtocolFailure = "candidate_resource_exhaustion"
	}
	return response, nil
}

//go:embed realistic_residual_matrix_v2.json
var realisticResidualMatrixV2JSON []byte

func CurrentRealisticResidualRunMatrixV2() (RealisticResidualRunMatrixV2, error) {
	var matrix RealisticResidualRunMatrixV2
	if err := json.Unmarshal(realisticResidualMatrixV2JSON, &matrix); err != nil {
		return matrix, fmt.Errorf("decode realistic residual matrix v2: %w", err)
	}
	if err := ValidateRealisticResidualRunMatrixV2(matrix); err != nil {
		return RealisticResidualRunMatrixV2{}, err
	}
	return matrix, nil
}

func ValidateRealisticResidualRunMatrixV2(matrix RealisticResidualRunMatrixV2) error {
	if matrix.SchemaVersion != RealisticResidualRunMatrixV2SchemaVersion || matrix.MatrixID != "realistic-continuity-and-rollout-residual-v31" || matrix.Status != "pending_independent_review" && matrix.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic residual matrix v2 identity drifted")
	}
	if len(matrix.ExecutionOrder) != 2 || matrix.ExecutionOrder[0] != "R05" || matrix.ExecutionOrder[1] != "R06" {
		return fmt.Errorf("realistic residual matrix v2 execution order drifted")
	}
	want := RealisticCandidate{ConfigurationID: "realistic-next.gpt54.medium.anchor.v1", Model: "openrouter/openai/gpt-5.4", Effort: "medium", ServingProvider: "openai"}
	if len(matrix.Candidates) != 1 || matrix.Candidates[0] != want {
		return fmt.Errorf("realistic residual matrix v2 must contain only the unseen GPT-5.4 binding")
	}
	if matrix.SelectionReason != "observe_the_only_still_unseen_v30_anchor_on_the_identical_frozen_R05_and_R06_cases_without_selecting_a_role" || matrix.StagePolicy != "run_R05_once_then_run_R06_only_after_a_valid_R05_pass" {
		return fmt.Errorf("realistic residual matrix v2 selection or stage policy drifted")
	}
	if matrix.MaxIncrementalSpendMicros != 1_000_000 || matrix.MaxTotalTokens != 150_000 || matrix.PerObservationTotalTokenReservation != 75_000 || matrix.PerObservationGeneratedTokenCeiling != 20_000 || matrix.Retries != 0 || matrix.OwnerInterventions != 0 || matrix.ProviderFallbacks {
		return fmt.Errorf("realistic residual matrix v2 budget or retry policy drifted")
	}
	if matrix.ObservationExhaustionPolicy != "reserved_per_observation_token_action_or_cost_exhaustion_is_candidate_local_and_never_labeled_campaign_budget_breach" {
		return fmt.Errorf("realistic residual matrix v2 local exhaustion policy drifted")
	}
	if !exactSet(matrix.GlobalHaltReasons, "credential_failure", "runtime_or_config_failure", "credit_failure", "transport_failure", "unsafe_execution", "campaign_budget_breach", "uncontrolled_spending", "evidence_corruption_making_further_results_unusable") || !exactSet(matrix.CandidateEliminationReasons, "competence_failure", "hard_disqualifier", "response_or_tool_protocol_failure", "candidate_resource_exhaustion") {
		return fmt.Errorf("realistic residual matrix v2 halt behavior drifted")
	}
	if matrix.AdvancementPolicy != "testing_evidence_only_no_role_assignment_no_production_promotion" {
		return fmt.Errorf("realistic residual matrix v2 advancement policy drifted")
	}
	return nil
}

func (matrix RealisticResidualRunMatrixV2) AsRealisticNextV1() RealisticNextRunMatrix {
	return RealisticNextRunMatrix{
		SchemaVersion: RealisticNextRunMatrixSchemaVersion, MatrixID: matrix.MatrixID, Status: matrix.Status,
		ExecutionOrder: append([]string(nil), matrix.ExecutionOrder...), Candidates: append([]RealisticCandidate(nil), matrix.Candidates...),
		SelectionReason: matrix.SelectionReason, StagePolicy: matrix.StagePolicy,
		MaxIncrementalSpendMicros: matrix.MaxIncrementalSpendMicros, MaxTotalTokens: matrix.MaxTotalTokens,
		PerObservationTotalTokenReservation: matrix.PerObservationTotalTokenReservation,
		PerObservationGeneratedTokenCeiling: matrix.PerObservationGeneratedTokenCeiling,
		Retries:                             matrix.Retries, OwnerInterventions: matrix.OwnerInterventions, ProviderFallbacks: matrix.ProviderFallbacks,
		GlobalHaltReasons:             []string{"credential_failure", "unsafe_execution", "budget_breach", "uncontrolled_spending", "evidence_or_transport_corruption_making_further_results_unusable"},
		CandidateEliminationReasons:   []string{"competence_failure", "hard_disqualifier", "response_or_tool_protocol_failure"},
		ObservationLocalInvalidPolicy: "preserve_mark_invalid_log_for_post_batch_correction_and_continue_unaffected_bindings",
		SharedFailurePolicy:           "same_case_failure_by_two_unrelated_candidates_triggers_post_batch_test_audit_without_changing_the_frozen_batch_or_halting_unaffected_evidence",
		AdvancementPolicy:             matrix.AdvancementPolicy,
	}
}
