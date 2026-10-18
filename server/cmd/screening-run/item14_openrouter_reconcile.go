package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type item14OpenRouterShadowHalt struct {
	Class           string `json:"class"`
	CleanupVerified bool   `json:"cleanup_verified"`
	Error           string `json:"error"`
	ProviderCalls   int    `json:"provider_calls"`
	RetryAllowed    bool   `json:"retry_allowed"`
}

type item14OpenRouterTerminalFailureReceipt struct {
	SchemaVersion                   string                                                      `json:"schema_version"`
	Status                          string                                                      `json:"status"`
	RunID                           string                                                      `json:"run_id"`
	WorkflowID                      string                                                      `json:"workflow_id"`
	DefinitionSHA256                string                                                      `json:"definition_sha256"`
	SuiteSHA256                     string                                                      `json:"suite_sha256"`
	AuthorityExhausted              bool                                                        `json:"authority_exhausted"`
	RetryAllowed                    bool                                                        `json:"retry_allowed"`
	PlannedProviderCalls            int                                                         `json:"planned_provider_calls"`
	ActualProviderCalls             int                                                         `json:"actual_provider_calls"`
	RemainingWorkflowsNotCalled     int                                                         `json:"remaining_workflows_not_called"`
	RequestedModel                  string                                                      `json:"requested_model"`
	ActualModel                     string                                                      `json:"actual_model"`
	RequestedServingProvider        string                                                      `json:"requested_serving_provider"`
	ActualServingProvider           string                                                      `json:"actual_serving_provider"`
	ServingProviderEvidence         string                                                      `json:"serving_provider_evidence"`
	ProviderFallbacks               bool                                                        `json:"provider_fallbacks"`
	SessionInspection               agentqualification.OpenRouterLocalShadowSessionInspectionV1 `json:"session_inspection"`
	ResponseContractValid           bool                                                        `json:"response_contract_valid"`
	ResponseContractError           string                                                      `json:"response_contract_error,omitempty"`
	CleanupVerified                 bool                                                        `json:"cleanup_verified"`
	ToolCallCount                   int                                                         `json:"tool_call_count"`
	RepositoryAccessCount           int                                                         `json:"repository_access_count"`
	ProductionAccessCount           int                                                         `json:"production_access_count"`
	OutboundActionCount             int                                                         `json:"outbound_action_count"`
	RoleModelSelectionCount         int                                                         `json:"role_model_selection_count"`
	ObservationID                   string                                                      `json:"observation_id"`
	ProviderResponseID              string                                                      `json:"provider_response_id"`
	StartedAt                       string                                                      `json:"started_at"`
	CompletedAt                     string                                                      `json:"completed_at"`
	TraceDurationMilliseconds       int64                                                       `json:"trace_duration_milliseconds"`
	AccountingWindowMilliseconds    int64                                                       `json:"accounting_window_milliseconds"`
	InputTokens                     int64                                                       `json:"input_tokens"`
	CachedInputTokens               int64                                                       `json:"cached_input_tokens"`
	OutputTokens                    int64                                                       `json:"output_tokens"`
	ReasoningTokens                 int64                                                       `json:"reasoning_tokens"`
	ProviderReportedCostMicros      int64                                                       `json:"provider_reported_cost_micros"`
	RawTraceSHA256                  string                                                      `json:"raw_trace_sha256"`
	ProviderRunFileSHA256           string                                                      `json:"provider_run_file_sha256"`
	ProviderSessionExportFileSHA256 string                                                      `json:"provider_session_export_file_sha256"`
	FailureClass                    string                                                      `json:"failure_class"`
	FailureReason                   string                                                      `json:"failure_reason"`
}

func reconcileItem14OpenRouterShadowFailure(artifactRoot string, optional ...agentqualification.QualificationContracts) (item14OpenRouterTerminalFailureReceipt, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	var receipt item14OpenRouterTerminalFailureReceipt
	if !filepath.IsAbs(artifactRoot) {
		return receipt, fmt.Errorf("Item 14 OpenRouter reconciliation root must be absolute")
	}
	definition, suite, plans, err := loadItem14OpenRouterShadowPlans(q)
	if err != nil {
		return receipt, err
	}
	if len(plans) != definition.MaxProviderCalls || len(plans) < 1 {
		return receipt, fmt.Errorf("Item 14 OpenRouter frozen plan count drifted")
	}
	plan := plans[0]
	workflowDir := filepath.Join(artifactRoot, fmt.Sprintf("%02d-%s", 1, plan.CaseID))

	var preflight item14OpenRouterShadowPreflight
	if err := readExactJSONFile(filepath.Join(artifactRoot, "provider-free-preflight.json"), &preflight); err != nil {
		return receipt, err
	}
	definitionHash := agentqualification.OpenRouterLocalShadowDefinitionV1SHA256()
	if preflight.SchemaVersion != "silver-mediums.openrouter-local-shadow-preflight.v1" || preflight.DefinitionSHA256 != definitionHash || preflight.SuiteSHA256 != definition.SuiteSHA256 || preflight.Model != definition.Candidate.Model || preflight.ServingProvider != definition.Candidate.ServingProvider || preflight.ProviderFallbacks || preflight.RetryCount != 0 || preflight.ProviderCallsMade != 0 || preflight.CredentialReads != 0 || preflight.CredentialIsolation != "fresh_empty_task_local_profile" || !preflight.TemporaryRootsAbsent || !preflight.Ready || len(preflight.ResolvedConfigSHA256) != len(plans) {
		return receipt, fmt.Errorf("Item 14 OpenRouter provider-free preflight evidence drifted")
	}

	var runClaim, authorityClaim item14OpenRouterShadowClaim
	if err := readExactJSONFile(filepath.Join(artifactRoot, "run-claim.json"), &runClaim); err != nil {
		return receipt, err
	}
	if err := readExactJSONFile(filepath.Join(filepath.Dir(artifactRoot), "item14-openrouter-authority-v1.claim.json"), &authorityClaim); err != nil {
		return receipt, err
	}
	if !reflect.DeepEqual(runClaim, authorityClaim) || runClaim.SchemaVersion != "silver-mediums.openrouter-local-shadow-claim.v1" || runClaim.RunID != definition.RunID || runClaim.DefinitionSHA256 != definitionHash || runClaim.SuiteSHA256 != definition.SuiteSHA256 || runClaim.Model != definition.Candidate.Model || runClaim.ServingProvider != definition.Candidate.ServingProvider || runClaim.MaxProviderCalls != definition.MaxProviderCalls || runClaim.RetryCount != 0 || runClaim.ReplayPolicy != "any_claim_without_receipt_is_ambiguous_and_never_replayed" {
		return receipt, fmt.Errorf("Item 14 OpenRouter authority claim drifted")
	}

	var workflowClaim item14OpenRouterWorkflowClaim
	if err := readExactJSONFile(filepath.Join(workflowDir, "claim.json"), &workflowClaim); err != nil {
		return receipt, err
	}
	if workflowClaim.SchemaVersion != "silver-mediums.openrouter-local-shadow-workflow-claim.v1" || workflowClaim.RunID != definition.RunID || workflowClaim.WorkflowID != plan.CaseID || workflowClaim.Sequence != 1 || workflowClaim.PromptSHA256 != plan.PromptSHA256 || workflowClaim.InlineConfigSHA256 != plan.InlineConfigSHA256 || workflowClaim.Model != plan.Model || workflowClaim.ServingProvider != plan.ServingProvider || workflowClaim.MaxProviderCalls != 1 || workflowClaim.RetryCount != 0 {
		return receipt, fmt.Errorf("Item 14 OpenRouter workflow claim drifted")
	}
	for index := 1; index < len(plans); index++ {
		path := filepath.Join(artifactRoot, fmt.Sprintf("%02d-%s", index+1, plans[index].CaseID))
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			return receipt, fmt.Errorf("Item 14 OpenRouter uncalled workflow %s is present or ambiguous", plans[index].CaseID)
		}
	}

	var halt item14OpenRouterShadowHalt
	if err := readExactJSONFile(filepath.Join(workflowDir, "global-halt.json"), &halt); err != nil {
		return receipt, err
	}
	if halt.Class != "identity_or_response_contract_failure" || !halt.CleanupVerified || halt.ProviderCalls != 1 || halt.RetryAllowed || halt.Error != "OpenRouter local shadow actual model, provider, or one-turn evidence drifted" {
		return receipt, fmt.Errorf("Item 14 OpenRouter terminal halt evidence drifted")
	}

	var rootBefore, callBefore, callAfter agentqualification.APISpendSnapshot
	if err := readExactJSONFile(filepath.Join(artifactRoot, "spend-before.json"), &rootBefore); err != nil {
		return receipt, err
	}
	if err := readExactJSONFile(filepath.Join(workflowDir, "spend-before.json"), &callBefore); err != nil {
		return receipt, err
	}
	if err := readExactJSONFile(filepath.Join(workflowDir, "spend-after.json"), &callAfter); err != nil {
		return receipt, err
	}
	if err := validateItem14OpenRouterSpendSnapshot(rootBefore); err != nil {
		return receipt, err
	}
	if err := validateItem14OpenRouterSpendSnapshot(callBefore); err != nil {
		return receipt, err
	}
	if err := validateItem14OpenRouterSpendSnapshot(callAfter); err != nil {
		return receipt, err
	}
	if rootBefore.SpentMicros != callBefore.SpentMicros || callAfter.SpentMicros < rootBefore.SpentMicros || callAfter.SpentMicros > definition.CumulativeOpenRouterCeilingMicros || callAfter.SpentMicros-rootBefore.SpentMicros > definition.SliceIncrementalCeilingMicros {
		return receipt, fmt.Errorf("Item 14 OpenRouter sealed spend evidence crossed its frozen boundary")
	}

	rawJSONL, err := os.ReadFile(filepath.Join(workflowDir, "provider-run.jsonl"))
	if err != nil {
		return receipt, err
	}
	rawExport, err := os.ReadFile(filepath.Join(workflowDir, "provider-session-export.json"))
	if err != nil {
		return receipt, err
	}
	result, responseRaw, err := agentqualification.ParseOpenCodeRunArtifact(rawJSONL, rawExport)
	if err != nil {
		return receipt, err
	}
	inspection, err := agentqualification.InspectOpenRouterLocalShadowFullSessionV1(rawExport)
	if err != nil {
		return receipt, err
	}
	if inspection.AssistantMessages != 1 || inspection.SummaryAssistants != 0 || inspection.ToolParts != 0 || !reflect.DeepEqual(inspection.Models, []string{plan.Model}) || len(inspection.ServingProviders) != 0 {
		return receipt, fmt.Errorf("Item 14 OpenRouter sealed session is not the exact provider-unobserved terminal case")
	}
	if result.TerminalState != "completed" || result.ReportedModel != plan.Model || result.ReportedServingProvider != "" || result.ObservationID == "" || result.ProviderResponseID == "" || !reflect.DeepEqual(result.ObservedProviderEventTypes, []string{"step_finish", "step_start", "text"}) || len(result.ObservedToolEventTypes) != 0 {
		return receipt, fmt.Errorf("Item 14 OpenRouter sealed provider result drifted")
	}
	workflow := suite.Workflows[0]
	if workflow.ID != plan.CaseID {
		return receipt, fmt.Errorf("Item 14 OpenRouter workflow order drifted")
	}
	_, responseContractErr := agentqualification.ValidateOpenRouterLocalShadowResponseV1(workflow, definition.SuiteSHA256, responseRaw)
	if result.Usage.InputTokens == nil || result.Usage.CachedInputTokens == nil || result.Usage.OutputTokens == nil || result.Usage.ReasoningTokens == nil || result.Usage.ProviderReportedCostMicros == nil || *result.Usage.InputTokens < 0 || *result.Usage.CachedInputTokens < 0 || *result.Usage.OutputTokens < 0 || *result.Usage.ReasoningTokens < 0 || *result.Usage.OutputTokens+*result.Usage.ReasoningTokens > int64(definition.GeneratedTokenCeilingPerCall) || *result.Usage.InputTokens > definition.MaxInputTokensPerCall || *result.Usage.ProviderReportedCostMicros < 0 || *result.Usage.ProviderReportedCostMicros > definition.PerCallReservationMicros || *result.Usage.ProviderReportedCostMicros > definition.SliceIncrementalCeilingMicros {
		return receipt, fmt.Errorf("Item 14 OpenRouter sealed usage evidence drifted")
	}

	startedAt, err := time.Parse(time.RFC3339Nano, result.StartedAt)
	if err != nil {
		return receipt, err
	}
	completedAt, err := time.Parse(time.RFC3339Nano, result.CompletedAt)
	if err != nil || completedAt.Before(startedAt) {
		return receipt, fmt.Errorf("Item 14 OpenRouter trace timing evidence invalid")
	}
	accountingStartedAt, err := time.Parse(time.RFC3339Nano, callBefore.ObservedAt)
	if err != nil {
		return receipt, err
	}
	accountingCompletedAt, err := time.Parse(time.RFC3339Nano, callAfter.ObservedAt)
	if err != nil || accountingCompletedAt.Before(accountingStartedAt) {
		return receipt, fmt.Errorf("Item 14 OpenRouter accounting timing evidence invalid")
	}

	status := "failed_closed_actual_serving_provider_unobserved"
	failureReason := "requested serving provider was pinned with fallbacks disabled, but the actual serving provider was not present in the sealed provider metadata"
	if responseContractErr != nil {
		status = "failed_closed_actual_serving_provider_unobserved_and_response_contract_rejected"
		failureReason += "; the returned JSON also failed the strict frozen response contract"
	}
	receipt = item14OpenRouterTerminalFailureReceipt{
		SchemaVersion: "silver-mediums.openrouter-local-shadow-terminal-failure.v1", Status: status,
		RunID: definition.RunID, WorkflowID: plan.CaseID, DefinitionSHA256: definitionHash, SuiteSHA256: definition.SuiteSHA256,
		AuthorityExhausted: true, RetryAllowed: false, PlannedProviderCalls: definition.MaxProviderCalls, ActualProviderCalls: 1, RemainingWorkflowsNotCalled: len(plans) - 1,
		RequestedModel: plan.Model, ActualModel: result.ReportedModel, RequestedServingProvider: plan.ServingProvider, ActualServingProvider: "", ServingProviderEvidence: "unobserved_no_selected_provider_metadata", ProviderFallbacks: false,
		SessionInspection: inspection, ResponseContractValid: responseContractErr == nil, CleanupVerified: halt.CleanupVerified,
		ToolCallCount: 0, RepositoryAccessCount: 0, ProductionAccessCount: 0, OutboundActionCount: 0, RoleModelSelectionCount: 0,
		ObservationID: result.ObservationID, ProviderResponseID: result.ProviderResponseID, StartedAt: result.StartedAt, CompletedAt: result.CompletedAt,
		TraceDurationMilliseconds: completedAt.Sub(startedAt).Milliseconds(), AccountingWindowMilliseconds: accountingCompletedAt.Sub(accountingStartedAt).Milliseconds(),
		InputTokens: *result.Usage.InputTokens, CachedInputTokens: *result.Usage.CachedInputTokens, OutputTokens: *result.Usage.OutputTokens, ReasoningTokens: *result.Usage.ReasoningTokens, ProviderReportedCostMicros: *result.Usage.ProviderReportedCostMicros,
		RawTraceSHA256: result.RawTraceSHA256, ProviderRunFileSHA256: sha256BytesHex(rawJSONL), ProviderSessionExportFileSHA256: sha256BytesHex(rawExport),
		FailureClass: halt.Class, FailureReason: failureReason,
	}
	if responseContractErr != nil {
		receipt.ResponseContractError = responseContractErr.Error()
	}
	accounting := item14OpenRouterShadowAccounting{
		SchemaVersion: "silver-mediums.openrouter-local-shadow-accounting.v1", Provider: rootBefore.Provider, ProjectLane: rootBefore.ProjectLane, WindowID: rootBefore.WindowID,
		BeforeSpentMicros: rootBefore.SpentMicros, AfterSpentMicros: callAfter.SpentMicros, ObservedIncrementMicros: callAfter.SpentMicros - rootBefore.SpentMicros, ReceiptCostMicros: *result.Usage.ProviderReportedCostMicros,
		SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeOpenRouterCeilingMicros,
		BeforeObservedAt: rootBefore.ObservedAt, AfterObservedAt: callAfter.ObservedAt, Source: callAfter.Source,
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "terminal-failure-receipt.json"), receipt); err != nil {
		return item14OpenRouterTerminalFailureReceipt{}, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "accounting.json"), accounting); err != nil {
		return item14OpenRouterTerminalFailureReceipt{}, err
	}
	return receipt, nil
}

func validateItem14OpenRouterSpendSnapshot(snapshot agentqualification.APISpendSnapshot) error {
	if snapshot.Provider != "openrouter" || snapshot.ProjectLane != "opencode_local_openrouter" || snapshot.Currency != "USD" || snapshot.WindowID != "openrouter_account_lifetime" || snapshot.SpentMicros < 0 || snapshot.Source != "authoritative_api_usage" {
		return fmt.Errorf("Item 14 OpenRouter spend snapshot drifted")
	}
	if _, err := time.Parse(time.RFC3339Nano, snapshot.ObservedAt); err != nil {
		return fmt.Errorf("Item 14 OpenRouter spend timestamp invalid: %w", err)
	}
	return nil
}

func readExactJSONFile(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON evidence %s has trailing data", path)
	}
	return nil
}

func sha256BytesHex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
