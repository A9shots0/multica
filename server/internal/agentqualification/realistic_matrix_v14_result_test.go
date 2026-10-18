package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed realistic_matrix_v14_result.json
var realisticMatrixV14ResultJSON []byte

func TestRealisticMatrixV14ResultReconciles(t *testing.T) {
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
			Outcome                string `json:"outcome"`
			ReliabilityGapClosed   bool   `json:"reliability_gap_closed"`
			ProductionRoleSelected bool   `json:"production_role_selected"`
		} `json:"result"`
	}
	if err := json.Unmarshal(realisticMatrixV14ResultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete_reliability_gap_closed" || len(result.Planned) != len(result.Attempted)+len(result.Skipped) || result.Calls != 12 || result.Retries != 0 || result.Intervention != 0 {
		t.Fatalf("v14 assignment accounting drifted: %+v", result)
	}
	if result.Input+result.Output+result.Reasoning != result.Total || result.Result.Outcome != "pass" || !result.Result.ReliabilityGapClosed || result.Result.ProductionRoleSelected {
		t.Fatalf("v14 result accounting drifted: %+v", result)
	}
}
