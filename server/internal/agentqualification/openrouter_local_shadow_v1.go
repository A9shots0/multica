package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

const (
	OpenRouterLocalShadowDefinitionSchemaV1 = "silver-mediums.openrouter-local-shadow-definition.v1"
	OpenRouterLocalShadowRunSchemaV1        = "silver-mediums.openrouter-local-shadow-run.v1"
	OpenRouterLocalShadowTransportV1        = OpenCodeToolFreeTransportV6
	OpenRouterLocalShadowGeneratedMaxV1     = 1200
)

//go:embed openrouter_local_shadow_v1.json
var openRouterLocalShadowDefinitionRawV1 []byte

type OpenRouterLocalShadowCandidateV1 struct {
	ConfigurationID      string `json:"configuration_id"`
	Model                string `json:"model"`
	ReasoningEffort      string `json:"reasoning_effort"`
	RuntimeProvider      string `json:"runtime_provider"`
	ServingProvider      string `json:"serving_provider"`
	ProviderFallbacks    bool   `json:"provider_fallbacks"`
	TemporaryFixtureOnly bool   `json:"temporary_fixture_only"`
}

type OpenRouterLocalShadowDefinitionV1 struct {
	SchemaVersion                      string                           `json:"schema_version"`
	RunID                              string                           `json:"run_id"`
	Status                             string                           `json:"status"`
	Purpose                            string                           `json:"purpose"`
	SuiteSHA256                        string                           `json:"suite_sha256"`
	EligibilityEvidenceSHA256          string                           `json:"eligibility_evidence_sha256"`
	Candidate                          OpenRouterLocalShadowCandidateV1 `json:"candidate"`
	Transport                          string                           `json:"transport"`
	WorkflowIDs                        []string                         `json:"workflow_ids"`
	MaxProviderCalls                   int                              `json:"max_provider_calls"`
	CallsPerWorkflow                   int                              `json:"calls_per_workflow"`
	RetryCount                         int                              `json:"retry_count"`
	GeneratedTokenCeilingPerCall       int                              `json:"generated_token_ceiling_per_call"`
	MaxInputTokensPerCall              int64                            `json:"max_input_tokens_per_call"`
	InputPriceMicrosPerMillion         int64                            `json:"input_price_micros_per_million"`
	GeneratedPriceMicrosPerMillion     int64                            `json:"generated_price_micros_per_million"`
	PerCallReservationMicros           int64                            `json:"per_call_reservation_micros"`
	TotalPriceReservationMicros        int64                            `json:"total_price_reservation_micros"`
	PriceEvidence                      string                           `json:"price_evidence"`
	CumulativeOpenRouterBaselineMicros int64                            `json:"cumulative_openrouter_baseline_micros"`
	CumulativeOpenRouterCeilingMicros  int64                            `json:"cumulative_openrouter_ceiling_micros"`
	SliceIncrementalCeilingMicros      int64                            `json:"slice_incremental_ceiling_micros"`
	ToolsAllowed                       bool                             `json:"tools_allowed"`
	RepositoryAccessAllowed            bool                             `json:"repository_access_allowed"`
	ProductionAccessAllowed            bool                             `json:"production_access_allowed"`
	OutboundActionsAllowed             bool                             `json:"outbound_actions_allowed"`
	PhoneFarmAccessAllowed             bool                             `json:"phone_farm_access_allowed"`
	RoleModelSelectionAllowed          bool                             `json:"role_model_selection_allowed"`
	StopOnIdentityOrEvidenceFailure    bool                             `json:"stop_on_identity_or_evidence_failure"`
}

type OpenRouterLocalShadowResponseV1 struct {
	WorkflowID              string   `json:"workflow_id"`
	SuiteSHA256             string   `json:"suite_sha256"`
	StageIDs                []string `json:"stage_ids"`
	EvidenceContractIDs     []string `json:"evidence_contract_ids"`
	BoundaryVerdict         string   `json:"boundary_verdict"`
	ProviderActionsPlanned  int      `json:"provider_actions_planned"`
	ToolCallsPlanned        int      `json:"tool_calls_planned"`
	RepositoryAccessPlanned bool     `json:"repository_access_planned"`
	ProductionAccessPlanned bool     `json:"production_access_planned"`
	OutboundActionsPlanned  bool     `json:"outbound_actions_planned"`
	RoleModelSelectionMade  bool     `json:"role_model_selection_made"`
}

type OpenRouterLocalShadowReceiptV1 struct {
	WorkflowID                 string                          `json:"workflow_id"`
	PromptSHA256               string                          `json:"prompt_sha256"`
	SchemaSHA256               string                          `json:"schema_sha256"`
	InlineConfigSHA256         string                          `json:"inline_config_sha256"`
	RequestedModel             string                          `json:"requested_model"`
	ActualModel                string                          `json:"actual_model"`
	RequestedServingProvider   string                          `json:"requested_serving_provider"`
	ActualServingProvider      string                          `json:"actual_serving_provider"`
	ServingProviderEvidence    string                          `json:"serving_provider_evidence"`
	ObservationID              string                          `json:"observation_id"`
	ProviderResponseID         string                          `json:"provider_response_id"`
	StartedAt                  string                          `json:"started_at"`
	CompletedAt                string                          `json:"completed_at"`
	InputTokens                int64                           `json:"input_tokens"`
	CachedInputTokens          int64                           `json:"cached_input_tokens"`
	OutputTokens               int64                           `json:"output_tokens"`
	ReasoningTokens            int64                           `json:"reasoning_tokens"`
	ProviderReportedCostMicros int64                           `json:"provider_reported_cost_micros"`
	ResponseSHA256             string                          `json:"response_sha256"`
	RawTraceSHA256             string                          `json:"raw_trace_sha256"`
	Response                   OpenRouterLocalShadowResponseV1 `json:"response"`
}

type OpenRouterLocalShadowRunV1 struct {
	SchemaVersion           string                           `json:"schema_version"`
	RunID                   string                           `json:"run_id"`
	ExecutionMode           string                           `json:"execution_mode"`
	DefinitionSHA256        string                           `json:"definition_sha256"`
	SuiteSHA256             string                           `json:"suite_sha256"`
	Model                   string                           `json:"model"`
	ServingProvider         string                           `json:"serving_provider"`
	ProviderRequestCount    int                              `json:"provider_request_count"`
	SimulatedResponseCount  int                              `json:"simulated_provider_response_count"`
	RetryCount              int                              `json:"retry_count"`
	ToolCallCount           int                              `json:"tool_call_count"`
	RepositoryAccessCount   int                              `json:"repository_access_count"`
	ProductionAccessCount   int                              `json:"production_access_count"`
	OutboundActionCount     int                              `json:"outbound_action_count"`
	RoleModelSelectionCount int                              `json:"role_model_selection_count"`
	TotalInputTokens        int64                            `json:"total_input_tokens"`
	TotalCachedInputTokens  int64                            `json:"total_cached_input_tokens"`
	TotalOutputTokens       int64                            `json:"total_output_tokens"`
	TotalReasoningTokens    int64                            `json:"total_reasoning_tokens"`
	TotalCostMicros         int64                            `json:"total_cost_micros"`
	Receipts                []OpenRouterLocalShadowReceiptV1 `json:"receipts"`
}

func (ctxContracts QualificationContracts) CurrentOpenRouterLocalShadowDefinitionV1() (OpenRouterLocalShadowDefinitionV1, error) {
	var definition OpenRouterLocalShadowDefinitionV1
	if err := decodeExactJSONV1(openRouterLocalShadowDefinitionRawV1, &definition); err != nil {
		return definition, fmt.Errorf("decode OpenRouter local shadow definition v1: %w", err)
	}
	if err := ctxContracts.ValidateOpenRouterLocalShadowDefinitionV1(definition); err != nil {
		return OpenRouterLocalShadowDefinitionV1{}, err
	}
	return definition, nil
}

func OpenRouterLocalShadowDefinitionV1SHA256() string {
	return sha256Hex(openRouterLocalShadowDefinitionRawV1)
}

func (ctxContracts QualificationContracts) ValidateOpenRouterLocalShadowDefinitionV1(definition OpenRouterLocalShadowDefinitionV1) error {
	wantWorkflows := []string{"local-intake-decision-coordination", "local-implementation-review-mobile", "local-release-regression-rollback", "local-interruption-recovery"}
	if definition.SchemaVersion != OpenRouterLocalShadowDefinitionSchemaV1 || definition.RunID != "item-14-openrouter-local-shadow-v1" || definition.Status != "prepared_frozen_not_launched" || definition.Purpose != "temporary exact-model transport and frozen workflow-boundary proof only" {
		return fmt.Errorf("OpenRouter local shadow definition identity drifted")
	}
	if definition.SuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() || definition.EligibilityEvidenceSHA256 != "84660c35c92e780c3c00f744a2380683accfc0158dea161b8353953247211b46" {
		return fmt.Errorf("OpenRouter local shadow antecedent hashes drifted")
	}
	candidate := definition.Candidate
	if candidate.ConfigurationID != "item14.fixture.qwen3codernext.default.openrouter.v1" || candidate.Model != "openrouter/qwen/qwen3-coder-next" || candidate.ReasoningEffort != "screening_default" || candidate.RuntimeProvider != OpenCodeCLIRuntimeProvider || candidate.ServingProvider != "alibaba" || candidate.ProviderFallbacks || !candidate.TemporaryFixtureOnly {
		return fmt.Errorf("OpenRouter local shadow temporary fixture drifted")
	}
	if err := ValidateOpenRouterEstablishedSmallModelRouteV2(candidate.Model, candidate.ServingProvider); err != nil {
		return err
	}
	if definition.Transport != OpenRouterLocalShadowTransportV1 || !reflect.DeepEqual(definition.WorkflowIDs, wantWorkflows) || definition.MaxProviderCalls != len(wantWorkflows) || definition.CallsPerWorkflow != 1 || definition.RetryCount != 0 || definition.GeneratedTokenCeilingPerCall != OpenRouterLocalShadowGeneratedMaxV1 {
		return fmt.Errorf("OpenRouter local shadow call or transport boundary drifted")
	}
	if definition.MaxInputTokensPerCall != 262144 || definition.InputPriceMicrosPerMillion != 300000 || definition.GeneratedPriceMicrosPerMillion != 1500000 || definition.PerCallReservationMicros != 80444 || definition.TotalPriceReservationMicros != 321776 || definition.TotalPriceReservationMicros != int64(definition.MaxProviderCalls)*definition.PerCallReservationMicros || definition.PriceEvidence != "https://openrouter.ai/provider/alibaba#qwen3-coder-next observed 2026-09-14" {
		return fmt.Errorf("OpenRouter local shadow preventive price reservation drifted")
	}
	inputReserve := (definition.MaxInputTokensPerCall*definition.InputPriceMicrosPerMillion + 999999) / 1000000
	generatedReserve := (int64(definition.GeneratedTokenCeilingPerCall)*definition.GeneratedPriceMicrosPerMillion + 999999) / 1000000
	if inputReserve+generatedReserve != definition.PerCallReservationMicros {
		return fmt.Errorf("OpenRouter local shadow price-derived reservation is inconsistent")
	}
	if definition.CumulativeOpenRouterBaselineMicros != 102104668 || definition.CumulativeOpenRouterCeilingMicros != 142104668 || definition.SliceIncrementalCeilingMicros != 1000000 || definition.CumulativeOpenRouterCeilingMicros-definition.CumulativeOpenRouterBaselineMicros != 40000000 {
		return fmt.Errorf("OpenRouter local shadow spend boundary drifted")
	}
	if definition.ToolsAllowed || definition.RepositoryAccessAllowed || definition.ProductionAccessAllowed || definition.OutboundActionsAllowed || definition.PhoneFarmAccessAllowed || definition.RoleModelSelectionAllowed || !definition.StopOnIdentityOrEvidenceFailure {
		return fmt.Errorf("OpenRouter local shadow opened a forbidden boundary")
	}
	return nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterLocalShadowPlansV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1) ([]OpenCodeScreeningPlan, error) {
	if err := ctxContracts.ValidateOpenRouterLocalShadowDefinitionV1(definition); err != nil {
		return nil, err
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return nil, err
	}
	if definition.SuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() {
		return nil, fmt.Errorf("OpenRouter local shadow suite binding drifted")
	}
	byID := make(map[string]LocalShadowWorkflowV1, len(suite.Workflows))
	for _, workflow := range suite.Workflows {
		byID[workflow.ID] = workflow
	}
	plans := make([]OpenCodeScreeningPlan, 0, len(definition.WorkflowIDs))
	for _, workflowID := range definition.WorkflowIDs {
		workflow, ok := byID[workflowID]
		if !ok {
			return nil, fmt.Errorf("OpenRouter local shadow workflow %s missing", workflowID)
		}
		plan, err := buildOpenRouterLocalShadowPlanV1(definition, workflow)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func buildOpenRouterLocalShadowPlanV1(definition OpenRouterLocalShadowDefinitionV1, workflow LocalShadowWorkflowV1) (OpenCodeScreeningPlan, error) {
	stageIDs, contractIDs := openRouterLocalShadowExpectedIDsV1(workflow)
	packet := struct {
		SuiteSHA256         string   `json:"suite_sha256"`
		WorkflowID          string   `json:"workflow_id"`
		Surfaces            []string `json:"surfaces"`
		SquadID             string   `json:"squad_id"`
		Capability          string   `json:"capability"`
		AccountableRoleID   string   `json:"accountable_role_id"`
		ControllerOwnsMoves bool     `json:"controller_owns_moves"`
		RequiresRelease     bool     `json:"requires_release"`
		StageIDs            []string `json:"stage_ids"`
		EvidenceContractIDs []string `json:"evidence_contract_ids"`
	}{definition.SuiteSHA256, workflow.ID, workflow.Surfaces, workflow.SquadID, workflow.Capability, workflow.AccountableRoleID, workflow.ControllerOwnsMoves, workflow.RequiresRelease, stageIDs, contractIDs}
	packetRaw, err := json.Marshal(packet)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	prompt := "Treat the following as an immutable provider-free workflow contract. Return exactly one JSON object that echoes the workflow_id, suite_sha256, stage_ids, and evidence_contract_ids without additions or reordering. Set boundary_verdict to accepted_frozen_provider_free_contract; provider_actions_planned and tool_calls_planned to 0; and every access/action/selection boolean to false. Do not use tools, files, repositories, network access, prior sessions, production systems, or outbound actions.\n\n<frozen-workflow>\n" + string(packetRaw) + "\n</frozen-workflow>"
	schema := openRouterLocalShadowSchemaV1()
	candidate := definition.Candidate
	modelID := strings.TrimPrefix(candidate.Model, "openrouter/")
	fallbacks := false
	model := openCodeModelConfig{Options: map[string]any{"provider": map[string]any{"order": []string{candidate.ServingProvider}, "allow_fallbacks": false}}, Variants: map[string]map[string]any{"screening_default": {}}}
	config := openCodeQualificationConfig{
		Schema: "https://opencode.ai/config.json", Autoupdate: false, Share: "disabled", Snapshot: false,
		Plugin: []string{}, Instructions: []string{}, Permission: "deny", Tools: map[string]bool{"*": false},
		Compaction: map[string]bool{"auto": false, "prune": false}, DefaultAgent: "qualification",
		Agent: map[string]openCodeQualificationAgent{"qualification": {
			Description: "Pinned Item 14 local-shadow boundary fixture", Mode: "primary", Model: candidate.Model,
			Variant: candidate.ReasoningEffort, Prompt: "Return only the exact JSON response requested by the supplied frozen workflow. No tools or external access.",
			Steps: 2, Permission: "deny", Tools: map[string]bool{"*": false},
		}},
		Provider: map[string]openCodeProviderConfig{"openrouter": {
			Options: map[string]any{"name": "OpenRouter"}, Headers: map[string]string{"X-OpenRouter-Metadata": "enabled"},
			Whitelist: []string{modelID}, Models: map[string]openCodeModelConfig{modelID: model},
		}},
	}
	inlineConfig, err := json.Marshal(config)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	return OpenCodeScreeningPlan{
		Transport: OpenRouterLocalShadowTransportV1, CandidateID: candidate.ConfigurationID, CaseID: workflow.ID, CaseVersion: "local-shadow.v1",
		Model: candidate.Model, Effort: candidate.ReasoningEffort, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)),
		Schema: schema, SchemaSHA256: sha256Hex(schema), InlineConfig: inlineConfig, InlineConfigSHA256: sha256Hex(inlineConfig),
		ArgsTemplate:        []string{"--pure", "run", "--format", "json", "--model", candidate.Model, "--variant", candidate.ReasoningEffort, "--agent", "qualification", "--title", "item14-local-shadow-<workflow-id>", "--dir", "<ephemeral-empty-dir>", "<exact-prompt-via-stdin>"},
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", OpenRouterLocalShadowGeneratedMaxV1)},
		ServingProvider:     candidate.ServingProvider, ProviderFallbacks: &fallbacks,
	}, nil
}

func openRouterLocalShadowSchemaV1() []byte {
	schema := map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"workflow_id", "suite_sha256", "stage_ids", "evidence_contract_ids", "boundary_verdict", "provider_actions_planned", "tool_calls_planned", "repository_access_planned", "production_access_planned", "outbound_actions_planned", "role_model_selection_made"},
		"properties": map[string]any{
			"workflow_id": map[string]any{"type": "string"}, "suite_sha256": map[string]any{"type": "string"},
			"stage_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "evidence_contract_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"boundary_verdict":         map[string]any{"type": "string", "enum": []string{"accepted_frozen_provider_free_contract"}},
			"provider_actions_planned": map[string]any{"type": "integer", "maximum": 0}, "tool_calls_planned": map[string]any{"type": "integer", "maximum": 0},
			"repository_access_planned": map[string]any{"type": "boolean"}, "production_access_planned": map[string]any{"type": "boolean"}, "outbound_actions_planned": map[string]any{"type": "boolean"}, "role_model_selection_made": map[string]any{"type": "boolean"},
		},
	}
	raw, _ := json.Marshal(schema)
	return raw
}

func ValidateOpenRouterLocalShadowResponseV1(workflow LocalShadowWorkflowV1, suiteSHA string, raw []byte) (OpenRouterLocalShadowResponseV1, error) {
	var response OpenRouterLocalShadowResponseV1
	if err := decodeExactJSONV1(raw, &response); err != nil {
		return response, fmt.Errorf("decode OpenRouter local shadow response: %w", err)
	}
	stageIDs, contractIDs := openRouterLocalShadowExpectedIDsV1(workflow)
	if response.WorkflowID != workflow.ID || response.SuiteSHA256 != suiteSHA || !reflect.DeepEqual(response.StageIDs, stageIDs) || !reflect.DeepEqual(response.EvidenceContractIDs, contractIDs) || response.BoundaryVerdict != "accepted_frozen_provider_free_contract" {
		return OpenRouterLocalShadowResponseV1{}, fmt.Errorf("OpenRouter local shadow response drifted from frozen workflow")
	}
	if response.ProviderActionsPlanned != 0 || response.ToolCallsPlanned != 0 || response.RepositoryAccessPlanned || response.ProductionAccessPlanned || response.OutboundActionsPlanned || response.RoleModelSelectionMade {
		return OpenRouterLocalShadowResponseV1{}, fmt.Errorf("OpenRouter local shadow response opened a forbidden boundary")
	}
	return response, nil
}

type OpenRouterLocalShadowSessionInspectionV1 struct {
	AssistantMessages int      `json:"assistant_messages"`
	SummaryAssistants int      `json:"summary_assistants"`
	Models            []string `json:"models"`
	ServingProviders  []string `json:"serving_providers"`
	ToolParts         int      `json:"tool_parts"`
}

func InspectOpenRouterLocalShadowFullSessionV1(rawExport []byte) (OpenRouterLocalShadowSessionInspectionV1, error) {
	var inspection OpenRouterLocalShadowSessionInspectionV1
	decoder := json.NewDecoder(bytes.NewReader(rawExport))
	var exported struct {
		Messages []struct {
			Info struct {
				Role       string `json:"role"`
				ModelID    string `json:"modelID"`
				ProviderID string `json:"providerID"`
				Summary    bool   `json:"summary"`
			} `json:"info"`
			Parts []struct {
				Type     string          `json:"type"`
				Metadata json.RawMessage `json:"metadata"`
			} `json:"parts"`
		} `json:"messages"`
	}
	if err := decoder.Decode(&exported); err != nil {
		return inspection, fmt.Errorf("decode OpenRouter local-shadow full session: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return inspection, fmt.Errorf("OpenRouter local-shadow full session has trailing data")
	}
	models, providers := map[string]struct{}{}, map[string]struct{}{}
	for _, message := range exported.Messages {
		if message.Info.Role != "assistant" {
			continue
		}
		inspection.AssistantMessages++
		if message.Info.Summary {
			inspection.SummaryAssistants++
		}
		if message.Info.ProviderID != "" || message.Info.ModelID != "" {
			models[message.Info.ProviderID+"/"+message.Info.ModelID] = struct{}{}
		}
		for _, part := range message.Parts {
			if part.Type == "tool" {
				inspection.ToolParts++
			}
			for _, provider := range openRouterSelectedProviders(part.Metadata) {
				providers[provider] = struct{}{}
			}
		}
	}
	for model := range models {
		inspection.Models = append(inspection.Models, model)
	}
	for provider := range providers {
		inspection.ServingProviders = append(inspection.ServingProviders, provider)
	}
	sort.Strings(inspection.Models)
	sort.Strings(inspection.ServingProviders)
	return inspection, nil
}

func ValidateOpenRouterLocalShadowFullSessionV1(plan OpenCodeScreeningPlan, rawExport []byte) error {
	inspection, err := InspectOpenRouterLocalShadowFullSessionV1(rawExport)
	if err != nil {
		return err
	}
	if inspection.AssistantMessages != 1 {
		return fmt.Errorf("OpenRouter local-shadow full session must contain exactly one assistant provider response")
	}
	if inspection.SummaryAssistants != 0 || inspection.ToolParts != 0 || !reflect.DeepEqual(inspection.Models, []string{plan.Model}) {
		return fmt.Errorf("OpenRouter local-shadow full session contains background, summary, tool, or wrong-model assistant activity")
	}
	if !reflect.DeepEqual(inspection.ServingProviders, []string{plan.ServingProvider}) {
		return fmt.Errorf("OpenRouter local-shadow full session lacks the exact selected serving provider")
	}
	return nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterLocalShadowReceiptV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1, plan OpenCodeScreeningPlan, result ProviderExecutionResult, responseRaw, rawExport []byte) (OpenRouterLocalShadowReceiptV1, error) {
	var receipt OpenRouterLocalShadowReceiptV1
	if err := ctxContracts.ValidateOpenRouterLocalShadowDefinitionV1(definition); err != nil {
		return receipt, err
	}
	var workflow *LocalShadowWorkflowV1
	for index := range suite.Workflows {
		if suite.Workflows[index].ID == plan.CaseID {
			workflow = &suite.Workflows[index]
			break
		}
	}
	if workflow == nil || plan.Transport != definition.Transport || plan.Model != definition.Candidate.Model || plan.ServingProvider != definition.Candidate.ServingProvider || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks || plan.CandidateID != definition.Candidate.ConfigurationID {
		return receipt, fmt.Errorf("OpenRouter local shadow plan identity drifted")
	}
	if result.TerminalState != "completed" || result.ReportedModel != plan.Model || result.ReportedServingProvider != plan.ServingProvider || result.ObservationID == "" || result.ProviderResponseID == "" || !reflect.DeepEqual(result.ObservedProviderEventTypes, []string{"step_finish", "step_start", "text"}) {
		return receipt, fmt.Errorf("OpenRouter local shadow actual model, provider, or one-turn evidence drifted")
	}
	if err := ValidateOpenRouterLocalShadowFullSessionV1(plan, rawExport); err != nil {
		return receipt, err
	}
	if result.Usage.InputTokens == nil || result.Usage.CachedInputTokens == nil || result.Usage.OutputTokens == nil || result.Usage.ReasoningTokens == nil || result.Usage.ProviderReportedCostMicros == nil || *result.Usage.InputTokens < 0 || *result.Usage.CachedInputTokens < 0 || *result.Usage.OutputTokens < 0 || *result.Usage.ReasoningTokens < 0 || *result.Usage.ProviderReportedCostMicros < 0 {
		return receipt, fmt.Errorf("OpenRouter local shadow usage evidence incomplete")
	}
	if *result.Usage.InputTokens > definition.MaxInputTokensPerCall || *result.Usage.OutputTokens+*result.Usage.ReasoningTokens > int64(definition.GeneratedTokenCeilingPerCall) || *result.Usage.ProviderReportedCostMicros > definition.PerCallReservationMicros {
		return receipt, fmt.Errorf("OpenRouter local shadow per-call token or cost reservation exceeded")
	}
	response, err := ValidateOpenRouterLocalShadowResponseV1(*workflow, definition.SuiteSHA256, responseRaw)
	if err != nil {
		return receipt, err
	}
	canonicalResponse, err := json.Marshal(response)
	if err != nil {
		return receipt, err
	}
	return OpenRouterLocalShadowReceiptV1{
		WorkflowID: workflow.ID, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256, InlineConfigSHA256: plan.InlineConfigSHA256,
		RequestedModel: plan.Model, ActualModel: result.ReportedModel, RequestedServingProvider: plan.ServingProvider, ActualServingProvider: result.ReportedServingProvider, ServingProviderEvidence: "openrouter_selected_provider_metadata",
		ObservationID: result.ObservationID, ProviderResponseID: result.ProviderResponseID, StartedAt: result.StartedAt, CompletedAt: result.CompletedAt,
		InputTokens: *result.Usage.InputTokens, CachedInputTokens: *result.Usage.CachedInputTokens, OutputTokens: *result.Usage.OutputTokens, ReasoningTokens: *result.Usage.ReasoningTokens, ProviderReportedCostMicros: *result.Usage.ProviderReportedCostMicros,
		ResponseSHA256: sha256Hex(canonicalResponse), RawTraceSHA256: result.RawTraceSHA256, Response: response,
	}, nil
}

func (ctxContracts QualificationContracts) RunOpenRouterLocalShadowFakeV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1) (OpenRouterLocalShadowRunV1, error) {
	plans, err := ctxContracts.BuildOpenRouterLocalShadowPlansV1(definition, suite)
	if err != nil {
		return OpenRouterLocalShadowRunV1{}, err
	}
	receipts := make([]OpenRouterLocalShadowReceiptV1, 0, len(plans))
	for index, plan := range plans {
		workflow := suite.Workflows[index]
		stageIDs, contractIDs := openRouterLocalShadowExpectedIDsV1(workflow)
		response := OpenRouterLocalShadowResponseV1{WorkflowID: workflow.ID, SuiteSHA256: definition.SuiteSHA256, StageIDs: stageIDs, EvidenceContractIDs: contractIDs, BoundaryVerdict: "accepted_frozen_provider_free_contract"}
		raw, _ := json.Marshal(response)
		input, cached, output, reasoning, cost := int64(100+index), int64(0), int64(20+index), int64(0), int64(10+index)
		result := ProviderExecutionResult{ObservationID: fmt.Sprintf("fake:item14:%d", index), ReportedModel: plan.Model, ReportedServingProvider: plan.ServingProvider, ProviderResponseID: fmt.Sprintf("fake-response-%d", index), StartedAt: fmt.Sprintf("2026-09-14T12:00:0%dZ", index), CompletedAt: fmt.Sprintf("2026-09-14T12:00:1%dZ", index), TerminalState: "completed", ObservedProviderEventTypes: []string{"step_finish", "step_start", "text"}, RawTraceSHA256: strings.Repeat(fmt.Sprintf("%x", index+1), 64)[:64], Usage: ExecutionReceiptUsage{InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output, ReasoningTokens: &reasoning, ProviderReportedCostMicros: &cost}}
		rawExport := fakeOpenRouterLocalShadowExportV1(plan)
		receipt, err := ctxContracts.BuildOpenRouterLocalShadowReceiptV1(definition, suite, plan, result, raw, rawExport)
		if err != nil {
			return OpenRouterLocalShadowRunV1{}, err
		}
		receipts = append(receipts, receipt)
	}
	run := buildOpenRouterLocalShadowRunV1(definition, receipts)
	run.ExecutionMode = "deterministic_fake_provider_free"
	run.ProviderRequestCount = 0
	run.SimulatedResponseCount = len(receipts)
	if err := ctxContracts.ValidateOpenRouterLocalShadowRunV1(definition, suite, run); err != nil {
		return OpenRouterLocalShadowRunV1{}, err
	}
	return run, nil
}

func fakeOpenRouterLocalShadowExportV1(plan OpenCodeScreeningPlan) []byte {
	value := map[string]any{
		"info": map[string]any{"id": "fake-session"},
		"messages": []any{
			map[string]any{
				"info":  map[string]any{"role": "user", "summary": false},
				"parts": []any{map[string]any{"type": "text"}},
			},
			map[string]any{
				"info":  map[string]any{"role": "assistant", "providerID": "openrouter", "modelID": strings.TrimPrefix(plan.Model, "openrouter/"), "summary": false},
				"parts": []any{map[string]any{"type": "text", "metadata": map[string]any{"openrouter": map[string]any{"selected": true, "provider": plan.ServingProvider}}}},
			},
		},
	}
	raw, _ := json.Marshal(value)
	return raw
}

func (ctxContracts QualificationContracts) BuildOpenRouterLocalShadowRunV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1, receipts []OpenRouterLocalShadowReceiptV1) (OpenRouterLocalShadowRunV1, error) {
	run := buildOpenRouterLocalShadowRunV1(definition, receipts)
	run.ExecutionMode = "real_model_shadow"
	run.ProviderRequestCount = len(receipts)
	if err := ctxContracts.ValidateOpenRouterLocalShadowRunV1(definition, suite, run); err != nil {
		return OpenRouterLocalShadowRunV1{}, err
	}
	return run, nil
}

func buildOpenRouterLocalShadowRunV1(definition OpenRouterLocalShadowDefinitionV1, receipts []OpenRouterLocalShadowReceiptV1) OpenRouterLocalShadowRunV1 {
	run := OpenRouterLocalShadowRunV1{SchemaVersion: OpenRouterLocalShadowRunSchemaV1, RunID: definition.RunID, DefinitionSHA256: OpenRouterLocalShadowDefinitionV1SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, Receipts: append([]OpenRouterLocalShadowReceiptV1(nil), receipts...)}
	for _, receipt := range receipts {
		run.TotalInputTokens += receipt.InputTokens
		run.TotalCachedInputTokens += receipt.CachedInputTokens
		run.TotalOutputTokens += receipt.OutputTokens
		run.TotalReasoningTokens += receipt.ReasoningTokens
		run.TotalCostMicros += receipt.ProviderReportedCostMicros
	}
	return run
}

func (ctxContracts QualificationContracts) ValidateOpenRouterLocalShadowRunV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1, run OpenRouterLocalShadowRunV1) error {
	if err := ctxContracts.ValidateOpenRouterLocalShadowDefinitionV1(definition); err != nil {
		return err
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return err
	}
	if run.SchemaVersion != OpenRouterLocalShadowRunSchemaV1 || run.RunID != definition.RunID || run.DefinitionSHA256 != OpenRouterLocalShadowDefinitionV1SHA256() || run.SuiteSHA256 != definition.SuiteSHA256 || run.Model != definition.Candidate.Model || run.ServingProvider != definition.Candidate.ServingProvider {
		return fmt.Errorf("OpenRouter local shadow run identity drifted")
	}
	wantProviderRequests, wantSimulated := definition.MaxProviderCalls, 0
	if run.ExecutionMode == "deterministic_fake_provider_free" {
		wantProviderRequests, wantSimulated = 0, definition.MaxProviderCalls
	} else if run.ExecutionMode != "real_model_shadow" {
		return fmt.Errorf("OpenRouter local shadow execution mode drifted")
	}
	if run.ProviderRequestCount != wantProviderRequests || run.SimulatedResponseCount != wantSimulated || run.RetryCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.RoleModelSelectionCount != 0 || len(run.Receipts) != definition.MaxProviderCalls {
		return fmt.Errorf("OpenRouter local shadow run count or boundary drifted")
	}
	plans, err := ctxContracts.BuildOpenRouterLocalShadowPlansV1(definition, suite)
	if err != nil {
		return err
	}
	seenObservation, seenResponse := map[string]struct{}{}, map[string]struct{}{}
	var input, cached, output, reasoning, cost int64
	for index, receipt := range run.Receipts {
		if receipt.WorkflowID != definition.WorkflowIDs[index] || receipt.RequestedModel != definition.Candidate.Model || receipt.ActualModel != definition.Candidate.Model || receipt.RequestedServingProvider != definition.Candidate.ServingProvider || receipt.ActualServingProvider != definition.Candidate.ServingProvider || receipt.ServingProviderEvidence != "openrouter_selected_provider_metadata" || receipt.PromptSHA256 != plans[index].PromptSHA256 || receipt.SchemaSHA256 != plans[index].SchemaSHA256 || receipt.InlineConfigSHA256 != plans[index].InlineConfigSHA256 || !validSHA256(receipt.ResponseSHA256) || !validSHA256(receipt.RawTraceSHA256) {
			return fmt.Errorf("OpenRouter local shadow receipt %d drifted", index)
		}
		if _, ok := seenObservation[receipt.ObservationID]; ok || receipt.ObservationID == "" {
			return fmt.Errorf("OpenRouter local shadow duplicate or missing observation")
		}
		seenObservation[receipt.ObservationID] = struct{}{}
		if _, ok := seenResponse[receipt.ProviderResponseID]; ok || receipt.ProviderResponseID == "" {
			return fmt.Errorf("OpenRouter local shadow duplicate or missing provider response")
		}
		seenResponse[receipt.ProviderResponseID] = struct{}{}
		raw, _ := json.Marshal(receipt.Response)
		if sha256Hex(raw) != receipt.ResponseSHA256 {
			return fmt.Errorf("OpenRouter local shadow response hash drifted")
		}
		if _, err := ValidateOpenRouterLocalShadowResponseV1(suite.Workflows[index], definition.SuiteSHA256, raw); err != nil {
			return err
		}
		input += receipt.InputTokens
		cached += receipt.CachedInputTokens
		output += receipt.OutputTokens
		reasoning += receipt.ReasoningTokens
		cost += receipt.ProviderReportedCostMicros
	}
	if run.TotalInputTokens != input || run.TotalCachedInputTokens != cached || run.TotalOutputTokens != output || run.TotalReasoningTokens != reasoning || run.TotalCostMicros != cost || run.TotalCostMicros > definition.SliceIncrementalCeilingMicros {
		return fmt.Errorf("OpenRouter local shadow aggregate accounting drifted")
	}
	return nil
}

func openRouterLocalShadowExpectedIDsV1(workflow LocalShadowWorkflowV1) ([]string, []string) {
	assignments := append([]LocalShadowStageAssignmentV1{workflow.Intake}, workflow.Stages...)
	stages := make([]string, 0, len(assignments))
	contracts := make([]string, 0)
	seen := map[string]struct{}{}
	for _, assignment := range assignments {
		stages = append(stages, assignment.Stage)
		for _, contractID := range assignment.EvidenceContractIDs {
			if _, ok := seen[contractID]; ok {
				continue
			}
			seen[contractID] = struct{}{}
			contracts = append(contracts, contractID)
		}
	}
	return stages, contracts
}

func decodeExactJSONV1(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON contains trailing data")
	}
	return nil
}
