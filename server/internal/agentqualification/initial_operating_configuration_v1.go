package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
	"time"
)

// InitialOperatingConfigurationV1Schema names the first configuration that lets
// the accepted publisher -> independent reviewer workflow run on ordinary
// internal issues. It is deliberately narrow: it inherits exactly the shape the
// accepted run demonstrated, keeps every authority that run did not exercise
// withheld, and can be revoked in the controller lifecycle registry.
const InitialOperatingConfigurationV1Schema = "silver-mediums.initial-operating-configuration.v1"

const initialOperatingConfigurationV1ID = "silver-mediums-initial-operating-configuration-20260916"

type InitialOperatingConfigurationSourceHashesV1 struct {
	NativePublicationAcceptance        string `json:"native_publication_acceptance"`
	ProvisionalConfigurationV2         string `json:"provisional_configuration_v2"`
	IntegrationReplayResult            string `json:"integration_replay_result"`
	ReliabilityTrialResult             string `json:"reliability_trial_result"`
	RoleSquadOperatingContract         string `json:"role_squad_operating_contract"`
	RoleSquadReadinessV3               string `json:"role_squad_readiness_v3"`
	RecoverySensitiveExclusionEvidence string `json:"recovery_sensitive_exclusion_evidence"`
}

type InitialOperatingEligibilityRequirementsV1 struct {
	Repository                            string   `json:"repository"`
	Host                                  string   `json:"host"`
	WorkspaceInternalOnly                 bool     `json:"workspace_internal_only"`
	AdmissibleIssueStates                 []string `json:"admissible_issue_states"`
	OwnerStartedRequired                  bool     `json:"owner_started_required"`
	ExecutionPlanMustNameDistinctReviewer bool     `json:"execution_plan_must_name_distinct_reviewer"`
	RequiredDeclaredReleaseGate           string   `json:"required_declared_release_gate"`
	MaximumExistingProviderTasks          int      `json:"maximum_existing_provider_tasks"`
}

type InitialOperatingLaneV1 struct {
	Lane                         string   `json:"lane"`
	RuntimeProvider              string   `json:"runtime_provider"`
	TransportLane                string   `json:"transport_lane"`
	RequestedPrimaryModel        string   `json:"requested_primary_model"`
	AllowedObservedModels        []string `json:"allowed_observed_models"`
	ReasoningEffort              string   `json:"reasoning_effort"`
	ExactSingleModelClaimAllowed bool     `json:"exact_single_model_claim_allowed"`
	SelectionBasis               string   `json:"selection_basis"`
}

type InitialOperatingConfigurationV1 struct {
	SchemaVersion                                   string                                      `json:"schema_version"`
	ConfigurationID                                 string                                      `json:"configuration_id"`
	Status                                          string                                      `json:"status"`
	ValidFrom                                       string                                      `json:"valid_from"`
	ValidUntil                                      string                                      `json:"valid_until"`
	ReviewIntervalDays                              int                                         `json:"review_interval_days"`
	CoexistsWithConfigurationID                     string                                      `json:"coexists_with_configuration_id"`
	SupersedesConfigurationID                       string                                      `json:"supersedes_configuration_id"`
	SourceHashes                                    InitialOperatingConfigurationSourceHashesV1 `json:"source_hashes"`
	EligibleWorkClass                               string                                      `json:"eligible_work_class"`
	AdmissionMode                                   string                                      `json:"admission_mode"`
	EligibilityRequirements                         InitialOperatingEligibilityRequirementsV1   `json:"eligibility_requirements"`
	OwnerGatedConditions                            []string                                    `json:"owner_gated_conditions"`
	OwnerGateEvent                                  string                                      `json:"owner_gate_event"`
	OwnerGateState                                  string                                      `json:"owner_gate_state"`
	RouteStages                                     []string                                    `json:"route_stages"`
	ExecutionRole                                   string                                      `json:"execution_role"`
	ExecutionRoleArtifactBindingID                  string                                      `json:"execution_role_artifact_binding_id"`
	ReviewerRole                                    string                                      `json:"reviewer_role"`
	ReviewerRoleArtifactBindingID                   string                                      `json:"reviewer_role_artifact_binding_id"`
	SeparateExecutionAndReviewSessions              bool                                        `json:"separate_execution_and_review_sessions"`
	IndependentReviewRequired                       bool                                        `json:"independent_review_required"`
	ReviewIndependenceClass                         string                                      `json:"review_independence_class"`
	SingleProviderReviewLimitationRecorded          bool                                        `json:"single_provider_review_limitation_recorded"`
	PermanentAgentSeats                             int                                         `json:"permanent_agent_seats"`
	TaskScopedRolesOnly                             bool                                        `json:"task_scoped_roles_only"`
	TaskScopedTemporarySquadOnly                    bool                                        `json:"task_scoped_temporary_squad_only"`
	ControllerOwnedTransitions                      bool                                        `json:"controller_owned_transitions"`
	WorkflowStateContract                           string                                      `json:"workflow_state_contract"`
	EmittedWorkflowEvents                           []string                                    `json:"emitted_workflow_events"`
	ReviewFailedState                               string                                      `json:"review_failed_state"`
	ReviewFailedEscalation                          string                                      `json:"review_failed_escalation"`
	MaximumCorrections                              int                                         `json:"maximum_corrections"`
	MaximumProviderTasksPerIssue                    int                                         `json:"maximum_provider_tasks_per_issue"`
	MaximumConcurrentEligibleIssues                 int                                         `json:"maximum_concurrent_eligible_issues"`
	MaxAttemptsPerTask                              int                                         `json:"max_attempts_per_task"`
	Retries                                         int                                         `json:"retries"`
	Fallbacks                                       bool                                        `json:"fallbacks"`
	ExecutionLane                                   InitialOperatingLaneV1                      `json:"execution_lane"`
	ReviewLane                                      InitialOperatingLaneV1                      `json:"review_lane"`
	ReasoningEffortEvidence                         string                                      `json:"reasoning_effort_evidence"`
	ModelAssignmentIsProvisional                    bool                                        `json:"model_assignment_is_provisional"`
	PermanentRoleOrModelSelectionAllowed            bool                                        `json:"permanent_role_or_model_selection_allowed"`
	PermanentSquadActivationAllowed                 bool                                        `json:"permanent_squad_activation_allowed"`
	RecoverySensitiveAuthority                      string                                      `json:"recovery_sensitive_authority"`
	RecoverySensitiveExclusionReason                string                                      `json:"recovery_sensitive_exclusion_reason"`
	ReleaseAndRuntimeVerificationAuthority          string                                      `json:"release_and_runtime_verification_authority"`
	RequiredRuntimeBindingSchema                    string                                      `json:"required_runtime_binding_schema"`
	RequiredCapabilityScopeSchema                   string                                      `json:"required_capability_scope_schema"`
	SeparatePerRunAuthorityRequired                 bool                                        `json:"separate_per_run_authority_required"`
	ConfigurationAuthorizesProviderCall             bool                                        `json:"configuration_authorizes_provider_call"`
	ConfigurationAuthorizesCredentialRead           bool                                        `json:"configuration_authorizes_credential_read"`
	ProviderChildToolsAllowed                       bool                                        `json:"provider_child_tools_allowed"`
	ProviderChildRepositoryAccessAllowed            bool                                        `json:"provider_child_repository_access_allowed"`
	DeterministicBrokerRepositoryReadAllowed        bool                                        `json:"deterministic_broker_repository_read_allowed"`
	DeterministicBrokerAdditiveEvidenceWriteAllowed bool                                        `json:"deterministic_broker_additive_evidence_write_allowed"`
	AutomaticDispatchAllowed                        bool                                        `json:"automatic_dispatch_allowed"`
	PublicAccessAllowed                             bool                                        `json:"public_access_allowed"`
	ProductionAccessAllowed                         bool                                        `json:"production_access_allowed"`
	OtherProductionSystemAccessAllowed              bool                                        `json:"other_production_system_access_allowed"`
	OtherRepositoryAccessAllowed                    bool                                        `json:"other_repository_access_allowed"`
	OutboundActionsAllowed                          bool                                        `json:"outbound_actions_allowed"`
	PhoneFarmWindowsAccessAllowed                   bool                                        `json:"phone_farm_windows_access_allowed"`
	ProvisionalItems                                []string                                    `json:"provisional_items"`
	ReevaluationTriggers                            []string                                    `json:"reevaluation_triggers"`
	Enforcement                                     string                                      `json:"enforcement"`
	EnforcementNote                                 string                                      `json:"enforcement_note"`
	ChangePath                                      string                                      `json:"change_path"`
	RollbackAction                                  string                                      `json:"rollback_action"`
	Reversible                                      bool                                        `json:"reversible"`
	RetireOnEvidenceDrift                           bool                                        `json:"retire_on_evidence_drift"`
	RetireOnTransportDrift                          bool                                        `json:"retire_on_transport_drift"`
	RetireOnScopeExpansion                          bool                                        `json:"retire_on_scope_expansion"`
}

//go:embed initial_operating_configuration_v1.json
var initialOperatingConfigurationV1JSON []byte

func CurrentInitialOperatingConfigurationV1() (InitialOperatingConfigurationV1, error) {
	var configuration InitialOperatingConfigurationV1
	if err := strictJSON(initialOperatingConfigurationV1JSON, &configuration); err != nil {
		return configuration, fmt.Errorf("decode initial operating configuration v1: %w", err)
	}
	return configuration, ValidateInitialOperatingConfigurationV1(configuration)
}

func InitialOperatingConfigurationV1SHA256() string {
	sum := sha256.Sum256(initialOperatingConfigurationV1JSON)
	return hex.EncodeToString(sum[:])
}

func initialOperatingConfigurationSourceHashesV1() InitialOperatingConfigurationSourceHashesV1 {
	return InitialOperatingConfigurationSourceHashesV1{
		NativePublicationAcceptance:        NativePublicationAcceptanceV1SHA256(),
		ProvisionalConfigurationV2:         ProvisionalRoleConfigurationV2SHA256(),
		IntegrationReplayResult:            IntegrationReplayResultV1SHA256(),
		ReliabilityTrialResult:             PrimarySubscriptionReliabilityTrialResultV2SHA256(),
		RoleSquadOperatingContract:         RoleSquadOperatingContractV1SHA256(),
		RoleSquadReadinessV3:               RoleSquadReadinessMatrixV3SHA256(),
		RecoverySensitiveExclusionEvidence: ClaudeSubscriptionRealisticV38FreezeSHA256(),
	}
}

// InitialOperatingOwnerGatedConditionsV1 is the exact, closed set of conditions
// that stop automatic work and wake the owner. Anything outside this list is an
// unrecognised condition and also stops the work, rather than being ignored.
func InitialOperatingOwnerGatedConditionsV1() []string {
	return []string{
		"requires_business_or_product_decision",
		"requires_broad_or_undefined_scope_change",
		"requires_credential_or_identity_change",
		"requires_destructive_or_irreversible_data_change",
		"requires_new_credential_source_or_spending",
		"requires_outbound_communication",
		"requires_phone_farm_access",
		"requires_production_deployment_or_rollback",
		"requires_public_exposure_change",
		"requires_recovery_or_restore_action",
		"touches_another_repository_or_host",
		"unresolved_high_risk_ambiguity",
	}
}

func ValidateInitialOperatingConfigurationV1(configuration InitialOperatingConfigurationV1) error {
	if configuration.SchemaVersion != InitialOperatingConfigurationV1Schema || configuration.ConfigurationID != initialOperatingConfigurationV1ID || configuration.Status != "accepted_reversible_initial_operating_configuration" || configuration.ValidFrom != "2026-09-16T00:00:00Z" || configuration.ValidUntil != "2026-09-30T00:00:00Z" || configuration.ReviewIntervalDays != 14 {
		return fmt.Errorf("initial operating configuration v1 identity drifted")
	}
	// This record adds a new work class; it does not quietly retire the separate
	// R01-like local default, which keeps its own window and lifecycle.
	if configuration.CoexistsWithConfigurationID != "item-14-local-r01-guarded-claude-composite-20260915" || configuration.SupersedesConfigurationID != "" {
		return fmt.Errorf("initial operating configuration v1 predecessor relationship drifted")
	}
	if configuration.SourceHashes != initialOperatingConfigurationSourceHashesV1() {
		return fmt.Errorf("initial operating configuration v1 source binding drifted")
	}
	if configuration.EligibleWorkClass != "internal_bounded_engineering_issue" || configuration.AdmissionMode != "owner_started_per_issue" {
		return fmt.Errorf("initial operating configuration v1 work class drifted")
	}
	wantRequirements := InitialOperatingEligibilityRequirementsV1{
		Repository:                            "SMK-multica-selfhost-pilot",
		Host:                                  "alienware_windows",
		WorkspaceInternalOnly:                 true,
		AdmissibleIssueStates:                 []string{"received", "queued"},
		OwnerStartedRequired:                  true,
		ExecutionPlanMustNameDistinctReviewer: true,
		RequiredDeclaredReleaseGate:           "none",
		MaximumExistingProviderTasks:          0,
	}
	if configuration.EligibilityRequirements.Repository != wantRequirements.Repository || configuration.EligibilityRequirements.Host != wantRequirements.Host || !configuration.EligibilityRequirements.WorkspaceInternalOnly || !slices.Equal(configuration.EligibilityRequirements.AdmissibleIssueStates, wantRequirements.AdmissibleIssueStates) || !configuration.EligibilityRequirements.OwnerStartedRequired || !configuration.EligibilityRequirements.ExecutionPlanMustNameDistinctReviewer || configuration.EligibilityRequirements.RequiredDeclaredReleaseGate != wantRequirements.RequiredDeclaredReleaseGate || configuration.EligibilityRequirements.MaximumExistingProviderTasks != 0 {
		return fmt.Errorf("initial operating configuration v1 eligibility requirements drifted")
	}
	if !slices.Equal(configuration.OwnerGatedConditions, InitialOperatingOwnerGatedConditionsV1()) || configuration.OwnerGateEvent != "owner_decision_required" || configuration.OwnerGateState != "needs_owner" {
		return fmt.Errorf("initial operating configuration v1 owner gate drifted")
	}
	executionRef, err := roleSquadArtifactRefV1("shared_windows_executor")
	if err != nil {
		return err
	}
	reviewerRef, err := roleSquadArtifactRefV1("quality_checker")
	if err != nil {
		return err
	}
	if !slices.Equal(configuration.RouteStages, []string{"controller_intake", "task_scoped_executor_session", "task_scoped_independent_reviewer_session"}) || configuration.ExecutionRole != "shared_windows_executor" || configuration.ExecutionRoleArtifactBindingID != executionRef.BindingID || configuration.ReviewerRole != "task_scoped_distinct_quality_checker" || configuration.ReviewerRoleArtifactBindingID != reviewerRef.BindingID {
		return fmt.Errorf("initial operating configuration v1 route drifted")
	}
	if !configuration.SeparateExecutionAndReviewSessions || !configuration.IndependentReviewRequired || configuration.ReviewIndependenceClass != "single_provider_distinct_session" || !configuration.SingleProviderReviewLimitationRecorded {
		return fmt.Errorf("initial operating configuration v1 review independence drifted")
	}
	// A temporary task-scoped role is enough for this work class, so no permanent
	// seat is created and none may be claimed later without a successor record.
	if configuration.PermanentAgentSeats != 0 || !configuration.TaskScopedRolesOnly || !configuration.TaskScopedTemporarySquadOnly || !configuration.ControllerOwnedTransitions || configuration.WorkflowStateContract != "docs/silver-mediums/WORKFLOW-STATE-CONTRACT.md" {
		return fmt.Errorf("initial operating configuration v1 organizational shape drifted")
	}
	if !slices.Equal(configuration.EmittedWorkflowEvents, []string{"implementation_ready", "review_passed", "review_failed", "owner_decision_required"}) {
		return fmt.Errorf("initial operating configuration v1 workflow event set drifted")
	}
	if configuration.MaximumCorrections != 0 || configuration.MaximumProviderTasksPerIssue != 2 || configuration.MaximumConcurrentEligibleIssues != 1 || configuration.MaxAttemptsPerTask != 1 || configuration.Retries != 0 || configuration.Fallbacks {
		return fmt.Errorf("initial operating configuration v1 execution bounds drifted")
	}
	// With no automatic correction, a failed review has to have a declared
	// destination. Leaving it unstated would make the first failure undefined.
	if configuration.ReviewFailedState != "blocked" || configuration.ReviewFailedEscalation != "owner_decision_required" {
		return fmt.Errorf("initial operating configuration v1 failed-review destination drifted")
	}
	if err := validateInitialOperatingLaneV1(configuration.ExecutionLane, "execution", "only_configuration_with_an_accepted_end_to_end_native_run"); err != nil {
		return err
	}
	if err := validateInitialOperatingLaneV1(configuration.ReviewLane, "independent_review", "no_second_provider_is_authorized_on_this_host_so_independence_is_session_level_only"); err != nil {
		return err
	}
	if !configuration.ModelAssignmentIsProvisional || configuration.PermanentRoleOrModelSelectionAllowed || configuration.PermanentSquadActivationAllowed {
		return fmt.Errorf("initial operating configuration v1 claimed a permanent selection")
	}
	// The accepted run recorded no reasoning effort, so the record may only
	// describe the effort it requests, never an observed one.
	if configuration.ReasoningEffortEvidence != "requested_only_not_observed_in_the_accepted_run" {
		return fmt.Errorf("initial operating configuration v1 overstated its effort evidence")
	}
	// Nothing in the running product consults this record, and it may not imply
	// otherwise.
	if configuration.Enforcement != "declarative_contract_checked_out_of_band" || configuration.EnforcementNote == "" {
		return fmt.Errorf("initial operating configuration v1 claimed an enforcement it does not have")
	}
	if configuration.RecoverySensitiveAuthority != "withheld" || configuration.RecoverySensitiveExclusionReason != "claude_subscription_composite_failed_r04_exact_once_recovery" || configuration.ReleaseAndRuntimeVerificationAuthority != "owner_operated" {
		return fmt.Errorf("initial operating configuration v1 recovery boundary drifted")
	}
	if !configuration.SeparatePerRunAuthorityRequired || configuration.RequiredRuntimeBindingSchema != QualifiedRuntimeBindingVersionV2 || configuration.RequiredCapabilityScopeSchema != TaskCapabilityScopeVersionV3 || configuration.ConfigurationAuthorizesProviderCall || configuration.ConfigurationAuthorizesCredentialRead || configuration.ProviderChildToolsAllowed || configuration.ProviderChildRepositoryAccessAllowed || !configuration.DeterministicBrokerRepositoryReadAllowed || !configuration.DeterministicBrokerAdditiveEvidenceWriteAllowed {
		return fmt.Errorf("initial operating configuration v1 expanded execution authority")
	}
	if configuration.AutomaticDispatchAllowed || configuration.PublicAccessAllowed || configuration.ProductionAccessAllowed || configuration.OtherProductionSystemAccessAllowed || configuration.OtherRepositoryAccessAllowed || configuration.OutboundActionsAllowed || configuration.PhoneFarmWindowsAccessAllowed {
		return fmt.Errorf("initial operating configuration v1 expanded environmental authority")
	}
	if len(configuration.ProvisionalItems) < 8 || len(configuration.ReevaluationTriggers) < 5 || configuration.ChangePath == "" || configuration.RollbackAction == "" {
		return fmt.Errorf("initial operating configuration v1 did not record what stays provisional")
	}
	if !configuration.Reversible || !configuration.RetireOnEvidenceDrift || !configuration.RetireOnTransportDrift || !configuration.RetireOnScopeExpansion {
		return fmt.Errorf("initial operating configuration v1 reversibility drifted")
	}
	validFrom, err := time.Parse(time.RFC3339, configuration.ValidFrom)
	if err != nil {
		return fmt.Errorf("parse initial operating configuration v1 valid_from: %w", err)
	}
	validUntil, err := time.Parse(time.RFC3339, configuration.ValidUntil)
	if err != nil || !validUntil.After(validFrom) || validUntil.Sub(validFrom) != time.Duration(configuration.ReviewIntervalDays)*24*time.Hour {
		return fmt.Errorf("initial operating configuration v1 validity window drifted")
	}
	acceptance, err := CurrentNativePublicationAcceptanceV1()
	if err != nil {
		return err
	}
	// The configuration may not describe a run larger than the one accepted.
	if configuration.MaximumProviderTasksPerIssue != acceptance.PublisherTurns+acceptance.ReviewerTurns || configuration.MaximumCorrections != acceptance.Corrections || configuration.Retries != acceptance.Retries {
		return fmt.Errorf("initial operating configuration v1 exceeded the accepted run shape")
	}
	if configuration.ExecutionLane.TransportLane != acceptance.TransportLane || configuration.ExecutionLane.RequestedPrimaryModel != acceptance.RequestedPrimaryModel || !slices.Equal(configuration.ExecutionLane.AllowedObservedModels, acceptance.ObservedModels) {
		return fmt.Errorf("initial operating configuration v1 exceeded the accepted transport evidence")
	}
	if acceptance.ReviewerWasDistinctProvider != (configuration.ReviewIndependenceClass == "distinct_provider") {
		return fmt.Errorf("initial operating configuration v1 overstated review independence")
	}
	if acceptance.RecoverySensitiveAuthorityDemonstrated || configuration.RecoverySensitiveAuthority != "withheld" {
		return fmt.Errorf("initial operating configuration v1 claimed unproven recovery authority")
	}
	exclusion, err := CurrentClaudeSubscriptionRealisticV38Freeze()
	if err != nil {
		return err
	}
	if exclusion.FreezeID != "claude-subscription-realistic-r04-phased-freeze-v38" || !slices.Equal(exclusion.ExecutionOrder, []string{"R04:v1"}) {
		return fmt.Errorf("initial operating configuration v1 recovery exclusion evidence drifted")
	}
	replay, err := CurrentIntegrationReplayResultV1()
	if err != nil {
		return err
	}
	if replay.Transport.Lane != configuration.ExecutionLane.TransportLane || replay.Transport.ExactSingleModelProven {
		return fmt.Errorf("initial operating configuration v1 exceeded integration evidence")
	}
	return nil
}

func validateInitialOperatingLaneV1(lane InitialOperatingLaneV1, wantLane, wantBasis string) error {
	if lane.Lane != wantLane || lane.RuntimeProvider != "claude" || lane.TransportLane != "claude_code_subscription_composite" || lane.RequestedPrimaryModel != "claude-sonnet-4-6" || !slices.Equal(lane.AllowedObservedModels, []string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"}) || lane.ReasoningEffort != "medium" || lane.ExactSingleModelClaimAllowed || lane.SelectionBasis != wantBasis {
		return fmt.Errorf("initial operating configuration v1 %s lane drifted", wantLane)
	}
	return nil
}

// InitialOperatingAdmissionFactsV1 are the facts a caller must supply about one
// real issue. Every field is required; an unstated fact is not a satisfied one.
type InitialOperatingAdmissionFactsV1 struct {
	Repository                         string
	Host                               string
	WorkspaceInternal                  bool
	IssueState                         string
	OwnerStarted                       bool
	ExecutionPlanNamesDistinctReviewer bool
	DeclaredReleaseGate                string
	ExistingProviderTasks              int
	ConcurrentEligibleIssues           int // issues already in flight under this configuration
	OwnerGatedConditions               []string
}

// EvaluateInitialOperatingAdmissionV1 decides whether one issue may run under
// this configuration. It fails closed: the first unmet requirement denies, an
// owner-gated condition denies and names the owner gate, and a condition the
// configuration does not recognise denies rather than being ignored.
func EvaluateInitialOperatingAdmissionV1(configuration InitialOperatingConfigurationV1, facts InitialOperatingAdmissionFactsV1, now time.Time) (bool, string) {
	if err := ValidateInitialOperatingConfigurationV1(configuration); err != nil {
		return false, "configuration_invalid"
	}
	validFrom, err := time.Parse(time.RFC3339, configuration.ValidFrom)
	if err != nil {
		return false, "configuration_invalid"
	}
	validUntil, err := time.Parse(time.RFC3339, configuration.ValidUntil)
	if err != nil {
		return false, "configuration_invalid"
	}
	now = now.UTC()
	if now.Before(validFrom) {
		return false, "configuration_not_yet_valid"
	}
	if !now.Before(validUntil) {
		return false, "configuration_expired"
	}
	requirements := configuration.EligibilityRequirements
	if facts.Repository != requirements.Repository {
		return false, "repository_out_of_scope"
	}
	if facts.Host != requirements.Host {
		return false, "host_out_of_scope"
	}
	if requirements.WorkspaceInternalOnly && !facts.WorkspaceInternal {
		return false, "workspace_not_internal"
	}
	if !slices.Contains(requirements.AdmissibleIssueStates, facts.IssueState) {
		return false, "issue_state_not_admissible"
	}
	if requirements.OwnerStartedRequired && !facts.OwnerStarted {
		return false, "owner_did_not_start_this_issue"
	}
	if requirements.ExecutionPlanMustNameDistinctReviewer && !facts.ExecutionPlanNamesDistinctReviewer {
		return false, "execution_plan_does_not_name_a_distinct_reviewer"
	}
	if facts.DeclaredReleaseGate != requirements.RequiredDeclaredReleaseGate {
		return false, "declared_release_gate_out_of_scope"
	}
	if facts.ExistingProviderTasks > requirements.MaximumExistingProviderTasks {
		return false, "issue_already_has_provider_tasks"
	}
	// ConcurrentEligibleIssues counts issues already in flight, so admitting this
	// one has to leave room under the ceiling.
	if facts.ConcurrentEligibleIssues >= configuration.MaximumConcurrentEligibleIssues {
		return false, "concurrent_eligible_issue_limit_reached"
	}
	recognised := InitialOperatingOwnerGatedConditionsV1()
	// An unrecognised condition anywhere in the list denies first: it means the
	// caller knows something this configuration has no rule for.
	for _, condition := range facts.OwnerGatedConditions {
		if !slices.Contains(recognised, condition) {
			return false, "unrecognised_condition_requires_owner"
		}
	}
	if len(facts.OwnerGatedConditions) > 0 {
		return false, facts.OwnerGatedConditions[0]
	}
	return true, "admitted"
}
