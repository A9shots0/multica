package agentqualification

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openRouterExecutionFixture(t *testing.T, batch ScreeningBatchDefinition) (OpenCodeScreeningPlan, []byte, []byte) {
	t.Helper()
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodePlanForTransport(batch.CandidateTransport, assignments[0].Configuration, batch.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	modelID := strings.TrimPrefix(plan.Model, "openrouter/")
	jsonl := []byte(openCodeFixtureJSONL)
	exported := strings.Replace(openCodeFixtureExport, `"modelID":"test-model","providerID":"opencode"`, `"modelID":"`+modelID+`","providerID":"openrouter"`, 1)
	exported = strings.Replace(exported, `"provider":"Fixture Provider"`, `"provider":"`+plan.ServingProvider+`"`, 1)
	return plan, jsonl, []byte(exported)
}

func openRouterExecutionInputs(t *testing.T) (ScreeningBatchDefinition, BatchUsageLedger, APISpendSnapshot, ExecutionAdapterIdentity, OpenCodeScreeningPlan, []byte, []byte) {
	t.Helper()
	batch, err := TwelfthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	plan, jsonl, exported := openRouterExecutionFixture(t, batch)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	spend := APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 102071724, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_api_usage"}
	hash := strings.Repeat("a", 64)
	adapter := ExecutionAdapterIdentity{
		Provider: "openrouter", Runtime: "opencode-cli", RuntimeVersion: "1.18.18", Endpoint: "local-cli-run-json",
		BillingBoundary: "openrouter_metered", AccountLane: "opencode_local_openrouter", ToolPolicy: "none",
		ProviderEventPolicyVersion: "opencode-run-json-export-routing.v2", ExecutorSHA256: hash,
		IsolationPolicySHA256: plan.InlineConfigSHA256, EnvironmentAllowlistSHA256: hash,
	}
	return batch, ledger, spend, adapter, plan, jsonl, exported
}

func TestExecuteOpenCodeReservedRunInvokesOnceAndSealsEvidence(t *testing.T) {
	batch, ledger, spend, adapter, plan, jsonl, exported := openRouterExecutionInputs(t)
	grant := ProviderCallGrantInput{BatchID: batch.BatchID, GrantedBy: "owner", Scope: OneAdmittedOpenRouterObservationScope, MaxProviderCalls: 1, GrantedAt: "2026-09-11T12:00:09Z"}
	invocations := 0
	completed, err := ExecuteOpenCodeReservedRun(t.TempDir(), batch, ledger, spend, "2026-09-11T12:00:10Z", "2026-09-11T12:00:11Z", resolvedOpenCodeConfigFor(plan), adapter, grant, func(got OpenCodeScreeningPlan) ([]byte, []byte, error) {
		invocations++
		if got.CandidateID != plan.CandidateID || got.PromptSHA256 != plan.PromptSHA256 {
			t.Fatalf("runner received drifted plan: %+v", got)
		}
		return jsonl, exported, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if invocations != 1 || completed.Receipt.RequestedModel != plan.Model || completed.Receipt.ReportedModel != plan.Model || len(completed.CandidateResponse) == 0 {
		t.Fatalf("execution was not bound exactly once: invocations=%d completed=%+v", invocations, completed)
	}
	if completed.Grant.Scope != OneAdmittedOpenRouterObservationScope || completed.Grant.EvaluationBinding != completed.Reservation.EvaluationBinding {
		t.Fatalf("provider grant was not bound to the admitted case-agnostic observation: %+v", completed.Grant)
	}
	for _, name := range []string{"reservation.json", "launch-capsule.json", "provider-call-grant.json", "provider-run.jsonl", "provider-session-export.json", "execution-receipt.json"} {
		if _, err := os.Stat(filepath.Join(completed.ReservationDir, name)); err != nil {
			t.Fatalf("missing sealed artifact %s: %v", name, err)
		}
	}
	if _, _, err := GradeSealedOpenCodeEvidenceForBatch(completed.ReservationDir, batch); err != nil {
		t.Fatal(err)
	}
	record, _, err := FinalizeSealedOpenCodeObservation(completed.ReservationDir, batch)
	if err != nil {
		t.Fatal(err)
	}
	if record.HarnessVersion != OpenCodeToolFreeTransportV6 || record.CandidateVersion != "v12" || record.CaseVersion != "v4" || record.GraderVersion != "deterministic-t11.v4" || record.RunReceipts[0].Usage.FreshInputTokens == nil || record.RunReceipts[0].Usage.OutputTokens == nil || *record.RunReceipts[0].Usage.OutputTokens != 30 || len(record.BlindGradeFailedChecks) == 0 {
		t.Fatalf("final observation lost transport/version/usage binding: %+v", record)
	}
}

func TestFinalizeRejectsGradeFromWrongCaseVersion(t *testing.T) {
	batch, ledger, spend, adapter, plan, jsonl, exported := openRouterExecutionInputs(t)
	grant := ProviderCallGrantInput{BatchID: batch.BatchID, GrantedBy: "owner", Scope: OneAdmittedOpenRouterObservationScope, MaxProviderCalls: 1, GrantedAt: "2026-09-11T12:00:09Z"}
	completed, err := ExecuteOpenCodeReservedRun(t.TempDir(), batch, ledger, spend, "2026-09-11T12:00:10Z", "2026-09-11T12:00:11Z", resolvedOpenCodeConfigFor(plan), adapter, grant, func(OpenCodeScreeningPlan) ([]byte, []byte, error) {
		return jsonl, exported, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := GradeSealedOpenCodeEvidenceVersion(completed.ReservationDir, "T12", "v4"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := FinalizeSealedOpenCodeObservation(completed.ReservationDir, batch); err == nil || !strings.Contains(err.Error(), "blind grade identity") {
		t.Fatalf("wrong-case grade was accepted: %v", err)
	}
}

func TestExecuteOpenCodeReservedRunDoesNotRetryAmbiguousFailure(t *testing.T) {
	batch, ledger, spend, adapter, plan, jsonl, _ := openRouterExecutionInputs(t)
	grant := ProviderCallGrantInput{BatchID: batch.BatchID, GrantedBy: "owner", Scope: OneAdmittedOpenRouterObservationScope, MaxProviderCalls: 1, GrantedAt: "2026-09-11T12:00:09Z"}
	root := t.TempDir()
	invocations := 0
	runner := func(OpenCodeScreeningPlan) ([]byte, []byte, error) {
		invocations++
		return jsonl, []byte(`{"partial":true}`), errors.New("transport interrupted")
	}
	failed, err := ExecuteOpenCodeReservedRun(root, batch, ledger, spend, "2026-09-11T12:00:10Z", "2026-09-11T12:00:11Z", resolvedOpenCodeConfigFor(plan), adapter, grant, runner)
	if err == nil || invocations != 1 {
		t.Fatalf("ambiguous execution did not stop after one invocation: calls=%d err=%v", invocations, err)
	}
	if _, err := os.Stat(filepath.Join(failed.ReservationDir, "execution-failure.json")); err != nil {
		t.Fatalf("ambiguous failure evidence missing: %v", err)
	}
	if _, err := ExecuteOpenCodeReservedRun(root, batch, ledger, spend, "2026-09-11T12:00:12Z", "2026-09-11T12:00:13Z", resolvedOpenCodeConfigFor(plan), adapter, grant, runner); !errors.Is(err, ErrScreeningAssignmentReserved) {
		t.Fatalf("ambiguous reservation was replayed: %v", err)
	}
	if invocations != 1 {
		t.Fatalf("runner retried after ambiguous failure: %d", invocations)
	}
}

func TestFinalizeResponseContractFailureWithoutRetry(t *testing.T) {
	batch, ledger, spend, adapter, plan, jsonl, exported := openRouterExecutionInputs(t)
	grant := ProviderCallGrantInput{BatchID: batch.BatchID, GrantedBy: "owner", Scope: OneAdmittedOpenRouterObservationScope, MaxProviderCalls: 1, GrantedAt: "2026-09-11T12:00:09Z"}
	invalidOutput := "```json\n{\"owner_observation_kind\":\"recurring_console_popup\"}\n```"
	encodedInvalid, err := json.Marshal(invalidOutput)
	if err != nil {
		t.Fatal(err)
	}
	jsonl = []byte(strings.Replace(string(jsonl), `"text":"{\"owner_outcome\":\"restore access\"}"`, `"text":`+string(encodedInvalid), 1))
	exported = []byte(strings.Replace(string(exported), `"text":"{\"owner_outcome\":\"restore access\"}"`, `"text":`+string(encodedInvalid), 1))
	invocations := 0
	failed, err := ExecuteOpenCodeReservedRun(t.TempDir(), batch, ledger, spend, "2026-09-11T12:00:10Z", "2026-09-11T12:00:11Z", resolvedOpenCodeConfigFor(plan), adapter, grant, func(OpenCodeScreeningPlan) ([]byte, []byte, error) {
		invocations++
		return jsonl, exported, nil
	})
	if err == nil || invocations != 1 || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("response contract violation was not sealed exactly once: calls=%d err=%v", invocations, err)
	}
	record, path, err := FinalizeSealedOpenCodeResponseContractFailure(failed.ReservationDir, batch, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if record.TrialStatus != "failed" || len(record.Disqualifiers) != 1 || record.Disqualifiers[0] != "invalid_response_format" || path == "" {
		t.Fatalf("response contract failure was not represented accurately: %+v", record)
	}
	if invocations != 1 {
		t.Fatalf("offline closure retried the provider: %d", invocations)
	}
}

func TestExecuteOpenCodeReservedRunRejectsAdapterBeforeReservation(t *testing.T) {
	batch, ledger, spend, adapter, plan, _, _ := openRouterExecutionInputs(t)
	grant := ProviderCallGrantInput{BatchID: batch.BatchID, GrantedBy: "owner", Scope: OneAdmittedOpenRouterObservationScope, MaxProviderCalls: 1, GrantedAt: "2026-09-11T12:00:09Z"}
	adapter.AccountLane = "wrong-lane"
	root := t.TempDir()
	invocations := 0
	if _, err := ExecuteOpenCodeReservedRun(root, batch, ledger, spend, "2026-09-11T12:00:10Z", "2026-09-11T12:00:11Z", resolvedOpenCodeConfigFor(plan), adapter, grant, func(OpenCodeScreeningPlan) ([]byte, []byte, error) {
		invocations++
		return nil, nil, nil
	}); err == nil {
		t.Fatal("mismatched adapter was admitted")
	}
	if invocations != 0 {
		t.Fatalf("mismatched adapter reached provider: %d", invocations)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("mismatched adapter created reservation artifacts: %v", entries)
	}
}

func TestExecuteOpenCodeReservedRunRejectsBroadGrantBeforeReservation(t *testing.T) {
	batch, ledger, spend, adapter, plan, _, _ := openRouterExecutionInputs(t)
	grant := ProviderCallGrantInput{BatchID: batch.BatchID, GrantedBy: "owner", Scope: "entire_batch", MaxProviderCalls: 2, GrantedAt: "2026-09-11T12:00:09Z"}
	root := t.TempDir()
	invocations := 0
	if _, err := ExecuteOpenCodeReservedRun(root, batch, ledger, spend, "2026-09-11T12:00:10Z", "2026-09-11T12:00:11Z", resolvedOpenCodeConfigFor(plan), adapter, grant, func(OpenCodeScreeningPlan) ([]byte, []byte, error) {
		invocations++
		return nil, nil, nil
	}); err == nil {
		t.Fatal("broad provider-call grant was admitted")
	}
	if invocations != 0 {
		t.Fatalf("broad grant reached provider: %d", invocations)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("broad grant created reservation artifacts: %v", entries)
	}
}
