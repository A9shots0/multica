package agentqualification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRealisticOpenCodeActionPlanPinsRouteAndDeniesBuiltInTools(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	request := RealisticProviderRequest{CaseID: "R01", RunFamilyID: "fixture", Attempt: 1, Task: "repair", AllowedActions: []string{"read", "finish"}, MaxSteps: 15}
	plan, err := BuildOpenCodeRealisticActionPlan(matrix.Candidates[0], request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Transport != OpenCodeRealisticActionTransport || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks || plan.ServingProvider != "alibaba" || plan.Model != "openrouter/qwen/qwen3-coder-flash" {
		t.Fatalf("realistic OpenCode route drifted: %+v", plan)
	}
	var config openCodeQualificationConfig
	if err := json.Unmarshal(plan.InlineConfig, &config); err != nil {
		t.Fatal(err)
	}
	if config.Permission != "deny" || config.Tools["*"] || config.Agent["qualification"].Permission != "deny" || config.Agent["qualification"].Tools["*"] {
		t.Fatal("OpenCode built-in tools are not denied")
	}
	provider := config.Provider["openrouter"]
	if len(provider.Whitelist) != 1 || provider.Whitelist[0] != "qwen/qwen3-coder-flash" {
		t.Fatal("OpenRouter whitelist is not exact")
	}
	schemaEnvelope := "<response_schema>\n" + string(plan.Schema) + "\n</response_schema>"
	if strings.Count(plan.Prompt, schemaEnvelope) != 1 || !strings.Contains(plan.Prompt, "<request>\n") {
		t.Fatal("candidate prompt does not carry the exact frozen response schema and request")
	}
}

func TestRealisticOpenCodeActionPlanRejectsCandidateOutsideMatrix(t *testing.T) {
	request := RealisticProviderRequest{CaseID: "R01", RunFamilyID: "fixture", Attempt: 1, Task: "repair", MaxSteps: 15}
	_, err := BuildOpenCodeRealisticActionPlan(RealisticCandidate{ConfigurationID: "other", Model: "openrouter/other/model", Effort: "low", ServingProvider: "other"}, request)
	if err == nil {
		t.Fatal("candidate outside matrix was accepted")
	}
}

func TestRealisticOpenCodeActionPlanAllowsOnlyR02CorrectionBudget(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v9")
	request := RealisticProviderRequest{CaseID: "R02", RunFamilyID: "fixture", Attempt: 1, Task: "repair", MaxSteps: 18}
	if _, err := BuildOpenCodeRealisticActionPlan(matrix.Candidates[0], request); err != nil {
		t.Fatalf("R02 correction budget was rejected: %v", err)
	}
	request.CaseID = "R01"
	if _, err := BuildOpenCodeRealisticActionPlan(matrix.Candidates[0], request); err == nil {
		t.Fatal("R02-only correction budget leaked to another case")
	}
}
