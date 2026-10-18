package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const (
	OpenRouterDirectShadowDefinitionSchemaV3 = "silver-mediums.openrouter-direct-shadow-definition.v3"
	OpenRouterDirectShadowRunSchemaV3        = "silver-mediums.openrouter-direct-shadow-run.v3"
	OpenRouterDirectShadowObservationV3      = "silver-mediums.openrouter-direct-shadow-observation.v3"
	OpenRouterDirectShadowTransportV3        = "openrouter_chat_completions_direct_router_metadata_v3"
)

//go:embed openrouter_direct_shadow_v3.json
var openRouterDirectShadowDefinitionRawV3 []byte

type OpenRouterDirectShadowDefinitionV3 struct {
	SchemaVersion                      string                            `json:"schema_version"`
	RunID                              string                            `json:"run_id"`
	Status                             string                            `json:"status"`
	Purpose                            string                            `json:"purpose"`
	AntecedentV2DefinitionSHA256       string                            `json:"antecedent_v2_definition_sha256"`
	AntecedentV2TerminalResponseSHA256 string                            `json:"antecedent_v2_terminal_response_sha256"`
	SuiteSHA256                        string                            `json:"suite_sha256"`
	EligibilityEvidenceSHA256          string                            `json:"eligibility_evidence_sha256"`
	Candidate                          OpenRouterDirectShadowCandidateV2 `json:"candidate"`
	ExpectedEndpointModel              string                            `json:"expected_endpoint_model"`
	Endpoint                           string                            `json:"endpoint"`
	Transport                          string                            `json:"transport"`
	WorkflowIDs                        []string                          `json:"workflow_ids"`
	MaxProviderCalls                   int                               `json:"max_provider_calls"`
	CallsPerWorkflow                   int                               `json:"calls_per_workflow"`
	RetryCount                         int                               `json:"retry_count"`
	RedirectsAllowed                   bool                              `json:"redirects_allowed"`
	ProviderOrderExact                 bool                              `json:"provider_order_exact"`
	ProviderOnlyExact                  bool                              `json:"provider_only_exact"`
	RouterMetadataRequired             bool                              `json:"router_metadata_required"`
	ProviderStructuredOutputAllowed    bool                              `json:"provider_structured_output_parameter_allowed"`
	ResponseValidation                 string                            `json:"response_validation"`
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

type OpenRouterDirectShadowRunV3 struct {
	SchemaVersion           string                                      `json:"schema_version"`
	RunID                   string                                      `json:"run_id"`
	ExecutionMode           string                                      `json:"execution_mode"`
	DefinitionSHA256        string                                      `json:"definition_sha256"`
	SuiteSHA256             string                                      `json:"suite_sha256"`
	Model                   string                                      `json:"model"`
	ServingProvider         string                                      `json:"serving_provider"`
	ExpectedEndpointModel   string                                      `json:"expected_endpoint_model"`
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

func (ctxContracts QualificationContracts) CurrentOpenRouterDirectShadowDefinitionV3() (OpenRouterDirectShadowDefinitionV3, error) {
	var definition OpenRouterDirectShadowDefinitionV3
	if err := decodeExactJSONV1(openRouterDirectShadowDefinitionRawV3, &definition); err != nil {
		return definition, fmt.Errorf("decode OpenRouter direct shadow definition v3: %w", err)
	}
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV3(definition); err != nil {
		return OpenRouterDirectShadowDefinitionV3{}, err
	}
	return definition, nil
}

func OpenRouterDirectShadowDefinitionV3SHA256() string {
	return sha256Hex(openRouterDirectShadowDefinitionRawV3)
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowDefinitionV3(definition OpenRouterDirectShadowDefinitionV3) error {
	wantWorkflows := []string{"local-intake-decision-coordination", "local-implementation-review-mobile", "local-release-regression-rollback", "local-interruption-recovery"}
	if definition.SchemaVersion != OpenRouterDirectShadowDefinitionSchemaV3 || definition.RunID != "item-14-openrouter-direct-shadow-v3" || definition.Status != "prepared_frozen_not_launched" || definition.Purpose != "direct exact-model and exact-serving-provider proof with provider-independent response validation" || definition.AntecedentV2DefinitionSHA256 != "fbe314bc6d779d0ad57bb6150f200e7a9db6af8e9a7c0b4ca6e814007c9441b4" || definition.AntecedentV2TerminalResponseSHA256 != "611dfac19c079fcd560ebf89f042049646a53312b0b9eac2bc14b4991aa04aa6" {
		return fmt.Errorf("OpenRouter direct shadow v3 identity or antecedent drifted")
	}
	if definition.SuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() || definition.EligibilityEvidenceSHA256 != "84660c35c92e780c3c00f744a2380683accfc0158dea161b8353953247211b46" {
		return fmt.Errorf("OpenRouter direct shadow v3 antecedent hashes drifted")
	}
	candidate := definition.Candidate
	if candidate.ConfigurationID != "item14.fixture.qwen3codernext.direct.openrouter.v3" || candidate.Model != "openrouter/qwen/qwen3-coder-next" || candidate.APIModel != "qwen/qwen3-coder-next" || candidate.ReasoningEffort != "screening_default" || candidate.RuntimeProvider != "openrouter_direct_api" || candidate.ServingProvider != "alibaba" || candidate.ProviderFallbacks || !candidate.TemporaryFixtureOnly || definition.ExpectedEndpointModel != "qwen/qwen3-coder-next-2025-02-03" {
		return fmt.Errorf("OpenRouter direct shadow v3 candidate drifted")
	}
	if err := ValidateOpenRouterEstablishedSmallModelRouteV2(candidate.Model, candidate.ServingProvider); err != nil {
		return err
	}
	if strings.TrimPrefix(candidate.Model, "openrouter/") != candidate.APIModel || definition.Endpoint != "https://openrouter.ai/api/v1/chat/completions" || definition.Transport != OpenRouterDirectShadowTransportV3 || !reflect.DeepEqual(definition.WorkflowIDs, wantWorkflows) || definition.MaxProviderCalls != 4 || definition.CallsPerWorkflow != 1 || definition.RetryCount != 0 || definition.RedirectsAllowed || !definition.ProviderOrderExact || !definition.ProviderOnlyExact || !definition.RouterMetadataRequired || definition.ProviderStructuredOutputAllowed || definition.ResponseValidation != "controller_strict_schema_after_exact_provider_response" || definition.GeneratedTokenCeilingPerCall != 1200 {
		return fmt.Errorf("OpenRouter direct shadow v3 transport boundary drifted")
	}
	if definition.MaxInputTokensPerCall != 262144 || definition.InputPriceMicrosPerMillion != 300000 || definition.GeneratedPriceMicrosPerMillion != 1500000 || definition.PerCallReservationMicros != 80444 || definition.TotalPriceReservationMicros != 321776 || definition.TotalPriceReservationMicros != int64(definition.MaxProviderCalls)*definition.PerCallReservationMicros || definition.PriceEvidence != "https://openrouter.ai/provider/alibaba#qwen3-coder-next observed 2026-09-14" || definition.CumulativeOpenRouterBaselineMicros != 102104668 || definition.CumulativeOpenRouterCeilingMicros != 142104668 || definition.SliceIncrementalCeilingMicros != 1000000 || definition.CumulativeOpenRouterCeilingMicros-definition.CumulativeOpenRouterBaselineMicros != 40000000 {
		return fmt.Errorf("OpenRouter direct shadow v3 spend boundary drifted")
	}
	inputReserve := (definition.MaxInputTokensPerCall*definition.InputPriceMicrosPerMillion + 999999) / 1000000
	outputReserve := (int64(definition.GeneratedTokenCeilingPerCall)*definition.GeneratedPriceMicrosPerMillion + 999999) / 1000000
	if inputReserve+outputReserve != definition.PerCallReservationMicros {
		return fmt.Errorf("OpenRouter direct shadow v3 price-derived reservation drifted")
	}
	if definition.ResponseContractFailurePolicy != "candidate_local_continue" || definition.IdentityOrProviderFailurePolicy != "global_halt_no_retry" || definition.ToolsAllowed || definition.RepositoryAccessAllowed || definition.ProductionAccessAllowed || definition.OutboundActionsAllowed || definition.PhoneFarmAccessAllowed || definition.RoleModelSelectionAllowed {
		return fmt.Errorf("OpenRouter direct shadow v3 failure or capability policy drifted")
	}
	return nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowPlansV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1) ([]OpenRouterDirectShadowPlanV2, error) {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV3(definition); err != nil {
		return nil, err
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return nil, err
	}
	byID := make(map[string]LocalShadowWorkflowV1, len(suite.Workflows))
	for _, workflow := range suite.Workflows {
		byID[workflow.ID] = workflow
	}
	plans := make([]OpenRouterDirectShadowPlanV2, 0, len(definition.WorkflowIDs))
	for _, workflowID := range definition.WorkflowIDs {
		workflow, ok := byID[workflowID]
		if !ok {
			return nil, fmt.Errorf("OpenRouter direct shadow v3 workflow missing")
		}
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
		packetRaw, _ := json.Marshal(packet)
		prompt := "Treat the following as an immutable provider-free workflow contract. Return exactly one JSON object that echoes the workflow_id, suite_sha256, stage_ids, and evidence_contract_ids without additions or reordering. Set boundary_verdict to accepted_frozen_provider_free_contract; provider_actions_planned and tool_calls_planned to 0; and every access/action/selection boolean to false. Do not use tools, files, repositories, network access, prior sessions, production systems, or outbound actions.\n\n<frozen-workflow>\n" + string(packetRaw) + "\n</frozen-workflow>"
		schema := openRouterLocalShadowSchemaV1()
		request := map[string]any{
			"model":      definition.Candidate.APIModel,
			"messages":   []map[string]string{{"role": "system", "content": "Return only the exact JSON object requested by the supplied frozen workflow. No tools or external access."}, {"role": "user", "content": prompt}},
			"max_tokens": definition.GeneratedTokenCeilingPerCall, "temperature": 0, "stream": false,
			"provider": map[string]any{"order": []string{definition.Candidate.ServingProvider}, "only": []string{definition.Candidate.ServingProvider}, "allow_fallbacks": false},
		}
		requestBody, _ := json.Marshal(request)
		plan := OpenRouterDirectShadowPlanV2{WorkflowID: workflow.ID, Model: definition.Candidate.Model, APIModel: definition.Candidate.APIModel, ServingProvider: definition.Candidate.ServingProvider, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)), Schema: schema, SchemaSHA256: sha256Hex(schema), RequestBody: requestBody, RequestSHA256: sha256Hex(requestBody), MaxOutputTokens: definition.GeneratedTokenCeilingPerCall}
		if err := ctxContracts.ValidateOpenRouterDirectShadowPlanV3(definition, plan); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowPlanV3(definition OpenRouterDirectShadowDefinitionV3, plan OpenRouterDirectShadowPlanV2) error {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV3(definition); err != nil {
		return err
	}
	if plan.Model != definition.Candidate.Model || plan.APIModel != definition.Candidate.APIModel || plan.ServingProvider != definition.Candidate.ServingProvider || plan.ProviderFallback || plan.MaxOutputTokens != definition.GeneratedTokenCeilingPerCall || plan.PromptSHA256 != sha256Hex([]byte(plan.Prompt)) || plan.SchemaSHA256 != sha256Hex(plan.Schema) || plan.RequestSHA256 != sha256Hex(plan.RequestBody) {
		return fmt.Errorf("OpenRouter direct shadow v3 plan drifted")
	}
	var body struct {
		Model       string                           `json:"model"`
		Messages    []struct{ Role, Content string } `json:"messages"`
		MaxTokens   int                              `json:"max_tokens"`
		Temperature int                              `json:"temperature"`
		Stream      bool                             `json:"stream"`
		Provider    struct {
			Order          []string `json:"order"`
			Only           []string `json:"only"`
			AllowFallbacks bool     `json:"allow_fallbacks"`
		} `json:"provider"`
	}
	decoder := json.NewDecoder(bytes.NewReader(plan.RequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(&struct{}{}) != io.EOF || body.Model != plan.APIModel || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" || body.Messages[1].Content != plan.Prompt || body.MaxTokens != plan.MaxOutputTokens || body.Temperature != 0 || body.Stream || !reflect.DeepEqual(body.Provider.Order, []string{plan.ServingProvider}) || !reflect.DeepEqual(body.Provider.Only, []string{plan.ServingProvider}) || body.Provider.AllowFallbacks {
		return fmt.Errorf("OpenRouter direct shadow v3 request boundary drifted")
	}
	return nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowObservationV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, raw []byte, providerRequests int, startedAt, completedAt string, durationMillis int64) (OpenRouterDirectShadowObservationV2Record, error) {
	record := OpenRouterDirectShadowObservationV2Record{SchemaVersion: OpenRouterDirectShadowObservationV3, WorkflowID: plan.WorkflowID, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256, RequestSHA256: plan.RequestSHA256, ResponseSHA256: sha256Hex(raw), RequestedModel: plan.Model, RequestedServingProvider: plan.ServingProvider, ProviderRequestCount: providerRequests, StartedAt: startedAt, CompletedAt: completedAt, DurationMillis: durationMillis}
	if err := ctxContracts.ValidateOpenRouterDirectShadowPlanV3(definition, plan); err != nil {
		return record, err
	}
	if providerRequests != 1 || len(raw) == 0 {
		return record, fmt.Errorf("OpenRouter direct shadow v3 provider request evidence drifted")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var response openRouterDirectResponseV2
	if err := decoder.Decode(&response); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return record, fmt.Errorf("decode OpenRouter direct v3 response: invalid JSON")
	}
	record.ProviderResponseID = response.ID
	record.ActualModel = "openrouter/" + response.Model
	if response.ID == "" || response.Object != "chat.completion" || response.Model != plan.APIModel || response.Metadata.Requested != plan.APIModel || response.Metadata.Attempt != 1 || len(response.Metadata.Attempts) != 1 || len(response.Metadata.Endpoints.Available) < 1 {
		return record, fmt.Errorf("OpenRouter direct v3 response model or metadata drifted")
	}
	selected := 0
	for _, endpoint := range response.Metadata.Endpoints.Available {
		if !endpoint.Selected {
			continue
		}
		if normalizeOpenRouterProviderV2(endpoint.Provider) != normalizeOpenRouterProviderV2(plan.ServingProvider) || endpoint.Model != definition.ExpectedEndpointModel {
			return record, fmt.Errorf("OpenRouter direct v3 selected a non-approved endpoint")
		}
		selected++
	}
	attempt := response.Metadata.Attempts[0]
	if selected != 1 || normalizeOpenRouterProviderV2(attempt.Provider) != normalizeOpenRouterProviderV2(plan.ServingProvider) || attempt.Model != definition.ExpectedEndpointModel || !openRouterAttemptSucceededV2(attempt.Status) {
		return record, fmt.Errorf("OpenRouter direct v3 did not prove one exact provider/model attempt")
	}
	record.ActualServingProvider = plan.ServingProvider
	record.RouterAttemptCount = 1
	if len(response.Choices) != 1 || response.Choices[0].Index != 0 || response.Choices[0].Message.Role != "assistant" || !guardedExactAbsentNullOrEmptyArrayV2(response.Choices[0].Message.ToolCalls) {
		return record, fmt.Errorf("OpenRouter direct v3 response contained hidden choice or tool activity")
	}
	if response.Usage.PromptTokens < 1 || response.Usage.CompletionTokens < 1 || response.Usage.TotalTokens < response.Usage.PromptTokens+response.Usage.CompletionTokens || response.Usage.PromptTokens > definition.MaxInputTokensPerCall || response.Usage.CompletionTokens > int64(definition.GeneratedTokenCeilingPerCall) || response.Usage.PromptTokensDetails.CachedTokens < 0 || response.Usage.CompletionTokensDetails.ReasoningTokens < 0 || response.Usage.CompletionTokensDetails.ReasoningTokens > response.Usage.CompletionTokens {
		return record, fmt.Errorf("OpenRouter direct v3 usage drifted")
	}
	cost, err := decimalUSDMicrosCeilingV2(response.Usage.Cost)
	if err != nil || cost < 0 || cost > definition.PerCallReservationMicros {
		return record, fmt.Errorf("OpenRouter direct v3 cost drifted")
	}
	record.InputTokens, record.CachedInputTokens, record.OutputTokens, record.ReasoningTokens, record.ProviderReportedCostMicros = response.Usage.PromptTokens, response.Usage.PromptTokensDetails.CachedTokens, response.Usage.CompletionTokens, response.Usage.CompletionTokensDetails.ReasoningTokens, cost
	var workflow *LocalShadowWorkflowV1
	for index := range suite.Workflows {
		if suite.Workflows[index].ID == plan.WorkflowID {
			workflow = &suite.Workflows[index]
			break
		}
	}
	if workflow == nil {
		return record, fmt.Errorf("OpenRouter direct shadow v3 workflow missing")
	}
	parsed, contractErr := ValidateOpenRouterLocalShadowResponseV1(*workflow, definition.SuiteSHA256, []byte(response.Choices[0].Message.Content))
	if contractErr != nil || response.Choices[0].FinishReason != "stop" {
		record.Status = "candidate_response_contract_failure"
		if contractErr != nil {
			record.ResponseContractError = contractErr.Error()
		} else {
			record.ResponseContractError = "OpenRouter direct v3 response did not stop normally"
		}
		return record, nil
	}
	record.Status, record.ResponseContractValid, record.Response = "passed", true, &parsed
	return record, nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowRunV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1, observations []OpenRouterDirectShadowObservationV2Record, fake bool) (OpenRouterDirectShadowRunV3, error) {
	run := OpenRouterDirectShadowRunV3{SchemaVersion: OpenRouterDirectShadowRunSchemaV3, RunID: definition.RunID, ExecutionMode: "real_direct_provider", DefinitionSHA256: OpenRouterDirectShadowDefinitionV3SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, ExpectedEndpointModel: definition.ExpectedEndpointModel, ProviderRequestCount: len(observations), Observations: append([]OpenRouterDirectShadowObservationV2Record(nil), observations...)}
	if fake {
		run.ExecutionMode, run.ProviderRequestCount, run.SimulatedResponseCount = "deterministic_fake_provider_free", 0, len(observations)
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
	if err := ctxContracts.ValidateOpenRouterDirectShadowRunV3(definition, suite, run); err != nil {
		return OpenRouterDirectShadowRunV3{}, err
	}
	return run, nil
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowRunV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1, run OpenRouterDirectShadowRunV3) error {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV3(definition); err != nil {
		return err
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return err
	}
	if run.SchemaVersion != OpenRouterDirectShadowRunSchemaV3 || run.RunID != definition.RunID || run.DefinitionSHA256 != OpenRouterDirectShadowDefinitionV3SHA256() || run.SuiteSHA256 != definition.SuiteSHA256 || run.Model != definition.Candidate.Model || run.ServingProvider != definition.Candidate.ServingProvider || run.ExpectedEndpointModel != definition.ExpectedEndpointModel || len(run.Observations) != 4 || run.CandidatePassCount+run.CandidateFailureCount != 4 || run.RetryCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.RoleModelSelectionCount != 0 {
		return fmt.Errorf("OpenRouter direct shadow v3 run boundary drifted")
	}
	if run.ExecutionMode == "deterministic_fake_provider_free" {
		if run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 4 {
			return fmt.Errorf("OpenRouter direct shadow v3 fake counts drifted")
		}
	} else if run.ExecutionMode == "real_direct_provider" {
		if run.ProviderRequestCount != 4 || run.SimulatedResponseCount != 0 {
			return fmt.Errorf("OpenRouter direct shadow v3 real counts drifted")
		}
	} else {
		return fmt.Errorf("OpenRouter direct shadow v3 mode drifted")
	}
	seen := map[string]struct{}{}
	var input, cached, output, reasoning, cost int64
	for index, observation := range run.Observations {
		if observation.SchemaVersion != OpenRouterDirectShadowObservationV3 || observation.WorkflowID != definition.WorkflowIDs[index] || observation.RequestedModel != definition.Candidate.Model || observation.ActualModel != definition.Candidate.Model || observation.RequestedServingProvider != definition.Candidate.ServingProvider || observation.ActualServingProvider != definition.Candidate.ServingProvider || observation.RouterAttemptCount != 1 || observation.ProviderRequestCount != 1 || observation.RetryCount != 0 || observation.ProviderResponseID == "" || !validSHA256(observation.PromptSHA256) || !validSHA256(observation.SchemaSHA256) || !validSHA256(observation.RequestSHA256) || !validSHA256(observation.ResponseSHA256) || observation.InputTokens < 1 || observation.OutputTokens < 1 || observation.ProviderReportedCostMicros < 0 || observation.ProviderReportedCostMicros > definition.PerCallReservationMicros {
			return fmt.Errorf("OpenRouter direct shadow v3 observation %d drifted", index)
		}
		if _, exists := seen[observation.ProviderResponseID]; exists {
			return fmt.Errorf("OpenRouter direct shadow v3 duplicate response")
		}
		seen[observation.ProviderResponseID] = struct{}{}
		if observation.Status == "passed" {
			if !observation.ResponseContractValid || observation.Response == nil || observation.ResponseContractError != "" {
				return fmt.Errorf("OpenRouter direct shadow v3 pass evidence drifted")
			}
		} else if observation.Status == "candidate_response_contract_failure" {
			if observation.ResponseContractValid || observation.Response != nil || observation.ResponseContractError == "" {
				return fmt.Errorf("OpenRouter direct shadow v3 failure evidence drifted")
			}
		} else {
			return fmt.Errorf("OpenRouter direct shadow v3 status drifted")
		}
		input += observation.InputTokens
		cached += observation.CachedInputTokens
		output += observation.OutputTokens
		reasoning += observation.ReasoningTokens
		cost += observation.ProviderReportedCostMicros
	}
	if input != run.TotalInputTokens || cached != run.TotalCachedInputTokens || output != run.TotalOutputTokens || reasoning != run.TotalReasoningTokens || cost != run.TotalCostMicros || cost > definition.SliceIncrementalCeilingMicros {
		return fmt.Errorf("OpenRouter direct shadow v3 totals drifted")
	}
	return nil
}
