package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const OpenCodeTopologyActionTransportV8 = "opencode_run_json_topology_action_v8"
const OpenCodeTopologyActionOutputTokenMaxV8 = 4000

func BuildOpenCodeTopologyActionPlanForFrozenExecutorV8(frozenExecutor, candidate RealisticCandidate, request RealisticProviderRequest) (OpenCodeScreeningPlan, error) {
	if candidate != frozenExecutor {
		return OpenCodeScreeningPlan{}, fmt.Errorf("topology v8 executor configuration is not frozen")
	}
	if request.CaseVersion == "" {
		group, groupErr := CurrentRealisticWorkflowGroup()
		if groupErr != nil {
			return OpenCodeScreeningPlan{}, groupErr
		}
		for _, candidateCase := range group.Cases {
			if candidateCase.CaseID == request.CaseID {
				request.CaseVersion = candidateCase.CaseVersion
				break
			}
		}
	}
	validMaxSteps := request.MaxSteps == 15 || request.CaseID == "R02" && request.MaxSteps == 18
	if request.CaseID == "" || request.CaseVersion == "" || request.RunFamilyID == "" || request.Attempt < 1 || !validMaxSteps {
		return OpenCodeScreeningPlan{}, fmt.Errorf("realistic provider request is incomplete")
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	schema := []byte(`{"type":"object","additionalProperties":false,"required":["action"],"properties":{"action":{"type":"object","additionalProperties":false,"required":["kind"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","browser","checkpoint_and_interrupt","finish"]},"path":{"type":"string"},"query":{"type":"string"},"contents":{"type":"string"},"check":{"type":"string"},"summary":{"type":"string"}}}}}`)
	prompt := "You are in a sealed synthetic qualification case. Return exactly one JSON action matching the supplied schema. You cannot call tools directly; the deterministic controller will execute only the allowed action and return its result in the next request. Do not claim an action or proof you have not received. Never request deployment, messaging, credentials, Phone Farm, host files, arbitrary shell, or external network. Finish only when the supplied evidence proves the local outcome.\n\n<response_schema>\n" + string(schema) + "\n</response_schema>\n\n<request>\n" + string(requestJSON) + "\n</request>\n"
	config := openCodeQualificationConfig{
		Schema: "https://opencode.ai/config.json", Autoupdate: false, Share: "disabled", Snapshot: false,
		Plugin: []string{}, Instructions: []string{}, Permission: "deny", Tools: map[string]bool{"*": false},
		Compaction: map[string]bool{"auto": false, "prune": false}, DefaultAgent: "qualification",
		Agent: map[string]openCodeQualificationAgent{"qualification": {
			Description: "Pinned realistic action candidate", Mode: "primary", Model: candidate.Model,
			Variant: candidate.Effort, Prompt: "Return one controller action as bare JSON. Do not use built-in tools, prior sessions, or hidden context.",
			Steps: 2, Permission: "deny", Tools: map[string]bool{"*": false},
		}},
	}
	modelID := candidate.Model
	if len(modelID) > len("openrouter/") && modelID[:len("openrouter/")] == "openrouter/" {
		modelID = modelID[len("openrouter/"):]
	} else {
		return OpenCodeScreeningPlan{}, fmt.Errorf("realistic model must use exact OpenRouter id")
	}
	model := openCodeModelConfig{Options: map[string]any{"provider": map[string]any{"order": []string{candidate.ServingProvider}, "allow_fallbacks": false}}}
	if candidate.Effort == "screening_default" {
		model.Variants = map[string]map[string]any{"screening_default": {}}
	}
	config.Provider = map[string]openCodeProviderConfig{"openrouter": {
		Options: map[string]any{"name": "OpenRouter"}, Headers: map[string]string{"X-OpenRouter-Metadata": "enabled"},
		Whitelist: []string{modelID}, Models: map[string]openCodeModelConfig{modelID: model},
	}}
	inlineConfig, err := json.Marshal(config)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	promptHash := sha256.Sum256([]byte(prompt))
	schemaHash := sha256.Sum256(schema)
	configHash := sha256.Sum256(inlineConfig)
	fallbacks := false
	return OpenCodeScreeningPlan{
		Transport: OpenCodeTopologyActionTransportV8, CandidateID: candidate.ConfigurationID, CaseID: request.CaseID, CaseVersion: request.CaseVersion,
		Model: candidate.Model, Effort: candidate.Effort, Prompt: prompt, PromptSHA256: hex.EncodeToString(promptHash[:]),
		Schema: schema, SchemaSHA256: hex.EncodeToString(schemaHash[:]), InlineConfig: inlineConfig, InlineConfigSHA256: hex.EncodeToString(configHash[:]),
		ArgsTemplate:        []string{"--pure", "run", "--format", "json", "--model", candidate.Model, "--variant", candidate.Effort, "--agent", "qualification", "--title", "realistic-<run-family>", "--dir", "<ephemeral-empty-dir>", "<exact-prompt-via-stdin>"},
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000"},
		ServingProvider:     candidate.ServingProvider, ProviderFallbacks: &fallbacks,
	}, nil
}
