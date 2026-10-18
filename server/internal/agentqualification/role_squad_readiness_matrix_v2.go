package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
)

const RoleSquadReadinessMatrixV2Schema = "silver-mediums.role-squad-readiness-matrix.v2"

type RoleSquadReadinessSourceHashesV2 struct {
	ReliabilityTrialResult   string `json:"reliability_trial_result"`
	ProvisionalConfiguration string `json:"provisional_configuration"`
	ProvisionalRehearsal     string `json:"provisional_rehearsal"`
	RoleStageContract        string `json:"role_stage_contract"`
	RoleStageRehearsal       string `json:"role_stage_rehearsal"`
}

type RolePacketReadinessV2 struct {
	RoleID            string `json:"role_id"`
	ArtifactBindingID string `json:"artifact_binding_id"`
	StructuralStatus  string `json:"structural_status"`
	RuntimeStatus     string `json:"runtime_status"`
}

type SquadReadinessV2 struct {
	SquadID    string `json:"squad_id"`
	RouteCount int    `json:"route_count"`
	Status     string `json:"status"`
}

type RoleSquadReadinessMatrixV2 struct {
	SchemaVersion                   string                           `json:"schema_version"`
	MatrixID                        string                           `json:"matrix_id"`
	Status                          string                           `json:"status"`
	SourceHashes                    RoleSquadReadinessSourceHashesV2 `json:"source_hashes"`
	RolePackets                     []RolePacketReadinessV2          `json:"role_packets"`
	Squads                          []SquadReadinessV2               `json:"squads"`
	ControllerOnlyOutcomeIdentities int                              `json:"controller_only_outcome_identities"`
	StructuralInstructionSkillPairs int                              `json:"structural_instruction_skill_pairs"`
	SkillOnlyPacketsStructural      int                              `json:"skill_only_packets_structural"`
	ProvisionalRuntimeRolePackets   int                              `json:"provisional_runtime_role_packets"`
	ProvisionalInlineSkills         int                              `json:"provisional_inline_skills"`
	ActivatedSquads                 int                              `json:"activated_squads"`
	ProviderLane                    string                           `json:"provider_lane"`
	ModelSelection                  string                           `json:"model_selection"`
	PermanentRoleSelection          string                           `json:"permanent_role_selection"`
	AutomaticDispatch               bool                             `json:"automatic_dispatch"`
	ProductionActivation            bool                             `json:"production_activation"`
	ProviderCalls                   int                              `json:"provider_calls"`
	CredentialReads                 int                              `json:"credential_reads"`
	NativeToolExecutions            int                              `json:"native_tool_executions"`
	RepositoryAccesses              int                              `json:"repository_accesses"`
	ProductionAccesses              int                              `json:"production_accesses"`
	OutboundActions                 int                              `json:"outbound_actions"`
	PhoneFarmWindowsAccesses        int                              `json:"phone_farm_windows_accesses"`
}

//go:embed role_squad_readiness_matrix_v2.json
var roleSquadReadinessMatrixV2JSON []byte

func (ctxContracts QualificationContracts) CurrentRoleSquadReadinessMatrixV2() (RoleSquadReadinessMatrixV2, error) {
	var matrix RoleSquadReadinessMatrixV2
	if err := strictJSON(roleSquadReadinessMatrixV2JSON, &matrix); err != nil {
		return matrix, fmt.Errorf("decode role squad readiness matrix v2: %w", err)
	}
	return matrix, ctxContracts.ValidateRoleSquadReadinessMatrixV2(matrix)
}

func RoleSquadReadinessMatrixV2SHA256() string {
	sum := sha256.Sum256(roleSquadReadinessMatrixV2JSON)
	return hex.EncodeToString(sum[:])
}

func (ctxContracts QualificationContracts) ValidateRoleSquadReadinessMatrixV2(matrix RoleSquadReadinessMatrixV2) error {
	if matrix.SchemaVersion != RoleSquadReadinessMatrixV2Schema || matrix.MatrixID != "item-14-role-squad-readiness-after-reliability-v2-20260914" || matrix.Status != "accepted_provider_free_structural_readiness_with_one_provisional_scope" {
		return fmt.Errorf("role squad readiness matrix v2 identity drifted")
	}
	wantSources := RoleSquadReadinessSourceHashesV2{ReliabilityTrialResult: PrimarySubscriptionReliabilityTrialResultV2SHA256(), ProvisionalConfiguration: ProvisionalRoleConfigurationV1SHA256(), ProvisionalRehearsal: ProvisionalRoleConfigurationRehearsalV1SHA256(), RoleStageContract: RoleStageEvidenceContractV1SHA256(), RoleStageRehearsal: hashEmbedded(roleStageEvidenceRehearsalV1ResultJSON)}
	if matrix.SourceHashes != wantSources {
		return fmt.Errorf("role squad readiness matrix v2 source binding drifted")
	}
	contract, err := ctxContracts.CurrentRoleStageEvidenceContractV1()
	if err != nil {
		return err
	}
	if len(matrix.RolePackets) != len(contract.RoleContracts) {
		return fmt.Errorf("role squad readiness matrix v2 role coverage drifted")
	}
	wantRuntime := map[string]string{
		"architecture_assurance":           "provisional_inline_skill_r01_only_not_separate_role",
		"company_auditor":                  "structural_only_no_runtime_reliability_evidence",
		"data_sync_integrity":              "structural_only_no_runtime_reliability_evidence",
		"mika_owner_interface":             "structural_only_no_runtime_reliability_evidence",
		"phone_fleet_reliability":          "structural_only_external_mac_authority_required",
		"phonefarm_application_specialist": "structural_only_external_mac_authority_required",
		"product_ui":                       "held_unqualified_r02_zero_of_six",
		"quality_checker":                  "structural_only_distinct_verdict_contract",
		"release_runtime_assurance":        "structural_only_no_runtime_reliability_evidence",
		"security_privacy_review":          "structural_only_no_runtime_reliability_evidence",
		"shared_windows_executor":          "provisional_r01_only_with_inline_architecture_skill",
	}
	for index, role := range matrix.RolePackets {
		expected := contract.RoleContracts[index]
		if role.RoleID != expected.RoleID || role.ArtifactBindingID != expected.ArtifactBindingID || role.StructuralStatus != "qualified_provider_free_contract_and_rehearsal" || role.RuntimeStatus != wantRuntime[role.RoleID] {
			return fmt.Errorf("role squad readiness matrix v2 role drifted for %s", role.RoleID)
		}
	}
	wantSquads := []SquadReadinessV2{{SquadID: "cross_boundary_change", RouteCount: 16, Status: "structural_provider_free_rehearsal_passed_activation_held"}, {SquadID: "independent_audit", RouteCount: 1, Status: "structural_provider_free_rehearsal_passed_activation_held"}, {SquadID: "standard_outcome_delivery", RouteCount: 3, Status: "structural_provider_free_rehearsal_passed_activation_held"}}
	if !slices.Equal(matrix.Squads, wantSquads) || matrix.ControllerOnlyOutcomeIdentities != 10 || matrix.StructuralInstructionSkillPairs != 11 || matrix.SkillOnlyPacketsStructural != 1 || matrix.ProvisionalRuntimeRolePackets != 1 || matrix.ProvisionalInlineSkills != 1 || matrix.ActivatedSquads != 0 {
		return fmt.Errorf("role squad readiness matrix v2 coverage drifted")
	}
	if matrix.ProviderLane != "unselected" || matrix.ModelSelection != "none" || matrix.PermanentRoleSelection != "none" || matrix.AutomaticDispatch || matrix.ProductionActivation || matrix.ProviderCalls != 0 || matrix.CredentialReads != 0 || matrix.NativeToolExecutions != 0 || matrix.RepositoryAccesses != 0 || matrix.ProductionAccesses != 0 || matrix.OutboundActions != 0 || matrix.PhoneFarmWindowsAccesses != 0 {
		return fmt.Errorf("role squad readiness matrix v2 expanded authority")
	}
	return nil
}
