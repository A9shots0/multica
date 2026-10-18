package agentqualification

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOpenRouterLocalShadowV1ProviderFreeDefinitionPlansAndFakeRun(t *testing.T) {
	definition, err := historicalContracts(t).CurrentOpenRouterLocalShadowDefinitionV1()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := historicalContracts(t).BuildOpenRouterLocalShadowPlansV1(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 4 {
		t.Fatalf("plan count = %d", len(plans))
	}
	for index, plan := range plans {
		if plan.CaseID != definition.WorkflowIDs[index] || plan.Transport != OpenRouterLocalShadowTransportV1 || plan.Model != definition.Candidate.Model || plan.ServingProvider != definition.Candidate.ServingProvider || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks {
			t.Fatalf("plan %d escaped route: %+v", index, plan)
		}
		if _, err := ValidateOpenCodeResolvedConfig(plan, resolvedOpenCodeConfigFor(plan)); err != nil {
			t.Fatalf("plan %d resolved config rejected: %v", index, err)
		}
	}
	run, err := historicalContracts(t).RunOpenRouterLocalShadowFakeV1(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	if err := historicalContracts(t).ValidateOpenRouterLocalShadowRunV1(definition, suite, run); err != nil {
		t.Fatal(err)
	}
	if run.ExecutionMode != "deterministic_fake_provider_free" || run.ProviderRequestCount != 0 || run.SimulatedResponseCount != 4 || run.RetryCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.RoleModelSelectionCount != 0 || len(run.Receipts) != 4 {
		t.Fatalf("fake run boundary drifted: %+v", run)
	}
}

func TestOpenRouterLocalShadowV1RejectsDefinitionRouteBudgetAndAuthorityDrift(t *testing.T) {
	definition, err := historicalContracts(t).CurrentOpenRouterLocalShadowDefinitionV1()
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*OpenRouterLocalShadowDefinitionV1){
		"Claude model": func(v *OpenRouterLocalShadowDefinitionV1) {
			v.Candidate.Model = "openrouter/anthropic/claude-sonnet-4.6"
			v.Candidate.ServingProvider = "anthropic"
		},
		"OpenAI model": func(v *OpenRouterLocalShadowDefinitionV1) {
			v.Candidate.Model = "openrouter/openai/gpt-5-mini"
			v.Candidate.ServingProvider = "openai"
		},
		"provider mismatch":   func(v *OpenRouterLocalShadowDefinitionV1) { v.Candidate.ServingProvider = "other" },
		"fallback":            func(v *OpenRouterLocalShadowDefinitionV1) { v.Candidate.ProviderFallbacks = true },
		"retry":               func(v *OpenRouterLocalShadowDefinitionV1) { v.RetryCount = 1 },
		"extra call":          func(v *OpenRouterLocalShadowDefinitionV1) { v.MaxProviderCalls = 5 },
		"spend expansion":     func(v *OpenRouterLocalShadowDefinitionV1) { v.CumulativeOpenRouterCeilingMicros++ },
		"increment expansion": func(v *OpenRouterLocalShadowDefinitionV1) { v.SliceIncrementalCeilingMicros++ },
		"eligibility hash": func(v *OpenRouterLocalShadowDefinitionV1) {
			v.EligibilityEvidenceSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"input cap":         func(v *OpenRouterLocalShadowDefinitionV1) { v.MaxInputTokensPerCall++ },
		"input price":       func(v *OpenRouterLocalShadowDefinitionV1) { v.InputPriceMicrosPerMillion++ },
		"output price":      func(v *OpenRouterLocalShadowDefinitionV1) { v.GeneratedPriceMicrosPerMillion++ },
		"price reservation": func(v *OpenRouterLocalShadowDefinitionV1) { v.PerCallReservationMicros++ },
		"tools":             func(v *OpenRouterLocalShadowDefinitionV1) { v.ToolsAllowed = true },
		"repository":        func(v *OpenRouterLocalShadowDefinitionV1) { v.RepositoryAccessAllowed = true },
		"selection":         func(v *OpenRouterLocalShadowDefinitionV1) { v.RoleModelSelectionAllowed = true },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			copy := definition
			copy.WorkflowIDs = append([]string(nil), definition.WorkflowIDs...)
			mutate(&copy)
			if err := historicalContracts(t).ValidateOpenRouterLocalShadowDefinitionV1(copy); err == nil {
				t.Fatal("altered definition passed")
			}
		})
	}
}

func TestOpenRouterLocalShadowV1RejectsAlteredDuplicateAndReplayEvidence(t *testing.T) {
	definition, _ := historicalContracts(t).CurrentOpenRouterLocalShadowDefinitionV1()
	suite, _ := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	base, err := historicalContracts(t).RunOpenRouterLocalShadowFakeV1(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*OpenRouterLocalShadowRunV1){
		"model":     func(v *OpenRouterLocalShadowRunV1) { v.Receipts[0].ActualModel = "openrouter/other/model" },
		"provider":  func(v *OpenRouterLocalShadowRunV1) { v.Receipts[0].ActualServingProvider = "other" },
		"workflow":  func(v *OpenRouterLocalShadowRunV1) { v.Receipts[0].WorkflowID = v.Receipts[1].WorkflowID },
		"response":  func(v *OpenRouterLocalShadowRunV1) { v.Receipts[0].Response.BoundaryVerdict = "other" },
		"duplicate": func(v *OpenRouterLocalShadowRunV1) { v.Receipts[1].ObservationID = v.Receipts[0].ObservationID },
		"replay": func(v *OpenRouterLocalShadowRunV1) {
			v.Receipts[1].ProviderResponseID = v.Receipts[0].ProviderResponseID
		},
		"retry": func(v *OpenRouterLocalShadowRunV1) { v.RetryCount = 1 },
		"tool":  func(v *OpenRouterLocalShadowRunV1) { v.ToolCallCount = 1 },
		"extra spend": func(v *OpenRouterLocalShadowRunV1) {
			v.Receipts[0].ProviderReportedCostMicros += definition.SliceIncrementalCeilingMicros
			v.TotalCostMicros += definition.SliceIncrementalCeilingMicros
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			clone := cloneOpenRouterLocalShadowRunForTest(t, base)
			mutate(&clone)
			if err := historicalContracts(t).ValidateOpenRouterLocalShadowRunV1(definition, suite, clone); err == nil {
				t.Fatal("altered run passed")
			}
		})
	}
}

func TestOpenRouterLocalShadowV1RejectsBackgroundSummaryAndWrongModelSessionActivity(t *testing.T) {
	definition, _ := historicalContracts(t).CurrentOpenRouterLocalShadowDefinitionV1()
	suite, _ := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	plans, err := historicalContracts(t).BuildOpenRouterLocalShadowPlansV1(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	base := fakeOpenRouterLocalShadowExportV1(plans[0])
	if err := ValidateOpenRouterLocalShadowFullSessionV1(plans[0], base); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(map[string]any){
		"background assistant": func(value map[string]any) {
			messages := value["messages"].([]any)
			value["messages"] = append(messages, map[string]any{"info": map[string]any{"role": "assistant", "providerID": "openrouter", "modelID": "qwen/qwen3-coder-next", "summary": false}, "parts": []any{map[string]any{"type": "text", "metadata": map[string]any{"selected": true, "provider": "alibaba"}}}})
		},
		"summary assistant": func(value map[string]any) {
			messages := value["messages"].([]any)
			messages[1].(map[string]any)["info"].(map[string]any)["summary"] = true
		},
		"wrong model": func(value map[string]any) {
			messages := value["messages"].([]any)
			messages[1].(map[string]any)["info"].(map[string]any)["modelID"] = "qwen/other"
		},
		"missing provider metadata": func(value map[string]any) {
			messages := value["messages"].([]any)
			messages[1].(map[string]any)["parts"] = []any{map[string]any{"type": "text"}}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(base, &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			raw, _ := json.Marshal(value)
			if err := ValidateOpenRouterLocalShadowFullSessionV1(plans[0], raw); err == nil {
				t.Fatal("unsafe full-session evidence passed")
			}
		})
	}
}

func cloneOpenRouterLocalShadowRunForTest(t *testing.T, source OpenRouterLocalShadowRunV1) OpenRouterLocalShadowRunV1 {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var clone OpenRouterLocalShadowRunV1
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(&clone, &source) && len(clone.Receipts) == 0 {
		t.Fatal("clone unexpectedly empty")
	}
	return clone
}
