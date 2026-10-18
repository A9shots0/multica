package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const claudeCalibrationExecutorVersion = "screening-run-claude-subscription-v29.v2-stdin"

type claudeCalibrationResult struct {
	SchemaVersion       string                               `json:"schema_version"`
	CalibrationID       string                               `json:"calibration_id"`
	DefinitionSHA256    string                               `json:"definition_sha256"`
	Status              string                               `json:"status"`
	ValidObservation    bool                                 `json:"valid_observation"`
	ProviderInvocations int                                  `json:"provider_invocations"`
	Retries             int                                  `json:"retries"`
	CandidateAdvances   bool                                 `json:"candidate_advances"`
	Verdict             string                               `json:"verdict,omitempty"`
	FailureReason       string                               `json:"failure_reason,omitempty"`
	Receipt             *agentqualification.ExecutionReceipt `json:"receipt,omitempty"`
	Grade               *agentqualification.BlindGrade       `json:"grade,omitempty"`
	EvidenceDir         string                               `json:"evidence_dir"`
	WallMilliseconds    int64                                `json:"wall_milliseconds"`
}

func claudeCalibrationPlan() (map[string]any, error) {
	definition, err := agentqualification.ClaudeSubscriptionCalibrationV29()
	if err != nil {
		return nil, err
	}
	plan, err := agentqualification.BuildClaudeSubscriptionScreeningPlan(definition.Configuration, definition.Case)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status": definition.Status, "provider_called": false,
		"calibration_id": definition.CalibrationID, "definition_sha256": agentqualification.ClaudeSubscriptionCalibrationV29SHA256(),
		"candidate_id": plan.CandidateID, "model": plan.Model, "reasoning_effort": plan.Effort,
		"case_id": plan.CaseID, "case_version": plan.CaseVersion, "transport": plan.Transport,
		"prompt_sha256": plan.PromptSHA256, "output_schema_sha256": plan.SchemaSHA256,
		"caps": definition.Caps, "billing_boundary": definition.BillingBoundary,
		"capacity_evidence": definition.CapacityEvidence, "production_allowed": false,
		"external_side_effects_allowed": false, "candidate_advancement_allowed": false,
		"args_template": plan.ArgsTemplate,
	}, nil
}

func executeClaudeCalibration(ctx context.Context, artifactRoot string) (claudeCalibrationResult, error) {
	definition, err := agentqualification.ClaudeSubscriptionCalibrationV29()
	if err != nil {
		return claudeCalibrationResult{}, err
	}
	if definition.Status != "frozen_reviewed" {
		return claudeCalibrationResult{}, fmt.Errorf("Claude calibration is not independently reviewed and frozen")
	}
	plan, err := agentqualification.BuildClaudeSubscriptionScreeningPlan(definition.Configuration, definition.Case)
	if err != nil {
		return claudeCalibrationResult{}, err
	}
	if !filepath.IsAbs(artifactRoot) {
		return claudeCalibrationResult{}, fmt.Errorf("Claude calibration evidence root must be absolute")
	}
	definitionHash := agentqualification.ClaudeSubscriptionCalibrationV29SHA256()
	binding := sha256String(definition.CalibrationID + "|" + definitionHash + "|observation-1")
	reservationDir := filepath.Join(artifactRoot, binding)
	if err := os.MkdirAll(artifactRoot, 0o700); err != nil {
		return claudeCalibrationResult{}, err
	}
	if err := os.Mkdir(reservationDir, 0o700); err != nil {
		return claudeCalibrationResult{}, fmt.Errorf("Claude calibration already reserved or ambiguous: %w", err)
	}
	reservedAt := time.Now().UTC()
	reservation := agentqualification.ScreeningReservation{
		SchemaVersion: agentqualification.ScreeningReservationSchemaVersion,
		BatchID:       definition.CalibrationID, EvaluationBinding: binding,
		CandidateID: definition.Configuration.ID, CaseID: definition.Case.CaseID,
		AdmissionInputSHA256: definitionHash, ReservedAt: reservedAt.Format(time.RFC3339Nano), State: "reserved",
	}
	if err := writeJSONExclusive(filepath.Join(reservationDir, "reservation.json"), reservation); err != nil {
		return claudeCalibrationResult{}, err
	}
	if err := writeJSONExclusive(filepath.Join(reservationDir, "launch-capsule.json"), map[string]any{
		"schema_version": "silver-mediums.claude-subscription-launch-capsule.v1", "state": "reserved_one_call",
		"provider_call_allowed": false, "definition_sha256": definitionHash, "evaluation_binding": binding,
		"transport": plan.Transport, "candidate_id": plan.CandidateID, "case_id": plan.CaseID, "case_version": plan.CaseVersion,
		"model": plan.Model, "reasoning_effort": plan.Effort, "prompt_sha256": plan.PromptSHA256,
		"output_schema_sha256": plan.SchemaSHA256, "args_template": plan.ArgsTemplate, "caps": definition.Caps,
	}); err != nil {
		return claudeCalibrationResult{}, err
	}
	if err := writeExclusiveFile(filepath.Join(reservationDir, "candidate-prompt.txt"), []byte(plan.Prompt)); err != nil {
		return claudeCalibrationResult{}, err
	}
	if err := writeExclusiveFile(filepath.Join(reservationDir, "output-schema.json"), plan.Schema); err != nil {
		return claudeCalibrationResult{}, err
	}

	executable := "claude"
	if runtime.GOOS == "windows" {
		executable = "claude.cmd"
	}
	executable, err = exec.LookPath(executable)
	if err != nil {
		return claudeCalibrationResult{}, fmt.Errorf("Claude executable unavailable before provider call: %w", err)
	}
	env, envNames := claudeCalibrationEnvironment()
	versionCommand := exec.CommandContext(ctx, executable, "--version")
	versionCommand.Env = env
	versionRaw, err := versionCommand.Output()
	if err != nil {
		return claudeCalibrationResult{}, fmt.Errorf("Claude version preflight failed before provider call: %w", err)
	}
	runtimeVersion := strings.TrimSpace(string(versionRaw))
	if runtimeVersion == "" {
		return claudeCalibrationResult{}, fmt.Errorf("Claude version preflight returned empty output")
	}

	workspace, err := os.MkdirTemp("", "sm-claude-v29-")
	if err != nil {
		return claudeCalibrationResult{}, err
	}
	defer os.RemoveAll(workspace)
	entries, err := os.ReadDir(workspace)
	if err != nil || len(entries) != 0 {
		return claudeCalibrationResult{}, fmt.Errorf("Claude calibration workspace is not empty")
	}
	args := materializeClaudeCalibrationArgs(plan)
	grant := map[string]any{
		"schema_version": "silver-mediums.provider-call-grant.v1", "calibration_id": definition.CalibrationID,
		"evaluation_binding": binding, "granted_by": "owner", "scope": "one_claude_subscription_transport_calibration",
		"max_provider_calls": 1, "granted_at": time.Now().UTC().Format(time.RFC3339Nano), "provider_call_allowed": true,
		"capacity_evidence": definition.CapacityEvidence,
	}
	if err := writeJSONExclusive(filepath.Join(reservationDir, "provider-call-grant.json"), grant); err != nil {
		return claudeCalibrationResult{}, err
	}
	if err := writeJSONExclusive(filepath.Join(reservationDir, "provider-call-started.json"), map[string]any{
		"provider_invocation": 1, "started_at": time.Now().UTC().Format(time.RFC3339Nano),
		"executable_sha256": sha256String(strings.ToLower(executable)), "runtime_version": runtimeVersion,
		"working_directory_policy": "new_empty_temporary_directory", "environment_allowlist_names": envNames,
	}); err != nil {
		return claudeCalibrationResult{}, err
	}

	started := time.Now().UTC()
	providerContext, cancelProvider := context.WithTimeout(ctx, time.Duration(definition.Caps.MaxWallSeconds)*time.Second)
	defer cancelProvider()
	command := exec.CommandContext(providerContext, executable, args...)
	command.Dir = workspace
	command.Env = env
	command.Stdin = strings.NewReader(plan.Prompt)
	var stdout, stderr strings.Builder
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	completed := time.Now().UTC()
	stdoutRaw, stderrRaw := []byte(stdout.String()), []byte(stderr.String())
	if err := writeExclusiveFile(filepath.Join(reservationDir, "provider-stdout.json"), stdoutRaw); err != nil {
		return claudeCalibrationResult{}, err
	}
	if err := writeExclusiveFile(filepath.Join(reservationDir, "provider-stderr.txt"), stderrRaw); err != nil {
		return claudeCalibrationResult{}, err
	}
	base := claudeCalibrationResult{
		SchemaVersion: "silver-mediums.claude-subscription-calibration-result.v1", CalibrationID: definition.CalibrationID,
		DefinitionSHA256: definitionHash, Status: "invalid", ProviderInvocations: 1, Retries: 0,
		CandidateAdvances: false, EvidenceDir: reservationDir, WallMilliseconds: completed.Sub(started).Milliseconds(),
	}
	if runErr != nil {
		base.FailureReason = "claude_transport_failure: " + runErr.Error()
		_ = writeJSONExclusive(filepath.Join(reservationDir, "calibration-result.json"), base)
		return base, fmt.Errorf("Claude calibration provider call failed without retry: %w", runErr)
	}
	providerResult, candidateResponse, err := agentqualification.ParseClaudeCodeSubscriptionJSONArtifact(stdoutRaw, started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano))
	if err != nil {
		base.FailureReason = "evidence_integrity_failure: " + err.Error()
		_ = writeJSONExclusive(filepath.Join(reservationDir, "calibration-result.json"), base)
		return base, err
	}
	totalTokens := *providerResult.Usage.InputTokens + *providerResult.Usage.OutputTokens
	if totalTokens > definition.Caps.MaxTotalReportedTokens || *providerResult.Usage.OutputTokens > definition.Caps.MaxOutputTokens {
		base.FailureReason = fmt.Sprintf("token_cap_breach: total=%d output=%d", totalTokens, *providerResult.Usage.OutputTokens)
		_ = writeJSONExclusive(filepath.Join(reservationDir, "calibration-result.json"), base)
		return base, errors.New(base.FailureReason)
	}
	assignment := agentqualification.ScreeningAssignment{
		BatchID: definition.CalibrationID, CaseID: definition.Case.CaseID, CaseVersion: definition.Case.CaseVersion,
		Attempt: 1, Configuration: definition.Configuration, CandidatePacketSHA256: definition.Case.CandidatePacketSHA256,
		EvaluationBinding: binding,
	}
	adapter := agentqualification.ExecutionAdapterIdentity{
		Provider: "anthropic", Runtime: "claude-code", RuntimeVersion: runtimeVersion,
		Endpoint: "local-cli-print-stdin", BillingBoundary: definition.BillingBoundary, AccountLane: "owner_subscription",
		ToolPolicy: "none", ProviderEventPolicyVersion: "claude-code-json-result.v1",
		ExecutorSHA256:             sha256String(claudeCalibrationExecutorVersion),
		IsolationPolicySHA256:      sha256String(plan.Transport + "|empty-workspace|restricted|tools-empty|mcp-empty|no-session|prompt-stdin"),
		EnvironmentAllowlistSHA256: sha256String(strings.Join(envNames, "\n")),
	}
	receipt, err := agentqualification.BuildExecutionReceipt(reservation, assignment, agentqualification.ClaudeExecutionPlanBinding(definition.CalibrationID, plan), adapter, providerResult)
	if err != nil {
		base.FailureReason = "evidence_integrity_failure: " + err.Error()
		_ = writeJSONExclusive(filepath.Join(reservationDir, "calibration-result.json"), base)
		return base, err
	}
	if _, err := agentqualification.SealExecutionReceipt(reservationDir, receipt); err != nil {
		return base, err
	}
	if err := writeExclusiveFile(filepath.Join(reservationDir, "candidate-response.json"), candidateResponse); err != nil {
		return base, err
	}
	grade, err := agentqualification.GradeBlindVersion(definition.Case.CaseID, definition.Case.CaseVersion, candidateResponse)
	if err != nil {
		base.FailureReason = "grader_failure: " + err.Error()
		_ = writeJSONExclusive(filepath.Join(reservationDir, "calibration-result.json"), base)
		return base, err
	}
	if err := writeJSONExclusive(filepath.Join(reservationDir, "blind-grade.json"), grade); err != nil {
		return base, err
	}
	base.ValidObservation, base.Status, base.Verdict = true, "complete", grade.Verdict
	base.Receipt, base.Grade = &receipt, &grade
	if grade.Verdict != "PASS" {
		base.FailureReason = strings.Join(append(append([]string{}, grade.FailedChecks...), grade.Disqualifiers...), "; ")
	}
	if err := writeJSONExclusive(filepath.Join(reservationDir, "calibration-result.json"), base); err != nil {
		return base, err
	}
	return base, nil
}

func materializeClaudeCalibrationArgs(plan agentqualification.ClaudeScreeningPlan) []string {
	args := make([]string, 0, len(plan.ArgsTemplate))
	for _, arg := range plan.ArgsTemplate {
		if arg == "<generated-schema>" {
			arg = string(plan.Schema)
		}
		args = append(args, arg)
	}
	return args
}

func claudeCalibrationEnvironment() ([]string, []string) {
	allowed := []string{"APPDATA", "COMSPEC", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "PATH", "PATHEXT", "SystemDrive", "SystemRoot", "TEMP", "TMP", "USERPROFILE", "WINDIR"}
	env := make([]string, 0, len(allowed)+2)
	names := make([]string, 0, len(allowed)+2)
	for _, name := range allowed {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			env = append(env, name+"="+value)
			names = append(names, name)
		}
	}
	env = append(env, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
	names = append(names, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC")
	return env, names
}

func marshalCalibrationResult(result claudeCalibrationResult) json.RawMessage {
	raw, _ := json.Marshal(result)
	return raw
}
