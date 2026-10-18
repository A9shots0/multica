package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func claudeTestConfiguration() Configuration {
	instructionHash := sha256.Sum256(screeningCandidateInstructionsV5)
	skillHash := sha256.Sum256(screeningSkillV2)
	return Configuration{ID: "screen.generalist.claude.medium.v1", RoleID: "screening_generalist", Model: "claude-sonnet-4-6", ReasoningEffort: "medium", InstructionVersion: "shadow-candidate.v5", InstructionSHA256: hex.EncodeToString(instructionHash[:]), SkillVersions: map[string]string{"silver_mediums_context": "item14.v2-inline"}, SkillContentSHA256: map[string]string{"silver_mediums_context": hex.EncodeToString(skillHash[:])}, PermissionVersion: "isolated-no-tools.v1", RuntimeProvider: "claude_code_subscription"}
}

func TestClaudePlanPinsPreventiveToolFreeEphemeralShape(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildClaudeScreeningPlan(claudeTestConfiguration(), suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	if plan.PromptSHA256 == "" || plan.SchemaSHA256 == "" || !containsText(plan.Prompt, "field_requirements") {
		t.Fatalf("plan not pinned: %+v", plan)
	}
	wantPairs := [][]string{{"--tools", ""}, {"--permission-prompts", "none"}, {"--output-format", "json"}, {"--max-turns", "1"}}
	for _, pair := range wantPairs {
		found := false
		for index := 0; index+1 < len(plan.ArgsTemplate); index++ {
			if plan.ArgsTemplate[index] == pair[0] && plan.ArgsTemplate[index+1] == pair[1] {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing args %q %q: %v", pair[0], pair[1], plan.ArgsTemplate)
		}
	}
}

func TestClaudePlanRejectsInstructionOrPacketDrift(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	configuration := claudeTestConfiguration()
	configuration.InstructionSHA256 = "changed"
	if _, err := BuildClaudeScreeningPlan(configuration, suite.Cases[0]); err == nil {
		t.Fatal("instruction drift passed")
	}
	configuration = claudeTestConfiguration()
	changed := suite.Cases[0]
	changed.CandidatePacketSHA256 = "changed"
	if _, err := BuildClaudeScreeningPlan(configuration, changed); err == nil {
		t.Fatal("packet drift passed")
	}
}
