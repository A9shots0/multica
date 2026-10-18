package agentqualification

import (
	"strings"
	"testing"
)

func TestExecutionNormalizerRejectsServingProviderDrift(t *testing.T) {
	batch, err := OpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	assignment := assignments[0]
	var screeningCase ScreeningCase
	for _, candidateCase := range batch.Cases {
		if candidateCase.CaseID == assignment.CaseID && candidateCase.CaseVersion == assignment.CaseVersion {
			screeningCase = candidateCase
			break
		}
	}
	plan, err := BuildOpenCodePlanForTransport(batch.CandidateTransport, assignment.Configuration, screeningCase)
	if err != nil {
		t.Fatal(err)
	}
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	zero, input, output, spend := int64(0), int64(100), int64(20), int64(1000)
	reservation := ScreeningReservation{SchemaVersion: ScreeningReservationSchemaVersion, BatchID: batch.BatchID, EvaluationBinding: assignment.EvaluationBinding, CandidateID: assignment.Configuration.ID, CaseID: assignment.CaseID, AdmissionInputSHA256: hash, ReservedAt: "2026-09-12T12:00:00Z", State: "reserved"}
	adapter := ExecutionAdapterIdentity{Provider: "openrouter", Runtime: "opencode-cli", RuntimeVersion: "test", Endpoint: "local-cli-run-json", BillingBoundary: "openrouter_metered", AccountLane: "opencode_local_openrouter", ToolPolicy: "none", ProviderEventPolicyVersion: "opencode-run-json-export-routing.v2", ExecutorSHA256: hash, IsolationPolicySHA256: hash, EnvironmentAllowlistSHA256: hash}
	result := ProviderExecutionResult{ObservationID: "obs-provider-drift", ReportedModel: assignment.Configuration.Model, ReportedModelVersionStatus: "unavailable_truthful", ReportedServingProvider: "wrong-provider", ProviderResponseID: "resp", StartedAt: "2026-09-12T12:00:00Z", CompletedAt: "2026-09-12T12:00:01Z", TerminalState: "completed", ObservedProviderEventTypes: []string{"step_start", "step_finish", "text"}, RawTraceSHA256: hash, OutputSHA256: hash, Usage: ExecutionReceiptUsage{InputTokens: &input, CachedInputTokens: &zero, OutputTokens: &output, ReasoningTokens: &zero, ReasoningTokensStatus: "reported", APISpendMicros: &spend}}
	if _, err := BuildExecutionReceipt(reservation, assignment, OpenCodeExecutionPlanBinding(batch.BatchID, plan), adapter, result); err == nil {
		t.Fatal("serving-provider drift was normalized")
	}
	result.ReportedServingProvider = strings.ToUpper(assignment.Configuration.ServingProvider)
	if _, err := BuildExecutionReceipt(reservation, assignment, OpenCodeExecutionPlanBinding(batch.BatchID, plan), adapter, result); err != nil {
		t.Fatalf("exact serving provider was rejected: %v", err)
	}
}

func TestExecutionNormalizerBindsReservationAssignmentPlanAndProviderTruth(t *testing.T) {
	batch, err := CurrentScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	assignment := assignments[0]
	plan, err := BuildDirectScreeningPlan(batch, assignment.Configuration.ID, assignment.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	reservation := ScreeningReservation{SchemaVersion: ScreeningReservationSchemaVersion, BatchID: batch.BatchID, EvaluationBinding: assignment.EvaluationBinding, CandidateID: assignment.Configuration.ID, CaseID: assignment.CaseID, AdmissionInputSHA256: hash, ReservedAt: "2026-09-11T12:00:00Z", State: "reserved"}
	adapter := ExecutionAdapterIdentity{Provider: "openai", Runtime: "responses", RuntimeVersion: "v1", Endpoint: "/v1/responses", BillingBoundary: "openai_api_metered", AccountLane: "qualification-project", ToolPolicy: "none", ProviderEventPolicyVersion: "openai-responses-tool-free.v1", ExecutorSHA256: hash, IsolationPolicySHA256: hash, EnvironmentAllowlistSHA256: hash}
	zero, input, output, spend := int64(0), int64(100), int64(20), int64(1000)
	result := ProviderExecutionResult{ObservationID: "obs", ReportedModel: assignment.Configuration.Model, ReportedModelVersionStatus: "unavailable_truthful", ProviderResponseID: "resp", StartedAt: "2026-09-11T12:00:00Z", CompletedAt: "2026-09-11T12:00:01Z", TerminalState: "completed", ObservedProviderEventTypes: []string{"response.created", "response.completed"}, RawTraceSHA256: hash, OutputSHA256: hash, Usage: ExecutionReceiptUsage{InputTokens: &input, CachedInputTokens: &zero, OutputTokens: &output, ReasoningTokens: &zero, ReasoningTokensStatus: "reported", APISpendMicros: &spend}}
	binding := DirectExecutionPlanBinding(plan)
	receipt, err := BuildExecutionReceipt(reservation, assignment, binding, adapter, result)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.EvaluationBinding != assignment.EvaluationBinding || receipt.PromptSHA256 != plan.PromptSHA256 || receipt.ReportedModel != assignment.Configuration.Model {
		t.Fatalf("normalization lost binding: %+v", receipt)
	}

	bad := reservation
	bad.CandidateID = "different"
	if _, err := BuildExecutionReceipt(bad, assignment, binding, adapter, result); err == nil {
		t.Fatal("binding drift was normalized")
	}
	result.ReportedModel = "different/model"
	if _, err := BuildExecutionReceipt(reservation, assignment, binding, adapter, result); err == nil {
		t.Fatal("provider model drift was normalized")
	}
	result.ReportedModel = assignment.Configuration.Model
	result.ObservedProviderEventTypes = append(result.ObservedProviderEventTypes, "response.unknown")
	if _, err := BuildExecutionReceipt(reservation, assignment, binding, adapter, result); err == nil {
		t.Fatal("unknown provider event was normalized")
	}
}
