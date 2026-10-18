package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestItem14OpenRouterDirectV3PlanFakePrepareProviderFree(t *testing.T) {
	plan, err := item14OpenRouterDirectPlanV3(offlineContracts(t))
	if err != nil || plan["provider_called"] != false || plan["credential_read"] != false {
		t.Fatalf("v3 plan failed: %+v %v", plan, err)
	}
	preflight, err := item14OpenRouterDirectPrepareV3(offlineContracts(t))
	if err != nil || !preflight.Ready || preflight.CredentialReads != 0 || preflight.ProviderCallsMade != 0 || preflight.ProviderSchemaParam || preflight.ExpectedEndpointModel != "qwen/qwen3-coder-next-2025-02-03" {
		t.Fatalf("v3 preflight failed: %+v %v", preflight, err)
	}
	run, err := item14OpenRouterDirectFakeV3()
	if err != nil || run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 4 || run.CandidatePassCount != 4 || run.RetryCount != 0 || run.ToolCallCount != 0 {
		t.Fatalf("v3 fake failed: %+v %v", run, err)
	}
}

func TestItem14OpenRouterDirectV3CandidateFailureContinuesAndProviderDriftHalts(t *testing.T) {
	t.Run("candidate local", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), ".qualification-evidence")
		_ = os.MkdirAll(parent, 0o700)
		deps := newFakeItem14OpenRouterDirectDependenciesV3()
		base := deps.InvokeOnce
		calls := 0
		deps.InvokeOnce = func(ctx context.Context, credential []byte, endpoint string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
			calls++
			raw, count, err := base(ctx, credential, endpoint, plan)
			if calls == 1 {
				var value map[string]any
				_ = json.Unmarshal(raw, &value)
				value["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"] = `{"workflow_id":"altered"}`
				raw, _ = json.Marshal(value)
			}
			return raw, count, err
		}
		run, err := executeItem14OpenRouterDirectV3(context.Background(), filepath.Join(parent, "item14-openrouter-direct-shadow-v3"), deps)
		if err != nil || calls != 4 || run.CandidateFailureCount != 1 || run.CandidatePassCount != 3 {
			t.Fatalf("v3 candidate-local continuation failed: %+v %v", run, err)
		}
	})
	t.Run("provider global", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), ".qualification-evidence")
		_ = os.MkdirAll(parent, 0o700)
		deps := newFakeItem14OpenRouterDirectDependenciesV3()
		base := deps.InvokeOnce
		calls := 0
		deps.InvokeOnce = func(ctx context.Context, credential []byte, endpoint string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
			calls++
			raw, count, err := base(ctx, credential, endpoint, plan)
			var value map[string]any
			_ = json.Unmarshal(raw, &value)
			value["openrouter_metadata"].(map[string]any)["attempts"].([]any)[0].(map[string]any)["provider"] = "Other"
			raw, _ = json.Marshal(value)
			return raw, count, err
		}
		root := filepath.Join(parent, "item14-openrouter-direct-shadow-v3")
		if _, err := executeItem14OpenRouterDirectV3(context.Background(), root, deps); err == nil || calls != 1 {
			t.Fatalf("v3 provider drift did not halt once: calls=%d err=%v", calls, err)
		}
		if _, err := executeItem14OpenRouterDirectV3(context.Background(), filepath.Join(parent, "alternate"), newFakeItem14OpenRouterDirectDependenciesV3()); err == nil {
			t.Fatal("v3 alternate-root replay passed")
		}
	})
}
