package agentqualification

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenRouterDirectShadowV2DefinitionAndPlans(t *testing.T) {
	definition, err := historicalContracts(t).CurrentOpenRouterDirectShadowDefinitionV2()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := historicalContracts(t).BuildOpenRouterDirectShadowPlansV2(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 4 || !validSHA256(OpenRouterDirectShadowDefinitionV2SHA256()) {
		t.Fatalf("unexpected direct-shadow plan count or hash")
	}
	for _, plan := range plans {
		if err := historicalContracts(t).ValidateOpenRouterDirectShadowPlanV2(definition, plan); err != nil {
			t.Fatal(err)
		}
		body := string(plan.RequestBody)
		for _, required := range []string{`"allow_fallbacks":false`, `"only":["alibaba"]`, `"order":["alibaba"]`, `"require_parameters":true`, `"stream":false`, `"strict":true`} {
			if !strings.Contains(body, required) {
				t.Fatalf("request omitted %s", required)
			}
		}
		for _, forbidden := range []string{`"tools":`, `"plugins":`, `"models":`, "anthropic", "openai"} {
			if strings.Contains(strings.ToLower(body), forbidden) {
				t.Fatalf("request contained forbidden surface %s", forbidden)
			}
		}
	}
}

func TestOpenRouterDirectShadowV2DefinitionMutationsFailClosed(t *testing.T) {
	definition, err := historicalContracts(t).CurrentOpenRouterDirectShadowDefinitionV2()
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*OpenRouterDirectShadowDefinitionV2){
		"Claude model": func(v *OpenRouterDirectShadowDefinitionV2) {
			v.Candidate.Model = "openrouter/anthropic/claude-sonnet-4"
		},
		"OpenAI provider":    func(v *OpenRouterDirectShadowDefinitionV2) { v.Candidate.ServingProvider = "openai" },
		"fallback":           func(v *OpenRouterDirectShadowDefinitionV2) { v.Candidate.ProviderFallbacks = true },
		"retry":              func(v *OpenRouterDirectShadowDefinitionV2) { v.RetryCount = 1 },
		"redirect":           func(v *OpenRouterDirectShadowDefinitionV2) { v.RedirectsAllowed = true },
		"provider only":      func(v *OpenRouterDirectShadowDefinitionV2) { v.ProviderOnlyExact = false },
		"metadata":           func(v *OpenRouterDirectShadowDefinitionV2) { v.RouterMetadataRequired = false },
		"response failure":   func(v *OpenRouterDirectShadowDefinitionV2) { v.ResponseContractFailurePolicy = "global_halt" },
		"tools":              func(v *OpenRouterDirectShadowDefinitionV2) { v.ToolsAllowed = true },
		"repository":         func(v *OpenRouterDirectShadowDefinitionV2) { v.RepositoryAccessAllowed = true },
		"production":         func(v *OpenRouterDirectShadowDefinitionV2) { v.ProductionAccessAllowed = true },
		"selection":          func(v *OpenRouterDirectShadowDefinitionV2) { v.RoleModelSelectionAllowed = true },
		"cumulative ceiling": func(v *OpenRouterDirectShadowDefinitionV2) { v.CumulativeOpenRouterCeilingMicros++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := definition
			mutate(&candidate)
			if err := historicalContracts(t).ValidateOpenRouterDirectShadowDefinitionV2(candidate); err == nil {
				t.Fatal("mutation passed")
			}
		})
	}
}

func TestOpenRouterDirectShadowV2RequestMutationsFailClosed(t *testing.T) {
	definition, suite, plan := loadOpenRouterDirectShadowTestPlanV2(t)
	mutations := map[string]func(map[string]any){
		"model": func(v map[string]any) { v["model"] = "qwen/other" },
		"fallback": func(v map[string]any) {
			v["provider"].(map[string]any)["allow_fallbacks"] = true
		},
		"provider only": func(v map[string]any) { v["provider"].(map[string]any)["only"] = []any{"other"} },
		"tools":         func(v map[string]any) { v["tools"] = []any{} },
		"second model":  func(v map[string]any) { v["models"] = []any{"qwen/qwen3-coder-next"} },
	}
	_ = suite
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var body map[string]any
			if err := json.Unmarshal(plan.RequestBody, &body); err != nil {
				t.Fatal(err)
			}
			mutate(body)
			planCopy := plan
			planCopy.RequestBody, _ = json.Marshal(body)
			planCopy.RequestSHA256 = sha256Hex(planCopy.RequestBody)
			if err := historicalContracts(t).ValidateOpenRouterDirectShadowPlanV2(definition, planCopy); err == nil {
				t.Fatal("request mutation passed")
			}
		})
	}
}

func TestOpenRouterDirectShadowV2ExactProviderAndCandidateLocalFailure(t *testing.T) {
	definition, suite, plan := loadOpenRouterDirectShadowTestPlanV2(t)
	raw := fakeOpenRouterDirectShadowResponseV2(t, definition, suite, plan, true)
	record, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV2(definition, suite, plan, raw, 1, "2026-09-14T17:00:00Z", "2026-09-14T17:00:01Z", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "passed" || !record.ResponseContractValid || record.ActualModel != definition.Candidate.Model || record.ActualServingProvider != definition.Candidate.ServingProvider || record.RouterAttemptCount != 1 || record.ProviderRequestCount != 1 || record.ProviderReportedCostMicros != 10 {
		t.Fatalf("unexpected exact-provider record: %+v", record)
	}

	invalidRaw := fakeOpenRouterDirectShadowResponseV2(t, definition, suite, plan, false)
	invalid, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV2(definition, suite, plan, invalidRaw, 1, "2026-09-14T17:00:00Z", "2026-09-14T17:00:01Z", 1000)
	if err != nil {
		t.Fatalf("candidate-local response failure became global: %v", err)
	}
	if invalid.Status != "candidate_response_contract_failure" || invalid.ResponseContractValid || invalid.ResponseContractError == "" {
		t.Fatalf("candidate-local failure not sealed: %+v", invalid)
	}
}

func TestOpenRouterDirectShadowV2IdentityMutationsHalt(t *testing.T) {
	definition, suite, plan := loadOpenRouterDirectShadowTestPlanV2(t)
	mutations := map[string]func(map[string]any){
		"model": func(v map[string]any) { v["model"] = "qwen/other" },
		"requested": func(v map[string]any) {
			v["openrouter_metadata"].(map[string]any)["requested"] = "qwen/other"
		},
		"provider": func(v map[string]any) {
			metadata := v["openrouter_metadata"].(map[string]any)
			metadata["attempts"].([]any)[0].(map[string]any)["provider"] = "Other"
		},
		"second attempt": func(v map[string]any) {
			metadata := v["openrouter_metadata"].(map[string]any)
			metadata["attempts"] = append(metadata["attempts"].([]any), map[string]any{"provider": "Alibaba", "model": plan.APIModel, "status": 200})
		},
		"tool call": func(v map[string]any) {
			choices := v["choices"].([]any)
			choices[0].(map[string]any)["message"].(map[string]any)["tool_calls"] = []any{map[string]any{"id": "bad"}}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(fakeOpenRouterDirectShadowResponseV2(t, definition, suite, plan, true), &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			raw, _ := json.Marshal(value)
			if _, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV2(definition, suite, plan, raw, 1, "start", "finish", 1); err == nil {
				t.Fatal("identity mutation passed")
			}
		})
	}
}

func TestOpenRouterDirectShadowV2CompleteFakeRunAndReplayMutation(t *testing.T) {
	definition, err := historicalContracts(t).CurrentOpenRouterDirectShadowDefinitionV2()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := historicalContracts(t).BuildOpenRouterDirectShadowPlansV2(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	records := make([]OpenRouterDirectShadowObservationV2Record, 0, len(plans))
	for index, plan := range plans {
		raw := fakeOpenRouterDirectShadowResponseV2(t, definition, suite, plan, true)
		record, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV2(definition, suite, plan, raw, 1, "start", "finish", int64(index+1))
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	run, err := historicalContracts(t).BuildOpenRouterDirectShadowRunV2(definition, suite, records, true)
	if err != nil {
		t.Fatal(err)
	}
	if run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 4 || run.CandidatePassCount != 4 || run.CandidateFailureCount != 0 || run.TotalCostMicros != 40 {
		t.Fatalf("unexpected fake run: %+v", run)
	}
	replayed := run
	replayed.Observations = append([]OpenRouterDirectShadowObservationV2Record(nil), run.Observations...)
	replayed.Observations[1].ProviderResponseID = replayed.Observations[0].ProviderResponseID
	if err := historicalContracts(t).ValidateOpenRouterDirectShadowRunV2(definition, suite, replayed); err == nil {
		t.Fatal("duplicate provider response replay passed")
	}
}

func TestDecimalUSDMicrosCeilingV2(t *testing.T) {
	for input, want := range map[string]int64{"0": 0, "0.000010": 10, "0.0000001": 1, "1.25": 1_250_000} {
		got, err := decimalStringToMicrosCeilingV2(input)
		if err != nil || got != want {
			t.Fatalf("%s => %d, %v; want %d", input, got, err, want)
		}
	}
}

func loadOpenRouterDirectShadowTestPlanV2(t *testing.T) (OpenRouterDirectShadowDefinitionV2, LocalShadowWorkflowSuiteV1, OpenRouterDirectShadowPlanV2) {
	t.Helper()
	definition, err := historicalContracts(t).CurrentOpenRouterDirectShadowDefinitionV2()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := historicalContracts(t).BuildOpenRouterDirectShadowPlansV2(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	return definition, suite, plans[0]
}

func fakeOpenRouterDirectShadowResponseV2(t *testing.T, definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, validContract bool) []byte {
	t.Helper()
	var workflow LocalShadowWorkflowV1
	for _, candidate := range suite.Workflows {
		if candidate.ID == plan.WorkflowID {
			workflow = candidate
			break
		}
	}
	stageIDs, contractIDs := openRouterLocalShadowExpectedIDsV1(workflow)
	contract := OpenRouterLocalShadowResponseV1{WorkflowID: workflow.ID, SuiteSHA256: definition.SuiteSHA256, StageIDs: stageIDs, EvidenceContractIDs: contractIDs, BoundaryVerdict: "accepted_frozen_provider_free_contract"}
	if !validContract {
		contract.WorkflowID = "altered"
	}
	content, _ := json.Marshal(contract)
	value := map[string]any{
		"id": "gen-" + plan.WorkflowID, "object": "chat.completion", "model": plan.APIModel,
		"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}},
		"usage":   map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120, "cost": json.Number("0.000010"), "prompt_tokens_details": map[string]any{"cached_tokens": 0}, "completion_tokens_details": map[string]any{"reasoning_tokens": 0}},
		"openrouter_metadata": map[string]any{
			"requested": plan.APIModel, "strategy": "direct", "attempt": 1,
			"endpoints": map[string]any{"total": 1, "available": []any{map[string]any{"provider": "Alibaba", "model": plan.APIModel, "selected": true}}},
			"attempts":  []any{map[string]any{"provider": "Alibaba", "model": plan.APIModel, "status": 200}},
		},
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSpace(buffer.Bytes())
}
