package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed screening_batch_openrouter_v22_result.json
var screeningBatchOpenRouterV22ResultJSON []byte

func TestTwentySecondOpenRouterResultReconciles(t *testing.T) {
	var result struct {
		Planned   int `json:"planned_observations"`
		Calls     int `json:"provider_calls"`
		Skips     int `json:"policy_skipped_assignments"`
		Retries   int `json:"retries"`
		Tokens    int `json:"provider_export_total_tokens"`
		Fresh     int `json:"fresh_input_tokens"`
		Cached    int `json:"cached_input_tokens"`
		Output    int `json:"output_tokens"`
		Reasoning int `json:"reasoning_tokens"`
	}
	if err := json.Unmarshal(screeningBatchOpenRouterV22ResultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Calls+result.Skips != result.Planned || result.Retries != 0 {
		t.Fatalf("assignment accounting drifted: %+v", result)
	}
	if result.Fresh+result.Cached+result.Output+result.Reasoning != result.Tokens {
		t.Fatalf("token accounting drifted: %+v", result)
	}
}
