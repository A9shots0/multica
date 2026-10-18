package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const OpenCodeTopologyReviewTransportV7 = "opencode_run_json_topology_review_v7"

// BuildOpenCodeTopologyV7ReviewPlanForFrozenReviewer binds a review plan to the
// reviewer frozen by the calling experiment generation. This v7-specific
// builder avoids changing the already-frozen v1-v6 review transport bytes.
func BuildOpenCodeTopologyV7ReviewPlanForFrozenReviewer(frozenReviewer, candidate RealisticCandidate, request TopologyReviewRequest) (OpenCodeScreeningPlan, error) {
	if candidate != frozenReviewer {
		return OpenCodeScreeningPlan{}, fmt.Errorf("topology reviewer configuration is not frozen")
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	schema := []byte(`{"type":"object","additionalProperties":false,"required":["verdict","failed_criteria","evidence","required_correction","owner_decision_required"],"properties":{"verdict":{"type":"string","enum":["PASS","FAIL"]},"failed_criteria":{"type":"array","items":{"type":"string"}},"evidence":{"type":"array","items":{"type":"string"}},"required_correction":{"type":"string"},"owner_decision_required":{"type":"boolean"}}}`)
	prompt := "Independently review one sealed synthetic result against only the public criteria. Return exactly one JSON verdict matching the schema. Use only listed evidence identifiers. Do not edit, execute tools, infer hidden grader rules, request owner input, expand scope, deploy, message, access credentials, or access Phone Farm. PASS requires complete public evidence. FAIL must name public failed criteria and the smallest local correction.\n\n<response_schema>\n" + string(schema) + "\n</response_schema>\n\n<review_request>\n" + string(requestJSON) + "\n</review_request>\n"
	config := openCodeQualificationConfig{
		Schema: "https://opencode.ai/config.json", Autoupdate: false, Share: "disabled", Snapshot: false,
		Plugin: []string{}, Instructions: []string{}, Permission: "deny", Tools: map[string]bool{"*": false}, Compaction: map[string]bool{"auto": false, "prune": false}, DefaultAgent: "qualification",
		Agent: map[string]openCodeQualificationAgent{"qualification": {Description: "Pinned independent topology reviewer", Mode: "primary", Model: candidate.Model, Variant: candidate.Effort, Prompt: "Return one evidence-bound review verdict as bare JSON.", Steps: 2, Permission: "deny", Tools: map[string]bool{"*": false}}},
	}
	modelID := strings.TrimPrefix(candidate.Model, "openrouter/")
	if modelID == candidate.Model {
		return OpenCodeScreeningPlan{}, fmt.Errorf("topology reviewer must use exact OpenRouter model id")
	}
	config.Provider = map[string]openCodeProviderConfig{"openrouter": {Options: map[string]any{"name": "OpenRouter"}, Headers: map[string]string{"X-OpenRouter-Metadata": "enabled"}, Whitelist: []string{modelID}, Models: map[string]openCodeModelConfig{modelID: {Options: map[string]any{"provider": map[string]any{"order": []string{candidate.ServingProvider}, "allow_fallbacks": false}}}}}}
	inlineConfig, err := json.Marshal(config)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	promptHash, schemaHash, configHash := sha256.Sum256([]byte(prompt)), sha256.Sum256(schema), sha256.Sum256(inlineConfig)
	fallbacks := false
	return OpenCodeScreeningPlan{
		Transport: OpenCodeTopologyReviewTransportV7, CandidateID: candidate.ConfigurationID, CaseID: request.CaseID, CaseVersion: request.CaseVersion,
		Model: candidate.Model, Effort: candidate.Effort, Prompt: prompt, PromptSHA256: hex.EncodeToString(promptHash[:]), Schema: schema, SchemaSHA256: hex.EncodeToString(schemaHash[:]), InlineConfig: inlineConfig, InlineConfigSHA256: hex.EncodeToString(configHash[:]),
		ArgsTemplate:        []string{"--pure", "run", "--format", "json", "--model", candidate.Model, "--variant", candidate.Effort, "--agent", "qualification", "--title", "topology-review-<run-id>", "--dir", "<ephemeral-empty-dir>", "<exact-prompt-via-stdin>"},
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000"},
		ServingProvider:     candidate.ServingProvider, ProviderFallbacks: &fallbacks,
	}, nil
}

func ValidateTopologyV7PlanBinding(expected RealisticCandidate, plan OpenCodeScreeningPlan, transport string) error {
	if plan.Transport != transport || plan.CandidateID != expected.ConfigurationID || plan.Model != expected.Model || plan.Effort != expected.Effort || plan.ServingProvider != expected.ServingProvider || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks || !validSHA256(plan.PromptSHA256) || !validSHA256(plan.SchemaSHA256) || !validSHA256(plan.InlineConfigSHA256) || !bytes.Equal([]byte(plan.InlineConfigSHA256), []byte(sha256Hex(plan.InlineConfig))) {
		return fmt.Errorf("topology v7 plan is not exactly bound")
	}
	return nil
}

func ValidateTopologyV7ResolvedConfig(plan OpenCodeScreeningPlan, raw []byte) (string, error) {
	if plan.Transport != OpenCodeTopologyActionTransportV7 && plan.Transport != OpenCodeTopologyReviewTransportV7 {
		return "", fmt.Errorf("topology v7 transport is not registered")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var config openCodeResolvedConfig
	if err := decoder.Decode(&config); err != nil {
		return "", fmt.Errorf("decode topology v7 resolved config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return "", fmt.Errorf("decode trailing topology v7 resolved config: %w", err)
		}
		return "", fmt.Errorf("topology v7 resolved config contains trailing data")
	}
	compactionAuto, compactionPinned := config.Compaction["auto"]
	globalToolsDisabled, globalToolsPinned := config.Tools["*"]
	if config.Autoupdate == nil || *config.Autoupdate || config.Share != "disabled" || config.Snapshot == nil || *config.Snapshot || len(config.Plugin) != 0 || len(config.Instructions) != 0 || len(config.MCP) != 0 || config.DefaultAgent != "qualification" || !compactionPinned || compactionAuto || !globalToolsPinned || globalToolsDisabled {
		return "", fmt.Errorf("topology v7 resolved config inherited unsafe state")
	}
	agent, ok := config.Agent["qualification"]
	agentToolsDisabled, agentToolsPinned := agent.Tools["*"]
	if !ok || agent.Model != plan.Model || agent.Variant != plan.Effort || agent.Steps != 2 || !agentToolsPinned || agentToolsDisabled || !openCodePermissionDeniesAll(config.Permission) || !openCodePermissionDeniesAll(agent.Permission) {
		return "", fmt.Errorf("topology v7 resolved qualification agent is not pinned deny-all")
	}
	rawProvider, ok := config.Provider["openrouter"]
	if !ok || len(config.Provider) != 1 || !resolvedOpenRouterRouteMatches(rawProvider, plan) {
		return "", fmt.Errorf("topology v7 resolved config did not preserve exact no-fallback route")
	}
	return sha256Hex(raw), nil
}
