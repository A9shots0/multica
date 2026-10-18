package agentqualification

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const RoleQualificationExecutionSchemaV2 = "silver-mediums.role-qualification-execution.v2"

type RoleQualificationObservationCapsV2 struct {
	MaxCalls        int   `json:"max_calls"`
	MaxInputTokens  int64 `json:"max_input_tokens"`
	MaxOutputTokens int64 `json:"max_output_tokens"`
	MaxTotalTokens  int64 `json:"max_total_tokens"`
	MaxWallSeconds  int64 `json:"max_wall_seconds"`
}
type RoleQualificationCampaignCapsV2 struct {
	MaxCalls                int   `json:"max_calls"`
	MaxInputTokens          int64 `json:"max_input_tokens"`
	MaxOutputTokens         int64 `json:"max_output_tokens"`
	MaxTotalTokens          int64 `json:"max_total_tokens"`
	MaxWallSeconds          int64 `json:"max_wall_seconds"`
	MaxOpenRouterCashMicros int64 `json:"max_openrouter_cash_micros"`
}
type RoleQualificationExecutionDefinitionV2 struct {
	SchemaVersion                string                             `json:"schema_version"`
	ExecutionID                  string                             `json:"execution_id"`
	Status                       string                             `json:"status"`
	V1FreezeSHA256               string                             `json:"v1_freeze_sha256"`
	ProviderCallsAllowed         bool                               `json:"provider_calls_allowed"`
	RuntimeWiring                string                             `json:"runtime_wiring"`
	FreshPreflightRequired       bool                               `json:"fresh_preflight_required"`
	Retries                      int                                `json:"retries"`
	Fallbacks                    bool                               `json:"fallbacks"`
	PerObservation               RoleQualificationObservationCapsV2 `json:"per_observation"`
	Campaign                     RoleQualificationCampaignCapsV2    `json:"campaign"`
	ClaudeSubscriptionAccounting string                             `json:"claude_subscription_accounting"`
	OrderPolicy                  string                             `json:"order_policy"`
	ProviderGate                 string                             `json:"provider_gate"`
	RoleModelSelectionAllowed    bool                               `json:"role_model_selection_allowed"`
	ProductionAllowed            bool                               `json:"production_allowed"`
	ExternalSideEffectsAllowed   bool                               `json:"external_side_effects_allowed"`
}

type RoleQualificationStructuredSchemaV2 struct {
	Type                 string                       `json:"type"`
	Properties           map[string]map[string]string `json:"properties"`
	Required             []string                     `json:"required"`
	AdditionalProperties bool                         `json:"additionalProperties"`
}
type RoleQualificationExecutionPacketV2 struct {
	CaseID              string                              `json:"case_id"`
	CandidateClass      string                              `json:"candidate_class"`
	RoleInstructions    string                              `json:"role_instructions"`
	RoleSkill           string                              `json:"role_skill"`
	Scenario            string                              `json:"scenario"`
	RequiredArtifactIDs []string                            `json:"required_artifact_ids"`
	ResponseSchema      RoleQualificationStructuredSchemaV2 `json:"response_schema"`
}
type RoleQualificationExecutionCellV2 struct {
	Ordinal            int                                   `json:"ordinal"`
	CellID             string                                `json:"cell_id"`
	ConfigurationClass roleQualificationConfigurationClassV1 `json:"configuration_class"`
	CandidateClass     string                                `json:"candidate_class"`
	CaseID             string                                `json:"case_id"`
	Packet             RoleQualificationExecutionPacketV2    `json:"candidate_packet"`
	PacketSHA256       string                                `json:"candidate_packet_sha256"`
	MaxCalls           int                                   `json:"max_calls"`
	Retries            int                                   `json:"retries"`
	Fallbacks          bool                                  `json:"fallbacks"`
}
type RoleQualificationExecutionPlanV2 struct {
	Status                string                             `json:"status"`
	ProviderCallsAllowed  bool                               `json:"provider_calls_allowed"`
	RuntimeWiring         string                             `json:"runtime_wiring"`
	V1FreezeSHA256        string                             `json:"v1_freeze_sha256"`
	ExecutionFreezeSHA256 string                             `json:"execution_freeze_sha256"`
	Cells                 []RoleQualificationExecutionCellV2 `json:"cells"`
	PerObservation        RoleQualificationObservationCapsV2 `json:"per_observation"`
	Campaign              RoleQualificationCampaignCapsV2    `json:"campaign"`
}

//go:embed role_qualification_execution_v2.json
var roleQualificationExecutionV2JSON []byte

func CurrentRoleQualificationExecutionDefinitionV2() (RoleQualificationExecutionDefinitionV2, error) {
	var definition RoleQualificationExecutionDefinitionV2
	if err := strictJSON(roleQualificationExecutionV2JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode role qualification execution v2: %w", err)
	}
	if err := ValidateRoleQualificationExecutionDefinitionV2(definition); err != nil {
		return RoleQualificationExecutionDefinitionV2{}, err
	}
	return definition, nil
}

func ValidateRoleQualificationExecutionDefinitionV2(d RoleQualificationExecutionDefinitionV2) error {
	if d.SchemaVersion != RoleQualificationExecutionSchemaV2 || d.ExecutionID != "role-qualification-comparison-v2" || d.Status != "frozen_provider_locked" || d.V1FreezeSHA256 != "b047faaec585beee2cf00640f53e35d2b09b54618c6923c6b9458d2851d1b2e4" {
		return fmt.Errorf("role qualification execution v2 identity drifted")
	}
	if RoleQualificationFreezeV1SHA256() != d.V1FreezeSHA256 {
		return fmt.Errorf("role qualification execution v2 v1 freeze drifted")
	}
	if d.ProviderCallsAllowed || d.RuntimeWiring != "none" || !d.FreshPreflightRequired || d.Retries != 0 || d.Fallbacks || d.RoleModelSelectionAllowed || d.ProductionAllowed || d.ExternalSideEffectsAllowed {
		return fmt.Errorf("role qualification execution v2 provider or authority gate opened")
	}
	if d.PerObservation != (RoleQualificationObservationCapsV2{1, 20000, 4000, 24000, 180}) || d.Campaign != (RoleQualificationCampaignCapsV2{36, 720000, 144000, 864000, 5400, 10000000}) {
		return fmt.Errorf("role qualification execution v2 caps drifted")
	}
	if d.ClaudeSubscriptionAccounting != "provider_cost_equivalent_micros_separate_not_marginal_cash" || d.OrderPolicy != "three_class_four_role_counterbalance_with_rotated_three_case_order" || d.ProviderGate != "disabled_until_exact_v2_freeze_and_fresh_transport_preflight_validate" {
		return fmt.Errorf("role qualification execution v2 policy drifted")
	}
	return nil
}

func BuildRoleQualificationExecutionPlanV2() (RoleQualificationExecutionPlanV2, error) {
	d, err := CurrentRoleQualificationExecutionDefinitionV2()
	if err != nil {
		return RoleQualificationExecutionPlanV2{}, err
	}
	if _, err := CurrentRoleQualificationExecutionFreezeV2(); err != nil {
		return RoleQualificationExecutionPlanV2{}, err
	}
	if _, err := CurrentRoleQualificationFreezeV1(); err != nil {
		return RoleQualificationExecutionPlanV2{}, err
	}
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		return RoleQualificationExecutionPlanV2{}, err
	}
	var matrix roleQualificationRunMatrixFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &matrix); err != nil {
		return RoleQualificationExecutionPlanV2{}, err
	}
	if err := validateRunMatrixV1(group, matrix); err != nil {
		return RoleQualificationExecutionPlanV2{}, err
	}
	packetSpecs, err := CurrentRoleQualificationCandidatePacketsV1()
	if err != nil {
		return RoleQualificationExecutionPlanV2{}, err
	}
	specByCase := map[string]RoleQualificationCandidatePacketV1{}
	for _, p := range packetSpecs {
		specByCase[p.CaseID] = p
	}
	bindings := map[string]roleQualificationRoleBindingV1{}
	for _, b := range matrix.RoleBindings {
		bindings[b.CandidateClass] = b
	}
	caseByID := map[string]RoleQualificationCase{}
	for _, c := range group.Cases {
		caseByID[c.CaseID] = c
	}
	cells := make([]RoleQualificationExecutionCellV2, 0, 36)
	for ci, config := range matrix.ConfigurationClasses {
		for ri := 0; ri < len(matrix.RoleBindings); ri++ {
			binding := matrix.RoleBindings[(ri+ci)%len(matrix.RoleBindings)]
			for ki := 0; ki < len(binding.CaseIDs); ki++ {
				caseID := binding.CaseIDs[(ki+ci+ri)%len(binding.CaseIDs)]
				c := caseByID[caseID]
				visible := specByCase[caseID]
				packet, err := buildRoleQualificationExecutionPacketV2(binding, c, visible)
				if err != nil {
					return RoleQualificationExecutionPlanV2{}, err
				}
				raw, _ := json.Marshal(packet)
				ordinal := len(cells) + 1
				cellID := fmt.Sprintf("rqv2-%02d-%s-%s-%s", ordinal, shortRoleQualificationClassV2(config.ClassID), binding.CandidateClass, strings.ToLower(caseID))
				cells = append(cells, RoleQualificationExecutionCellV2{Ordinal: ordinal, CellID: cellID, ConfigurationClass: config, CandidateClass: binding.CandidateClass, CaseID: caseID, Packet: packet, PacketSHA256: sha256Hex(raw), MaxCalls: 1, Retries: 0, Fallbacks: false})
			}
		}
	}
	if len(cells) != 36 {
		return RoleQualificationExecutionPlanV2{}, fmt.Errorf("role qualification execution v2 plan count drifted")
	}
	return RoleQualificationExecutionPlanV2{Status: "frozen_provider_locked_not_executed", ProviderCallsAllowed: false, RuntimeWiring: "none", V1FreezeSHA256: d.V1FreezeSHA256, ExecutionFreezeSHA256: RoleQualificationExecutionFreezeV2SHA256(), Cells: cells, PerObservation: d.PerObservation, Campaign: d.Campaign}, nil
}

func buildRoleQualificationExecutionPacketV2(binding roleQualificationRoleBindingV1, c RoleQualificationCase, visible RoleQualificationCandidatePacketV1) (RoleQualificationExecutionPacketV2, error) {
	instructionBase := strings.TrimSuffix(binding.InstructionArtifactID, "_instructions_v1")
	skillBase := strings.TrimSuffix(binding.SkillArtifactID, "_skill_v1")
	if instructionBase == binding.InstructionArtifactID || skillBase == binding.SkillArtifactID || instructionBase != skillBase {
		return RoleQualificationExecutionPacketV2{}, fmt.Errorf("role artifact binding invalid")
	}
	instructionName := "role_artifacts/v1/" + instructionBase + ".instructions.md"
	skillName := "role_artifacts/v1/" + skillBase + ".skill.md"
	instructions, _ := roleQualificationV1Files.ReadFile(instructionName)
	skill, _ := roleQualificationV1Files.ReadFile(skillName)
	props := map[string]map[string]string{}
	for _, field := range visible.ResponseSchema {
		props[field] = map[string]string{"type": "string"}
	}
	return RoleQualificationExecutionPacketV2{CaseID: c.CaseID, CandidateClass: binding.CandidateClass, RoleInstructions: string(instructions), RoleSkill: string(skill), Scenario: c.Scenario, RequiredArtifactIDs: append([]string(nil), c.RequiredArtifactIDs...), ResponseSchema: RoleQualificationStructuredSchemaV2{Type: "object", Properties: props, Required: append([]string(nil), visible.ResponseSchema...), AdditionalProperties: false}}, nil
}

func ValidateAndGradeRoleQualificationExecutionOutputV2(caseID string, raw []byte) ([]byte, RoleQualificationGradeV1, error) {
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		return nil, RoleQualificationGradeV1{}, err
	}
	var cell *RoleQualificationExecutionCellV2
	for i := range plan.Cells {
		if plan.Cells[i].CaseID == caseID {
			cell = &plan.Cells[i]
			break
		}
	}
	if cell == nil {
		return nil, RoleQualificationGradeV1{}, fmt.Errorf("unknown role qualification case %s", caseID)
	}
	var values map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&values); err != nil {
		return nil, RoleQualificationGradeV1{}, fmt.Errorf("structured output invalid: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, RoleQualificationGradeV1{}, fmt.Errorf("structured output trailing JSON")
	}
	if len(values) != len(cell.Packet.ResponseSchema.Required) {
		return nil, RoleQualificationGradeV1{}, fmt.Errorf("structured output field count drifted")
	}
	normalized := map[string]string{}
	parts := []string{}
	for _, field := range cell.Packet.ResponseSchema.Required {
		valueRaw, ok := values[field]
		if !ok {
			return nil, RoleQualificationGradeV1{}, fmt.Errorf("structured output missing %s", field)
		}
		var value string
		if err := json.Unmarshal(valueRaw, &value); err != nil || strings.TrimSpace(value) == "" {
			return nil, RoleQualificationGradeV1{}, fmt.Errorf("structured output field %s must be a nonempty string", field)
		}
		normalized[field] = value
		parts = append(parts, value)
	}
	for field := range values {
		if _, ok := cell.Packet.ResponseSchema.Properties[field]; !ok {
			return nil, RoleQualificationGradeV1{}, fmt.Errorf("structured output unexpected field %s", field)
		}
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return nil, RoleQualificationGradeV1{}, err
	}
	var hidden roleQualificationHiddenGradersFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &hidden); err != nil {
		return nil, RoleQualificationGradeV1{}, err
	}
	for _, grader := range hidden.Graders {
		if grader.CaseID == caseID {
			return canonical, GradeRoleQualificationAnswerV1(strings.Join(parts, "\n"), grader), nil
		}
	}
	return nil, RoleQualificationGradeV1{}, fmt.Errorf("hidden grader missing for %s", caseID)
}

func shortRoleQualificationClassV2(id string) string {
	switch id {
	case "carryforward.claudesonnet5.medium.openrouter.anchor.v1":
		return "sonnet5"
	case "carryforward.claude46.medium.subscription.phased.v1":
		return "claude46"
	case "carryforward.nexn25pro.free.openrouter.floor.v1":
		return "nex"
	default:
		return "unknown"
	}
}
func roleQualificationCanonicalHashV2(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func sortedRoleQualificationKeysV2(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
