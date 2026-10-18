package agentqualification

import (
	"os"
	"testing"
)

func validExecutionReceipt() ExecutionReceipt {
	zero, input, output, spend := int64(0), int64(100), int64(20), int64(1000)
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return ExecutionReceipt{
		SchemaVersion: ExecutionReceiptSchemaVersion, BatchID: "batch", ObservationID: "obs", EvaluationBinding: "binding",
		ReservationSHA256: hash, ConfigurationSHA256: hash, PromptSHA256: hash, SchemaSHA256: hash, AdmissionInputSHA256: hash,
		Provider: "openai", Runtime: "responses", RuntimeVersion: "v1", Endpoint: "/v1/responses",
		BillingBoundary: "openai_api_metered", AccountLane: "qualification-project", RequestedModel: "candidate", RequestedEffort: "medium",
		ReportedModel: "candidate", ReportedModelVersionStatus: "unavailable_truthful", ProviderResponseID: "resp_test",
		StartedAt: "2026-09-11T12:00:00Z", CompletedAt: "2026-09-11T12:00:01Z", TerminalState: "completed",
		ToolPolicy: "none", ProviderEventPolicyVersion: "openai-responses-tool-free.v1", ObservedProviderEventTypes: []string{"response.created", "response.completed"},
		RawTraceSHA256: hash, OutputSHA256: hash, ExecutorSHA256: hash, IsolationPolicySHA256: hash, EnvironmentAllowlistSHA256: hash,
		Usage: ExecutionReceiptUsage{InputTokens: &input, CachedInputTokens: &zero, OutputTokens: &output, ReasoningTokens: &zero, ReasoningTokensStatus: "reported", APISpendMicros: &spend},
	}
}

func TestExecutionReceiptRejectsUnknownProviderEvents(t *testing.T) {
	receipt := validExecutionReceipt()
	receipt.ObservedProviderEventTypes = append(receipt.ObservedProviderEventTypes, "response.future_tool.magic")
	reasons := ValidateExecutionReceipt(receipt)
	if !contains(reasons, "provider_event_unregistered:response.future_tool.magic") {
		t.Fatalf("unknown provider event was accepted: %v", reasons)
	}
}

func TestExecutionReceiptRequiresCompleteProvenance(t *testing.T) {
	receipt := validExecutionReceipt()
	if reasons := ValidateExecutionReceipt(receipt); len(reasons) != 0 {
		t.Fatalf("complete receipt rejected: %v", reasons)
	}
	receipt.Usage.InputTokens = nil
	receipt.ObservedToolEventTypes = []string{"future_tool"}
	reasons := ValidateExecutionReceipt(receipt)
	if !contains(reasons, "input_tokens_unavailable") || !contains(reasons, "tool_policy_violated") {
		t.Fatalf("incomplete or tool-using receipt accepted: %v", reasons)
	}
	receipt = validExecutionReceipt()
	receipt.ToolPolicy = "auto"
	if reasons := ValidateExecutionReceipt(receipt); !contains(reasons, "tool_policy_invalid") {
		t.Fatalf("non-preventive tool policy was accepted: %v", reasons)
	}
}

func TestExecutionReceiptSealsExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	receipt := validExecutionReceipt()
	path, err := SealExecutionReceipt(dir, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := SealExecutionReceipt(dir, receipt); err == nil {
		t.Fatal("receipt overwrite was accepted")
	}
}
