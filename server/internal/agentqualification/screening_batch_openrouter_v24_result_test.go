package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed screening_batch_openrouter_v24_result.json
var screeningBatchOpenRouterV24ResultJSON []byte

func TestTwentyFourthOpenRouterResultReconciles(t *testing.T) {
	var result struct {
		Planned, Receipted, Sealed, Skips, Retries int
		Fresh, Cached, Output, Reasoning, Total    int
		Results                                    []struct {
			Result string `json:"result"`
		} `json:"results"`
	}
	var raw struct {
		Planned   int `json:"planned_observations"`
		Receipted int `json:"receipted_provider_calls"`
		Sealed    int `json:"sealed_observations"`
		Skips     int `json:"policy_skipped_assignments"`
		Retries   int `json:"retries"`
		Fresh     int `json:"fresh_input_tokens"`
		Cached    int `json:"cached_input_tokens"`
		Output    int `json:"output_tokens"`
		Reasoning int `json:"reasoning_tokens"`
		Total     int `json:"provider_export_total_tokens"`
		Results   []struct {
			Result string `json:"result"`
		} `json:"results"`
	}
	if err := json.Unmarshal(screeningBatchOpenRouterV24ResultJSON, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Receipted != raw.Sealed || raw.Sealed+raw.Skips != raw.Planned || raw.Retries != 0 {
		t.Fatalf("assignment accounting drifted: %+v", raw)
	}
	if raw.Fresh+raw.Cached+raw.Output+raw.Reasoning != raw.Total {
		t.Fatalf("token accounting drifted: %+v", raw)
	}
	if len(raw.Results) != 2 || raw.Results[0].Result != "PASS" || raw.Results[1].Result != "PASS" {
		t.Fatalf("unexpected outcomes: %+v", raw.Results)
	}
	_ = result
}
