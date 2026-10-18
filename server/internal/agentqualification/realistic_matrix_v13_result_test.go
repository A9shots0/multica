package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed realistic_matrix_v13_result.json
var realisticMatrixV13ResultJSON []byte

func TestRealisticMatrixV13ResultReconciles(t *testing.T) {
	var result struct {
		Status       string `json:"status"`
		Planned      int    `json:"planned_observations"`
		Attempted    int    `json:"attempted_observations"`
		Skipped      int    `json:"policy_skipped_observations"`
		Calls        int    `json:"provider_calls"`
		Retries      int    `json:"retries"`
		Intervention int    `json:"owner_interventions"`
		Input        int64  `json:"input_tokens"`
		Output       int64  `json:"output_tokens"`
		Reasoning    int64  `json:"reasoning_tokens"`
		Total        int64  `json:"total_tokens"`
		Results      []struct {
			Calls    int   `json:"calls"`
			Tokens   int64 `json:"tokens"`
			Advances bool  `json:"advances"`
		} `json:"results"`
	}
	if err := json.Unmarshal(realisticMatrixV13ResultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete_no_survivor" || result.Attempted+result.Skipped != result.Planned || result.Retries != 0 || result.Intervention != 0 {
		t.Fatalf("v13 assignment accounting drifted: %+v", result)
	}
	if result.Input+result.Output+result.Reasoning != result.Total {
		t.Fatalf("v13 token accounting drifted: %+v", result)
	}
	var calls int
	var tokens int64
	for _, candidate := range result.Results {
		calls += candidate.Calls
		tokens += candidate.Tokens
		if candidate.Advances {
			t.Fatal("v13 result unexpectedly promotes a candidate")
		}
	}
	if calls != result.Calls || tokens != result.Total {
		t.Fatalf("v13 candidate totals do not reconcile: calls=%d tokens=%d result=%+v", calls, tokens, result)
	}
}
