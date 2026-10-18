package main

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type realClaudeV38Provider struct{}

func (realClaudeV38Provider) Plan(ctx context.Context, plan agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
	executable := "claude"
	if runtime.GOOS == "windows" {
		executable = "claude.cmd"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return agentqualification.ClaudeRealisticV37Plan{}, nil, agentqualification.ErrRealisticCredentialFailure
	}
	temporary, err := os.MkdirTemp("", "sm-claude-realistic-v38-")
	if err != nil {
		return agentqualification.ClaudeRealisticV37Plan{}, nil, failV38(v38Evidence, err)
	}
	defer os.RemoveAll(temporary)
	args := make([]string, 0, len(plan.ArgsTemplate))
	for _, arg := range plan.ArgsTemplate {
		if arg == "<generated-schema>" {
			arg = string(plan.Schema)
		}
		args = append(args, arg)
	}
	started := time.Now().UTC()
	command := exec.CommandContext(ctx, path, args...)
	command.Dir = temporary
	command.Env = claudeV38Environment()
	command.Stdin = strings.NewReader(plan.Prompt)
	var stdout, stderr strings.Builder
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	completed := time.Now().UTC()
	raw := []byte(stdout.String())
	if runErr != nil {
		failureClass := v38Transport
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			failureClass = v38Campaign
		}
		if claudeV38StructuredCredentialFailure(raw) {
			failureClass = v38Credential
		}
		failureReceipt := buildV38ProviderFailureReceipt(plan, request, raw, []byte(stderr.String()), started, completed, failureClass)
		if failureClass == v38Credential {
			return agentqualification.ClaudeRealisticV37Plan{}, failureReceipt, agentqualification.ErrRealisticCredentialFailure
		}
		if failureClass == v38Campaign {
			return agentqualification.ClaudeRealisticV37Plan{}, failureReceipt, failV38(v38Campaign, ctx.Err())
		}
		return agentqualification.ClaudeRealisticV37Plan{}, failureReceipt, failV38(v38Transport, errors.New(strings.TrimSpace(stderr.String())))
	}
	candidate, receipt, err := agentqualification.ParseClaudeSubscriptionRealisticV38Response(request, plan, raw, started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano))
	if err != nil {
		failureReceipt := buildV38ProviderFailureReceipt(plan, request, raw, []byte(stderr.String()), started, completed, v38Protocol)
		return candidate, failureReceipt, failV38(v38Protocol, err)
	}
	return candidate, receipt, nil
}

func buildV38ProviderFailureReceipt(plan agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest, stdout, stderr []byte, started, completed time.Time, class v38StopClass) []byte {
	d, _ := agentqualification.ClaudeSubscriptionRealisticV38()
	requestRaw, _ := json.Marshal(request)
	receipt := v38ProviderFailureReceipt{
		SchemaVersion: "silver-mediums.claude-subscription-realistic-provider-failure.v38", ProviderCallID: request.ProviderCallID,
		RequestSHA256: hashV38Bytes(requestRaw), PromptSHA256: plan.PromptSHA256, OutputSchemaSHA256: plan.SchemaSHA256,
		Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort, ServingProvider: d.Configuration.ServingProvider, Transport: d.Transport,
		StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano), FailureClass: string(class), AccountingStatus: "conservative",
		RawStdout: append([]byte(nil), stdout...), RawStdoutSHA256: hashV38Bytes(stdout), RawStderr: append([]byte(nil), stderr...), RawStderrSHA256: hashV38Bytes(stderr), RequestJSON: requestRaw,
		ReservedTotalTokens: d.Caps.MaxTotalReportedTokens, ReservedOutput: d.Caps.MaxGeneratedTokens, ReservedCostMicros: d.Caps.MaxCostMicros,
	}
	var envelope struct {
		TotalCostUSD json.Number `json:"total_cost_usd"`
		Usage        struct {
			InputTokens              int64 `json:"input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(stdout)))
	decoder.UseNumber()
	if decoder.Decode(&envelope) == nil {
		input, ok := checkedAddV38Int64(envelope.Usage.InputTokens, envelope.Usage.CacheCreationInputTokens)
		if ok {
			input, ok = checkedAddV38Int64(input, envelope.Usage.CacheReadInputTokens)
		}
		cost, costOK := microsV38(envelope.TotalCostUSD.String())
		if ok && costOK && input >= 0 && envelope.Usage.OutputTokens >= 0 {
			receipt.AccountingStatus = "exact"
			receipt.InputTokens, receipt.OutputTokens, receipt.CostMicros = input, envelope.Usage.OutputTokens, cost
			receipt.ReservedTotalTokens, receipt.ReservedOutput, receipt.ReservedCostMicros = 0, 0, 0
		}
	}
	raw, _ := json.Marshal(receipt)
	return raw
}

func microsV38(value string) (int64, bool) {
	amount := new(big.Rat)
	if value == "" {
		return 0, false
	}
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

func claudeV38StructuredCredentialFailure(raw []byte) bool {
	var envelope struct {
		Type          string                     `json:"type"`
		Subtype       string                     `json:"subtype"`
		IsError       bool                       `json:"is_error"`
		DurationAPIMs int64                      `json:"duration_api_ms"`
		NumTurns      int                        `json:"num_turns"`
		Result        string                     `json:"result"`
		ModelUsage    map[string]json.RawMessage `json:"modelUsage"`
		Error         *struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Type != "result" || envelope.Subtype != "error_during_execution" || !envelope.IsError {
		return false
	}
	if envelope.Error != nil && envelope.Error.Type == "authentication_error" {
		return true
	}
	return envelope.DurationAPIMs == 0 && envelope.NumTurns == 0 && len(envelope.ModelUsage) == 0 && envelope.Result == "Not logged in · Please run /login"
}

func claudeV38Environment() []string {
	names := []string{"APPDATA", "COMSPEC", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "PATH", "PATHEXT", "SystemDrive", "SystemRoot", "TEMP", "TMP", "USERPROFILE", "WINDIR"}
	environment := make([]string, 0, len(names)+1)
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
}

func verifyClaudeV38RuntimeIdentity() error {
	d, err := agentqualification.ClaudeSubscriptionRealisticV38()
	if err != nil {
		return err
	}
	executable := "claude"
	if runtime.GOOS == "windows" {
		executable = "claude.cmd"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil || !strings.EqualFold(filepath.Clean(absolute), filepath.Clean(d.RuntimeIdentity.ExecutablePath)) {
		return errors.New("Claude executable path drift")
	}
	hash, err := fileHashV38(absolute)
	if err != nil || hash != d.RuntimeIdentity.ExecutableSHA256 {
		return errors.New("Claude executable hash drift")
	}
	return nil
}
