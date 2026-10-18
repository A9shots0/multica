package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
)

const PrimarySubscriptionReliabilityTrialResultV2Schema = "silver-mediums.primary-subscription-reliability-trial-result.v2"

type PrimarySubscriptionReliabilityTrialLaneResultV2 struct {
	Sessions            int    `json:"sessions"`
	CompletedSessions   int    `json:"completed_sessions"`
	InputTokens         int64  `json:"input_tokens"`
	CachedInputTokens   int64  `json:"cached_input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	ReasoningTokens     int64  `json:"reasoning_tokens"`
	WallMillis          int64  `json:"wall_millis"`
	CostMicros          int64  `json:"cost_micros"`
	CostBoundary        string `json:"cost_boundary"`
	RequestedModel      string `json:"requested_model"`
	ObservedModelStatus string `json:"observed_model_status"`
}

type PrimarySubscriptionReliabilityTrialResultV2 struct {
	SchemaVersion            string                                          `json:"schema_version"`
	TrialID                  string                                          `json:"trial_id"`
	Status                   string                                          `json:"status"`
	DefinitionSHA256         string                                          `json:"definition_sha256"`
	PriorTrialResultSHA256   string                                          `json:"prior_trial_result_sha256"`
	LedgerSHA256             string                                          `json:"ledger_sha256"`
	EvidenceManifestSHA256   string                                          `json:"evidence_manifest_sha256"`
	IndependentReviewSHA256  string                                          `json:"independent_review_sha256"`
	Sessions                 int                                             `json:"sessions"`
	ProviderCalls            int                                             `json:"provider_calls"`
	Observations             int                                             `json:"observations"`
	Passes                   int                                             `json:"passes"`
	Invalid                  int                                             `json:"invalid"`
	Failed                   int                                             `json:"failed"`
	Claude                   PrimarySubscriptionReliabilityTrialLaneResultV2 `json:"claude"`
	Codex                    PrimarySubscriptionReliabilityTrialLaneResultV2 `json:"codex"`
	Cells                    []PrimarySubscriptionReliabilityTrialCellV2     `json:"cells"`
	Retries                  int                                             `json:"retries"`
	Fallbacks                int                                             `json:"fallbacks"`
	NativeToolExecutions     int                                             `json:"native_tool_executions"`
	RepositoryAccesses       int                                             `json:"repository_accesses"`
	ProductionAccesses       int                                             `json:"production_accesses"`
	BusinessActions          int                                             `json:"business_actions"`
	PhoneFarmAccesses        int                                             `json:"phone_farm_accesses"`
	PermanentSelections      int                                             `json:"permanent_selections"`
	ProvisionalConfiguration string                                          `json:"provisional_configuration"`
	ProductUIScope           string                                          `json:"product_ui_scope"`
	ModelSelection           string                                          `json:"model_selection"`
	PermanentRoleSelection   string                                          `json:"permanent_role_selection"`
	ProductionActivation     string                                          `json:"production_activation"`
	NextBoundary             string                                          `json:"next_boundary"`
}

//go:embed primary_subscription_reliability_trial_result_v2.json
var primarySubscriptionReliabilityTrialResultV2JSON []byte

func CurrentPrimarySubscriptionReliabilityTrialResultV2() (PrimarySubscriptionReliabilityTrialResultV2, error) {
	var result PrimarySubscriptionReliabilityTrialResultV2
	if err := strictJSON(primarySubscriptionReliabilityTrialResultV2JSON, &result); err != nil {
		return result, fmt.Errorf("decode primary subscription reliability trial result v2: %w", err)
	}
	return result, ValidatePrimarySubscriptionReliabilityTrialResultV2(result)
}

func PrimarySubscriptionReliabilityTrialResultV2SHA256() string {
	sum := sha256.Sum256(primarySubscriptionReliabilityTrialResultV2JSON)
	return hex.EncodeToString(sum[:])
}

func ValidatePrimarySubscriptionReliabilityTrialResultV2(result PrimarySubscriptionReliabilityTrialResultV2) error {
	if result.SchemaVersion != PrimarySubscriptionReliabilityTrialResultV2Schema || result.TrialID != "item-14-primary-subscription-reliability-trial-v2-20260914" || result.Status != "evidence_valid_provisional_scope_only" || result.DefinitionSHA256 != PrimarySubscriptionReliabilityTrialV2SHA256() || result.PriorTrialResultSHA256 != PrimarySubscriptionRoleTrialResultV1SHA256() {
		return fmt.Errorf("primary subscription reliability trial result v2 identity drifted")
	}
	for _, hash := range []string{result.LedgerSHA256, result.EvidenceManifestSHA256, result.IndependentReviewSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("primary subscription reliability trial result v2 hash invalid")
		}
	}
	if result.LedgerSHA256 != "cea2000f9811d0ae5254ea8c86a94faf793389d9340cf29677ebb3f4478e730c" || result.EvidenceManifestSHA256 != "c81d4c185bca6a01c1fa51670af6c9987e5e9ff55e0f7e569b09204f115f8eb2" || result.IndependentReviewSHA256 != "f215234e6bf352255e72bef543160aeb6580d82daf385ac793fc825123c0dd8f" {
		return fmt.Errorf("primary subscription reliability trial result v2 evidence binding drifted")
	}
	if result.Sessions != 12 || result.ProviderCalls != 12 || result.Observations != 12 || result.Passes != 6 || result.Invalid != 0 || result.Failed != 6 {
		return fmt.Errorf("primary subscription reliability trial result v2 counts drifted")
	}
	wantClaude := PrimarySubscriptionReliabilityTrialLaneResultV2{Sessions: 6, CompletedSessions: 6, InputTokens: 43484, CachedInputTokens: 17080, OutputTokens: 15550, WallMillis: 239440, CostMicros: 396763, CostBoundary: "claude_code_subscription_cli_api_equivalent_estimate", RequestedModel: "claude-sonnet-4-6", ObservedModelStatus: "sonnet_in_six_receipts_composite_transport_limitation_retained"}
	wantCodex := PrimarySubscriptionReliabilityTrialLaneResultV2{Sessions: 6, CompletedSessions: 6, InputTokens: 56475, OutputTokens: 5362, WallMillis: 151585, CostBoundary: "codex_subscription_no_provider_cash_receipt", RequestedModel: "gpt-5.6-luna", ObservedModelStatus: "unavailable_truthful_cli_request_pinned"}
	if result.Claude != wantClaude || result.Codex != wantCodex {
		return fmt.Errorf("primary subscription reliability trial result v2 lane accounting drifted")
	}
	wantCells := []PrimarySubscriptionReliabilityTrialCellV2{
		{Lane: "claude_code_subscription", CaseID: "R01", Observations: 3, Passes: 3},
		{Lane: "claude_code_subscription", CaseID: "R02", Observations: 3, Failed: 3},
		{Lane: "codex_subscription", CaseID: "R01", Observations: 3, Passes: 3},
		{Lane: "codex_subscription", CaseID: "R02", Observations: 3, Failed: 3},
	}
	if !slices.Equal(result.Cells, wantCells) {
		return fmt.Errorf("primary subscription reliability trial result v2 cells drifted")
	}
	if result.Retries != 0 || result.Fallbacks != 0 || result.NativeToolExecutions != 0 || result.RepositoryAccesses != 0 || result.ProductionAccesses != 0 || result.BusinessActions != 0 || result.PhoneFarmAccesses != 0 || result.PermanentSelections != 0 {
		return fmt.Errorf("primary subscription reliability trial result v2 expanded authority")
	}
	if result.ProvisionalConfiguration != "local_r01_inline_shared_executor_exact_skill_provider_unselected" || result.ProductUIScope != "held_unqualified" || result.ModelSelection != "none" || result.PermanentRoleSelection != "none" || result.ProductionActivation != "forbidden" || result.NextBoundary != "provider_free_provisional_role_stage_binding_and_shadow_rehearsal" {
		return fmt.Errorf("primary subscription reliability trial result v2 decision boundary drifted")
	}
	return nil
}
