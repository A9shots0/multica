package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed screening_batch_openrouter_v23_result.json
var screeningBatchOpenRouterV23ResultJSON []byte

func TestTwentyThirdOpenRouterResultReconciles(t *testing.T) {
	var result struct {
		Planned   int `json:"planned_observations"`
		Receipted int `json:"receipted_provider_calls"`
		Invalid   int `json:"invalid_provider_invocations"`
		Sealed    int `json:"sealed_observations"`
		Skips     int `json:"policy_skipped_assignments"`
		Retries   int `json:"retries"`
		Tokens    int `json:"provider_export_total_tokens"`
		Fresh     int `json:"fresh_input_tokens"`
		Cached    int `json:"cached_input_tokens"`
		Output    int `json:"output_tokens"`
		Reasoning int `json:"reasoning_tokens"`
	}
	if err := json.Unmarshal(screeningBatchOpenRouterV23ResultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Receipted+result.Invalid != result.Sealed || result.Sealed+result.Skips != result.Planned || result.Retries != 0 {
		t.Fatalf("assignment accounting drifted: %+v", result)
	}
	if result.Fresh+result.Cached+result.Output+result.Reasoning != result.Tokens {
		t.Fatalf("token accounting drifted: %+v", result)
	}
}
