package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type item14OpenRouterDirectTerminalV2 struct {
	SchemaVersion                string   `json:"schema_version"`
	Status                       string   `json:"status"`
	DefinitionSHA256             string   `json:"definition_sha256"`
	SuiteSHA256                  string   `json:"suite_sha256"`
	RequestedModel               string   `json:"requested_model"`
	RequestedServingProvider     string   `json:"requested_serving_provider"`
	ObservedAvailableModel       string   `json:"observed_available_model"`
	ObservedAvailableProvider    string   `json:"observed_available_provider"`
	RouterRequestCount           int      `json:"router_request_count"`
	ProviderInferenceAttempts    int      `json:"provider_inference_attempts"`
	RetryCount                   int      `json:"retry_count"`
	CredentialReads              int      `json:"credential_reads"`
	CredentialPersistenceSurface string   `json:"credential_persistence_surface"`
	CredentialMemoryCleanup      string   `json:"credential_memory_cleanup"`
	BeforeSpentMicros            int64    `json:"before_spent_micros"`
	AfterSpentMicros             int64    `json:"after_spent_micros"`
	ObservedIncrementMicros      int64    `json:"observed_increment_micros"`
	ResponseSHA256               string   `json:"response_sha256"`
	RemainingWorkflowsNotCalled  int      `json:"remaining_workflows_not_called"`
	AuthorityExhausted           bool     `json:"authority_exhausted"`
	RetryAllowed                 bool     `json:"retry_allowed"`
	EvidenceFiles                []string `json:"evidence_files"`
}

func reconcileItem14OpenRouterDirectV2(root string) (item14OpenRouterDirectTerminalV2, error) {
	var terminal item14OpenRouterDirectTerminalV2
	definition, _, plans, err := loadItem14OpenRouterDirectPlansV2()
	if err != nil {
		return terminal, err
	}
	if !filepath.IsAbs(root) || filepath.Base(root) != "item14-openrouter-direct-shadow-v2" {
		return terminal, fmt.Errorf("OpenRouter direct v2 reconciliation root drifted")
	}
	terminalPath := filepath.Join(root, "terminal-failure-receipt.json")
	if _, err := os.Stat(terminalPath); err == nil || !os.IsNotExist(err) {
		return terminal, fmt.Errorf("OpenRouter direct v2 terminal receipt already exists or cannot be proven absent")
	}
	required := []string{"provider-free-preflight.json", "run-claim.json", "spend-before.json", filepath.Join("01-"+plans[0].WorkflowID, "claim.json"), filepath.Join("01-"+plans[0].WorkflowID, "spend-before.json"), filepath.Join("01-"+plans[0].WorkflowID, "provider-response.json"), filepath.Join("01-"+plans[0].WorkflowID, "spend-after.json"), filepath.Join("01-"+plans[0].WorkflowID, "global-halt.json")}
	for _, relative := range required {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			return terminal, fmt.Errorf("OpenRouter direct v2 evidence missing %s", relative)
		}
	}
	for _, forbidden := range []string{"run.json", "accounting.json", "02-" + plans[1].WorkflowID, "03-" + plans[2].WorkflowID, "04-" + plans[3].WorkflowID} {
		if _, err := os.Stat(filepath.Join(root, forbidden)); !os.IsNotExist(err) {
			return terminal, fmt.Errorf("OpenRouter direct v2 terminal evidence unexpectedly contains %s", forbidden)
		}
	}
	var halt struct {
		Class               string `json:"class"`
		Error               string `json:"error"`
		ProviderCallsAtMost int    `json:"provider_calls_at_most"`
		RetryAllowed        bool   `json:"retry_allowed"`
	}
	if err := readExactJSONFileV2(filepath.Join(root, "01-"+plans[0].WorkflowID, "global-halt.json"), &halt); err != nil {
		return terminal, err
	}
	if halt.Class != "transport_or_http_failure" || halt.Error != "OpenRouter direct request returned status 404" || halt.ProviderCallsAtMost != 1 || halt.RetryAllowed {
		return terminal, fmt.Errorf("OpenRouter direct v2 halt receipt drifted")
	}
	rawResponse, err := os.ReadFile(filepath.Join(root, "01-"+plans[0].WorkflowID, "provider-response.json"))
	if err != nil {
		return terminal, err
	}
	var response struct {
		Error struct {
			Code     int    `json:"code"`
			Message  string `json:"message"`
			Metadata struct {
				FailedRoutingStep string `json:"failed_routing_step"`
			} `json:"metadata"`
		} `json:"error"`
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
			Attempts []json.RawMessage `json:"attempts"`
		} `json:"openrouter_metadata"`
	}
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		return terminal, err
	}
	if response.Error.Code != 404 || !strings.Contains(response.Error.Message, "No endpoints found that can handle the requested parameters") || response.Error.Metadata.FailedRoutingStep != "Filter by Parameters" || response.Metadata.Requested != definition.Candidate.APIModel || response.Metadata.Strategy != "direct" || response.Metadata.Attempt != 0 || len(response.Metadata.Attempts) != 0 || response.Metadata.Endpoints.Total != 4 {
		return terminal, fmt.Errorf("OpenRouter direct v2 zero-attempt metadata drifted")
	}
	var matched bool
	for _, endpoint := range response.Metadata.Endpoints.Available {
		if endpoint.Selected {
			return terminal, fmt.Errorf("OpenRouter direct v2 unexpectedly selected an endpoint")
		}
		if strings.EqualFold(endpoint.Provider, definition.Candidate.ServingProvider) {
			if endpoint.Model != "qwen/qwen3-coder-next-2025-02-03" {
				return terminal, fmt.Errorf("OpenRouter direct v2 Alibaba endpoint model drifted")
			}
			matched = true
		}
	}
	if !matched {
		return terminal, fmt.Errorf("OpenRouter direct v2 did not report the approved provider as available")
	}
	var before, after agentqualification.APISpendSnapshot
	if err := readExactJSONFileV2(filepath.Join(root, "spend-before.json"), &before); err != nil {
		return terminal, err
	}
	if err := readExactJSONFileV2(filepath.Join(root, "01-"+plans[0].WorkflowID, "spend-after.json"), &after); err != nil {
		return terminal, err
	}
	if before.Provider != "openrouter" || before.ProjectLane != "direct_openrouter" || before.SpentMicros != after.SpentMicros || before.SpentMicros != 111582154 || after.Source != "authoritative_api_usage" {
		return terminal, fmt.Errorf("OpenRouter direct v2 accounting drifted")
	}
	terminal = item14OpenRouterDirectTerminalV2{
		SchemaVersion: "silver-mediums.openrouter-direct-shadow-terminal.v2", Status: "failed_closed_no_eligible_parameter_endpoint", DefinitionSHA256: agentqualification.OpenRouterDirectShadowDefinitionV2SHA256(), SuiteSHA256: definition.SuiteSHA256,
		RequestedModel: definition.Candidate.Model, RequestedServingProvider: definition.Candidate.ServingProvider, ObservedAvailableModel: "openrouter/qwen/qwen3-coder-next-2025-02-03", ObservedAvailableProvider: definition.Candidate.ServingProvider,
		RouterRequestCount: 1, ProviderInferenceAttempts: 0, RetryCount: 0, CredentialReads: 1, CredentialPersistenceSurface: "none_by_construction_no_header_or_credential_artifact", CredentialMemoryCleanup: "deferred_byte_zeroing_on_every_post_read_return",
		BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: after.SpentMicros, ObservedIncrementMicros: 0, ResponseSHA256: agentqualification.GuardedPromptSHA256V1(string(rawResponse)), RemainingWorkflowsNotCalled: 3, AuthorityExhausted: true, RetryAllowed: false, EvidenceFiles: required,
	}
	if terminal.ResponseSHA256 != "611dfac19c079fcd560ebf89f042049646a53312b0b9eac2bc14b4991aa04aa6" || !reflect.DeepEqual(terminal.EvidenceFiles, required) {
		return item14OpenRouterDirectTerminalV2{}, fmt.Errorf("OpenRouter direct v2 immutable terminal binding drifted")
	}
	if err := writeExclusiveJSON(terminalPath, terminal); err != nil {
		return item14OpenRouterDirectTerminalV2{}, err
	}
	accounting := item14OpenRouterDirectAccountingV2{SchemaVersion: "silver-mediums.openrouter-direct-shadow-accounting.v2", Provider: before.Provider, ProjectLane: before.ProjectLane, WindowID: before.WindowID, BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: after.SpentMicros, ObservedIncrementMicros: 0, ReceiptCostMicros: 0, SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeOpenRouterCeilingMicros, BeforeObservedAt: before.ObservedAt, AfterObservedAt: after.ObservedAt, Source: after.Source, CredentialReads: 1, CredentialCopyPersisted: false, CredentialMemoryZeroed: true}
	if err := writeExclusiveJSON(filepath.Join(root, "accounting.json"), accounting); err != nil {
		return item14OpenRouterDirectTerminalV2{}, err
	}
	return terminal, nil
}

func readExactJSONFileV2(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON evidence has trailing data")
	}
	return nil
}
