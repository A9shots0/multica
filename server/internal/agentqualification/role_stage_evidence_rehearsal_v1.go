package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const RoleStageEvidenceRehearsalV1Schema = "silver-mediums.role-stage-evidence-rehearsal.v1"

//go:embed role_stage_evidence_rehearsal_v1_result.json
var roleStageEvidenceRehearsalV1ResultJSON []byte

type RoleStageEvidenceRehearsalReceiptV1 struct {
	Source                      string `json:"source"`
	RouteID                     string `json:"route_id"`
	Sequence                    int    `json:"sequence"`
	Mode                        string `json:"mode"`
	RoleID                      string `json:"role_id"`
	ArtifactBindingID           string `json:"artifact_binding_id"`
	ResponseClass               string `json:"response_class"`
	ResponseFieldSetSHA256      string `json:"response_field_set_sha256,omitempty"`
	ControllerHandoff           bool   `json:"controller_handoff"`
	CompletePacketReissued      bool   `json:"complete_packet_reissued"`
	PriorEvidenceAdvisoryOnly   bool   `json:"prior_evidence_advisory_only"`
	SelfContainedActionRequired bool   `json:"self_contained_action_required"`
	SharedSkillAttachment       bool   `json:"shared_skill_attachment"`
	ControllerOnlyBinding       bool   `json:"controller_only_binding"`
	SideEffects                 bool   `json:"side_effects"`
}

type RoleStageEvidenceRehearsalResultV1 struct {
	SchemaVersion                  string `json:"schema_version"`
	RunID                          string `json:"run_id"`
	ExecutionMode                  string `json:"execution_mode"`
	ContractSHA256                 string `json:"contract_sha256"`
	AllRouteTraceSHA256            string `json:"all_route_trace_sha256"`
	LocalShadowSuiteSHA256         string `json:"local_shadow_suite_sha256"`
	CombinedTraceSHA256            string `json:"combined_trace_sha256"`
	AllRouteReceipts               int    `json:"all_route_receipts"`
	LocalShadowReceipts            int    `json:"local_shadow_receipts"`
	ResponseContractBindings       int    `json:"response_contract_bindings"`
	ControllerOnlyBindings         int    `json:"controller_only_bindings"`
	ControllerHandoffs             int    `json:"controller_handoffs"`
	CompletePacketReissues         int    `json:"complete_packet_reissues"`
	SharedSkillAttachments         int    `json:"shared_skill_attachments"`
	SelfContainedActionObligations int    `json:"self_contained_action_obligations"`
	UniqueResponseFields           int    `json:"unique_response_fields"`
	TrialHandoffFailuresPreserved  int    `json:"trial_handoff_failures_preserved"`
	ProviderRequests               int    `json:"provider_requests"`
	CredentialReads                int    `json:"credential_reads"`
	ToolCalls                      int    `json:"tool_calls"`
	RepositoryAccesses             int    `json:"repository_accesses"`
	ProductionActions              int    `json:"production_actions"`
	OutboundActions                int    `json:"outbound_actions"`
	PhoneFarmAccesses              int    `json:"phone_farm_accesses"`
	ModelOrSeatSelections          int    `json:"model_or_seat_selections"`
	Valid                          bool   `json:"valid"`
}

func (ctxContracts QualificationContracts) CurrentRoleStageEvidenceRehearsalV1() (RoleStageEvidenceRehearsalResultV1, error) {
	var recorded RoleStageEvidenceRehearsalResultV1
	if err := strictJSON(roleStageEvidenceRehearsalV1ResultJSON, &recorded); err != nil {
		return recorded, fmt.Errorf("decode role stage evidence rehearsal result: %w", err)
	}
	expected, receipts, err := ctxContracts.RunRoleStageEvidenceRehearsalV1()
	if err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, err
	}
	if recorded != expected {
		return RoleStageEvidenceRehearsalResultV1{}, fmt.Errorf("recorded role stage evidence rehearsal drifted")
	}
	if err := ctxContracts.ValidateRoleStageEvidenceRehearsalV1(recorded, receipts); err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, err
	}
	return recorded, nil
}

func (ctxContracts QualificationContracts) RunRoleStageEvidenceRehearsalV1() (RoleStageEvidenceRehearsalResultV1, []RoleStageEvidenceRehearsalReceiptV1, error) {
	contract, err := ctxContracts.CurrentRoleStageEvidenceContractV1()
	if err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, nil, err
	}
	roleContracts := make(map[string]RoleStageResponseContractV1, len(contract.RoleContracts))
	uniqueFields := map[string]struct{}{}
	for _, roleContract := range contract.RoleContracts {
		roleContracts[roleContract.RoleID] = roleContract
		for _, field := range roleContract.TopLevelFields {
			uniqueFields[roleContract.RoleID+"."+field] = struct{}{}
		}
	}
	allRoute, allReceipts, err := ctxContracts.RunRoleSquadRehearsalV1()
	if err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, nil, err
	}
	var receipts []RoleStageEvidenceRehearsalReceiptV1
	responseBindings, controllerOnly, handoffs, packetReissues, sharedSkills, actionObligations := 0, 0, 0, 0, 0, 0
	for _, source := range allReceipts {
		roleContract, ok := roleContracts[source.RoleID]
		if !ok {
			return RoleStageEvidenceRehearsalResultV1{}, nil, fmt.Errorf("all-route role %s lacks a response contract", source.RoleID)
		}
		shared := source.Mode == "shared_skill_attachment"
		receipt := roleStageEvidenceReceiptV1("all_route", source.RouteID, source.StageSequence, source.Mode, source.RoleID, source.ArtifactBindingID, roleContract, !shared, shared)
		receipts = append(receipts, receipt)
		responseBindings++
		if shared {
			sharedSkills++
		} else {
			handoffs++
			packetReissues++
		}
		if receipt.SelfContainedActionRequired {
			actionObligations++
		}
	}
	suite, err := ctxContracts.CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, nil, err
	}
	localRun, err := ctxContracts.RunLocalShadowWorkflowSuiteFakeV1(suite)
	if err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, nil, err
	}
	for _, source := range localRun.Receipts {
		if strings.HasPrefix(source.ArtifactBindingID, "silver-mediums.role-catalog.v1.") {
			receipts = append(receipts, RoleStageEvidenceRehearsalReceiptV1{Source: "local_shadow", RouteID: source.WorkflowID, Sequence: source.Sequence, Mode: source.Stage, RoleID: source.RoleID, ArtifactBindingID: source.ArtifactBindingID, ResponseClass: "controller_only_outcome_binding", ControllerOnlyBinding: true})
			controllerOnly++
			continue
		}
		roleContract, ok := roleContracts[source.RoleID]
		if !ok || roleContract.ArtifactBindingID != source.ArtifactBindingID {
			return RoleStageEvidenceRehearsalResultV1{}, nil, fmt.Errorf("local shadow role %s lacks an exact response contract", source.RoleID)
		}
		receipt := roleStageEvidenceReceiptV1("local_shadow", source.WorkflowID, source.Sequence, source.Stage, source.RoleID, source.ArtifactBindingID, roleContract, true, false)
		receipts = append(receipts, receipt)
		responseBindings++
		handoffs++
		packetReissues++
		if receipt.SelfContainedActionRequired {
			actionObligations++
		}
	}
	trace, err := json.Marshal(receipts)
	if err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, nil, err
	}
	traceSum := sha256.Sum256(trace)
	result := RoleStageEvidenceRehearsalResultV1{
		SchemaVersion: RoleStageEvidenceRehearsalV1Schema, RunID: "item-14-role-stage-evidence-rehearsal-20260914", ExecutionMode: "deterministic_provider_free_integrated_rehearsal",
		ContractSHA256: RoleStageEvidenceContractV1SHA256(), AllRouteTraceSHA256: allRoute.TraceSHA256, LocalShadowSuiteSHA256: ctxContracts.LocalShadowWorkflowSuiteV1SHA256(), CombinedTraceSHA256: hex.EncodeToString(traceSum[:]),
		AllRouteReceipts: len(allReceipts), LocalShadowReceipts: len(localRun.Receipts), ResponseContractBindings: responseBindings, ControllerOnlyBindings: controllerOnly,
		ControllerHandoffs: handoffs, CompletePacketReissues: packetReissues, SharedSkillAttachments: sharedSkills, SelfContainedActionObligations: actionObligations,
		UniqueResponseFields: len(uniqueFields), TrialHandoffFailuresPreserved: contract.TrialFindings.DownstreamExecutorContractFailures, Valid: true,
	}
	if err := ctxContracts.ValidateRoleStageEvidenceRehearsalV1(result, receipts); err != nil {
		return RoleStageEvidenceRehearsalResultV1{}, nil, err
	}
	return result, receipts, nil
}

func (ctxContracts QualificationContracts) ValidateRoleStageEvidenceRehearsalV1(result RoleStageEvidenceRehearsalResultV1, receipts []RoleStageEvidenceRehearsalReceiptV1) error {
	contract, err := ctxContracts.CurrentRoleStageEvidenceContractV1()
	if err != nil {
		return err
	}
	allRoute, _, err := ctxContracts.RunRoleSquadRehearsalV1()
	if err != nil {
		return err
	}
	if result.SchemaVersion != RoleStageEvidenceRehearsalV1Schema || result.RunID != "item-14-role-stage-evidence-rehearsal-20260914" || result.ExecutionMode != "deterministic_provider_free_integrated_rehearsal" || result.ContractSHA256 != RoleStageEvidenceContractV1SHA256() || result.AllRouteTraceSHA256 != allRoute.TraceSHA256 || result.LocalShadowSuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() || result.AllRouteReceipts != 69 || result.LocalShadowReceipts != 22 || result.ResponseContractBindings != 89 || result.ControllerOnlyBindings != 2 || result.ControllerHandoffs != 79 || result.CompletePacketReissues != 79 || result.SharedSkillAttachments != 10 || result.SelfContainedActionObligations != 20 || result.UniqueResponseFields != 112 || result.TrialHandoffFailuresPreserved != 2 || !result.Valid {
		return fmt.Errorf("role stage evidence rehearsal aggregate drifted")
	}
	if result.ProviderRequests != 0 || result.CredentialReads != 0 || result.ToolCalls != 0 || result.RepositoryAccesses != 0 || result.ProductionActions != 0 || result.OutboundActions != 0 || result.PhoneFarmAccesses != 0 || result.ModelOrSeatSelections != 0 {
		return fmt.Errorf("role stage evidence rehearsal crossed a forbidden boundary")
	}
	if len(receipts) != result.AllRouteReceipts+result.LocalShadowReceipts {
		return fmt.Errorf("role stage evidence rehearsal receipt coverage drifted")
	}
	roles := make(map[string]RoleStageResponseContractV1, len(contract.RoleContracts))
	for _, roleContract := range contract.RoleContracts {
		roles[roleContract.RoleID] = roleContract
	}
	seen := map[string]struct{}{}
	for _, receipt := range receipts {
		identity := fmt.Sprintf("%s/%s/%d/%s/%s", receipt.Source, receipt.RouteID, receipt.Sequence, receipt.Mode, receipt.RoleID)
		if _, duplicate := seen[identity]; duplicate {
			return fmt.Errorf("duplicate role stage evidence rehearsal receipt")
		}
		seen[identity] = struct{}{}
		if receipt.SideEffects {
			return fmt.Errorf("role stage evidence rehearsal receipt has side effects")
		}
		if receipt.ControllerOnlyBinding {
			if receipt.Source != "local_shadow" || receipt.ResponseClass != "controller_only_outcome_binding" || receipt.ControllerHandoff || receipt.CompletePacketReissued || receipt.PriorEvidenceAdvisoryOnly || receipt.SelfContainedActionRequired || receipt.SharedSkillAttachment || !strings.HasPrefix(receipt.ArtifactBindingID, "silver-mediums.role-catalog.v1.") {
				return fmt.Errorf("controller-only role stage evidence receipt drifted")
			}
			continue
		}
		roleContract, ok := roles[receipt.RoleID]
		fieldsRaw, _ := json.Marshal(roleContract.TopLevelFields)
		if !ok || receipt.ArtifactBindingID != roleContract.ArtifactBindingID || receipt.ResponseClass != roleContract.ResponseClass || receipt.ResponseFieldSetSHA256 != localShadowSHA256V1(fieldsRaw) {
			return fmt.Errorf("role stage response contract receipt drifted")
		}
		if receipt.SharedSkillAttachment {
			if receipt.Source != "all_route" || receipt.ControllerHandoff || receipt.CompletePacketReissued || receipt.PriorEvidenceAdvisoryOnly || receipt.SelfContainedActionRequired {
				return fmt.Errorf("shared skill became a handoff or executor")
			}
		} else if !receipt.ControllerHandoff || !receipt.CompletePacketReissued || !receipt.PriorEvidenceAdvisoryOnly || receipt.SelfContainedActionRequired != (receipt.RoleID == contract.ControllerActionProtocol.AppliesToRoleID) {
			return fmt.Errorf("controller handoff or action obligation drifted")
		}
	}
	trace, err := json.Marshal(receipts)
	if err != nil {
		return err
	}
	traceSum := sha256.Sum256(trace)
	if result.CombinedTraceSHA256 != hex.EncodeToString(traceSum[:]) {
		return fmt.Errorf("role stage evidence rehearsal trace drifted")
	}
	return nil
}

func roleStageEvidenceReceiptV1(source, routeID string, sequence int, mode, roleID, binding string, contract RoleStageResponseContractV1, controllerHandoff, shared bool) RoleStageEvidenceRehearsalReceiptV1 {
	fieldsRaw, _ := json.Marshal(contract.TopLevelFields)
	return RoleStageEvidenceRehearsalReceiptV1{
		Source: source, RouteID: routeID, Sequence: sequence, Mode: mode, RoleID: roleID, ArtifactBindingID: binding,
		ResponseClass: contract.ResponseClass, ResponseFieldSetSHA256: localShadowSHA256V1(fieldsRaw), ControllerHandoff: controllerHandoff,
		CompletePacketReissued: controllerHandoff, PriorEvidenceAdvisoryOnly: controllerHandoff,
		SelfContainedActionRequired: controllerHandoff && roleID == "shared_windows_executor", SharedSkillAttachment: shared,
	}
}
