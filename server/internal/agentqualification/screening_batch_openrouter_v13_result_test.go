package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed screening_batch_openrouter_v13_result.json
var screeningBatchOpenRouterV13ResultJSON []byte

func TestThirteenthOpenRouterResultAccountsForEveryFrozenAssignment(t *testing.T) {
	var result struct {
		BatchID, Status, TestValidity                                string
		PlannedObservations, ProviderCalls, PolicySkippedAssignments int
		Retries, ToolEvents, OwnerInterventions                      int
		ProviderTokens, NormalizedTokens                             int64
		Observations                                                 []struct {
			InputTokens     int64 `json:"input_tokens"`
			CachedTokens    int64 `json:"cached_input_tokens"`
			OutputTokens    int64 `json:"output_tokens"`
			ReasoningTokens int64 `json:"reasoning_tokens"`
			TotalTokens     int64 `json:"total_tokens"`
		}
		Results []struct {
			CandidateID                 string `json:"candidate_id"`
			AdvancesToReliabilityScreen bool   `json:"advances_to_reliability_screen"`
		}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(screeningBatchOpenRouterV13ResultJSON, &raw); err != nil {
		t.Fatal(err)
	}
	decode := func(key string, target any) {
		if err := json.Unmarshal(raw[key], target); err != nil {
			t.Fatalf("decode %s: %v", key, err)
		}
	}
	decode("batch_id", &result.BatchID)
	decode("status", &result.Status)
	decode("test_validity", &result.TestValidity)
	decode("planned_observations", &result.PlannedObservations)
	decode("provider_calls", &result.ProviderCalls)
	decode("policy_skipped_assignments", &result.PolicySkippedAssignments)
	decode("retries", &result.Retries)
	decode("tool_events", &result.ToolEvents)
	decode("owner_interventions", &result.OwnerInterventions)
	decode("provider_export_total_tokens", &result.ProviderTokens)
	decode("normalized_component_sum_tokens", &result.NormalizedTokens)
	if err := json.Unmarshal(raw["observations"], &result.Observations); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw["results"], &result.Results); err != nil {
		t.Fatal(err)
	}
	batch, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	if result.BatchID != batch.BatchID || result.Status != "complete_with_candidate_eliminations" || result.TestValidity != "valid" {
		t.Fatalf("result identity/status mismatch: %+v", result)
	}
	if result.ProviderCalls != len(result.Observations) || result.ProviderCalls+result.PolicySkippedAssignments != result.PlannedObservations || result.PlannedObservations != ProjectScreeningBatch(batch).Observations {
		t.Fatalf("assignment accounting mismatch: %+v", result)
	}
	if result.Retries != 0 || result.ToolEvents != 0 || result.OwnerInterventions != 0 {
		t.Fatalf("bounded execution violated: %+v", result)
	}
	var tokens int64
	for index, observation := range result.Observations {
		if observation.InputTokens+observation.CachedTokens+observation.OutputTokens+observation.ReasoningTokens != observation.TotalTokens {
			t.Fatalf("observation %d token components do not reconcile", index)
		}
		tokens += observation.TotalTokens
	}
	if tokens != result.ProviderTokens || tokens != result.NormalizedTokens {
		t.Fatalf("batch token totals do not reconcile: got %d provider %d normalized %d", tokens, result.ProviderTokens, result.NormalizedTokens)
	}
	advanced := 0
	for _, candidate := range result.Results {
		if candidate.AdvancesToReliabilityScreen {
			advanced++
			if candidate.CandidateID != "screen.generalist.glm53flash.low.openrouter.v13" {
				t.Fatalf("unexpected advancing candidate: %s", candidate.CandidateID)
			}
		}
	}
	if advanced != 1 {
		t.Fatalf("expected one reliability-screen candidate, got %d", advanced)
	}
}
