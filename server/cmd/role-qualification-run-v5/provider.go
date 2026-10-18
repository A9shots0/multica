package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const openRouterCreditsURLV5 = "https://openrouter.ai/api/v1/credits"

type liveProviderV5 struct {
	classID             string
	executable          string
	runtimeVersion      string
	executableHash      string
	config              []byte
	resolvedHash        string
	client              *http.Client
	command             *exec.Cmd
	baseURL             string
	configDir           string
	workspace           string
	providerCallStarted bool
	transportStage      agentqualification.RoleQualificationTransportStageV5
	transportCause      agentqualification.RoleQualificationTransportCauseV5
	httpStatus          int
	contentType         string
	partialBody         []byte
	readError           string
}

func runLivePreflightV5(ctx context.Context, classID string) (agentqualification.RoleQualificationPreflightV5, error) {
	cells, err := agentqualification.RoleQualificationClassCellsV4(classID)
	if err != nil {
		return agentqualification.RoleQualificationPreflightV5{}, err
	}
	p, err := newLiveProviderV5(ctx, classID)
	if err != nil {
		return agentqualification.RoleQualificationPreflightV5{}, err
	}
	defer p.Close()
	cell := cells[0]
	if cell.ConfigurationClass.RuntimeProvider != "claude_code_subscription" {
		if err := p.startOpenCode(ctx); err != nil {
			return agentqualification.RoleQualificationPreflightV5{}, err
		}
		p.stopOpenCode()
	}
	baseline := int64(0)
	credential := "authenticated"
	if cell.ConfigurationClass.RuntimeProvider != "claude_code_subscription" {
		baseline, err = readOpenRouterSpendV5(ctx)
		if err != nil {
			return agentqualification.RoleQualificationPreflightV5{}, err
		}
	}
	now := time.Now().UTC()
	return agentqualification.RoleQualificationPreflightV5{SchemaVersion: "silver-mediums.role-qualification-preflight.v5", ExecutionFreezeSHA256: agentqualification.RoleQualificationExecutionFreezeV5SHA256(), V3FreezeSHA256: agentqualification.RoleQualificationExecutionFreezeV3SHA256(), ConfigurationClassID: classID, RequestedModel: cell.ConfigurationClass.Model, RequestedProvider: cell.ConfigurationClass.ServingProvider, RequestedEffort: cell.ConfigurationClass.Effort, Transport: transportForCellV5(cell), RuntimeVersion: p.runtimeVersion, RuntimeExecutablePath: p.executable, RuntimeExecutableSHA256: p.executableHash, ResolvedConfigSHA256: p.resolvedHash, ExactRouteValidated: true, FallbacksDisabled: true, CredentialState: credential, OpenRouterSpendBaselineMicros: baseline, ObservedAt: now.Format(time.RFC3339Nano), Ready: true}, nil
}

func (p *liveProviderV5) validateAgainstSealedPreflight(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "preflight", p.classID+".json"))
	if err != nil {
		return err
	}
	var proof agentqualification.RoleQualificationPreflightV5
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proof); err != nil {
		return err
	}
	if proof.RuntimeExecutablePath != p.executable || proof.RuntimeExecutableSHA256 != p.executableHash || proof.RuntimeVersion != p.runtimeVersion || proof.ResolvedConfigSHA256 != p.resolvedHash {
		return errors.New("live runtime differs from sealed fresh preflight")
	}
	return agentqualification.ValidateRoleQualificationPreflightV5(proof, p.classID, time.Now().UTC())
}

func newLiveProviderV5(ctx context.Context, classID string) (*liveProviderV5, error) {
	cells, err := agentqualification.RoleQualificationClassCellsV4(classID)
	if err != nil {
		return nil, err
	}
	cell := cells[0]
	p := &liveProviderV5{classID: classID, client: &http.Client{Timeout: 120 * time.Second}}
	if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		name := "claude"
		if runtime.GOOS == "windows" {
			name = "claude.cmd"
		}
		p.executable, err = exec.LookPath(name)
		if err != nil {
			return nil, fmt.Errorf("Claude credential/runtime unavailable: %w", err)
		}
		raw, err := os.ReadFile(p.executable)
		if err != nil {
			return nil, err
		}
		p.executableHash = hashBytesV5(raw)
		out, err := exec.CommandContext(ctx, p.executable, "--version").Output()
		if err != nil {
			return nil, err
		}
		p.runtimeVersion = strings.TrimSpace(string(out))
		auth := exec.CommandContext(ctx, p.executable, "auth", "status", "--json")
		authRaw, err := auth.Output()
		var authState struct {
			LoggedIn bool `json:"loggedIn"`
		}
		if err != nil || json.Unmarshal(authRaw, &authState) != nil || !authState.LoggedIn {
			return nil, errors.New("Claude credential state is not authenticated")
		}
		p.resolvedHash = hashBytesV5([]byte("claude_code_subscription_tool_free:no_fallback:" + p.runtimeVersion))
		p.workspace, err = os.MkdirTemp("", "rq-v5-claude-work-")
		if err != nil {
			return nil, err
		}
		return p, nil
	}
	p.executable = os.Getenv("OPENCODE_V33_EXECUTABLE")
	if p.executable == "" {
		return nil, errors.New("OPENCODE_V33_EXECUTABLE required")
	}
	raw, err := os.ReadFile(p.executable)
	if err != nil {
		return nil, err
	}
	p.executableHash = hashBytesV5(raw)
	if p.executableHash != agentqualification.OpenCodeStructuredV33ExecutableSHA256 {
		return nil, errors.New("OpenCode executable hash mismatch")
	}
	p.runtimeVersion = agentqualification.OpenCodeStructuredV33Version
	p.config, err = inlineConfigV5(cell)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "rq-v5-config-")
	if err != nil {
		return nil, err
	}
	p.configDir = dir
	workspace, err := os.MkdirTemp("", "rq-v5-work-")
	if err != nil {
		p.Close()
		return nil, err
	}
	p.workspace = workspace
	cmd := exec.CommandContext(ctx, p.executable, "--pure", "debug", "config")
	cmd.Dir = p.workspace
	cmd.Env = opencodeEnvV5(p.config, p.configDir)
	resolved, err := cmd.Output()
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("OpenCode resolved config: %w", err)
	}
	plan := openCodePlanV5(cell, p.config)
	p.resolvedHash, err = agentqualification.ValidateOpenCodeResolvedConfig(plan, resolved)
	if err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (p *liveProviderV5) Call(ctx context.Context, cell agentqualification.RoleQualificationExecutionCellV3) agentqualification.RoleQualificationProviderResultV5 {
	started := time.Now().UTC()
	p.providerCallStarted = false
	p.transportStage = agentqualification.RoleQualificationStagePreProvider
	p.transportCause = agentqualification.RoleQualificationCauseRuntime
	p.httpStatus, p.contentType, p.partialBody, p.readError = 0, "", nil, ""
	if cell.ConfigurationClass.ClassID != p.classID {
		p.transportCause = agentqualification.RoleQualificationCauseRoute
		return transportFailureResultV5(started, p, errors.New("provider class substitution"))
	}
	var raw json.RawMessage
	var err error
	if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		p.providerCallStarted = true
		p.transportStage = agentqualification.RoleQualificationStageProvider
		raw, err = p.callClaude(ctx, cell)
	} else {
		raw, err = p.callOpenCode(ctx, cell)
	}
	if err != nil {
		return transportFailureResultV5(started, p, err)
	}
	return agentqualification.RoleQualificationProviderResultV5{RawResponse: raw}
}

func transportFailureResultV5(started time.Time, p *liveProviderV5, err error) agentqualification.RoleQualificationProviderResultV5 {
	cause := p.transportCause
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		cause = agentqualification.RoleQualificationCauseTimeout
	} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		cause = agentqualification.RoleQualificationCauseTimeout
	} else if _, ok := err.(net.Error); ok {
		cause = agentqualification.RoleQualificationCauseConnection
	}
	completed := time.Now().UTC()
	f := &agentqualification.RoleQualificationTransportFailureV5{Stage: p.transportStage, Cause: cause, ProviderCallStarted: p.providerCallStarted, UsageKnown: !p.providerCallStarted, StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano), ErrorDetail: err.Error(), ReportedUsage: agentqualification.RoleQualificationUsageV3{WallMillis: completed.Sub(started).Milliseconds()}, TimingKnown: true, HTTPStatus: p.httpStatus, ContentType: p.contentType, PartialBody: append([]byte(nil), p.partialBody...), ReadError: p.readError}
	return agentqualification.RoleQualificationProviderResultV5{TransportFailure: f}
}
func (p *liveProviderV5) callClaude(ctx context.Context, cell agentqualification.RoleQualificationExecutionCellV3) (json.RawMessage, error) {
	prompt, schema, err := promptAndSchemaV5(cell)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseConfig
		return nil, err
	}
	args := []string{"-p", "--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--no-session-persistence", "--permission-prompts", "none", "--output-format", "json", "--json-schema", string(schema), "--max-turns", "1", "--model", cell.ConfigurationClass.Model, "--effort", cell.ConfigurationClass.Effort}
	started := time.Now().UTC()
	cmd := exec.CommandContext(ctx, p.executable, args...)
	cmd.Dir = p.workspace
	if cmd.Dir == "" {
		cmd.Dir = os.TempDir()
	}
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	completed := time.Now().UTC()
	if runErr != nil {
		p.transportCause = agentqualification.RoleQualificationCauseRuntime
		return nil, fmt.Errorf("Claude transport failure: %w: %s", runErr, stderr.String())
	}
	result, structured, err := agentqualification.ParseClaudeCodeSubscriptionJSONArtifact(stdout.Bytes(), started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano))
	if err != nil {
		p.transportStage = agentqualification.RoleQualificationStageParse
		p.transportCause = agentqualification.RoleQualificationCauseMalformed
		p.contentType = "application/json"
		p.partialBody = append([]byte(nil), stdout.Bytes()...)
		return nil, err
	}
	if result.ReportedModel != cell.ConfigurationClass.Model {
		p.transportCause = agentqualification.RoleQualificationCauseModel
		p.partialBody = append([]byte(nil), stdout.Bytes()...)
		return nil, errors.New("Claude actual model drift")
	}
	input, output, cost := int64(0), int64(0), int64(0)
	if result.Usage.InputTokens != nil {
		input = *result.Usage.InputTokens
	}
	if result.Usage.OutputTokens != nil {
		output = *result.Usage.OutputTokens
	}
	if result.Usage.ProviderReportedCostMicros != nil {
		cost = *result.Usage.ProviderReportedCostMicros
	}
	u := agentqualification.RoleQualificationUsageV3{Calls: 1, InputTokens: input, OutputTokens: output, TotalTokens: input + output, ClaudeSubscriptionCostEquivalentMicros: cost, WallMillis: completed.Sub(started).Milliseconds()}
	return rawEnvelopeV5(cell, result.ProviderResponseID, structured, "", u, started, completed), nil
}

func (p *liveProviderV5) callOpenCode(ctx context.Context, cell agentqualification.RoleQualificationExecutionCellV3) (json.RawMessage, error) {
	p.transportStage = agentqualification.RoleQualificationStagePreProvider
	p.transportCause = agentqualification.RoleQualificationCauseRuntime
	if err := p.startOpenCode(ctx); err != nil {
		return nil, err
	}
	defer p.stopOpenCode()
	started := time.Now().UTC()
	sessionReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/session?directory="+url.QueryEscape(p.workspace), strings.NewReader(`{"title":"role-qualification-v5"}`))
	sessionReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(sessionReq)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseConnection
		return nil, err
	}
	sessionRaw, readErr := io.ReadAll(resp.Body)
	p.httpStatus, p.contentType, p.partialBody = resp.StatusCode, resp.Header.Get("Content-Type"), append([]byte(nil), sessionRaw...)
	resp.Body.Close()
	if readErr != nil {
		p.transportStage = agentqualification.RoleQualificationStageResponse
		p.transportCause = agentqualification.RoleQualificationCauseRead
		p.readError = readErr.Error()
		return nil, readErr
	}
	session, err := agentqualification.ParseOpenCodeStructuredV33CreatedSession(resp.StatusCode, resp.Header.Get("Content-Type"), sessionRaw)
	if err != nil {
		p.transportStage = agentqualification.RoleQualificationStageParse
		p.transportCause = agentqualification.RoleQualificationCauseMalformed
		return nil, err
	}
	prompt, schema, err := promptAndSchemaV5(cell)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseConfig
		return nil, err
	}
	messageID := "msg_rqv5_" + hashBytesV5([]byte(cell.CellID + started.String()))[:20]
	model := strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/")
	body, _ := json.Marshal(map[string]any{"messageID": messageID, "model": map[string]string{"providerID": "openrouter", "modelID": model}, "agent": "qualification", "tools": map[string]bool{"*": false}, "format": map[string]any{"type": "json_schema", "schema": json.RawMessage(schema), "retryCount": 0}, "variant": cell.ConfigurationClass.Effort, "parts": []map[string]string{{"type": "text", "text": prompt}}})
	capsule := agentqualification.OpenCodeStructuredV33Capsule{SessionID: session.ID, RequestMessageID: messageID, Workspace: p.workspace, ResolvedConfigSHA256: p.resolvedHash, RoutePolicy: "enforced_single_route_no_fallback"}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/session/"+url.PathEscape(session.ID)+"/message?directory="+url.QueryEscape(p.workspace), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	p.providerCallStarted = true
	p.transportStage = agentqualification.RoleQualificationStageProvider
	response, err := p.client.Do(req)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseConnection
		return nil, err
	}
	raw, readErr := io.ReadAll(response.Body)
	p.httpStatus, p.contentType, p.partialBody = response.StatusCode, response.Header.Get("Content-Type"), append([]byte(nil), raw...)
	response.Body.Close()
	if readErr != nil {
		p.transportStage = agentqualification.RoleQualificationStageResponse
		p.transportCause = agentqualification.RoleQualificationCauseRead
		p.readError = readErr.Error()
		return nil, readErr
	}
	completed := time.Now().UTC()
	p.transportStage = agentqualification.RoleQualificationStageParse
	receipt, err := agentqualification.ParseOpenCodeStructuredV33HTTPResponse(capsule, response.StatusCode, response.Header.Get("Content-Type"), raw)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseMalformed
		return nil, err
	}
	if receipt.ProviderID != "openrouter" {
		p.transportCause = agentqualification.RoleQualificationCauseProvider
		return nil, errors.New("OpenCode provider drift")
	}
	if receipt.ModelID != model {
		p.transportCause = agentqualification.RoleQualificationCauseModel
		return nil, errors.New("OpenCode model drift")
	}
	if receipt.Variant != cell.ConfigurationClass.Effort {
		p.transportCause = agentqualification.RoleQualificationCauseRoute
		return nil, errors.New("OpenCode effort drift")
	}
	if receipt.ProviderTurns != 1 {
		p.transportCause = agentqualification.RoleQualificationCauseTurn
		return nil, errors.New("OpenCode turn drift")
	}
	failure := ""
	structured := receipt.Structured
	if receipt.Outcome != "completed" {
		failure = "response_or_tool_protocol_failure"
		structured = nil
	}
	u := agentqualification.RoleQualificationUsageV3{Calls: 1, InputTokens: receipt.Usage.InputTokens, OutputTokens: receipt.Usage.OutputTokens, ReasoningTokens: receipt.Usage.ReasoningTokens, TotalTokens: receipt.Usage.InputTokens + receipt.Usage.OutputTokens + receipt.Usage.ReasoningTokens, OpenRouterCashMicros: receipt.Usage.CostMicros, WallMillis: completed.Sub(started).Milliseconds()}
	return rawEnvelopeV5(cell, receipt.AssistantMessageID, structured, failure, u, started, completed), nil
}

func (p *liveProviderV5) startOpenCode(ctx context.Context) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	p.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	p.command = exec.CommandContext(ctx, p.executable, "serve", "--pure", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port))
	p.command.Dir = p.workspace
	p.command.Env = append(opencodeEnvV5(p.config, p.configDir), "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000")
	if err := p.command.Start(); err != nil {
		return err
	}
	for until := time.Now().Add(15 * time.Second); time.Now().Before(until); {
		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, p.baseURL+"/doc", nil)
		resp, err := p.client.Do(req)
		cancel()
		if err == nil {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 && hashBytesV5(raw) == agentqualification.OpenCodeStructuredV33OpenAPISHA256 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("OpenCode readiness failed")
}
func (p *liveProviderV5) stopOpenCode() {
	if p.command == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/global/dispose", nil)
	if resp, err := p.client.Do(req); err == nil {
		resp.Body.Close()
	}
	done := make(chan error, 1)
	go func() { done <- p.command.Wait() }()
	select {
	case <-done:
	case <-ctx.Done():
		_ = p.command.Process.Kill()
		<-done
	}
	p.command = nil
}
func (p *liveProviderV5) Close() error {
	p.stopOpenCode()
	if p.configDir != "" {
		_ = os.RemoveAll(p.configDir)
	}
	if p.workspace != "" {
		_ = os.RemoveAll(p.workspace)
	}
	return nil
}

func promptAndSchemaV5(cell agentqualification.RoleQualificationExecutionCellV3) (string, []byte, error) {
	schema, err := json.Marshal(cell.Request.CandidatePacket.ResponseSchema)
	if err != nil {
		return "", nil, err
	}
	packet, err := json.Marshal(cell.Request.CandidatePacket)
	if err != nil {
		return "", nil, err
	}
	return "Follow the role instructions and skill. Answer the scenario using only the exact typed JSON schema.\n<packet>\n" + string(packet) + "\n</packet>\n", schema, nil
}
func rawEnvelopeV5(cell agentqualification.RoleQualificationExecutionCellV3, id string, structured json.RawMessage, failure string, u agentqualification.RoleQualificationUsageV3, started, completed time.Time) json.RawMessage {
	p := agentqualification.RoleQualificationRawProviderResponseV3{SchemaVersion: "silver-mediums.role-qualification-raw-provider-response.v3", ProviderCallID: id, ActualModel: cell.ConfigurationClass.Model, ActualProvider: cell.ConfigurationClass.ServingProvider, ActualEffort: cell.ConfigurationClass.Effort, ActualTransport: transportForCellV5(cell), FallbackUsed: false, RetryCount: 0, StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano), StructuredOutput: structured, FailureClass: failure, Usage: u}
	raw, _ := json.Marshal(p)
	return raw
}
func transportForCellV5(c agentqualification.RoleQualificationExecutionCellV3) string {
	if c.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		return agentqualification.RoleQualificationClaudeTransportV3
	}
	return agentqualification.RoleQualificationOpenCodeTransportV3
}
func inlineConfigV5(cell agentqualification.RoleQualificationExecutionCellV3) ([]byte, error) {
	model := strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/")
	return json.Marshal(map[string]any{"$schema": "https://opencode.ai/config.json", "autoupdate": false, "share": "disabled", "snapshot": false, "plugin": []string{}, "instructions": []string{}, "permission": "deny", "tools": map[string]bool{"*": false}, "compaction": map[string]bool{"auto": false}, "default_agent": "qualification", "agent": map[string]any{"qualification": map[string]any{"description": "Frozen role qualification", "mode": "primary", "model": cell.ConfigurationClass.Model, "variant": cell.ConfigurationClass.Effort, "prompt": "", "steps": 2, "permission": "deny", "tools": map[string]bool{"*": false}}}, "provider": map[string]any{"openrouter": map[string]any{"options": map[string]any{"name": "OpenRouter"}, "whitelist": []string{model}, "models": map[string]any{model: map[string]any{"options": map[string]any{"provider": map[string]any{"allow_fallbacks": false, "order": []string{cell.ConfigurationClass.ServingProvider}}}}}}}})
}
func openCodePlanV5(cell agentqualification.RoleQualificationExecutionCellV3, config []byte) agentqualification.OpenCodeScreeningPlan {
	no := false
	return agentqualification.OpenCodeScreeningPlan{Transport: agentqualification.OpenCodeToolFreeTransportV6, CandidateID: cell.ConfigurationClass.ClassID, CaseID: cell.CaseID, CaseVersion: "role-v1", Model: cell.ConfigurationClass.Model, Effort: cell.ConfigurationClass.Effort, InlineConfig: config, InlineConfigSHA256: hashBytesV5(config), ServingProvider: cell.ConfigurationClass.ServingProvider, ProviderFallbacks: &no}
}
func opencodeEnvV5(config []byte, dir string) []string {
	return append(os.Environ(), "OPENCODE_CONFIG_CONTENT="+string(config), "OPENCODE_CONFIG_DIR="+dir, "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true")
}
func readOpenRouterSpendV5(ctx context.Context) (int64, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(filepath.Join(home, ".local", "share", "opencode", "auth.json"))
	if err != nil {
		return 0, err
	}
	var auth map[string]struct {
		Type string `json:"type"`
		Key  string `json:"key"`
	}
	if json.Unmarshal(raw, &auth) != nil || auth["openrouter"].Type != "api" || strings.TrimSpace(auth["openrouter"].Key) == "" {
		return 0, errors.New("OpenRouter credential unavailable")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, openRouterCreditsURLV5, nil)
	req.Header.Set("Authorization", "Bearer "+auth["openrouter"].Key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("OpenRouter credits status %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			TotalUsage json.Number `json:"total_usage"`
		} `json:"data"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if dec.Decode(&out) != nil {
		return 0, errors.New("OpenRouter credits invalid")
	}
	return usdMicrosV5(out.Data.TotalUsage.String())
}
func usdMicrosV5(s string) (int64, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() < 0 {
		return 0, errors.New("invalid USD")
	}
	r.Mul(r, big.NewRat(1_000_000, 1))
	q := new(big.Int).Quo(r.Num(), r.Denom())
	if new(big.Int).Mod(r.Num(), r.Denom()).Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, errors.New("USD overflow")
	}
	return q.Int64(), nil
}
func hashBytesV5(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
