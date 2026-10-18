package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const CapabilityCaseGroupSchemaVersion = "silver-mediums.capability-case-group.v1"

type CapabilityCaseGroup struct {
	SchemaVersion       string          `json:"schema_version"`
	GroupID             string          `json:"group_id"`
	Status              string          `json:"status"`
	MeasurementBoundary string          `json:"measurement_boundary"`
	Cases               []ScreeningCase `json:"cases"`
	ValidationFixtures  []string        `json:"validation_fixtures"`
	ReviewPolicy        string          `json:"review_policy"`
	FreezePolicy        string          `json:"freeze_policy"`
}

//go:embed capability_case_group_v1.json
var capabilityCaseGroupV1JSON []byte

func CurrentCapabilityCaseGroup() (CapabilityCaseGroup, error) {
	var group CapabilityCaseGroup
	if err := json.Unmarshal(capabilityCaseGroupV1JSON, &group); err != nil {
		return group, fmt.Errorf("decode capability case group: %w", err)
	}
	if err := ValidateCapabilityCaseGroup(group); err != nil {
		return CapabilityCaseGroup{}, err
	}
	return group, nil
}

func ValidateCapabilityCaseGroup(group CapabilityCaseGroup) error {
	if group.SchemaVersion != CapabilityCaseGroupSchemaVersion || group.GroupID != "static-capability-breadth-v1" {
		return fmt.Errorf("capability group identity is incomplete")
	}
	if group.Status != "pending_independent_review" && group.Status != "frozen_reviewed" {
		return fmt.Errorf("capability group status is invalid")
	}
	if group.MeasurementBoundary != "static_tool_free_reasoning_only_not_execution_or_role_qualification" {
		return fmt.Errorf("capability group measurement boundary drifted")
	}
	want := []string{"T11:v4", "T08:v3", "T16:v3", "T04:v1", "T09:v1"}
	if len(group.Cases) != len(want) {
		return fmt.Errorf("capability group must contain five cases")
	}
	for index, screeningCase := range group.Cases {
		if screeningCase.CaseID+":"+screeningCase.CaseVersion != want[index] {
			return fmt.Errorf("capability group case order changed")
		}
		candidateHash, err := EmbeddedScreeningPacketVersionSHA256(screeningCase.CaseID, screeningCase.CaseVersion, "candidate")
		if err != nil || candidateHash != screeningCase.CandidatePacketSHA256 {
			return fmt.Errorf("case %s candidate hash mismatch", screeningCase.CaseID)
		}
		graderHash, err := EmbeddedScreeningPacketVersionSHA256(screeningCase.CaseID, screeningCase.CaseVersion, "grader")
		if err != nil || graderHash != screeningCase.GraderPacketSHA256 {
			return fmt.Errorf("case %s grader hash mismatch", screeningCase.CaseID)
		}
		if err := validatePublicResponseContract(screeningCase.CaseID, screeningCase.CaseVersion); err != nil {
			return err
		}
	}
	wantFixtures := []string{"strong_pass", "semantic_paraphrase", "subtle_failure", "inverse_or_contradiction", "adversarial_answer", "missing_field", "extra_field", "wrong_type"}
	if !exactSet(group.ValidationFixtures, wantFixtures...) {
		return fmt.Errorf("capability group validation fixtures are incomplete")
	}
	if group.ReviewPolicy != "one_independent_complete_group_review_then_one_consolidated_correction_if_required" || group.FreezePolicy != "packet_grader_matrix_accounting_and_stop_behavior_hash_pinned_before_provider_calls" {
		return fmt.Errorf("capability group review or freeze policy drifted")
	}
	return nil
}
