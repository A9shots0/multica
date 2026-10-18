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

type realClaudeV37Provider struct{}

func (realClaudeV37Provider) Plan(ctx context.Context, plan agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
	executable := "claude"
	if runtime.GOOS == "windows" {
		executable = "claude.cmd"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return agentqualification.ClaudeRealisticV37Plan{}, nil, agentqualification.ErrRealisticCredentialFailure
	}
	temporary, err := os.MkdirTemp("", "sm-claude-realistic-v37-")
	if err != nil {
		return agentqualification.ClaudeRealisticV37Plan{}, nil, failV37(v37Evidence, err)
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
	command.Env = claudeV37Environment()
	command.Stdin = strings.NewReader(plan.Prompt)
	var stdout, stderr strings.Builder
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	completed := time.Now().UTC()
	raw := []byte(stdout.String())
	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return agentqualification.ClaudeRealisticV37Plan{}, nil, failV37(v37Campaign, ctx.Err())
		}
		if claudeV37StructuredCredentialFailure(raw) {
			return agentqualification.ClaudeRealisticV37Plan{}, nil, agentqualification.ErrRealisticCredentialFailure
		}
		return agentqualification.ClaudeRealisticV37Plan{}, nil, failV37(v37Transport, errors.New(strings.TrimSpace(stderr.String())))
	}
	candidate, receipt, err := agentqualification.ParseClaudeSubscriptionRealisticV37Response(request, plan, raw, started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano))
	if err != nil {
		return candidate, receipt, failV37(v37Protocol, err)
	}
	return candidate, receipt, nil
}

func claudeV37StructuredCredentialFailure(raw []byte) bool {
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

func claudeV37Environment() []string {
	names := []string{"APPDATA", "COMSPEC", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "PATH", "PATHEXT", "SystemDrive", "SystemRoot", "TEMP", "TMP", "USERPROFILE", "WINDIR"}
	environment := make([]string, 0, len(names)+1)
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
}

func verifyClaudeV37RuntimeIdentity() error {
	d, err := agentqualification.ClaudeSubscriptionRealisticV37()
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
	hash, err := fileHashV37(absolute)
	if err != nil || hash != d.RuntimeIdentity.ExecutableSHA256 {
		return errors.New("Claude executable hash drift")
	}
	return nil
}
