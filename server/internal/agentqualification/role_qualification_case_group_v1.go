package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const RoleQualificationCaseGroupSchemaVersion = "silver-mediums.role-qualification-case-group.v1"

type RoleQualificationCandidateClass struct {
	ID                    string `json:"id"`
	InstructionArtifactID string `json:"instruction_artifact_id"`
}

type RoleQualificationSourceArtifact struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Purpose string `json:"purpose"`
}

type RoleQualificationCase struct {
	CaseID                     string   `json:"case_id"`
	Title                      string   `json:"title"`
	Purpose                    string   `json:"purpose"`
	Scenario                   string   `json:"scenario"`
	PassRequirements           []string `json:"pass_requirements"`
	SubtleFailures             []string `json:"subtle_failures"`
	AdversarialFailures        []string `json:"adversarial_failures"`
	RequiredArtifactIDs        []string `json:"required_artifact_ids"`
	ApplicableCandidateClasses []string `json:"applicable_candidate_classes"`
	CoverageTags               []string `json:"coverage_tags"`
}

type RoleQualificationCaseGroup struct {
	SchemaVersion       string                            `json:"schema_version"`
	GroupID             string                            `json:"group_id"`
	Status              string                            `json:"status"`
	MeasurementBoundary string                            `json:"measurement_boundary"`
	ProviderPolicy      string                            `json:"provider_policy"`
	RuntimeWiring       string                            `json:"runtime_wiring"`
	Models              []string                          `json:"models"`
	CandidateClasses    []RoleQualificationCandidateClass `json:"candidate_classes"`
	SourceArtifacts     []RoleQualificationSourceArtifact `json:"source_artifacts"`
	RequiredCoverage    []string                          `json:"required_coverage"`
	Cases               []RoleQualificationCase           `json:"cases"`
	PrevalidationPolicy string                            `json:"prevalidation_policy"`
	FreezePolicy        string                            `json:"freeze_policy"`
	AdvancementPolicy   string                            `json:"advancement_policy"`
}

//go:embed role_qualification_case_group_v1.json
var roleQualificationCaseGroupV1JSON []byte

func CurrentRoleQualificationCaseGroupV1() (RoleQualificationCaseGroup, error) {
	var group RoleQualificationCaseGroup
	decoder := json.NewDecoder(bytes.NewReader(roleQualificationCaseGroupV1JSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&group); err != nil {
		return RoleQualificationCaseGroup{}, fmt.Errorf("decode role qualification case group v1: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return RoleQualificationCaseGroup{}, fmt.Errorf("decode role qualification case group v1: trailing JSON")
	}
	if err := ValidateRoleQualificationCaseGroupV1(group); err != nil {
		return RoleQualificationCaseGroup{}, err
	}
	return group, nil
}

func ValidateRoleQualificationCaseGroupV1(group RoleQualificationCaseGroup) error {
	if group.SchemaVersion != RoleQualificationCaseGroupSchemaVersion || group.GroupID != "role-qualification-core-v1" {
		return fmt.Errorf("role qualification case group identity drifted")
	}
	if group.Status != "draft_prevalidation" {
		return fmt.Errorf("role qualification case group must remain draft_prevalidation")
	}
	if group.MeasurementBoundary != "static_provider_free_role_behavior_cases_only_no_execution_no_role_or_model_selection" {
		return fmt.Errorf("role qualification measurement boundary drifted")
	}
	if group.ProviderPolicy != "forbidden_draft_prevalidation" || group.RuntimeWiring != "none" || len(group.Models) != 0 {
		return fmt.Errorf("role qualification provider, runtime, or model boundary drifted")
	}

	expectedClasses := []RoleQualificationCandidateClass{
		{ID: "mika_owner_interface", InstructionArtifactID: "mika_owner_interface_instructions_v1"},
		{ID: "quality_checker", InstructionArtifactID: "quality_checker_instructions_v1"},
		{ID: "product_ui", InstructionArtifactID: "product_ui_instructions_v1"},
		{ID: "shared_windows_executor", InstructionArtifactID: "shared_windows_executor_instructions_v1"},
	}
	if len(group.CandidateClasses) != len(expectedClasses) {
		return fmt.Errorf("role qualification group must contain exactly four candidate classes")
	}
	classInstructions := make(map[string]string, len(expectedClasses))
	for index, expected := range expectedClasses {
		actual := group.CandidateClasses[index]
		if actual != expected {
			return fmt.Errorf("role qualification candidate classes changed or reordered")
		}
		classInstructions[actual.ID] = actual.InstructionArtifactID
	}

	expectedArtifacts := []RoleQualificationSourceArtifact{
		{ID: "hosted_workflow_interaction_audit_20260910", Path: "docs/silver-mediums/HOSTED-WORKFLOW-INTERACTION-AUDIT-20260910.md"},
		{ID: "item_09_to_12_acceptance_matrix", Path: "docs/silver-mediums/ACCEPTANCE.md"},
		{ID: "item_10_inbox_chat_acceptance", Path: "docs/silver-mediums/ITEM-10-INBOX-CHAT-ACCEPTANCE.md"},
		{ID: "item_11_mobile_parity", Path: "docs/silver-mediums/ITEM-11-MOBILE-PARITY.md"},
		{ID: "item_12_sms_notifications", Path: "docs/silver-mediums/ITEM-12-SMS-NOTIFICATIONS.md"},
		{ID: "topology_scenario_bank_v2", Path: "docs/silver-mediums/ITEM-14-TOPOLOGY-SCENARIO-BANK.md"},
		{ID: "role_design_decision_register_v1", Path: "docs/silver-mediums/ITEM-14-ROLE-DESIGN-DECISION-REGISTER.md"},
		{ID: "mika_owner_interface_instructions_v1", Path: "server/internal/agentqualification/role_artifacts/v1/mika_owner_interface.instructions.md"},
		{ID: "quality_checker_instructions_v1", Path: "server/internal/agentqualification/role_artifacts/v1/quality_checker.instructions.md"},
		{ID: "product_ui_instructions_v1", Path: "server/internal/agentqualification/role_artifacts/v1/product_ui.instructions.md"},
		{ID: "shared_windows_executor_instructions_v1", Path: "server/internal/agentqualification/role_artifacts/v1/shared_windows_executor.instructions.md"},
	}
	if len(group.SourceArtifacts) != len(expectedArtifacts) {
		return fmt.Errorf("role qualification source artifact coverage changed")
	}
	artifactIDs := make(map[string]struct{}, len(expectedArtifacts))
	for index, expected := range expectedArtifacts {
		actual := group.SourceArtifacts[index]
		if actual.ID != expected.ID || actual.Path != expected.Path || strings.TrimSpace(actual.Purpose) == "" {
			return fmt.Errorf("role qualification source artifact %d drifted or is incomplete", index)
		}
		if _, duplicate := artifactIDs[actual.ID]; duplicate {
			return fmt.Errorf("duplicate role qualification source artifact %s", actual.ID)
		}
		artifactIDs[actual.ID] = struct{}{}
	}

	expectedCoverage := []string{
		"direct_answer_anti_echo",
		"chat_vs_inbox_routing",
		"concern_as_hypothesis",
		"genuine_needs_you_vs_automatic_continuation",
		"false_completion_detection",
		"deployed_not_live_detection",
		"regression_detection",
		"ui_function_vs_owner_taste",
		"bounded_root_cause_reasoning",
		"blast_radius_reasoning",
		"evidence_lane_separation",
		"dirty_work_preservation",
		"phone_farm_prohibition",
	}
	if !exactSet(group.RequiredCoverage, expectedCoverage...) {
		return fmt.Errorf("role qualification required coverage drifted")
	}
	coverageAllowed := make(map[string]struct{}, len(expectedCoverage))
	for _, tag := range expectedCoverage {
		coverageAllowed[tag] = struct{}{}
	}

	expectedCases := []struct {
		id      string
		classID string
	}{
		{id: "RQ-MIKA-01", classID: "mika_owner_interface"},
		{id: "RQ-MIKA-02", classID: "mika_owner_interface"},
		{id: "RQ-MIKA-03", classID: "mika_owner_interface"},
		{id: "RQ-QC-01", classID: "quality_checker"},
		{id: "RQ-QC-02", classID: "quality_checker"},
		{id: "RQ-QC-03", classID: "quality_checker"},
		{id: "RQ-PUI-01", classID: "product_ui"},
		{id: "RQ-PUI-02", classID: "product_ui"},
		{id: "RQ-PUI-03", classID: "product_ui"},
		{id: "RQ-WIN-01", classID: "shared_windows_executor"},
		{id: "RQ-WIN-02", classID: "shared_windows_executor"},
		{id: "RQ-WIN-03", classID: "shared_windows_executor"},
	}
	if len(group.Cases) != len(expectedCases) {
		return fmt.Errorf("role qualification group must contain exactly twelve cases")
	}
	seenCases := make(map[string]struct{}, len(expectedCases))
	classCounts := make(map[string]int, len(expectedClasses))
	covered := make(map[string]struct{}, len(expectedCoverage))
	for index, expected := range expectedCases {
		candidate := group.Cases[index]
		if candidate.CaseID != expected.id {
			return fmt.Errorf("role qualification cases changed or reordered at %d", index)
		}
		if _, duplicate := seenCases[candidate.CaseID]; duplicate {
			return fmt.Errorf("duplicate role qualification case %s", candidate.CaseID)
		}
		seenCases[candidate.CaseID] = struct{}{}
		if strings.TrimSpace(candidate.Title) == "" || strings.TrimSpace(candidate.Purpose) == "" || strings.TrimSpace(candidate.Scenario) == "" {
			return fmt.Errorf("role qualification case %s has incomplete identity or scenario", candidate.CaseID)
		}
		if err := validateRoleQualificationStringList(candidate.CaseID, "pass requirements", candidate.PassRequirements, 3); err != nil {
			return err
		}
		if err := validateRoleQualificationStringList(candidate.CaseID, "subtle failures", candidate.SubtleFailures, 3); err != nil {
			return err
		}
		if err := validateRoleQualificationStringList(candidate.CaseID, "adversarial failures", candidate.AdversarialFailures, 3); err != nil {
			return err
		}
		if len(candidate.ApplicableCandidateClasses) != 1 || candidate.ApplicableCandidateClasses[0] != expected.classID {
			return fmt.Errorf("role qualification case %s candidate applicability drifted", candidate.CaseID)
		}
		classCounts[expected.classID]++
		if err := validateRoleQualificationStringList(candidate.CaseID, "required artifact ids", candidate.RequiredArtifactIDs, 3); err != nil {
			return err
		}
		requiredArtifacts := make(map[string]struct{}, len(candidate.RequiredArtifactIDs))
		for _, artifactID := range candidate.RequiredArtifactIDs {
			if _, ok := artifactIDs[artifactID]; !ok {
				return fmt.Errorf("role qualification case %s references unknown artifact %s", candidate.CaseID, artifactID)
			}
			requiredArtifacts[artifactID] = struct{}{}
		}
		if _, ok := requiredArtifacts["role_design_decision_register_v1"]; !ok {
			return fmt.Errorf("role qualification case %s omits the role decision register", candidate.CaseID)
		}
		if _, ok := requiredArtifacts[classInstructions[expected.classID]]; !ok {
			return fmt.Errorf("role qualification case %s omits its candidate instruction artifact", candidate.CaseID)
		}
		if err := validateRoleQualificationStringList(candidate.CaseID, "coverage tags", candidate.CoverageTags, 1); err != nil {
			return err
		}
		for _, tag := range candidate.CoverageTags {
			if _, ok := coverageAllowed[tag]; !ok {
				return fmt.Errorf("role qualification case %s has unknown coverage tag %s", candidate.CaseID, tag)
			}
			covered[tag] = struct{}{}
		}
	}
	for _, class := range expectedClasses {
		if classCounts[class.ID] != 3 {
			return fmt.Errorf("candidate class %s must have exactly three cases", class.ID)
		}
	}
	if len(covered) != len(expectedCoverage) {
		return fmt.Errorf("role qualification cases do not cover every required behavior")
	}
	for _, tag := range expectedCoverage {
		if _, ok := covered[tag]; !ok {
			return fmt.Errorf("role qualification cases omit coverage %s", tag)
		}
	}

	if group.PrevalidationPolicy != "validate_strong_pass_subtle_failure_paraphrase_and_adversarial_examples_for_every_case_then_one_independent_complete_group_review_before_freeze" {
		return fmt.Errorf("role qualification prevalidation policy drifted")
	}
	if group.FreezePolicy != "forbidden_while_status_is_draft_prevalidation" {
		return fmt.Errorf("role qualification freeze policy drifted")
	}
	if group.AdvancementPolicy != "evidence_only_no_role_model_topology_permission_or_production_selection" {
		return fmt.Errorf("role qualification advancement policy drifted")
	}
	return nil
}

func validateRoleQualificationStringList(caseID, field string, values []string, minimum int) error {
	if len(values) < minimum {
		return fmt.Errorf("role qualification case %s has incomplete %s", caseID, field)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("role qualification case %s has blank %s", caseID, field)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("role qualification case %s has duplicate %s", caseID, field)
		}
		seen[value] = struct{}{}
	}
	return nil
}
