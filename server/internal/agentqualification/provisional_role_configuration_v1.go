package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
)

const ProvisionalRoleConfigurationV1Schema = "silver-mediums.provisional-role-configuration.v1"

type ProvisionalRoleConfigurationSourceHashesV1 struct {
	ReliabilityTrialResult     string `json:"reliability_trial_result"`
	RoleStageContract          string `json:"role_stage_contract"`
	RoleStageRehearsalResult   string `json:"role_stage_rehearsal_result"`
	RoleSquadOperatingContract string `json:"role_squad_operating_contract"`
}

type ProvisionalRoleConfigurationV1 struct {
	SchemaVersion                 string                                     `json:"schema_version"`
	ConfigurationID               string                                     `json:"configuration_id"`
	Status                        string                                     `json:"status"`
	SourceHashes                  ProvisionalRoleConfigurationSourceHashesV1 `json:"source_hashes"`
	AllowedCaseIDs                []string                                   `json:"allowed_case_ids"`
	Participant                   string                                     `json:"participant"`
	ParticipantKind               string                                     `json:"participant_kind"`
	ParticipantArtifactBindingID  string                                     `json:"participant_artifact_binding_id"`
	InlineSkillPacket             string                                     `json:"inline_skill_packet"`
	InlineSkillArtifactBindingID  string                                     `json:"inline_skill_artifact_binding_id"`
	ExecutionMode                 string                                     `json:"execution_mode"`
	ProviderLane                  string                                     `json:"provider_lane"`
	Model                         string                                     `json:"model"`
	Squad                         string                                     `json:"squad"`
	ExcludedScope                 []string                                   `json:"excluded_scope"`
	Reversible                    bool                                       `json:"reversible"`
	AutomaticDispatchAllowed      bool                                       `json:"automatic_dispatch_allowed"`
	ProviderCallsAllowed          bool                                       `json:"provider_calls_allowed"`
	CredentialReadsAllowed        bool                                       `json:"credential_reads_allowed"`
	NativeToolsAllowed            bool                                       `json:"native_tools_allowed"`
	RepositoryAccessAllowed       bool                                       `json:"repository_access_allowed"`
	ProductionAccessAllowed       bool                                       `json:"production_access_allowed"`
	OutboundActionsAllowed        bool                                       `json:"outbound_actions_allowed"`
	PhoneFarmWindowsAccessAllowed bool                                       `json:"phone_farm_windows_access_allowed"`
	Retries                       int                                        `json:"retries"`
	Fallbacks                     bool                                       `json:"fallbacks"`
	PermanentSelectionAllowed     bool                                       `json:"permanent_selection_allowed"`
	ProductionActivationAllowed   bool                                       `json:"production_activation_allowed"`
}

//go:embed provisional_role_configuration_v1.json
var provisionalRoleConfigurationV1JSON []byte

func CurrentProvisionalRoleConfigurationV1() (ProvisionalRoleConfigurationV1, error) {
	var configuration ProvisionalRoleConfigurationV1
	if err := strictJSON(provisionalRoleConfigurationV1JSON, &configuration); err != nil {
		return configuration, fmt.Errorf("decode provisional role configuration: %w", err)
	}
	return configuration, ValidateProvisionalRoleConfigurationV1(configuration)
}

func ProvisionalRoleConfigurationV1SHA256() string {
	sum := sha256.Sum256(provisionalRoleConfigurationV1JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateProvisionalRoleConfigurationV1(configuration ProvisionalRoleConfigurationV1) error {
	if configuration.SchemaVersion != ProvisionalRoleConfigurationV1Schema || configuration.ConfigurationID != "item-14-local-r01-inline-architecture-20260914" || configuration.Status != "accepted_reversible_provider_free_local_scope" {
		return fmt.Errorf("provisional role configuration identity drifted")
	}
	if configuration.SourceHashes != (ProvisionalRoleConfigurationSourceHashesV1{ReliabilityTrialResult: PrimarySubscriptionReliabilityTrialResultV2SHA256(), RoleStageContract: RoleStageEvidenceContractV1SHA256(), RoleStageRehearsalResult: hashEmbedded(roleStageEvidenceRehearsalV1ResultJSON), RoleSquadOperatingContract: RoleSquadOperatingContractV1SHA256()}) {
		return fmt.Errorf("provisional role configuration source binding drifted")
	}
	participantRef, err := roleSquadArtifactRefV1("shared_windows_executor")
	if err != nil {
		return err
	}
	skillRef, err := roleSquadArtifactRefV1("architecture_assurance")
	if err != nil {
		return err
	}
	if !slices.Equal(configuration.AllowedCaseIDs, []string{"R01"}) || configuration.Participant != "shared_windows_executor" || configuration.ParticipantKind != "executor" || configuration.ParticipantArtifactBindingID != participantRef.BindingID || configuration.InlineSkillPacket != "architecture_assurance" || configuration.InlineSkillArtifactBindingID != skillRef.BindingID || configuration.ExecutionMode != "deterministic_provider_free_local_fixture_only" || configuration.ProviderLane != "unselected" || configuration.Model != "unselected" || configuration.Squad != "unselected" {
		return fmt.Errorf("provisional role configuration exact scope drifted")
	}
	wantExcluded := []string{"R02", "product_ui", "provider_or_model_selection", "separate_specialist_handoff", "real_repository", "production", "outbound_business_action", "phone_farm", "automatic_retry_or_fallback", "permanent_role_or_seat"}
	if !slices.Equal(configuration.ExcludedScope, wantExcluded) || !configuration.Reversible || configuration.AutomaticDispatchAllowed || configuration.ProviderCallsAllowed || configuration.CredentialReadsAllowed || configuration.NativeToolsAllowed || configuration.RepositoryAccessAllowed || configuration.ProductionAccessAllowed || configuration.OutboundActionsAllowed || configuration.PhoneFarmWindowsAccessAllowed || configuration.Retries != 0 || configuration.Fallbacks || configuration.PermanentSelectionAllowed || configuration.ProductionActivationAllowed {
		return fmt.Errorf("provisional role configuration expanded authority")
	}
	result, err := CurrentPrimarySubscriptionReliabilityTrialResultV2()
	if err != nil {
		return err
	}
	if result.ProvisionalConfiguration != "local_r01_inline_shared_executor_exact_skill_provider_unselected" || result.Cells[0].Passes != 3 || result.Cells[2].Passes != 3 || result.Cells[1].Passes != 0 || result.Cells[3].Passes != 0 {
		return fmt.Errorf("provisional role configuration exceeded reliability evidence")
	}
	return nil
}

func hashEmbedded(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
