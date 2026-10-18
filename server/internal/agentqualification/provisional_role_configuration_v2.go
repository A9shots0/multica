package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
	"time"
)

const ProvisionalRoleConfigurationV2Schema = "silver-mediums.provisional-role-configuration.v2"

type ProvisionalRoleConfigurationSourceHashesV2 struct {
	IntegrationReplayResult    string `json:"integration_replay_result"`
	ReliabilityTrialResult     string `json:"reliability_trial_result"`
	ProvisionalConfigurationV1 string `json:"provisional_configuration_v1"`
	ProvisionalRehearsalV1     string `json:"provisional_rehearsal_v1"`
	RoleSquadOperatingContract string `json:"role_squad_operating_contract"`
	RoleSquadReadinessV2       string `json:"role_squad_readiness_v2"`
}

type ProvisionalRoleConfigurationV2 struct {
	SchemaVersion                                   string                                     `json:"schema_version"`
	ConfigurationID                                 string                                     `json:"configuration_id"`
	Status                                          string                                     `json:"status"`
	ValidFrom                                       string                                     `json:"valid_from"`
	ValidUntil                                      string                                     `json:"valid_until"`
	SupersedesConfigurationID                       string                                     `json:"supersedes_configuration_id"`
	SupersedesConfigurationSHA256                   string                                     `json:"supersedes_configuration_sha256"`
	SourceHashes                                    ProvisionalRoleConfigurationSourceHashesV2 `json:"source_hashes"`
	CaseFamily                                      string                                     `json:"case_family"`
	WorkflowKind                                    string                                     `json:"workflow_kind"`
	RouteStages                                     []string                                   `json:"route_stages"`
	Participant                                     string                                     `json:"participant"`
	ParticipantArtifactBindingID                    string                                     `json:"participant_artifact_binding_id"`
	InlineSkillPacket                               string                                     `json:"inline_skill_packet"`
	InlineSkillArtifactBindingID                    string                                     `json:"inline_skill_artifact_binding_id"`
	IntakeRole                                      string                                     `json:"intake_role"`
	ReviewerRole                                    string                                     `json:"reviewer_role"`
	ReviewerArtifactBindingID                       string                                     `json:"reviewer_artifact_binding_id"`
	IndependentReviewRequired                       bool                                       `json:"independent_review_required"`
	MaximumCorrections                              int                                        `json:"maximum_corrections"`
	MaximumProviderTasks                            int                                        `json:"maximum_provider_tasks"`
	ExecutionMode                                   string                                     `json:"execution_mode"`
	TransportLane                                   string                                     `json:"transport_lane"`
	RuntimeProvider                                 string                                     `json:"runtime_provider"`
	RequestedPrimaryModel                           string                                     `json:"requested_primary_model"`
	AllowedObservedModels                           []string                                   `json:"allowed_observed_models"`
	ReasoningEffort                                 string                                     `json:"reasoning_effort"`
	ExactSingleModelClaimAllowed                    bool                                       `json:"exact_single_model_claim_allowed"`
	SeparatePerRunAuthorityRequired                 bool                                       `json:"separate_per_run_authority_required"`
	RequiredRuntimeBindingSchema                    string                                     `json:"required_runtime_binding_schema"`
	RequiredCapabilityScopeSchema                   string                                     `json:"required_capability_scope_schema"`
	ConfigurationAuthorizesProviderCall             bool                                       `json:"configuration_authorizes_provider_call"`
	ConfigurationAuthorizesCredentialRead           bool                                       `json:"configuration_authorizes_credential_read"`
	ProviderChildToolsAllowed                       bool                                       `json:"provider_child_tools_allowed"`
	ProviderChildRepositoryAccessAllowed            bool                                       `json:"provider_child_repository_access_allowed"`
	DeterministicBrokerRepositoryReadAllowed        bool                                       `json:"deterministic_broker_repository_read_allowed"`
	DeterministicBrokerAdditiveEvidenceWriteAllowed bool                                       `json:"deterministic_broker_additive_evidence_write_allowed"`
	ExistingFileMutationAllowed                     bool                                       `json:"existing_file_mutation_allowed"`
	AutomaticDispatchAllowed                        bool                                       `json:"automatic_dispatch_allowed"`
	MaxAttemptsPerTask                              int                                        `json:"max_attempts_per_task"`
	Retries                                         int                                        `json:"retries"`
	Fallbacks                                       bool                                       `json:"fallbacks"`
	TaskScopedTemporarySquadOnly                    bool                                       `json:"task_scoped_temporary_squad_only"`
	PermanentRoleOrSeatSelectionAllowed             bool                                       `json:"permanent_role_or_seat_selection_allowed"`
	PermanentSquadActivationAllowed                 bool                                       `json:"permanent_squad_activation_allowed"`
	ProductionAccessAllowed                         bool                                       `json:"production_access_allowed"`
	OutboundActionsAllowed                          bool                                       `json:"outbound_actions_allowed"`
	PhoneFarmWindowsAccessAllowed                   bool                                       `json:"phone_farm_windows_access_allowed"`
	Reversible                                      bool                                       `json:"reversible"`
	RetireOnEvidenceDrift                           bool                                       `json:"retire_on_evidence_drift"`
	RetireOnTransportDrift                          bool                                       `json:"retire_on_transport_drift"`
	RetireOnScopeExpansion                          bool                                       `json:"retire_on_scope_expansion"`
}

//go:embed provisional_role_configuration_v2.json
var provisionalRoleConfigurationV2JSON []byte

func CurrentProvisionalRoleConfigurationV2() (ProvisionalRoleConfigurationV2, error) {
	var configuration ProvisionalRoleConfigurationV2
	if err := strictJSON(provisionalRoleConfigurationV2JSON, &configuration); err != nil {
		return configuration, fmt.Errorf("decode provisional role configuration v2: %w", err)
	}
	return configuration, ValidateProvisionalRoleConfigurationV2(configuration)
}

func ProvisionalRoleConfigurationV2SHA256() string {
	sum := sha256.Sum256(provisionalRoleConfigurationV2JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateProvisionalRoleConfigurationV2(configuration ProvisionalRoleConfigurationV2) error {
	if configuration.SchemaVersion != ProvisionalRoleConfigurationV2Schema || configuration.ConfigurationID != "item-14-local-r01-guarded-claude-composite-20260915" || configuration.Status != "accepted_reversible_local_operating_default" || configuration.ValidFrom != "2026-09-15T13:15:00Z" || configuration.ValidUntil != "2026-09-22T13:15:00Z" {
		return fmt.Errorf("provisional role configuration v2 identity drifted")
	}
	if configuration.SupersedesConfigurationID != "item-14-local-r01-inline-architecture-20260914" || configuration.SupersedesConfigurationSHA256 != ProvisionalRoleConfigurationV1SHA256() {
		return fmt.Errorf("provisional role configuration v2 predecessor drifted")
	}
	wantSources := ProvisionalRoleConfigurationSourceHashesV2{
		IntegrationReplayResult:    IntegrationReplayResultV1SHA256(),
		ReliabilityTrialResult:     PrimarySubscriptionReliabilityTrialResultV2SHA256(),
		ProvisionalConfigurationV1: ProvisionalRoleConfigurationV1SHA256(),
		ProvisionalRehearsalV1:     ProvisionalRoleConfigurationRehearsalV1SHA256(),
		RoleSquadOperatingContract: RoleSquadOperatingContractV1SHA256(),
		RoleSquadReadinessV2:       RoleSquadReadinessMatrixV2SHA256(),
	}
	if configuration.SourceHashes != wantSources {
		return fmt.Errorf("provisional role configuration v2 source binding drifted")
	}
	participantRef, err := roleSquadArtifactRefV1("shared_windows_executor")
	if err != nil {
		return err
	}
	skillRef, err := roleSquadArtifactRefV1("architecture_assurance")
	if err != nil {
		return err
	}
	reviewerRef, err := roleSquadArtifactRefV1("quality_checker")
	if err != nil {
		return err
	}
	if configuration.CaseFamily != "R01_like_controller_bound_read_only_local_inventory" || configuration.WorkflowKind != "controller_bound_read_only_windows_repository_inventory" || !slices.Equal(configuration.RouteStages, []string{"task_scoped_intake_leader", "shared_windows_executor", "task_scoped_independent_reviewer"}) || configuration.Participant != "shared_windows_executor" || configuration.ParticipantArtifactBindingID != participantRef.BindingID || configuration.InlineSkillPacket != "architecture_assurance" || configuration.InlineSkillArtifactBindingID != skillRef.BindingID || configuration.IntakeRole != "task_scoped_intake_leader" || configuration.ReviewerRole != "task_scoped_distinct_quality_checker" || configuration.ReviewerArtifactBindingID != reviewerRef.BindingID {
		return fmt.Errorf("provisional role configuration v2 route drifted")
	}
	if !configuration.IndependentReviewRequired || configuration.MaximumCorrections != 0 || configuration.MaximumProviderTasks != 3 || configuration.ExecutionMode != "manual_guarded_local_integration_only" || configuration.TransportLane != "claude_code_subscription_composite" || configuration.RuntimeProvider != "claude" || configuration.RequestedPrimaryModel != "claude-sonnet-4-6" || !slices.Equal(configuration.AllowedObservedModels, []string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"}) || configuration.ReasoningEffort != "medium" || configuration.ExactSingleModelClaimAllowed {
		return fmt.Errorf("provisional role configuration v2 execution binding drifted")
	}
	if !configuration.SeparatePerRunAuthorityRequired || configuration.RequiredRuntimeBindingSchema != QualifiedRuntimeBindingVersionV2 || configuration.RequiredCapabilityScopeSchema != TaskCapabilityScopeVersionV3 || configuration.ConfigurationAuthorizesProviderCall || configuration.ConfigurationAuthorizesCredentialRead || configuration.ProviderChildToolsAllowed || configuration.ProviderChildRepositoryAccessAllowed || !configuration.DeterministicBrokerRepositoryReadAllowed || !configuration.DeterministicBrokerAdditiveEvidenceWriteAllowed || configuration.ExistingFileMutationAllowed || configuration.AutomaticDispatchAllowed || configuration.MaxAttemptsPerTask != 1 || configuration.Retries != 0 || configuration.Fallbacks {
		return fmt.Errorf("provisional role configuration v2 expanded execution authority")
	}
	if !configuration.TaskScopedTemporarySquadOnly || configuration.PermanentRoleOrSeatSelectionAllowed || configuration.PermanentSquadActivationAllowed || configuration.ProductionAccessAllowed || configuration.OutboundActionsAllowed || configuration.PhoneFarmWindowsAccessAllowed || !configuration.Reversible || !configuration.RetireOnEvidenceDrift || !configuration.RetireOnTransportDrift || !configuration.RetireOnScopeExpansion {
		return fmt.Errorf("provisional role configuration v2 expanded organizational authority")
	}
	validFrom, err := time.Parse(time.RFC3339, configuration.ValidFrom)
	if err != nil {
		return fmt.Errorf("parse provisional role configuration v2 valid_from: %w", err)
	}
	validUntil, err := time.Parse(time.RFC3339, configuration.ValidUntil)
	if err != nil || !validUntil.After(validFrom) || validUntil.Sub(validFrom) != 7*24*time.Hour {
		return fmt.Errorf("provisional role configuration v2 validity window drifted")
	}
	reliability, err := CurrentPrimarySubscriptionReliabilityTrialResultV2()
	if err != nil {
		return err
	}
	if reliability.Cells[0].CaseID != "R01" || reliability.Cells[0].Passes != 3 || reliability.Cells[2].CaseID != "R01" || reliability.Cells[2].Passes != 3 || reliability.Cells[1].Passes != 0 || reliability.Cells[3].Passes != 0 {
		return fmt.Errorf("provisional role configuration v2 exceeded reliability evidence")
	}
	replay, err := CurrentIntegrationReplayResultV1()
	if err != nil {
		return err
	}
	if replay.ReviewOutcome != "ITEM14_REVIEW_PASS" || replay.ProviderTaskInvocations != 3 || replay.CorrectionCount != 0 || replay.Transport.Lane != configuration.TransportLane || replay.Transport.RequestedPrimaryModel != configuration.RequestedPrimaryModel || replay.Transport.ExactSingleModelProven || replay.GuardRoleInterpretation != "transport_guard_label_only_not_permanent_role_qualification" {
		return fmt.Errorf("provisional role configuration v2 exceeded integration evidence")
	}
	return nil
}
