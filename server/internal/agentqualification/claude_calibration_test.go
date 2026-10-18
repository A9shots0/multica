package agentqualification

import (
	"slices"
	"testing"
)

func TestClaudeSubscriptionCalibrationPinsOneRestrictedOAuthCompatibleCall(t *testing.T) {
	definition, err := ClaudeSubscriptionCalibrationV29()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildClaudeSubscriptionScreeningPlan(definition.Configuration, definition.Case)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Transport != ClaudeCodeSubscriptionToolFreeTransport || slices.Contains(plan.ArgsTemplate, "--bare") {
		t.Fatalf("subscription plan must preserve OAuth without bare mode: %+v", plan.ArgsTemplate)
	}
	for _, required := range []string{"--restricted", "--tools", "--strict-mcp-config", "--no-session-persistence", "--permission-prompts", "--json-schema", "--max-turns", "--model", "--effort"} {
		if !slices.Contains(plan.ArgsTemplate, required) {
			t.Fatalf("missing preventive argument %s: %v", required, plan.ArgsTemplate)
		}
	}
	if definition.Caps.MaxProviderInvocations != 1 || definition.Caps.MaxRetries != 0 || definition.ProductionAllowed || definition.ExternalSideEffectsAllowed || definition.AdvancementAllowed {
		t.Fatalf("calibration authority widened: %+v", definition)
	}
}
