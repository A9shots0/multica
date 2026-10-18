package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type realClaudeV36Provider struct{}

func newRealClaudeV36Provider() agentqualification.RealisticProvider { return realClaudeV36Provider{} }

func (realClaudeV36Provider) Next(ctx context.Context, request agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
	plan, err := agentqualification.BuildClaudeSubscriptionRealisticV36ActionPlan(request)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, failV36(v36Evidence, err)
	}
	executable := "claude"
	if runtime.GOOS == "windows" {
		executable = "claude.cmd"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, agentqualification.ErrRealisticCredentialFailure
	}
	workspace, err := os.MkdirTemp("", "sm-claude-realistic-v36-")
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, failV36(v36Evidence, err)
	}
	defer os.RemoveAll(workspace)
	args := make([]string, 0, len(plan.ArgsTemplate))
	for _, arg := range plan.ArgsTemplate {
		if arg == "<generated-schema>" {
			arg = string(plan.Schema)
		}
		args = append(args, arg)
	}
	started := time.Now().UTC()
	command := exec.CommandContext(ctx, path, args...)
	command.Dir = workspace
	command.Env = claudeV36Environment()
	command.Stdin = strings.NewReader(plan.Prompt)
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	completed := time.Now().UTC()
	raw := []byte(stdout.String())
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return agentqualification.RealisticProviderResponse{}, failV36(v36CampaignCap, ctx.Err())
		}
		if claudeV36StructuredCredentialFailure(raw) {
			return agentqualification.RealisticProviderResponse{}, agentqualification.ErrRealisticCredentialFailure
		}
		return agentqualification.RealisticProviderResponse{}, failV36(v36ObservationInvalid, errors.New(strings.TrimSpace(stderr.String())))
	}
	response, err := agentqualification.ParseClaudeSubscriptionRealisticV36Response(request, plan, raw, started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano))
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, failV36(v36Evidence, err)
	}
	return response, nil
}

func claudeV36StructuredCredentialFailure(raw []byte) bool {
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

func claudeV36Environment() []string {
	names := []string{"APPDATA", "COMSPEC", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "PATH", "PATHEXT", "SystemDrive", "SystemRoot", "TEMP", "TMP", "USERPROFILE", "WINDIR"}
	env := make([]string, 0, len(names)+1)
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			env = append(env, name+"="+value)
		}
	}
	return append(env, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
}

func verifyClaudeV36RuntimeIdentity() error {
	d, err := agentqualification.ClaudeSubscriptionRealisticV36()
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
	hash, err := fileHashV36(absolute)
	if err != nil || hash != d.RuntimeIdentity.ExecutableSHA256 {
		return errors.New("Claude executable hash drift")
	}
	return nil
}
