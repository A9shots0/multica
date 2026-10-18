package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

//go:embed realistic_execute.go
var realisticExecutorSource []byte

//go:embed openrouter_execute.go
var realisticOpenCodeRuntimeSource []byte

// openCodeRealisticProvider is deliberately narrower than the static-screen
// executor. OpenCode may produce one JSON action, but only the deterministic
// realistic controller can execute it.
type openCodeRealisticProvider struct {
	candidate           agentqualification.RealisticCandidate
	baselineSpendMicros int64
	maxSpendMicros      int64
}

type realisticRawProviderReceipt struct {
	JSONL   string                           `json:"jsonl"`
	Export  json.RawMessage                  `json:"export"`
	Routing realisticOpenCodeRoutingEvidence `json:"routing"`
}

// realisticOpenCodeRoutingEvidence is harness-derived evidence, not a model
// claim. The resolved config is validated before inference and its exact hash
// is bound into the write-once raw provider receipt. OpenCode's export proves
// the direct provider is OpenRouter; the upstream provider remains explicitly
// unavailable unless OpenRouter metadata actually reports it.
type realisticOpenCodeRoutingEvidence struct {
	ResolvedConfigSHA256        string   `json:"resolved_config_sha256"`
	InlineConfigSHA256          string   `json:"inline_config_sha256"`
	DirectProvider              string   `json:"direct_provider"`
	DirectProviderEvidence      string   `json:"direct_provider_evidence"`
	RequestedUpstreamProvider   string   `json:"requested_upstream_provider"`
	ProviderOrder               []string `json:"provider_order"`
	AllowFallbacks              bool     `json:"allow_fallbacks"`
	ActualUpstreamProvider      string   `json:"actual_upstream_provider,omitempty"`
	ActualUpstreamProviderState string   `json:"actual_upstream_provider_state"`
}

func (provider openCodeRealisticProvider) Next(ctx context.Context, request agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
	spend, err := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	const reserveMicros = int64(250000)
	if provider.baselineSpendMicros < 0 || provider.maxSpendMicros <= 0 || spend.SpentMicros-provider.baselineSpendMicros+reserveMicros > provider.maxSpendMicros {
		return agentqualification.RealisticProviderResponse{}, agentqualification.ErrRealisticBudgetBreach
	}
	requiredExecutorHash, err := agentqualification.RequiredTopologyV6OpenCodeProviderSHA256()
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	actualExecutorHash := fmt.Sprintf("%x", sha256.Sum256(realisticExecutorSource))
	if actualExecutorHash != requiredExecutorHash {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("realistic provider executor is not the frozen reviewed source")
	}
	requiredRuntimeHash, err := agentqualification.RequiredTopologyV6OpenCodeRuntimeSHA256()
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(realisticOpenCodeRuntimeSource)) != requiredRuntimeHash {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("realistic OpenCode runtime is not the frozen reviewed source")
	}
	plan, err := agentqualification.BuildOpenCodeRealisticActionPlan(provider.candidate, request)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	runtimeState, err := newOpenCodeCommandRuntime(plan)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	resolvedConfig, err := runtimeState.resolvedConfig(ctx)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	routing, err := realisticOpenCodeRoutingEvidenceFor(plan, resolvedConfig)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	rawJSONL, rawExport, err := runtimeState.runOnce(plan)
	if err != nil {
		partial := realisticPartialProviderResponse(rawJSONL, rawExport, err)
		return partial, err
	}
	return translateRealisticOpenCodeArtifact(provider.candidate, routing, rawJSONL, rawExport)
}

func realisticOpenCodeRoutingEvidenceFor(plan agentqualification.OpenCodeScreeningPlan, resolvedConfig []byte) (realisticOpenCodeRoutingEvidence, error) {
	resolvedHash, err := agentqualification.ValidateOpenCodeResolvedConfig(plan, resolvedConfig)
	if err != nil {
		return realisticOpenCodeRoutingEvidence{}, err
	}
	inlineHash := sha256.Sum256(plan.InlineConfig)
	if fmt.Sprintf("%x", inlineHash) != plan.InlineConfigSHA256 || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks || strings.TrimSpace(plan.ServingProvider) == "" {
		return realisticOpenCodeRoutingEvidence{}, fmt.Errorf("OpenCode route plan is not hash-bound and no-fallback")
	}
	return realisticOpenCodeRoutingEvidence{
		ResolvedConfigSHA256: resolvedHash, InlineConfigSHA256: plan.InlineConfigSHA256,
		DirectProvider: "openrouter", DirectProviderEvidence: "opencode_export_provider_id",
		RequestedUpstreamProvider: plan.ServingProvider, ProviderOrder: []string{plan.ServingProvider},
		AllowFallbacks: false, ActualUpstreamProviderState: "unavailable_from_provider_metadata",
	}, nil
}

func translateRealisticOpenCodeArtifact(candidate agentqualification.RealisticCandidate, routing realisticOpenCodeRoutingEvidence, rawJSONL, rawExport []byte) (agentqualification.RealisticProviderResponse, error) {
	translated := realisticPartialProviderResponse(rawJSONL, rawExport, nil)
	result, candidateJSON, err := agentqualification.ParseOpenCodeRunEnvelope(rawJSONL, rawExport)
	if err != nil {
		return translated, err
	}
	if result.ReportedServingProvider != "" {
		routing.ActualUpstreamProvider = result.ReportedServingProvider
		routing.ActualUpstreamProviderState = "reported_by_openrouter_metadata"
	} else {
		routing.ActualUpstreamProvider = ""
		routing.ActualUpstreamProviderState = "unavailable_from_provider_metadata"
	}
	rawReceipt, err := json.Marshal(realisticRawProviderReceipt{JSONL: string(rawJSONL), Export: append(json.RawMessage(nil), rawExport...), Routing: routing})
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, err
	}
	translated.RawReceipt = rawReceipt
	rawReceiptHash := sha256.Sum256(rawReceipt)
	translated.RawReceiptSHA256 = fmt.Sprintf("%x", rawReceiptHash)
	if result.ReportedModel != candidate.Model {
		return translated, fmt.Errorf("%w: OpenCode reported model %q, expected %q", agentqualification.ErrRealisticEvidenceIntegrity, result.ReportedModel, candidate.Model)
	}
	if routing.DirectProvider != "openrouter" || routing.DirectProviderEvidence != "opencode_export_provider_id" || routing.RequestedUpstreamProvider != candidate.ServingProvider || routing.AllowFallbacks || len(routing.ProviderOrder) != 1 || routing.ProviderOrder[0] != candidate.ServingProvider || len(routing.ResolvedConfigSHA256) != sha256.Size*2 || len(routing.InlineConfigSHA256) != sha256.Size*2 {
		return translated, fmt.Errorf("%w: validated OpenCode route binding is incomplete", agentqualification.ErrRealisticEvidenceIntegrity)
	}
	if !strings.HasPrefix(result.ReportedModel, routing.DirectProvider+"/") {
		return translated, fmt.Errorf("%w: OpenCode direct provider does not match validated route", agentqualification.ErrRealisticEvidenceIntegrity)
	}
	if result.ReportedServingProvider != "" && !strings.EqualFold(result.ReportedServingProvider, candidate.ServingProvider) {
		return translated, fmt.Errorf("%w: OpenRouter serving provider %q does not match pinned route %q", agentqualification.ErrRealisticEvidenceIntegrity, result.ReportedServingProvider, candidate.ServingProvider)
	}
	var response struct {
		Action agentqualification.RealisticCandidateAction `json:"action"`
	}
	if !json.Valid(candidateJSON) {
		translated.ProtocolFailure = "candidate output is not valid JSON"
	} else {
		decoder := json.NewDecoder(strings.NewReader(string(candidateJSON)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&response); err != nil {
			translated.ProtocolFailure = "candidate action schema mismatch: " + err.Error()
		} else if err := decoder.Decode(&struct{}{}); err != io.EOF {
			translated.ProtocolFailure = "candidate action has trailing JSON"
		} else if response.Action.Kind == "" {
			translated.ProtocolFailure = "candidate action is missing"
		}
	}
	usage := result.Usage
	translated.Action = response.Action
	translated.InputTokens, translated.OutputTokens = valueOrZero(usage.InputTokens), valueOrZero(usage.OutputTokens)
	translated.ReasoningTokens, translated.CostMicros = valueOrZero(usage.ReasoningTokens), valueOrZero(usage.ProviderReportedCostMicros)
	translated.AccountingStatus = "exact"
	translated.TokenAccountingStatus = "exact"
	translated.CostAccountingStatus = "exact"
	return translated, nil
}

func realisticPartialProviderResponse(rawJSONL, rawExport []byte, transportErr error) agentqualification.RealisticProviderResponse {
	receipt := map[string]any{"jsonl": string(rawJSONL), "export_raw": string(rawExport)}
	if json.Valid(rawExport) {
		receipt["export"] = json.RawMessage(rawExport)
	}
	if transportErr != nil {
		receipt["transport_error"] = transportErr.Error()
	}
	rawReceipt, _ := json.Marshal(receipt)
	rawReceiptHash := sha256.Sum256(rawReceipt)
	response := agentqualification.RealisticProviderResponse{RawReceipt: rawReceipt, RawReceiptSHA256: fmt.Sprintf("%x", rawReceiptHash), AccountingStatus: "ambiguous", TokenAccountingStatus: "unknown", CostAccountingStatus: "unknown"}
	scanner := bufio.NewScanner(bytes.NewReader(rawJSONL))
	for scanner.Scan() {
		var event struct {
			Type string `json:"type"`
			Part struct {
				Cost   json.Number `json:"cost"`
				Tokens struct {
					Input     int64 `json:"input"`
					Output    int64 `json:"output"`
					Reasoning int64 `json:"reasoning"`
					Cache     struct {
						Read  int64 `json:"read"`
						Write int64 `json:"write"`
					} `json:"cache"`
				} `json:"tokens"`
			} `json:"part"`
		}
		if json.Unmarshal(bytes.TrimSpace(scanner.Bytes()), &event) != nil || event.Type != "step_finish" {
			continue
		}
		if event.Part.Tokens.Input < 0 || event.Part.Tokens.Output < 0 || event.Part.Tokens.Reasoning < 0 || event.Part.Tokens.Cache.Read < 0 || event.Part.Tokens.Cache.Write < 0 {
			continue
		}
		response.InputTokens = event.Part.Tokens.Input + event.Part.Tokens.Cache.Read + event.Part.Tokens.Cache.Write
		response.OutputTokens = event.Part.Tokens.Output
		response.ReasoningTokens = event.Part.Tokens.Reasoning
		response.AccountingStatus = "partial"
		response.TokenAccountingStatus = "reported"
		if costMicros, ok := realisticPartialCostMicros(event.Part.Cost.String()); ok {
			response.CostMicros = costMicros
			response.CostAccountingStatus = "reported"
		}
		break
	}
	return response
}

func trustworthyTopologyReviewPartialAccounting(rawJSONL []byte, response agentqualification.RealisticProviderResponse) bool {
	if response.TokenAccountingStatus != "reported" || response.CostAccountingStatus != "reported" {
		return false
	}
	finishCount := 0
	scanner := bufio.NewScanner(bytes.NewReader(rawJSONL))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event struct {
			Type string `json:"type"`
			Part struct {
				Cost   json.Number `json:"cost"`
				Tokens struct {
					Input     int64 `json:"input"`
					Output    int64 `json:"output"`
					Reasoning int64 `json:"reasoning"`
					Cache     struct {
						Read  int64 `json:"read"`
						Write int64 `json:"write"`
					} `json:"cache"`
				} `json:"tokens"`
			} `json:"part"`
		}
		if json.Unmarshal(line, &event) != nil {
			return false
		}
		if event.Type != "step_finish" {
			continue
		}
		finishCount++
		if event.Part.Tokens.Input < 0 || event.Part.Tokens.Output < 0 || event.Part.Tokens.Reasoning < 0 || event.Part.Tokens.Cache.Read < 0 || event.Part.Tokens.Cache.Write < 0 {
			return false
		}
		if _, ok := realisticPartialCostMicros(event.Part.Cost.String()); !ok {
			return false
		}
	}
	return scanner.Err() == nil && finishCount == 1
}

func realisticPartialCostMicros(value string) (int64, bool) {
	amount := new(big.Rat)
	if _, ok := amount.SetString(value); !ok || amount.Sign() < 0 {
		return 0, false
	}
	amount.Mul(amount, big.NewRat(1_000_000, 1))
	quotient := new(big.Int).Quo(amount.Num(), amount.Denom())
	if new(big.Int).Mod(amount.Num(), amount.Denom()).Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, false
	}
	return quotient.Int64(), true
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
