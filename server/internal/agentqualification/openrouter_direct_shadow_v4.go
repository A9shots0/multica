package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

const (
	OpenRouterDirectShadowDefinitionSchemaV4 = "silver-mediums.openrouter-direct-shadow-continuation.v4"
	OpenRouterDirectShadowRunSchemaV4        = "silver-mediums.openrouter-direct-shadow-run.v4"
	OpenRouterDirectShadowObservationV4      = "silver-mediums.openrouter-direct-shadow-observation.v4"
	OpenRouterDirectShadowTransportV4        = "openrouter_chat_completions_direct_top_provider_selected_endpoint_v4"
)

//go:embed openrouter_direct_shadow_v4.json
var openRouterDirectShadowDefinitionRawV4 []byte

//go:embed openrouter_direct_shadow_v3_terminal_antecedent.json
var openRouterDirectShadowV3TerminalAntecedentRaw []byte

type OpenRouterDirectShadowDefinitionV4 struct {
	SchemaVersion                     string                            `json:"schema_version"`
	RunID                             string                            `json:"run_id"`
	Status                            string                            `json:"status"`
	Purpose                           string                            `json:"purpose"`
	AntecedentV3DefinitionSHA256      string                            `json:"antecedent_v3_definition_sha256"`
	AntecedentV3ResponseSHA256        string                            `json:"antecedent_v3_response_sha256"`
	AntecedentV3TerminalReceiptSHA256 string                            `json:"antecedent_v3_terminal_receipt_sha256"`
	AntecedentWorkflowID              string                            `json:"antecedent_workflow_id"`
	AntecedentResponseContractValid   bool                              `json:"antecedent_response_contract_valid"`
	AntecedentExactRouteProved        bool                              `json:"antecedent_exact_route_proved"`
	SuiteSHA256                       string                            `json:"suite_sha256"`
	EligibilityEvidenceSHA256         string                            `json:"eligibility_evidence_sha256"`
	Candidate                         OpenRouterDirectShadowCandidateV2 `json:"candidate"`
	ExpectedEndpointModel             string                            `json:"expected_endpoint_model"`
	Endpoint                          string                            `json:"endpoint"`
	Transport                         string                            `json:"transport"`
	WorkflowIDs                       []string                          `json:"workflow_ids"`
	RequestSHA256ByWorkflow           map[string]string                 `json:"request_sha256_by_workflow"`
	ResponseSchemaSHA256              string                            `json:"response_schema_sha256"`
	MaxProviderCalls                  int                               `json:"max_provider_calls"`
	CallsPerWorkflow                  int                               `json:"calls_per_workflow"`
	RetryCount                        int                               `json:"retry_count"`
	RedirectsAllowed                  bool                              `json:"redirects_allowed"`
	ProviderOrderExact                bool                              `json:"provider_order_exact"`
	ProviderOnlyExact                 bool                              `json:"provider_only_exact"`
	RouterMetadataRequired            bool                              `json:"router_metadata_required"`
	TopLevelProviderRequired          bool                              `json:"top_level_provider_required"`
	ProviderAttemptEvidencePolicy     string                            `json:"provider_attempt_evidence_policy"`
	ResponseValidation                string                            `json:"response_validation"`
	GeneratedTokenCeilingPerCall      int                               `json:"generated_token_ceiling_per_call"`
	MaxInputTokensPerCall             int64                             `json:"max_input_tokens_per_call"`
	PerCallReservationMicros          int64                             `json:"per_call_reservation_micros"`
	TotalPriceReservationMicros       int64                             `json:"total_price_reservation_micros"`
	CumulativeBaselineMicros          int64                             `json:"cumulative_openrouter_baseline_micros"`
	CumulativeCeilingMicros           int64                             `json:"cumulative_openrouter_ceiling_micros"`
	SliceIncrementalCeilingMicros     int64                             `json:"slice_incremental_ceiling_micros"`
	ResponseContractFailurePolicy     string                            `json:"response_contract_failure_policy"`
	IdentityOrProviderFailurePolicy   string                            `json:"identity_or_provider_failure_policy"`
	ToolsAllowed                      bool                              `json:"tools_allowed"`
	RepositoryAccessAllowed           bool                              `json:"repository_access_allowed"`
	ProductionAccessAllowed           bool                              `json:"production_access_allowed"`
	OutboundActionsAllowed            bool                              `json:"outbound_actions_allowed"`
	PhoneFarmAccessAllowed            bool                              `json:"phone_farm_access_allowed"`
	RoleModelSelectionAllowed         bool                              `json:"role_model_selection_allowed"`
}

type OpenRouterDirectShadowRunV4 struct {
	SchemaVersion            string                                      `json:"schema_version"`
	RunID                    string                                      `json:"run_id"`
	ExecutionMode            string                                      `json:"execution_mode"`
	DefinitionSHA256         string                                      `json:"definition_sha256"`
	SuiteSHA256              string                                      `json:"suite_sha256"`
	Model                    string                                      `json:"model"`
	ServingProvider          string                                      `json:"serving_provider"`
	ExpectedEndpointModel    string                                      `json:"expected_endpoint_model"`
	ProviderRequestCount     int                                         `json:"provider_request_count"`
	SimulatedResponseCount   int                                         `json:"simulated_provider_response_count"`
	CurrentPassCount         int                                         `json:"current_pass_count"`
	CurrentFailureCount      int                                         `json:"current_failure_count"`
	AntecedentFailureCount   int                                         `json:"antecedent_failure_count"`
	TotalEvaluatedWorkflows  int                                         `json:"total_evaluated_workflows"`
	CandidateQualified       bool                                        `json:"candidate_qualified"`
	RetryCount               int                                         `json:"retry_count"`
	ToolCallCount            int                                         `json:"tool_call_count"`
	RepositoryAccessCount    int                                         `json:"repository_access_count"`
	ProductionAccessCount    int                                         `json:"production_access_count"`
	OutboundActionCount      int                                         `json:"outbound_action_count"`
	RoleModelSelectionCount  int                                         `json:"role_model_selection_count"`
	CurrentInputTokens       int64                                       `json:"current_input_tokens"`
	CurrentCachedInputTokens int64                                       `json:"current_cached_input_tokens"`
	CurrentOutputTokens      int64                                       `json:"current_output_tokens"`
	CurrentReasoningTokens   int64                                       `json:"current_reasoning_tokens"`
	CurrentCostMicros        int64                                       `json:"current_cost_micros"`
	AntecedentInputTokens    int64                                       `json:"antecedent_input_tokens"`
	AntecedentOutputTokens   int64                                       `json:"antecedent_output_tokens"`
	AntecedentCostMicros     int64                                       `json:"antecedent_cost_micros"`
	CombinedInputTokens      int64                                       `json:"combined_input_tokens"`
	CombinedOutputTokens     int64                                       `json:"combined_output_tokens"`
	CombinedCostMicros       int64                                       `json:"combined_cost_micros"`
	Observations             []OpenRouterDirectShadowObservationV2Record `json:"observations"`
}

func (ctxContracts QualificationContracts) CurrentOpenRouterDirectShadowDefinitionV4() (OpenRouterDirectShadowDefinitionV4, error) {
	var definition OpenRouterDirectShadowDefinitionV4
	if err := decodeExactJSONV1(openRouterDirectShadowDefinitionRawV4, &definition); err != nil {
		return definition, err
	}
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV4(definition); err != nil {
		return OpenRouterDirectShadowDefinitionV4{}, err
	}
	return definition, nil
}

func OpenRouterDirectShadowDefinitionV4SHA256() string {
	return sha256Hex(openRouterDirectShadowDefinitionRawV4)
}

type openRouterDirectShadowV3TerminalAntecedent struct {
	SchemaVersion                string `json:"schema_version"`
	Status                       string `json:"status"`
	DefinitionSHA256             string `json:"definition_sha256"`
	SuiteSHA256                  string `json:"suite_sha256"`
	RequestedModel               string `json:"requested_model"`
	ActualModel                  string `json:"actual_model"`
	RequestedServingProvider     string `json:"requested_serving_provider"`
	ActualServingProvider        string `json:"actual_serving_provider"`
	ActualEndpointModel          string `json:"actual_endpoint_model"`
	ExactRouteProved             bool   `json:"exact_route_proved"`
	RouterRequestCount           int    `json:"router_request_count"`
	ProviderInferenceAttempts    int    `json:"provider_inference_attempts"`
	RetryCount                   int    `json:"retry_count"`
	InputTokens                  int64  `json:"input_tokens"`
	OutputTokens                 int64  `json:"output_tokens"`
	ReasoningTokens              int64  `json:"reasoning_tokens"`
	ProviderReportedCostMicros   int64  `json:"provider_reported_cost_micros"`
	BeforeSpentMicros            int64  `json:"before_spent_micros"`
	AfterSpentMicros             int64  `json:"after_spent_micros"`
	ObservedIncrementMicros      int64  `json:"observed_increment_micros"`
	ResponseContractValid        bool   `json:"response_contract_valid"`
	ResponseContractError        string `json:"response_contract_error"`
	HarnessFailure               string `json:"harness_failure"`
	ResponseSHA256               string `json:"response_sha256"`
	RemainingWorkflowsNotCalled  int    `json:"remaining_workflows_not_called"`
	CredentialReads              int    `json:"credential_reads"`
	CredentialPersistenceSurface string `json:"credential_persistence_surface"`
	CredentialMemoryCleanup      string `json:"credential_memory_cleanup"`
	AuthorityExhausted           bool   `json:"authority_exhausted"`
	RetryAllowed                 bool   `json:"retry_allowed"`
}

func validateOpenRouterDirectShadowV3TerminalAntecedent(definition OpenRouterDirectShadowDefinitionV4) error {
	if sha256Hex(openRouterDirectShadowV3TerminalAntecedentRaw) != definition.AntecedentV3TerminalReceiptSHA256 {
		return fmt.Errorf("OpenRouter direct shadow v4 antecedent receipt hash drifted")
	}
	var receipt openRouterDirectShadowV3TerminalAntecedent
	if err := decodeExactJSONV1(openRouterDirectShadowV3TerminalAntecedentRaw, &receipt); err != nil {
		return err
	}
	if receipt.SchemaVersion != "silver-mediums.openrouter-direct-shadow-terminal.v3" || receipt.Status != "failed_closed_harness_metadata_shape_mismatch" || receipt.DefinitionSHA256 != definition.AntecedentV3DefinitionSHA256 || receipt.SuiteSHA256 != definition.SuiteSHA256 || receipt.RequestedModel != definition.Candidate.Model || receipt.ActualModel != definition.Candidate.Model || normalizeOpenRouterProviderV2(receipt.RequestedServingProvider) != normalizeOpenRouterProviderV2(definition.Candidate.ServingProvider) || normalizeOpenRouterProviderV2(receipt.ActualServingProvider) != normalizeOpenRouterProviderV2(definition.Candidate.ServingProvider) || receipt.ActualEndpointModel != definition.ExpectedEndpointModel || !receipt.ExactRouteProved || receipt.RouterRequestCount != 1 || receipt.ProviderInferenceAttempts != 1 || receipt.RetryCount != 0 || receipt.InputTokens != 321 || receipt.OutputTokens != 268 || receipt.ReasoningTokens != 0 || receipt.ProviderReportedCostMicros != 499 || receipt.BeforeSpentMicros != 111582154 || receipt.AfterSpentMicros != 111582154 || receipt.ObservedIncrementMicros != 0 || receipt.ResponseContractValid || receipt.ResponseContractError == "" || receipt.HarnessFailure == "" || receipt.ResponseSHA256 != definition.AntecedentV3ResponseSHA256 || receipt.RemainingWorkflowsNotCalled != len(definition.WorkflowIDs) || receipt.CredentialReads != 1 || receipt.CredentialPersistenceSurface != "none_by_construction_no_header_or_credential_artifact" || receipt.CredentialMemoryCleanup != "deferred_byte_zeroing_on_every_post_read_return" || !receipt.AuthorityExhausted || receipt.RetryAllowed {
		return fmt.Errorf("OpenRouter direct shadow v4 antecedent receipt drifted")
	}
	return nil
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowDefinitionV4(definition OpenRouterDirectShadowDefinitionV4) error {
	wantWorkflows := []string{"local-implementation-review-mobile", "local-release-regression-rollback", "local-interruption-recovery"}
	if definition.SchemaVersion != OpenRouterDirectShadowDefinitionSchemaV4 || definition.RunID != "item-14-openrouter-direct-shadow-v4" || definition.Status != "prepared_frozen_not_launched" || definition.Purpose != "finish the three unattempted frozen workflows without replaying the terminal first workflow" || definition.AntecedentV3DefinitionSHA256 != "b3241a97ac57418868c71059a6b443754e61069405025d0f3da56ad913372c5c" || definition.AntecedentV3ResponseSHA256 != "52e3ebb36be48988d3eaef1db17c20e73551173e5618e2338479066a2226b98e" || definition.AntecedentV3TerminalReceiptSHA256 != "116630044075634e7d6bf479fd5efd2c0cc6c33542e44f26eaa3bf4369780ff1" || definition.AntecedentWorkflowID != "local-intake-decision-coordination" || definition.AntecedentResponseContractValid || !definition.AntecedentExactRouteProved {
		return fmt.Errorf("OpenRouter direct shadow v4 identity or antecedent drifted")
	}
	if definition.SuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() || definition.EligibilityEvidenceSHA256 != "84660c35c92e780c3c00f744a2380683accfc0158dea161b8353953247211b46" {
		return fmt.Errorf("OpenRouter direct shadow v4 suite binding drifted")
	}
	candidate := definition.Candidate
	if candidate.ConfigurationID != "item14.fixture.qwen3codernext.direct.openrouter.v4" || candidate.Model != "openrouter/qwen/qwen3-coder-next" || candidate.APIModel != "qwen/qwen3-coder-next" || candidate.RuntimeProvider != "openrouter_direct_api" || candidate.ServingProvider != "alibaba" || candidate.ProviderFallbacks || !candidate.TemporaryFixtureOnly || definition.ExpectedEndpointModel != "qwen/qwen3-coder-next-2025-02-03" || ValidateOpenRouterEstablishedSmallModelRouteV2(candidate.Model, candidate.ServingProvider) != nil {
		return fmt.Errorf("OpenRouter direct shadow v4 candidate drifted")
	}
	if definition.Endpoint != "https://openrouter.ai/api/v1/chat/completions" || definition.Transport != OpenRouterDirectShadowTransportV4 || !reflect.DeepEqual(definition.WorkflowIDs, wantWorkflows) || definition.MaxProviderCalls != 3 || definition.CallsPerWorkflow != 1 || definition.RetryCount != 0 || definition.RedirectsAllowed || !definition.ProviderOrderExact || !definition.ProviderOnlyExact || !definition.RouterMetadataRequired || !definition.TopLevelProviderRequired || definition.ProviderAttemptEvidencePolicy != "absent_or_one_exact" || definition.ResponseValidation != "controller_strict_schema_after_exact_provider_response" || definition.GeneratedTokenCeilingPerCall != 1200 || definition.MaxInputTokensPerCall != 262144 || definition.PerCallReservationMicros != 80444 || definition.TotalPriceReservationMicros != 241332 || definition.TotalPriceReservationMicros != int64(definition.MaxProviderCalls)*definition.PerCallReservationMicros || definition.CumulativeBaselineMicros != 102104668 || definition.CumulativeCeilingMicros != 142104668 || definition.SliceIncrementalCeilingMicros != 1000000 || definition.CumulativeCeilingMicros-definition.CumulativeBaselineMicros != 40000000 {
		return fmt.Errorf("OpenRouter direct shadow v4 transport or spend boundary drifted")
	}
	if definition.ResponseContractFailurePolicy != "candidate_local_continue" || definition.IdentityOrProviderFailurePolicy != "global_halt_no_retry" || definition.ToolsAllowed || definition.RepositoryAccessAllowed || definition.ProductionAccessAllowed || definition.OutboundActionsAllowed || definition.PhoneFarmAccessAllowed || definition.RoleModelSelectionAllowed {
		return fmt.Errorf("OpenRouter direct shadow v4 capability boundary drifted")
	}
	wantRequestHashes := map[string]string{
		"local-implementation-review-mobile": "fdde405dc81ddc0142d04c47ab105a266180941eac6703144674b335b0926f57",
		"local-release-regression-rollback":  "a2e12b9d7dd223f1b9d978d7aab0db5741014672141432690baef04274e51d9e",
		"local-interruption-recovery":        "43f9467eaf515c75233c860506b0ac927fd8ed865e0161be39f0cc22f02b652c",
	}
	if !reflect.DeepEqual(definition.RequestSHA256ByWorkflow, wantRequestHashes) || definition.ResponseSchemaSHA256 != "2bbf1935488085fe0fb5cc8a63fa680da92a5df4819742c5c5084b330fcaa94c" {
		return fmt.Errorf("OpenRouter direct shadow v4 frozen plan hashes drifted")
	}
	if err := validateOpenRouterDirectShadowV3TerminalAntecedent(definition); err != nil {
		return err
	}
	return nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowPlansV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1) ([]OpenRouterDirectShadowPlanV2, error) {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV4(definition); err != nil {
		return nil, err
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return nil, err
	}
	byID := map[string]LocalShadowWorkflowV1{}
	for _, workflow := range suite.Workflows {
		byID[workflow.ID] = workflow
	}
	plans := make([]OpenRouterDirectShadowPlanV2, 0, 3)
	candidate := definition.Candidate
	for _, workflowID := range definition.WorkflowIDs {
		workflow, ok := byID[workflowID]
		if !ok {
			return nil, fmt.Errorf("OpenRouter direct shadow v4 workflow missing")
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
		request := map[string]any{"model": candidate.APIModel, "messages": []map[string]string{{"role": "system", "content": "Return only the exact JSON object requested by the supplied frozen workflow. No tools or external access."}, {"role": "user", "content": prompt}}, "max_tokens": definition.GeneratedTokenCeilingPerCall, "temperature": 0, "stream": false, "provider": map[string]any{"order": []string{candidate.ServingProvider}, "only": []string{candidate.ServingProvider}, "allow_fallbacks": false}}
		requestBody, _ := json.Marshal(request)
		plan := OpenRouterDirectShadowPlanV2{WorkflowID: workflow.ID, Model: candidate.Model, APIModel: candidate.APIModel, ServingProvider: candidate.ServingProvider, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)), Schema: schema, SchemaSHA256: sha256Hex(schema), RequestBody: requestBody, RequestSHA256: sha256Hex(requestBody), MaxOutputTokens: definition.GeneratedTokenCeilingPerCall}
		if err := ctxContracts.ValidateOpenRouterDirectShadowPlanV4(definition, plan); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowPlanV4(definition OpenRouterDirectShadowDefinitionV4, plan OpenRouterDirectShadowPlanV2) error {
	if err := ctxContracts.ValidateOpenRouterDirectShadowDefinitionV4(definition); err != nil {
		return err
	}
	expectedRequestSHA256, workflowAllowed := definition.RequestSHA256ByWorkflow[plan.WorkflowID]
	if !workflowAllowed || plan.Model != definition.Candidate.Model || plan.APIModel != definition.Candidate.APIModel || plan.ServingProvider != definition.Candidate.ServingProvider || plan.ProviderFallback || plan.MaxOutputTokens != definition.GeneratedTokenCeilingPerCall || plan.Prompt == "" || plan.PromptSHA256 != sha256Hex([]byte(plan.Prompt)) || plan.SchemaSHA256 != sha256Hex(plan.Schema) || plan.SchemaSHA256 != definition.ResponseSchemaSHA256 || plan.RequestSHA256 != sha256Hex(plan.RequestBody) || plan.RequestSHA256 != expectedRequestSHA256 {
		return fmt.Errorf("OpenRouter direct shadow v4 plan drifted")
	}
	var body struct {
		Model       string                           `json:"model"`
		Messages    []struct{ Role, Content string } `json:"messages"`
		MaxTokens   int                              `json:"max_tokens"`
		Temperature int                              `json:"temperature"`
		Stream      bool                             `json:"stream"`
		Provider    struct {
			Order, Only    []string
			AllowFallbacks bool `json:"allow_fallbacks"`
		} `json:"provider"`
	}
	decoder := json.NewDecoder(bytes.NewReader(plan.RequestBody))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF || body.Model != plan.APIModel || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[0].Content != "Return only the exact JSON object requested by the supplied frozen workflow. No tools or external access." || body.Messages[1].Role != "user" || body.Messages[1].Content != plan.Prompt || body.MaxTokens != 1200 || body.Temperature != 0 || body.Stream || !reflect.DeepEqual(body.Provider.Order, []string{plan.ServingProvider}) || !reflect.DeepEqual(body.Provider.Only, []string{plan.ServingProvider}) || body.Provider.AllowFallbacks {
		return fmt.Errorf("OpenRouter direct shadow v4 request boundary drifted")
	}
	return nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowObservationV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, raw []byte, providerRequests int, startedAt, completedAt string, durationMillis int64) (OpenRouterDirectShadowObservationV2Record, error) {
	record := OpenRouterDirectShadowObservationV2Record{SchemaVersion: OpenRouterDirectShadowObservationV4, WorkflowID: plan.WorkflowID, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256, RequestSHA256: plan.RequestSHA256, ResponseSHA256: sha256Hex(raw), RequestedModel: plan.Model, RequestedServingProvider: plan.ServingProvider, ProviderRequestCount: providerRequests, StartedAt: startedAt, CompletedAt: completedAt, DurationMillis: durationMillis}
	if ctxContracts.ValidateOpenRouterDirectShadowPlanV4(definition, plan) != nil || providerRequests != 1 || len(raw) == 0 {
		return record, fmt.Errorf("OpenRouter direct shadow v4 plan or request evidence drifted")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var response openRouterDirectResponseV2
	if decoder.Decode(&response) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return record, fmt.Errorf("OpenRouter direct shadow v4 response invalid")
	}
	record.ProviderResponseID, record.ActualModel = response.ID, "openrouter/"+response.Model
	if response.ID == "" || response.Object != "chat.completion" || response.Model != plan.APIModel || normalizeOpenRouterProviderV2(response.Provider) != normalizeOpenRouterProviderV2(plan.ServingProvider) || response.Metadata.Requested != plan.APIModel || response.Metadata.Attempt != 1 || len(response.Metadata.Attempts) > 1 || len(response.Choices) != 1 || response.Choices[0].Index != 0 || response.Choices[0].Message.Role != "assistant" || !guardedExactAbsentNullOrEmptyArrayV2(response.Choices[0].Message.ToolCalls) {
		return record, fmt.Errorf("OpenRouter direct shadow v4 exact route or assistant evidence drifted")
	}
	if len(response.Metadata.Attempts) == 1 {
		attempt := response.Metadata.Attempts[0]
		if normalizeOpenRouterProviderV2(attempt.Provider) != normalizeOpenRouterProviderV2(plan.ServingProvider) || attempt.Model != definition.ExpectedEndpointModel {
			return record, fmt.Errorf("OpenRouter direct shadow v4 provider attempt drifted")
		}
	}
	selected := 0
	for _, endpoint := range response.Metadata.Endpoints.Available {
		if endpoint.Selected {
			if normalizeOpenRouterProviderV2(endpoint.Provider) != normalizeOpenRouterProviderV2(plan.ServingProvider) || endpoint.Model != definition.ExpectedEndpointModel {
				return record, fmt.Errorf("OpenRouter direct shadow v4 selected endpoint drifted")
			}
			selected++
		}
	}
	if selected != 1 {
		return record, fmt.Errorf("OpenRouter direct shadow v4 selected endpoint count drifted")
	}
	record.ActualServingProvider, record.RouterAttemptCount = plan.ServingProvider, 1
	if response.Usage.PromptTokens < 1 || response.Usage.CompletionTokens < 1 || response.Usage.TotalTokens < response.Usage.PromptTokens+response.Usage.CompletionTokens || response.Usage.PromptTokens > definition.MaxInputTokensPerCall || response.Usage.CompletionTokens > int64(definition.GeneratedTokenCeilingPerCall) {
		return record, fmt.Errorf("OpenRouter direct shadow v4 usage drifted")
	}
	cost, err := decimalUSDMicrosCeilingV2(response.Usage.Cost)
	if err != nil || cost > definition.PerCallReservationMicros {
		return record, fmt.Errorf("OpenRouter direct shadow v4 cost drifted")
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
		return record, fmt.Errorf("OpenRouter direct shadow v4 workflow missing")
	}
	parsed, contractErr := ValidateOpenRouterLocalShadowResponseV1(*workflow, definition.SuiteSHA256, []byte(response.Choices[0].Message.Content))
	if contractErr != nil || response.Choices[0].FinishReason != "stop" {
		record.Status, record.ResponseContractError = "candidate_response_contract_failure", "response did not match the frozen field schema"
		if contractErr == nil {
			record.ResponseContractError = "response did not stop normally"
		}
		return record, nil
	}
	record.Status, record.ResponseContractValid, record.Response = "passed", true, &parsed
	return record, nil
}

type OpenRouterDirectShadowResponseAccountingV4 struct {
	ResponseSHA256             string `json:"response_sha256"`
	ActualModel                string `json:"actual_model,omitempty"`
	ActualServingProvider      string `json:"actual_serving_provider,omitempty"`
	InputTokens                int64  `json:"input_tokens"`
	CachedInputTokens          int64  `json:"cached_input_tokens"`
	OutputTokens               int64  `json:"output_tokens"`
	ReasoningTokens            int64  `json:"reasoning_tokens"`
	ProviderReportedCostMicros int64  `json:"provider_reported_cost_micros"`
	UsageObserved              bool   `json:"usage_observed"`
}

// InspectOpenRouterDirectShadowResponseAccountingV4 extracts only immutable
// identity and accounting fields. It deliberately does not treat them as an
// exact-route verdict, so a failed identity/provider observation can still be
// settled without being promoted into valid workflow evidence.
func InspectOpenRouterDirectShadowResponseAccountingV4(raw []byte) (OpenRouterDirectShadowResponseAccountingV4, error) {
	receipt := OpenRouterDirectShadowResponseAccountingV4{ResponseSHA256: sha256Hex(raw)}
	if len(raw) == 0 {
		return receipt, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var response openRouterDirectResponseV2
	if err := decoder.Decode(&response); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return receipt, nil
	}
	receipt.ActualModel = response.Model
	receipt.ActualServingProvider = response.Provider
	receipt.InputTokens = response.Usage.PromptTokens
	receipt.CachedInputTokens = response.Usage.PromptTokensDetails.CachedTokens
	receipt.OutputTokens = response.Usage.CompletionTokens
	receipt.ReasoningTokens = response.Usage.CompletionTokensDetails.ReasoningTokens
	if response.Usage.Cost.String() != "" {
		cost, err := decimalUSDMicrosCeilingV2(response.Usage.Cost)
		if err != nil {
			return receipt, fmt.Errorf("OpenRouter direct shadow v4 terminal cost invalid")
		}
		receipt.ProviderReportedCostMicros = cost
	}
	receipt.UsageObserved = response.Usage.PromptTokens > 0 || response.Usage.CompletionTokens > 0 || response.Usage.Cost.String() != ""
	return receipt, nil
}

func (ctxContracts QualificationContracts) BuildOpenRouterDirectShadowRunV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1, observations []OpenRouterDirectShadowObservationV2Record, fake bool) (OpenRouterDirectShadowRunV4, error) {
	run := OpenRouterDirectShadowRunV4{SchemaVersion: OpenRouterDirectShadowRunSchemaV4, RunID: definition.RunID, ExecutionMode: "real_direct_provider", DefinitionSHA256: OpenRouterDirectShadowDefinitionV4SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, ExpectedEndpointModel: definition.ExpectedEndpointModel, ProviderRequestCount: len(observations), AntecedentFailureCount: 1, TotalEvaluatedWorkflows: 4, CandidateQualified: false, AntecedentInputTokens: 321, AntecedentOutputTokens: 268, AntecedentCostMicros: 499, Observations: append([]OpenRouterDirectShadowObservationV2Record(nil), observations...)}
	if fake {
		run.ExecutionMode, run.ProviderRequestCount, run.SimulatedResponseCount = "deterministic_fake_provider_free", 0, len(observations)
	}
	for _, observation := range observations {
		if observation.Status == "passed" {
			run.CurrentPassCount++
		} else {
			run.CurrentFailureCount++
		}
		run.CurrentInputTokens += observation.InputTokens
		run.CurrentCachedInputTokens += observation.CachedInputTokens
		run.CurrentOutputTokens += observation.OutputTokens
		run.CurrentReasoningTokens += observation.ReasoningTokens
		run.CurrentCostMicros += observation.ProviderReportedCostMicros
	}
	run.CombinedInputTokens, run.CombinedOutputTokens, run.CombinedCostMicros = run.AntecedentInputTokens+run.CurrentInputTokens, run.AntecedentOutputTokens+run.CurrentOutputTokens, run.AntecedentCostMicros+run.CurrentCostMicros
	if err := ctxContracts.ValidateOpenRouterDirectShadowRunV4(definition, suite, run); err != nil {
		return OpenRouterDirectShadowRunV4{}, err
	}
	return run, nil
}

func (ctxContracts QualificationContracts) ValidateOpenRouterDirectShadowRunV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1, run OpenRouterDirectShadowRunV4) error {
	if ctxContracts.ValidateOpenRouterDirectShadowDefinitionV4(definition) != nil || ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite) != nil || run.SchemaVersion != OpenRouterDirectShadowRunSchemaV4 || run.RunID != definition.RunID || run.DefinitionSHA256 != OpenRouterDirectShadowDefinitionV4SHA256() || run.SuiteSHA256 != definition.SuiteSHA256 || run.Model != definition.Candidate.Model || run.ServingProvider != definition.Candidate.ServingProvider || run.ExpectedEndpointModel != definition.ExpectedEndpointModel || len(run.Observations) != 3 || run.CurrentPassCount+run.CurrentFailureCount != 3 || run.AntecedentFailureCount != 1 || run.TotalEvaluatedWorkflows != 4 || run.CandidateQualified || run.RetryCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.RoleModelSelectionCount != 0 {
		return fmt.Errorf("OpenRouter direct shadow v4 run boundary drifted")
	}
	if run.ExecutionMode == "deterministic_fake_provider_free" {
		if run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 3 {
			return fmt.Errorf("OpenRouter direct shadow v4 fake counts drifted")
		}
	} else if run.ExecutionMode == "real_direct_provider" {
		if run.ProviderRequestCount != 3 || run.SimulatedResponseCount != 0 {
			return fmt.Errorf("OpenRouter direct shadow v4 real counts drifted")
		}
	} else {
		return fmt.Errorf("OpenRouter direct shadow v4 mode drifted")
	}
	seen := map[string]struct{}{}
	var input, cached, output, reasoning, cost int64
	for index, observation := range run.Observations {
		if observation.SchemaVersion != OpenRouterDirectShadowObservationV4 || observation.WorkflowID != definition.WorkflowIDs[index] || observation.RequestedModel != definition.Candidate.Model || observation.ActualModel != definition.Candidate.Model || observation.RequestedServingProvider != definition.Candidate.ServingProvider || observation.ActualServingProvider != definition.Candidate.ServingProvider || observation.RouterAttemptCount != 1 || observation.ProviderRequestCount != 1 || observation.RetryCount != 0 || observation.ProviderResponseID == "" || !validSHA256(observation.ResponseSHA256) || observation.InputTokens < 1 || observation.OutputTokens < 1 || observation.ProviderReportedCostMicros < 0 || observation.ProviderReportedCostMicros > definition.PerCallReservationMicros {
			return fmt.Errorf("OpenRouter direct shadow v4 observation drifted")
		}
		if _, exists := seen[observation.ProviderResponseID]; exists {
			return fmt.Errorf("OpenRouter direct shadow v4 duplicate response")
		}
		seen[observation.ProviderResponseID] = struct{}{}
		if observation.Status == "passed" {
			if !observation.ResponseContractValid || observation.Response == nil {
				return fmt.Errorf("OpenRouter direct shadow v4 pass drifted")
			}
		} else if observation.Status == "candidate_response_contract_failure" {
			if observation.ResponseContractValid || observation.Response != nil || observation.ResponseContractError == "" {
				return fmt.Errorf("OpenRouter direct shadow v4 failure drifted")
			}
		} else {
			return fmt.Errorf("OpenRouter direct shadow v4 status drifted")
		}
		input += observation.InputTokens
		cached += observation.CachedInputTokens
		output += observation.OutputTokens
		reasoning += observation.ReasoningTokens
		cost += observation.ProviderReportedCostMicros
	}
	if input != run.CurrentInputTokens || cached != run.CurrentCachedInputTokens || output != run.CurrentOutputTokens || reasoning != run.CurrentReasoningTokens || cost != run.CurrentCostMicros || run.CombinedInputTokens != input+321 || run.CombinedOutputTokens != output+268 || run.CombinedCostMicros != cost+499 || cost > definition.SliceIncrementalCeilingMicros {
		return fmt.Errorf("OpenRouter direct shadow v4 totals drifted")
	}
	return nil
}
