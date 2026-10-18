package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed realistic_matrix_v12_result.json
var realisticMatrixV12ResultJSON []byte

func TestRealisticMatrixV12ResultReconciles(t *testing.T) {
	var result struct {
		Status       string   `json:"status"`
		Planned      []string `json:"planned_cases"`
		Attempted    []string `json:"attempted_cases"`
		Skipped      []string `json:"policy_skipped_cases"`
		Calls        int      `json:"provider_calls"`
		Retries      int      `json:"retries"`
		Input        int64    `json:"input_tokens"`
		Output       int64    `json:"output_tokens"`
		Reasoning    int64    `json:"reasoning_tokens"`
		Total        int64    `json:"total_tokens"`
		Intervention int      `json:"owner_interventions"`
		Result       struct {
			Outcome  string `json:"outcome"`
			Advances bool   `json:"advances"`
		} `json:"result"`
	}
	if err := json.Unmarshal(realisticMatrixV12ResultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete_no_survivor" || len(result.Planned) != len(result.Attempted)+len(result.Skipped) || result.Calls != 8 || result.Retries != 0 || result.Intervention != 0 {
		t.Fatalf("v12 assignment accounting drifted: %+v", result)
	}
	if result.Input+result.Output+result.Reasoning != result.Total || result.Result.Outcome != "candidate_fail" || result.Result.Advances {
		t.Fatalf("v12 result accounting drifted: %+v", result)
	}
}
