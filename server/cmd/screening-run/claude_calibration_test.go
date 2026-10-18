package main

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestClaudeCalibrationPlanIsProviderFreeAndFrozen(t *testing.T) {
	plan, err := claudeCalibrationPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan["status"] != "frozen_reviewed" || plan["provider_called"] != false || plan["transport"] != "claude_code_print_restricted_tool_free_v2" {
		t.Fatalf("unexpected calibration plan: %+v", plan)
	}
}

func TestClaudeCalibrationPromptIsSentOnStdinNotCommandLine(t *testing.T) {
	definition, err := agentqualification.ClaudeSubscriptionCalibrationV29()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := agentqualification.BuildClaudeSubscriptionScreeningPlan(definition.Configuration, definition.Case)
	if err != nil {
		t.Fatal(err)
	}
	args := materializeClaudeCalibrationArgs(plan)
	joined := strings.Join(args, "\n")
	if strings.Contains(joined, plan.Prompt) || strings.Contains(joined, "<generated-schema>") {
		t.Fatal("prompt leaked onto the command line or schema placeholder was not resolved")
	}
	if len(plan.Prompt) < 6000 {
		t.Fatalf("test no longer exercises the long prompt that caused the Windows launch failure: %d bytes", len(plan.Prompt))
	}
}

func TestClaudeCalibrationEnvironmentExcludesCredentialAndRepositoryVariables(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "must-not-pass")
	t.Setenv("DATABASE_URL", "must-not-pass")
	env, names := claudeCalibrationEnvironment()
	joined := strings.Join(append(env, names...), "\n")
	for _, forbidden := range []string{"ANTHROPIC_API_KEY", "DATABASE_URL", "must-not-pass"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("environment leaked %s", forbidden)
		}
	}
}
