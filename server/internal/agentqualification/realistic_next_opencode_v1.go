package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// RequireRealisticNextReviewedFreeze is the sole paid-launch gate for R05/R06.
// Planning remains available while review is pending, but execution does not.
func RequireRealisticNextReviewedFreeze() error {
	group, err := CurrentRealisticNextGroup()
	if err != nil {
		return err
	}
	matrix, err := CurrentRealisticNextRunMatrix()
	if err != nil {
		return err
	}
	freeze, err := CurrentRealisticNextFreeze()
	if err != nil {
		return err
	}
	if group.Status != "frozen_reviewed" || matrix.Status != "frozen_reviewed" || freeze.Status != "frozen_reviewed" || !freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "go" {
		return fmt.Errorf("realistic-next provider execution is locked until group and matrix are independently reviewed and frozen")
	}
	return nil
}

func BuildOpenCodeRealisticNextActionPlan(candidate RealisticCandidate, request RealisticProviderRequest) (OpenCodeScreeningPlan, error) {
	matrix, err := CurrentRealisticNextRunMatrix()
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	found := false
	for _, exact := range matrix.Candidates {
		if exact == candidate {
			found = true
			break
		}
	}
	if !found {
		return OpenCodeScreeningPlan{}, fmt.Errorf("candidate is not an exact realistic-next matrix configuration")
	}
	if request.CaseVersion == "" {
		request.CaseVersion = "v1"
	}
	if !contains([]string{"R05", "R06"}, request.CaseID) || request.CaseVersion != "v1" || request.RunFamilyID == "" || request.Attempt != 1 || request.MaxSteps < 1 || request.MaxSteps > 64 {
		return OpenCodeScreeningPlan{}, fmt.Errorf("realistic-next provider request is incomplete")
	}
	if !exactSet(request.AllowedActions, "read", "search", "edit", "run_check", "finish") {
		return OpenCodeScreeningPlan{}, fmt.Errorf("realistic-next action boundary drifted")
	}
	expectedChecks := []string{R05V1SimulateCheck, R05V1VerifyCheck}
	if request.CaseID == "R06" {
		expectedChecks = []string{R06V1ContractCheck, R06V1ReleaseCRM, R06V1ReleaseLinkHub, R06V1ReleaseCanopy, R06V1AuthenticatedVerify, R06V1Rollback, R06V1VerifyBaseline}
	}
	if !exactSet(request.AllowedChecks, expectedChecks...) {
		return OpenCodeScreeningPlan{}, fmt.Errorf("realistic-next check boundary drifted")
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	schema := []byte(`{"type":"object","additionalProperties":false,"required":["action"],"properties":{"action":{"type":"object","additionalProperties":false,"required":["kind"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","finish"]},"path":{"type":"string"},"query":{"type":"string"},"contents":{"type":"string"},"check":{"type":"string"},"summary":{"type":"string"}}}}}`)
	prompt := "You are in a sealed synthetic qualification case. Return exactly one JSON action matching the supplied schema. You cannot call tools directly; the deterministic controller executes only the disclosed action and returns its result next turn. Do not claim actions or proof you have not received. Never request deployment, messaging, credentials, Phone Farm, host files, arbitrary shell, Browser, or external network. Finish only when local evidence proves the requested bounded outcome.\n\n<response_schema>\n" + string(schema) + "\n</response_schema>\n\n<request>\n" + string(requestJSON) + "\n</request>\n"
	modelID := strings.TrimPrefix(candidate.Model, "openrouter/")
	if modelID == candidate.Model || modelID == "" {
		return OpenCodeScreeningPlan{}, fmt.Errorf("realistic-next model must use an exact OpenRouter id")
	}
	model := openCodeModelConfig{Options: map[string]any{"provider": map[string]any{"order": []string{candidate.ServingProvider}, "allow_fallbacks": false}}}
	config := openCodeQualificationConfig{
		Schema: "https://opencode.ai/config.json", Autoupdate: false, Share: "disabled", Snapshot: false,
		Plugin: []string{}, Instructions: []string{}, Permission: "deny", Tools: map[string]bool{"*": false},
		Compaction: map[string]bool{"auto": false, "prune": false}, DefaultAgent: "qualification",
		Agent:    map[string]openCodeQualificationAgent{"qualification": {Description: "Pinned realistic-next action candidate", Mode: "primary", Model: candidate.Model, Variant: candidate.Effort, Prompt: "Return one controller action as bare JSON. Do not use built-in tools, prior sessions, or hidden context.", Steps: 2, Permission: "deny", Tools: map[string]bool{"*": false}}},
		Provider: map[string]openCodeProviderConfig{"openrouter": {Options: map[string]any{"name": "OpenRouter"}, Headers: map[string]string{"X-OpenRouter-Metadata": "enabled"}, Whitelist: []string{modelID}, Models: map[string]openCodeModelConfig{modelID: model}}},
	}
	inlineConfig, err := json.Marshal(config)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	fallbacks := false
	return OpenCodeScreeningPlan{
		Transport: OpenCodeRealisticActionTransport, CandidateID: candidate.ConfigurationID, CaseID: request.CaseID, CaseVersion: request.CaseVersion,
		Model: candidate.Model, Effort: candidate.Effort, Prompt: prompt, PromptSHA256: hashRealisticNextBytes([]byte(prompt)), Schema: schema, SchemaSHA256: hashRealisticNextBytes(schema), InlineConfig: inlineConfig, InlineConfigSHA256: hashRealisticNextBytes(inlineConfig),
		ArgsTemplate:        []string{"--pure", "run", "--format", "json", "--model", candidate.Model, "--variant", candidate.Effort, "--agent", "qualification", "--title", "realistic-next-<run-family>", "--dir", "<ephemeral-empty-dir>", "<exact-prompt-via-stdin>"},
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", OpenCodeRealisticActionOutputTokenMax)},
		ServingProvider:     candidate.ServingProvider, ProviderFallbacks: &fallbacks,
	}, nil
}

func hashRealisticNextBytes(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
