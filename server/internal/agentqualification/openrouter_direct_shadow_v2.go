package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
)

const (
	OpenRouterDirectShadowDefinitionSchemaV2 = "silver-mediums.openrouter-direct-shadow-definition.v2"
	OpenRouterDirectShadowRunSchemaV2        = "silver-mediums.openrouter-direct-shadow-run.v2"
	OpenRouterDirectShadowObservationV2      = "silver-mediums.openrouter-direct-shadow-observation.v2"
	OpenRouterDirectShadowTransportV2        = "openrouter_chat_completions_direct_router_metadata_v2"
)

//go:embed openrouter_direct_shadow_v2.json
var openRouterDirectShadowDefinitionRawV2 []byte

type OpenRouterDirectShadowCandidateV2 struct {
	ConfigurationID      string `json:"configuration_id"`
	Model                string `json:"model"`
	APIModel             string `json:"api_model"`
	ReasoningEffort      string `json:"reasoning_effort"`
	RuntimeProvider      string `json:"runtime_provider"`
	ServingProvider      string `json:"serving_provider"`
	ProviderFallbacks    bool   `json:"provider_fallbacks"`
	TemporaryFixtureOnly bool   `json:"temporary_fixture_only"`
}

type OpenRouterDirectShadowDefinitionV2 struct {
	SchemaVersion                      string                            `json:"schema_version"`
	RunID                              string                            `json:"run_id"`
	Status                             string                            `json:"status"`
	Purpose                            string                            `json:"purpose"`
	SuiteSHA256                        string                            `json:"suite_sha256"`
	EligibilityEvidenceSHA256          string                            `json:"eligibility_evidence_sha256"`
	Candidate                          OpenRouterDirectShadowCandidateV2 `json:"candidate"`
	Endpoint                           string                            `json:"endpoint"`
	Transport                          string                            `json:"transport"`
	WorkflowIDs                        []string                          `json:"workflow_ids"`
	MaxProviderCalls                   int                               `json:"max_provider_calls"`
	CallsPerWorkflow                   int                               `json:"calls_per_workflow"`
	RetryCount                         int                               `json:"retry_count"`
	RedirectsAllowed                   bool                              `json:"redirects_allowed"`
	ProviderOrderExact                 bool                              `json:"provider_order_exact"`
	ProviderOnlyExact                  bool                              `json:"provider_only_exact"`
	RequireParameters                  bool                              `json:"require_parameters"`
	RouterMetadataRequired             bool                              `json:"router_metadata_required"`
	GeneratedTokenCeilingPerCall       int                               `json:"generated_token_ceiling_per_call"`
	MaxInputTokensPerCall              int64                             `json:"max_input_tokens_per_call"`
	InputPriceMicrosPerMillion         int64                             `json:"input_price_micros_per_million"`
	GeneratedPriceMicrosPerMillion     int64                             `json:"generated_price_micros_per_million"`
	PerCallReservationMicros           int64                             `json:"per_call_reservation_micros"`
	TotalPriceReservationMicros        int64                             `json:"total_price_reservation_micros"`
	PriceEvidence                      string                            `json:"price_evidence"`
	CumulativeOpenRouterBaselineMicros int64                             `json:"cumulative_openrouter_baseline_micros"`
	CumulativeOpenRouterCeilingMicros  int64                             `json:"cumulative_openrouter_ceiling_micros"`
	SliceIncrementalCeilingMicros      int64                             `json:"slice_incremental_ceiling_micros"`
	ResponseContractFailurePolicy      string                            `json:"response_contract_failure_policy"`
	IdentityOrProviderFailurePolicy    string                            `json:"identity_or_provider_failure_policy"`
	ToolsAllowed                       bool                              `json:"tools_allowed"`
	RepositoryAccessAllowed            bool                              `json:"repository_access_allowed"`
	ProductionAccessAllowed            bool                              `json:"production_access_allowed"`
	OutboundActionsAllowed             bool                              `json:"outbound_actions_allowed"`
	PhoneFarmAccessAllowed             bool                              `json:"phone_farm_access_allowed"`
	RoleModelSelectionAllowed          bool                              `json:"role_model_selection_allowed"`
}

type OpenRouterDirectShadowPlanV2 struct {
	WorkflowID       string `json:"workflow_id"`
	Model            string `json:"model"`
	APIModel         string `json:"api_model"`
	ServingProvider  string `json:"serving_provider"`
	Prompt           string `json:"-"`
	PromptSHA256     string `json:"prompt_sha256"`
	Schema           []byte `json:"-"`
	SchemaSHA256     string `json:"schema_sha256"`
	RequestBody      []byte `json:"-"`
	RequestSHA256    string `json:"request_sha256"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
	ProviderFallback bool   `json:"provider_fallbacks"`
}

type OpenRouterDirectShadowObservationV2Record struct {
	SchemaVersion              string                           `json:"schema_version"`
	WorkflowID                 string                           `json:"workflow_id"`
	Status                     string                           `json:"status"`
	PromptSHA256               string                           `json:"prompt_sha256"`
	SchemaSHA256               string                           `json:"schema_sha256"`
	RequestSHA256              string                           `json:"request_sha256"`
	ResponseSHA256             string                           `json:"response_sha256"`
	ProviderResponseID         string                           `json:"provider_response_id"`
	RequestedModel             string                           `json:"requested_model"`
	ActualModel                string                           `json:"actual_model"`
	RequestedServingProvider   string                           `json:"requested_serving_provider"`
	ActualServingProvider      string                           `json:"actual_serving_provider"`
	RouterAttemptCount         int                              `json:"router_attempt_count"`
	ProviderRequestCount       int                              `json:"provider_request_count"`
	RetryCount                 int                              `json:"retry_count"`
	InputTokens                int64                            `json:"input_tokens"`
	CachedInputTokens          int64                            `json:"cached_input_tokens"`
	OutputTokens               int64                            `json:"output_tokens"`
	ReasoningTokens            int64                            `json:"reasoning_tokens"`
	ProviderReportedCostMicros int64                            `json:"provider_reported_cost_micros"`
	StartedAt                  string                           `json:"started_at"`
	CompletedAt                string                           `json:"completed_at"`
	DurationMillis             int64                            `json:"duration_millis"`
	ResponseContractValid      bool                             `json:"response_contract_valid"`
	ResponseContractError      string                           `json:"response_contract_error,omitempty"`
	Response                   *OpenRouterLocalShadowResponseV1 `json:"response,omitempty"`
}

type OpenRouterDirectShadowRunV2 struct {
	SchemaVersion           string                                      `json:"schema_version"`
	RunID                   string                                      `json:"run_id"`
	ExecutionMode           string                                      `json:"execution_mode"`
	DefinitionSHA256        string                                      `json:"definition_sha256"`
	SuiteSHA256             string                                      `json:"suite_sha256"`
	Model                   string                                      `json:"model"`
	ServingProvider         string                                      `json:"serving_provider"`
	ProviderRequestCount    int                                         `json:"provider_request_count"`
	SimulatedResponseCount  int                                         `json:"simulated_provider_response_count"`
	CandidatePassCount      int                                         `json:"candidate_pass_count"`
	CandidateFailureCount   int                                         `json:"candidate_failure_count"`
	RetryCount              int                                         `json:"retry_count"`
	ToolCallCount           int                                         `json:"tool_call_count"`
	RepositoryAccessCount   int                                         `json:"repository_access_count"`
	ProductionAccessCount   int                                         `json:"production_access_count"`
	OutboundActionCount     int                                         `json:"outbound_action_count"`
	RoleModelSelectionCount int                                         `json:"role_model_selection_count"`
	TotalInputTokens        int64                                       `json:"total_input_tokens"`
	TotalCachedInputTokens  int64                                       `json:"total_cached_input_tokens"`
	TotalOutputTokens       int64                                       `json:"total_output_tokens"`
	TotalReasoningTokens    int64                                       `json:"total_reasoning_tokens"`
	TotalCostMicros         int64                                       `json:"total_cost_micros"`
	Observations            []OpenRouterDirectShadowObservationV2Record `json:"observations"`
}

func (ctxContracts QualificationContracts) CurrentOpenRouterDirectShadowDefinitionV2() (OpenRouterDirectShadowDefinitionV2, error) {
	var definition OpenRouterDirectShadowDefinitionV2
	if err := decodeExactJSONV1(openRouterDirectShadowDefinitionRawV2, &definition); err != nil {
		return definition, fmt.Errorf("decode OpenRouter direct shadow definition v2: %w", err)
	}
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV2(definition); err != nil {
		return OpenRouterDirectShadowDefinitionV2{}, err
	}
	return definition, nil
}

func OpenRouterDirectShadowDefinitionV2SHA256() string {
	return sha256Hex(openRouterDirectShadowDefinitionRawV2)
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowDefinitionV2(definition OpenRouterDirectShadowDefinitionV2) error {
	wantWorkflows := []string{"local-intake-decision-coordination", "local-implementation-review-mobile", "local-release-regression-rollback", "local-interruption-recovery"}
	if definition.SchemaVersion != OpenRouterDirectShadowDefinitionSchemaV2 || definition.RunID != "item-14-openrouter-direct-shadow-v2" || definition.Status != "prepared_frozen_not_launched" || definition.Purpose != "versioned direct exact-model and exact-serving-provider proof across the frozen local shadow suite" {
		return fmt.Errorf("OpenRouter direct shadow definition identity drifted")
	}
	if definition.SuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() || definition.EligibilityEvidenceSHA256 != "84660c35c92e780c3c00f744a2380683accfc0158dea161b8353953247211b46" {
		return fmt.Errorf("OpenRouter direct shadow antecedent hashes drifted")
	}
	candidate := definition.Candidate
	if candidate.ConfigurationID != "item14.fixture.qwen3codernext.direct.openrouter.v2" || candidate.Model != "openrouter/qwen/qwen3-coder-next" || candidate.APIModel != "qwen/qwen3-coder-next" || candidate.ReasoningEffort != "screening_default" || candidate.RuntimeProvider != "openrouter_direct_api" || candidate.ServingProvider != "alibaba" || candidate.ProviderFallbacks || !candidate.TemporaryFixtureOnly {
		return fmt.Errorf("OpenRouter direct shadow candidate drifted")
	}
	if err := ValidateOpenRouterEstablishedSmallModelRouteV2(candidate.Model, candidate.ServingProvider); err != nil {
		return err
	}
	if strings.TrimPrefix(candidate.Model, "openrouter/") != candidate.APIModel || definition.Endpoint != "https://openrouter.ai/api/v1/chat/completions" || definition.Transport != OpenRouterDirectShadowTransportV2 || !reflect.DeepEqual(definition.WorkflowIDs, wantWorkflows) {
		return fmt.Errorf("OpenRouter direct shadow route or workflow drifted")
	}
	if definition.MaxProviderCalls != len(wantWorkflows) || definition.CallsPerWorkflow != 1 || definition.RetryCount != 0 || definition.RedirectsAllowed || !definition.ProviderOrderExact || !definition.ProviderOnlyExact || !definition.RequireParameters || !definition.RouterMetadataRequired || definition.GeneratedTokenCeilingPerCall != 1200 {
		return fmt.Errorf("OpenRouter direct shadow call boundary drifted")
	}
	if definition.MaxInputTokensPerCall != 262144 || definition.InputPriceMicrosPerMillion != 300000 || definition.GeneratedPriceMicrosPerMillion != 1500000 || definition.PerCallReservationMicros != 80444 || definition.TotalPriceReservationMicros != 321776 || definition.TotalPriceReservationMicros != int64(definition.MaxProviderCalls)*definition.PerCallReservationMicros || definition.PriceEvidence != "https://openrouter.ai/provider/alibaba#qwen3-coder-next observed 2026-09-14" {
		return fmt.Errorf("OpenRouter direct shadow price reservation drifted")
	}
	inputReserve := (definition.MaxInputTokensPerCall*definition.InputPriceMicrosPerMillion + 999999) / 1000000
	outputReserve := (int64(definition.GeneratedTokenCeilingPerCall)*definition.GeneratedPriceMicrosPerMillion + 999999) / 1000000
	if inputReserve+outputReserve != definition.PerCallReservationMicros || definition.CumulativeOpenRouterBaselineMicros != 102104668 || definition.CumulativeOpenRouterCeilingMicros != 142104668 || definition.SliceIncrementalCeilingMicros != 1000000 || definition.CumulativeOpenRouterCeilingMicros-definition.CumulativeOpenRouterBaselineMicros != 40000000 {
		return fmt.Errorf("OpenRouter direct shadow spend boundary drifted")
	}
	if definition.ResponseContractFailurePolicy != "candidate_local_continue" || definition.IdentityOrProviderFailurePolicy != "global_halt_no_retry" || definition.ToolsAllowed || definition.RepositoryAccessAllowed || definition.ProductionAccessAllowed || definition.OutboundActionsAllowed || definition.PhoneFarmAccessAllowed || definition.RoleModelSelectionAllowed {
		return fmt.Errorf("OpenRouter direct shadow failure or capability policy drifted")
	}
	return nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowPlansV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1) ([]OpenRouterDirectShadowPlanV2, error) {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV2(definition); err != nil {
		return nil, err
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return nil, err
	}
	if definition.SuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() {
		return nil, fmt.Errorf("OpenRouter direct shadow suite binding drifted")
	}
	byID := make(map[string]LocalShadowWorkflowV1, len(suite.Workflows))
	for _, workflow := range suite.Workflows {
		byID[workflow.ID] = workflow
	}
	plans := make([]OpenRouterDirectShadowPlanV2, 0, len(definition.WorkflowIDs))
	for _, workflowID := range definition.WorkflowIDs {
		workflow, ok := byID[workflowID]
		if !ok {
			return nil, fmt.Errorf("OpenRouter direct shadow workflow %s missing", workflowID)
		}
		plan, err := ctxContracts.buildOpenRouterDirectShadowPlanV2(definition, workflow)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (ctxContracts QualificationContracts) buildOpenRouterDirectShadowPlanV2(definition OpenRouterDirectShadowDefinitionV2, workflow LocalShadowWorkflowV1) (OpenRouterDirectShadowPlanV2, error) {
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
		return OpenRouterDirectShadowPlanV2{}, err
	}
	prompt := "Treat the following as an immutable provider-free workflow contract. Return exactly one JSON object that echoes the workflow_id, suite_sha256, stage_ids, and evidence_contract_ids without additions or reordering. Set boundary_verdict to accepted_frozen_provider_free_contract; provider_actions_planned and tool_calls_planned to 0; and every access/action/selection boolean to false. Do not use tools, files, repositories, network access, prior sessions, production systems, or outbound actions.\n\n<frozen-workflow>\n" + string(packetRaw) + "\n</frozen-workflow>"
	schema := openRouterLocalShadowSchemaV1()
	request := map[string]any{
		"model": definition.Candidate.APIModel,
		"messages": []map[string]string{
			{"role": "system", "content": "Return only the exact JSON object requested by the supplied frozen workflow. No tools or external access."},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  definition.GeneratedTokenCeilingPerCall,
		"temperature": 0,
		"stream":      false,
		"provider": map[string]any{
			"order":              []string{definition.Candidate.ServingProvider},
			"only":               []string{definition.Candidate.ServingProvider},
			"allow_fallbacks":    false,
			"require_parameters": true,
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "item14_local_shadow_v2",
				"strict": true,
				"schema": json.RawMessage(schema),
			},
		},
	}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return OpenRouterDirectShadowPlanV2{}, err
	}
	plan := OpenRouterDirectShadowPlanV2{
		WorkflowID: workflow.ID, Model: definition.Candidate.Model, APIModel: definition.Candidate.APIModel,
		ServingProvider: definition.Candidate.ServingProvider, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)),
		Schema: schema, SchemaSHA256: sha256Hex(schema), RequestBody: requestBody, RequestSHA256: sha256Hex(requestBody),
		MaxOutputTokens: definition.GeneratedTokenCeilingPerCall, ProviderFallback: false,
	}
	if err := ctxContracts.ValidateOpenRouterDirectShadowPlanV2(definition, plan); err != nil {
		return OpenRouterDirectShadowPlanV2{}, err
	}
	return plan, nil
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowPlanV2(definition OpenRouterDirectShadowDefinitionV2, plan OpenRouterDirectShadowPlanV2) error {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV2(definition); err != nil {
		return err
	}
	if plan.Model != definition.Candidate.Model || plan.APIModel != definition.Candidate.APIModel || plan.ServingProvider != definition.Candidate.ServingProvider || plan.ProviderFallback || plan.MaxOutputTokens != definition.GeneratedTokenCeilingPerCall || strings.TrimSpace(plan.Prompt) == "" || plan.PromptSHA256 != sha256Hex([]byte(plan.Prompt)) || plan.SchemaSHA256 != sha256Hex(plan.Schema) || plan.RequestSHA256 != sha256Hex(plan.RequestBody) {
		return fmt.Errorf("OpenRouter direct shadow plan identity drifted")
	}
	var body struct {
		Model       string                           `json:"model"`
		Messages    []struct{ Role, Content string } `json:"messages"`
		MaxTokens   int                              `json:"max_tokens"`
		Temperature int                              `json:"temperature"`
		Stream      bool                             `json:"stream"`
		Provider    struct {
			Order             []string `json:"order"`
			Only              []string `json:"only"`
			AllowFallbacks    bool     `json:"allow_fallbacks"`
			RequireParameters bool     `json:"require_parameters"`
		} `json:"provider"`
		ResponseFormat struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Name   string          `json:"name"`
				Strict bool            `json:"strict"`
				Schema json.RawMessage `json:"schema"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}
	decoder := json.NewDecoder(bytes.NewReader(plan.RequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("OpenRouter direct shadow request body is not exact")
	}
	if body.Model != plan.APIModel || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[0].Content != "Return only the exact JSON object requested by the supplied frozen workflow. No tools or external access." || body.Messages[1].Role != "user" || body.Messages[1].Content != plan.Prompt || body.MaxTokens != plan.MaxOutputTokens || body.Temperature != 0 || body.Stream || !reflect.DeepEqual(body.Provider.Order, []string{plan.ServingProvider}) || !reflect.DeepEqual(body.Provider.Only, []string{plan.ServingProvider}) || body.Provider.AllowFallbacks || !body.Provider.RequireParameters || body.ResponseFormat.Type != "json_schema" || body.ResponseFormat.JSONSchema.Name != "item14_local_shadow_v2" || !body.ResponseFormat.JSONSchema.Strict || sha256Hex(body.ResponseFormat.JSONSchema.Schema) != plan.SchemaSHA256 {
		return fmt.Errorf("OpenRouter direct shadow request boundary drifted")
	}
	return nil
}

type openRouterDirectResponseV2 struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
	Choices  []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			ToolCalls json.RawMessage `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int64       `json:"prompt_tokens"`
		CompletionTokens    int64       `json:"completion_tokens"`
		TotalTokens         int64       `json:"total_tokens"`
		Cost                json.Number `json:"cost"`
		PromptTokensDetails struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int64 `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Metadata struct {
		Requested string `json:"requested"`
		Strategy  string `json:"strategy"`
		Attempt   int    `json:"attempt"`
		Endpoints struct {
			Total     int `json:"total"`
			Available []struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
				Selected bool   `json:"selected"`
			} `json:"available"`
		} `json:"endpoints"`
		Attempts []struct {
			Provider string          `json:"provider"`
			Model    string          `json:"model"`
			Status   json.RawMessage `json:"status"`
		} `json:"attempts"`
	} `json:"openrouter_metadata"`
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowObservationV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, raw []byte, providerRequests int, startedAt, completedAt string, durationMillis int64) (OpenRouterDirectShadowObservationV2Record, error) {
	record := OpenRouterDirectShadowObservationV2Record{SchemaVersion: OpenRouterDirectShadowObservationV2, WorkflowID: plan.WorkflowID, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256, RequestSHA256: plan.RequestSHA256, ResponseSHA256: sha256Hex(raw), RequestedModel: plan.Model, RequestedServingProvider: plan.ServingProvider, ProviderRequestCount: providerRequests, StartedAt: startedAt, CompletedAt: completedAt, DurationMillis: durationMillis}
	if err := ctxContracts.ValidateOpenRouterDirectShadowPlanV2(definition, plan); err != nil {
		return record, err
	}
	if providerRequests != 1 || len(raw) == 0 {
		return record, fmt.Errorf("OpenRouter direct shadow provider request evidence drifted")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var response openRouterDirectResponseV2
	if err := decoder.Decode(&response); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return record, fmt.Errorf("decode OpenRouter direct response: invalid JSON")
	}
	record.ProviderResponseID = response.ID
	record.ActualModel = "openrouter/" + response.Model
	if response.ID == "" || response.Object != "chat.completion" || response.Model != plan.APIModel || response.Metadata.Requested != plan.APIModel || response.Metadata.Attempt != 1 || len(response.Metadata.Attempts) != 1 || response.Metadata.Endpoints.Total < 1 || len(response.Metadata.Endpoints.Available) < 1 {
		return record, fmt.Errorf("OpenRouter direct response model or router metadata drifted")
	}
	selectedProviders := make([]string, 0, 1)
	for _, endpoint := range response.Metadata.Endpoints.Available {
		if normalizeOpenRouterProviderV2(endpoint.Provider) != normalizeOpenRouterProviderV2(plan.ServingProvider) || endpoint.Model != plan.APIModel {
			return record, fmt.Errorf("OpenRouter direct response exposed a non-approved endpoint")
		}
		if endpoint.Selected {
			selectedProviders = append(selectedProviders, endpoint.Provider)
		}
	}
	attempt := response.Metadata.Attempts[0]
	if len(selectedProviders) != 1 || normalizeOpenRouterProviderV2(attempt.Provider) != normalizeOpenRouterProviderV2(plan.ServingProvider) || attempt.Model != plan.APIModel || !openRouterAttemptSucceededV2(attempt.Status) {
		return record, fmt.Errorf("OpenRouter direct response did not prove one exact provider attempt")
	}
	record.ActualServingProvider = plan.ServingProvider
	record.RouterAttemptCount = 1
	if len(response.Choices) != 1 || response.Choices[0].Index != 0 || response.Choices[0].Message.Role != "assistant" || !guardedExactAbsentNullOrEmptyArrayV2(response.Choices[0].Message.ToolCalls) {
		return record, fmt.Errorf("OpenRouter direct response contained hidden choice or tool activity")
	}
	if response.Usage.PromptTokens < 1 || response.Usage.CompletionTokens < 1 || response.Usage.TotalTokens < response.Usage.PromptTokens+response.Usage.CompletionTokens || response.Usage.PromptTokens > definition.MaxInputTokensPerCall || response.Usage.CompletionTokens > int64(definition.GeneratedTokenCeilingPerCall) || response.Usage.PromptTokensDetails.CachedTokens < 0 || response.Usage.CompletionTokensDetails.ReasoningTokens < 0 || response.Usage.CompletionTokensDetails.ReasoningTokens > response.Usage.CompletionTokens {
		return record, fmt.Errorf("OpenRouter direct response usage drifted")
	}
	costMicros, err := decimalUSDMicrosCeilingV2(response.Usage.Cost)
	if err != nil || costMicros < 0 || costMicros > definition.PerCallReservationMicros {
		return record, fmt.Errorf("OpenRouter direct response cost drifted")
	}
	record.InputTokens = response.Usage.PromptTokens
	record.CachedInputTokens = response.Usage.PromptTokensDetails.CachedTokens
	record.OutputTokens = response.Usage.CompletionTokens
	record.ReasoningTokens = response.Usage.CompletionTokensDetails.ReasoningTokens
	record.ProviderReportedCostMicros = costMicros
	var workflow *LocalShadowWorkflowV1
	for index := range suite.Workflows {
		if suite.Workflows[index].ID == plan.WorkflowID {
			workflow = &suite.Workflows[index]
			break
		}
	}
	if workflow == nil {
		return record, fmt.Errorf("OpenRouter direct shadow workflow missing")
	}
	parsed, contractErr := ValidateOpenRouterLocalShadowResponseV1(*workflow, definition.SuiteSHA256, []byte(response.Choices[0].Message.Content))
	if contractErr != nil || response.Choices[0].FinishReason != "stop" {
		record.Status = "candidate_response_contract_failure"
		if contractErr != nil {
			record.ResponseContractError = contractErr.Error()
		} else {
			record.ResponseContractError = "OpenRouter direct shadow response did not stop normally"
		}
		return record, nil
	}
	record.Status = "passed"
	record.ResponseContractValid = true
	record.Response = &parsed
	return record, nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowRunV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1, observations []OpenRouterDirectShadowObservationV2Record, fake bool) (OpenRouterDirectShadowRunV2, error) {
	run := OpenRouterDirectShadowRunV2{SchemaVersion: OpenRouterDirectShadowRunSchemaV2, RunID: definition.RunID, ExecutionMode: "real_direct_provider", DefinitionSHA256: OpenRouterDirectShadowDefinitionV2SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, ProviderRequestCount: len(observations), Observations: append([]OpenRouterDirectShadowObservationV2Record(nil), observations...)}
	if fake {
		run.ExecutionMode = "deterministic_fake_provider_free"
		run.ProviderRequestCount = 0
		run.SimulatedResponseCount = len(observations)
	}
	for _, observation := range observations {
		if observation.Status == "passed" {
			run.CandidatePassCount++
		} else {
			run.CandidateFailureCount++
		}
		run.TotalInputTokens += observation.InputTokens
		run.TotalCachedInputTokens += observation.CachedInputTokens
		run.TotalOutputTokens += observation.OutputTokens
		run.TotalReasoningTokens += observation.ReasoningTokens
		run.TotalCostMicros += observation.ProviderReportedCostMicros
	}
	if err := ctxContracts.ValidateOpenRouterDirectShadowRunV2(definition, suite, run); err != nil {
		return OpenRouterDirectShadowRunV2{}, err
	}
	return run, nil
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowRunV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1, run OpenRouterDirectShadowRunV2) error {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV2(definition); err != nil {
		return err
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return err
	}
	if run.SchemaVersion != OpenRouterDirectShadowRunSchemaV2 || run.RunID != definition.RunID || run.DefinitionSHA256 != OpenRouterDirectShadowDefinitionV2SHA256() || run.SuiteSHA256 != definition.SuiteSHA256 || run.Model != definition.Candidate.Model || run.ServingProvider != definition.Candidate.ServingProvider || len(run.Observations) != definition.MaxProviderCalls || run.CandidatePassCount+run.CandidateFailureCount != len(run.Observations) || run.RetryCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.RoleModelSelectionCount != 0 {
		return fmt.Errorf("OpenRouter direct shadow run boundary drifted")
	}
	if run.ExecutionMode == "deterministic_fake_provider_free" {
		if run.ProviderRequestCount != 0 || run.SimulatedResponseCount != definition.MaxProviderCalls {
			return fmt.Errorf("OpenRouter direct shadow fake counts drifted")
		}
	} else if run.ExecutionMode == "real_direct_provider" {
		if run.ProviderRequestCount != definition.MaxProviderCalls || run.SimulatedResponseCount != 0 {
			return fmt.Errorf("OpenRouter direct shadow real counts drifted")
		}
	} else {
		return fmt.Errorf("OpenRouter direct shadow execution mode drifted")
	}
	seenResponses := map[string]struct{}{}
	var input, cached, output, reasoning, cost int64
	for index, observation := range run.Observations {
		if observation.SchemaVersion != OpenRouterDirectShadowObservationV2 || observation.WorkflowID != definition.WorkflowIDs[index] || observation.RequestedModel != definition.Candidate.Model || observation.ActualModel != definition.Candidate.Model || observation.RequestedServingProvider != definition.Candidate.ServingProvider || observation.ActualServingProvider != definition.Candidate.ServingProvider || observation.RouterAttemptCount != 1 || observation.ProviderRequestCount != 1 || observation.RetryCount != 0 || observation.ProviderResponseID == "" || !validSHA256(observation.PromptSHA256) || !validSHA256(observation.SchemaSHA256) || !validSHA256(observation.RequestSHA256) || !validSHA256(observation.ResponseSHA256) || observation.InputTokens < 1 || observation.OutputTokens < 1 || observation.ProviderReportedCostMicros < 0 || observation.ProviderReportedCostMicros > definition.PerCallReservationMicros {
			return fmt.Errorf("OpenRouter direct shadow observation %d drifted", index)
		}
		if _, exists := seenResponses[observation.ProviderResponseID]; exists {
			return fmt.Errorf("OpenRouter direct shadow duplicate provider response")
		}
		seenResponses[observation.ProviderResponseID] = struct{}{}
		if observation.Status == "passed" {
			if !observation.ResponseContractValid || observation.Response == nil || observation.ResponseContractError != "" {
				return fmt.Errorf("OpenRouter direct shadow passed observation lacks response evidence")
			}
		} else if observation.Status == "candidate_response_contract_failure" {
			if observation.ResponseContractValid || observation.Response != nil || observation.ResponseContractError == "" {
				return fmt.Errorf("OpenRouter direct shadow candidate failure drifted")
			}
		} else {
			return fmt.Errorf("OpenRouter direct shadow observation status drifted")
		}
		input += observation.InputTokens
		cached += observation.CachedInputTokens
		output += observation.OutputTokens
		reasoning += observation.ReasoningTokens
		cost += observation.ProviderReportedCostMicros
	}
	if input != run.TotalInputTokens || cached != run.TotalCachedInputTokens || output != run.TotalOutputTokens || reasoning != run.TotalReasoningTokens || cost != run.TotalCostMicros || cost > definition.SliceIncrementalCeilingMicros {
		return fmt.Errorf("OpenRouter direct shadow totals drifted")
	}
	return nil
}

func normalizeOpenRouterProviderV2(value string) string {
	var normalized strings.Builder
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			normalized.WriteRune(char)
		}
	}
	return normalized.String()
}

func openRouterAttemptSucceededV2(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return bytes.Equal(trimmed, []byte("200"))
}

func guardedExactAbsentNullOrEmptyArrayV2(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]"))
}

func decimalUSDMicrosCeilingV2(value json.Number) (int64, error) {
	return decimalStringToMicrosCeilingV2(string(value))
}

func decimalStringToMicrosCeilingV2(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, "eE") {
		return 0, fmt.Errorf("invalid decimal USD value")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid decimal USD value")
	}
	var whole int64
	for _, char := range parts[0] {
		if char < '0' || char > '9' || whole > 9_000_000_000_000 {
			return 0, fmt.Errorf("invalid decimal USD value")
		}
		whole = whole*10 + int64(char-'0')
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		for _, char := range fraction {
			if char < '0' || char > '9' {
				return 0, fmt.Errorf("invalid decimal USD value")
			}
		}
	}
	padded := fraction + "000000"
	var micros int64
	for _, char := range padded[:6] {
		micros = micros*10 + int64(char-'0')
	}
	if len(fraction) > 6 {
		for _, char := range fraction[6:] {
			if char != '0' {
				micros++
				break
			}
		}
	}
	if whole > (math.MaxInt64-micros)/1_000_000 {
		return 0, fmt.Errorf("decimal USD value exceeds int64 micros")
	}
	return whole*1_000_000 + micros, nil
}
