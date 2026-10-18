package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type protocolV32PreproviderFake struct {
	binding *agentqualification.ProtocolAblationV32Binding
	calls   *int
}

func (p protocolV32PreproviderFake) Call(_ context.Context, _ agentqualification.OpenCodeScreeningPlan) (agentqualification.ProtocolAblationV32ProviderResponse, error) {
	*p.calls++
	binding := *p.binding
	return agentqualification.ProtocolAblationV32ProviderResponse{
		CandidateJSON: json.RawMessage(`{}`), RawReceipt: json.RawMessage(`{"provider":"fake"}`),
		ProviderIdentity: agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: strings.TrimPrefix(binding.Candidate.Model, "openrouter/"), ReportedEffort: binding.Candidate.Effort, ReportedServingProvider: binding.Candidate.ServingProvider, ResolvedConfigSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Usage:            agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 1, OutputTokens: 1, CostMicros: 1},
	}, nil
}

func TestProtocolV32ExecutionUnlockRequiresExactFreezeBeforeProvider(t *testing.T) {
	called := false
	invalid := protocolV32SpendSnapshot{Provider: "openrouter", Source: "not_authoritative"}
	_, err := executeProtocolAblationV32(context.Background(), t.TempDir(), invalid, invalid, func(agentqualification.ProtocolAblationV32Binding, string) agentqualification.ProtocolAblationV32Provider {
		called = true
		return nil
	})
	if err == nil || called {
		t.Fatalf("execution escaped exact freeze/spend preflight: called=%v err=%v", called, err)
	}
	if err := agentqualification.RequireProtocolAblationV32ReviewedFreeze(); err != nil {
		t.Fatalf("exact atomic freeze did not unlock execution: %v", err)
	}
	if err := verifyProtocolV32CommandFreeze(); err != nil {
		t.Fatalf("exact command sources did not unlock execution: %v", err)
	}
}

func TestProtocolV32RawReceiptSealIsWriteOnce(t *testing.T) {
	root := t.TempDir()
	observation := agentqualification.ProtocolAblationV32Observation{BindingID: "binding", RawReceipts: []json.RawMessage{json.RawMessage(`{"raw":true}`)}}
	if err := sealProtocolV32RawObservation(root, observation); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "observations", "binding", "provider-01.json")
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != `{"raw":true}` {
		t.Fatalf("raw receipt was not sealed exactly: %s err=%v", raw, err)
	}
	if err := sealProtocolV32RawObservation(root, observation); err == nil {
		t.Fatal("write-once raw receipt was overwritten")
	}
}

func TestProtocolV32ClaimFileIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claims", "one.claim")
	if err := createProtocolV32ExclusiveClaim(path, "binding"); err != nil {
		t.Fatal(err)
	}
	if err := createProtocolV32ExclusiveClaim(path, "binding"); err == nil {
		t.Fatal("duplicate provider claim was not rejected")
	}
}

func TestProtocolV32FreshArtifactRootPreparesWorkspaceBeforeClaimAndReachesProvider(t *testing.T) {
	root := filepath.Join(t.TempDir(), "fresh-artifact-root")
	calls := 0
	authoritative := protocolV32SpendSnapshot{Provider: "openrouter", Source: "authoritative_api_usage", ObservedAt: "test", SpentMicros: 100}
	ledger, err := executeProtocolAblationV32(context.Background(), root, authoritative, authoritative, func(binding agentqualification.ProtocolAblationV32Binding, _ string) agentqualification.ProtocolAblationV32Provider {
		copy := binding
		return protocolV32PreproviderFake{binding: &copy, calls: &calls}
	})
	if err != nil || calls != 4 {
		t.Fatalf("fresh artifact execution did not reach each fake provider: calls=%d ledger=%+v err=%v", calls, ledger, err)
	}
	for bindingID, claim := range ledger.Claims {
		if claim.Status == "pending" {
			t.Fatalf("local workspace setup stranded pending claim %s: %+v", bindingID, ledger)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "ledger.json")); err != nil {
		t.Fatalf("durable ledger was not written: %v", err)
	}
}

func TestProtocolV32LocalWorkspaceSetupFailureCannotStrandPendingClaim(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "workspaces"), []byte("blocks directory creation"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	authoritative := protocolV32SpendSnapshot{Provider: "openrouter", Source: "authoritative_api_usage", ObservedAt: "test", SpentMicros: 100}
	ledger, err := executeProtocolAblationV32(context.Background(), root, authoritative, authoritative, func(binding agentqualification.ProtocolAblationV32Binding, _ string) agentqualification.ProtocolAblationV32Provider {
		calls++
		return protocolV32PreproviderFake{binding: &binding, calls: &calls}
	})
	if err == nil || calls != 0 || len(ledger.Claims) != 0 {
		t.Fatalf("local setup failure crossed paid-call claim boundary: calls=%d ledger=%+v err=%v", calls, ledger, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "ledger.json")); !os.IsNotExist(statErr) {
		t.Fatalf("local setup failure wrote a pending ledger: %v", statErr)
	}
}

func TestProtocolV32SourceManifestCoversEveryExecutableSource(t *testing.T) {
	hashes := protocolV32SourceHashes()
	for _, name := range []string{"protocol_ablation_v32.json", "protocol_ablation_v32.go", "protocol_ablation_v32_test.go", "cmd/protocol-ablation-v32/main.go", "cmd/protocol-ablation-v32/execute.go", "cmd/protocol-ablation-v32/main_test.go", "cmd/protocol-ablation-v32/execute_test.go"} {
		if len(hashes[name]) != 64 {
			t.Fatalf("v32 source is not hash-bound: %s=%q", name, hashes[name])
		}
	}
}

func TestProtocolV32IdentitySeparatesDirectProviderFromUpstreamModel(t *testing.T) {
	reportedProvider, upstream, ok := strings.Cut("openrouter/anthropic/claude-sonnet-5", "/")
	if !ok || reportedProvider != "openrouter" || upstream != "anthropic/claude-sonnet-5" {
		t.Fatalf("route identity was not separated: %q %q %v", reportedProvider, upstream, ok)
	}
}

func TestProtocolV32RecoveryProviderExtractionRequiresOneSelectedProvider(t *testing.T) {
	raw := []byte(`{"metadata":{"openrouter":{"provider":{"selected":true,"provider":"Anthropic"}}}}`)
	if got := protocolV32SelectedProvider(raw); got != "Anthropic" {
		t.Fatalf("serving provider evidence was lost: %q", got)
	}
}

func TestProtocolV32V3CarryForwardRejectsTamperWrongBindingAndDoubleImport(t *testing.T) {
	v2, expected := makeProtocolV32ImportFixture(t, protocolV32RecoveryBinding)
	v3 := filepath.Join(t.TempDir(), "v3")
	if _, err := initializeProtocolV32V3WithExpectation(v2, v3, expected); err != nil {
		t.Fatal(err)
	}
	if _, err := initializeProtocolV32V3WithExpectation(v2, v3, expected); err == nil {
		t.Fatal("double carry-forward import was accepted")
	}

	tamperedV2, tamperedExpected := makeProtocolV32ImportFixture(t, protocolV32RecoveryBinding)
	exportPath := filepath.Join(tamperedV2, "recovered-observations", safeProtocolV32Name(protocolV32RecoveryBinding), "session-export.json")
	if err := os.WriteFile(exportPath, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := initializeProtocolV32V3WithExpectation(tamperedV2, filepath.Join(t.TempDir(), "v3"), tamperedExpected); err == nil {
		t.Fatal("tampered recovered export was accepted")
	}

	wrongV2, wrongExpected := makeProtocolV32ImportFixture(t, "protocol-v32.gpt54.medium.one-shot-plan-v1")
	wrongExpected.BindingID = protocolV32RecoveryBinding
	if _, err := initializeProtocolV32V3WithExpectation(wrongV2, filepath.Join(t.TempDir(), "v3"), wrongExpected); err == nil {
		t.Fatal("wrong-binding carry-forward was accepted")
	}
}

func TestProtocolV32V3CarryForwardSkipsConsumedCellAndKeepsFrozenOrder(t *testing.T) {
	v2, expected := makeProtocolV32ImportFixture(t, protocolV32RecoveryBinding)
	v3 := filepath.Join(t.TempDir(), "v3")
	if _, err := initializeProtocolV32V3WithExpectation(v2, v3, expected); err != nil {
		t.Fatal(err)
	}
	authoritative := protocolV32SpendSnapshot{Provider: "openrouter", Source: "authoritative_api_usage", ObservedAt: "test", SpentMicros: 100}
	var launched []string
	_, err := executeProtocolAblationV32(context.Background(), v3, authoritative, authoritative, func(binding agentqualification.ProtocolAblationV32Binding, _ string) agentqualification.ProtocolAblationV32Provider {
		launched = append(launched, binding.BindingID)
		copy := binding
		calls := 0
		return protocolV32PreproviderFake{binding: &copy, calls: &calls}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"protocol-v32.claudesonnet5.medium.one-shot-plan-v1", "protocol-v32.gpt54.medium.one-shot-plan-v1", "protocol-v32.gpt54.medium.one-action-loop-v1"}
	if len(launched) != len(want) {
		t.Fatalf("unexpected launches: %+v", launched)
	}
	for index := range want {
		if launched[index] != want[index] {
			t.Fatalf("launch order=%+v want=%+v", launched, want)
		}
	}
}

func makeProtocolV32ImportFixture(t *testing.T, bindingID string) (string, protocolV32ImportExpectation) {
	t.Helper()
	v2 := t.TempDir()
	old := agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{bindingID: {BindingID: bindingID, Status: "pending"}}}
	ledgerRaw, _ := json.MarshalIndent(old, "", "  ")
	if err := os.WriteFile(filepath.Join(v2, "ledger.json"), ledgerRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	exportRaw := []byte(`{"info":{"id":"` + protocolV32RecoverySession + `","model":{"id":"anthropic/claude-sonnet-5","providerID":"openrouter","variant":"medium"},"cost":0.017724,"tokens":{"input":2,"output":383,"reasoning":218,"cache":{"read":0,"write":4684}}}}`)
	usage := agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 4686, OutputTokens: 383, ReasoningTokens: 218, CostMicros: 17724}
	observation := agentqualification.ProtocolAblationV32Observation{BindingID: bindingID, Outcome: "invalid", FailureClass: "test_invalidity", FailureScope: "observation", Usage: usage, ProviderIdentity: agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: "anthropic/claude-sonnet-5", ReportedEffort: "medium"}, RawReceipts: []json.RawMessage{exportRaw}, Actions: []agentqualification.RealisticCandidateAction{}, GraderResult: agentqualification.RealisticNextCalibrationResult{CaseID: "P32"}}
	observation.EvidenceSHA256 = agentqualification.ProtocolAblationV32ObservationEvidenceSHA(observation)
	record := protocolV32RecoveryRecord{SessionID: protocolV32RecoverySession, ProviderCalls: 0, Classification: "test_invalidity", Scope: "observation", OriginalLedgerSHA: protocolV32SHA(ledgerRaw), Observation: observation}
	recordRaw, _ := json.MarshalIndent(record, "", "  ")
	dir := filepath.Join(v2, "recovered-observations", safeProtocolV32Name(bindingID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session-export.json"), exportRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "observation.json"), recordRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	return v2, protocolV32ImportExpectation{LedgerSHA256: protocolV32SHA(ledgerRaw), ExportSHA256: protocolV32SHA(exportRaw), RecordSHA256: protocolV32SHA(recordRaw), SessionID: protocolV32RecoverySession, BindingID: bindingID, Usage: usage}
}

func TestProtocolV32V4CarryForwardRejectsTamperDoubleImportAndProviderContradiction(t *testing.T) {
	v3, expected := makeProtocolV32V4Fixture(t, "")
	v4 := filepath.Join(t.TempDir(), "v4")
	if _, err := recoverProtocolV32V3AndInitializeV4WithExpectation(v3, v4, expected); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverProtocolV32V3AndInitializeV4WithExpectation(v3, v4, expected); err == nil {
		t.Fatal("double v4 import was accepted")
	}

	tampered, tamperedExpected := makeProtocolV32V4Fixture(t, "")
	path := filepath.Join(tampered, "presealed", safeProtocolV32Name(protocolV32ClaudePlanBinding), "provider-01.json")
	if err := os.WriteFile(path, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverProtocolV32V3AndInitializeV4WithExpectation(tampered, filepath.Join(t.TempDir(), "v4"), tamperedExpected); err == nil {
		t.Fatal("tampered v3 receipt was accepted")
	}

	contradictory, contradictoryExpected := makeProtocolV32V4Fixture(t, "OpenAI")
	if _, err := recoverProtocolV32V3AndInitializeV4WithExpectation(contradictory, filepath.Join(t.TempDir(), "v4"), contradictoryExpected); err == nil {
		t.Fatal("contradictory serving-provider evidence was accepted")
	}
}

func TestProtocolV32V4CarryForwardLaunchesOnlyGPTInFrozenOrder(t *testing.T) {
	v3, expected := makeProtocolV32V4Fixture(t, "")
	v4 := filepath.Join(t.TempDir(), "v4")
	if _, err := recoverProtocolV32V3AndInitializeV4WithExpectation(v3, v4, expected); err != nil {
		t.Fatal(err)
	}
	authoritative := protocolV32SpendSnapshot{Provider: "openrouter", Source: "authoritative_api_usage", ObservedAt: "test", SpentMicros: 100}
	var launched []string
	_, err := executeProtocolAblationV32(context.Background(), v4, authoritative, authoritative, func(binding agentqualification.ProtocolAblationV32Binding, _ string) agentqualification.ProtocolAblationV32Provider {
		launched = append(launched, binding.BindingID)
		copy, calls := binding, 0
		return protocolV32PreproviderFake{binding: &copy, calls: &calls}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"protocol-v32.gpt54.medium.one-shot-plan-v1", "protocol-v32.gpt54.medium.one-action-loop-v1"}
	if len(launched) != 2 || launched[0] != want[0] || launched[1] != want[1] {
		t.Fatalf("launch order=%+v want=%+v", launched, want)
	}
}

func makeProtocolV32V4Fixture(t *testing.T, selectedProvider string) (string, protocolV32V4ImportExpectation) {
	t.Helper()
	v3 := t.TempDir()
	priorUsage := protocolV32V3ExactImport.Usage
	priorObservation := agentqualification.ProtocolAblationV32Observation{BindingID: protocolV32RecoveryBinding, Outcome: "invalid", FailureClass: "test_invalidity", FailureScope: "observation", Usage: priorUsage}
	old := agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{
		protocolV32RecoveryBinding:   {BindingID: protocolV32RecoveryBinding, Status: "completed", Completed: &priorObservation},
		protocolV32ClaudePlanBinding: {BindingID: protocolV32ClaudePlanBinding, Status: "pending"},
	}, Usage: priorUsage}
	ledgerRaw, _ := json.MarshalIndent(old, "", "  ")
	if err := os.WriteFile(filepath.Join(v3, "ledger.json"), ledgerRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	claimRaw := []byte(protocolV32ClaudePlanBinding + "\n")
	claimPath := filepath.Join(v3, "claims", safeProtocolV32Name(protocolV32ClaudePlanBinding)+".claim")
	if err := os.MkdirAll(filepath.Dir(claimPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claimPath, claimRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata := ""
	if selectedProvider != "" {
		metadata = `,"metadata":{"openrouter":{"route":{"selected":true,"provider":"` + selectedProvider + `"}}}`
	}
	jsonl := `{"type":"step_start","timestamp":1789128000000,"sessionID":"` + protocolV32ClaudePlanSession + `","part":{"id":"p_start","sessionID":"` + protocolV32ClaudePlanSession + `","messageID":"msg_fixture","type":"step-start"}}` + "\n" +
		`{"type":"text","timestamp":1789128000900,"sessionID":"` + protocolV32ClaudePlanSession + `","part":{"id":"p_text","sessionID":"` + protocolV32ClaudePlanSession + `","messageID":"msg_fixture","type":"text","text":"markdown, not json"}}` + "\n" +
		`{"type":"step_finish","timestamp":1789128001000,"sessionID":"` + protocolV32ClaudePlanSession + `","part":{"id":"p_finish","sessionID":"` + protocolV32ClaudePlanSession + `","messageID":"msg_fixture","type":"step-finish","reason":"stop","cost":0.0280465,"tokens":{"input":2,"output":1362,"reasoning":269,"cache":{"read":0,"write":4693}}}}` + "\n"
	exported := []byte(`{"info":{"id":"` + protocolV32ClaudePlanSession + `"},"messages":[{"info":{"id":"msg_fixture","sessionID":"` + protocolV32ClaudePlanSession + `","role":"assistant","modelID":"anthropic/claude-sonnet-5","providerID":"openrouter","summary":false,"finish":"stop","cost":0.0280465,"tokens":{"input":2,"output":1362,"reasoning":269,"cache":{"read":0,"write":4693}}},"parts":[{"id":"p_text","sessionID":"` + protocolV32ClaudePlanSession + `","messageID":"msg_fixture","type":"text","text":"markdown, not json"` + metadata + `}]}]}`)
	receiptRaw, _ := json.Marshal(map[string]any{"jsonl": jsonl, "export": json.RawMessage(exported), "resolved_config_sha256": protocolV32V4ExactImport.ResolvedConfigSHA256, "prompt_sha256": protocolV32V4ExactImport.PromptSHA256, "schema_sha256": protocolV32V4ExactImport.SchemaSHA256})
	presealedPath := filepath.Join(v3, "presealed", safeProtocolV32Name(protocolV32ClaudePlanBinding), "provider-01.json")
	if err := os.MkdirAll(filepath.Dir(presealedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(presealedPath, receiptRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	return v3, protocolV32V4ImportExpectation{LedgerSHA256: protocolV32SHA(ledgerRaw), ClaimSHA256: protocolV32SHA(claimRaw), PresealedSHA256: protocolV32SHA(receiptRaw), SessionID: protocolV32ClaudePlanSession, BindingID: protocolV32ClaudePlanBinding, ResolvedConfigSHA256: protocolV32V4ExactImport.ResolvedConfigSHA256, PromptSHA256: protocolV32V4ExactImport.PromptSHA256, SchemaSHA256: protocolV32V4ExactImport.SchemaSHA256, Usage: protocolV32V4ExactImport.Usage}
}
