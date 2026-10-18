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
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const v33PaidConfirmation = "YES_I_ACCEPT_V33_MAX_1_805_USD"

const v33ReadinessRequestTimeout = 2 * time.Second

type v33Provider struct {
	d                                                     agentqualification.ProtocolAblationV32Definition
	b                                                     agentqualification.ProtocolAblationV32Binding
	workspace, artifact, resolvedHash, baseURL, configDir string
	executable                                            string
	baseEnv                                               []string
	currentCeiling                                        int64
	usage                                                 agentqualification.ProtocolAblationV32Usage
	identity                                              agentqualification.ProtocolAblationV32ProviderIdentity
	rawReceipts                                           []json.RawMessage
	call                                                  int
	client                                                *http.Client
	command                                               *exec.Cmd
}

func executeV33(ctx context.Context, root string) (agentqualification.ProtocolAblationV32Ledger, error) {
	var ledger agentqualification.ProtocolAblationV32Ledger
	if os.Getenv("MULTICA_V33_EXECUTE_PAID") != v33PaidConfirmation {
		return ledger, errors.New("v33 paid execution is frozen but not confirmed")
	}
	if err := verifyV33CommandFreeze(); err != nil {
		return ledger, err
	}
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return ledger, err
	}
	order, err := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	if err != nil || len(order) != 4 {
		return ledger, fmt.Errorf("v33 exact four-cell order unavailable: %w", err)
	}
	ledgerPath := filepath.Join(root, "ledger.json")
	if raw, e := os.ReadFile(ledgerPath); e == nil {
		if json.Unmarshal(raw, &ledger) != nil {
			return ledger, errors.New("v33 ledger corrupt")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return ledger, e
	}
	for _, b := range order {
		if claim, ok := ledger.Claims[b.BindingID]; ok {
			if claim.Status == "completed" {
				continue
			}
			ledger.GlobalHalt = true
			ledger.HaltReason = "ambiguous_transport"
			_ = writeV33Atomic(ledgerPath, ledger)
			return ledger, errors.New("v33 pending claim requires global halt; no retry")
		}
		total, ok := safeTotalV33(ledger.Usage)
		if !ok {
			return ledger, errors.New("v33 ledger usage overflow")
		}
		if err := agentqualification.AdmitOpenCodeStructuredV33Observation(d, b, agentqualification.OpenCodeStructuredV33Aggregate{Tokens: total, CostMicros: ledger.Usage.CostMicros, ProviderTurns: ledger.Usage.ProviderTurns}); err != nil {
			return ledger, err
		}
		workspace, err := prepareV33Workspace(root, b.BindingID)
		if err != nil {
			return ledger, err
		}
		plan, err := agentqualification.BuildProtocolAblationV32OpenCodePlan(d, b)
		if err != nil {
			return ledger, err
		}
		provider := &v33Provider{d: d, b: b, workspace: workspace.Root, artifact: root}
		preflight, err := provider.prepare(ctx, plan)
		if err != nil {
			_ = provider.close()
			return ledger, sealV33Halt(root, ledgerPath, &ledger, b.BindingID, "runtime_failure", err)
		}
		if err := writeV33Once(filepath.Join(root, "preflight", safeV33(b.BindingID)+".json"), mustV33(preflight)); err != nil {
			provider.close()
			return ledger, err
		}
		decision, _, err := agentqualification.BeginProtocolAblationV32Observation(d, &ledger, b)
		if err != nil || decision != "launch" {
			provider.close()
			return ledger, fmt.Errorf("v33 claim admission: %w", err)
		}
		if err := writeV33Once(filepath.Join(root, "claims", safeV33(b.BindingID)+".claim"), []byte(b.BindingID+"\n")); err != nil {
			provider.close()
			return ledger, err
		}
		if err := writeV33Atomic(ledgerPath, ledger); err != nil {
			provider.close()
			return ledger, err
		}
		observation, runErr := agentqualification.RunProtocolAblationV32Observation(ctx, d, b, workspace, provider)
		shutdownErr := provider.close()
		if runErr != nil {
			if errors.Is(runErr, agentqualification.ErrOpenCodeStructuredV33CandidateResourceExhaustion) {
				observation.Outcome = "failed"
				observation.FailureClass = "candidate_resource_exhaustion"
				observation.FailureScope = "observation"
				observation.Usage = provider.usage
				observation.ProviderIdentity = provider.identity
				observation.RawReceipts = append([]json.RawMessage(nil), provider.rawReceipts...)
				observation.GraderResult = agentqualification.RealisticNextCalibrationResult{CaseID: "P32", Verdict: agentqualification.RealisticNextCandidateFailure, Reasons: []string{"reserved_resource_exhaustion_before_completion"}}
				observation.EvidenceSHA256 = agentqualification.ProtocolAblationV32ObservationEvidenceSHA(observation)
				if shutdownErr != nil {
					return ledger, sealV33Halt(root, ledgerPath, &ledger, b.BindingID, "runtime_failure", shutdownErr)
				}
				if err := agentqualification.CompleteOpenCodeStructuredV33ResourceExhaustion(d, &ledger, b, observation); err != nil {
					return ledger, err
				}
				if err := writeV33Once(filepath.Join(root, "observations", safeV33(b.BindingID), "observation.json"), mustV33(observation)); err != nil {
					return ledger, err
				}
				if err := writeV33Atomic(ledgerPath, ledger); err != nil {
					return ledger, err
				}
				continue
			}
			return ledger, sealV33Halt(root, ledgerPath, &ledger, b.BindingID, "transport_failure", runErr)
		}
		if shutdownErr != nil {
			return ledger, sealV33Halt(root, ledgerPath, &ledger, b.BindingID, "runtime_failure", shutdownErr)
		}
		if err := agentqualification.CompleteProtocolAblationV32Observation(d, &ledger, b, observation); err != nil {
			return ledger, err
		}
		if err := writeV33Once(filepath.Join(root, "observations", safeV33(b.BindingID), "observation.json"), mustV33(observation)); err != nil {
			return ledger, err
		}
		if err := writeV33Atomic(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		if ledger.GlobalHalt {
			return ledger, nil
		}
	}
	return ledger, nil
}

func prepareV33Workspace(root, bindingID string) (agentqualification.RealisticNextWorkspace, error) {
	parent := filepath.Join(root, "workspaces", safeV33(bindingID))
	if err := os.MkdirAll(parent, 0700); err != nil {
		return agentqualification.RealisticNextWorkspace{}, err
	}
	return agentqualification.PrepareRealisticNextR05V1(parent)
}

func (p *v33Provider) prepare(ctx context.Context, plan agentqualification.OpenCodeScreeningPlan) (agentqualification.OpenCodeStructuredV33Preflight, error) {
	var proof agentqualification.OpenCodeStructuredV33Preflight
	exe := os.Getenv("OPENCODE_V33_EXECUTABLE")
	if exe == "" {
		return proof, errors.New("OPENCODE_V33_EXECUTABLE required")
	}
	p.executable = exe
	rawExe, err := os.ReadFile(exe)
	if err != nil {
		return proof, err
	}
	exeHash, _ := agentqualification.HashOpenCodeStructuredV33Runtime(rawExe, nil)
	if exeHash != agentqualification.OpenCodeStructuredV33ExecutableSHA256 {
		return proof, errors.New("OpenCode executable hash mismatch")
	}
	configDir, err := os.MkdirTemp("", "multica-v33-config-")
	if err != nil {
		return proof, err
	}
	p.configDir = configDir
	env := append(os.Environ(), "OPENCODE_CONFIG_CONTENT="+string(plan.InlineConfig), "OPENCODE_CONFIG_DIR="+configDir, "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true")
	p.baseEnv = env
	debug := exec.CommandContext(ctx, exe, "--pure", "debug", "config")
	debug.Dir = p.workspace
	debug.Env = env
	resolved, err := debug.Output()
	if err != nil {
		return proof, fmt.Errorf("resolved config: %w", err)
	}
	p.resolvedHash, err = agentqualification.ValidateOpenCodeResolvedConfig(plan, resolved)
	if err != nil {
		return proof, err
	}
	apiHash, err := p.startServer(ctx, p.d.PerObservationGeneratedTokenCeiling)
	if err != nil {
		return proof, err
	}
	proof = agentqualification.OpenCodeStructuredV33Preflight{Version: agentqualification.OpenCodeStructuredV33Version, ExecutableSHA256: exeHash, OpenAPISHA256: apiHash, InlineConfigSHA256: plan.InlineConfigSHA256, ResolvedConfigSHA256: p.resolvedHash, BaseURL: p.baseURL, Ready: apiHash == agentqualification.OpenCodeStructuredV33OpenAPISHA256}
	if err := agentqualification.ValidateOpenCodeStructuredV33Ready(proof); err != nil {
		return proof, err
	}
	return proof, nil
}

func (p *v33Provider) startServer(ctx context.Context, ceiling int64) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	p.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	p.command = exec.CommandContext(ctx, p.executable, "serve", "--pure", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port))
	p.command.Dir = p.workspace
	p.command.Env = append(append([]string(nil), p.baseEnv...), fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", ceiling))
	var stderr bytes.Buffer
	p.command.Stderr = &stderr
	if err := p.command.Start(); err != nil {
		return "", err
	}
	p.client = newV33HTTPClient()
	p.currentCeiling = ceiling
	var openAPI []byte
	var observedHash string
	var readinessErr error
	for until := time.Now().Add(15 * time.Second); time.Now().Before(until); {
		openAPI, observedHash, readinessErr = fetchV33OpenAPI(ctx, p.baseURL)
		if readinessErr == nil {
			if observedHash != agentqualification.OpenCodeStructuredV33OpenAPISHA256 {
				return observedHash, fmt.Errorf("OpenCode OpenAPI hash mismatch: observed=%s expected=%s bytes=%d", observedHash, agentqualification.OpenCodeStructuredV33OpenAPISHA256, len(openAPI))
			}
			return observedHash, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if readinessErr == nil {
		readinessErr = errors.New("OpenCode readiness deadline elapsed without response")
	}
	return "", fmt.Errorf("OpenCode readiness transport failure: %w", readinessErr)
}

func fetchV33OpenAPI(ctx context.Context, baseURL string) ([]byte, string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, v33ReadinessRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/doc", nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Timeout: v33ReadinessRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("OpenCode readiness HTTP status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("OpenCode readiness body: %w", err)
	}
	if len(raw) == 0 {
		return nil, "", errors.New("OpenCode readiness body empty")
	}
	_, hash := agentqualification.HashOpenCodeStructuredV33Runtime(nil, raw)
	return raw, hash, nil
}

func newV33HTTPClient() *http.Client {
	return &http.Client{Timeout: time.Duration(agentqualification.OpenCodeStructuredV33HTTPTimeoutSeconds) * time.Second}
}

func (p *v33Provider) Call(ctx context.Context, plan agentqualification.OpenCodeScreeningPlan) (agentqualification.ProtocolAblationV32ProviderResponse, error) {
	var out agentqualification.ProtocolAblationV32ProviderResponse
	budgetCapsule, err := agentqualification.BuildOpenCodeStructuredV33CapsuleForPlan(p.d, p.b, plan, "ses_budget", "msg_budget", p.workspace, p.resolvedHash)
	if err != nil {
		return out, err
	}
	inputBound := int64(len(plan.Prompt) + len(plan.Schema) + len(plan.InlineConfig) + 32768)
	budgetCapsule, err = agentqualification.PrepareOpenCodeStructuredV33Step(p.d, budgetCapsule, p.usage, inputBound)
	if err != nil {
		return out, err
	}
	if budgetCapsule.GeneratedTokenCeiling != p.currentCeiling {
		if err := p.stopServer(); err != nil {
			return out, err
		}
		if _, err := p.startServer(ctx, budgetCapsule.GeneratedTokenCeiling); err != nil {
			return out, err
		}
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/session?directory="+url.QueryEscape(p.workspace), strings.NewReader(`{"title":"protocol-ablation-v33"}`))
	request.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(request)
	if err != nil {
		return out, err
	}
	sessionRaw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	session, err := agentqualification.ParseOpenCodeStructuredV33CreatedSession(resp.StatusCode, resp.Header.Get("Content-Type"), sessionRaw)
	if err != nil {
		return out, err
	}
	messageID := fmt.Sprintf("msg_v33_%02d_%d", p.call, time.Now().UnixNano())
	capsule, err := agentqualification.BuildOpenCodeStructuredV33CapsuleForPlan(p.d, p.b, plan, session.ID, messageID, p.workspace, p.resolvedHash)
	if err != nil {
		return out, err
	}
	capsule, err = agentqualification.PrepareOpenCodeStructuredV33Step(p.d, capsule, p.usage, inputBound)
	if err != nil {
		return out, err
	}
	if capsule.GeneratedTokenCeiling != p.currentCeiling {
		return out, errors.New("v33 runtime output cap differs from admitted capsule")
	}
	receipt, err := (agentqualification.OpenCodeStructuredV33HTTPClient{Client: p.client, BaseURL: p.baseURL}).PromptOncePlan(ctx, p.d, p.b, plan, capsule)
	if err != nil {
		return out, err
	}
	if err := agentqualification.ValidateOpenCodeStructuredV33ReceiptForPlan(p.d, p.b, plan, capsule, receipt); err != nil {
		return out, err
	}
	receiptRaw := mustV33(receipt)
	if err := writeV33Once(filepath.Join(p.artifact, "raw", safeV33(p.b.BindingID), fmt.Sprintf("provider-%02d.json", p.call+1)), receiptRaw); err != nil {
		return out, err
	}
	p.call++
	if !addUsageV33(&p.usage, receipt.Usage) {
		return out, errors.New("v33 cumulative usage overflow")
	}
	out.CandidateJSON = append(json.RawMessage(nil), receipt.Structured...)
	out.RawReceipt = receiptRaw
	out.Usage = receipt.Usage
	out.ProviderIdentity = agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: receipt.ModelID, ReportedEffort: receipt.Variant, ReportedServingProvider: p.b.Candidate.ServingProvider, ResolvedConfigSHA256: p.resolvedHash, ServingProviderEvidence: "enforced_single_route_no_fallback"}
	if p.call == 1 {
		p.identity = out.ProviderIdentity
	} else if p.identity != out.ProviderIdentity {
		return out, errors.New("v33 provider identity changed within observation")
	}
	p.rawReceipts = append(p.rawReceipts, append(json.RawMessage(nil), receiptRaw...))
	return out, nil
}

func (p *v33Provider) stopServer() error {
	if p.command == nil {
		return nil
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
	case err := <-done:
		p.command = nil
		return err
	case <-ctx.Done():
		_ = p.command.Process.Kill()
		<-done
		p.command = nil
		return nil
	}
}
func (p *v33Provider) close() error {
	if p.configDir != "" {
		defer os.RemoveAll(p.configDir)
	}
	return p.stopServer()
}
func sealV33Halt(root, ledgerPath string, ledger *agentqualification.ProtocolAblationV32Ledger, binding, class string, cause error) error {
	ledger.GlobalHalt = true
	ledger.HaltReason = class
	digest := hashV33([]byte(cause.Error()))
	halt, _ := agentqualification.SealOpenCodeStructuredV33GlobalHalt(class, digest)
	_ = writeV33Once(filepath.Join(root, "halts", safeV33(binding)+".json"), mustV33(map[string]any{"event": halt, "class": class, "error": cause.Error(), "error_sha256": digest}))
	_ = writeV33Atomic(ledgerPath, *ledger)
	return cause
}
func safeTotalV33(u agentqualification.ProtocolAblationV32Usage) (int64, bool) {
	if u.InputTokens < 0 || u.OutputTokens < 0 || u.ReasoningTokens < 0 {
		return 0, false
	}
	if u.InputTokens > int64(^uint64(0)>>1)-u.OutputTokens {
		return 0, false
	}
	n := u.InputTokens + u.OutputTokens
	if n > int64(^uint64(0)>>1)-u.ReasoningTokens {
		return 0, false
	}
	return n + u.ReasoningTokens, true
}
func addUsageV33(dst *agentqualification.ProtocolAblationV32Usage, src agentqualification.ProtocolAblationV32Usage) bool {
	if src.ProviderTurns < 0 || dst.ProviderTurns < 0 || dst.ProviderTurns > math.MaxInt-src.ProviderTurns {
		return false
	}
	t, ok := safeTotalV33(agentqualification.ProtocolAblationV32Usage{InputTokens: dst.InputTokens, OutputTokens: src.InputTokens})
	if !ok {
		return false
	}
	dst.InputTokens = t
	t, ok = safeTotalV33(agentqualification.ProtocolAblationV32Usage{InputTokens: dst.OutputTokens, OutputTokens: src.OutputTokens})
	if !ok {
		return false
	}
	dst.OutputTokens = t
	t, ok = safeTotalV33(agentqualification.ProtocolAblationV32Usage{InputTokens: dst.ReasoningTokens, OutputTokens: src.ReasoningTokens})
	if !ok {
		return false
	}
	dst.ReasoningTokens = t
	t, ok = safeTotalV33(agentqualification.ProtocolAblationV32Usage{InputTokens: dst.CostMicros, OutputTokens: src.CostMicros})
	if !ok {
		return false
	}
	dst.CostMicros = t
	dst.ProviderTurns += src.ProviderTurns
	return true
}
func writeV33Once(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
func writeV33Atomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw := mustV33(v)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func mustV33(v any) []byte    { raw, _ := json.MarshalIndent(v, "", "  "); return raw }
func safeV33(v string) string { return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(v) }
func hashV33(v []byte) string { d := sha256.Sum256(v); return hex.EncodeToString(d[:]) }
