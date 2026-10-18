package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const ClaudeCodeToolFreeTransport = "claude_code_print_tool_free_v1"
const ClaudeCodeSubscriptionToolFreeTransport = "claude_code_print_restricted_tool_free_v2"

type ClaudeScreeningPlan struct {
	Transport    string   `json:"transport"`
	CandidateID  string   `json:"candidate_id"`
	CaseID       string   `json:"case_id"`
	CaseVersion  string   `json:"case_version"`
	Model        string   `json:"model"`
	Effort       string   `json:"reasoning_effort"`
	Prompt       string   `json:"-"`
	PromptSHA256 string   `json:"prompt_sha256"`
	Schema       []byte   `json:"-"`
	SchemaSHA256 string   `json:"output_schema_sha256"`
	ArgsTemplate []string `json:"args_template"`
}

func BuildClaudeScreeningPlan(configuration Configuration, screeningCase ScreeningCase) (ClaudeScreeningPlan, error) {
	return buildClaudeScreeningPlan(configuration, screeningCase, ClaudeCodeToolFreeTransport, true)
}

// BuildClaudeSubscriptionScreeningPlan preserves Claude Code's signed-in
// subscription authentication while retaining the preventive restricted,
// tool-free, MCP-free and ephemeral execution boundary. Claude Code --bare is
// intentionally excluded because it disables OAuth/keychain reads and would
// silently move this observation off the declared subscription lane.
func BuildClaudeSubscriptionScreeningPlan(configuration Configuration, screeningCase ScreeningCase) (ClaudeScreeningPlan, error) {
	return buildClaudeScreeningPlan(configuration, screeningCase, ClaudeCodeSubscriptionToolFreeTransport, false)
}

func buildClaudeScreeningPlan(configuration Configuration, screeningCase ScreeningCase, transport string, bare bool) (ClaudeScreeningPlan, error) {
	var plan ClaudeScreeningPlan
	if err := ValidateConfiguration(configuration); err != nil {
		return plan, err
	}
	if configuration.RuntimeProvider != "claude_code_subscription" || configuration.InstructionVersion != "shadow-candidate.v5" {
		return plan, fmt.Errorf("configuration is not pinned to the Claude tool-free screening contract")
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
	args := []string{"-p"}
	if bare {
		args = append(args, "--bare")
	}
	args = append(args, "--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--no-session-persistence", "--permission-prompts", "none", "--output-format", "json", "--json-schema", "<generated-schema>", "--max-turns", "1", "--model", configuration.Model, "--effort", configuration.ReasoningEffort)
	plan = ClaudeScreeningPlan{
		Transport: transport, CandidateID: configuration.ID, CaseID: screeningCase.CaseID, CaseVersion: screeningCase.CaseVersion,
		Model: configuration.Model, Effort: configuration.ReasoningEffort, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)), Schema: schema, SchemaSHA256: sha256Hex(schema),
		ArgsTemplate: args,
	}
	return plan, nil
}
