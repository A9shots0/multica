package agentqualification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"sort"
)

type claudeJSONUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
}

type claudeJSONResult struct {
	Type             string                     `json:"type"`
	Subtype          string                     `json:"subtype"`
	IsError          bool                       `json:"is_error"`
	Result           string                     `json:"result"`
	StructuredOutput json.RawMessage            `json:"structured_output"`
	SessionID        string                     `json:"session_id"`
	TotalCostUSD     json.Number                `json:"total_cost_usd"`
	Usage            claudeJSONUsage            `json:"usage"`
	ModelUsage       map[string]json.RawMessage `json:"modelUsage"`
}

// ParseClaudeCodeJSONResult calibrates only Claude Code's one-result JSON
// envelope. It does not invoke Claude and refuses model aliases, missing usage,
// malformed structured output, multiple reported models, or imprecise cost.
func ParseClaudeCodeJSONResult(raw []byte, observedAt, completedAt string) (ProviderExecutionResult, error) {
	result, _, err := ParseClaudeCodeJSONArtifact(raw, observedAt, completedAt)
	return result, err
}

// ParseClaudeCodeJSONArtifact returns both the normalized receipt input and the
// exact structured candidate response from the same single terminal envelope.
func ParseClaudeCodeJSONArtifact(raw []byte, observedAt, completedAt string) (ProviderExecutionResult, json.RawMessage, error) {
	return parseClaudeCodeJSONArtifact(raw, observedAt, completedAt, true)
}

// ParseClaudeCodeSubscriptionJSONArtifact preserves the raw decimal in sealed
// evidence and rounds any sub-micro-dollar API-equivalent estimate upward for
// the normalized integer receipt. Subscription billing is not inferred from
// that informational CLI estimate.
func ParseClaudeCodeSubscriptionJSONArtifact(raw []byte, observedAt, completedAt string) (ProviderExecutionResult, json.RawMessage, error) {
	return parseClaudeCodeJSONArtifact(raw, observedAt, completedAt, false)
}

func parseClaudeCodeJSONArtifact(raw []byte, observedAt, completedAt string, requireExactMicroUSD bool) (ProviderExecutionResult, json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded claudeJSONResult
	if err := decoder.Decode(&decoded); err != nil {
		return ProviderExecutionResult{}, nil, fmt.Errorf("decode Claude JSON result: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ProviderExecutionResult{}, nil, fmt.Errorf("Claude JSON result has trailing data")
	}
	if decoded.Type != "result" || decoded.Subtype != "success" || decoded.IsError || decoded.SessionID == "" {
		return ProviderExecutionResult{}, nil, fmt.Errorf("Claude result is not a successful terminal receipt")
	}
	models := make([]string, 0, len(decoded.ModelUsage))
	for model := range decoded.ModelUsage {
		if model != "" {
			models = append(models, model)
		}
	}
	sort.Strings(models)
	if len(models) != 1 {
		return ProviderExecutionResult{}, nil, fmt.Errorf("Claude receipt must report exactly one model")
	}
	if decoded.Usage.InputTokens < 0 || decoded.Usage.CacheCreationInputTokens < 0 || decoded.Usage.CacheReadInputTokens < 0 || decoded.Usage.OutputTokens <= 0 {
		return ProviderExecutionResult{}, nil, fmt.Errorf("Claude usage is incomplete or invalid")
	}
	output := decoded.StructuredOutput
	if len(output) == 0 || string(output) == "null" {
		output = json.RawMessage(decoded.Result)
	}
	if !json.Valid(output) {
		return ProviderExecutionResult{}, nil, fmt.Errorf("Claude structured output is not valid JSON")
	}
	var costMicros int64
	var err error
	if requireExactMicroUSD {
		costMicros, err = exactDecimalUSDMicros(decoded.TotalCostUSD.String())
	} else {
		costMicros, err = decimalUSDMicros(decoded.TotalCostUSD.String())
	}
	if err != nil {
		return ProviderExecutionResult{}, nil, fmt.Errorf("Claude cost invalid: %w", err)
	}
	totalInput := decoded.Usage.InputTokens + decoded.Usage.CacheCreationInputTokens + decoded.Usage.CacheReadInputTokens
	cached := decoded.Usage.CacheReadInputTokens
	input, outputTokens := totalInput, decoded.Usage.OutputTokens
	result := ProviderExecutionResult{
		ObservationID: "claude:" + decoded.SessionID, ReportedModel: models[0], ReportedModelVersionStatus: "reported",
		ReportedModelVersion: models[0], ProviderResponseID: decoded.SessionID, StartedAt: observedAt, CompletedAt: completedAt,
		TerminalState: "completed", ObservedProviderEventTypes: []string{"result"}, RawTraceSHA256: sha256Hex(raw), OutputSHA256: sha256Hex(output),
		Usage: ExecutionReceiptUsage{InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &outputTokens, ReasoningTokensStatus: "provider_unavailable", ProviderReportedCostMicros: &costMicros},
	}
	return result, append(json.RawMessage(nil), output...), nil
}

func decimalUSDMicros(value string) (int64, error) {
	amount := new(big.Rat)
	if _, ok := amount.SetString(value); !ok || amount.Sign() < 0 {
		return 0, fmt.Errorf("invalid non-negative decimal")
	}
	amount.Mul(amount, big.NewRat(1_000_000, 1))
	quotient := new(big.Int).Quo(amount.Num(), amount.Denom())
	if new(big.Int).Mod(amount.Num(), amount.Denom()).Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("micro-USD overflow")
	}
	return quotient.Int64(), nil
}

func exactDecimalUSDMicros(value string) (int64, error) {
	amount := new(big.Rat)
	if _, ok := amount.SetString(value); !ok || amount.Sign() < 0 {
		return 0, fmt.Errorf("invalid non-negative decimal")
	}
	amount.Mul(amount, big.NewRat(1_000_000, 1))
	if new(big.Int).Mod(amount.Num(), amount.Denom()).Sign() != 0 {
		return 0, fmt.Errorf("cost precision exceeds micro-USD")
	}
	quotient := new(big.Int).Quo(amount.Num(), amount.Denom())
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("micro-USD overflow")
	}
	return quotient.Int64(), nil
}
