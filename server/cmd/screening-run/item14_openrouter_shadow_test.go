package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestItem14OpenRouterShadowPlanAndFakeAreProviderFree(t *testing.T) {
	plan, err := item14OpenRouterShadowPlan(offlineContracts(t))
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["credential_read"] != false || plan["status"] != "frozen_not_launched" {
		t.Fatalf("unsafe plan result: %+v", plan)
	}
	run, err := item14OpenRouterShadowFake()
	if err != nil {
		t.Fatal(err)
	}
	if run.ExecutionMode != "deterministic_fake_provider_free" || run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 4 || run.RetryCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.RoleModelSelectionCount != 0 {
		t.Fatalf("unsafe fake result: %+v", run)
	}
}

func TestItem14OpenRouterShadowEvidenceWritesAreExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := writeExclusiveJSON(path, map[string]any{"state": "claimed"}); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveJSON(path, map[string]any{"state": "replayed"}); err == nil {
		t.Fatal("duplicate evidence write passed")
	}
}

func TestItem14OpenRouterShadowAuthorityClaimBlocksAlternateRootReplay(t *testing.T) {
	parent := filepath.Join(t.TempDir(), ".qualification-evidence")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	firstRoot := filepath.Join(parent, "item14-openrouter-local-shadow-v1")
	if _, err := executeItem14OpenRouterShadowWithDependencies(context.Background(), firstRoot, newFakeItem14OpenRouterDependencies()); err != nil {
		t.Fatal(err)
	}
	secondRoot := filepath.Join(parent, "alternate-evidence-root")
	if _, err := executeItem14OpenRouterShadowWithDependencies(context.Background(), secondRoot, newFakeItem14OpenRouterDependencies()); err == nil {
		t.Fatal("alternate-root replay passed")
	}
	if _, err := os.Stat(secondRoot); !os.IsNotExist(err) {
		t.Fatalf("replay created second evidence root: %v", err)
	}
}

func TestItem14OpenRouterShadowAmbiguousClaimAndPostCallFailureAreNeverReplayable(t *testing.T) {
	for _, failure := range []string{"invoke", "post-spend"} {
		t.Run(failure, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), ".qualification-evidence")
			if err := os.MkdirAll(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(parent, "item14-openrouter-local-shadow-v1")
			deps := newFakeItem14OpenRouterDependencies()
			originalInvoke := deps.InvokeOnce
			calls := 0
			deps.InvokeOnce = func(ctx context.Context, plan agentqualification.OpenCodeScreeningPlan) ([]byte, []byte, bool, error) {
				calls++
				if failure == "invoke" {
					return nil, nil, true, errors.New("injected ambiguous invoke failure")
				}
				return originalInvoke(ctx, plan)
			}
			if failure == "post-spend" {
				originalSpend := deps.SpendSnapshot
				deps.SpendSnapshot = func(ctx context.Context, at time.Time) (agentqualification.APISpendSnapshot, error) {
					if calls > 0 {
						return agentqualification.APISpendSnapshot{}, errors.New("injected post-exposure accounting failure")
					}
					return originalSpend(ctx, at)
				}
			}
			if _, err := executeItem14OpenRouterShadowWithDependencies(context.Background(), root, deps); err == nil {
				t.Fatal("injected failure passed")
			}
			if calls != 1 {
				t.Fatalf("provider seam called %d times", calls)
			}
			alternate := filepath.Join(parent, "alternate")
			if _, err := executeItem14OpenRouterShadowWithDependencies(context.Background(), alternate, newFakeItem14OpenRouterDependencies()); err == nil {
				t.Fatal("ambiguous claim replayed through alternate root")
			}
		})
	}
}

func TestItem14OpenRouterShadowReconcilesSealedProviderUnobservedFailureWithoutReplay(t *testing.T) {
	root := writeFakeItem14OpenRouterProviderUnobservedEvidence(t)
	receipt, err := reconcileItem14OpenRouterShadowFailure(root, offlineContracts(t))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "failed_closed_actual_serving_provider_unobserved" || !receipt.AuthorityExhausted || receipt.RetryAllowed || receipt.ActualProviderCalls != 1 || receipt.RemainingWorkflowsNotCalled != 3 || receipt.ActualModel != "openrouter/qwen/qwen3-coder-next" || receipt.ActualServingProvider != "" || receipt.RequestedServingProvider != "alibaba" || receipt.ProviderFallbacks || receipt.SessionInspection.AssistantMessages != 1 || receipt.SessionInspection.SummaryAssistants != 0 || receipt.SessionInspection.ToolParts != 0 || len(receipt.SessionInspection.ServingProviders) != 0 || !receipt.ResponseContractValid || !receipt.CleanupVerified || receipt.ProviderReportedCostMicros != 10 {
		t.Fatalf("unsafe terminal reconciliation: %+v", receipt)
	}
	if _, err := os.Stat(filepath.Join(root, "accounting.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileItem14OpenRouterShadowFailure(root, offlineContracts(t)); err == nil {
		t.Fatal("terminal reconciliation replay passed")
	}
}

func TestItem14OpenRouterShadowReconciliationRejectsHiddenAssistant(t *testing.T) {
	root := writeFakeItem14OpenRouterProviderUnobservedEvidence(t)
	exportPath := filepath.Join(root, "01-local-intake-decision-coordination", "provider-session-export.json")
	raw, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	messages := value["messages"].([]any)
	value["messages"] = append(messages, map[string]any{
		"info":  map[string]any{"id": "hidden", "sessionID": "ses_item14_fake_0", "role": "assistant", "providerID": "openrouter", "modelID": "qwen/qwen3-coder-next", "summary": true, "finish": "stop", "cost": 0, "tokens": map[string]any{"input": 0, "output": 0, "reasoning": 0, "cache": map[string]any{"read": 0, "write": 0}}},
		"parts": []any{},
	})
	raw, _ = json.Marshal(value)
	if err := os.WriteFile(exportPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileItem14OpenRouterShadowFailure(root, offlineContracts(t)); err == nil {
		t.Fatal("hidden assistant passed terminal reconciliation")
	}
	if _, err := os.Stat(filepath.Join(root, "terminal-failure-receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid reconciliation wrote a receipt: %v", err)
	}
}

func writeFakeItem14OpenRouterProviderUnobservedEvidence(t *testing.T) string {
	t.Helper()
	definition, _, plans, err := loadItem14OpenRouterShadowPlans(offlineContracts(t))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), ".qualification-evidence", "item14-openrouter-local-shadow-v1")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	resolved := map[string]string{}
	for _, plan := range plans {
		resolved[plan.CaseID] = plan.InlineConfigSHA256
	}
	preflight := item14OpenRouterShadowPreflight{SchemaVersion: "silver-mediums.openrouter-local-shadow-preflight.v1", DefinitionSHA256: agentqualification.OpenRouterLocalShadowDefinitionV1SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, ProviderFallbacks: false, RetryCount: 0, ProviderCallsMade: 0, CredentialReads: 0, CredentialIsolation: "fresh_empty_task_local_profile", ResolvedConfigSHA256: resolved, TemporaryRootsRemoved: len(plans), TemporaryRootsAbsent: true, Ready: true}
	claim := item14OpenRouterShadowClaim{SchemaVersion: "silver-mediums.openrouter-local-shadow-claim.v1", RunID: definition.RunID, DefinitionSHA256: agentqualification.OpenRouterLocalShadowDefinitionV1SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, MaxProviderCalls: definition.MaxProviderCalls, RetryCount: 0, ClaimedAt: "2026-09-14T14:58:32Z", ReplayPolicy: "any_claim_without_receipt_is_ambiguous_and_never_replayed"}
	if err := writeExclusiveJSON(filepath.Join(filepath.Dir(root), "item14-openrouter-authority-v1.claim.json"), claim); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveJSON(filepath.Join(root, "provider-free-preflight.json"), preflight); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveJSON(filepath.Join(root, "run-claim.json"), claim); err != nil {
		t.Fatal(err)
	}
	spend := agentqualification.APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 111000000, ObservedAt: "2026-09-14T14:58:32Z", Source: "authoritative_api_usage"}
	if err := writeExclusiveJSON(filepath.Join(root, "spend-before.json"), spend); err != nil {
		t.Fatal(err)
	}
	plan := plans[0]
	workflowDir := filepath.Join(root, "01-"+plan.CaseID)
	if err := os.Mkdir(workflowDir, 0o700); err != nil {
		t.Fatal(err)
	}
	workflowClaim := item14OpenRouterWorkflowClaim{SchemaVersion: "silver-mediums.openrouter-local-shadow-workflow-claim.v1", RunID: definition.RunID, WorkflowID: plan.CaseID, Sequence: 1, PromptSHA256: plan.PromptSHA256, InlineConfigSHA256: plan.InlineConfigSHA256, Model: plan.Model, ServingProvider: plan.ServingProvider, MaxProviderCalls: 1, RetryCount: 0, ClaimedAt: "2026-09-14T14:58:33Z"}
	if err := writeExclusiveJSON(filepath.Join(workflowDir, "claim.json"), workflowClaim); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveJSON(filepath.Join(workflowDir, "spend-before.json"), spend); err != nil {
		t.Fatal(err)
	}
	spend.ObservedAt = "2026-09-14T14:58:41Z"
	if err := writeExclusiveJSON(filepath.Join(workflowDir, "spend-after.json"), spend); err != nil {
		t.Fatal(err)
	}
	rawJSONL, rawExport, _, err := fakeItem14OpenRouterRawArtifacts(plan, 0)
	if err != nil {
		t.Fatal(err)
	}
	var exported map[string]any
	if err := json.Unmarshal(rawExport, &exported); err != nil {
		t.Fatal(err)
	}
	messages := exported["messages"].([]any)
	parts := messages[0].(map[string]any)["parts"].([]any)
	delete(parts[0].(map[string]any), "metadata")
	rawExport, _ = json.Marshal(exported)
	if err := writeExclusiveBytes(filepath.Join(workflowDir, "provider-run.jsonl"), rawJSONL); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveBytes(filepath.Join(workflowDir, "provider-session-export.json"), rawExport); err != nil {
		t.Fatal(err)
	}
	halt := item14OpenRouterShadowHalt{Class: "identity_or_response_contract_failure", CleanupVerified: true, Error: "OpenRouter local shadow actual model, provider, or one-turn evidence drifted", ProviderCalls: 1, RetryAllowed: false}
	if err := writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), halt); err != nil {
		t.Fatal(err)
	}
	return root
}
