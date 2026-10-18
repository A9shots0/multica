package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestOpenRouterDirectShadowV4ContinuesOnlyUnattemptedWorkflows(t *testing.T) {
	definition, suite, plans := loadOpenRouterDirectShadowV4Test(t)
	if len(plans) != 3 || plans[0].WorkflowID != "local-implementation-review-mobile" || definition.AntecedentWorkflowID == plans[0].WorkflowID {
		t.Fatal("v4 replayed or omitted a workflow")
	}
	for _, plan := range plans {
		if err := historicalContracts(t).ValidateOpenRouterDirectShadowPlanV4(definition, plan); err != nil {
			t.Fatal(err)
		}
	}
	_ = suite
}

func TestOpenRouterDirectShadowV4DefinitionMutationsFailClosed(t *testing.T) {
	definition, _, _ := loadOpenRouterDirectShadowV4Test(t)
	mutations := []func(*OpenRouterDirectShadowDefinitionV4){
		func(v *OpenRouterDirectShadowDefinitionV4) { v.AntecedentExactRouteProved = false },
		func(v *OpenRouterDirectShadowDefinitionV4) { v.AntecedentV3TerminalReceiptSHA256 = "altered" },
		func(v *OpenRouterDirectShadowDefinitionV4) { v.AntecedentResponseContractValid = true },
		func(v *OpenRouterDirectShadowDefinitionV4) {
			v.WorkflowIDs = append([]string{v.AntecedentWorkflowID}, v.WorkflowIDs...)
		},
		func(v *OpenRouterDirectShadowDefinitionV4) { v.Candidate.ServingProvider = "other" },
		func(v *OpenRouterDirectShadowDefinitionV4) { v.RetryCount = 1 },
		func(v *OpenRouterDirectShadowDefinitionV4) { v.ProviderAttemptEvidencePolicy = "required" },
		func(v *OpenRouterDirectShadowDefinitionV4) { v.ToolsAllowed = true },
		func(v *OpenRouterDirectShadowDefinitionV4) { v.TotalPriceReservationMicros++ },
		func(v *OpenRouterDirectShadowDefinitionV4) {
			v.RequestSHA256ByWorkflow = map[string]string{"local-implementation-review-mobile": "altered"}
		},
		func(v *OpenRouterDirectShadowDefinitionV4) { v.ResponseSchemaSHA256 = "altered" },
	}
	for index, mutate := range mutations {
		candidate := definition
		mutate(&candidate)
		if err := historicalContracts(t).ValidateOpenRouterDirectShadowDefinitionV4(candidate); err == nil {
			t.Fatalf("v4 mutation %d passed", index)
		}
	}
}

func TestOpenRouterDirectShadowV4PlanAndAttemptMutationsFailClosed(t *testing.T) {
	definition, suite, plans := loadOpenRouterDirectShadowV4Test(t)
	plan := plans[0]
	alteredPlan := plan
	alteredPlan.WorkflowID = "local-intake-decision-coordination"
	if err := historicalContracts(t).ValidateOpenRouterDirectShadowPlanV4(definition, alteredPlan); err == nil {
		t.Fatal("v4 unlisted workflow passed")
	}
	var body map[string]any
	if err := json.Unmarshal(plan.RequestBody, &body); err != nil {
		t.Fatal(err)
	}
	body["messages"].([]any)[0].(map[string]any)["content"] = "altered"
	alteredPlan = plan
	alteredPlan.RequestBody, _ = json.Marshal(body)
	alteredPlan.RequestSHA256 = sha256Hex(alteredPlan.RequestBody)
	if err := historicalContracts(t).ValidateOpenRouterDirectShadowPlanV4(definition, alteredPlan); err == nil {
		t.Fatal("v4 altered system prompt passed")
	}

	raw := fakeOpenRouterDirectShadowResponseV4Test(t, definition, suite, plan, true)
	var response map[string]any
	_ = json.Unmarshal(raw, &response)
	metadata := response["openrouter_metadata"].(map[string]any)
	metadata["attempts"] = []any{map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "status": 200}}
	oneExact, _ := json.Marshal(response)
	if _, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV4(definition, suite, plan, oneExact, 1, "start", "finish", 1); err != nil {
		t.Fatalf("v4 one exact attempt failed: %v", err)
	}
	metadata["attempts"] = []any{map[string]any{"provider": "Other", "model": definition.ExpectedEndpointModel, "status": 200}}
	wrong, _ := json.Marshal(response)
	if _, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV4(definition, suite, plan, wrong, 1, "start", "finish", 1); err == nil {
		t.Fatal("v4 wrong attempt passed")
	}
	metadata["attempts"] = []any{
		map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "status": 200},
		map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "status": 200},
	}
	multiple, _ := json.Marshal(response)
	if _, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV4(definition, suite, plan, multiple, 1, "start", "finish", 1); err == nil {
		t.Fatal("v4 multiple attempts passed")
	}
}

func TestOpenRouterDirectShadowV4ActualMetadataShapeAndCandidateLocalFailure(t *testing.T) {
	definition, suite, plans := loadOpenRouterDirectShadowV4Test(t)
	raw := fakeOpenRouterDirectShadowResponseV4Test(t, definition, suite, plans[0], true)
	record, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV4(definition, suite, plans[0], raw, 1, "start", "finish", 1)
	if err != nil || record.Status != "passed" || record.ActualServingProvider != "alibaba" || record.RouterAttemptCount != 1 {
		t.Fatalf("v4 actual metadata shape failed: %+v %v", record, err)
	}
	invalid := fakeOpenRouterDirectShadowResponseV4Test(t, definition, suite, plans[0], false)
	record, err = historicalContracts(t).BuildOpenRouterDirectShadowObservationV4(definition, suite, plans[0], invalid, 1, "start", "finish", 1)
	if err != nil || record.Status != "candidate_response_contract_failure" {
		t.Fatalf("v4 candidate failure did not remain local: %+v %v", record, err)
	}
	var drift map[string]any
	_ = json.Unmarshal(raw, &drift)
	drift["provider"] = "Other"
	driftRaw, _ := json.Marshal(drift)
	if _, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV4(definition, suite, plans[0], driftRaw, 1, "start", "finish", 1); err == nil {
		t.Fatal("v4 top-level provider drift passed")
	}
}

func TestOpenRouterDirectShadowV4CombinedRunCannotQualifyAfterAntecedentFailure(t *testing.T) {
	definition, suite, plans := loadOpenRouterDirectShadowV4Test(t)
	records := make([]OpenRouterDirectShadowObservationV2Record, 0, 3)
	for _, plan := range plans {
		record, err := historicalContracts(t).BuildOpenRouterDirectShadowObservationV4(definition, suite, plan, fakeOpenRouterDirectShadowResponseV4Test(t, definition, suite, plan, true), 1, "start", "finish", 1)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	run, err := historicalContracts(t).BuildOpenRouterDirectShadowRunV4(definition, suite, records, true)
	if err != nil || run.CandidateQualified || run.AntecedentFailureCount != 1 || run.TotalEvaluatedWorkflows != 4 || run.CurrentPassCount != 3 || run.CombinedCostMicros != 529 {
		t.Fatalf("v4 combined result drifted: %+v %v", run, err)
	}
}

func loadOpenRouterDirectShadowV4Test(t *testing.T) (OpenRouterDirectShadowDefinitionV4, LocalShadowWorkflowSuiteV1, []OpenRouterDirectShadowPlanV2) {
	t.Helper()
	definition, err := historicalContracts(t).CurrentOpenRouterDirectShadowDefinitionV4()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := historicalContracts(t).BuildOpenRouterDirectShadowPlansV4(definition, suite)
	if err != nil {
		t.Fatal(err)
	}
	return definition, suite, plans
}

func fakeOpenRouterDirectShadowResponseV4Test(t *testing.T, definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, valid bool) []byte {
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
	value := map[string]any{"id": "gen-v4-" + plan.WorkflowID, "object": "chat.completion", "model": plan.APIModel, "provider": "Alibaba", "choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120, "cost": json.Number("0.000010"), "prompt_tokens_details": map[string]any{"cached_tokens": 0}, "completion_tokens_details": map[string]any{"reasoning_tokens": 0}}, "openrouter_metadata": map[string]any{"requested": plan.APIModel, "strategy": "direct", "attempt": 1, "endpoints": map[string]any{"total": 4, "available": []any{map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "selected": true}}}}}
	raw, _ := json.Marshal(value)
	return raw
}
