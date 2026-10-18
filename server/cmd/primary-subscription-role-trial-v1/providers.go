package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type realTrialInvoker struct{}

func (realTrialInvoker) Invoke(ctx context.Context, sessionRoot string, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) trialSessionReceipt {
	if invocation.Lane == "claude_code_subscription" {
		return invokeClaudeTrial(ctx, sessionRoot, invocation, prompt, schema)
	}
	return invokeCodexTrial(ctx, sessionRoot, invocation, prompt, schema)
}

func (protocol trialProtocol) baseSessionReceipt(invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) trialSessionReceipt {
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialSessionReceipt{Status: "invalid", FailureClass: "definition_invalid", FailureDetail: err.Error()}
	}
	lane := laneFor(definition, invocation.Lane)
	return trialSessionReceipt{Ordinal: invocation.Ordinal, ObservationID: invocation.ObservationID, Lane: invocation.Lane, Participant: invocation.Participant, ParticipantKind: invocation.ParticipantKind, Status: "invalid", RequestedModel: lane.RequestedModel, RequestedEffort: lane.Effort, ObservedModels: []string{}, NativeToolEvents: []string{}, PromptSHA256: hashBytes([]byte(prompt)), OutputSchemaSHA256: hashBytes(schema), ProviderWorkdirRemoved: true, Retries: 0, Fallbacks: false, CostBoundary: "subscription_no_cash_charge_claim"}
}

func (protocol trialProtocol) invokeClaudeTrial(ctx context.Context, sessionRoot string, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) (receipt trialSessionReceipt) {
	if protocol.contracts.Historical() {
		return trialSessionReceipt{FailureClass: "historical_input_not_executable"}
	}

	receipt = protocol.baseSessionReceipt(invocation, prompt, schema)
	executable, err := exec.LookPath(windowsExecutable("claude"))
	if err != nil {
		receipt.FailureClass, receipt.FailureDetail = "runtime_unavailable", err.Error()
		return receipt
	}
	workdir, err := os.MkdirTemp("", "sm-item14-claude-session-")
	if err != nil {
		receipt.FailureClass, receipt.FailureDetail = "workdir_failure", err.Error()
		return receipt
	}
	receipt.ProviderWorkdirRemoved = false
	defer func() { receipt.ProviderWorkdirRemoved = cleanupPath(workdir) }()
	args := []string{"-p", "--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--no-session-persistence", "--permission-prompts", "none", "--output-format", "json", "--json-schema", string(schema), "--max-turns", "1", "--model", "claude-sonnet-4-6", "--effort", "medium"}
	started := time.Now().UTC()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = workdir
	command.Env = providerChildEnvironment(invocation.Lane)
	command.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	completed := time.Now().UTC()
	receipt.StartedAt, receipt.CompletedAt, receipt.WallMillis = started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano), completed.Sub(started).Milliseconds()
	receipt.RawResponse = append([]byte(nil), stdout.Bytes()...)
	if runErr != nil {
		receipt.ProviderCallCount = claudeCallCount(stdout.Bytes())
		receipt.FailureClass = "provider_or_transport_failure"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			receipt.FailureClass = "session_timeout"
		}
		receipt.FailureDetail = sanitizedFailure(runErr, stderr.String())
		return receipt
	}
	parsed, candidate, parseErr := parseClaudeTrialResult(stdout.Bytes())
	parsed.Ordinal, parsed.ObservationID, parsed.Lane, parsed.Participant, parsed.ParticipantKind = receipt.Ordinal, receipt.ObservationID, receipt.Lane, receipt.Participant, receipt.ParticipantKind
	parsed.RequestedModel, parsed.RequestedEffort, parsed.PromptSHA256, parsed.OutputSchemaSHA256 = receipt.RequestedModel, receipt.RequestedEffort, receipt.PromptSHA256, receipt.OutputSchemaSHA256
	parsed.StartedAt, parsed.CompletedAt, parsed.WallMillis, parsed.RawResponse = receipt.StartedAt, receipt.CompletedAt, receipt.WallMillis, receipt.RawResponse
	parsed.CostBoundary, parsed.Retries, parsed.Fallbacks = "claude_code_subscription_cli_api_equivalent_estimate", 0, false
	if parseErr != nil {
		parsed.Status, parsed.FailureClass, parsed.FailureDetail = "invalid", "provider_receipt_invalid", parseErr.Error()
		return parsed
	}
	parsed.Status, parsed.Candidate, parsed.CandidateOutputSHA256 = "completed", candidate, hashBytes(candidate)
	return parsed
}

type claudeTrialEnvelope struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	IsError          bool            `json:"is_error"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	SessionID        string          `json:"session_id"`
	TotalCostUSD     json.Number     `json:"total_cost_usd"`
	Usage            struct {
		InputTokens              int64 `json:"input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
	} `json:"usage"`
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
}

func parseClaudeTrialResult(raw []byte) (trialSessionReceipt, json.RawMessage, error) {
	var receipt trialSessionReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var envelope claudeTrialEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return receipt, nil, err
	}
	if envelope.Type != "result" || envelope.Subtype != "success" || envelope.IsError || envelope.SessionID == "" || envelope.Usage.InputTokens < 0 || envelope.Usage.CacheCreationInputTokens < 0 || envelope.Usage.CacheReadInputTokens < 0 || envelope.Usage.OutputTokens <= 0 {
		return receipt, nil, fmt.Errorf("Claude terminal receipt incomplete")
	}
	models := agentqualification.PrimarySubscriptionTrialSortedModelsV1(envelope.ModelUsage)
	if !containsString(models, "claude-sonnet-4-6") {
		return receipt, nil, fmt.Errorf("requested Sonnet primary absent from Claude modelUsage")
	}
	for _, model := range models {
		if model != "claude-sonnet-4-6" && model != "claude-haiku-4-5-20251001" {
			return receipt, nil, fmt.Errorf("unapproved Claude model identity %s", model)
		}
	}
	cost, err := decimalMicros(envelope.TotalCostUSD.String())
	if err != nil {
		return receipt, nil, err
	}
	candidate := envelope.StructuredOutput
	if len(candidate) == 0 || string(candidate) == "null" {
		candidate = json.RawMessage(envelope.Result)
	}
	if !json.Valid(candidate) {
		return receipt, nil, fmt.Errorf("Claude candidate output is not JSON")
	}
	receipt.ObservedModels = models
	receipt.ObservedModelStatus = "reported_composite_allowed"
	receipt.InputTokens = envelope.Usage.InputTokens + envelope.Usage.CacheCreationInputTokens + envelope.Usage.CacheReadInputTokens
	receipt.CachedInputTokens = envelope.Usage.CacheReadInputTokens
	receipt.OutputTokens, receipt.CostMicros, receipt.ProviderCallCount = envelope.Usage.OutputTokens, cost, 1
	receipt.ModelUsage = parseClaudeModelUsage(envelope.ModelUsage)
	return receipt, append(json.RawMessage(nil), candidate...), nil
}

func parseClaudeModelUsage(raw map[string]json.RawMessage) []trialModelUsage {
	result := []trialModelUsage{}
	for model, entry := range raw {
		var usage struct {
			InputTokens              int64       `json:"inputTokens"`
			OutputTokens             int64       `json:"outputTokens"`
			CacheReadInputTokens     int64       `json:"cacheReadInputTokens"`
			CacheCreationInputTokens int64       `json:"cacheCreationInputTokens"`
			CostUSD                  json.Number `json:"costUSD"`
		}
		decoder := json.NewDecoder(bytes.NewReader(entry))
		decoder.UseNumber()
		if decoder.Decode(&usage) != nil {
			result = append(result, trialModelUsage{Model: model})
			continue
		}
		cost, _ := decimalMicros(usage.CostUSD.String())
		result = append(result, trialModelUsage{Model: model, InputTokens: usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens, OutputTokens: usage.OutputTokens, CostMicros: cost})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Model < result[j].Model })
	return result
}

func claudeCallCount(raw []byte) int {
	var envelope claudeTrialEnvelope
	if json.Unmarshal(raw, &envelope) == nil && (envelope.SessionID != "" || len(envelope.ModelUsage) != 0) {
		return 1
	}
	return 0
}

func (protocol trialProtocol) invokeCodexTrial(ctx context.Context, sessionRoot string, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) (receipt trialSessionReceipt) {
	if protocol.contracts.Historical() {
		return trialSessionReceipt{FailureClass: "historical_input_not_executable"}
	}

	receipt = protocol.baseSessionReceipt(invocation, prompt, schema)
	executable, err := exec.LookPath(windowsExecutable("codex"))
	if err != nil {
		receipt.FailureClass, receipt.FailureDetail = "runtime_unavailable", err.Error()
		return receipt
	}
	workdir, err := os.MkdirTemp("", "sm-item14-codex-session-")
	if err != nil {
		receipt.FailureClass, receipt.FailureDetail = "workdir_failure", err.Error()
		return receipt
	}
	receipt.ProviderWorkdirRemoved = false
	defer func() { receipt.ProviderWorkdirRemoved = cleanupPath(workdir) }()
	profile := filepath.Join(workdir, "codex-home")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		receipt.FailureClass, receipt.FailureDetail = "workdir_failure", err.Error()
		return receipt
	}
	if err := copyOpaqueCodexAuth(filepath.Join(profile, "auth.json")); err != nil {
		receipt.FailureClass, receipt.FailureDetail = "credential_unavailable", err.Error()
		return receipt
	}
	schemaPath, outputPath := filepath.Join(workdir, "output-schema.json"), filepath.Join(workdir, "candidate.json")
	if err := os.WriteFile(schemaPath, schema, 0o600); err != nil {
		receipt.FailureClass, receipt.FailureDetail = "workdir_failure", err.Error()
		return receipt
	}
	args := []string{"exec", "--ignore-user-config", "--ignore-rules", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--model", "gpt-5.6-luna", "--output-schema", schemaPath, "--output-last-message", outputPath, "--json", "-C", workdir,
		"-c", `model_reasoning_effort="medium"`, "-c", "request_max_retries=0", "-c", "stream_max_retries=0", "-c", "project_doc_max_bytes=0", "-c", "project_root_markers=[]", "-c", `web_search="disabled"`, "-c", `approval_policy="never"`}
	for _, feature := range codexDisabledFeatures() {
		args = append(args, "--disable", feature)
	}
	args = append(args, "-")
	started := time.Now().UTC()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir, command.Env, command.Stdin = workdir, replaceEnvironment(providerChildEnvironment(invocation.Lane), "CODEX_HOME", profile), strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	completed := time.Now().UTC()
	receipt.StartedAt, receipt.CompletedAt, receipt.WallMillis = started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano), completed.Sub(started).Milliseconds()
	receipt.RawResponse = append([]byte(nil), stdout.Bytes()...)
	parsed := parseCodexTrialEvents(stdout.Bytes())
	receipt.InputTokens, receipt.CachedInputTokens, receipt.OutputTokens, receipt.ReasoningTokens = parsed.InputTokens, parsed.CachedInputTokens, parsed.OutputTokens, parsed.ReasoningTokens
	receipt.NativeToolEvents, receipt.ProviderCallCount = parsed.NativeToolEvents, parsed.ProviderCallCount
	receipt.ObservedModelStatus = "unavailable_truthful_cli_request_pinned"
	receipt.ObservedModels = []string{}
	receipt.CostMicros, receipt.CostBoundary = 0, "codex_subscription_no_provider_cash_receipt"
	if runErr != nil {
		receipt.FailureClass, receipt.FailureDetail = "provider_or_transport_failure", sanitizedFailure(runErr, stderr.String())
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			receipt.FailureClass = "session_timeout"
		}
		return receipt
	}
	if parsed.Failure != "" || !parsed.Completed {
		receipt.FailureClass, receipt.FailureDetail = "provider_receipt_invalid", parsed.Failure
		return receipt
	}
	if len(receipt.NativeToolEvents) != 0 {
		receipt.FailureClass, receipt.FailureDetail = "native_tool_execution", strings.Join(receipt.NativeToolEvents, ",")
		return receipt
	}
	candidate, err := os.ReadFile(outputPath)
	if err != nil || !json.Valid(candidate) {
		receipt.FailureClass, receipt.FailureDetail = "response_protocol_failure", "Codex final structured output missing or invalid"
		return receipt
	}
	receipt.Status, receipt.Candidate, receipt.CandidateOutputSHA256 = "completed", append(json.RawMessage(nil), candidate...), hashBytes(candidate)
	return receipt
}

type codexTrialEvents struct {
	Completed         bool
	ProviderCallCount int
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	ReasoningTokens   int64
	NativeToolEvents  []string
	Failure           string
}

func parseCodexTrialEvents(raw []byte) codexTrialEvents {
	var result codexTrialEvents
	seenThread, seenTurn := false, false
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		var event map[string]any
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			result.Failure = "Codex emitted malformed JSONL"
			continue
		}
		typeName, _ := event["type"].(string)
		switch typeName {
		case "thread.started":
			seenThread = true
		case "turn.started":
			seenTurn, result.ProviderCallCount = true, 1
		case "item.started", "item.completed":
			item, _ := event["item"].(map[string]any)
			itemType, _ := item["type"].(string)
			if itemType != "agent_message" && itemType != "reasoning" && itemType != "error" {
				result.NativeToolEvents = append(result.NativeToolEvents, typeName+":"+itemType)
			}
		case "turn.completed":
			result.Completed = true
			usage, _ := event["usage"].(map[string]any)
			result.InputTokens = jsonInt64(usage["input_tokens"])
			result.CachedInputTokens = jsonInt64(usage["cached_input_tokens"])
			result.OutputTokens = jsonInt64(usage["output_tokens"])
			result.ReasoningTokens = jsonInt64(usage["reasoning_tokens"])
		case "error", "turn.failed":
			result.Failure = typeName
		default:
			if strings.Contains(strings.ToLower(typeName), "tool") || strings.Contains(strings.ToLower(typeName), "command") || strings.Contains(strings.ToLower(typeName), "web") {
				result.NativeToolEvents = append(result.NativeToolEvents, typeName)
			}
		}
	}
	if scanner.Err() != nil {
		result.Failure = scanner.Err().Error()
	}
	if !seenThread || !seenTurn {
		result.Failure = "Codex lifecycle evidence incomplete"
	}
	return result
}

func jsonInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	case string:
		result, _ := strconv.ParseInt(typed, 10, 64)
		return result
	}
	return 0
}

func codexDisabledFeatures() []string {
	return []string{"apps", "auth_elicitation", "browser_use", "browser_use_external", "browser_use_full_cdp_access", "code_mode", "code_mode_host", "code_mode_only", "computer_use", "default_mode_request_user_input", "deferred_executor", "goals", "image_generation", "in_app_browser", "in_app_chat", "memories", "multi_agent", "multi_agent_v2", "plugins", "remote_plugin", "request_permissions_tool", "shell_tool", "skill_mcp_dependency_install", "skill_search", "sleep_tool", "standalone_web_search", "tool_suggest", "unified_exec", "view_image", "workspace_dependencies"}
}

func providerChildEnvironment(lane string) []string {
	names := []string{"APPDATA", "COMSPEC", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "PATH", "PATHEXT", "PROGRAMDATA", "ProgramFiles", "ProgramFiles(x86)", "SystemDrive", "SystemRoot", "TEMP", "TMP", "USERDOMAIN", "USERNAME", "USERPROFILE", "WINDIR"}
	if lane == "codex_subscription" {
		names = append(names, "CODEX_HOME")
	}
	result := []string{}
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			result = append(result, name+"="+value)
		}
	}
	if lane == "claude_code_subscription" {
		result = append(result, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
	}
	return result
}

func copyOpaqueCodexAuth(destination string) error {
	profile := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if profile == "" {
		profile = filepath.Join(os.Getenv("USERPROFILE"), ".codex")
	}
	source, err := os.Open(filepath.Join(profile, "auth.json"))
	if err != nil {
		return fmt.Errorf("approved Codex subscription authentication is unavailable")
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("approved Codex subscription authentication is unavailable")
	}
	target, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create task-local Codex authentication capsule: %w", err)
	}
	committed := false
	defer func() {
		_ = target.Close()
		if !committed {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy task-local Codex authentication capsule")
	}
	if err := target.Sync(); err != nil {
		return fmt.Errorf("sync task-local Codex authentication capsule")
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close task-local Codex authentication capsule")
	}
	committed = true
	return nil
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := strings.ToLower(name) + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(strings.ToLower(entry), prefix) {
			result = append(result, entry)
		}
	}
	return append(result, name+"="+value)
}

func decimalMicros(value string) (int64, error) {
	if value == "" {
		value = "0"
	}
	amount := new(big.Rat)
	if _, ok := amount.SetString(value); !ok || amount.Sign() < 0 {
		return 0, fmt.Errorf("invalid non-negative provider cost")
	}
	amount.Mul(amount, big.NewRat(1_000_000, 1))
	quotient := new(big.Int).Quo(amount.Num(), amount.Denom())
	if new(big.Int).Mod(amount.Num(), amount.Denom()).Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("provider cost overflow")
	}
	return quotient.Int64(), nil
}

func sanitizedFailure(runErr error, stderr string) string {
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = runErr.Error()
	}
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
