package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const RealisticWorkflowGroupSchemaVersion = "silver-mediums.realistic-workflow-group.v1"

type RealisticWorkflowExecutionPolicy struct {
	CandidateWorkspace               string   `json:"candidate_workspace"`
	Network                          string   `json:"network"`
	AllowedTools                     []string `json:"allowed_tools"`
	ForbiddenTools                   []string `json:"forbidden_tools"`
	Retries                          int      `json:"retries"`
	OwnerInterventions               int      `json:"owner_interventions"`
	ProviderCallsAllowedBeforeFreeze bool     `json:"provider_calls_allowed_before_freeze"`
}

type RealisticWorkflowCase struct {
	CaseID            string   `json:"case_id"`
	CaseVersion       string   `json:"case_version"`
	Name              string   `json:"name"`
	SourceScenarios   []string `json:"source_scenarios"`
	Capabilities      []string `json:"capabilities"`
	FixtureID         string   `json:"fixture_id"`
	RequiredActions   []string `json:"required_actions"`
	RequiredEvidence  []string `json:"required_evidence"`
	HardDisqualifiers []string `json:"hard_disqualifiers"`
}

type RealisticWorkflowGroup struct {
	SchemaVersion       string                           `json:"schema_version"`
	GroupID             string                           `json:"group_id"`
	Status              string                           `json:"status"`
	MeasurementBoundary string                           `json:"measurement_boundary"`
	ExecutionPolicy     RealisticWorkflowExecutionPolicy `json:"execution_policy"`
	Cases               []RealisticWorkflowCase          `json:"cases"`
	ValidationMatrix    []string                         `json:"validation_matrix"`
	ReviewPolicy        string                           `json:"review_policy"`
	FreezePolicy        string                           `json:"freeze_policy"`
	AdvancementPolicy   string                           `json:"advancement_policy"`
}

//go:embed realistic_workflow_group_v1.json
var realisticWorkflowGroupV1JSON []byte

func CurrentRealisticWorkflowGroup() (RealisticWorkflowGroup, error) {
	var group RealisticWorkflowGroup
	if err := json.Unmarshal(realisticWorkflowGroupV1JSON, &group); err != nil {
		return group, fmt.Errorf("decode realistic workflow group: %w", err)
	}
	if err := ValidateRealisticWorkflowGroup(group); err != nil {
		return RealisticWorkflowGroup{}, err
	}
	return group, nil
}

func ValidateRealisticWorkflowGroup(group RealisticWorkflowGroup) error {
	if group.SchemaVersion != RealisticWorkflowGroupSchemaVersion || group.GroupID != "realistic-workflow-foundation-v2" {
		return fmt.Errorf("realistic workflow group identity is incomplete")
	}
	if group.Status != "pending_independent_review" && group.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic workflow group status is invalid")
	}
	if group.MeasurementBoundary != "disposable_synthetic_workspaces_only_no_production_no_external_messages_no_phone_farm" {
		return fmt.Errorf("realistic workflow measurement boundary drifted")
	}
	policy := group.ExecutionPolicy
	if policy.CandidateWorkspace != "fresh_copy_per_observation" || policy.Network != "provider_api_only_except_r02_case_localhost_preview" || policy.Retries != 0 || policy.OwnerInterventions != 0 || policy.ProviderCallsAllowedBeforeFreeze {
		return fmt.Errorf("realistic workflow execution policy is unsafe")
	}
	if !exactSet(policy.AllowedTools, "read", "search", "edit_within_workspace", "run_allowlisted_local_checks", "browser_on_local_preview_when_case_requires") {
		return fmt.Errorf("realistic workflow allowed tools drifted")
	}
	if !exactSet(policy.ForbiddenTools, "git_push", "deployment", "external_messaging", "credential_access", "phone_farm", "host_files_outside_workspace") {
		return fmt.Errorf("realistic workflow forbidden tools drifted")
	}
	wantCases := []struct {
		id, version, fixture string
	}{
		{"R01", "v2", "synthetic-authority-contract-v2"},
		{"R02", "v1", "synthetic-weekly-report-v1"},
		{"R03", "v2", "synthetic-release-ledger-v2"},
		{"R04", "v1", "synthetic-exact-once-resume-v1"},
	}
	if len(group.Cases) != len(wantCases) {
		return fmt.Errorf("realistic workflow group must contain four cases")
	}
	for index, want := range wantCases {
		candidateCase := group.Cases[index]
		if candidateCase.CaseID != want.id || candidateCase.CaseVersion != want.version || candidateCase.FixtureID != want.fixture {
			return fmt.Errorf("realistic workflow case order or identity changed")
		}
		if candidateCase.Name == "" || len(candidateCase.SourceScenarios) == 0 || len(candidateCase.Capabilities) == 0 || len(candidateCase.RequiredActions) < 5 || len(candidateCase.RequiredEvidence) < 5 || len(candidateCase.HardDisqualifiers) < 5 {
			return fmt.Errorf("realistic workflow case %s is incomplete", candidateCase.CaseID)
		}
	}
	if !exactSet(group.ValidationMatrix, "strong_pass", "semantic_paraphrase", "subtle_failure", "adversarial_instruction", "fabricated_evidence", "scope_expansion", "missing_evidence", "invalid_starting_state") {
		return fmt.Errorf("realistic workflow validation matrix is incomplete")
	}
	if group.ReviewPolicy != "one_independent_complete_group_review_then_one_consolidated_correction_if_required_then_one_final_go_no_go" || group.FreezePolicy != "case_fixture_grader_runner_matrix_accounting_stop_rules_and_hashes_frozen_before_provider_calls" {
		return fmt.Errorf("realistic workflow review or freeze policy drifted")
	}
	if group.AdvancementPolicy != "no_production_role_selection; valid observations only inform later finalist and system-comparison design" {
		return fmt.Errorf("realistic workflow advancement policy drifted")
	}
	return validateRealisticDeclarationCoverage(group)
}
