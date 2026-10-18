package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed screening_batch_openrouter_v14_result.json
var screeningBatchOpenRouterV14ResultJSON []byte

//go:embed screening_batch_openrouter_v15_result.json
var screeningBatchOpenRouterV15ResultJSON []byte

//go:embed screening_batch_openrouter_v16_result.json
var screeningBatchOpenRouterV16ResultJSON []byte

//go:embed screening_batch_openrouter_v17_result.json
var screeningBatchOpenRouterV17ResultJSON []byte

//go:embed screening_batch_openrouter_v19_result.json
var screeningBatchOpenRouterV19ResultJSON []byte

//go:embed screening_batch_openrouter_v20_result.json
var screeningBatchOpenRouterV20ResultJSON []byte

func TestRecentScreeningResultsReconcileCallsSkipsAndTokens(t *testing.T) {
	for name, raw := range map[string][]byte{"v14": screeningBatchOpenRouterV14ResultJSON, "v15": screeningBatchOpenRouterV15ResultJSON, "v16": screeningBatchOpenRouterV16ResultJSON, "v17": screeningBatchOpenRouterV17ResultJSON, "v19": screeningBatchOpenRouterV19ResultJSON, "v20": screeningBatchOpenRouterV20ResultJSON} {
		var result struct {
			Planned          int   `json:"planned_observations"`
			Calls            int   `json:"provider_calls"`
			Skips            int   `json:"policy_skipped_assignments"`
			Retries          int   `json:"retries"`
			ProviderTokens   int64 `json:"provider_export_total_tokens"`
			NormalizedTokens int64 `json:"normalized_component_sum_tokens"`
			Observations     []struct {
				Input     int64 `json:"input_tokens"`
				Cached    int64 `json:"cached_input_tokens"`
				Output    int64 `json:"output_tokens"`
				Reasoning int64 `json:"reasoning_tokens"`
				Total     int64 `json:"total_tokens"`
			} `json:"observations"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Calls != len(result.Observations) || result.Calls+result.Skips != result.Planned || result.Retries != 0 {
			t.Fatalf("%s assignment accounting: %+v", name, result)
		}
		var total int64
		for i, observation := range result.Observations {
			if observation.Input+observation.Cached+observation.Output+observation.Reasoning != observation.Total {
				t.Fatalf("%s observation %d token mismatch", name, i)
			}
			total += observation.Total
		}
		if total != result.ProviderTokens || total != result.NormalizedTokens {
			t.Fatalf("%s total token mismatch: %d %d %d", name, total, result.ProviderTokens, result.NormalizedTokens)
		}
	}
}
