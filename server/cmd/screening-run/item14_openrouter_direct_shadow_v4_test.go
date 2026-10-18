package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestItem14OpenRouterDirectV4ProviderFreeGates(t *testing.T) {
	plan, err := item14OpenRouterDirectPlanV4(offlineContracts(t))
	if err != nil || plan["provider_called"] != false || plan["credential_read"] != false {
		t.Fatalf("v4 plan failed: %+v %v", plan, err)
	}
	preflight, err := item14OpenRouterDirectPrepareV4(offlineContracts(t))
	if err != nil || !preflight.Ready || preflight.ProviderCallsMade != 0 || preflight.CredentialReads != 0 || !preflight.TopLevelProviderRequired || preflight.ProviderAttemptEvidencePolicy != "absent_or_one_exact" {
		t.Fatalf("v4 preflight failed: %+v %v", preflight, err)
	}
	run, err := item14OpenRouterDirectFakeV4()
	if err != nil || run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 3 || run.CurrentPassCount != 3 || run.AntecedentFailureCount != 1 || run.CandidateQualified || run.ToolCallCount != 0 {
		t.Fatalf("v4 fake failed: %+v %v", run, err)
	}
}

func TestItem14OpenRouterDirectV4CandidateFailureContinuesAndReplayFails(t *testing.T) {
	parent := filepath.Join(t.TempDir(), ".qualification-evidence")
	_ = os.MkdirAll(parent, 0o700)
	deps := newFakeItem14OpenRouterDirectDependenciesV4()
	base := deps.InvokeOnce
	calls := 0
	deps.InvokeOnce = func(ctx context.Context, key []byte, endpoint string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
		calls++
		raw, count, err := base(ctx, key, endpoint, plan)
		if calls == 1 {
			var value map[string]any
			_ = json.Unmarshal(raw, &value)
			value["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"] = `{"workflow_id":"altered"}`
			raw, _ = json.Marshal(value)
		}
		return raw, count, err
	}
	root := filepath.Join(parent, "item14-openrouter-direct-shadow-v4")
	run, err := executeItem14OpenRouterDirectV4(context.Background(), root, deps)
	if err != nil || calls != 3 || run.CurrentFailureCount != 1 || run.CurrentPassCount != 2 {
		t.Fatalf("v4 continuation failed: %+v %v", run, err)
	}
	if _, err := executeItem14OpenRouterDirectV4(context.Background(), filepath.Join(parent, "alternate"), newFakeItem14OpenRouterDirectDependenciesV4()); err == nil {
		t.Fatal("v4 alternate-root replay passed")
	}
}

func TestItem14OpenRouterDirectV4ProviderDriftHaltsOnce(t *testing.T) {
	parent := filepath.Join(t.TempDir(), ".qualification-evidence")
	_ = os.MkdirAll(parent, 0o700)
	deps := newFakeItem14OpenRouterDirectDependenciesV4()
	base := deps.InvokeOnce
	calls := 0
	deps.InvokeOnce = func(ctx context.Context, key []byte, endpoint string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
		calls++
		raw, count, err := base(ctx, key, endpoint, plan)
		var value map[string]any
		_ = json.Unmarshal(raw, &value)
		value["provider"] = "Other"
		raw, _ = json.Marshal(value)
		return raw, count, err
	}
	root := filepath.Join(parent, "item14-openrouter-direct-shadow-v4")
	if _, err := executeItem14OpenRouterDirectV4(context.Background(), root, deps); err == nil || calls != 1 {
		t.Fatalf("v4 provider drift calls=%d err=%v", calls, err)
	}
	var terminal item14OpenRouterDirectTerminalV4
	terminalRaw, err := os.ReadFile(filepath.Join(root, "terminal-failure-receipt.json"))
	if err != nil || json.Unmarshal(terminalRaw, &terminal) != nil || terminal.FailureClass != "identity_provider_or_evidence_failure" || terminal.ProviderRequestCount != 1 || terminal.CompletedObservationCount != 0 || terminal.RemainingWorkflowCount != 2 || !terminal.ResponseAccounting.UsageObserved || terminal.ResponseAccounting.ProviderReportedCostMicros != 10 || !terminal.CredentialMemoryZeroed || !terminal.AuthorityExhausted || terminal.RetryAllowed {
		t.Fatalf("v4 terminal settlement missing: %+v %v", terminal, err)
	}
	var accounting item14OpenRouterDirectAccountingV2
	accountingRaw, err := os.ReadFile(filepath.Join(root, "accounting.json"))
	if err != nil || json.Unmarshal(accountingRaw, &accounting) != nil || accounting.ReceiptCostMicros != 10 || !accounting.CredentialMemoryZeroed {
		t.Fatalf("v4 terminal accounting missing: %+v %v", accounting, err)
	}
	if _, err := executeItem14OpenRouterDirectV4(context.Background(), filepath.Join(parent, "alternate"), newFakeItem14OpenRouterDirectDependenciesV4()); err == nil {
		t.Fatal("v4 terminal authority replay passed")
	}
}
