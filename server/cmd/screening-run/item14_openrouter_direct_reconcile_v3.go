package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type item14OpenRouterDirectTerminalV3 struct {
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

func reconcileItem14OpenRouterDirectV3(root string) (item14OpenRouterDirectTerminalV3, error) {
	var terminal item14OpenRouterDirectTerminalV3
	definition, suite, plans, err := loadItem14OpenRouterDirectPlansV3()
	if err != nil {
		return terminal, err
	}
	if !filepath.IsAbs(root) || filepath.Base(root) != "item14-openrouter-direct-shadow-v3" {
		return terminal, fmt.Errorf("OpenRouter direct v3 reconciliation root drifted")
	}
	terminalPath := filepath.Join(root, "terminal-failure-receipt.json")
	if _, err := os.Stat(terminalPath); err == nil || !os.IsNotExist(err) {
		return terminal, fmt.Errorf("OpenRouter direct v3 terminal receipt already exists or cannot be proven absent")
	}
	workflowDir := filepath.Join(root, "01-"+plans[0].WorkflowID)
	var halt struct {
		Class         string `json:"class"`
		Error         string `json:"error"`
		ProviderCalls int    `json:"provider_calls"`
		RetryAllowed  bool   `json:"retry_allowed"`
	}
	if err := readExactJSONFileV2(filepath.Join(workflowDir, "global-halt.json"), &halt); err != nil {
		return terminal, err
	}
	if halt.Class != "identity_provider_or_evidence_failure" || halt.Error != "OpenRouter direct v3 response model or metadata drifted" || halt.ProviderCalls != 1 || halt.RetryAllowed {
		return terminal, fmt.Errorf("OpenRouter direct v3 halt evidence drifted")
	}
	raw, err := os.ReadFile(filepath.Join(workflowDir, "provider-response.json"))
	if err != nil {
		return terminal, err
	}
	if agentqualification.GuardedPromptSHA256V1(string(raw)) != "52e3ebb36be48988d3eaef1db17c20e73551173e5618e2338479066a2226b98e" {
		return terminal, fmt.Errorf("OpenRouter direct v3 response hash drifted")
	}
	var response struct {
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
			PromptTokens            int64       `json:"prompt_tokens"`
			CompletionTokens        int64       `json:"completion_tokens"`
			Cost                    json.Number `json:"cost"`
			CompletionTokensDetails struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
		Metadata struct {
			Requested string `json:"requested"`
			Attempt   int    `json:"attempt"`
			Endpoints struct {
				Available []struct {
					Provider string `json:"provider"`
					Model    string `json:"model"`
					Selected bool   `json:"selected"`
				} `json:"available"`
			} `json:"endpoints"`
			Attempts []json.RawMessage `json:"attempts"`
		} `json:"openrouter_metadata"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil {
		return terminal, err
	}
	if response.ID == "" || response.Object != "chat.completion" || response.Model != definition.Candidate.APIModel || !strings.EqualFold(response.Provider, definition.Candidate.ServingProvider) || response.Metadata.Requested != definition.Candidate.APIModel || response.Metadata.Attempt != 1 || len(response.Metadata.Attempts) != 0 || len(response.Choices) != 1 || response.Choices[0].Index != 0 || response.Choices[0].FinishReason != "stop" || response.Choices[0].Message.Role != "assistant" || !guardedDirectRawAbsentV3(response.Choices[0].Message.ToolCalls) {
		return terminal, fmt.Errorf("OpenRouter direct v3 positive route response drifted")
	}
	selected := 0
	for _, endpoint := range response.Metadata.Endpoints.Available {
		if endpoint.Selected {
			if !strings.EqualFold(endpoint.Provider, definition.Candidate.ServingProvider) || endpoint.Model != definition.ExpectedEndpointModel {
				return terminal, fmt.Errorf("OpenRouter direct v3 selected endpoint drifted")
			}
			selected++
		}
	}
	if selected != 1 {
		return terminal, fmt.Errorf("OpenRouter direct v3 selected endpoint count drifted")
	}
	cost, err := ceilingUSDMicros(response.Usage.Cost.String())
	if err != nil || cost != 499 || response.Usage.PromptTokens != 321 || response.Usage.CompletionTokens != 268 || response.Usage.CompletionTokensDetails.ReasoningTokens != 0 {
		return terminal, fmt.Errorf("OpenRouter direct v3 usage drifted")
	}
	var workflow agentqualification.LocalShadowWorkflowV1
	for _, candidate := range suite.Workflows {
		if candidate.ID == plans[0].WorkflowID {
			workflow = candidate
			break
		}
	}
	_, contractErr := agentqualification.ValidateOpenRouterLocalShadowResponseV1(workflow, definition.SuiteSHA256, []byte(response.Choices[0].Message.Content))
	if contractErr == nil {
		return terminal, fmt.Errorf("OpenRouter direct v3 response unexpectedly passed the frozen contract")
	}
	var before, after agentqualification.APISpendSnapshot
	if err := readExactJSONFileV2(filepath.Join(root, "spend-before.json"), &before); err != nil {
		return terminal, err
	}
	if err := readExactJSONFileV2(filepath.Join(workflowDir, "spend-after.json"), &after); err != nil {
		return terminal, err
	}
	if before.SpentMicros != 111582154 || after.SpentMicros != before.SpentMicros {
		return terminal, fmt.Errorf("OpenRouter direct v3 account snapshot drifted")
	}
	terminal = item14OpenRouterDirectTerminalV3{
		SchemaVersion: "silver-mediums.openrouter-direct-shadow-terminal.v3", Status: "failed_closed_harness_metadata_shape_mismatch", DefinitionSHA256: agentqualification.OpenRouterDirectShadowDefinitionV3SHA256(), SuiteSHA256: definition.SuiteSHA256,
		RequestedModel: definition.Candidate.Model, ActualModel: definition.Candidate.Model, RequestedServingProvider: definition.Candidate.ServingProvider, ActualServingProvider: definition.Candidate.ServingProvider, ActualEndpointModel: definition.ExpectedEndpointModel,
		ExactRouteProved: true, RouterRequestCount: 1, ProviderInferenceAttempts: 1, RetryCount: 0, InputTokens: response.Usage.PromptTokens, OutputTokens: response.Usage.CompletionTokens, ReasoningTokens: 0, ProviderReportedCostMicros: cost,
		BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: after.SpentMicros, ObservedIncrementMicros: 0, ResponseContractValid: false, ResponseContractError: "response did not match the frozen field schema", HarnessFailure: "successful router metadata omitted the documented attempts array; top-level provider and selected endpoint still proved the exact route",
		ResponseSHA256: "52e3ebb36be48988d3eaef1db17c20e73551173e5618e2338479066a2226b98e", RemainingWorkflowsNotCalled: 3, CredentialReads: 1, CredentialPersistenceSurface: "none_by_construction_no_header_or_credential_artifact", CredentialMemoryCleanup: "deferred_byte_zeroing_on_every_post_read_return", AuthorityExhausted: true, RetryAllowed: false,
	}
	if err := writeExclusiveJSON(terminalPath, terminal); err != nil {
		return item14OpenRouterDirectTerminalV3{}, err
	}
	accounting := item14OpenRouterDirectAccountingV2{SchemaVersion: "silver-mediums.openrouter-direct-shadow-accounting.v3", Provider: before.Provider, ProjectLane: before.ProjectLane, WindowID: before.WindowID, BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: after.SpentMicros, ObservedIncrementMicros: 0, ReceiptCostMicros: cost, SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeOpenRouterCeilingMicros, BeforeObservedAt: before.ObservedAt, AfterObservedAt: after.ObservedAt, Source: after.Source, CredentialReads: 1, CredentialCopyPersisted: false, CredentialMemoryZeroed: true}
	if err := writeExclusiveJSON(filepath.Join(root, "accounting.json"), accounting); err != nil {
		return item14OpenRouterDirectTerminalV3{}, err
	}
	return terminal, nil
}

func guardedDirectRawAbsentV3(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]"))
}
