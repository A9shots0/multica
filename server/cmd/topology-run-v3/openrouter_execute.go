package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

const (
	openRouterCreditsURL   = "https://openrouter.ai/api/v1/credits"
	openCodeRuntimeVersion = "1.18.18"
)

type openRouterAuthFile struct {
	OpenRouter struct {
		Type string `json:"type"`
		Key  string `json:"key"`
	} `json:"openrouter"`
}

type openRouterCreditsResponse struct {
	Data struct {
		TotalUsage json.Number `json:"total_usage"`
	} `json:"data"`
}

type openCodeCommandRuntime struct {
	Executable string
	WorkDir    string
	Env        []string
}

func executeOneOpenRouterObservation(ctx context.Context, artifactRoot string) (agentqualification.CompletedOpenCodeLaunch, error) {
	var completed agentqualification.CompletedOpenCodeLaunch
	batch, err := agentqualification.OpenRouterScreeningBatch()
	if err != nil {
		return completed, err
	}
	ledger, err := loadOpenRouterBatchLedger(artifactRoot, batch)
	if err != nil {
		return completed, err
	}
	selectionTime := time.Now().UTC()
	selectionSpend, err := readOpenRouterSpendSnapshot(ctx, selectionTime)
	if err != nil {
		return completed, err
	}
	prepared, err := agentqualification.PrepareNextScreeningAPILaunch(batch, ledger, selectionSpend, selectionTime.Format(time.RFC3339Nano))
	if err != nil {
		return completed, err
	}
	if prepared.Assignment == nil || !prepared.Admission.CanLaunch {
		return completed, fmt.Errorf("no admitted OpenRouter assignment: %v", prepared.Admission.Reasons)
	}
	var screeningCase agentqualification.ScreeningCase
	for _, candidateCase := range batch.Cases {
		if candidateCase.CaseID == prepared.Assignment.CaseID && candidateCase.CaseVersion == prepared.Assignment.CaseVersion {
			screeningCase = candidateCase
			break
		}
	}
	plan, err := agentqualification.BuildOpenCodePlanForTransport(batch.CandidateTransport, prepared.Assignment.Configuration, screeningCase)
	if err != nil {
		return completed, err
	}
	runtimeState, err := newOpenCodeCommandRuntime(plan)
	if err != nil {
		return completed, err
	}
	resolvedConfig, err := runtimeState.resolvedConfig(ctx)
	if err != nil {
		return completed, err
	}
	if _, err := agentqualification.ValidateOpenCodeResolvedConfig(plan, resolvedConfig); err != nil {
		return completed, err
	}
	now := time.Now().UTC()
	spend, err := readOpenRouterSpendSnapshot(ctx, now)
	if err != nil {
		return completed, err
	}
	adapter := openRouterExecutionAdapter(plan)
	grant := agentqualification.ProviderCallGrantInput{
		BatchID: batch.BatchID, GrantedBy: "owner", Scope: agentqualification.OneAdmittedOpenRouterObservationScope,
		MaxProviderCalls: 1, GrantedAt: now.Format(time.RFC3339Nano),
	}
	evaluatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	return agentqualification.ExecuteOpenCodeReservedRun(artifactRoot, batch, ledger, spend, evaluatedAt, evaluatedAt, resolvedConfig, adapter, grant, runtimeState.runOnce)
}

func openRouterPlanForReservationDir(reservationDir string, batch agentqualification.ScreeningBatchDefinition) (agentqualification.OpenCodeScreeningPlan, error) {
	var reservation agentqualification.ScreeningReservation
	raw, err := os.ReadFile(filepath.Join(reservationDir, "reservation.json"))
	if err != nil {
		return agentqualification.OpenCodeScreeningPlan{}, err
	}
	if err := json.Unmarshal(raw, &reservation); err != nil {
		return agentqualification.OpenCodeScreeningPlan{}, err
	}
	assignments, err := agentqualification.ScreeningAssignments(batch)
	if err != nil {
		return agentqualification.OpenCodeScreeningPlan{}, err
	}
	for _, assignment := range assignments {
		if assignment.EvaluationBinding != reservation.EvaluationBinding || assignment.Configuration.ID != reservation.CandidateID || assignment.CaseID != reservation.CaseID {
			continue
		}
		for _, screeningCase := range batch.Cases {
			if screeningCase.CaseID == assignment.CaseID && screeningCase.CaseVersion == assignment.CaseVersion {
				return agentqualification.BuildOpenCodePlanForTransport(batch.CandidateTransport, assignment.Configuration, screeningCase)
			}
		}
	}
	return agentqualification.OpenCodeScreeningPlan{}, fmt.Errorf("reservation is not a frozen batch assignment")
}

func loadOpenRouterBatchLedger(artifactRoot string, batch agentqualification.ScreeningBatchDefinition) (agentqualification.BatchUsageLedger, error) {
	ledger := agentqualification.BatchUsageLedger{SchemaVersion: agentqualification.BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	if _, err := os.Stat(artifactRoot); os.IsNotExist(err) {
		return ledger, nil
	} else if err != nil {
		return ledger, err
	}
	err := filepath.WalkDir(artifactRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "observation.json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var record agentqualification.ObservationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		if validity := agentqualification.ValidateObservation(record); !validity.Valid {
			return fmt.Errorf("invalid prior observation %s: %v", path, validity.Reasons)
		}
		ledger.ObservationReceipts = append(ledger.ObservationReceipts, record)
		return nil
	})
	return ledger, err
}

func openRouterExecutionAdapter(plan agentqualification.OpenCodeScreeningPlan) agentqualification.ExecutionAdapterIdentity {
	return agentqualification.ExecutionAdapterIdentity{
		Provider: "openrouter", Runtime: "opencode-cli", RuntimeVersion: openCodeRuntimeVersion,
		Endpoint: "local-cli-run-json", BillingBoundary: "openrouter_metered", AccountLane: "opencode_local_openrouter",
		ToolPolicy: "none", ProviderEventPolicyVersion: "opencode-run-json-export-routing.v2",
		ExecutorSHA256: sha256String("screening-run-openrouter-executor.v1"), IsolationPolicySHA256: plan.InlineConfigSHA256,
		EnvironmentAllowlistSHA256: sha256String(strings.Join(openCodeEnvironmentNames(), "\n")),
	}
}

func newOpenCodeCommandRuntime(plan agentqualification.OpenCodeScreeningPlan) (openCodeCommandRuntime, error) {
	executable, err := exec.LookPath("opencode")
	if err != nil {
		return openCodeCommandRuntime{}, err
	}
	tempRoot, err := os.MkdirTemp("", "multica-item14-opencode-")
	if err != nil {
		return openCodeCommandRuntime{}, err
	}
	workDir := filepath.Join(tempRoot, "work")
	configDir := filepath.Join(tempRoot, "config")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return openCodeCommandRuntime{}, err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return openCodeCommandRuntime{}, err
	}
	values := map[string]string{}
	for _, name := range openCodeEnvironmentNames() {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			values[name] = value
		}
	}
	values["OPENCODE_CONFIG_CONTENT"] = string(plan.InlineConfig)
	values["OPENCODE_CONFIG_DIR"] = configDir
	values["OPENCODE_DISABLE_GLOBAL_CONFIG"] = "true"
	values["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
	values["OPENCODE_DISABLE_PRUNE"] = "true"
	if plan.Transport == agentqualification.OpenCodeToolFreeTransportV4 {
		values["OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX"] = fmt.Sprintf("%d", agentqualification.OpenCodeQualificationOutputTokenMaxV4)
	}
	if plan.Transport == agentqualification.OpenCodeToolFreeTransportV5 {
		values["OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX"] = fmt.Sprintf("%d", agentqualification.OpenCodeQualificationOutputTokenMax)
	}
	if plan.Transport == agentqualification.OpenCodeToolFreeTransportV6 {
		values["OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX"] = fmt.Sprintf("%d", agentqualification.OpenCodeQualificationOutputTokenMaxV4)
	}
	if plan.Transport == agentqualification.OpenCodeRealisticActionTransport {
		values["OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX"] = fmt.Sprintf("%d", agentqualification.OpenCodeRealisticActionOutputTokenMax)
	}
	env := make([]string, 0, len(values))
	for _, name := range append(openCodeEnvironmentNames(), "OPENCODE_CONFIG_CONTENT", "OPENCODE_CONFIG_DIR", "OPENCODE_DISABLE_GLOBAL_CONFIG", "OPENCODE_DISABLE_AUTOUPDATE", "OPENCODE_DISABLE_PRUNE", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX") {
		if value, ok := values[name]; ok {
			env = append(env, name+"="+value)
		}
	}
	return openCodeCommandRuntime{Executable: executable, WorkDir: workDir, Env: env}, nil
}

func openCodeEnvironmentNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"PATH", "SystemRoot", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "APPDATA", "TEMP", "TMP"}
	}
	return []string{"PATH", "HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "TMPDIR"}
}

func (runtimeState openCodeCommandRuntime) command(ctx context.Context, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, runtimeState.Executable, args...)
	command.Dir = runtimeState.WorkDir
	command.Env = append([]string(nil), runtimeState.Env...)
	return command
}

func (runtimeState openCodeCommandRuntime) resolvedConfig(ctx context.Context) ([]byte, error) {
	command := runtimeState.command(ctx, "--pure", "debug", "config")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("OpenCode resolved-config preflight failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func (runtimeState openCodeCommandRuntime) runOnce(plan agentqualification.OpenCodeScreeningPlan) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command, err := runtimeState.buildRunCommand(ctx, plan)
	if err != nil {
		return nil, nil, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	rawJSONL, err := command.Output()
	if err != nil {
		return rawJSONL, []byte(`{"export_unavailable":true}`), fmt.Errorf("OpenCode run failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	sessionID, err := sessionIDFromOpenCodeJSONL(rawJSONL)
	if err != nil {
		return rawJSONL, []byte(`{"export_unavailable":true}`), err
	}
	exportCommand := runtimeState.command(ctx, "--pure", "export", sessionID)
	stderr.Reset()
	exportCommand.Stderr = &stderr
	rawExport, err := exportCommand.Output()
	if err != nil {
		return rawJSONL, rawExportOrMarker(rawExport), fmt.Errorf("OpenCode export failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return rawJSONL, rawExport, nil
}

func (runtimeState openCodeCommandRuntime) buildRunCommand(ctx context.Context, plan agentqualification.OpenCodeScreeningPlan) (*exec.Cmd, error) {
	if plan.Transport != agentqualification.OpenCodeToolFreeTransportV3 && plan.Transport != agentqualification.OpenCodeToolFreeTransportV4 && plan.Transport != agentqualification.OpenCodeToolFreeTransportV5 && plan.Transport != agentqualification.OpenCodeToolFreeTransportV6 && plan.Transport != agentqualification.OpenCodeRealisticActionTransport {
		return nil, fmt.Errorf("production OpenCode runner requires a registered exact stdin transport")
	}
	command := runtimeState.command(ctx, "--pure", "run", "--format", "json", "--model", plan.Model, "--variant", plan.Effort, "--agent", "qualification", "--title", "qualification-"+plan.CandidateID, "--dir", runtimeState.WorkDir)
	command.Stdin = strings.NewReader(plan.Prompt)
	return command, nil
}

func sessionIDFromOpenCodeJSONL(raw []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var sessionID string
	for scanner.Scan() {
		var event struct {
			SessionID string `json:"sessionID"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return "", err
		}
		if event.SessionID == "" {
			return "", fmt.Errorf("OpenCode event omitted session ID")
		}
		if sessionID == "" {
			sessionID = event.SessionID
		} else if sessionID != event.SessionID {
			return "", fmt.Errorf("OpenCode run emitted multiple sessions")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if sessionID == "" {
		return "", fmt.Errorf("OpenCode run emitted no session")
	}
	return sessionID, nil
}

func rawExportOrMarker(raw []byte) []byte {
	if len(raw) != 0 {
		return raw
	}
	return []byte(`{"export_unavailable":true}`)
}

func readOpenRouterSpendSnapshot(ctx context.Context, observedAt time.Time) (agentqualification.APISpendSnapshot, error) {
	profile, err := os.UserHomeDir()
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	rawAuth, err := os.ReadFile(filepath.Join(profile, ".local", "share", "opencode", "auth.json"))
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	var auth openRouterAuthFile
	if err := json.Unmarshal(rawAuth, &auth); err != nil || auth.OpenRouter.Type != "api" || strings.TrimSpace(auth.OpenRouter.Key) == "" {
		return agentqualification.APISpendSnapshot{}, fmt.Errorf("OpenRouter API credential is unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterCreditsURL, nil)
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	request.Header.Set("Authorization", "Bearer "+auth.OpenRouter.Key)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	if response.StatusCode != http.StatusOK {
		return agentqualification.APISpendSnapshot{}, fmt.Errorf("OpenRouter credits read failed with status %d", response.StatusCode)
	}
	var credits openRouterCreditsResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&credits); err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	spentMicros, err := ceilingUSDMicros(credits.Data.TotalUsage.String())
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	return agentqualification.APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: spentMicros, ObservedAt: observedAt.Format(time.RFC3339Nano), Source: "authoritative_api_usage"}, nil
}

func ceilingUSDMicros(value string) (int64, error) {
	rational, ok := new(big.Rat).SetString(value)
	if !ok || rational.Sign() < 0 {
		return 0, fmt.Errorf("invalid USD amount")
	}
	rational.Mul(rational, big.NewRat(1_000_000, 1))
	quotient := new(big.Int).Quo(rational.Num(), rational.Denom())
	if new(big.Int).Mod(rational.Num(), rational.Denom()).Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("USD amount exceeds int64 micros")
	}
	return quotient.Int64(), nil
}

func sha256String(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
