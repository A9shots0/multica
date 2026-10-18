package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// BuildOpenCodeTopologyReviewPlanForFrozenReviewer binds a review plan to the
// reviewer frozen by the calling experiment generation. This v5-specific
// builder avoids changing the already-frozen v1-v4 review transport bytes.
func BuildOpenCodeTopologyReviewPlanForFrozenReviewer(frozenReviewer, candidate RealisticCandidate, request TopologyReviewRequest) (OpenCodeScreeningPlan, error) {
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
		Transport: OpenCodeTopologyReviewTransport, CandidateID: candidate.ConfigurationID, CaseID: request.CaseID, CaseVersion: request.CaseVersion,
		Model: candidate.Model, Effort: candidate.Effort, Prompt: prompt, PromptSHA256: hex.EncodeToString(promptHash[:]), Schema: schema, SchemaSHA256: hex.EncodeToString(schemaHash[:]), InlineConfig: inlineConfig, InlineConfigSHA256: hex.EncodeToString(configHash[:]),
		ArgsTemplate:        []string{"--pure", "run", "--format", "json", "--model", candidate.Model, "--variant", candidate.Effort, "--agent", "qualification", "--title", "topology-review-<run-id>", "--dir", "<ephemeral-empty-dir>", "<exact-prompt-via-stdin>"},
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000"},
		ServingProvider:     candidate.ServingProvider, ProviderFallbacks: &fallbacks,
	}, nil
}
