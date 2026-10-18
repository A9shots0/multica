package agentqualification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenRouterDirectShadowV3RemovesOnlyUnsupportedParameter(t *testing.T) {
	definition, suite, plans := loadOpenRouterDirectShadowV3Test(t)
	if definition.AntecedentV2DefinitionSHA256 != OpenRouterDirectShadowDefinitionV2SHA256() || len(plans) != 4 {
		t.Fatal("v3 antecedent or plan count drifted")
	}
	for _, plan := range plans {
		if err := historicalContracts(t).ValidateOpenRouterDirectShadowPlanV3(definition, plan); err != nil {
			t.Fatal(err)
		}
		body := string(plan.RequestBody)
		for _, required := range []string{`"allow_fallbacks":false`, `"only":["alibaba"]`, `"order":["alibaba"]`, `"stream":false`} {
			if !strings.Contains(body, required) {
				t.Fatalf("v3 request omitted %s", required)
			}
		}
		for _, forbidden := range []string{`"tools":`, `"plugins":`, `"models":`, `"response_format":`, `"require_parameters":`, "anthropic", "openai"} {
			if strings.Contains(strings.ToLower(body), forbidden) {
				t.Fatalf("v3 request contained forbidden parameter %s", forbidden)
			}
		}
	}
	_ = suite
}

func TestOpenRouterDirectShadowV3DefinitionMutationsFailClosed(t *testing.T) {
	definition, _, _ := loadOpenRouterDirectShadowV3Test(t)
	mutations := map[string]func(*OpenRouterDirectShadowDefinitionV3){
		"v2 receipt": func(v *OpenRouterDirectShadowDefinitionV3) {
			v.AntecedentV2TerminalResponseSHA256 = strings.Repeat("0", 64)
		},
		"endpoint model": func(v *OpenRouterDirectShadowDefinitionV3) { v.ExpectedEndpointModel = v.Candidate.APIModel },
		"Claude": func(v *OpenRouterDirectShadowDefinitionV3) {
			v.Candidate.Model = "openrouter/anthropic/claude-sonnet-4"
		},
		"fallback":          func(v *OpenRouterDirectShadowDefinitionV3) { v.Candidate.ProviderFallbacks = true },
		"retry":             func(v *OpenRouterDirectShadowDefinitionV3) { v.RetryCount = 1 },
		"redirect":          func(v *OpenRouterDirectShadowDefinitionV3) { v.RedirectsAllowed = true },
		"structured output": func(v *OpenRouterDirectShadowDefinitionV3) { v.ProviderStructuredOutputAllowed = true },
		"tool":              func(v *OpenRouterDirectShadowDefinitionV3) { v.ToolsAllowed = true },
		"production":        func(v *OpenRouterDirectShadowDefinitionV3) { v.ProductionAccessAllowed = true },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := definition
			mutate(&candidate)
			if err := historicalContracts(t).ValidateOpenRouterDirectShadowDefinitionV3(candidate); err == nil {
				t.Fatal("v3 definition mutation passed")
			}
		})
	}
}

func TestOpenRouterDirectShadowV3ExactRevisionAndCandidateLocalFailure(t *testing.T) {
	definition, suite, plans := loadOpenRouterDirectShadowV3Test(t)
	raw := fakeOpenRouterDirectShadowResponseV3Test(t, definition, suite, plans[0], true)
	record, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV3(definition, suite, plans[0], raw, 1, "start", "finish", 1)
	if err != nil || record.Status != "passed" || record.ActualModel != definition.Candidate.Model || record.ActualServingProvider != definition.Candidate.ServingProvider || record.RouterAttemptCount != 1 {
		t.Fatalf("exact v3 observation failed: %+v %v", record, err)
	}
	invalid := fakeOpenRouterDirectShadowResponseV3Test(t, definition, suite, plans[0], false)
	record, err = historicalContracts(t).BuildOpenRouterDirectShadowObservationV3(definition, suite, plans[0], invalid, 1, "start", "finish", 1)
	if err != nil || record.Status != "candidate_response_contract_failure" || record.ResponseContractError == "" {
		t.Fatalf("candidate response failure did not stay local: %+v %v", record, err)
	}
	var drift map[string]any
	_ = json.Unmarshal(raw, &drift)
	metadata := drift["openrouter_metadata"].(map[string]any)
	endpoints := metadata["endpoints"].(map[string]any)
	endpoints["total"] = 2
	endpoints["available"] = append(endpoints["available"].([]any), map[string]any{"provider": "Other", "model": "other/model", "selected": false})
	withUnselected, _ := json.Marshal(drift)
	if _, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV3(definition, suite, plans[0], withUnselected, 1, "start", "finish", 1); err != nil {
		t.Fatalf("non-selected router inventory incorrectly invalidated exact selected route: %v", err)
	}
	_ = json.Unmarshal(raw, &drift)
	metadata = drift["openrouter_metadata"].(map[string]any)
	metadata["attempts"].([]any)[0].(map[string]any)["model"] = definition.Candidate.APIModel
	driftRaw, _ := json.Marshal(drift)
	if _, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV3(definition, suite, plans[0], driftRaw, 1, "start", "finish", 1); err == nil {
		t.Fatal("unversioned endpoint model passed")
	}
}

func TestOpenRouterDirectShadowV3CompleteFakeRun(t *testing.T) {
	definition, suite, plans := loadOpenRouterDirectShadowV3Test(t)
	records := make([]OpenRouterDirectShadowObservationV2Record, 0, 4)
	for _, plan := range plans {
		record, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV3(definition, suite, plan, fakeOpenRouterDirectShadowResponseV3Test(t, definition, suite, plan, true), 1, "start", "finish", 1)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	run, err := historicalContracts(t).BuildOpenRouterDirectShadowRunV3(definition, suite, records, true)
	if err != nil || run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 4 || run.CandidatePassCount != 4 || run.TotalCostMicros != 40 {
		t.Fatalf("v3 fake run failed: %+v %v", run, err)
	}
	run.Observations[1].ProviderResponseID = run.Observations[0].ProviderResponseID
	if err := historicalContracts(t).ValidateOpenRouterDirectShadowRunV3(definition, suite, run); err == nil {
		t.Fatal("v3 duplicate response replay passed")
	}
}

func loadOpenRouterDirectShadowV3Test(t *testing.T) (OpenRouterDirectShadowDefinitionV3, LocalShadowWorkflowSuiteV1, []OpenRouterDirectShadowPlanV2) {
	t.Helper()
	definition, err := historicalContracts(t).CurrentOpenRouterDirectShadowDefinitionV3()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := historicalContracts(t).BuildOpenRouterDirectShadowPlansV3(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	return definition, suite, plans
}

func fakeOpenRouterDirectShadowResponseV3Test(t *testing.T, definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, valid bool) []byte {
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
	if !valid {
		contract.WorkflowID = "altered"
	}
	content, _ := json.Marshal(contract)
	value := map[string]any{
		"id": "gen-v3-" + plan.WorkflowID, "object": "chat.completion", "model": plan.APIModel,
		"choices":             []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}},
		"usage":               map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120, "cost": json.Number("0.000010"), "prompt_tokens_details": map[string]any{"cached_tokens": 0}, "completion_tokens_details": map[string]any{"reasoning_tokens": 0}},
		"openrouter_metadata": map[string]any{"requested": plan.APIModel, "strategy": "direct", "attempt": 1, "endpoints": map[string]any{"total": 1, "available": []any{map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "selected": true}}}, "attempts": []any{map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "status": 200}}},
	}
	raw, _ := json.Marshal(value)
	return raw
}
