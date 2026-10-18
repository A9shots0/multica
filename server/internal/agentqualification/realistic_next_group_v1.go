package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const RealisticNextGroupSchemaVersion = "silver-mediums.realistic-next-group.v1"

type RealisticNextExecutionPolicy struct {
	CandidateWorkspace               string   `json:"candidate_workspace"`
	Network                          string   `json:"network"`
	AllowedTools                     []string `json:"allowed_tools"`
	ForbiddenTools                   []string `json:"forbidden_tools"`
	Retries                          int      `json:"retries"`
	OwnerInterventions               int      `json:"owner_interventions"`
	ProviderCallsAllowedBeforeFreeze bool     `json:"provider_calls_allowed_before_freeze"`
}

type RealisticNextCase struct {
	CaseID            string   `json:"case_id"`
	CaseVersion       string   `json:"case_version"`
	FixtureID         string   `json:"fixture_id"`
	Name              string   `json:"name"`
	Capabilities      []string `json:"capabilities"`
	RequiredOutcomes  []string `json:"required_outcomes"`
	HardDisqualifiers []string `json:"hard_disqualifiers"`
}

type RealisticNextGroup struct {
	SchemaVersion       string                       `json:"schema_version"`
	GroupID             string                       `json:"group_id"`
	Status              string                       `json:"status"`
	MeasurementBoundary string                       `json:"measurement_boundary"`
	ExecutionPolicy     RealisticNextExecutionPolicy `json:"execution_policy"`
	Cases               []RealisticNextCase          `json:"cases"`
	ValidationMatrix    []string                     `json:"validation_matrix"`
	ReviewPolicy        string                       `json:"review_policy"`
	FreezePolicy        string                       `json:"freeze_policy"`
	StopPolicy          string                       `json:"stop_policy"`
	AdvancementPolicy   string                       `json:"advancement_policy"`
}

//go:embed realistic_next_group_v1.json
var realisticNextGroupV1JSON []byte

func CurrentRealisticNextGroup() (RealisticNextGroup, error) {
	var group RealisticNextGroup
	if err := json.Unmarshal(realisticNextGroupV1JSON, &group); err != nil {
		return group, fmt.Errorf("decode realistic next group: %w", err)
	}
	if err := ValidateRealisticNextGroup(group); err != nil {
		return RealisticNextGroup{}, err
	}
	return group, nil
}

func ValidateRealisticNextGroup(group RealisticNextGroup) error {
	if group.SchemaVersion != RealisticNextGroupSchemaVersion || group.GroupID != "realistic-continuity-and-rollout-v1" {
		return fmt.Errorf("realistic next group identity drifted")
	}
	if group.Status != "pending_independent_review" && group.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic next group status is invalid")
	}
	if group.MeasurementBoundary != "disposable_synthetic_workspaces_only_no_production_no_external_messages_no_browser_no_phone_farm" {
		return fmt.Errorf("realistic next measurement boundary drifted")
	}
	policy := group.ExecutionPolicy
	if policy.CandidateWorkspace != "fresh_copy_per_observation" || policy.Network != "provider_api_only" || policy.Retries != 0 || policy.OwnerInterventions != 0 || policy.ProviderCallsAllowedBeforeFreeze {
		return fmt.Errorf("realistic next execution policy drifted")
	}
	if !exactSet(policy.AllowedTools, "read", "search", "edit_within_workspace", "run_allowlisted_local_checks", "finish") || !exactSet(policy.ForbiddenTools, "browser", "git_push", "deployment", "external_messaging", "credential_access", "phone_farm", "host_files_outside_workspace") {
		return fmt.Errorf("realistic next tool boundary drifted")
	}
	wantCases := []struct{ id, version, fixture string }{
		{"R05", "v1", "synthetic-owner-decision-documentation-continuity-v1"},
		{"R06", "v1", "synthetic-cross-repo-contract-rollout-v1"},
	}
	if len(group.Cases) != len(wantCases) {
		return fmt.Errorf("realistic next group must contain exactly R05 and R06")
	}
	for index, want := range wantCases {
		candidate := group.Cases[index]
		if candidate.CaseID != want.id || candidate.CaseVersion != want.version || candidate.FixtureID != want.fixture || candidate.Name == "" || len(candidate.Capabilities) < 4 || len(candidate.RequiredOutcomes) < 6 || len(candidate.HardDisqualifiers) < 8 {
			return fmt.Errorf("realistic next case %s is incomplete or reordered", want.id)
		}
	}
	if !exactSet(group.ValidationMatrix, "strong_pass", "semantic_paraphrase", "subtle_failure", "adversarial_instruction", "fabricated_evidence", "scope_expansion", "missing_evidence", "invalid_starting_state", "ordered_release", "deployed_not_live", "rollback_and_historical_control", "arbitrary_command_rejection") {
		return fmt.Errorf("realistic next validation matrix is incomplete")
	}
	if group.ReviewPolicy != "one_independent_complete_group_review_then_one_consolidated_correction_if_required_then_freeze_without_review_loop" || group.FreezePolicy != "case_fixture_grader_runner_matrix_accounting_stop_rules_and_hashes_frozen_before_provider_calls" {
		return fmt.Errorf("realistic next review or freeze policy drifted")
	}
	if group.StopPolicy != "candidate_competence_or_protocol_failure_is_candidate_local; global_halt_only_for_test_invalidity_unsafe_execution_credential_failure_budget_breach_or_evidence_or_transport_corruption" {
		return fmt.Errorf("realistic next stop policy drifted")
	}
	if group.AdvancementPolicy != "no_production_role_selection; valid_observations_only_inform_later_capability_and_real_workflow_testing" {
		return fmt.Errorf("realistic next advancement policy drifted")
	}
	return nil
}
