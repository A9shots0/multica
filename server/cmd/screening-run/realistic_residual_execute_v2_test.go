package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type residualV2UsageProvider struct {
	input, output, reasoning int64
	cost                     float64
	calls                    int
	err                      error
}

func (provider *residualV2UsageProvider) Next(_ context.Context, _ agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
	provider.calls++
	raw := residualV2Receipt(provider.calls, provider.input, provider.output, provider.reasoning, provider.cost)
	return agentqualification.RealisticProviderResponse{RawReceipt: raw, Action: agentqualification.RealisticCandidateAction{Kind: "read", Path: "TASK.md"}}, provider.err
}

func residualV2Receipt(call int, input, output, reasoning int64, cost float64) []byte {
	session := fmt.Sprintf("residual-v2-%d", call)
	message := fmt.Sprintf("message-%d", call)
	usage := fmt.Sprintf(`"cost":%f,"tokens":{"input":%d,"output":%d,"reasoning":%d,"cache":{"read":0,"write":0}}`, cost, input, output, reasoning)
	jsonl := fmt.Sprintf("{\"type\":\"step_start\",\"timestamp\":1000,\"sessionID\":%q,\"part\":{\"sessionID\":%q,\"messageID\":%q,\"type\":\"step-start\"}}\n{\"type\":\"text\",\"timestamp\":1001,\"sessionID\":%q,\"part\":{\"sessionID\":%q,\"messageID\":%q,\"type\":\"text\",\"text\":\"{\\\"ok\\\":true}\"}}\n{\"type\":\"step_finish\",\"timestamp\":1002,\"sessionID\":%q,\"part\":{\"sessionID\":%q,\"messageID\":%q,\"type\":\"step-finish\",%s}}\n", session, session, message, session, session, message, session, session, message, usage)
	exported := fmt.Sprintf(`{"info":{"id":%q},"messages":[{"info":{"id":%q,"sessionID":%q,"role":"assistant","modelID":"fake","providerID":"openrouter","summary":false,"finish":"stop",%s,"error":null},"parts":[{"sessionID":%q,"messageID":%q,"type":"text","text":"{\"ok\":true}"}]}]}`, session, message, session, usage, session, message)
	raw, _ := json.Marshal(map[string]any{"jsonl": jsonl, "export": json.RawMessage(exported), "routing": map[string]any{"allow_fallbacks": false}})
	return raw
}

func TestRealisticResidualV2PlanIsProviderFreeAndReviewed(t *testing.T) {
	plan, err := realisticResidualV2Plan()
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["provider_ready"] != true || plan["matrix_status"] != "frozen_reviewed" {
		t.Fatalf("reviewed residual plan is inconsistent: %+v", plan)
	}
	plans := plan["plans"].([]map[string]any)
	if len(plans) != 2 {
		t.Fatalf("residual plan must contain exact R05/R06 bindings: %+v", plans)
	}
	for _, binding := range plans {
		if binding["model"] != "openrouter/openai/gpt-5.4" || binding["effort"] != "medium" || binding["serving_provider"] != "openai" || binding["provider_fallbacks"] != false {
			t.Fatalf("residual binding drifted: %+v", binding)
		}
	}
}

func TestRunRealisticResidualV2PassesBothAndReusesDurableResults(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
	matrix.Status = "frozen_reviewed"
	root, _ := filepath.Abs(t.TempDir())
	factoryCalls := 0
	factory := func(candidate agentqualification.RealisticCandidate, _, _, _ int64) agentqualification.RealisticProvider {
		factoryCalls++
		if factoryCalls == 1 {
			return &realisticNextCommandFakeProvider{key: candidate.ConfigurationID + "-r05", actions: realisticNextCommandR05Pass()}
		}
		return &realisticNextCommandFakeProvider{key: candidate.ConfigurationID + "-r06", actions: realisticNextCommandR06Pass()}
	}
	result, err := runRealisticResidualV2(context.Background(), root, matrix, 0, factory)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || len(result.Observations) != 2 || result.Observations[0].Status != "passed" || result.Observations[1].Status != "passed" || factoryCalls != 2 {
		t.Fatalf("residual staged run failed: %+v factory=%d", result, factoryCalls)
	}
	again, err := runRealisticResidualV2(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		t.Fatal("immutable completed residual campaign attempted another provider")
		return nil
	})
	if err != nil || !reflectCampaignAccountingEqual(result, again) {
		t.Fatalf("durable residual result was not reused: first=%+v again=%+v err=%v", result, again, err)
	}
}

func reflectCampaignAccountingEqual(a, b realisticNextCampaignResult) bool {
	return a.SchemaVersion == b.SchemaVersion && a.MatrixID == b.MatrixID && a.Status == b.Status && a.ProviderCalls == b.ProviderCalls && a.InputTokens == b.InputTokens && a.OutputTokens == b.OutputTokens && a.ReasoningTokens == b.ReasoningTokens && a.CostMicros == b.CostMicros && len(a.Observations) == len(b.Observations)
}

func TestRunRealisticResidualV2MakesObservationExhaustionCandidateLocal(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
	matrix.Status = "frozen_reviewed"
	root, _ := filepath.Abs(t.TempDir())
	provider := &residualV2UsageProvider{input: 75_001, output: 1, reasoning: 0, cost: 0.01}
	result, err := runRealisticResidualV2(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		return provider
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || result.HaltReason != "" || len(result.Observations) != 2 || result.Observations[0].Status != "failed" || result.Observations[0].FailureClass != "candidate_resource_exhaustion" || result.Observations[0].Run == nil || result.Observations[0].Run.GlobalHalt || result.Observations[1].Status != "skipped" || provider.calls != 1 {
		t.Fatalf("observation exhaustion leaked into campaign halt: %+v calls=%d", result, provider.calls)
	}
}

func TestRunRealisticResidualV2EnforcesCombinedGeneratedCeilingBeforeSecondCall(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
	matrix.Status = "frozen_reviewed"
	root, _ := filepath.Abs(t.TempDir())
	provider := &residualV2UsageProvider{input: 35_000, output: 10_001, reasoning: 10_000, cost: 0.01}
	result, err := runRealisticResidualV2(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		return provider
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || result.Observations[0].FailureClass != "candidate_resource_exhaustion" || provider.calls != 1 {
		t.Fatalf("combined generated ceiling did not stop before a second call: %+v calls=%d", result, provider.calls)
	}
}

func TestRunRealisticResidualV2RuntimePreflightFailureIsDurableGlobalHalt(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
	matrix.Status = "frozen_reviewed"
	root, _ := filepath.Abs(t.TempDir())
	provider := &residualV2UsageProvider{err: agentqualification.ErrRealisticProviderPreflight}
	result, err := runRealisticResidualV2(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		return provider
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "global_halt" || result.HaltReason != "runtime_or_config_failure" || len(result.Observations) != 1 || provider.calls != 1 {
		t.Fatalf("runtime preflight failure was not a global halt: %+v calls=%d", result, provider.calls)
	}
	var stored realisticNextCampaignResult
	if err := readJSONFile(filepath.Join(root, "campaign-result.json"), &stored); err != nil || stored.HaltReason != "runtime_or_config_failure" {
		t.Fatalf("runtime global halt was not durably recorded: %+v err=%v", stored, err)
	}
}

func TestRunRealisticResidualV2InfrastructureFailuresAreGlobal(t *testing.T) {
	tests := []struct {
		name, want string
		err        error
	}{
		{name: "credential", err: agentqualification.ErrRealisticCredentialFailure, want: "credential_failure"},
		{name: "credits", err: agentqualification.ErrRealisticBudgetBreach, want: "credit_failure"},
		{name: "transport", err: agentqualification.ErrRealisticProviderOutcomeAmbiguous, want: "transport_failure"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
			matrix.Status = "frozen_reviewed"
			root, _ := filepath.Abs(t.TempDir())
			provider := &residualV2UsageProvider{input: 10, output: 2, reasoning: 1, cost: 0.000003, err: test.err}
			result, err := runRealisticResidualV2(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
				return provider
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "global_halt" || result.HaltReason != test.want {
				t.Fatalf("%s failure was not globally halted: %+v", test.name, result)
			}
			var stored realisticNextCampaignResult
			if err := readJSONFile(filepath.Join(root, "campaign-result.json"), &stored); err != nil || stored.HaltReason != test.want {
				t.Fatalf("%s halt was not durable: %+v err=%v", test.name, stored, err)
			}
		})
	}
}

func TestRunRealisticResidualV2ActualCampaignAdmissionFailureIsGlobal(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
	matrix.Status = "frozen_reviewed"
	matrix.MaxIncrementalSpendMicros = 437_499
	root, _ := filepath.Abs(t.TempDir())
	factoryCalls := 0
	result, err := runRealisticResidualV2(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		factoryCalls++
		return &realisticNextCommandFakeProvider{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "global_halt" || result.HaltReason != "campaign_budget_breach" || result.ProviderCalls != 0 || factoryCalls != 0 {
		t.Fatalf("actual campaign cap did not halt before provider: %+v factory=%d", result, factoryCalls)
	}
}

func TestRealisticResidualV2ConservativePriceAndCampaignBoundaries(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
	candidate := matrix.Candidates[0]
	reservation, err := realisticResidualV2WorstCaseCostMicros(candidate, matrix)
	if err != nil || reservation != 687_500 {
		t.Fatalf("combined input/output/reasoning price reservation drifted: %d err=%v", reservation, err)
	}
	legacy := matrix.AsRealisticNextV1()
	if !realisticNextAdmitObservation(realisticNextCampaignResult{CostMicros: 312_500, InputTokens: 75_000}, legacy, reservation) {
		t.Fatal("exact $1/150k campaign boundary should admit one fully reserved observation")
	}
	if realisticNextAdmitObservation(realisticNextCampaignResult{CostMicros: 312_501, InputTokens: 75_000}, legacy, reservation) {
		t.Fatal("spend boundary admitted a call that could overshoot $1")
	}
	if realisticNextAdmitObservation(realisticNextCampaignResult{CostMicros: 312_500, InputTokens: 75_001}, legacy, reservation) {
		t.Fatal("token boundary admitted a call that could overshoot 150k")
	}
}

func TestRunRealisticResidualV2UnfinishedClaimHaltsBeforeProvider(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticResidualRunMatrixV2()
	matrix.Status = "frozen_reviewed"
	legacy := matrix.AsRealisticNextV1()
	legacy.Status = "frozen_reviewed"
	root, _ := filepath.Abs(t.TempDir())
	candidate := matrix.Candidates[0]
	claim := realisticNextProviderClaim{SchemaVersion: realisticNextClaimSchema, MatrixID: matrix.MatrixID, CandidateID: candidate.ConfigurationID, CaseID: "R05", RunFamilyID: realisticNextRunFamilyID(matrix.MatrixID, candidate.ConfigurationID, "R05"), Status: "provider_call_reserved"}
	if err := writeJSONExclusive(realisticNextClaimPath(root, candidate.ConfigurationID, "R05"), claim); err != nil {
		t.Fatal(err)
	}
	factoryCalls := 0
	result, err := runRealisticResidualV2(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		factoryCalls++
		return &realisticNextCommandFakeProvider{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "global_halt" || result.HaltReason != "transport_failure" || result.ProviderCalls != 0 || factoryCalls != 0 {
		t.Fatalf("unfinished claim did not halt before provider: %+v factory=%d legacy=%s", result, factoryCalls, legacy.MatrixID)
	}
}

func TestResidualV2CommandSourceNamesAreStable(t *testing.T) {
	for _, name := range []string{"realistic_residual_execute_v2.go", "realistic_residual_execute_v2_test.go"} {
		if !strings.HasSuffix(name, ".go") {
			t.Fatal(name)
		}
	}
	if _, err := os.Stat("realistic_residual_execute_v2.go"); err != nil {
		t.Fatal(err)
	}
}
