package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

const RoleStageEvidenceContractV1Schema = "silver-mediums.role-stage-evidence-contract.v1"

type RoleStageEvidenceSourceHashesV1 struct {
	OperatingContract              string `json:"operating_contract"`
	RoleSquadRehearsalResult       string `json:"role_squad_rehearsal_result"`
	PrimarySubscriptionTrialResult string `json:"primary_subscription_trial_result"`
}

type RoleStageResponseContractV1 struct {
	RoleID            string   `json:"role_id"`
	ArtifactBindingID string   `json:"artifact_binding_id"`
	ResponseClass     string   `json:"response_class"`
	TopLevelFields    []string `json:"top_level_fields"`
}

type RoleStageControllerActionProtocolV1 struct {
	AppliesToRoleID                    string   `json:"applies_to_role_id"`
	MaximumActions                     int      `json:"maximum_actions"`
	AllowedActionKinds                 []string `json:"allowed_action_kinds"`
	RequiredActionFields               []string `json:"required_action_fields"`
	EditContentsSemantics              string   `json:"edit_contents_semantics"`
	CheckSemantics                     string   `json:"check_semantics"`
	BrowserSemantics                   string   `json:"browser_semantics"`
	FinishSemantics                    string   `json:"finish_semantics"`
	ProviderNativeToolsAllowed         bool     `json:"provider_native_tools_allowed"`
	ParticipantRepositoryAccessAllowed bool     `json:"participant_repository_access_allowed"`
	AutomaticRepairOrReplayAllowed     bool     `json:"automatic_repair_or_replay_allowed"`
}

type RoleStageTrialFindingsV1 struct {
	SeparateProductUISpecialistSessionsComplete int    `json:"separate_product_ui_specialist_sessions_completed"`
	SeparateProductUIObservationsPassed         int    `json:"separate_product_ui_observations_passed"`
	DownstreamExecutorContractFailures          int    `json:"downstream_executor_contract_failures"`
	InlineObservationsPassed                    int    `json:"inline_observations_passed"`
	SeparateObservationsPassed                  int    `json:"separate_observations_passed"`
	ConfigurationSelection                      string `json:"configuration_selection"`
	PermanentSeatSelection                      string `json:"permanent_seat_selection"`
}

type RoleStageEvidenceContractV1 struct {
	SchemaVersion                                    string                              `json:"schema_version"`
	ContractID                                       string                              `json:"contract_id"`
	Status                                           string                              `json:"status"`
	SourceHashes                                     RoleStageEvidenceSourceHashesV1     `json:"source_hashes"`
	ProviderCallsAllowed                             bool                                `json:"provider_calls_allowed"`
	CredentialReadsAllowed                           bool                                `json:"credential_reads_allowed"`
	RepositoryAccessAllowed                          bool                                `json:"repository_access_allowed"`
	ProductionActionsAllowed                         bool                                `json:"production_actions_allowed"`
	PhoneFarmWindowsAccessAllowed                    bool                                `json:"phone_farm_windows_access_allowed"`
	ModelSelectionAllowed                            bool                                `json:"model_selection_allowed"`
	PermanentSeatSelectionAllowed                    bool                                `json:"permanent_seat_selection_allowed"`
	DirectRoleToRoleHandoffAllowed                   bool                                `json:"direct_role_to_role_handoff_allowed"`
	ControllerReissuesCompletePacket                 bool                                `json:"controller_reissues_complete_packet"`
	SpecialistEvidenceIsAdvisory                     bool                                `json:"specialist_evidence_is_advisory"`
	SpecialistEvidenceCanExpandAuthority             bool                                `json:"specialist_evidence_can_expand_authority"`
	DownstreamExecutorMustReturnSelfContainedPayload bool                                `json:"downstream_executor_must_return_self_contained_payload"`
	FailedStageIsTerminalWithoutAutomaticReplay      bool                                `json:"failed_stage_is_terminal_without_automatic_replay"`
	RouteCount                                       int                                 `json:"route_count"`
	ParticipantStageCount                            int                                 `json:"participant_stage_count"`
	SharedSkillAttachmentCount                       int                                 `json:"shared_skill_attachment_count"`
	RoleContracts                                    []RoleStageResponseContractV1       `json:"role_contracts"`
	ControllerActionProtocol                         RoleStageControllerActionProtocolV1 `json:"controller_action_protocol"`
	TrialFindings                                    RoleStageTrialFindingsV1            `json:"trial_findings"`
}

//go:embed role_stage_evidence_contract_v1.json
var roleStageEvidenceContractV1JSON []byte

func (ctxContracts QualificationContracts) CurrentRoleStageEvidenceContractV1() (RoleStageEvidenceContractV1, error) {
	var contract RoleStageEvidenceContractV1
	if err := strictJSON(roleStageEvidenceContractV1JSON, &contract); err != nil {
		return contract, fmt.Errorf("decode role stage evidence contract: %w", err)
	}
	return contract, ctxContracts.ValidateRoleStageEvidenceContractV1(contract)
}

func RoleStageEvidenceContractV1SHA256() string {
	sum := sha256.Sum256(roleStageEvidenceContractV1JSON)
	return hex.EncodeToString(sum[:])
}

func (ctxContracts QualificationContracts) ValidateRoleStageEvidenceContractV1(contract RoleStageEvidenceContractV1) error {
	if contract.SchemaVersion != RoleStageEvidenceContractV1Schema || contract.ContractID != "item-14-role-stage-evidence-contract-20260914" || contract.Status != "accepted_provider_free_candidate_contract" {
		return fmt.Errorf("role stage evidence contract identity drifted")
	}
	if contract.ProviderCallsAllowed || contract.CredentialReadsAllowed || contract.RepositoryAccessAllowed || contract.ProductionActionsAllowed || contract.PhoneFarmWindowsAccessAllowed || contract.ModelSelectionAllowed || contract.PermanentSeatSelectionAllowed || contract.DirectRoleToRoleHandoffAllowed || contract.SpecialistEvidenceCanExpandAuthority {
		return fmt.Errorf("role stage evidence contract expanded authority")
	}
	if !contract.ControllerReissuesCompletePacket || !contract.SpecialistEvidenceIsAdvisory || !contract.DownstreamExecutorMustReturnSelfContainedPayload || !contract.FailedStageIsTerminalWithoutAutomaticReplay {
		return fmt.Errorf("role stage handoff guarantee drifted")
	}
	contractHash := RoleSquadOperatingContractV1SHA256()
	rehearsalHash := sha256.Sum256(roleSquadRehearsalResultV1JSON)
	if contract.SourceHashes.OperatingContract != contractHash || contract.SourceHashes.RoleSquadRehearsalResult != hex.EncodeToString(rehearsalHash[:]) || contract.SourceHashes.PrimarySubscriptionTrialResult != PrimarySubscriptionRoleTrialResultV1SHA256() {
		return fmt.Errorf("role stage evidence source hash drifted")
	}
	rehearsal, err := ctxContracts.CurrentRoleSquadRehearsalResultV1()
	if err != nil {
		return err
	}
	if contract.RouteCount != rehearsal.CaseRouteCount+rehearsal.AdditionalRouteCount || contract.ParticipantStageCount != rehearsal.ParticipantStageCount || contract.SharedSkillAttachmentCount != rehearsal.SharedSkillAttachmentCount {
		return fmt.Errorf("role stage route coverage drifted")
	}
	wantClasses := map[string]string{
		"architecture_assurance":           "conditional_specialist_evidence",
		"company_auditor":                  "independent_audit_evidence",
		"data_sync_integrity":              "conditional_specialist_evidence",
		"mika_owner_interface":             "owner_interface_response",
		"phone_fleet_reliability":          "external_mac_sanitized_evidence",
		"phonefarm_application_specialist": "external_mac_sanitized_evidence",
		"product_ui":                       "conditional_specialist_evidence",
		"quality_checker":                  "independent_outcome_verdict",
		"release_runtime_assurance":        "release_runtime_evidence",
		"security_privacy_review":          "conditional_specialist_evidence",
		"shared_windows_executor":          "primary_executor_evidence",
	}
	if len(contract.RoleContracts) != len(wantClasses) {
		return fmt.Errorf("role stage evidence role coverage incomplete")
	}
	seen := map[string]struct{}{}
	lastRole := ""
	for _, roleContract := range contract.RoleContracts {
		if roleContract.RoleID <= lastRole || roleContract.ResponseClass != wantClasses[roleContract.RoleID] || !sort.StringsAreSorted(roleContract.TopLevelFields) {
			return fmt.Errorf("role stage response class or canonical order drifted for %s", roleContract.RoleID)
		}
		lastRole = roleContract.RoleID
		if _, duplicate := seen[roleContract.RoleID]; duplicate {
			return fmt.Errorf("duplicate role stage evidence contract for %s", roleContract.RoleID)
		}
		seen[roleContract.RoleID] = struct{}{}
		ref, err := roleSquadArtifactRefV1(roleContract.RoleID)
		if err != nil || roleContract.ArtifactBindingID != ref.BindingID {
			return fmt.Errorf("role stage artifact binding drifted for %s", roleContract.RoleID)
		}
		instructions, skill, err := roleStageArtifactBytesV1(ref)
		if err != nil {
			return err
		}
		fields, err := roleStageInstructionFieldsV1(instructions)
		if err != nil || !slices.Equal(fields, roleContract.TopLevelFields) {
			return fmt.Errorf("role stage response fields drifted for %s", roleContract.RoleID)
		}
		skillLower := strings.ToLower(string(skill))
		if !strings.Contains(skillLower, "json") || !strings.Contains(skillLower, "emit only") {
			return fmt.Errorf("role stage skill lost its exact response boundary for %s", roleContract.RoleID)
		}
	}
	for roleID := range wantClasses {
		if _, ok := seen[roleID]; !ok {
			return fmt.Errorf("role stage evidence contract missing %s", roleID)
		}
	}
	if err := validateRoleStageActionProtocolV1(contract.ControllerActionProtocol); err != nil {
		return err
	}
	result, err := CurrentPrimarySubscriptionRoleTrialResultV1()
	if err != nil {
		return err
	}
	wantFindings := RoleStageTrialFindingsV1{
		SeparateProductUISpecialistSessionsComplete: result.SeparateProductUISpecialistSessionsComplete,
		SeparateProductUIObservationsPassed:         result.SeparateProductUIObservationsPassed,
		DownstreamExecutorContractFailures:          result.DownstreamExecutorContractFailures,
		InlineObservationsPassed:                    result.InlinePasses, SeparateObservationsPassed: result.SeparatePasses,
		ConfigurationSelection: result.ConfigurationSelection, PermanentSeatSelection: result.PermanentSeatSelection,
	}
	if contract.TrialFindings != wantFindings {
		return fmt.Errorf("role stage trial finding drifted")
	}
	return nil
}

func validateRoleStageActionProtocolV1(protocol RoleStageControllerActionProtocolV1) error {
	if protocol.AppliesToRoleID != "shared_windows_executor" || protocol.MaximumActions != 10 ||
		!slices.Equal(protocol.AllowedActionKinds, []string{"read", "search", "edit", "run_check", "browser", "finish"}) ||
		!slices.Equal(protocol.RequiredActionFields, []string{"check", "contents", "kind", "path", "query", "summary"}) ||
		protocol.EditContentsSemantics != "complete_literal_file_contents_not_instructions_diff_or_placeholder" ||
		protocol.CheckSemantics != "exact_controller_allowlist_member" ||
		protocol.BrowserSemantics != "controller_owned_local_loopback_only_when_case_requires" ||
		protocol.FinishSemantics != "exactly_once_final_action_with_meaningful_summary" ||
		protocol.ProviderNativeToolsAllowed || protocol.ParticipantRepositoryAccessAllowed || protocol.AutomaticRepairOrReplayAllowed {
		return fmt.Errorf("role stage controller action protocol drifted")
	}
	return nil
}

func roleStageArtifactBytesV1(ref ShadowRoleArtifactRefV1) ([]byte, []byte, error) {
	if ref.ArtifactVersion == "v2" {
		instructions, err := roleArtifactSetV2FS.ReadFile(ref.InstructionPath)
		if err != nil {
			return nil, nil, err
		}
		skill, err := roleArtifactSetV2FS.ReadFile(ref.SkillPath)
		return instructions, skill, err
	}
	values := map[string]struct{ instructions, skill []byte }{
		"mika_owner_interface":    {roleQualificationFreezeMikaInstructionsV1, roleQualificationFreezeMikaSkillV1},
		"product_ui":              {roleQualificationFreezeProductInstructionsV1, roleQualificationFreezeProductSkillV1},
		"quality_checker":         {roleQualificationFreezeQualityInstructionsV1, roleQualificationFreezeQualitySkillV1},
		"shared_windows_executor": {roleQualificationFreezeWindowsInstructionsV1, roleQualificationFreezeWindowsSkillV1},
	}
	value, ok := values[ref.RoleID]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported v1 role stage artifact %s", ref.RoleID)
	}
	return value.instructions, value.skill, nil
}

func roleStageInstructionFieldsV1(instructions []byte) ([]string, error) {
	text := string(instructions)
	start := strings.Index(text, "```json")
	if start < 0 {
		return nil, fmt.Errorf("role instruction JSON example missing")
	}
	start += len("```json")
	end := strings.Index(text[start:], "```")
	if end < 0 {
		return nil, fmt.Errorf("role instruction JSON example unterminated")
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(text[start:start+end])), &object); err != nil {
		return nil, fmt.Errorf("role instruction JSON example invalid: %w", err)
	}
	fields := make([]string, 0, len(object))
	for field := range object {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields, nil
}
