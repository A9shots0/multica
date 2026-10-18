package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

const PrimarySubscriptionRoleTrialResultV1Schema = "silver-mediums.primary-subscription-role-trial-result.v1"

type PrimarySubscriptionRoleTrialLaneResultV1 struct {
	Sessions            int    `json:"sessions"`
	CompletedSessions   int    `json:"completed_sessions"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	WallMillis          int64  `json:"wall_millis"`
	CostMicros          int64  `json:"cost_micros"`
	RequestedModel      string `json:"requested_model"`
	ObservedModelStatus string `json:"observed_model_status"`
}

type PrimarySubscriptionRoleTrialResultV1 struct {
	SchemaVersion                               string                                   `json:"schema_version"`
	TrialID                                     string                                   `json:"trial_id"`
	Status                                      string                                   `json:"status"`
	DefinitionSHA256                            string                                   `json:"definition_sha256"`
	LedgerSHA256                                string                                   `json:"ledger_sha256"`
	EvidenceManifestSHA256                      string                                   `json:"evidence_manifest_sha256"`
	IndependentReviewSHA256                     string                                   `json:"independent_review_sha256"`
	CorrectionRecordSHA256                      string                                   `json:"correction_record_sha256"`
	Sessions                                    int                                      `json:"sessions"`
	ProviderCalls                               int                                      `json:"provider_calls"`
	Observations                                int                                      `json:"observations"`
	Passes                                      int                                      `json:"passes"`
	Invalid                                     int                                      `json:"invalid"`
	Failed                                      int                                      `json:"failed"`
	InlineObservations                          int                                      `json:"inline_observations"`
	InlinePasses                                int                                      `json:"inline_passes"`
	SeparateObservations                        int                                      `json:"separate_observations"`
	SeparatePasses                              int                                      `json:"separate_passes"`
	Claude                                      PrimarySubscriptionRoleTrialLaneResultV1 `json:"claude"`
	Codex                                       PrimarySubscriptionRoleTrialLaneResultV1 `json:"codex"`
	SeparateProductUISpecialistSessionsComplete int                                      `json:"separate_product_ui_specialist_sessions_completed"`
	SeparateProductUIObservationsPassed         int                                      `json:"separate_product_ui_observations_passed"`
	DownstreamExecutorContractFailures          int                                      `json:"downstream_executor_contract_failures"`
	Retries                                     int                                      `json:"retries"`
	Fallbacks                                   int                                      `json:"fallbacks"`
	NativeToolExecutions                        int                                      `json:"native_tool_executions"`
	RepositoryAccesses                          int                                      `json:"repository_accesses"`
	ProductionAccesses                          int                                      `json:"production_accesses"`
	BusinessActions                             int                                      `json:"business_actions"`
	PhoneFarmAccesses                           int                                      `json:"phone_farm_accesses"`
	PermanentSelections                         int                                      `json:"permanent_selections"`
	ConfigurationSelection                      string                                   `json:"configuration_selection"`
	PermanentSeatSelection                      string                                   `json:"permanent_seat_selection"`
	NextBoundary                                string                                   `json:"next_boundary"`
}

//go:embed primary_subscription_role_trial_result_v1.json
var primarySubscriptionRoleTrialResultV1JSON []byte

func CurrentPrimarySubscriptionRoleTrialResultV1() (PrimarySubscriptionRoleTrialResultV1, error) {
	var result PrimarySubscriptionRoleTrialResultV1
	if err := strictJSON(primarySubscriptionRoleTrialResultV1JSON, &result); err != nil {
		return result, fmt.Errorf("decode primary subscription trial result: %w", err)
	}
	return result, ValidatePrimarySubscriptionRoleTrialResultV1(result)
}

func PrimarySubscriptionRoleTrialResultV1SHA256() string {
	sum := sha256.Sum256(primarySubscriptionRoleTrialResultV1JSON)
	return hex.EncodeToString(sum[:])
}

func ValidatePrimarySubscriptionRoleTrialResultV1(result PrimarySubscriptionRoleTrialResultV1) error {
	if result.SchemaVersion != PrimarySubscriptionRoleTrialResultV1Schema || result.TrialID != "item-14-primary-subscription-role-trial-20260914" || result.Status != "evidence_valid_selection_held" || result.DefinitionSHA256 != PrimarySubscriptionRoleTrialV1SHA256() {
		return fmt.Errorf("primary subscription trial result identity drifted")
	}
	for _, hash := range []string{result.LedgerSHA256, result.EvidenceManifestSHA256, result.IndependentReviewSHA256, result.CorrectionRecordSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("primary subscription trial result hash invalid")
		}
	}
	if result.Sessions != 12 || result.ProviderCalls != 12 || result.Observations != 8 || result.Passes != 5 || result.Invalid != 2 || result.Failed != 1 || result.InlineObservations != 4 || result.InlinePasses != 3 || result.SeparateObservations != 4 || result.SeparatePasses != 2 {
		return fmt.Errorf("primary subscription trial result counts drifted")
	}
	if result.Claude != (PrimarySubscriptionRoleTrialLaneResultV1{Sessions: 6, CompletedSessions: 5, InputTokens: 43004, OutputTokens: 12801, WallMillis: 219484, CostMicros: 372820, RequestedModel: "claude-sonnet-4-6", ObservedModelStatus: "sonnet_only_in_six_receipts_composite_transport_limitation_retained"}) {
		return fmt.Errorf("Claude trial result drifted")
	}
	if result.Codex != (PrimarySubscriptionRoleTrialLaneResultV1{Sessions: 6, CompletedSessions: 5, InputTokens: 45755, OutputTokens: 3702, WallMillis: 95121, CostMicros: 0, RequestedModel: "gpt-5.6-luna", ObservedModelStatus: "unavailable_truthful_cli_request_pinned"}) {
		return fmt.Errorf("Codex trial result drifted")
	}
	if result.SeparateProductUISpecialistSessionsComplete != 2 || result.SeparateProductUIObservationsPassed != 0 || result.DownstreamExecutorContractFailures != 2 {
		return fmt.Errorf("trial handoff finding drifted")
	}
	if result.Retries != 0 || result.Fallbacks != 0 || result.NativeToolExecutions != 0 || result.RepositoryAccesses != 0 || result.ProductionAccesses != 0 || result.BusinessActions != 0 || result.PhoneFarmAccesses != 0 || result.PermanentSelections != 0 {
		return fmt.Errorf("trial result expanded authority")
	}
	if result.ConfigurationSelection != "held_no_configuration_qualified" || result.PermanentSeatSelection != "held_no_permanent_seat_selected" || result.NextBoundary != "provider_free_role_stage_handoff_and_executable_payload_contract" {
		return fmt.Errorf("trial result decision boundary drifted")
	}
	return nil
}
