package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestItem14OpenRouterDirectV2PlanFakeAndPrepareAreProviderFree(t *testing.T) {
	plan, err := item14OpenRouterDirectPlanV2(offlineContracts(t))
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["credential_read"] != false || plan["status"] != "frozen_not_launched" {
		t.Fatalf("unsafe plan: %+v", plan)
	}
	preflight, err := item14OpenRouterDirectPrepareV2(offlineContracts(t))
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.Ready || preflight.ProviderCallsMade != 0 || preflight.CredentialReads != 0 || preflight.RetryCount != 0 || preflight.RedirectCount != 0 || !preflight.ProviderOrderExact || !preflight.ProviderOnlyExact || !preflight.RouterMetadata {
		t.Fatalf("unsafe preflight: %+v", preflight)
	}
	run, err := item14OpenRouterDirectFakeV2()
	if err != nil {
		t.Fatal(err)
	}
	if run.ExecutionMode != "deterministic_fake_provider_free" || run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 4 || run.CandidatePassCount != 4 || run.RetryCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.RoleModelSelectionCount != 0 {
		t.Fatalf("unsafe fake run: %+v", run)
	}
}

func TestItem14OpenRouterDirectV2AuthorityClaimBlocksReplayAndAlternateRoot(t *testing.T) {
	parent := filepath.Join(t.TempDir(), ".qualification-evidence")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "item14-openrouter-direct-shadow-v2")
	if _, err := executeItem14OpenRouterDirectV2(context.Background(), root, newFakeItem14OpenRouterDirectDependenciesV2()); err != nil {
		t.Fatal(err)
	}
	if _, err := executeItem14OpenRouterDirectV2(context.Background(), root, newFakeItem14OpenRouterDirectDependenciesV2()); err == nil {
		t.Fatal("same-root replay passed")
	}
	if _, err := executeItem14OpenRouterDirectV2(context.Background(), filepath.Join(parent, "alternate"), newFakeItem14OpenRouterDirectDependenciesV2()); err == nil {
		t.Fatal("alternate-root replay passed")
	}
}

func TestItem14OpenRouterDirectV2CandidateResponseFailureContinues(t *testing.T) {
	parent := filepath.Join(t.TempDir(), ".qualification-evidence")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	deps := newFakeItem14OpenRouterDirectDependenciesV2()
	baseInvoke := deps.InvokeOnce
	calls := 0
	deps.InvokeOnce = func(ctx context.Context, credential []byte, endpoint string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
		calls++
		raw, requests, err := baseInvoke(ctx, credential, endpoint, plan)
		if calls == 1 && err == nil {
			var response map[string]any
			if decodeErr := json.Unmarshal(raw, &response); decodeErr != nil {
				return nil, requests, decodeErr
			}
			choice := response["choices"].([]any)[0].(map[string]any)
			choice["message"].(map[string]any)["content"] = `{"workflow_id":"altered"}`
			raw, _ = json.Marshal(response)
		}
		return raw, requests, err
	}
	run, err := executeItem14OpenRouterDirectV2(context.Background(), filepath.Join(parent, "item14-openrouter-direct-shadow-v2"), deps)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || run.CandidateFailureCount != 1 || run.CandidatePassCount != 3 || run.Observations[0].Status != "candidate_response_contract_failure" {
		t.Fatalf("candidate-local failure halted unaffected workflows: %+v", run)
	}
}

func TestItem14OpenRouterDirectV2ProviderDriftGloballyHaltsWithoutReplay(t *testing.T) {
	parent := filepath.Join(t.TempDir(), ".qualification-evidence")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	deps := newFakeItem14OpenRouterDirectDependenciesV2()
	baseInvoke := deps.InvokeOnce
	calls := 0
	deps.InvokeOnce = func(ctx context.Context, credential []byte, endpoint string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
		calls++
		raw, requests, err := baseInvoke(ctx, credential, endpoint, plan)
		if err != nil {
			return raw, requests, err
		}
		var response map[string]any
		if decodeErr := json.Unmarshal(raw, &response); decodeErr != nil {
			return nil, requests, decodeErr
		}
		metadata := response["openrouter_metadata"].(map[string]any)
		metadata["attempts"].([]any)[0].(map[string]any)["provider"] = "Other"
		raw, _ = json.Marshal(response)
		return raw, requests, nil
	}
	root := filepath.Join(parent, "item14-openrouter-direct-shadow-v2")
	if _, err := executeItem14OpenRouterDirectV2(context.Background(), root, deps); err == nil {
		t.Fatal("provider drift passed")
	}
	if calls != 1 {
		t.Fatalf("provider drift made %d calls", calls)
	}
	if _, err := os.Stat(filepath.Join(root, "01-local-intake-decision-coordination", "global-halt.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := executeItem14OpenRouterDirectV2(context.Background(), filepath.Join(parent, "alternate"), newFakeItem14OpenRouterDirectDependenciesV2()); err == nil {
		t.Fatal("globally halted authority replayed")
	}
}

func TestOpenRouterDirectRoundTripperV2DeniesDuplicateAndMutation(t *testing.T) {
	definition, _, plans, err := loadItem14OpenRouterDirectPlansV2(offlineContracts(t))
	if err != nil {
		t.Fatal(err)
	}
	plan := plans[0]
	base := roundTripFuncV2(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(fakeOpenRouterDirectResponseV2(plan)))}, nil
	})
	gate := &openRouterDirectRoundTripperV2{endpoint: definition.Endpoint, plan: plan, base: base}
	request := newOpenRouterDirectTestRequestV2(t, definition.Endpoint, plan.RequestBody)
	if _, err := gate.RoundTrip(request); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.RoundTrip(newOpenRouterDirectTestRequestV2(t, definition.Endpoint, plan.RequestBody)); err == nil {
		t.Fatal("duplicate transport request passed")
	}

	mutatedGate := &openRouterDirectRoundTripperV2{endpoint: definition.Endpoint, plan: plan, base: base}
	mutated := append([]byte(nil), plan.RequestBody...)
	mutated = bytes.Replace(mutated, []byte(`"allow_fallbacks":false`), []byte(`"allow_fallbacks":true`), 1)
	if _, err := mutatedGate.RoundTrip(newOpenRouterDirectTestRequestV2(t, definition.Endpoint, mutated)); err == nil {
		t.Fatal("mutated transport request passed")
	}
}

type roundTripFuncV2 func(*http.Request) (*http.Response, error)

func (fn roundTripFuncV2) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func newOpenRouterDirectTestRequestV2(t *testing.T, endpoint string, body []byte) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request.GetBody = nil
	request.Header.Set("Authorization", "Bearer fixture")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-OpenRouter-Metadata", "enabled")
	return request
}
