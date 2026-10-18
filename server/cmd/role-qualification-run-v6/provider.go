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
	"strings"
	"time"
)

const openRouterCreditsURLV6 = "https://openrouter.ai/api/v1/credits"

type liveProviderV6 struct {
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
	transportStage      agentqualification.RoleQualificationTransportStageV6
	transportCause      agentqualification.RoleQualificationTransportCauseV6
	httpStatus          int
	contentType         string
	partialBody         []byte
	readError           string
}

func runLivePreflightV6(ctx context.Context, classID string) (agentqualification.RoleQualificationPreflightV6, error) {
	cells, err := agentqualification.RoleQualificationClassCellsV4(classID)
	if err != nil {
		return agentqualification.RoleQualificationPreflightV6{}, err
	}
	p, err := newLiveProviderV6(ctx, classID)
	if err != nil {
		return agentqualification.RoleQualificationPreflightV6{}, err
	}
	defer p.Close()
	cell := cells[0]
	if cell.ConfigurationClass.RuntimeProvider != "claude_code_subscription" {
		if err := p.startOpenCode(ctx); err != nil {
			return agentqualification.RoleQualificationPreflightV6{}, err
		}
		p.stopOpenCode()
	}
	baseline := int64(0)
	credential := "authenticated"
	if cell.ConfigurationClass.RuntimeProvider != "claude_code_subscription" {
		baseline, err = readOpenRouterSpendV6(ctx)
		if err != nil {
			return agentqualification.RoleQualificationPreflightV6{}, err
		}
	}
	now := time.Now().UTC()
	return agentqualification.RoleQualificationPreflightV6{SchemaVersion: "silver-mediums.role-qualification-preflight.v6", ExecutionFreezeSHA256: agentqualification.RoleQualificationExecutionFreezeV6SHA256(), V3FreezeSHA256: agentqualification.RoleQualificationExecutionFreezeV3SHA256(), ConfigurationClassID: classID, RequestedModel: cell.ConfigurationClass.Model, RequestedProvider: cell.ConfigurationClass.ServingProvider, RequestedEffort: cell.ConfigurationClass.Effort, Transport: transportForCellV6(cell), RuntimeVersion: p.runtimeVersion, RuntimeExecutablePath: p.executable, RuntimeExecutableSHA256: p.executableHash, ResolvedConfigSHA256: p.resolvedHash, ExactRouteValidated: true, FallbacksDisabled: true, CredentialState: credential, OpenRouterSpendBaselineMicros: baseline, ObservedAt: now.Format(time.RFC3339Nano), Ready: true}, nil
}

func (p *liveProviderV6) validateAgainstSealedPreflight(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "preflight", p.classID+".json"))
	if err != nil {
		return err
	}
	var proof agentqualification.RoleQualificationPreflightV6
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proof); err != nil {
		return err
	}
	if proof.RuntimeExecutablePath != p.executable || proof.RuntimeExecutableSHA256 != p.executableHash || proof.RuntimeVersion != p.runtimeVersion || proof.ResolvedConfigSHA256 != p.resolvedHash {
		return errors.New("live runtime differs from sealed fresh preflight")
	}
	return agentqualification.ValidateRoleQualificationPreflightV6(proof, p.classID, time.Now().UTC())
}

func newLiveProviderV6(ctx context.Context, classID string) (*liveProviderV6, error) {
	cells, err := agentqualification.RoleQualificationClassCellsV4(classID)
	if err != nil {
		return nil, err
	}
	cell := cells[0]
	p := &liveProviderV6{classID: classID, client: &http.Client{Timeout: 120 * time.Second}}
	if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		return nil, errors.New("v6 Claude subscription class held incomplete: multi-model modelUsage route evidence unresolved")
	}
	p.executable = os.Getenv("OPENCODE_V33_EXECUTABLE")
	if p.executable == "" {
		return nil, errors.New("OPENCODE_V33_EXECUTABLE required")
	}
	raw, err := os.ReadFile(p.executable)
	if err != nil {
		return nil, err
	}
	p.executableHash = hashBytesV6(raw)
	if p.executableHash != agentqualification.OpenCodeStructuredV33ExecutableSHA256 {
		return nil, errors.New("OpenCode executable hash mismatch")
	}
	p.runtimeVersion = agentqualification.OpenCodeStructuredV33Version
	p.config, err = inlineConfigV6(cell)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "rq-v6-config-")
	if err != nil {
		return nil, err
	}
	p.configDir = dir
	workspace, err := os.MkdirTemp("", "rq-v6-work-")
	if err != nil {
		p.Close()
		return nil, err
	}
	p.workspace = workspace
	cmd := exec.CommandContext(ctx, p.executable, "--pure", "debug", "config")
	cmd.Dir = p.workspace
	cmd.Env = opencodeEnvV6(p.config, p.configDir)
	resolved, err := cmd.Output()
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("OpenCode resolved config: %w", err)
	}
	plan := openCodePlanV6(cell, p.config)
	p.resolvedHash, err = agentqualification.ValidateOpenCodeResolvedConfig(plan, resolved)
	if err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (p *liveProviderV6) Call(ctx context.Context, cell agentqualification.RoleQualificationExecutionCellV3) agentqualification.RoleQualificationProviderResultV6 {
	started := time.Now().UTC()
	p.providerCallStarted = false
	p.transportStage = agentqualification.RoleQualificationStagePreProviderV6
	p.transportCause = agentqualification.RoleQualificationCauseRuntimeV6
	p.httpStatus, p.contentType, p.partialBody, p.readError = 0, "", nil, ""
	if cell.ConfigurationClass.ClassID != p.classID {
		p.transportCause = agentqualification.RoleQualificationCauseRouteV6
		return transportFailureResultV6(started, p, errors.New("provider class substitution"))
	}
	var raw json.RawMessage
	var evidence *agentqualification.RoleQualificationOpenCodeEvidenceV6
	var err error
	if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		return transportFailureResultV6(started, p, errors.New("v6 Claude subscription class held incomplete"))
	}
	raw, evidence, err = p.callOpenCode(ctx, cell)
	if err != nil {
		return transportFailureResultV6(started, p, err)
	}
	return agentqualification.RoleQualificationProviderResultV6{RawResponse: raw, OpenCodeEvidence: evidence}
}

func transportFailureResultV6(started time.Time, p *liveProviderV6, err error) agentqualification.RoleQualificationProviderResultV6 {
	cause := p.transportCause
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		cause = agentqualification.RoleQualificationCauseTimeoutV6
	} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		cause = agentqualification.RoleQualificationCauseTimeoutV6
	} else if _, ok := err.(net.Error); ok {
		cause = agentqualification.RoleQualificationCauseConnectionV6
	}
	completed := time.Now().UTC()
	f := &agentqualification.RoleQualificationTransportFailureV6{Stage: p.transportStage, Cause: cause, ProviderCallStarted: p.providerCallStarted, UsageKnown: !p.providerCallStarted, StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano), ErrorDetail: err.Error(), ReportedUsage: agentqualification.RoleQualificationUsageV3{WallMillis: completed.Sub(started).Milliseconds()}, TimingKnown: true, HTTPStatus: p.httpStatus, ContentType: p.contentType, PartialBody: append([]byte(nil), p.partialBody...), ReadError: p.readError}
	return agentqualification.RoleQualificationProviderResultV6{TransportFailure: f}
}
func (p *liveProviderV6) callOpenCode(ctx context.Context, cell agentqualification.RoleQualificationExecutionCellV3) (json.RawMessage, *agentqualification.RoleQualificationOpenCodeEvidenceV6, error) {
	p.transportStage = agentqualification.RoleQualificationStagePreProviderV6
	p.transportCause = agentqualification.RoleQualificationCauseRuntimeV6
	if err := p.startOpenCode(ctx); err != nil {
		return nil, nil, err
	}
	defer p.stopOpenCode()
	started := time.Now().UTC()
	sessionReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/session?directory="+url.QueryEscape(p.workspace), strings.NewReader(`{"title":"role-qualification-v6"}`))
	sessionReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(sessionReq)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseConnectionV6
		return nil, nil, err
	}
	sessionRaw, readErr := io.ReadAll(resp.Body)
	p.httpStatus, p.contentType, p.partialBody = resp.StatusCode, resp.Header.Get("Content-Type"), append([]byte(nil), sessionRaw...)
	resp.Body.Close()
	if readErr != nil {
		p.transportStage = agentqualification.RoleQualificationStageResponseV6
		p.transportCause = agentqualification.RoleQualificationCauseReadV6
		p.readError = readErr.Error()
		return nil, nil, readErr
	}
	session, err := agentqualification.ParseOpenCodeStructuredV33CreatedSession(resp.StatusCode, resp.Header.Get("Content-Type"), sessionRaw)
	if err != nil {
		p.transportStage = agentqualification.RoleQualificationStageParseV6
		p.transportCause = agentqualification.RoleQualificationCauseMalformedV6
		return nil, nil, err
	}
	prompt, schema, err := promptAndSchemaV6(cell)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseConfigV6
		return nil, nil, err
	}
	messageID := "msg_rqv6_" + hashBytesV6([]byte(cell.CellID + started.String()))[:20]
	model := strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/")
	body, _ := json.Marshal(map[string]any{"messageID": messageID, "model": map[string]string{"providerID": "openrouter", "modelID": model}, "agent": "qualification", "tools": map[string]bool{"*": false}, "format": map[string]any{"type": "json_schema", "schema": json.RawMessage(schema), "retryCount": 0}, "variant": cell.ConfigurationClass.Effort, "parts": []map[string]string{{"type": "text", "text": prompt}}})
	capsule := agentqualification.OpenCodeStructuredV33Capsule{BindingID: cell.CellID, OpenCodeVersion: agentqualification.OpenCodeStructuredV33Version, Transport: agentqualification.OpenCodeStructuredV33Transport, Method: "POST", PathTemplate: "/session/{sessionID}/message?directory={url-escaped-workspace}", SessionID: session.ID, RequestMessageID: messageID, Workspace: p.workspace, ServerArgs: []string{"serve", "--pure", "--hostname", "127.0.0.1", "--port", "<ephemeral-port>"}, EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000"}, InlineConfig: append(json.RawMessage(nil), p.config...), RequestBody: append(json.RawMessage(nil), body...), InlineConfigSHA256: hashBytesV6(p.config), ResolvedConfigSHA256: p.resolvedHash, RequestBodySHA256: hashBytesV6(body), PromptSHA256: hashBytesV6([]byte(prompt)), SchemaSHA256: hashBytesV6(schema), RoutePolicy: "enforced_single_route_no_fallback", RetryCount: 0, HTTPTimeoutSeconds: agentqualification.OpenCodeStructuredV33HTTPTimeoutSeconds, GeneratedTokenCeiling: cell.Request.MaxOutputTokens, ToolsDenied: true, CLIFormatJSONEnforcement: false}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/session/"+url.PathEscape(session.ID)+"/message?directory="+url.QueryEscape(p.workspace), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	p.providerCallStarted = true
	p.transportStage = agentqualification.RoleQualificationStageProviderV6
	response, err := p.client.Do(req)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseConnectionV6
		return nil, nil, err
	}
	raw, readErr := io.ReadAll(response.Body)
	p.httpStatus, p.contentType, p.partialBody = response.StatusCode, response.Header.Get("Content-Type"), append([]byte(nil), raw...)
	response.Body.Close()
	if readErr != nil {
		p.transportStage = agentqualification.RoleQualificationStageResponseV6
		p.transportCause = agentqualification.RoleQualificationCauseReadV6
		p.readError = readErr.Error()
		return nil, nil, readErr
	}
	completed := time.Now().UTC()
	p.transportStage = agentqualification.RoleQualificationStageParseV6
	receipt, err := agentqualification.ParseOpenCodeStructuredV33HTTPResponse(capsule, response.StatusCode, response.Header.Get("Content-Type"), raw)
	if err != nil {
		p.transportCause = agentqualification.RoleQualificationCauseMalformedV6
		return nil, nil, err
	}
	if receipt.ProviderID != "openrouter" {
		p.transportCause = agentqualification.RoleQualificationCauseProviderV6
		return nil, nil, errors.New("OpenCode provider drift")
	}
	if receipt.ModelID != model {
		p.transportCause = agentqualification.RoleQualificationCauseModelV6
		return nil, nil, errors.New("OpenCode model drift")
	}
	if receipt.Variant != cell.ConfigurationClass.Effort {
		p.transportCause = agentqualification.RoleQualificationCauseRouteV6
		return nil, nil, errors.New("OpenCode effort drift")
	}
	if receipt.ProviderTurns != 1 {
		p.transportCause = agentqualification.RoleQualificationCauseTurnV6
		return nil, nil, errors.New("OpenCode turn drift")
	}
	failure := ""
	structured := receipt.Structured
	if !isOpenCodeStructuredSuccessV6(receipt.Outcome) {
		failure = "response_or_tool_protocol_failure"
		structured = nil
	}
	u := agentqualification.RoleQualificationUsageV3{Calls: 1, InputTokens: receipt.Usage.InputTokens, OutputTokens: receipt.Usage.OutputTokens, ReasoningTokens: receipt.Usage.ReasoningTokens, TotalTokens: receipt.Usage.InputTokens + receipt.Usage.OutputTokens + receipt.Usage.ReasoningTokens, OpenRouterCashMicros: receipt.Usage.CostMicros, WallMillis: completed.Sub(started).Milliseconds()}
	rawEnvelope := rawEnvelopeV6(cell, receipt.AssistantMessageID, structured, failure, u, started, completed)
	capsuleRaw, _ := json.Marshal(capsule)
	evidence := &agentqualification.RoleQualificationOpenCodeEvidenceV6{SchemaVersion: "silver-mediums.role-qualification-opencode-http-evidence.v6", CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, Capsule: capsule, CapsuleSHA256: hashBytesV6(capsuleRaw), HTTPStatus: response.StatusCode, ContentType: response.Header.Get("Content-Type"), RawHTTPResponse: append(json.RawMessage(nil), raw...), RawHTTPResponseSHA: hashBytesV6(raw), StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano), SealedAt: completed.Format(time.RFC3339Nano)}
	return rawEnvelope, evidence, nil
}

func isOpenCodeStructuredSuccessV6(outcome string) bool { return outcome == "structured" }

func (p *liveProviderV6) startOpenCode(ctx context.Context) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	p.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	p.command = exec.CommandContext(ctx, p.executable, "serve", "--pure", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port))
	p.command.Dir = p.workspace
	p.command.Env = append(opencodeEnvV6(p.config, p.configDir), "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000")
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
			if resp.StatusCode == 200 && hashBytesV6(raw) == agentqualification.OpenCodeStructuredV33OpenAPISHA256 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("OpenCode readiness failed")
}
func (p *liveProviderV6) stopOpenCode() {
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
func (p *liveProviderV6) Close() error {
	p.stopOpenCode()
	if p.configDir != "" {
		_ = os.RemoveAll(p.configDir)
	}
	if p.workspace != "" {
		_ = os.RemoveAll(p.workspace)
	}
	return nil
}

func promptAndSchemaV6(cell agentqualification.RoleQualificationExecutionCellV3) (string, []byte, error) {
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
func rawEnvelopeV6(cell agentqualification.RoleQualificationExecutionCellV3, id string, structured json.RawMessage, failure string, u agentqualification.RoleQualificationUsageV3, started, completed time.Time) json.RawMessage {
	p := agentqualification.RoleQualificationRawProviderResponseV3{SchemaVersion: "silver-mediums.role-qualification-raw-provider-response.v3", ProviderCallID: id, ActualModel: cell.ConfigurationClass.Model, ActualProvider: cell.ConfigurationClass.ServingProvider, ActualEffort: cell.ConfigurationClass.Effort, ActualTransport: transportForCellV6(cell), FallbackUsed: false, RetryCount: 0, StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano), StructuredOutput: structured, FailureClass: failure, Usage: u}
	raw, _ := json.Marshal(p)
	return raw
}
func transportForCellV6(c agentqualification.RoleQualificationExecutionCellV3) string {
	if c.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		return agentqualification.RoleQualificationClaudeTransportV3
	}
	return agentqualification.RoleQualificationOpenCodeTransportV3
}
func inlineConfigV6(cell agentqualification.RoleQualificationExecutionCellV3) ([]byte, error) {
	model := strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/")
	return json.Marshal(map[string]any{"$schema": "https://opencode.ai/config.json", "autoupdate": false, "share": "disabled", "snapshot": false, "plugin": []string{}, "instructions": []string{}, "permission": "deny", "tools": map[string]bool{"*": false}, "compaction": map[string]bool{"auto": false}, "default_agent": "qualification", "agent": map[string]any{"qualification": map[string]any{"description": "Frozen role qualification", "mode": "primary", "model": cell.ConfigurationClass.Model, "variant": cell.ConfigurationClass.Effort, "prompt": "", "steps": 2, "permission": "deny", "tools": map[string]bool{"*": false}}}, "provider": map[string]any{"openrouter": map[string]any{"options": map[string]any{"name": "OpenRouter"}, "whitelist": []string{model}, "models": map[string]any{model: map[string]any{"options": map[string]any{"provider": map[string]any{"allow_fallbacks": false, "order": []string{cell.ConfigurationClass.ServingProvider}}}}}}}})
}
func openCodePlanV6(cell agentqualification.RoleQualificationExecutionCellV3, config []byte) agentqualification.OpenCodeScreeningPlan {
	no := false
	return agentqualification.OpenCodeScreeningPlan{Transport: agentqualification.OpenCodeToolFreeTransportV6, CandidateID: cell.ConfigurationClass.ClassID, CaseID: cell.CaseID, CaseVersion: "role-v1", Model: cell.ConfigurationClass.Model, Effort: cell.ConfigurationClass.Effort, InlineConfig: config, InlineConfigSHA256: hashBytesV6(config), ServingProvider: cell.ConfigurationClass.ServingProvider, ProviderFallbacks: &no}
}
func opencodeEnvV6(config []byte, dir string) []string {
	return append(os.Environ(), "OPENCODE_CONFIG_CONTENT="+string(config), "OPENCODE_CONFIG_DIR="+dir, "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true")
}
func readOpenRouterSpendV6(ctx context.Context) (int64, error) {
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, openRouterCreditsURLV6, nil)
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
	return usdMicrosV6(out.Data.TotalUsage.String())
}
func usdMicrosV6(s string) (int64, error) {
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
func hashBytesV6(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
