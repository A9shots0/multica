package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
)

const RoleSquadReadinessMatrixV3Schema = "silver-mediums.role-squad-readiness-matrix.v3"

type RoleSquadReadinessSourceHashesV3 struct {
	PredecessorReadinessV2              string `json:"predecessor_readiness_v2"`
	ProvisionalConfigurationV2          string `json:"provisional_configuration_v2"`
	ProvisionalConfigurationLifecycleV2 string `json:"provisional_configuration_lifecycle_v2"`
	ProvisionalRehearsalV2              string `json:"provisional_rehearsal_v2"`
	IntegrationReplayResult             string `json:"integration_replay_result"`
	RoleSquadOperatingContract          string `json:"role_squad_operating_contract"`
}

type TaskScopedFunctionReadinessV3 struct {
	FunctionID string `json:"function_id"`
	Binding    string `json:"binding"`
	Status     string `json:"status"`
}

type RoleSquadReadinessMatrixV3 struct {
	SchemaVersion                       string                           `json:"schema_version"`
	MatrixID                            string                           `json:"matrix_id"`
	Status                              string                           `json:"status"`
	ValidUntil                          string                           `json:"valid_until"`
	SourceHashes                        RoleSquadReadinessSourceHashesV3 `json:"source_hashes"`
	RolePackets                         []RolePacketReadinessV2          `json:"role_packets"`
	TaskScopedFunctions                 []TaskScopedFunctionReadinessV3  `json:"task_scoped_functions"`
	Squads                              []SquadReadinessV2               `json:"squads"`
	ControllerOnlyOutcomeIdentities     int                              `json:"controller_only_outcome_identities"`
	StructuralInstructionSkillPairs     int                              `json:"structural_instruction_skill_pairs"`
	ProvisionalLocalOperatingDefaults   int                              `json:"provisional_local_operating_defaults"`
	ProvisionalRuntimeRolePackets       int                              `json:"provisional_runtime_role_packets"`
	ProvisionalTaskScopedFunctions      int                              `json:"provisional_task_scoped_functions"`
	ActivatedPermanentSquads            int                              `json:"activated_permanent_squads"`
	TransportDefault                    string                           `json:"transport_default"`
	RequestedPrimaryModelDefault        string                           `json:"requested_primary_model_default"`
	ExactSingleModelQualification       bool                             `json:"exact_single_model_qualification"`
	PermanentModelSelection             string                           `json:"permanent_model_selection"`
	PermanentRoleSelection              string                           `json:"permanent_role_selection"`
	AutomaticDispatch                   bool                             `json:"automatic_dispatch"`
	ConfigurationAuthorizesProviderCall bool                             `json:"configuration_authorizes_provider_call"`
	ProductionActivation                bool                             `json:"production_activation"`
	ProviderCalls                       int                              `json:"provider_calls"`
	CredentialReads                     int                              `json:"credential_reads"`
	NativeToolExecutions                int                              `json:"native_tool_executions"`
	ProviderChildRepositoryAccesses     int                              `json:"provider_child_repository_accesses"`
	ProductionAccesses                  int                              `json:"production_accesses"`
	OutboundActions                     int                              `json:"outbound_actions"`
	PhoneFarmWindowsAccesses            int                              `json:"phone_farm_windows_accesses"`
}

//go:embed role_squad_readiness_matrix_v3.json
var roleSquadReadinessMatrixV3JSON []byte

func (ctxContracts QualificationContracts) CurrentRoleSquadReadinessMatrixV3() (RoleSquadReadinessMatrixV3, error) {
	var matrix RoleSquadReadinessMatrixV3
	if err := strictJSON(roleSquadReadinessMatrixV3JSON, &matrix); err != nil {
		return matrix, fmt.Errorf("decode role squad readiness matrix v3: %w", err)
	}
	return matrix, ctxContracts.ValidateRoleSquadReadinessMatrixV3(matrix)
}

func RoleSquadReadinessMatrixV3SHA256() string {
	sum := sha256.Sum256(roleSquadReadinessMatrixV3JSON)
	return hex.EncodeToString(sum[:])
}

func (ctxContracts QualificationContracts) ValidateRoleSquadReadinessMatrixV3(matrix RoleSquadReadinessMatrixV3) error {
	if matrix.SchemaVersion != RoleSquadReadinessMatrixV3Schema || matrix.MatrixID != "item-14-role-squad-readiness-after-cef7-20260915" || matrix.Status != "accepted_reversible_local_default_with_structural_roles_held" || matrix.ValidUntil != "2026-09-22T13:15:00Z" {
		return fmt.Errorf("role squad readiness matrix v3 identity drifted")
	}
	wantSources := RoleSquadReadinessSourceHashesV3{PredecessorReadinessV2: RoleSquadReadinessMatrixV2SHA256(), ProvisionalConfigurationV2: ProvisionalRoleConfigurationV2SHA256(), ProvisionalConfigurationLifecycleV2: ProvisionalRoleConfigurationLifecycleV2SHA256(), ProvisionalRehearsalV2: ProvisionalRoleConfigurationRehearsalV2SHA256(), IntegrationReplayResult: IntegrationReplayResultV1SHA256(), RoleSquadOperatingContract: RoleSquadOperatingContractV1SHA256()}
	if matrix.SourceHashes != wantSources {
		return fmt.Errorf("role squad readiness matrix v3 source binding drifted")
	}
	predecessor, err := ctxContracts.CurrentRoleSquadReadinessMatrixV2()
	if err != nil {
		return err
	}
	if len(matrix.RolePackets) != len(predecessor.RolePackets) {
		return fmt.Errorf("role squad readiness matrix v3 role coverage drifted")
	}
	wantRuntime := map[string]string{
		"architecture_assurance":           "provisional_inline_skill_exact_local_r01_like_scope_only",
		"company_auditor":                  "structural_only_no_runtime_reliability_evidence",
		"data_sync_integrity":              "structural_only_cef7_guard_label_is_not_role_qualification",
		"mika_owner_interface":             "structural_only_no_runtime_reliability_evidence",
		"phone_fleet_reliability":          "structural_only_external_mac_authority_required",
		"phonefarm_application_specialist": "structural_only_external_mac_authority_required",
		"product_ui":                       "held_unqualified_r02_zero_of_six",
		"quality_checker":                  "structural_only_cef7_temporary_reviewer_is_not_quality_checker_role_qualification",
		"release_runtime_assurance":        "structural_only_no_runtime_reliability_evidence",
		"security_privacy_review":          "structural_only_no_runtime_reliability_evidence",
		"shared_windows_executor":          "provisional_manual_guarded_local_r01_like_with_inline_architecture_skill",
	}
	for index, role := range matrix.RolePackets {
		expected := predecessor.RolePackets[index]
		if role.RoleID != expected.RoleID || role.ArtifactBindingID != expected.ArtifactBindingID || role.StructuralStatus != expected.StructuralStatus || role.RuntimeStatus != wantRuntime[role.RoleID] {
			return fmt.Errorf("role squad readiness matrix v3 role drifted for %s", role.RoleID)
		}
	}
	wantFunctions := []TaskScopedFunctionReadinessV3{{FunctionID: "intake_leader", Binding: "task_scoped_no_permanent_seat", Status: "provisional_exact_local_r01_like_route_only"}, {FunctionID: "independent_reviewer", Binding: "cef7_exact_temporary_reviewer_contract_not_permanent_quality_checker_qualification", Status: "provisional_exact_local_r01_like_route_only"}}
	if !slices.Equal(matrix.TaskScopedFunctions, wantFunctions) || !slices.Equal(matrix.Squads, predecessor.Squads) || matrix.ControllerOnlyOutcomeIdentities != predecessor.ControllerOnlyOutcomeIdentities || matrix.StructuralInstructionSkillPairs != predecessor.StructuralInstructionSkillPairs || matrix.ProvisionalLocalOperatingDefaults != 1 || matrix.ProvisionalRuntimeRolePackets != 2 || matrix.ProvisionalTaskScopedFunctions != 2 || matrix.ActivatedPermanentSquads != 0 {
		return fmt.Errorf("role squad readiness matrix v3 coverage drifted")
	}
	if matrix.TransportDefault != "claude_code_subscription_composite" || matrix.RequestedPrimaryModelDefault != "claude-sonnet-4-6" || matrix.ExactSingleModelQualification || matrix.PermanentModelSelection != "none" || matrix.PermanentRoleSelection != "none" || matrix.AutomaticDispatch || matrix.ConfigurationAuthorizesProviderCall || matrix.ProductionActivation || matrix.ProviderCalls != 0 || matrix.CredentialReads != 0 || matrix.NativeToolExecutions != 0 || matrix.ProviderChildRepositoryAccesses != 0 || matrix.ProductionAccesses != 0 || matrix.OutboundActions != 0 || matrix.PhoneFarmWindowsAccesses != 0 {
		return fmt.Errorf("role squad readiness matrix v3 expanded authority")
	}
	return nil
}
