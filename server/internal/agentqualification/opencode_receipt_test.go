package agentqualification

import (
	"bytes"
	"testing"
)

const openCodeFixtureJSONL = `{"type":"step_start","timestamp":1789128000000,"sessionID":"ses_fixture","part":{"id":"p_start","sessionID":"ses_fixture","messageID":"msg_fixture","type":"step-start"}}
{"type":"text","timestamp":1789128000900,"sessionID":"ses_fixture","part":{"id":"p_text","sessionID":"ses_fixture","messageID":"msg_fixture","type":"text","text":"{\"owner_outcome\":\"restore access\"}"}}
{"type":"step_finish","timestamp":1789128001000,"sessionID":"ses_fixture","part":{"id":"p_finish","sessionID":"ses_fixture","messageID":"msg_fixture","type":"step-finish","reason":"stop","cost":0.012345,"tokens":{"input":100,"output":25,"reasoning":5,"cache":{"read":80,"write":20}}}}
`

const openCodeFixtureExport = `{"info":{"id":"ses_fixture"},"messages":[{"info":{"id":"msg_fixture","sessionID":"ses_fixture","role":"assistant","modelID":"test-model","providerID":"opencode","summary":false,"finish":"stop","cost":0.012345,"tokens":{"input":100,"output":25,"reasoning":5,"cache":{"read":80,"write":20}}},"parts":[{"id":"p_text","sessionID":"ses_fixture","messageID":"msg_fixture","type":"text","text":"{\"owner_outcome\":\"restore access\"}","metadata":{"openrouter_metadata":{"endpoints":{"available":[{"provider":"Fixture Provider","selected":true}]}}}},{"id":"p_finish","sessionID":"ses_fixture","messageID":"msg_fixture","type":"step-finish","cost":0.012345,"tokens":{"input":100,"output":25,"reasoning":5,"cache":{"read":80,"write":20}}}]}]}`

func TestOpenCodeReceiptFixtureBindsJSONLToSessionExport(t *testing.T) {
	result, err := ParseOpenCodeRunResult([]byte(openCodeFixtureJSONL), []byte(openCodeFixtureExport))
	if err != nil {
		t.Fatal(err)
	}
	if result.ReportedModel != "opencode/test-model" || *result.Usage.InputTokens != 200 || *result.Usage.CachedInputTokens != 80 || *result.Usage.OutputTokens != 25 || *result.Usage.ReasoningTokens != 5 || *result.Usage.ProviderReportedCostMicros != 12345 {
		t.Fatalf("unexpected normalized result: %+v", result)
	}
}

func TestProviderCostRoundsUpConservativelyToMicros(t *testing.T) {
	got, err := decimalUSDMicros("0.00036926")
	if err != nil || got != 370 {
		t.Fatalf("fine-grained provider cost was not conservatively rounded: %d %v", got, err)
	}
}

func TestOpenRouterSelectedProviderComesOnlyFromSelectedMetadata(t *testing.T) {
	raw := []byte(`{"openrouter_metadata":{"endpoints":{"available":[{"provider":"Other","selected":false},{"provider":"NVIDIA","selected":true}]}}}`)
	providers := openRouterSelectedProviders(raw)
	if len(providers) != 1 || providers[0] != "NVIDIA" {
		t.Fatalf("unexpected providers: %v", providers)
	}
}

func TestOpenCodeReceiptRejectsToolsUnknownEventsAndExportDrift(t *testing.T) {
	tool := []byte(`{"type":"tool_use","timestamp":1789128000500,"sessionID":"ses_fixture","part":{"sessionID":"ses_fixture","messageID":"msg_fixture","type":"tool"}}` + "\n")
	if _, err := ParseOpenCodeRunResult(append(tool, []byte(openCodeFixtureJSONL)...), []byte(openCodeFixtureExport)); err == nil {
		t.Fatal("tool event passed")
	}
	unknown := []byte(`{"type":"reasoning","timestamp":1789128000500,"sessionID":"ses_fixture","part":{"sessionID":"ses_fixture","messageID":"msg_fixture","type":"reasoning"}}` + "\n")
	if _, err := ParseOpenCodeRunResult(append(unknown, []byte(openCodeFixtureJSONL)...), []byte(openCodeFixtureExport)); err == nil {
		t.Fatal("unknown event passed")
	}
	drift := []byte(`{"info":{"id":"ses_fixture"},"messages":[{"info":{"id":"msg_fixture","sessionID":"ses_fixture","role":"assistant","modelID":"other-model","providerID":"other","finish":"stop","cost":0.02,"tokens":{"input":100,"output":25,"reasoning":5,"cache":{"read":80,"write":20}}},"parts":[]}]}`)
	if _, err := ParseOpenCodeRunResult([]byte(openCodeFixtureJSONL), drift); err == nil {
		t.Fatal("export drift passed")
	}
}

func TestOpenCodeEnvelopePreservesExactUsageWhenModelEmitsReasoningButNoText(t *testing.T) {
	jsonl := []byte(`{"type":"step_start","timestamp":1789128000000,"sessionID":"ses_empty","part":{"id":"p_start","sessionID":"ses_empty","messageID":"msg_empty","type":"step-start"}}
{"type":"step_finish","timestamp":1789128001000,"sessionID":"ses_empty","part":{"id":"p_finish","sessionID":"ses_empty","messageID":"msg_empty","type":"step-finish","reason":"stop","cost":0.00006131,"tokens":{"input":1413,"output":0,"reasoning":116,"cache":{"read":128,"write":0}}}}
`)
	exported := []byte(`{"info":{"id":"ses_empty"},"messages":[{"info":{"id":"msg_empty","sessionID":"ses_empty","role":"assistant","modelID":"openai/gpt-oss-20b","providerID":"openrouter","summary":false,"finish":"stop","cost":0.00006131,"tokens":{"input":1413,"output":0,"reasoning":116,"cache":{"read":128,"write":0}}},"parts":[{"id":"p_start","sessionID":"ses_empty","messageID":"msg_empty","type":"step-start"},{"id":"p_reasoning","sessionID":"ses_empty","messageID":"msg_empty","type":"reasoning","text":"candidate placed its proposed action here"},{"id":"p_finish","sessionID":"ses_empty","messageID":"msg_empty","type":"step-finish","cost":0.00006131,"tokens":{"input":1413,"output":0,"reasoning":116,"cache":{"read":128,"write":0}}}]}]}`)

	result, output, err := ParseOpenCodeRunEnvelope(jsonl, exported)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 0 || *result.Usage.InputTokens != 1541 || *result.Usage.OutputTokens != 0 || *result.Usage.ReasoningTokens != 116 || *result.Usage.ProviderReportedCostMicros != 62 {
		t.Fatalf("zero-text receipt was not preserved exactly: result=%+v output=%q", result, output)
	}
	if _, _, err := ParseOpenCodeRunArtifact(jsonl, exported); err == nil {
		t.Fatal("zero-text candidate output passed the JSON action contract")
	}
}

func TestOpenCodeEnvelopeAcceptsProviderRecordedRepeatedStartsForOneMessage(t *testing.T) {
	repeated := []byte(`{"type":"step_start","timestamp":1789128000000,"sessionID":"ses_fixture","part":{"id":"p_start_1","sessionID":"ses_fixture","messageID":"msg_fixture","type":"step-start"}}
{"type":"step_start","timestamp":1789128000100,"sessionID":"ses_fixture","part":{"id":"p_start_2","sessionID":"ses_fixture","messageID":"msg_fixture","type":"step-start"}}
{"type":"text","timestamp":1789128000900,"sessionID":"ses_fixture","part":{"id":"p_text","sessionID":"ses_fixture","messageID":"msg_fixture","type":"text","text":"{\"owner_outcome\":\"restore access\"}"}}
{"type":"step_finish","timestamp":1789128001000,"sessionID":"ses_fixture","part":{"id":"p_finish","sessionID":"ses_fixture","messageID":"msg_fixture","type":"step-finish","reason":"stop","cost":0.012345,"tokens":{"input":100,"output":25,"reasoning":5,"cache":{"read":80,"write":20}}}}
`)
	if _, _, err := ParseOpenCodeRunEnvelope(repeated, []byte(openCodeFixtureExport)); err != nil {
		t.Fatalf("same-message provider lifecycle starts were misclassified as transport loss: %v", err)
	}

	drifted := bytes.Replace(repeated, []byte(`"messageID":"msg_fixture","type":"step-start"`), []byte(`"messageID":"msg_other","type":"step-start"`), 1)
	if _, _, err := ParseOpenCodeRunEnvelope(drifted, []byte(openCodeFixtureExport)); err == nil {
		t.Fatal("cross-message repeated start passed")
	}
}

func TestOpenCodeFixtureNormalizesIntoCompleteReceipt(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	configuration := openCodeTestConfiguration()
	plan, err := BuildOpenCodeScreeningPlan(configuration, suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	result, err := ParseOpenCodeRunResult([]byte(openCodeFixtureJSONL), []byte(openCodeFixtureExport))
	if err != nil {
		t.Fatal(err)
	}
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	assignment := ScreeningAssignment{BatchID: "opencode-fixture-batch", CaseID: plan.CaseID, CaseVersion: plan.CaseVersion, Attempt: 1, Configuration: configuration, CandidatePacketSHA256: suite.Cases[0].CandidatePacketSHA256, EvaluationBinding: "binding"}
	reservation := ScreeningReservation{SchemaVersion: ScreeningReservationSchemaVersion, BatchID: assignment.BatchID, EvaluationBinding: assignment.EvaluationBinding, CandidateID: configuration.ID, CaseID: assignment.CaseID, AdmissionInputSHA256: hash, ReservedAt: "2026-09-11T12:00:00Z", State: "reserved"}
	adapter := ExecutionAdapterIdentity{Provider: "opencode", Runtime: "opencode-cli", RuntimeVersion: "1.18.18", Endpoint: "local-cli-run-json", BillingBoundary: "opencode_zen_metered", AccountLane: "fixture_only", ToolPolicy: "none", ProviderEventPolicyVersion: "opencode-run-json-export.v1", ExecutorSHA256: hash, IsolationPolicySHA256: plan.InlineConfigSHA256, EnvironmentAllowlistSHA256: hash}
	receipt, err := BuildExecutionReceipt(reservation, assignment, OpenCodeExecutionPlanBinding(assignment.BatchID, plan), adapter, result)
	if err != nil {
		t.Fatal(err)
	}
	if reasons := ValidateExecutionReceipt(receipt); len(reasons) != 0 {
		t.Fatalf("normalized OpenCode receipt invalid: %v", reasons)
	}
}
