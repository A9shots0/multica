package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type protocolV32SpendSnapshot struct {
	Provider    string `json:"provider"`
	Source      string `json:"source"`
	ObservedAt  string `json:"observed_at"`
	SpentMicros int64  `json:"spent_micros"`
}

type protocolV32ProviderFactory func(agentqualification.ProtocolAblationV32Binding, string) agentqualification.ProtocolAblationV32Provider

func executeProtocolAblationV32(ctx context.Context, artifactRoot string, baseline, current protocolV32SpendSnapshot, factory protocolV32ProviderFactory) (agentqualification.ProtocolAblationV32Ledger, error) {
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	if err := agentqualification.RequireProtocolAblationV32ReviewedFreeze(); err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	if err := verifyProtocolV32CommandFreeze(); err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	if baseline.Provider != "openrouter" || current.Provider != "openrouter" || baseline.Source != "authoritative_api_usage" || current.Source != "authoritative_api_usage" || baseline.SpentMicros < 0 || current.SpentMicros < baseline.SpentMicros || current.SpentMicros-baseline.SpentMicros > d.MaxCombinedCostMicros {
		return agentqualification.ProtocolAblationV32Ledger{}, errors.New("v32 authoritative spend preflight failed")
	}
	ledgerPath := filepath.Join(artifactRoot, "ledger.json")
	ledger, err := loadProtocolV32Ledger(ledgerPath)
	if err != nil {
		return ledger, err
	}
	order, err := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	if err != nil {
		return ledger, err
	}
	for _, binding := range order {
		if existing, ok := ledger.Claims[binding.BindingID]; ok {
			decision, _, err := agentqualification.BeginProtocolAblationV32Observation(d, &ledger, binding)
			if err != nil || decision == "halt" {
				return ledger, err
			}
			if decision == "reuse" && existing.Status == "completed" {
				continue
			}
		}
		workspaceParent := filepath.Join(artifactRoot, "workspaces", safeProtocolV32Name(binding.BindingID))
		if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
			return ledger, fmt.Errorf("prepare v32 disposable workspace parent: %w", err)
		}
		workspace, err := agentqualification.PrepareRealisticNextR05V1(workspaceParent)
		if err != nil {
			return ledger, err
		}
		decision, _, err := agentqualification.BeginProtocolAblationV32Observation(d, &ledger, binding)
		if err != nil || decision == "halt" {
			return ledger, err
		}
		if decision == "reuse" {
			continue
		}
		claimPath := filepath.Join(artifactRoot, "claims", safeProtocolV32Name(binding.BindingID)+".claim")
		if err := createProtocolV32ExclusiveClaim(claimPath, binding.BindingID); err != nil {
			ledger.GlobalHalt, ledger.HaltReason = true, "ambiguous_transport"
			_ = writeProtocolV32Ledger(ledgerPath, ledger)
			return ledger, err
		}
		if err := writeProtocolV32Ledger(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		observation, err := agentqualification.RunProtocolAblationV32Observation(ctx, d, binding, workspace, factory(binding, workspace.Root))
		if err != nil {
			return ledger, err
		}
		if err := agentqualification.CompleteProtocolAblationV32Observation(d, &ledger, binding, observation); err != nil {
			return ledger, err
		}
		if err := sealProtocolV32RawObservation(artifactRoot, observation); err != nil {
			return ledger, err
		}
		if err := writeProtocolV32Ledger(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		if ledger.GlobalHalt {
			return ledger, nil
		}
	}
	return ledger, nil
}

func createProtocolV32ExclusiveClaim(path, bindingID string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("v32 exclusive claim already exists or cannot be created: %w", err)
	}
	_, writeErr := file.WriteString(bindingID + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func executeProtocolAblationV32Live(ctx context.Context, artifactRoot string) (agentqualification.ProtocolAblationV32Ledger, error) {
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	if err := agentqualification.RequireProtocolAblationV32ReviewedFreeze(); err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	if err := verifyProtocolV32CommandFreeze(); err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	baselinePath := filepath.Join(artifactRoot, "baseline-spend.json")
	var baseline protocolV32SpendSnapshot
	if raw, readErr := os.ReadFile(baselinePath); readErr == nil {
		if err := json.Unmarshal(raw, &baseline); err != nil {
			return agentqualification.ProtocolAblationV32Ledger{}, err
		}
	} else if errors.Is(readErr, os.ErrNotExist) {
		baseline, err = readProtocolV32OpenRouterSpend(ctx)
		if err != nil {
			return agentqualification.ProtocolAblationV32Ledger{}, err
		}
		if err := os.MkdirAll(artifactRoot, 0o700); err != nil {
			return agentqualification.ProtocolAblationV32Ledger{}, err
		}
		raw, _ := json.MarshalIndent(baseline, "", "  ")
		if err := os.WriteFile(baselinePath, raw, 0o600); err != nil {
			return agentqualification.ProtocolAblationV32Ledger{}, err
		}
	} else {
		return agentqualification.ProtocolAblationV32Ledger{}, readErr
	}
	current, err := readProtocolV32OpenRouterSpend(ctx)
	if err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	return executeProtocolAblationV32(ctx, artifactRoot, baseline, current, func(binding agentqualification.ProtocolAblationV32Binding, workspaceRoot string) agentqualification.ProtocolAblationV32Provider {
		return &protocolV32OpenCodeProvider{binding: binding, workspaceRoot: workspaceRoot, artifactRoot: artifactRoot, baselineSpendMicros: baseline.SpentMicros, maxSpendMicros: d.MaxCombinedCostMicros, reservedCostMicros: agentqualification.ProtocolAblationV32CostReservation(d, binding.Candidate)}
	})
}

func readProtocolV32OpenRouterSpend(ctx context.Context) (protocolV32SpendSnapshot, error) {
	profile, err := os.UserHomeDir()
	if err != nil {
		return protocolV32SpendSnapshot{}, err
	}
	raw, err := os.ReadFile(filepath.Join(profile, ".local", "share", "opencode", "auth.json"))
	if err != nil {
		return protocolV32SpendSnapshot{}, err
	}
	var auth struct {
		OpenRouter struct{ Type, Key string } `json:"openrouter"`
	}
	if json.Unmarshal(raw, &auth) != nil || auth.OpenRouter.Type != "api" || strings.TrimSpace(auth.OpenRouter.Key) == "" {
		return protocolV32SpendSnapshot{}, errors.New("OpenRouter credential is unavailable")
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/credits", nil)
	request.Header.Set("Authorization", "Bearer "+auth.OpenRouter.Key)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return protocolV32SpendSnapshot{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode != http.StatusOK {
		return protocolV32SpendSnapshot{}, fmt.Errorf("OpenRouter credits preflight failed with status %d: %w", response.StatusCode, err)
	}
	var payload struct {
		Data struct {
			TotalUsage json.Number `json:"total_usage"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return protocolV32SpendSnapshot{}, err
	}
	rational, ok := new(big.Rat).SetString(payload.Data.TotalUsage.String())
	if !ok || rational.Sign() < 0 {
		return protocolV32SpendSnapshot{}, errors.New("OpenRouter spend was invalid")
	}
	rational.Mul(rational, big.NewRat(1_000_000, 1))
	micros := new(big.Int).Quo(rational.Num(), rational.Denom())
	if new(big.Int).Mod(rational.Num(), rational.Denom()).Sign() != 0 {
		micros.Add(micros, big.NewInt(1))
	}
	if !micros.IsInt64() {
		return protocolV32SpendSnapshot{}, errors.New("OpenRouter spend exceeded int64")
	}
	return protocolV32SpendSnapshot{Provider: "openrouter", Source: "authoritative_api_usage", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), SpentMicros: micros.Int64()}, nil
}

func loadProtocolV32Ledger(path string) (agentqualification.ProtocolAblationV32Ledger, error) {
	var ledger agentqualification.ProtocolAblationV32Ledger
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ledger, nil
	}
	if err != nil {
		return ledger, err
	}
	if err := json.Unmarshal(raw, &ledger); err != nil {
		return ledger, fmt.Errorf("decode v32 ledger: %w", err)
	}
	return ledger, nil
}

func writeProtocolV32Ledger(path string, ledger agentqualification.ProtocolAblationV32Ledger) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func sealProtocolV32RawObservation(root string, observation agentqualification.ProtocolAblationV32Observation) error {
	directory := filepath.Join(root, "observations", safeProtocolV32Name(observation.BindingID))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	for index, receipt := range observation.RawReceipts {
		path := filepath.Join(directory, fmt.Sprintf("provider-%02d.json", index+1))
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		if _, err := file.Write(receipt); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	raw, _ := json.MarshalIndent(observation, "", "  ")
	return os.WriteFile(filepath.Join(directory, "observation.json"), raw, 0o600)
}

func safeProtocolV32Name(value string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(value)
}

type protocolV32OpenCodeProvider struct {
	binding             agentqualification.ProtocolAblationV32Binding
	workspaceRoot       string
	baselineSpendMicros int64
	maxSpendMicros      int64
	reservedCostMicros  int64
	artifactRoot        string
	callIndex           int
}

func (p *protocolV32OpenCodeProvider) Call(ctx context.Context, plan agentqualification.OpenCodeScreeningPlan) (agentqualification.ProtocolAblationV32ProviderResponse, error) {
	var response agentqualification.ProtocolAblationV32ProviderResponse
	spend, err := readProtocolV32OpenRouterSpend(ctx)
	if err != nil {
		return response, fmt.Errorf("v32 per-call authoritative spend preflight failed: %w", err)
	}
	if spend.SpentMicros < p.baselineSpendMicros || spend.SpentMicros-p.baselineSpendMicros+p.reservedCostMicros > p.maxSpendMicros {
		return response, errors.New("v32 per-call authoritative spend preflight exceeded the campaign cap")
	}
	executable, err := exec.LookPath("opencode")
	if err != nil {
		return response, err
	}
	configDir, err := os.MkdirTemp("", "multica-v32-config-")
	if err != nil {
		return response, err
	}
	defer os.RemoveAll(configDir)
	env := protocolV32Environment(plan, configDir)
	resolved := exec.CommandContext(ctx, executable, "--pure", "debug", "config")
	resolved.Dir, resolved.Env = p.workspaceRoot, env
	resolvedRaw, err := resolved.Output()
	if err != nil {
		return response, fmt.Errorf("v32 resolved-config preflight: %w", err)
	}
	resolvedHash, err := agentqualification.ValidateOpenCodeResolvedConfig(plan, resolvedRaw)
	if err != nil {
		return response, err
	}
	command := exec.CommandContext(ctx, executable, "--pure", "run", "--format", "json", "--model", plan.Model, "--variant", plan.Effort, "--agent", "qualification", "--title", "protocol-v32-"+safeProtocolV32Name(plan.CandidateID), "--dir", p.workspaceRoot)
	command.Dir, command.Env, command.Stdin = p.workspaceRoot, env, strings.NewReader(plan.Prompt)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	rawJSONL, err := command.Output()
	if err != nil {
		return response, fmt.Errorf("v32 OpenCode run: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	sessionID, err := protocolV32SessionID(rawJSONL)
	if err != nil {
		return response, err
	}
	export := exec.CommandContext(ctx, executable, "--pure", "export", sessionID)
	export.Dir, export.Env = p.workspaceRoot, env
	rawExport, err := export.Output()
	if err != nil {
		return response, fmt.Errorf("v32 OpenCode export: %w", err)
	}
	result, candidateJSON, err := agentqualification.ParseOpenCodeRunEnvelope(rawJSONL, rawExport)
	if err != nil {
		return response, err
	}
	rawReceipt, _ := json.Marshal(map[string]any{"jsonl": string(rawJSONL), "export": json.RawMessage(rawExport), "resolved_config_sha256": resolvedHash, "prompt_sha256": plan.PromptSHA256, "schema_sha256": plan.SchemaSHA256})
	response.CandidateJSON = append(json.RawMessage(nil), candidateJSON...)
	response.RawReceipt = rawReceipt
	reportedProvider, reportedUpstreamModel, ok := strings.Cut(result.ReportedModel, "/")
	evidenceMode := "reported_serving_provider"
	if result.ReportedServingProvider == "" {
		evidenceMode = "enforced_single_route_no_fallback"
	}
	response.ProviderIdentity = agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: reportedProvider, ReportedModel: reportedUpstreamModel, ReportedEffort: plan.Effort, ReportedServingProvider: result.ReportedServingProvider, ResolvedConfigSHA256: resolvedHash, ServingProviderEvidence: evidenceMode}
	response.Usage.ProviderTurns = 1
	response.Usage.InputTokens = protocolV32Value(result.Usage.InputTokens) + protocolV32Value(result.Usage.CachedInputTokens)
	response.Usage.OutputTokens = protocolV32Value(result.Usage.OutputTokens)
	response.Usage.ReasoningTokens = protocolV32Value(result.Usage.ReasoningTokens)
	response.Usage.CostMicros = protocolV32Value(result.Usage.ProviderReportedCostMicros)
	// Seal the raw transport evidence before any identity rejection. A parser or
	// route assertion can invalidate an observation, but can never erase a paid call.
	if p.artifactRoot != "" {
		path := filepath.Join(p.artifactRoot, "presealed", safeProtocolV32Name(p.binding.BindingID), fmt.Sprintf("provider-%02d.json", p.callIndex+1))
		if err := writeProtocolV32Once(path, rawReceipt); err != nil {
			return response, fmt.Errorf("v32 preseal raw receipt: %w", err)
		}
		p.callIndex++
	}
	expectedUpstreamModel := strings.TrimPrefix(plan.Model, "openrouter/")
	if !ok || reportedProvider != "openrouter" || reportedUpstreamModel != expectedUpstreamModel || (result.ReportedServingProvider != "" && !strings.EqualFold(result.ReportedServingProvider, plan.ServingProvider)) {
		return response, errors.New("v32 actual direct provider, upstream model, or serving provider did not match the resolved plan")
	}
	return response, nil
}

func writeProtocolV32Once(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

const protocolV32RecoverySession = "ses_f65cc7190ffeCTv632lIB62CN5"
const protocolV32RecoveryBinding = "protocol-v32.claudesonnet5.medium.one-action-loop-v1"

var protocolV32V3ExactImport = protocolV32ImportExpectation{
	LedgerSHA256: "ebe1c31fa7b5b010e736e685ebed5110dc5d599dfbee89e219b860872efb5417",
	ExportSHA256: "03bef64fa13daf0827532bd6347ac9904137b85b39453ddc63df5f5bbbd5abcc",
	RecordSHA256: "21dd49a4f6b7356bc5406bdd4897399c9f3361c6f6179661ea341723ee6fa6dc",
	SessionID:    protocolV32RecoverySession, BindingID: protocolV32RecoveryBinding,
	Usage: agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 4686, OutputTokens: 383, ReasoningTokens: 218, CostMicros: 17724},
}

const protocolV32ClaudePlanBinding = "protocol-v32.claudesonnet5.medium.one-shot-plan-v1"
const protocolV32ClaudePlanSession = "ses_f65c026bdffew1QiN3HWdL8v6A"

var protocolV32V4ExactImport = protocolV32V4ImportExpectation{
	LedgerSHA256:    "153501f3a889398dad5a6442de511ffbc0510fe84d0603e567eeb3bf38612d36",
	ClaimSHA256:     "385504ef20bfd6865ff3e54d454d8b256cb16e8e96b6af7549560f4d1a6a45d3",
	PresealedSHA256: "3970cabf0fb183e36036dfe9f13f3f6dd6156b6e3120cb3946eeacad162d2c3a",
	SessionID:       protocolV32ClaudePlanSession, BindingID: protocolV32ClaudePlanBinding,
	ResolvedConfigSHA256: "157c9e75f8c93357437232b4927bbaf90566b0a4991389ea68e305e44881c5e4",
	PromptSHA256:         "6176ef65f05fc298a8daab6f579300901963feee139648e01505db903c52b0da",
	SchemaSHA256:         "80f94b820491033497f4d8fed2d1ec4a0c350404a5f9f556851e217e193ecaa7",
	Usage:                agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 4695, OutputTokens: 1362, ReasoningTokens: 269, CostMicros: 28047},
}

type protocolV32V4ImportExpectation struct {
	LedgerSHA256, ClaimSHA256, PresealedSHA256       string
	SessionID, BindingID                             string
	ResolvedConfigSHA256, PromptSHA256, SchemaSHA256 string
	Usage                                            agentqualification.ProtocolAblationV32Usage
}

type protocolV32ImportExpectation struct {
	LedgerSHA256, ExportSHA256, RecordSHA256 string
	SessionID, BindingID                     string
	Usage                                    agentqualification.ProtocolAblationV32Usage
}

type protocolV32RecoveryRecord struct {
	SessionID         string                                            `json:"session_id"`
	ProviderCalls     int                                               `json:"provider_calls"`
	Classification    string                                            `json:"classification"`
	Scope             string                                            `json:"scope"`
	OriginalLedgerSHA string                                            `json:"original_ledger_sha256"`
	Observation       agentqualification.ProtocolAblationV32Observation `json:"observation"`
}

func initializeProtocolV32V3(v2Root, v3Root string) (agentqualification.ProtocolAblationV32Ledger, error) {
	if filepath.Base(filepath.Clean(v2Root)) != "protocol-ablation-v32-v2" || filepath.Base(filepath.Clean(v3Root)) != "protocol-ablation-v32-v3" {
		return agentqualification.ProtocolAblationV32Ledger{}, errors.New("v32 import requires the exact preserved v2 and clean v3 artifact identities")
	}
	return initializeProtocolV32V3WithExpectation(v2Root, v3Root, protocolV32V3ExactImport)
}

func initializeProtocolV32V3WithExpectation(v2Root, v3Root string, expected protocolV32ImportExpectation) (agentqualification.ProtocolAblationV32Ledger, error) {
	var ledger agentqualification.ProtocolAblationV32Ledger
	if err := agentqualification.RequireProtocolAblationV32ReviewedFreeze(); err != nil {
		return ledger, err
	}
	if err := verifyProtocolV32CommandFreeze(); err != nil {
		return ledger, err
	}
	recoveryDir := filepath.Join(v2Root, "recovered-observations", safeProtocolV32Name(expected.BindingID))
	ledgerRaw, err := os.ReadFile(filepath.Join(v2Root, "ledger.json"))
	if err != nil {
		return ledger, err
	}
	exportRaw, err := os.ReadFile(filepath.Join(recoveryDir, "session-export.json"))
	if err != nil {
		return ledger, err
	}
	recordRaw, err := os.ReadFile(filepath.Join(recoveryDir, "observation.json"))
	if err != nil {
		return ledger, err
	}
	if protocolV32SHA(ledgerRaw) != expected.LedgerSHA256 || protocolV32SHA(exportRaw) != expected.ExportSHA256 || protocolV32SHA(recordRaw) != expected.RecordSHA256 {
		return ledger, errors.New("v32 carry-forward source hash mismatch")
	}
	var oldLedger agentqualification.ProtocolAblationV32Ledger
	var record protocolV32RecoveryRecord
	if json.Unmarshal(ledgerRaw, &oldLedger) != nil || json.Unmarshal(recordRaw, &record) != nil {
		return ledger, errors.New("v32 carry-forward source JSON is invalid")
	}
	claim, only := oldLedger.Claims[expected.BindingID]
	if !only || len(oldLedger.Claims) != 1 || claim.BindingID != expected.BindingID || claim.Status != "pending" || oldLedger.GlobalHalt || oldLedger.Usage != (agentqualification.ProtocolAblationV32Usage{}) {
		return ledger, errors.New("v32 carry-forward does not match the sole pending paid cell")
	}
	obs := record.Observation
	if record.SessionID != expected.SessionID || record.ProviderCalls != 0 || record.Classification != "test_invalidity" || record.Scope != "observation" || record.OriginalLedgerSHA != expected.LedgerSHA256 || obs.BindingID != expected.BindingID || obs.Outcome != "invalid" || obs.FailureClass != "test_invalidity" || obs.FailureScope != "observation" || obs.Usage != expected.Usage || obs.EvidenceSHA256 != agentqualification.ProtocolAblationV32ObservationEvidenceSHA(obs) {
		return ledger, errors.New("v32 carry-forward recovered observation mismatch")
	}
	var exported struct {
		Info struct {
			ID     string                                   `json:"id"`
			Model  struct{ ID, ProviderID, Variant string } `json:"model"`
			Cost   float64                                  `json:"cost"`
			Tokens struct {
				Input, Output, Reasoning int64
				Cache                    struct{ Read, Write int64 } `json:"cache"`
			} `json:"tokens"`
		} `json:"info"`
	}
	if json.Unmarshal(exportRaw, &exported) != nil || exported.Info.ID != expected.SessionID || exported.Info.Model.ProviderID != "openrouter" || exported.Info.Model.ID != "anthropic/claude-sonnet-5" || exported.Info.Model.Variant != "medium" {
		return ledger, errors.New("v32 carry-forward session identity mismatch")
	}
	exportUsage := agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: exported.Info.Tokens.Input + exported.Info.Tokens.Cache.Read + exported.Info.Tokens.Cache.Write, OutputTokens: exported.Info.Tokens.Output, ReasoningTokens: exported.Info.Tokens.Reasoning, CostMicros: int64(math.Round(exported.Info.Cost * 1_000_000))}
	if exportUsage != expected.Usage {
		return ledger, errors.New("v32 carry-forward authoritative usage mismatch")
	}
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return ledger, err
	}
	found := false
	for _, binding := range agentqualification.ProtocolAblationV32Bindings(d) {
		if binding.BindingID == expected.BindingID {
			found = true
		}
	}
	if !found {
		return ledger, errors.New("v32 carry-forward binding is not frozen")
	}
	ledger = agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{expected.BindingID: {BindingID: expected.BindingID, Status: "completed", Completed: &obs}}, Usage: expected.Usage}
	if err := os.MkdirAll(v3Root, 0o700); err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	finalPath := filepath.Join(v3Root, "ledger.json")
	if _, err := os.Stat(finalPath); !errors.Is(err, os.ErrNotExist) {
		return agentqualification.ProtocolAblationV32Ledger{}, errors.New("v32 carry-forward target is not clean")
	}
	raw, _ := json.MarshalIndent(ledger, "", "  ")
	temporary := finalPath + ".seed.tmp"
	if err := writeProtocolV32Once(temporary, raw); err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	if err := os.Rename(temporary, finalPath); err != nil {
		return agentqualification.ProtocolAblationV32Ledger{}, err
	}
	return ledger, nil
}

func recoverProtocolV32V3AndInitializeV4(v3Root, v4Root string) (agentqualification.ProtocolAblationV32Ledger, error) {
	if filepath.Base(filepath.Clean(v3Root)) != "protocol-ablation-v32-v3" || filepath.Base(filepath.Clean(v4Root)) != "protocol-ablation-v32-v4" {
		return agentqualification.ProtocolAblationV32Ledger{}, errors.New("v32 recovery requires the exact preserved v3 and clean v4 artifact identities")
	}
	return recoverProtocolV32V3AndInitializeV4WithExpectation(v3Root, v4Root, protocolV32V4ExactImport)
}

func recoverProtocolV32V3AndInitializeV4WithExpectation(v3Root, v4Root string, expected protocolV32V4ImportExpectation) (agentqualification.ProtocolAblationV32Ledger, error) {
	var empty agentqualification.ProtocolAblationV32Ledger
	if err := agentqualification.RequireProtocolAblationV32ReviewedFreeze(); err != nil {
		return empty, err
	}
	if err := verifyProtocolV32CommandFreeze(); err != nil {
		return empty, err
	}
	ledgerPath := filepath.Join(v3Root, "ledger.json")
	claimPath := filepath.Join(v3Root, "claims", safeProtocolV32Name(expected.BindingID)+".claim")
	presealedPath := filepath.Join(v3Root, "presealed", safeProtocolV32Name(expected.BindingID), "provider-01.json")
	ledgerRaw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return empty, err
	}
	claimRaw, err := os.ReadFile(claimPath)
	if err != nil {
		return empty, err
	}
	presealedRaw, err := os.ReadFile(presealedPath)
	if err != nil {
		return empty, err
	}
	if protocolV32SHA(ledgerRaw) != expected.LedgerSHA256 || protocolV32SHA(claimRaw) != expected.ClaimSHA256 || protocolV32SHA(presealedRaw) != expected.PresealedSHA256 {
		return empty, errors.New("v32 v4 carry-forward source hash mismatch")
	}
	var old agentqualification.ProtocolAblationV32Ledger
	if json.Unmarshal(ledgerRaw, &old) != nil || len(old.Claims) != 2 || old.GlobalHalt || old.Usage != protocolV32V3ExactImport.Usage {
		return empty, errors.New("v32 v3 ledger state is not the exact two-cell handoff")
	}
	if prior := old.Claims[protocolV32RecoveryBinding]; prior.Status != "completed" || prior.Completed == nil || prior.Completed.FailureClass != "test_invalidity" || prior.Completed.FailureScope != "observation" {
		return empty, errors.New("v32 v3 prior invalid cell drifted")
	}
	if pending := old.Claims[expected.BindingID]; pending.BindingID != expected.BindingID || pending.Status != "pending" || pending.Completed != nil || string(claimRaw) != expected.BindingID+"\n" {
		return empty, errors.New("v32 v3 Claude plan cell is not the exact pending claim")
	}
	var receipt struct {
		JSONL                string          `json:"jsonl"`
		Export               json.RawMessage `json:"export"`
		ResolvedConfigSHA256 string          `json:"resolved_config_sha256"`
		PromptSHA256         string          `json:"prompt_sha256"`
		SchemaSHA256         string          `json:"schema_sha256"`
	}
	if json.Unmarshal(presealedRaw, &receipt) != nil || receipt.ResolvedConfigSHA256 != expected.ResolvedConfigSHA256 || receipt.PromptSHA256 != expected.PromptSHA256 || receipt.SchemaSHA256 != expected.SchemaSHA256 {
		return empty, errors.New("v32 v3 presealed route/config binding mismatch")
	}
	result, candidate, err := agentqualification.ParseOpenCodeRunEnvelope([]byte(receipt.JSONL), receipt.Export)
	if err != nil {
		return empty, fmt.Errorf("v32 v3 presealed envelope invalid: %w", err)
	}
	provider, upstream, ok := strings.Cut(result.ReportedModel, "/")
	if !ok || provider != "openrouter" || upstream != "anthropic/claude-sonnet-5" || result.ObservationID != "opencode:"+expected.SessionID+":"+strings.Split(result.ProviderResponseID, ":")[1] {
		return empty, errors.New("v32 v3 recovered provider/session identity mismatch")
	}
	if result.ReportedServingProvider != "" && !strings.EqualFold(result.ReportedServingProvider, "anthropic") {
		return empty, errors.New("v32 v3 recovered serving-provider contradiction")
	}
	usage := agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: protocolV32Value(result.Usage.InputTokens) + protocolV32Value(result.Usage.CachedInputTokens), OutputTokens: protocolV32Value(result.Usage.OutputTokens), ReasoningTokens: protocolV32Value(result.Usage.ReasoningTokens), CostMicros: protocolV32Value(result.Usage.ProviderReportedCostMicros)}
	if usage != expected.Usage {
		return empty, errors.New("v32 v3 recovered authoritative usage mismatch")
	}
	if json.Valid(bytes.TrimSpace(candidate)) {
		return empty, errors.New("v32 v3 candidate unexpectedly satisfies the JSON protocol")
	}
	identity := agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: provider, ReportedModel: upstream, ReportedEffort: "medium", ReportedServingProvider: result.ReportedServingProvider, ResolvedConfigSHA256: receipt.ResolvedConfigSHA256, ServingProviderEvidence: "enforced_single_route_no_fallback"}
	if result.ReportedServingProvider != "" {
		identity.ServingProviderEvidence = "reported_serving_provider"
	}
	observation := agentqualification.ProtocolAblationV32Observation{BindingID: expected.BindingID, Outcome: "failed", FailureClass: "response_or_tool_protocol_failure", Usage: usage, ProviderIdentity: identity, RawReceipts: []json.RawMessage{append(json.RawMessage(nil), presealedRaw...)}, Actions: []agentqualification.RealisticCandidateAction{}, GraderResult: agentqualification.RealisticNextCalibrationResult{CaseID: "P32"}}
	observation.EvidenceSHA256 = agentqualification.ProtocolAblationV32ObservationEvidenceSHA(observation)
	recoveryRecord := struct {
		SessionID             string                                            `json:"session_id"`
		SourceLedgerSHA256    string                                            `json:"source_ledger_sha256"`
		SourceClaimSHA256     string                                            `json:"source_claim_sha256"`
		SourcePresealedSHA256 string                                            `json:"source_presealed_sha256"`
		ProviderCalls         int                                               `json:"provider_calls"`
		Observation           agentqualification.ProtocolAblationV32Observation `json:"observation"`
	}{expected.SessionID, expected.LedgerSHA256, expected.ClaimSHA256, expected.PresealedSHA256, 0, observation}
	recoveryRaw, _ := json.MarshalIndent(recoveryRecord, "", "  ")
	recoveryPath := filepath.Join(v3Root, "recovered-observations", safeProtocolV32Name(expected.BindingID), "observation.json")
	if err := writeProtocolV32Once(recoveryPath, recoveryRaw); err != nil {
		return empty, err
	}
	prior := old.Claims[protocolV32RecoveryBinding]
	ledger := agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{protocolV32RecoveryBinding: prior, expected.BindingID: {BindingID: expected.BindingID, Status: "completed", Completed: &observation}}, Usage: agentqualification.ProtocolAblationV32Usage{ProviderTurns: prior.Completed.Usage.ProviderTurns + usage.ProviderTurns, InputTokens: prior.Completed.Usage.InputTokens + usage.InputTokens, OutputTokens: prior.Completed.Usage.OutputTokens + usage.OutputTokens, ReasoningTokens: prior.Completed.Usage.ReasoningTokens + usage.ReasoningTokens, CostMicros: prior.Completed.Usage.CostMicros + usage.CostMicros}}
	if err := os.MkdirAll(v4Root, 0o700); err != nil {
		return empty, err
	}
	finalPath := filepath.Join(v4Root, "ledger.json")
	if _, err := os.Stat(finalPath); !errors.Is(err, os.ErrNotExist) {
		return empty, errors.New("v32 v4 target is not clean")
	}
	raw, _ := json.MarshalIndent(ledger, "", "  ")
	temporary := finalPath + ".seed.tmp"
	if err := writeProtocolV32Once(temporary, raw); err != nil {
		return empty, err
	}
	if err := os.Rename(temporary, finalPath); err != nil {
		return empty, err
	}
	return ledger, nil
}

// recoverProtocolV32Session seals the already-existing paid session without
// executing `opencode run` or changing any file that was part of the failed run.
func recoverProtocolV32Session(ctx context.Context, artifactRoot, sessionID string) (protocolV32RecoveryRecord, error) {
	var record protocolV32RecoveryRecord
	if filepath.Base(filepath.Clean(artifactRoot)) != "protocol-ablation-v32-v2" || sessionID != protocolV32RecoverySession {
		return record, errors.New("v32 recovery is restricted to the preserved v2 Claude session")
	}
	ledgerRaw, err := os.ReadFile(filepath.Join(artifactRoot, "ledger.json"))
	if err != nil {
		return record, err
	}
	var ledger agentqualification.ProtocolAblationV32Ledger
	if json.Unmarshal(ledgerRaw, &ledger) != nil || ledger.Claims[protocolV32RecoveryBinding].Status != "pending" {
		return record, errors.New("v32 recovery source is not the exact pending observation")
	}
	executable, err := exec.LookPath("opencode")
	if err != nil {
		return record, err
	}
	command := exec.CommandContext(ctx, executable, "--pure", "export", sessionID)
	rawExport, err := command.Output()
	if err != nil {
		return record, fmt.Errorf("v32 local session export: %w", err)
	}
	start := bytes.IndexByte(rawExport, '{')
	if start < 0 {
		return record, errors.New("v32 recovery export omitted JSON")
	}
	rawExport = rawExport[start:]
	var exported struct {
		Info struct {
			ID     string                                   `json:"id"`
			Model  struct{ ID, ProviderID, Variant string } `json:"model"`
			Cost   float64                                  `json:"cost"`
			Tokens struct {
				Input, Output, Reasoning int64
				Cache                    struct{ Read, Write int64 } `json:"cache"`
			} `json:"tokens"`
		} `json:"info"`
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(rawExport, &exported); err != nil || exported.Info.ID != sessionID || exported.Info.Model.ProviderID != "openrouter" || exported.Info.Model.ID != "anthropic/claude-sonnet-5" || exported.Info.Model.Variant != "medium" {
		return record, errors.New("v32 recovery export identity mismatch")
	}
	serving := protocolV32SelectedProvider(rawExport)
	usage := agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: exported.Info.Tokens.Input + exported.Info.Tokens.Cache.Read + exported.Info.Tokens.Cache.Write, OutputTokens: exported.Info.Tokens.Output, ReasoningTokens: exported.Info.Tokens.Reasoning, CostMicros: int64(math.Round(exported.Info.Cost * 1_000_000))}
	observation := agentqualification.ProtocolAblationV32Observation{BindingID: protocolV32RecoveryBinding, Outcome: "invalid", FailureClass: "test_invalidity", FailureScope: "observation", Usage: usage, ProviderIdentity: agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: exported.Info.Model.ID, ReportedEffort: exported.Info.Model.Variant, ReportedServingProvider: serving, ResolvedConfigSHA256: "unavailable_preserved_parser_failure"}, RawReceipts: []json.RawMessage{append(json.RawMessage(nil), rawExport...)}, Actions: []agentqualification.RealisticCandidateAction{}, GraderResult: agentqualification.RealisticNextCalibrationResult{CaseID: "P32"}}
	observation.EvidenceSHA256 = agentqualification.ProtocolAblationV32ObservationEvidenceSHA(observation)
	record = protocolV32RecoveryRecord{SessionID: sessionID, ProviderCalls: 0, Classification: "test_invalidity", Scope: "observation", OriginalLedgerSHA: protocolV32SHA(ledgerRaw), Observation: observation}
	dir := filepath.Join(artifactRoot, "recovered-observations", safeProtocolV32Name(protocolV32RecoveryBinding))
	if err := writeProtocolV32Once(filepath.Join(dir, "session-export.json"), rawExport); err != nil {
		return record, err
	}
	rawRecord, _ := json.MarshalIndent(record, "", "  ")
	if err := writeProtocolV32Once(filepath.Join(dir, "observation.json"), rawRecord); err != nil {
		return record, err
	}
	if after, err := os.ReadFile(filepath.Join(artifactRoot, "ledger.json")); err != nil || !bytes.Equal(after, ledgerRaw) {
		return record, errors.New("v32 recovery changed the preserved ledger")
	}
	return record, nil
}

func protocolV32SelectedProvider(raw []byte) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	providers := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			if selected, _ := n["selected"].(bool); selected {
				if p, _ := n["provider"].(string); p != "" {
					providers[p] = true
				}
			}
			for _, child := range n {
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(value)
	if len(providers) != 1 {
		return ""
	}
	for provider := range providers {
		return provider
	}
	return ""
}

func protocolV32Environment(plan agentqualification.OpenCodeScreeningPlan, configDir string) []string {
	names := []string{"PATH", "SystemRoot", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "APPDATA", "TEMP", "TMP"}
	if runtime.GOOS != "windows" {
		names = []string{"PATH", "HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "TMPDIR"}
	}
	values := map[string]string{"OPENCODE_CONFIG_CONTENT": string(plan.InlineConfig), "OPENCODE_CONFIG_DIR": configDir, "OPENCODE_DISABLE_GLOBAL_CONFIG": "true", "OPENCODE_DISABLE_AUTOUPDATE": "true", "OPENCODE_DISABLE_PRUNE": "true", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX": "20000"}
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			values[name] = value
		}
	}
	env := make([]string, 0, len(values))
	for _, name := range append(names, "OPENCODE_CONFIG_CONTENT", "OPENCODE_CONFIG_DIR", "OPENCODE_DISABLE_GLOBAL_CONFIG", "OPENCODE_DISABLE_AUTOUPDATE", "OPENCODE_DISABLE_PRUNE", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX") {
		if value, ok := values[name]; ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func protocolV32SessionID(raw []byte) (string, error) {
	var id string
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct {
			SessionID string `json:"sessionID"`
		}
		if json.Unmarshal(line, &event) != nil || event.SessionID == "" || (id != "" && id != event.SessionID) {
			return "", errors.New("v32 OpenCode emitted invalid session identity")
		}
		id = event.SessionID
	}
	if id == "" {
		return "", errors.New("v32 OpenCode emitted no session")
	}
	return id, nil
}

func protocolV32Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func protocolV32SHA(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
