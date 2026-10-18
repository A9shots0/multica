package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed screening_batch_openrouter_v25_result.json
var screeningBatchOpenRouterV25ResultJSON []byte

func TestTwentyFifthOpenRouterResultReconciles(t *testing.T) {
	var r struct {
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
			Advances bool `json:"advances_to_reliability_screen"`
		} `json:"results"`
	}
	if err := json.Unmarshal(screeningBatchOpenRouterV25ResultJSON, &r); err != nil {
		t.Fatal(err)
	}
	if r.Receipted != r.Sealed || r.Sealed+r.Skips != r.Planned || r.Retries != 0 {
		t.Fatalf("assignment accounting drift: %+v", r)
	}
	if r.Fresh+r.Cached+r.Output+r.Reasoning != r.Total {
		t.Fatalf("token accounting drift: %+v", r)
	}
	advanced := 0
	for _, result := range r.Results {
		if result.Advances {
			advanced++
		}
	}
	if advanced != 3 {
		t.Fatalf("unexpected survivor count %d", advanced)
	}
}
