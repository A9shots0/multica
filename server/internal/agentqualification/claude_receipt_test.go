package agentqualification

import "testing"

func TestClaudeReceiptFixtureNormalizesModelUsageAndCost(t *testing.T) {
	raw := []byte(`{"type":"result","subtype":"success","is_error":false,"result":"","structured_output":{"owner_outcome":"restore access"},"session_id":"11111111-1111-4111-8111-111111111111","total_cost_usd":0.012345,"usage":{"input_tokens":100,"cache_creation_input_tokens":20,"cache_read_input_tokens":80,"output_tokens":25},"modelUsage":{"claude-sonnet-4-6":{"inputTokens":100}}}`)
	result, err := ParseClaudeCodeJSONResult(raw, "2026-09-11T12:00:00Z", "2026-09-11T12:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if result.ReportedModel != "claude-sonnet-4-6" || *result.Usage.InputTokens != 200 || *result.Usage.CachedInputTokens != 80 || *result.Usage.ProviderReportedCostMicros != 12345 || result.Usage.ReasoningTokens != nil || result.Usage.ReasoningTokensStatus != "provider_unavailable" {
		t.Fatalf("unexpected normalized result: %+v", result)
	}
}

func TestClaudeReceiptFixtureRejectsAliasAmbiguityAndCostRounding(t *testing.T) {
	multiple := []byte(`{"type":"result","subtype":"success","is_error":false,"structured_output":{},"session_id":"x","total_cost_usd":0.01,"usage":{"input_tokens":1,"output_tokens":1},"modelUsage":{"model-a":{},"model-b":{}}}`)
	if _, err := ParseClaudeCodeJSONResult(multiple, "2026-09-11T12:00:00Z", "2026-09-11T12:00:01Z"); err == nil {
		t.Fatal("multiple reported models passed")
	}
	tooPrecise := []byte(`{"type":"result","subtype":"success","is_error":false,"structured_output":{},"session_id":"x","total_cost_usd":0.0000001,"usage":{"input_tokens":1,"output_tokens":1},"modelUsage":{"model-a":{}}}`)
	if _, err := ParseClaudeCodeJSONResult(tooPrecise, "2026-09-11T12:00:00Z", "2026-09-11T12:00:01Z"); err == nil {
		t.Fatal("imprecise cost rounding passed")
	}
}

func TestClaudeSubscriptionReceiptPreservesSubMicroCostByCeiling(t *testing.T) {
	raw := []byte(`{"type":"result","subtype":"success","is_error":false,"structured_output":{"owner_hypothesis_supported":false},"session_id":"x","total_cost_usd":0.0000001,"usage":{"input_tokens":1,"output_tokens":1},"modelUsage":{"claude-sonnet-4-6":{}}}`)
	result, _, err := ParseClaudeCodeSubscriptionJSONArtifact(raw, "2026-09-11T12:00:00Z", "2026-09-11T12:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage.ProviderReportedCostMicros == nil || *result.Usage.ProviderReportedCostMicros != 1 {
		t.Fatalf("unexpected rounded subscription estimate: %+v", result.Usage)
	}
}

func TestClaudeFixtureNormalizesIntoCompleteSealableReceipt(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	configuration := claudeTestConfiguration()
	plan, err := BuildClaudeScreeningPlan(configuration, suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"type":"result","subtype":"success","is_error":false,"structured_output":{"owner_outcome":"restore access"},"session_id":"11111111-1111-4111-8111-111111111111","total_cost_usd":0.012345,"usage":{"input_tokens":100,"cache_creation_input_tokens":20,"cache_read_input_tokens":80,"output_tokens":25},"modelUsage":{"claude-sonnet-4-6":{}}}`)
	result, err := ParseClaudeCodeJSONResult(raw, "2026-09-11T12:00:00Z", "2026-09-11T12:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	assignment := ScreeningAssignment{BatchID: "claude-fixture-batch", CaseID: plan.CaseID, CaseVersion: plan.CaseVersion, Attempt: 1, Configuration: configuration, CandidatePacketSHA256: suite.Cases[0].CandidatePacketSHA256, EvaluationBinding: "binding"}
	reservation := ScreeningReservation{SchemaVersion: ScreeningReservationSchemaVersion, BatchID: assignment.BatchID, EvaluationBinding: assignment.EvaluationBinding, CandidateID: configuration.ID, CaseID: assignment.CaseID, AdmissionInputSHA256: hash, ReservedAt: "2026-09-11T12:00:00Z", State: "reserved"}
	adapter := ExecutionAdapterIdentity{Provider: "anthropic", Runtime: "claude-code", RuntimeVersion: "2.1.263", Endpoint: "local-cli-print", BillingBoundary: "anthropic_subscription", AccountLane: "owner_subscription", ToolPolicy: "none", ProviderEventPolicyVersion: "claude-code-json-result.v1", ExecutorSHA256: hash, IsolationPolicySHA256: hash, EnvironmentAllowlistSHA256: hash}
	receipt, err := BuildExecutionReceipt(reservation, assignment, ClaudeExecutionPlanBinding(assignment.BatchID, plan), adapter, result)
	if err != nil {
		t.Fatal(err)
	}
	if reasons := ValidateExecutionReceipt(receipt); len(reasons) != 0 {
		t.Fatalf("normalized Claude receipt invalid: %v", reasons)
	}
}
