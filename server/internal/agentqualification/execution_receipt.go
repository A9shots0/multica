package agentqualification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ExecutionReceiptSchemaVersion = "silver-mediums.screening-execution-receipt.v1"

type ExecutionReceiptUsage struct {
	InputTokens                *int64 `json:"input_tokens"`
	CachedInputTokens          *int64 `json:"cached_input_tokens"`
	OutputTokens               *int64 `json:"output_tokens"`
	ReasoningTokens            *int64 `json:"reasoning_tokens,omitempty"`
	ReasoningTokensStatus      string `json:"reasoning_tokens_status"`
	APISpendMicros             *int64 `json:"api_spend_micros,omitempty"`
	ProviderReportedCostMicros *int64 `json:"provider_reported_cost_micros,omitempty"`
}

// ExecutionReceipt binds provider truth to the exact admitted configuration.
// A provider that does not expose a model build/version must say unavailable;
// it must never silently substitute the requested alias as reported truth.
type ExecutionReceipt struct {
	SchemaVersion              string                `json:"schema_version"`
	BatchID                    string                `json:"batch_id"`
	ObservationID              string                `json:"observation_id"`
	EvaluationBinding          string                `json:"evaluation_binding"`
	ReservationSHA256          string                `json:"reservation_sha256"`
	ConfigurationSHA256        string                `json:"configuration_sha256"`
	PromptSHA256               string                `json:"prompt_sha256"`
	SchemaSHA256               string                `json:"output_schema_sha256"`
	AdmissionInputSHA256       string                `json:"admission_input_sha256"`
	Provider                   string                `json:"provider"`
	Runtime                    string                `json:"runtime"`
	RuntimeVersion             string                `json:"runtime_version"`
	Endpoint                   string                `json:"endpoint"`
	BillingBoundary            string                `json:"billing_boundary"`
	AccountLane                string                `json:"account_lane"`
	RequestedModel             string                `json:"requested_model"`
	RequestedEffort            string                `json:"requested_effort"`
	ReportedModel              string                `json:"reported_model"`
	ReportedModelVersion       string                `json:"reported_model_version,omitempty"`
	ReportedModelVersionStatus string                `json:"reported_model_version_status"`
	RequestedServingProvider   string                `json:"requested_serving_provider,omitempty"`
	ProviderFallbacks          *bool                 `json:"provider_fallbacks,omitempty"`
	ReportedServingProvider    string                `json:"reported_serving_provider,omitempty"`
	ActualServingProvider      string                `json:"actual_serving_provider,omitempty"`
	ServingProviderEvidence    string                `json:"serving_provider_evidence,omitempty"`
	ProviderResponseID         string                `json:"provider_response_id"`
	StartedAt                  string                `json:"started_at"`
	CompletedAt                string                `json:"completed_at"`
	TerminalState              string                `json:"terminal_state"`
	ToolPolicy                 string                `json:"tool_policy"`
	ProviderEventPolicyVersion string                `json:"provider_event_policy_version"`
	ObservedProviderEventTypes []string              `json:"observed_provider_event_types"`
	ObservedToolEventTypes     []string              `json:"observed_tool_event_types,omitempty"`
	RawTraceSHA256             string                `json:"raw_trace_sha256"`
	OutputSHA256               string                `json:"output_sha256"`
	ExecutorSHA256             string                `json:"executor_sha256"`
	IsolationPolicySHA256      string                `json:"isolation_policy_sha256"`
	EnvironmentAllowlistSHA256 string                `json:"environment_allowlist_sha256"`
	Usage                      ExecutionReceiptUsage `json:"usage"`
}

func ValidateExecutionReceipt(receipt ExecutionReceipt) []string {
	reasons := []string{}
	add := func(reason string) { reasons = append(reasons, reason) }
	if receipt.SchemaVersion != ExecutionReceiptSchemaVersion {
		add("schema_version_invalid")
	}
	for name, value := range map[string]string{
		"batch_id": receipt.BatchID, "observation_id": receipt.ObservationID,
		"evaluation_binding": receipt.EvaluationBinding, "provider": receipt.Provider,
		"runtime": receipt.Runtime, "runtime_version": receipt.RuntimeVersion,
		"endpoint": receipt.Endpoint, "account_lane": receipt.AccountLane,
		"requested_model": receipt.RequestedModel, "requested_effort": receipt.RequestedEffort,
		"reported_model": receipt.ReportedModel, "provider_response_id": receipt.ProviderResponseID,
	} {
		if strings.TrimSpace(value) == "" {
			add(name + "_missing")
		}
	}
	for name, value := range map[string]string{
		"reservation": receipt.ReservationSHA256, "configuration": receipt.ConfigurationSHA256,
		"prompt": receipt.PromptSHA256, "schema": receipt.SchemaSHA256,
		"admission_input": receipt.AdmissionInputSHA256, "raw_trace": receipt.RawTraceSHA256,
		"output": receipt.OutputSHA256, "executor": receipt.ExecutorSHA256,
		"isolation_policy": receipt.IsolationPolicySHA256, "environment_allowlist": receipt.EnvironmentAllowlistSHA256,
	} {
		if !validSHA256(value) {
			add(name + "_sha256_invalid")
		}
	}
	if !contains([]string{"openai_api_metered", "chatgpt_codex_subscription", "anthropic_subscription", "opencode_zen_metered", "opencode_go_subscription", "openrouter_metered", "local_compute"}, receipt.BillingBoundary) {
		add("billing_boundary_invalid")
	}
	if !contains([]string{"reported", "unavailable_truthful"}, receipt.ReportedModelVersionStatus) {
		add("reported_model_version_status_invalid")
	}
	if receipt.ReportedModelVersionStatus == "reported" && strings.TrimSpace(receipt.ReportedModelVersion) == "" {
		add("reported_model_version_missing")
	}
	if receipt.ProviderEventPolicyVersion == "opencode-run-json-export-routing.v2" {
		if strings.TrimSpace(receipt.RequestedServingProvider) == "" {
			add("requested_serving_provider_missing")
		}
		if receipt.ProviderFallbacks == nil || *receipt.ProviderFallbacks {
			add("provider_fallbacks_not_disabled")
		}
		if strings.TrimSpace(receipt.ActualServingProvider) == "" {
			add("actual_serving_provider_missing")
		}
		if !contains([]string{"provider_reported_metadata", "enforced_single_route_no_fallback"}, receipt.ServingProviderEvidence) {
			add("serving_provider_evidence_invalid")
		}
	}
	if !contains([]string{"completed", "invalid", "ambiguous"}, receipt.TerminalState) {
		add("terminal_state_invalid")
	}
	started, startErr := time.Parse(time.RFC3339Nano, receipt.StartedAt)
	completed, completeErr := time.Parse(time.RFC3339Nano, receipt.CompletedAt)
	if startErr != nil || completeErr != nil || (startErr == nil && completeErr == nil && completed.Before(started)) {
		add("timestamps_invalid")
	}
	if receipt.ToolPolicy != "none" {
		add("tool_policy_invalid")
	} else if len(receipt.ObservedToolEventTypes) != 0 {
		add("tool_policy_violated")
	}
	if len(receipt.ObservedProviderEventTypes) == 0 {
		add("provider_event_manifest_missing")
	} else if allowed, ok := providerEventPolicies[receipt.ProviderEventPolicyVersion]; !ok {
		add("provider_event_policy_invalid")
	} else {
		seen := map[string]struct{}{}
		for _, eventType := range receipt.ObservedProviderEventTypes {
			if _, duplicate := seen[eventType]; duplicate {
				add("provider_event_manifest_duplicate")
			}
			seen[eventType] = struct{}{}
			if _, registered := allowed[eventType]; !registered {
				add("provider_event_unregistered:" + eventType)
			}
		}
	}
	for name, value := range map[string]*int64{
		"input_tokens": receipt.Usage.InputTokens, "cached_input_tokens": receipt.Usage.CachedInputTokens,
		"output_tokens": receipt.Usage.OutputTokens,
	} {
		if value == nil {
			add(name + "_unavailable")
		} else if *value < 0 {
			add(name + "_negative")
		}
	}
	if receipt.Usage.ReasoningTokensStatus != "reported" && receipt.Usage.ReasoningTokensStatus != "provider_unavailable" {
		add("reasoning_tokens_status_invalid")
	} else if receipt.Usage.ReasoningTokensStatus == "reported" {
		if receipt.Usage.ReasoningTokens == nil {
			add("reasoning_tokens_unavailable")
		} else if *receipt.Usage.ReasoningTokens < 0 {
			add("reasoning_tokens_negative")
		}
	} else if receipt.Usage.ReasoningTokens != nil {
		add("reasoning_tokens_status_conflict")
	}
	if receipt.Usage.ProviderReportedCostMicros != nil && *receipt.Usage.ProviderReportedCostMicros < 0 {
		add("provider_reported_cost_negative")
	}
	if receipt.BillingBoundary == "openai_api_metered" {
		if receipt.Usage.APISpendMicros == nil {
			add("api_spend_unavailable")
		} else if *receipt.Usage.APISpendMicros < 0 {
			add("api_spend_negative")
		}
	}
	return unique(reasons)
}

var providerEventPolicies = map[string]map[string]struct{}{
	"openai-responses-tool-free.v1": stringSet(
		"response.created", "response.queued", "response.in_progress", "response.completed", "response.failed", "response.incomplete",
		"response.output_item.added", "response.output_item.done", "response.content_part.added", "response.content_part.done",
		"response.output_text.delta", "response.output_text.done", "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done",
	),
	"codex-exec-jsonl.v1": stringSet(
		"thread.started", "turn.started", "item.started", "item.updated", "item.completed", "turn.completed", "turn.failed", "error",
	),
	"claude-code-json-result.v1":          stringSet("result"),
	"opencode-run-json-export.v1":         stringSet("step_start", "step_finish", "text"),
	"opencode-run-json-export-routing.v2": stringSet("step_start", "step_finish", "text"),
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// SealExecutionReceipt writes exactly once. A pre-existing receipt—complete or
// partial—is ambiguous and must be reconciled, never overwritten or retried.
func SealExecutionReceipt(reservationDir string, receipt ExecutionReceipt) (string, error) {
	if !filepath.IsAbs(reservationDir) {
		return "", fmt.Errorf("reservation directory must be absolute")
	}
	if reasons := ValidateExecutionReceipt(receipt); len(reasons) != 0 {
		return "", fmt.Errorf("execution receipt invalid: %s", strings.Join(reasons, ","))
	}
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(reservationDir, "execution-receipt.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return path, err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return path, err
	}
	if closeErr != nil {
		return path, closeErr
	}
	return path, nil
}
