package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const OpenCodeToolFreeTransport = "opencode_run_json_tool_free_v1"
const OpenCodeToolFreeTransportV2 = "opencode_run_json_tool_free_v2"
const OpenCodeToolFreeTransportV3 = "opencode_run_json_stdin_tool_free_v3"
const OpenCodeToolFreeTransportV4 = "opencode_run_json_stdin_tool_free_output_capped_v4"
const OpenCodeToolFreeTransportV5 = "opencode_run_json_stdin_tool_free_output_capped_v5"
const OpenCodeToolFreeTransportV6 = "opencode_run_json_stdin_tool_free_routing_receipted_v6"
const OpenCodeQualificationOutputTokenMaxV4 = 8000
const OpenCodeQualificationOutputTokenMax = 20000
const OpenCodeCLIRuntimeProvider = "opencode_cli"

type OpenCodeScreeningPlan struct {
	Transport           string   `json:"transport"`
	CandidateID         string   `json:"candidate_id"`
	CaseID              string   `json:"case_id"`
	CaseVersion         string   `json:"case_version"`
	Model               string   `json:"model"`
	Effort              string   `json:"reasoning_effort"`
	Prompt              string   `json:"-"`
	PromptSHA256        string   `json:"prompt_sha256"`
	Schema              []byte   `json:"-"`
	SchemaSHA256        string   `json:"output_schema_sha256"`
	InlineConfig        []byte   `json:"-"`
	InlineConfigSHA256  string   `json:"inline_config_sha256"`
	ArgsTemplate        []string `json:"args_template"`
	EnvironmentTemplate []string `json:"environment_template"`
	ServingProvider     string   `json:"serving_provider,omitempty"`
	ProviderFallbacks   *bool    `json:"provider_fallbacks,omitempty"`
}

type openCodeQualificationAgent struct {
	Description string          `json:"description"`
	Mode        string          `json:"mode"`
	Model       string          `json:"model"`
	Variant     string          `json:"variant"`
	Prompt      string          `json:"prompt"`
	Steps       int             `json:"steps"`
	Permission  string          `json:"permission"`
	Tools       map[string]bool `json:"tools"`
}

type openCodeQualificationConfig struct {
	Schema       string                                `json:"$schema"`
	Autoupdate   bool                                  `json:"autoupdate"`
	Share        string                                `json:"share"`
	Snapshot     bool                                  `json:"snapshot"`
	Plugin       []string                              `json:"plugin"`
	Instructions []string                              `json:"instructions"`
	Permission   string                                `json:"permission"`
	Tools        map[string]bool                       `json:"tools"`
	Compaction   map[string]bool                       `json:"compaction"`
	DefaultAgent string                                `json:"default_agent"`
	Agent        map[string]openCodeQualificationAgent `json:"agent"`
	Provider     map[string]openCodeProviderConfig     `json:"provider,omitempty"`
}

type openCodeProviderConfig struct {
	Options   map[string]any                 `json:"options,omitempty"`
	Headers   map[string]string              `json:"headers,omitempty"`
	Whitelist []string                       `json:"whitelist,omitempty"`
	Models    map[string]openCodeModelConfig `json:"models,omitempty"`
}

type openCodeModelConfig struct {
	Options  map[string]any            `json:"options,omitempty"`
	Variants map[string]map[string]any `json:"variants,omitempty"`
}

type openCodeResolvedConfig struct {
	Autoupdate   *bool                      `json:"autoupdate"`
	Share        string                     `json:"share"`
	Snapshot     *bool                      `json:"snapshot"`
	Plugin       []json.RawMessage          `json:"plugin"`
	Instructions []string                   `json:"instructions"`
	Permission   json.RawMessage            `json:"permission"`
	Tools        map[string]bool            `json:"tools"`
	Compaction   map[string]bool            `json:"compaction"`
	DefaultAgent string                     `json:"default_agent"`
	MCP          map[string]json.RawMessage `json:"mcp"`
	Agent        map[string]struct {
		Model      string          `json:"model"`
		Variant    string          `json:"variant"`
		Steps      int             `json:"steps"`
		Permission json.RawMessage `json:"permission"`
		Tools      map[string]bool `json:"tools"`
	} `json:"agent"`
	Provider map[string]json.RawMessage `json:"provider"`
}

// ValidateOpenCodeResolvedConfig is the mandatory no-inference preflight for
// the fully merged configuration returned by `opencode --pure debug config`.
// Inline overrides do not erase unrelated global MCP entries; any inherited
// MCP therefore stops the trial even when permissions deny it.
func ValidateOpenCodeResolvedConfig(plan OpenCodeScreeningPlan, raw []byte) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	var config openCodeResolvedConfig
	if err := decoder.Decode(&config); err != nil {
		return "", fmt.Errorf("decode OpenCode resolved config: %w", err)
	}
	compactionAuto, compactionPinned := config.Compaction["auto"]
	globalToolsDisabled, globalToolsPinned := config.Tools["*"]
	if config.Autoupdate == nil || *config.Autoupdate || config.Share != "disabled" || config.Snapshot == nil || *config.Snapshot || len(config.Plugin) != 0 || len(config.Instructions) != 0 || len(config.MCP) != 0 || config.DefaultAgent != "qualification" || !compactionPinned || compactionAuto || !globalToolsPinned || globalToolsDisabled {
		return "", fmt.Errorf("OpenCode resolved config inherited unsafe state")
	}
	expectedSteps := 1
	if plan.Transport == OpenCodeToolFreeTransportV2 || plan.Transport == OpenCodeToolFreeTransportV3 || plan.Transport == OpenCodeToolFreeTransportV4 || plan.Transport == OpenCodeToolFreeTransportV5 || plan.Transport == OpenCodeToolFreeTransportV6 || plan.Transport == OpenCodeRealisticActionTransport {
		expectedSteps = 2
	} else if plan.Transport != OpenCodeToolFreeTransport {
		return "", fmt.Errorf("OpenCode transport is not registered")
	}
	agent, ok := config.Agent["qualification"]
	agentToolsDisabled, agentToolsPinned := agent.Tools["*"]
	if !ok || agent.Model != plan.Model || agent.Variant != plan.Effort || agent.Steps != expectedSteps || !agentToolsPinned || agentToolsDisabled || !openCodePermissionDeniesAll(config.Permission) || !openCodePermissionDeniesAll(agent.Permission) {
		return "", fmt.Errorf("OpenCode resolved qualification agent is not pinned deny-all")
	}
	if plan.Transport == OpenCodeToolFreeTransportV6 || plan.Transport == OpenCodeRealisticActionTransport {
		rawProvider, ok := config.Provider["openrouter"]
		if !ok || !resolvedOpenRouterRouteMatches(rawProvider, plan) {
			return "", fmt.Errorf("OpenCode resolved config did not preserve the exact no-fallback OpenRouter route")
		}
	}
	return sha256Hex(raw), nil
}

func resolvedOpenRouterRouteMatches(raw json.RawMessage, plan OpenCodeScreeningPlan) bool {
	var provider struct {
		Whitelist []string       `json:"whitelist"`
		Options   map[string]any `json:"options"`
		Models    map[string]struct {
			Options map[string]any `json:"options"`
		} `json:"models"`
	}
	if json.Unmarshal(raw, &provider) != nil || plan.ServingProvider == "" || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks {
		return false
	}
	modelID := strings.TrimPrefix(plan.Model, "openrouter/")
	model, ok := provider.Models[modelID]
	if !ok || len(provider.Whitelist) != 1 || provider.Whitelist[0] != modelID || fmt.Sprint(provider.Options["name"]) != "OpenRouter" {
		return false
	}
	route, ok := model.Options["provider"].(map[string]any)
	if !ok || route["allow_fallbacks"] != false {
		return false
	}
	order, ok := route["order"].([]any)
	return ok && len(order) == 1 && fmt.Sprint(order[0]) == plan.ServingProvider
}

func openCodePermissionDeniesAll(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte(`"deny"`)) {
		return true
	}
	var rules map[string]string
	return json.Unmarshal(trimmed, &rules) == nil && rules["*"] == "deny"
}

func BuildOpenCodeScreeningPlan(configuration Configuration, screeningCase ScreeningCase) (OpenCodeScreeningPlan, error) {
	return buildOpenCodeScreeningPlan(configuration, screeningCase, OpenCodeToolFreeTransport, 1)
}

// BuildOpenCodeScreeningPlanV2 leaves one normal model turn before OpenCode's
// final-step warning. With tools omitted and denied, a compliant candidate
// still makes exactly one provider call. V1 is retained as invalid historical
// evidence and is never rewritten or retried.
func BuildOpenCodeScreeningPlanV2(configuration Configuration, screeningCase ScreeningCase) (OpenCodeScreeningPlan, error) {
	return buildOpenCodeScreeningPlan(configuration, screeningCase, OpenCodeToolFreeTransportV2, 2)
}

func BuildOpenCodeScreeningPlanV3(configuration Configuration, screeningCase ScreeningCase) (OpenCodeScreeningPlan, error) {
	return buildOpenCodeScreeningPlan(configuration, screeningCase, OpenCodeToolFreeTransportV3, 2)
}

// BuildOpenCodeScreeningPlanV4 preserves the exact v3 stdin/no-tool contract
// and pins OpenCode's documented provider-output ceiling. This makes the API
// cost bound preventive instead of relying on typical model behavior.
func BuildOpenCodeScreeningPlanV4(configuration Configuration, screeningCase ScreeningCase) (OpenCodeScreeningPlan, error) {
	return buildOpenCodeScreeningPlan(configuration, screeningCase, OpenCodeToolFreeTransportV4, 2)
}

// BuildOpenCodeScreeningPlanV5 corrects v4's pre-generation conflict between
// Qwen High's 16,000-token reasoning budget and the 8,000-token total ceiling.
// V4 is retained as immutable invalid harness evidence and is never retried.
func BuildOpenCodeScreeningPlanV5(configuration Configuration, screeningCase ScreeningCase) (OpenCodeScreeningPlan, error) {
	return buildOpenCodeScreeningPlan(configuration, screeningCase, OpenCodeToolFreeTransportV5, 2)
}

func BuildOpenCodeScreeningPlanV6(configuration Configuration, screeningCase ScreeningCase) (OpenCodeScreeningPlan, error) {
	if configuration.ServingProvider == "" || configuration.ProviderFallbacks == nil || *configuration.ProviderFallbacks {
		return OpenCodeScreeningPlan{}, fmt.Errorf("v6 transport requires one serving provider and disabled fallbacks")
	}
	return buildOpenCodeScreeningPlan(configuration, screeningCase, OpenCodeToolFreeTransportV6, 2)
}

func buildOpenCodeScreeningPlan(configuration Configuration, screeningCase ScreeningCase, transport string, steps int) (OpenCodeScreeningPlan, error) {
	var plan OpenCodeScreeningPlan
	if err := ValidateConfiguration(configuration); err != nil {
		return plan, err
	}
	if configuration.RuntimeProvider != OpenCodeCLIRuntimeProvider || configuration.InstructionVersion != "shadow-candidate.v5" {
		return plan, fmt.Errorf("configuration is not pinned to the OpenCode tool-free screening contract")
	}
	if !strings.Contains(configuration.Model, "/") {
		return plan, fmt.Errorf("OpenCode model must pin provider/model")
	}
	instructionHash := sha256.Sum256(screeningCandidateInstructionsV5)
	if configuration.InstructionSHA256 != hex.EncodeToString(instructionHash[:]) {
		return plan, fmt.Errorf("candidate instruction hash mismatch")
	}
	skillHash := sha256.Sum256(screeningSkillV2)
	if configuration.SkillVersions["silver_mediums_context"] != "item14.v2-inline" || configuration.SkillContentSHA256["silver_mediums_context"] != hex.EncodeToString(skillHash[:]) {
		return plan, fmt.Errorf("candidate qualification skill is not pinned")
	}
	candidateHash, err := EmbeddedScreeningPacketVersionSHA256(screeningCase.CaseID, screeningCase.CaseVersion, "candidate")
	if err != nil || candidateHash != screeningCase.CandidatePacketSHA256 {
		return plan, fmt.Errorf("candidate packet hash mismatch")
	}
	if err := validatePublicResponseContract(screeningCase.CaseID, screeningCase.CaseVersion); err != nil {
		return plan, err
	}
	packet, err := CandidateScreeningPacketVersion(screeningCase.CaseID, screeningCase.CaseVersion)
	if err != nil {
		return plan, err
	}
	schema, err := directOutputSchema(screeningCase.CaseID, screeningCase.CaseVersion)
	if err != nil {
		return plan, err
	}
	prompt := strings.TrimSpace(string(screeningCandidateInstructionsV5)) +
		"\n\n<qualification-policy>\n" + strings.TrimSpace(string(screeningSkillV2)) +
		"\n</qualification-policy>\n\n<case-packet>\n" + strings.TrimSpace(string(packet)) + "\n</case-packet>\n"
	config := openCodeQualificationConfig{
		Schema: "https://opencode.ai/config.json", Autoupdate: false, Share: "disabled", Snapshot: false,
		Plugin: []string{}, Instructions: []string{}, Permission: "deny", Tools: map[string]bool{"*": false},
		Compaction: map[string]bool{"auto": false, "prune": false}, DefaultAgent: "qualification",
		Agent: map[string]openCodeQualificationAgent{"qualification": {
			Description: "Pinned tool-free qualification candidate", Mode: "primary", Model: configuration.Model,
			Variant: configuration.ReasoningEffort, Prompt: "Answer only the supplied qualification case. Do not use tools, subagents, files, network access, or prior sessions.",
			Steps: steps, Permission: "deny", Tools: map[string]bool{"*": false},
		}},
	}
	if transport == OpenCodeToolFreeTransportV6 {
		modelID := strings.TrimPrefix(configuration.Model, "openrouter/")
		model := openCodeModelConfig{Options: map[string]any{"provider": map[string]any{"order": []string{configuration.ServingProvider}, "allow_fallbacks": false}}}
		if configuration.ReasoningEffort == "screening_default" {
			model.Variants = map[string]map[string]any{"screening_default": {}}
		}
		config.Provider = map[string]openCodeProviderConfig{"openrouter": {
			Options: map[string]any{"name": "OpenRouter"}, Headers: map[string]string{"X-OpenRouter-Metadata": "enabled"},
			Whitelist: []string{modelID}, Models: map[string]openCodeModelConfig{modelID: model},
		}}
	}
	inlineConfig, err := json.Marshal(config)
	if err != nil {
		return plan, err
	}
	argsTemplate := []string{"--pure", "run", "--format", "json", "--model", configuration.Model, "--variant", configuration.ReasoningEffort, "--agent", "qualification", "--title", "qualification-<reservation-id>", "--dir", "<ephemeral-empty-dir>", "<prompt>"}
	if transport == OpenCodeToolFreeTransportV3 || transport == OpenCodeToolFreeTransportV4 || transport == OpenCodeToolFreeTransportV5 || transport == OpenCodeToolFreeTransportV6 {
		argsTemplate[len(argsTemplate)-1] = "<exact-prompt-via-stdin>"
	}
	plan = OpenCodeScreeningPlan{
		Transport: transport, CandidateID: configuration.ID, CaseID: screeningCase.CaseID, CaseVersion: screeningCase.CaseVersion,
		Model: configuration.Model, Effort: configuration.ReasoningEffort, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)),
		Schema: schema, SchemaSHA256: sha256Hex(schema), InlineConfig: inlineConfig, InlineConfigSHA256: sha256Hex(inlineConfig),
		ArgsTemplate:        argsTemplate,
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true"},
		ServingProvider:     configuration.ServingProvider, ProviderFallbacks: configuration.ProviderFallbacks,
	}
	if transport == OpenCodeToolFreeTransportV4 {
		plan.EnvironmentTemplate = append(plan.EnvironmentTemplate, fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", OpenCodeQualificationOutputTokenMaxV4))
	}
	if transport == OpenCodeToolFreeTransportV5 {
		plan.EnvironmentTemplate = append(plan.EnvironmentTemplate, fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", OpenCodeQualificationOutputTokenMax))
	}
	if transport == OpenCodeToolFreeTransportV6 {
		plan.EnvironmentTemplate = append(plan.EnvironmentTemplate, fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", OpenCodeQualificationOutputTokenMaxV4))
	}
	return plan, nil
}
