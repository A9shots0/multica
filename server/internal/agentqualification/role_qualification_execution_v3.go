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

const RoleQualificationExecutionSchemaV3 = "silver-mediums.role-qualification-execution.v3"

type RoleQualificationCampaignCapsV3 struct {
	MaxCalls                      int   `json:"max_calls"`
	MaxInputTokens                int64 `json:"max_input_tokens"`
	MaxOutputTokens               int64 `json:"max_output_tokens"`
	MaxTotalTokens                int64 `json:"max_total_tokens"`
	MaxWallSeconds                int64 `json:"max_wall_seconds"`
	MaxOpenRouterCashMicros       int64 `json:"max_openrouter_cash_micros"`
	MaxClaudeCostEquivalentMicros int64 `json:"max_claude_cost_equivalent_micros"`
}

type RoleQualificationExecutionDefinitionV3 struct {
	SchemaVersion              string                             `json:"schema_version"`
	ExecutionID                string                             `json:"execution_id"`
	Status                     string                             `json:"status"`
	V1FreezeSHA256             string                             `json:"v1_freeze_sha256"`
	RejectedV2FreezeSHA256     string                             `json:"rejected_v2_freeze_sha256"`
	ProviderCallsAllowed       bool                               `json:"provider_calls_allowed"`
	RuntimeWiring              string                             `json:"runtime_wiring"`
	FreshPreflightRequired     bool                               `json:"fresh_preflight_required"`
	Retries                    int                                `json:"retries"`
	Fallbacks                  bool                               `json:"fallbacks"`
	PerObservation             RoleQualificationObservationCapsV2 `json:"per_observation"`
	Campaign                   RoleQualificationCampaignCapsV3    `json:"campaign"`
	ProviderGate               string                             `json:"provider_gate"`
	RoleModelSelectionAllowed  bool                               `json:"role_model_selection_allowed"`
	ProductionAllowed          bool                               `json:"production_allowed"`
	ExternalSideEffectsAllowed bool                               `json:"external_side_effects_allowed"`
}

type RoleQualificationFieldSchemaV3 struct {
	Type  string                          `json:"type"`
	Items *RoleQualificationFieldSchemaV3 `json:"items,omitempty"`
}
type RoleQualificationTypedSchemaV3 struct {
	Type                 string                                    `json:"type"`
	Properties           map[string]RoleQualificationFieldSchemaV3 `json:"properties"`
	Required             []string                                  `json:"required"`
	AdditionalProperties bool                                      `json:"additionalProperties"`
}
type RoleQualificationExecutionPacketV3 struct {
	CaseID              string                         `json:"case_id"`
	CandidateClass      string                         `json:"candidate_class"`
	RoleInstructions    string                         `json:"role_instructions"`
	RoleSkill           string                         `json:"role_skill"`
	Scenario            string                         `json:"scenario"`
	RequiredArtifactIDs []string                       `json:"required_artifact_ids"`
	ResponseSchema      RoleQualificationTypedSchemaV3 `json:"response_schema"`
}
type RoleQualificationFrozenRequestV3 struct {
	SchemaVersion      string                                `json:"schema_version"`
	CellID             string                                `json:"cell_id"`
	ConfigurationClass roleQualificationConfigurationClassV1 `json:"configuration_class"`
	CandidatePacket    RoleQualificationExecutionPacketV3    `json:"candidate_packet"`
	MaxOutputTokens    int64                                 `json:"max_output_tokens"`
	Retries            int                                   `json:"retries"`
	Fallbacks          bool                                  `json:"fallbacks"`
}
type RoleQualificationExecutionCellV3 struct {
	Ordinal            int                                   `json:"ordinal"`
	CellID             string                                `json:"cell_id"`
	ConfigurationClass roleQualificationConfigurationClassV1 `json:"configuration_class"`
	CandidateClass     string                                `json:"candidate_class"`
	CaseID             string                                `json:"case_id"`
	Request            RoleQualificationFrozenRequestV3      `json:"frozen_request"`
	RequestSHA256      string                                `json:"frozen_request_sha256"`
}
type RoleQualificationExecutionPlanV3 struct {
	Status                 string                             `json:"status"`
	ProviderCallsAllowed   bool                               `json:"provider_calls_allowed"`
	RuntimeWiring          string                             `json:"runtime_wiring"`
	V1FreezeSHA256         string                             `json:"v1_freeze_sha256"`
	RejectedV2FreezeSHA256 string                             `json:"rejected_v2_freeze_sha256"`
	ExecutionFreezeSHA256  string                             `json:"execution_freeze_sha256"`
	Cells                  []RoleQualificationExecutionCellV3 `json:"cells"`
	PerObservation         RoleQualificationObservationCapsV2 `json:"per_observation"`
	Campaign               RoleQualificationCampaignCapsV3    `json:"campaign"`
}

//go:embed role_qualification_execution_v3.json
var roleQualificationExecutionV3JSON []byte

func CurrentRoleQualificationExecutionDefinitionV3() (RoleQualificationExecutionDefinitionV3, error) {
	var d RoleQualificationExecutionDefinitionV3
	if err := strictJSON(roleQualificationExecutionV3JSON, &d); err != nil {
		return d, err
	}
	if err := ValidateRoleQualificationExecutionDefinitionV3(d); err != nil {
		return RoleQualificationExecutionDefinitionV3{}, err
	}
	return d, nil
}
func ValidateRoleQualificationExecutionDefinitionV3(d RoleQualificationExecutionDefinitionV3) error {
	if d.SchemaVersion != RoleQualificationExecutionSchemaV3 || d.ExecutionID != "role-qualification-comparison-v3" || d.Status != "corrected_frozen_provider_locked" || d.V1FreezeSHA256 != "b047faaec585beee2cf00640f53e35d2b09b54618c6923c6b9458d2851d1b2e4" || d.V1FreezeSHA256 != RoleQualificationFreezeV1SHA256() || d.RejectedV2FreezeSHA256 != "39f31fd1725d38cf00640f53e35d2b09b54618c6923c6b9458d2851d1b2e4" {
		return fmt.Errorf("execution v3 identity drift")
	}
	if d.ProviderCallsAllowed || d.RuntimeWiring != "none" || !d.FreshPreflightRequired || d.Retries != 0 || d.Fallbacks || d.RoleModelSelectionAllowed || d.ProductionAllowed || d.ExternalSideEffectsAllowed {
		return fmt.Errorf("execution v3 authority opened")
	}
	if d.PerObservation != (RoleQualificationObservationCapsV2{1, 20000, 4000, 24000, 180}) || d.Campaign != (RoleQualificationCampaignCapsV3{36, 720000, 144000, 864000, 5400, 10000000, 10000000}) || d.ProviderGate != "disabled_until_exact_v3_freeze_and_fresh_transport_preflight_validate" {
		return fmt.Errorf("execution v3 caps or gate drift")
	}
	return nil
}

func BuildRoleQualificationExecutionPlanV3() (RoleQualificationExecutionPlanV3, error) {
	d, err := CurrentRoleQualificationExecutionDefinitionV3()
	if err != nil {
		return RoleQualificationExecutionPlanV3{}, err
	}
	if RoleQualificationExecutionFreezeV3SHA256() == "" {
		return RoleQualificationExecutionPlanV3{}, fmt.Errorf("execution v3 freeze missing")
	}
	if _, err := CurrentRoleQualificationFreezeV1(); err != nil {
		return RoleQualificationExecutionPlanV3{}, err
	}
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		return RoleQualificationExecutionPlanV3{}, err
	}
	var matrix roleQualificationRunMatrixFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &matrix); err != nil {
		return RoleQualificationExecutionPlanV3{}, err
	}
	if err := validateRunMatrixV1(group, matrix); err != nil {
		return RoleQualificationExecutionPlanV3{}, err
	}
	visible, err := CurrentRoleQualificationCandidatePacketsV1()
	if err != nil {
		return RoleQualificationExecutionPlanV3{}, err
	}
	visibleBy := map[string]RoleQualificationCandidatePacketV1{}
	for _, p := range visible {
		visibleBy[p.CaseID] = p
	}
	caseBy := map[string]RoleQualificationCase{}
	for _, c := range group.Cases {
		caseBy[c.CaseID] = c
	}
	cells := make([]RoleQualificationExecutionCellV3, 0, 36)
	for ci, config := range matrix.ConfigurationClasses {
		for ri := 0; ri < len(matrix.RoleBindings); ri++ {
			binding := matrix.RoleBindings[(ri+ci)%4]
			for ki := 0; ki < 3; ki++ {
				caseID := binding.CaseIDs[(ki+ci+ri)%3]
				packet, err := buildRoleQualificationExecutionPacketV3(binding, caseBy[caseID], visibleBy[caseID])
				if err != nil {
					return RoleQualificationExecutionPlanV3{}, err
				}
				ordinal := len(cells) + 1
				cellID := fmt.Sprintf("rqv3-%02d-%s-%s-%s", ordinal, shortRoleQualificationClassV2(config.ClassID), binding.CandidateClass, strings.ToLower(caseID))
				request := RoleQualificationFrozenRequestV3{SchemaVersion: "silver-mediums.role-qualification-request.v3", CellID: cellID, ConfigurationClass: config, CandidatePacket: packet, MaxOutputTokens: 4000, Retries: 0, Fallbacks: false}
				hash, err := roleQualificationCanonicalHashV3(request)
				if err != nil {
					return RoleQualificationExecutionPlanV3{}, err
				}
				cells = append(cells, RoleQualificationExecutionCellV3{ordinal, cellID, config, binding.CandidateClass, caseID, request, hash})
			}
		}
	}
	if len(cells) != 36 {
		return RoleQualificationExecutionPlanV3{}, fmt.Errorf("execution v3 cell count")
	}
	return RoleQualificationExecutionPlanV3{"corrected_frozen_provider_locked_not_executed", false, "none", d.V1FreezeSHA256, d.RejectedV2FreezeSHA256, RoleQualificationExecutionFreezeV3SHA256(), cells, d.PerObservation, d.Campaign}, nil
}

func buildRoleQualificationExecutionPacketV3(binding roleQualificationRoleBindingV1, c RoleQualificationCase, visible RoleQualificationCandidatePacketV1) (RoleQualificationExecutionPacketV3, error) {
	base := strings.TrimSuffix(binding.InstructionArtifactID, "_instructions_v1")
	if base == binding.InstructionArtifactID || strings.TrimSuffix(binding.SkillArtifactID, "_skill_v1") != base {
		return RoleQualificationExecutionPacketV3{}, fmt.Errorf("role artifact binding")
	}
	instructions, err := roleQualificationV1Files.ReadFile("role_artifacts/v1/" + base + ".instructions.md")
	if err != nil {
		return RoleQualificationExecutionPacketV3{}, err
	}
	skill, err := roleQualificationV1Files.ReadFile("role_artifacts/v1/" + base + ".skill.md")
	if err != nil {
		return RoleQualificationExecutionPacketV3{}, err
	}
	schema, err := roleQualificationTypedSchemaV3(binding.CandidateClass, visible.ResponseSchema)
	if err != nil {
		return RoleQualificationExecutionPacketV3{}, err
	}
	return RoleQualificationExecutionPacketV3{c.CaseID, binding.CandidateClass, string(instructions), string(skill), c.Scenario, append([]string(nil), c.RequiredArtifactIDs...), schema}, nil
}
func roleQualificationTypedSchemaV3(class string, fields []string) (RoleQualificationTypedSchemaV3, error) {
	types := map[string]string{}
	switch class {
	case "mika_owner_interface":
		types = map[string]string{"answer": "string", "truth": "string", "action": "string", "owner_decision_needed": "boolean"}
	case "quality_checker":
		types = map[string]string{"verdict": "string", "failed_criteria": "array", "evidence": "array", "required_correction": "string", "completion_boundary": "string", "independence_check": "string"}
	case "product_ui":
		types = map[string]string{"user_problem": "string", "information_priority": "array", "minimum_state": "string", "typical_state": "string", "maximum_state": "string", "narrow_view": "string", "medium_view": "string", "wide_view": "string", "interaction_and_accessibility": "array", "functional_acceptance": "array", "taste_decision_needed": "boolean"}
	case "shared_windows_executor":
		types = map[string]string{"first_failing_boundary": "string", "accountable_actor": "string", "implementation_plan": "array", "blast_radius": "array", "authority_and_host_limits": "array", "focused_checks": "array", "regression_checks": "array", "completion_proof": "array", "prohibited_shortcuts": "array", "owner_decision_needed": "boolean"}
	default:
		return RoleQualificationTypedSchemaV3{}, fmt.Errorf("unknown class")
	}
	if len(types) != len(fields) {
		return RoleQualificationTypedSchemaV3{}, fmt.Errorf("schema field coverage")
	}
	props := map[string]RoleQualificationFieldSchemaV3{}
	for _, f := range fields {
		typ, ok := types[f]
		if !ok {
			return RoleQualificationTypedSchemaV3{}, fmt.Errorf("unknown field %s", f)
		}
		p := RoleQualificationFieldSchemaV3{Type: typ}
		if typ == "array" {
			p.Items = &RoleQualificationFieldSchemaV3{Type: "string"}
		}
		props[f] = p
	}
	return RoleQualificationTypedSchemaV3{"object", props, append([]string(nil), fields...), false}, nil
}

type RoleQualificationTypedOutputV3 map[string]any

func ParseRoleQualificationTypedOutputV3(schema RoleQualificationTypedSchemaV3, raw []byte) (RoleQualificationTypedOutputV3, []byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var values map[string]any
	if err := decoder.Decode(&values); err != nil {
		return nil, nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, nil, fmt.Errorf("trailing JSON")
	}
	if len(values) != len(schema.Required) {
		return nil, nil, fmt.Errorf("field count")
	}
	for _, field := range schema.Required {
		value, ok := values[field]
		if !ok {
			return nil, nil, fmt.Errorf("missing field %s", field)
		}
		spec := schema.Properties[field]
		switch spec.Type {
		case "string":
			s, ok := value.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return nil, nil, fmt.Errorf("field %s type", field)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return nil, nil, fmt.Errorf("field %s type", field)
			}
		case "array":
			items, ok := value.([]any)
			if !ok || len(items) == 0 {
				return nil, nil, fmt.Errorf("field %s type", field)
			}
			for _, item := range items {
				if s, ok := item.(string); !ok || strings.TrimSpace(s) == "" {
					return nil, nil, fmt.Errorf("field %s item type", field)
				}
			}
		default:
			return nil, nil, fmt.Errorf("schema type")
		}
	}
	for field := range values {
		if _, ok := schema.Properties[field]; !ok {
			return nil, nil, fmt.Errorf("unexpected field %s", field)
		}
	}
	canonical, err := json.Marshal(values)
	return values, canonical, err
}

func GradeRoleQualificationTypedOutputV3(caseID string, values RoleQualificationTypedOutputV3) (RoleQualificationGradeV1, error) {
	var hidden roleQualificationHiddenGradersFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &hidden); err != nil {
		return RoleQualificationGradeV1{}, err
	}
	var grader *RoleQualificationHiddenGraderV1
	for i := range hidden.Graders {
		if hidden.Graders[i].CaseID == caseID {
			grader = &hidden.Graders[i]
			break
		}
	}
	if grader == nil {
		return RoleQualificationGradeV1{}, fmt.Errorf("grader missing")
	}
	grade := RoleQualificationGradeV1{Pass: true}
	for _, concept := range grader.RequiredConcepts {
		field := roleQualificationConceptFieldV3(caseID, concept.ID)
		if field == "" || !qualificationConceptMatches(qualificationTokenSet(roleQualificationFieldTextV3(values[field])), concept) {
			grade.MissingConcepts = append(grade.MissingConcepts, concept.ID)
		}
	}
	for _, concept := range grader.ForbiddenConcepts {
		field := roleQualificationConceptFieldV3(caseID, concept.ID)
		if field != "" && qualificationConceptMatches(qualificationTokenSet(roleQualificationFieldTextV3(values[field])), concept) {
			grade.TriggeredFailures = append(grade.TriggeredFailures, concept.ID)
		}
	}
	for field, want := range roleQualificationBooleanExpectationsV3(caseID) {
		actual, ok := values[field].(bool)
		if !ok || actual != want {
			grade.MissingConcepts = append(grade.MissingConcepts, "boolean:"+field)
		}
	}
	grade.Pass = len(grade.MissingConcepts) == 0 && len(grade.TriggeredFailures) == 0
	return grade, nil
}
func roleQualificationFieldTextV3(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}
func roleQualificationConceptFieldV3(caseID, concept string) string {
	fields := map[string]map[string]string{
		"RQ-MIKA-01": {"inbound_unproven": "truth", "new_blocker": "truth", "automatic_next": "action", "evidence_boundary": "truth", "fabricated_success": "truth", "owner_polling": "action", "echo_template": "answer"},
		"RQ-MIKA-02": {"brainstorm_in_chat": "action", "one_issue_binding": "action", "existing_work_in_inbox": "action", "no_chat_workflow": "action", "duplicate_issue": "action", "chat_dispatch": "action", "chat_polling": "action"},
		"RQ-MIKA-03": {"cause_is_hypothesis": "truth", "automatic_correction": "action", "only_brand_needs_owner": "answer", "no_routine_needs_you": "action", "assumed_cause": "truth", "owner_technical_choice": "action", "silent_taste": "answer"},
		"RQ-QC-01":   {"fail_verdict": "verdict", "diagnosis_not_completion": "completion_boundary", "missing_evidence_lanes": "evidence", "bounded_correction": "required_correction", "false_pass": "verdict", "reviewer_executes": "required_correction", "owner_technical_review": "required_correction"},
		"RQ-QC-02":   {"not_live_or_done": "verdict", "rollback_only_baseline": "completion_boundary", "regression_detected": "evidence", "fresh_candidate_proof": "required_correction", "deployment_equals_live": "completion_boundary", "rollback_equals_done": "completion_boundary", "approve_unproven": "verdict"},
		"RQ-QC-03":   {"invalid_evidence_verdict": "verdict", "candidate_mismatch": "evidence", "independence_broken": "independence_check", "fresh_bound_receipts": "required_correction", "mixed_candidate_pass": "verdict", "fresh_context_only": "independence_check", "fabricate_business": "evidence"},
		"RQ-PUI-01":  {"no_taste_decision": "functional_acceptance", "information_priority": "information_priority", "data_states": "functional_acceptance", "responsive_accessible": "interaction_and_accessibility", "routine_owner_choice": "functional_acceptance", "desktop_only": "wide_view", "preview_equals_complete": "functional_acceptance"},
		"RQ-PUI-02":  {"one_brand_question": "user_problem", "option_consequences": "information_priority", "routine_craft_not_escalated": "functional_acceptance", "no_implementation_before_choice": "functional_acceptance", "silent_choice": "user_problem", "many_routine_questions": "user_problem", "self_approval": "functional_acceptance"},
		"RQ-PUI-03":  {"rich_owner_actions": "functional_acceptance", "current_picture_first": "information_priority", "preserve_backend_truth": "functional_acceptance", "qc_separate": "functional_acceptance", "approve_reject_only": "functional_acceptance", "ui_overrides_truth": "functional_acceptance", "self_review": "functional_acceptance"},
		"RQ-WIN-01":  {"hypothesis_not_fact": "first_failing_boundary", "process_chain": "implementation_plan", "bounded_first_divergence": "implementation_plan", "completion_controls": "completion_proof", "disable_all": "prohibited_shortcuts", "delete_logs": "prohibited_shortcuts", "premature_complete": "completion_proof"},
		"RQ-WIN-02":  {"dirty_work_preserved": "authority_and_host_limits", "trace_authority_consumers": "blast_radius", "smallest_root_fix": "implementation_plan", "separate_evidence_lanes": "completion_proof", "destroy_dirty_work": "prohibited_shortcuts", "unsafe_startup": "prohibited_shortcuts", "unauthorized_release": "authority_and_host_limits"},
		"RQ-WIN-03":  {"windows_boundary": "authority_and_host_limits", "sanitized_receipt_only": "completion_proof", "first_divergence": "first_failing_boundary", "route_to_mac_and_verify": "implementation_plan", "windows_ssh": "prohibited_shortcuts", "windows_smc": "prohibited_shortcuts", "rewrite_authority": "implementation_plan"}}
	return fields[caseID][concept]
}
func roleQualificationBooleanExpectationsV3(caseID string) map[string]bool {
	switch caseID {
	case "RQ-MIKA-03":
		return map[string]bool{"owner_decision_needed": true}
	case "RQ-MIKA-01", "RQ-MIKA-02", "RQ-WIN-01", "RQ-WIN-02", "RQ-WIN-03":
		return map[string]bool{"owner_decision_needed": false}
	case "RQ-PUI-02":
		return map[string]bool{"taste_decision_needed": true}
	case "RQ-PUI-01", "RQ-PUI-03":
		return map[string]bool{"taste_decision_needed": false}
	default:
		return nil
	}
}
func roleQualificationCanonicalHashV3(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func sortedRoleQualificationFieldsV3(v map[string]any) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
