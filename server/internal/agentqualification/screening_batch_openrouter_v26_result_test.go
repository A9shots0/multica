package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed screening_batch_openrouter_v26_result.json
var screeningBatchOpenRouterV26ResultJSON []byte

func TestTwentySixthOpenRouterResultReconciles(t *testing.T) {
	var r struct {
		Planned, Receipted, Sealed, Skips, Retries int
		Fresh, Cached, Output, Reasoning, Total    int
		Results                                    []struct {
			Advances bool `json:"advances_to_capability_screen"`
		} `json:"results"`
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(screeningBatchOpenRouterV26ResultJSON, &raw); err != nil {
		t.Fatal(err)
	}
	decode := func(key string, target any) {
		if err := json.Unmarshal(raw[key], target); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	decode("planned_observations", &r.Planned)
	decode("receipted_provider_calls", &r.Receipted)
	decode("sealed_observations", &r.Sealed)
	decode("policy_skipped_assignments", &r.Skips)
	decode("retries", &r.Retries)
	decode("fresh_input_tokens", &r.Fresh)
	decode("cached_input_tokens", &r.Cached)
	decode("output_tokens", &r.Output)
	decode("reasoning_tokens", &r.Reasoning)
	decode("provider_export_total_tokens", &r.Total)
	decode("results", &r.Results)
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
	if advanced != 4 {
		t.Fatalf("unexpected survivor count %d", advanced)
	}
}
