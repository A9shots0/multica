package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func openCodeTestConfiguration() Configuration {
	instructionHash := sha256.Sum256(screeningCandidateInstructionsV5)
	skillHash := sha256.Sum256(screeningSkillV2)
	return Configuration{ID: "screen.generalist.opencode.medium.v1", RoleID: "screening_generalist", Model: "opencode/test-model", ReasoningEffort: "medium", InstructionVersion: "shadow-candidate.v5", InstructionSHA256: hex.EncodeToString(instructionHash[:]), SkillVersions: map[string]string{"silver_mediums_context": "item14.v2-inline"}, SkillContentSHA256: map[string]string{"silver_mediums_context": hex.EncodeToString(skillHash[:])}, PermissionVersion: "isolated-no-tools.v1", RuntimeProvider: OpenCodeCLIRuntimeProvider}
}

func TestOpenCodePlanPinsEmptyWorkspaceAndDenyAllTools(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeScreeningPlan(openCodeTestConfiguration(), suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	if plan.PromptSHA256 == "" || plan.SchemaSHA256 == "" || plan.InlineConfigSHA256 == "" || !containsText(plan.Prompt, "field_requirements") {
		t.Fatalf("plan not pinned: %+v", plan)
	}
	var config openCodeQualificationConfig
	if err := json.Unmarshal(plan.InlineConfig, &config); err != nil {
		t.Fatal(err)
	}
	if config.Permission != "deny" || config.Tools["*"] || config.Share != "disabled" || config.Snapshot || len(config.Plugin) != 0 || config.Compaction["auto"] || config.Agent["qualification"].Permission != "deny" || config.Agent["qualification"].Tools["*"] || config.Agent["qualification"].Steps != 1 {
		t.Fatalf("OpenCode isolation config is fail-open: %+v", config)
	}
	for _, required := range []string{"--pure", "--format", "--model", "--variant", "--agent", "--title", "--dir"} {
		if !contains(plan.ArgsTemplate, required) {
			t.Fatalf("missing arg %s: %v", required, plan.ArgsTemplate)
		}
	}
}

func TestOpenCodePlanV2AvoidsFirstTurnStepCapWithoutEnablingTools(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeScreeningPlanV2(openCodeTestConfiguration(), suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	var config openCodeQualificationConfig
	if err := json.Unmarshal(plan.InlineConfig, &config); err != nil {
		t.Fatal(err)
	}
	agent := config.Agent["qualification"]
	if plan.Transport != OpenCodeToolFreeTransportV2 || agent.Steps != 2 || agent.Tools["*"] || config.Tools["*"] || config.Permission != "deny" || agent.Permission != "deny" {
		t.Fatalf("v2 did not isolate one normal no-tool turn: %+v %+v", plan, agent)
	}
	if _, err := ValidateOpenCodeResolvedConfig(plan, resolvedOpenCodeConfigFor(plan)); err != nil {
		t.Fatalf("v2 resolved config rejected: %v", err)
	}
}

func TestOpenCodePlanV4PinsPreventiveOutputCeiling(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeScreeningPlanV4(openCodeTestConfiguration(), suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	if plan.Transport != OpenCodeToolFreeTransportV4 || !contains(plan.EnvironmentTemplate, "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=8000") {
		t.Fatalf("v4 output ceiling is not pinned: %+v", plan)
	}
	if _, err := ValidateOpenCodeResolvedConfig(plan, resolvedOpenCodeConfigFor(plan)); err != nil {
		t.Fatalf("v4 resolved config rejected: %v", err)
	}
}

func TestOpenCodePlanV5PinsReasoningCompatibleOutputCeiling(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeScreeningPlanV5(openCodeTestConfiguration(), suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	if plan.Transport != OpenCodeToolFreeTransportV5 || !contains(plan.EnvironmentTemplate, "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=20000") {
		t.Fatalf("v5 output ceiling is not pinned: %+v", plan)
	}
	if _, err := ValidateOpenCodeResolvedConfig(plan, resolvedOpenCodeConfigFor(plan)); err != nil {
		t.Fatalf("v5 resolved config rejected: %v", err)
	}
}

func TestOpenCodePlanV6PinsOneOpenRouterProviderAndDisablesFallbacks(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	configuration := openCodeTestConfiguration()
	configuration.Model = "openrouter/inclusionai/ling-3.0-flash"
	configuration.ServingProvider = "novita"
	fallbacks := false
	configuration.ProviderFallbacks = &fallbacks
	plan, err := BuildOpenCodeScreeningPlanV6(configuration, suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	var config openCodeQualificationConfig
	if err := json.Unmarshal(plan.InlineConfig, &config); err != nil {
		t.Fatal(err)
	}
	provider := config.Provider["openrouter"]
	model := provider.Models["inclusionai/ling-3.0-flash"]
	route := model.Options["provider"].(map[string]any)
	if plan.Transport != OpenCodeToolFreeTransportV6 || provider.Whitelist[0] != "inclusionai/ling-3.0-flash" || route["allow_fallbacks"] != false || provider.Headers["X-OpenRouter-Metadata"] != "enabled" {
		t.Fatalf("v6 route is not exact and receiptable: %+v %+v", plan, provider)
	}
}

func TestOpenCodePlanRejectsProviderlessModelAndPacketDrift(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	configuration := openCodeTestConfiguration()
	configuration.Model = "test-model"
	if _, err := BuildOpenCodeScreeningPlan(configuration, suite.Cases[0]); err == nil {
		t.Fatal("providerless model passed")
	}
	changed := suite.Cases[0]
	changed.CandidatePacketSHA256 = "changed"
	if _, err := BuildOpenCodeScreeningPlan(openCodeTestConfiguration(), changed); err == nil {
		t.Fatal("packet drift passed")
	}
}

func TestOpenCodeResolvedConfigRejectsInheritedMCP(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeScreeningPlan(openCodeTestConfiguration(), suite.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	safe := []byte(`{"autoupdate":false,"share":"disabled","snapshot":false,"plugin":[],"instructions":[],"permission":{"*":"deny"},"tools":{"*":false},"compaction":{"auto":false,"prune":false},"default_agent":"qualification","mcp":{},"agent":{"qualification":{"model":"opencode/test-model","variant":"medium","steps":1,"permission":{"*":"deny"},"tools":{"*":false}}}}`)
	if hash, err := ValidateOpenCodeResolvedConfig(plan, safe); err != nil || hash == "" {
		t.Fatalf("safe resolved config rejected: %s %v", hash, err)
	}
	inherited := []byte(`{"autoupdate":false,"share":"disabled","snapshot":false,"plugin":[],"instructions":[],"permission":{"*":"deny"},"tools":{"*":false},"compaction":{"auto":false,"prune":false},"default_agent":"qualification","mcp":{"inherited":{"type":"remote","url":"https://example.invalid"}},"agent":{"qualification":{"model":"opencode/test-model","variant":"medium","steps":1,"permission":{"*":"deny"},"tools":{"*":false}}}}`)
	if _, err := ValidateOpenCodeResolvedConfig(plan, inherited); err == nil {
		t.Fatal("inherited MCP passed resolved-config preflight")
	}
	missingDefaults := []byte(`{"share":"disabled","permission":{"*":"deny"},"tools":{"*":false},"default_agent":"qualification","mcp":{},"agent":{"qualification":{"model":"opencode/test-model","variant":"medium","steps":1,"permission":{"*":"deny"},"tools":{"*":false}}}}`)
	if _, err := ValidateOpenCodeResolvedConfig(plan, missingDefaults); err == nil {
		t.Fatal("missing default-on isolation settings passed preflight")
	}
}
