package agentqualification

import (
	"reflect"
	"testing"
)

func TestOpenRouterScreeningBatchIsCappedComparableAndUnlaunched(t *testing.T) {
	batch, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "planned_not_launched" || len(batch.Candidates) != 4 || len(batch.Cases) != 5 || batch.Cases[0].CaseID != "T11" || batch.Cases[4].CaseID != "T09" || batch.GraderConfigurationID != "static-capability-breadth-v1" {
		t.Fatalf("unexpected batch shape: %+v", batch)
	}
	if batch.BatchID != "screen-static-capability-breadth-openrouter-v13" || batch.CandidateTransport != OpenCodeToolFreeTransportV6 || len(batch.APIBudgets) != 1 || len(batch.CapacityBudgets) != 0 || batch.APIBudgets[0].Provider != "openrouter" || batch.APIBudgets[0].MaxSpendMicros != 142104668 || batch.PerLaunchTokenReservation != 10000 {
		t.Fatalf("unexpected metered budget: %+v", batch.APIBudgets)
	}
	first := batch.Candidates[0]
	for _, candidate := range batch.Candidates[1:] {
		if candidate.RoleID != first.RoleID || candidate.InstructionSHA256 != first.InstructionSHA256 || candidate.PermissionVersion != first.PermissionVersion || candidate.RuntimeProvider != first.RuntimeProvider || candidate.ServingProvider == "" || candidate.ProviderFallbacks == nil || *candidate.ProviderFallbacks || !equalStringMap(candidate.SkillContentSHA256, first.SkillContentSHA256) {
			t.Fatalf("comparison changes more than model: %+v %+v", first, candidate)
		}
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 20 || projection.ReservedTokens != 200000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected projection: %+v", projection)
	}
	if batch.APIBudgets[0].MaxSpendMicros-102104668 != 40000000 {
		t.Fatalf("campaign spend ceiling is not the authorized $40 above baseline: %+v", batch.APIBudgets[0])
	}
	for _, required := range []string{"disqualify_candidate_on_competence_failure_or_disqualifier", "halt_batch_only_on_test_invalidity_evidence_integrity_unsafe_execution_transport_failure_or_budget_breach", "shared_failure_across_unrelated_models_triggers_test_audit_before_stronger_calls", "one_observation_per_candidate_case_no_retries", "static_screen_only_no_role_promotion"} {
		if !contains(batch.StopRules, required) {
			t.Fatalf("missing frozen campaign rule %s", required)
		}
	}
}

func TestCurrentOpenRouterBatchIsSurvivorReliabilityV26(t *testing.T) {
	batch, err := OpenRouterScreeningBatch()
	if err != nil {
		t.Fatalf("reviewed frozen batch unavailable: %v", err)
	}
	if batch.BatchID != "screen-static-survivor-reliability-openrouter-v26" || len(batch.Candidates) != 5 || len(batch.Cases) != 5 || batch.Repeats != 2 || batch.Cases[4].CaseVersion != "v2" || ProjectScreeningBatch(batch).Observations != 50 {
		t.Fatalf("unexpected current broad-screen batch: %+v", batch)
	}
}

func TestTwentySecondOpenRouterBatchExactlyRepeatsV21Survivors(t *testing.T) {
	batch, err := TwentySecondOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "planned_not_launched" || batch.Repeats != 2 || len(batch.Candidates) != 2 || len(batch.Cases) != 5 {
		t.Fatalf("unexpected v22 shape: %+v", batch)
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 20 || projection.ReservedTokens != 200000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected v22 projection: %+v", projection)
	}
	want := map[string]string{
		"openrouter/qwen/qwen3.8-flash":      "alibaba",
		"openrouter/openai/gpt-5.6-luna-pro": "openai",
	}
	for _, candidate := range batch.Candidates {
		if want[candidate.Model] != candidate.ServingProvider || candidate.ProviderFallbacks == nil || *candidate.ProviderFallbacks {
			t.Fatalf("v22 route drifted: %+v", candidate)
		}
		delete(want, candidate.Model)
	}
	if len(want) != 0 {
		t.Fatalf("v22 survivor missing: %+v", want)
	}
	prior, err := TwentyFirstOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	for index := range batch.Cases {
		if batch.Cases[index] != prior.Cases[index] {
			t.Fatalf("v22 case %d drifted from v21", index)
		}
	}
}

func TestTwentyFirstOpenRouterBatchScreensOnlyNewExactConfigurations(t *testing.T) {
	batch, err := TwentyFirstOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "planned_not_launched" || len(batch.Candidates) != 8 || len(batch.Cases) != 5 || batch.Repeats != 1 {
		t.Fatalf("unexpected v21 discovery shape: %+v", batch)
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 40 || projection.ReservedTokens != 400000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected v21 projection: %+v", projection)
	}
	wantModels := map[string]string{
		"openrouter/inference-net/schematron-v2-turbo": "inference-net",
		"openrouter/deepseek/deepseek-v4.1-flash":      "deepseek",
		"openrouter/qwen/qwen3.8-flash":                "alibaba",
		"openrouter/ibm-granite/granite-4.2-8b":        "deepinfra",
		"openrouter/minimax/minimax-m3":                "coreweave",
		"openrouter/nex-agi/nex-n2.5-mini:free":        "nex-agi",
		"openrouter/openai/gpt-5.6-luna-pro":           "openai",
		"openrouter/poolside/laguna-s-2.1":             "poolside",
	}
	for _, candidate := range batch.Candidates {
		wantEffort := "screening_default"
		if candidate.Model == "openrouter/openai/gpt-5.6-luna-pro" {
			wantEffort = "medium"
		}
		if wantModels[candidate.Model] != candidate.ServingProvider || candidate.ReasoningEffort != wantEffort || candidate.ProviderFallbacks == nil || *candidate.ProviderFallbacks {
			t.Fatalf("candidate route or effort drifted: %+v", candidate)
		}
		delete(wantModels, candidate.Model)
	}
	if len(wantModels) != 0 {
		t.Fatalf("missing v21 candidates: %+v", wantModels)
	}
	prior, err := NineteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	for index := range batch.Cases {
		if batch.Cases[index] != prior.Cases[index] {
			t.Fatalf("frozen case %d drifted", index)
		}
	}
	for _, required := range []string{"continue_unaffected_candidate_after_candidate_specific_failure", "preserve_non_safety_harness_imperfection_as_observation_local_invalid", "one_observation_per_candidate_case_no_retries", "do_not_escalate_effort_inside_this_batch", "new_model_discovery_only_no_role_promotion"} {
		if !contains(batch.StopRules, required) {
			t.Fatalf("missing v21 discovery rule %s", required)
		}
	}
}

func TestTwentiethOpenRouterBatchRepeatsV19LunaConfiguration(t *testing.T) {
	batch, err := TwentiethOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "planned_not_launched" || len(batch.Candidates) != 1 || len(batch.Cases) != 5 || batch.Repeats != 3 {
		t.Fatalf("unexpected Luna reliability shape: %+v", batch)
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 15 || projection.ReservedTokens != 150000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected Luna reliability projection: %+v", projection)
	}
	if batch.APIBudgets[0].MaxSpendMicros != 110605537 || batch.APIBudgets[0].PerRequestReserveMicros != 25000 {
		t.Fatalf("unexpected Luna reliability budget: %+v", batch.APIBudgets)
	}
	prior, err := NineteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	for index := range batch.Cases {
		if batch.Cases[index] != prior.Cases[index] {
			t.Fatalf("frozen case %d drifted", index)
		}
	}
	got, want := batch.Candidates[0], prior.Candidates[0]
	got.ID, want.ID = "", ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Luna reliability configuration drifted: %+v != %+v", got, want)
	}
	for _, required := range []string{"preserve_non_safety_harness_imperfection_as_observation_local_invalid", "one_observation_per_candidate_case_repeat_no_retries", "three_new_full_suite_repeats", "reliability_repeat_only_no_role_promotion"} {
		if !contains(batch.StopRules, required) {
			t.Fatalf("missing Luna reliability rule %s", required)
		}
	}
}

func TestNineteenthOpenRouterBatchIsCurrentLightweightDiscovery(t *testing.T) {
	batch, err := NineteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "planned_not_launched" || len(batch.Candidates) != 2 || len(batch.Cases) != 5 || batch.Repeats != 1 {
		t.Fatalf("unexpected current discovery shape: %+v", batch)
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 10 || projection.ReservedTokens != 100000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected current discovery projection: %+v", projection)
	}
	wantModels := map[string]string{"openrouter/openai/gpt-5.6-luna": "openai", "openrouter/google/gemini-3.8-flash": "google-ai-studio"}
	for _, candidate := range batch.Candidates {
		wantEffort := "screening_default"
		if candidate.Model == "openrouter/openai/gpt-5.6-luna" {
			wantEffort = "medium"
		}
		if wantModels[candidate.Model] != candidate.ServingProvider || candidate.ReasoningEffort != wantEffort || candidate.ProviderFallbacks == nil || *candidate.ProviderFallbacks {
			t.Fatalf("candidate route or effort drifted: %+v", candidate)
		}
		delete(wantModels, candidate.Model)
	}
	if len(wantModels) != 0 {
		t.Fatalf("missing current lightweight candidates: %+v", wantModels)
	}
	for _, required := range []string{"continue_unaffected_candidate_after_candidate_specific_failure", "preserve_non_safety_harness_imperfection_as_observation_local_invalid", "one_observation_per_candidate_case_no_retries", "static_screen_only_no_role_promotion"} {
		if !contains(batch.StopRules, required) {
			t.Fatalf("missing current discovery rule %s", required)
		}
	}
	prior, err := SeventeenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	for index := range batch.Cases {
		if batch.Cases[index] != prior.Cases[index] {
			t.Fatalf("frozen case %d drifted", index)
		}
	}
}

func TestEighteenthOpenRouterBatchClosesInvalidV6CoverageWithoutRoleSelection(t *testing.T) {
	batch, err := EighteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "planned_not_launched" || len(batch.Candidates) != 2 || len(batch.Cases) != 6 || batch.Cases[0].CaseID != "T12" || batch.Cases[0].CaseVersion != "v4" {
		t.Fatalf("unexpected coverage-closure shape: %+v", batch)
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 12 || projection.ReservedTokens != 120000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected coverage-closure projection: %+v", projection)
	}
	if batch.APIBudgets[0].MaxSpendMicros != 106500793 || batch.APIBudgets[0].PerRequestReserveMicros != 10000 {
		t.Fatalf("unexpected capped budget: %+v", batch.APIBudgets)
	}
	wantModels := map[string]string{"openrouter/deepseek/deepseek-v4-flash": "deepinfra", "openrouter/qwen/qwen3.7-flash": "alibaba"}
	for _, candidate := range batch.Candidates {
		if wantModels[candidate.Model] != candidate.ServingProvider || candidate.ReasoningEffort != "high" || candidate.ProviderFallbacks == nil || *candidate.ProviderFallbacks {
			t.Fatalf("candidate route or effort drifted: %+v", candidate)
		}
		delete(wantModels, candidate.Model)
	}
	if len(wantModels) != 0 {
		t.Fatalf("missing coverage candidates: %+v", wantModels)
	}
	for _, required := range []string{"t12_v4_gate_before_frozen_capability_group", "continue_unaffected_candidate_after_candidate_specific_failure", "one_observation_per_candidate_case_no_retries", "coverage_closure_only_no_role_promotion"} {
		if !contains(batch.StopRules, required) {
			t.Fatalf("missing coverage-closure rule %s", required)
		}
	}
}

func TestSeventeenthOpenRouterBatchIsFinalBoundedLightweightDiscovery(t *testing.T) {
	batch, err := SeventeenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "planned_not_launched" || len(batch.Candidates) != 8 || len(batch.Cases) != 5 || batch.Repeats != 1 {
		t.Fatalf("unexpected discovery shape: %+v", batch)
	}
	if batch.APIBudgets[0].MaxSpendMicros != 103175689 || batch.APIBudgets[0].PerRequestReserveMicros != 10000 {
		t.Fatalf("unexpected sub-dollar budget: %+v", batch.APIBudgets)
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 40 || projection.ReservedTokens != 400000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected discovery projection: %+v", projection)
	}
	seenModels := map[string]bool{}
	for _, candidate := range batch.Candidates {
		if seenModels[candidate.Model] || candidate.ReasoningEffort != "screening_default" || candidate.ServingProvider == "" || candidate.ProviderFallbacks == nil || *candidate.ProviderFallbacks {
			t.Fatalf("candidate is not a unique pinned default-effort route: %+v", candidate)
		}
		seenModels[candidate.Model] = true
	}
	for _, excluded := range []string{"openrouter/inclusionai/ling-3.0-flash", "openrouter/z-ai/glm-5.3-flash", "openrouter/qwen/qwen3-coder-flash", "openrouter/minimax/minimax-m2.7", "openrouter/xiaomi/mimo-v2.5", "openrouter/anthropic/claude-sonnet-5"} {
		if seenModels[excluded] {
			t.Fatalf("previously screened model was repeated: %s", excluded)
		}
	}
	for _, required := range []string{"final_lightweight_discovery_cohort_under_one_usd", "repeat_only_perfect_survivors_in_separate_frozen_batch", "stop_static_screening_after_this_cohort_and_any_survivor_repeat", "static_screen_only_no_role_promotion"} {
		if !contains(batch.StopRules, required) {
			t.Fatalf("missing final discovery rule %s", required)
		}
	}
	prior, err := SixteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	for index := range batch.Cases {
		if batch.Cases[index] != prior.Cases[index] {
			t.Fatalf("frozen case %d drifted", index)
		}
	}
}

func TestSixteenthOpenRouterBatchIsBoundedStrongAnchor(t *testing.T) {
	batch, err := SixteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Candidates) != 1 || batch.Candidates[0].Model != "openrouter/anthropic/claude-sonnet-5" || batch.Candidates[0].ServingProvider != "anthropic" || batch.Candidates[0].ReasoningEffort != "medium" || batch.Candidates[0].ProviderFallbacks == nil || *batch.Candidates[0].ProviderFallbacks {
		t.Fatalf("unexpected strong anchor: %+v", batch.Candidates)
	}
	if !contains(batch.StopRules, "strong_anchor_only_after_no_cheap_survivor") || !contains(batch.StopRules, "comparison_anchor_only_no_role_promotion") {
		t.Fatalf("strong-anchor boundary missing: %+v", batch.StopRules)
	}
}

func TestFifteenthOpenRouterBatchChangesOnlyGLMEffort(t *testing.T) {
	batch, err := FifteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	prior, err := FourteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Cases) != len(prior.Cases) || len(batch.Candidates) != 1 {
		t.Fatalf("effort comparison shape drifted: %+v", batch)
	}
	for index := range batch.Cases {
		if batch.Cases[index] != prior.Cases[index] {
			t.Fatalf("effort comparison case %d drifted", index)
		}
	}
	got, want := batch.Candidates[0], prior.Candidates[0]
	if got.ReasoningEffort != "medium" || want.ReasoningEffort != "low" {
		t.Fatalf("unexpected effort pair: %s %s", want.ReasoningEffort, got.ReasoningEffort)
	}
	got.ID, want.ID, got.ReasoningEffort, want.ReasoningEffort = "", "", "", ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effort comparison changed more than effort: %+v != %+v", got, want)
	}
}

func TestFourteenthOpenRouterBatchIsAnExactGLMReliabilityRepeat(t *testing.T) {
	batch, err := FourteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	prior, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Candidates) != 1 || batch.Candidates[0].Model != "openrouter/z-ai/glm-5.3-flash" || batch.Candidates[0].ReasoningEffort != "low" || batch.Candidates[0].ServingProvider != "z-ai" || batch.Candidates[0].ProviderFallbacks == nil || *batch.Candidates[0].ProviderFallbacks {
		t.Fatalf("unexpected reliability candidate: %+v", batch.Candidates)
	}
	if len(batch.Cases) != len(prior.Cases) {
		t.Fatalf("reliability case count drifted: %+v", batch.Cases)
	}
	for index := range batch.Cases {
		if batch.Cases[index] != prior.Cases[index] {
			t.Fatalf("reliability case %d drifted: %+v != %+v", index, batch.Cases[index], prior.Cases[index])
		}
	}
	want := prior.Candidates[1]
	got := batch.Candidates[0]
	want.ID, got.ID = "", ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GLM configuration drifted between capability and reliability batches: %+v != %+v", got, want)
	}
	if projection := ProjectScreeningBatch(batch); projection.Observations != 5 || projection.ReservedTokens != 50000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("unexpected reliability projection: %+v", projection)
	}
}

func TestTwelfthOpenRouterBatchRemainsUnlaunchedHistoricalPlan(t *testing.T) {
	batch, err := TwelfthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-owner-anchoring-t11-minimax-recovery-openrouter-v12" || len(batch.Candidates) != 1 || len(batch.Cases) != 1 {
		t.Fatalf("twelfth plan drifted: %+v", batch)
	}
}

func TestEleventhOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := EleventhOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-owner-anchoring-t11-corrected-openrouter-v11" || len(batch.Candidates) != 5 || batch.Cases[0].CaseVersion != "v4" || batch.APIBudgets[0].MaxSpendMicros != 102124669 {
		t.Fatalf("eleventh historical batch drifted: %+v", batch)
	}
}

func TestTenthOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := TenthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-owner-anchoring-t11-strong-anchor-openrouter-v10" || len(batch.Candidates) != 1 || batch.Cases[0].CaseVersion != "v3" || batch.APIBudgets[0].MaxSpendMicros != 102135232 {
		t.Fatalf("tenth historical batch drifted: %+v", batch)
	}
}

func TestNinthOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := NinthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-owner-anchoring-t11-openrouter-v9" || batch.CandidateTransport != OpenCodeToolFreeTransportV6 || len(batch.Candidates) != 5 || batch.Cases[0].CaseVersion != "v3" || batch.APIBudgets[0].MaxSpendMicros != 102101994 {
		t.Fatalf("ninth historical batch drifted: %+v", batch)
	}
}

func TestEighthOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := EighthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-consistency-openrouter-v8" || batch.CandidateTransport != OpenCodeToolFreeTransportV6 || len(batch.Candidates) != 5 || batch.Cases[0].CaseVersion != "v4" || batch.APIBudgets[0].MaxSpendMicros != 102099156 {
		t.Fatalf("eighth historical batch drifted: %+v", batch)
	}
}

func TestSeventhOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := SeventhOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-openrouter-v7" || batch.CandidateTransport != OpenCodeToolFreeTransportV6 || len(batch.Candidates) != 6 || batch.Cases[0].CaseVersion != "v4" || batch.APIBudgets[0].MaxSpendMicros != 102095659 {
		t.Fatalf("seventh historical batch drifted: %+v", batch)
	}
}

func TestSixthOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := SixthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-openrouter-v6" || batch.CandidateTransport != OpenCodeToolFreeTransportV5 || batch.Cases[0].CaseVersion != "v3" || batch.PerLaunchTokenReservation != 24000 {
		t.Fatalf("sixth historical batch drifted: %+v", batch)
	}
}

func TestFifthOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := FifthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-openrouter-v5" || batch.CandidateTransport != OpenCodeToolFreeTransportV5 || batch.Cases[0].CaseVersion != "v2" || batch.PerLaunchTokenReservation != 24000 {
		t.Fatalf("fifth historical batch drifted: %+v", batch)
	}
}

func TestFourthOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := FourthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-openrouter-v4" || batch.CandidateTransport != OpenCodeToolFreeTransportV4 || batch.PerLaunchTokenReservation != 12000 {
		t.Fatalf("fourth historical batch drifted: %+v", batch)
	}
}

func TestThirdOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := ThirdOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-openrouter-v3" || batch.CandidateTransport != OpenCodeToolFreeTransportV3 || batch.APIBudgets[0].MaxSpendMicros != 102123942 {
		t.Fatalf("third historical batch drifted: %+v", batch)
	}
}

func TestSecondOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := SecondOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-openrouter-v2" || batch.CandidateTransport != OpenCodeToolFreeTransportV2 || batch.APIBudgets[0].MaxSpendMicros != 102123860 {
		t.Fatalf("second historical batch drifted: %+v", batch)
	}
}

func TestFirstOpenRouterBatchRemainsImmutableHistoricalEvidence(t *testing.T) {
	batch, err := FirstOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if batch.BatchID != "screen-blast-radius-t12-openrouter-v1" || batch.CandidateTransport != OpenCodeToolFreeTransport || batch.APIBudgets[0].MaxSpendMicros != 102121724 {
		t.Fatalf("historical batch drifted: %+v", batch)
	}
}

func TestTwentyThirdOpenRouterBatchPinsEightNewCurrentLightweightConfigurations(t *testing.T) {
	batch, err := TwentyThirdOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"openrouter/moonshotai/kimi-k2.7-code":    "moonshotai",
		"openrouter/qwen/qwen3-coder-next":        "alibaba",
		"openrouter/kwaipilot/kat-coder-pro-v2.5": "atlas-cloud",
		"openrouter/mistralai/devstral-2512":      "mistral",
		"openrouter/stepfun/step-3.7-flash":       "stepfun",
		"openrouter/xiaomi/mimo-v2.5-pro":         "xiaomi",
		"openrouter/google/gemini-3.5-flash-lite": "google-ai-studio",
		"openrouter/openai/gpt-5.4-nano":          "openai",
	}
	if batch.BatchID != "screen-static-capability-current-lightweight-openrouter-v23" || len(batch.Candidates) != len(want) || len(batch.Cases) != 5 || batch.Repeats != 1 || batch.TokenCeiling != 400000 || batch.PerLaunchTokenReservation != 10000 {
		t.Fatalf("unexpected v23 shape: %+v", batch)
	}
	seen := map[string]bool{}
	for _, candidate := range batch.Candidates {
		if seen[candidate.Model] || want[candidate.Model] != candidate.ServingProvider || candidate.ReasoningEffort != "screening_default" || candidate.ProviderFallbacks == nil || *candidate.ProviderFallbacks {
			t.Fatalf("v23 candidate drifted: %+v", candidate)
		}
		seen[candidate.Model] = true
	}
	if batch.APIBudgets[0].MaxSpendMicros != 111180870 || batch.MaxOwnerInterventions != 0 || batch.ProductionAllowed || batch.ExternalSideEffectsAllowed || !contains(batch.StopRules, "one_observation_per_candidate_case_no_retries") {
		t.Fatalf("v23 safety boundary drifted: %+v", batch)
	}
}
