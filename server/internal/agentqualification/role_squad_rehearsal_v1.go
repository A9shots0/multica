package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

const RoleSquadRehearsalSchemaV1 = "silver-mediums.role-squad-rehearsal.v1"

//go:embed role_squad_rehearsal_v1_result.json
var roleSquadRehearsalResultV1JSON []byte

type RoleSquadRehearsalReceiptV1 struct {
	RouteID            string   `json:"route_id"`
	StageSequence      int      `json:"stage_sequence"`
	Mode               string   `json:"mode"`
	RoleID             string   `json:"role_id"`
	ArtifactVersion    string   `json:"artifact_version"`
	ArtifactBindingID  string   `json:"artifact_binding_id"`
	InstructionSHA256  string   `json:"instruction_sha256"`
	SkillSHA256        string   `json:"skill_sha256"`
	Contribution       string   `json:"contribution"`
	RequiredEvidence   []string `json:"required_evidence,omitempty"`
	ControllerBound    bool     `json:"controller_bound"`
	SeparateWorker     bool     `json:"separate_worker"`
	IndependentVerdict bool     `json:"independent_verdict"`
}

type RoleSquadRehearsalResultV1 struct {
	SchemaVersion                string `json:"schema_version"`
	RunID                        string `json:"run_id"`
	ExecutionMode                string `json:"execution_mode"`
	OperatingContractSHA256      string `json:"operating_contract_sha256"`
	TraceSHA256                  string `json:"trace_sha256"`
	CaseRouteCount               int    `json:"case_route_count"`
	AdditionalRouteCount         int    `json:"additional_route_count"`
	ParticipantStageCount        int    `json:"participant_stage_count"`
	SharedSkillAttachmentCount   int    `json:"shared_skill_attachment_count"`
	ArtifactResolutionCount      int    `json:"artifact_resolution_count"`
	UniqueArtifactRoleCount      int    `json:"unique_artifact_role_count"`
	RequiredEvidenceFieldCount   int    `json:"required_evidence_field_count"`
	FinalIndependentVerdictCount int    `json:"final_independent_verdict_count"`
	ControllerMoveCount          int    `json:"controller_move_count"`
	MaxObservedConcurrentWorkers int    `json:"max_observed_concurrent_workers"`
	ProviderRequests             int    `json:"provider_requests"`
	CredentialReads              int    `json:"credential_reads"`
	ToolCalls                    int    `json:"tool_calls"`
	RepositoryAccesses           int    `json:"repository_accesses"`
	ProductionActions            int    `json:"production_actions"`
	OutboundActions              int    `json:"outbound_actions"`
	PhoneFarmAccesses            int    `json:"phone_farm_accesses"`
	ModelOrSeatSelections        int    `json:"model_or_seat_selections"`
	Valid                        bool   `json:"valid"`
}

func (ctxContracts QualificationContracts) CurrentRoleSquadRehearsalResultV1() (RoleSquadRehearsalResultV1, error) {
	var recorded RoleSquadRehearsalResultV1
	if err := strictJSON(roleSquadRehearsalResultV1JSON, &recorded); err != nil {
		return recorded, fmt.Errorf("decode role/squad rehearsal result: %w", err)
	}
	resolve := memoizedRoleSquadArtifactsV1(roleSquadArtifactRefV1)
	expected, receipts, err := ctxContracts.runRoleSquadRehearsalV1(resolve)
	if err != nil {
		return RoleSquadRehearsalResultV1{}, err
	}
	if err := validateRoleSquadRehearsalWithResolverV1(recorded, receipts, resolve); err != nil {
		return RoleSquadRehearsalResultV1{}, err
	}
	if recorded != expected {
		return RoleSquadRehearsalResultV1{}, fmt.Errorf("recorded role/squad rehearsal result drifted")
	}
	return recorded, nil
}

func (ctxContracts QualificationContracts) RunRoleSquadRehearsalV1() (RoleSquadRehearsalResultV1, []RoleSquadRehearsalReceiptV1, error) {
	return ctxContracts.runRoleSquadRehearsalV1(memoizedRoleSquadArtifactsV1(roleSquadArtifactRefV1))
}

func (ctxContracts QualificationContracts) runRoleSquadRehearsalV1(resolve roleSquadArtifactResolverV1) (RoleSquadRehearsalResultV1, []RoleSquadRehearsalReceiptV1, error) {
	contract, err := ctxContracts.CurrentRoleSquadOperatingContractV1()
	if err != nil {
		return RoleSquadRehearsalResultV1{}, nil, err
	}
	cases, err := Cases()
	if err != nil {
		return RoleSquadRehearsalResultV1{}, nil, err
	}
	caseByID := make(map[string]Case, len(cases))
	for _, item := range cases {
		caseByID[item.ID] = item
	}
	var receipts []RoleSquadRehearsalReceiptV1
	uniqueRoles := map[string]struct{}{}
	requiredEvidenceFields := 0
	finalVerdicts := 0
	controllerMoves := 0
	sharedSkills := 0
	participantStages := 0
	for _, route := range contract.CaseRoutes {
		item := caseByID[route.CaseID]
		requiredEvidenceFields += len(item.RequiredEvidence)
		for index, roleID := range route.Participants {
			ref, err := resolve(roleID)
			if err != nil {
				return RoleSquadRehearsalResultV1{}, nil, fmt.Errorf("case %s participant %s: %w", route.CaseID, roleID, err)
			}
			receipt := roleSquadRehearsalReceiptV1(route.CaseID, index+1, "participant_stage", ref)
			receipt.SeparateWorker = true
			if roleID == "quality_checker" {
				receipt.IndependentVerdict = true
				receipt.RequiredEvidence = append([]string(nil), item.RequiredEvidence...)
				finalVerdicts++
			}
			receipts = append(receipts, receipt)
			uniqueRoles[roleID] = struct{}{}
			controllerMoves++
			participantStages++
			if roleID == "shared_windows_executor" {
				for _, sharedRoleID := range route.SharedExecutorSkills {
					ref, err := resolve(sharedRoleID)
					if err != nil {
						return RoleSquadRehearsalResultV1{}, nil, fmt.Errorf("case %s shared skill %s: %w", route.CaseID, sharedRoleID, err)
					}
					receipt := roleSquadRehearsalReceiptV1(route.CaseID, index+1, "shared_skill_attachment", ref)
					receipts = append(receipts, receipt)
					uniqueRoles[sharedRoleID] = struct{}{}
					sharedSkills++
				}
			}
		}
	}
	for _, route := range contract.AdditionalRoutes {
		for index, roleID := range route.Participants {
			ref, err := resolve(roleID)
			if err != nil {
				return RoleSquadRehearsalResultV1{}, nil, fmt.Errorf("route %s participant %s: %w", route.ID, roleID, err)
			}
			receipt := roleSquadRehearsalReceiptV1(route.ID, index+1, "participant_stage", ref)
			receipt.SeparateWorker = true
			if roleID == "quality_checker" {
				receipt.IndependentVerdict = true
				finalVerdicts++
			}
			receipts = append(receipts, receipt)
			uniqueRoles[roleID] = struct{}{}
			controllerMoves++
			participantStages++
		}
	}
	trace, err := json.Marshal(receipts)
	if err != nil {
		return RoleSquadRehearsalResultV1{}, nil, err
	}
	traceSum := sha256.Sum256(trace)
	result := RoleSquadRehearsalResultV1{
		SchemaVersion:                RoleSquadRehearsalSchemaV1,
		RunID:                        "item-14-provider-free-role-squad-rehearsal-20260914",
		ExecutionMode:                "deterministic_artifact_and_collaboration_rehearsal",
		OperatingContractSHA256:      RoleSquadOperatingContractV1SHA256(),
		TraceSHA256:                  hex.EncodeToString(traceSum[:]),
		CaseRouteCount:               len(contract.CaseRoutes),
		AdditionalRouteCount:         len(contract.AdditionalRoutes),
		ParticipantStageCount:        participantStages,
		SharedSkillAttachmentCount:   sharedSkills,
		ArtifactResolutionCount:      len(receipts),
		UniqueArtifactRoleCount:      len(uniqueRoles),
		RequiredEvidenceFieldCount:   requiredEvidenceFields,
		FinalIndependentVerdictCount: finalVerdicts,
		ControllerMoveCount:          controllerMoves,
		MaxObservedConcurrentWorkers: 1,
		Valid:                        true,
	}
	if err := validateRoleSquadRehearsalWithResolverV1(result, receipts, resolve); err != nil {
		return RoleSquadRehearsalResultV1{}, nil, err
	}
	return result, receipts, nil
}

func validateRoleSquadRehearsalV1(result RoleSquadRehearsalResultV1, receipts []RoleSquadRehearsalReceiptV1) error {
	return validateRoleSquadRehearsalWithResolverV1(result, receipts, memoizedRoleSquadArtifactsV1(roleSquadArtifactRefV1))
}

func validateRoleSquadRehearsalWithResolverV1(result RoleSquadRehearsalResultV1, receipts []RoleSquadRehearsalReceiptV1, resolve roleSquadArtifactResolverV1) error {
	if result.SchemaVersion != RoleSquadRehearsalSchemaV1 || result.RunID != "item-14-provider-free-role-squad-rehearsal-20260914" ||
		result.ExecutionMode != "deterministic_artifact_and_collaboration_rehearsal" || result.OperatingContractSHA256 != RoleSquadOperatingContractV1SHA256() ||
		result.CaseRouteCount != 19 || result.AdditionalRouteCount != 1 || result.ParticipantStageCount != 59 ||
		result.SharedSkillAttachmentCount != 10 || result.ArtifactResolutionCount != 69 || result.UniqueArtifactRoleCount != 11 ||
		result.RequiredEvidenceFieldCount != 76 || result.FinalIndependentVerdictCount != 20 || result.ControllerMoveCount != 59 ||
		result.MaxObservedConcurrentWorkers != 1 || result.ProviderRequests != 0 || result.CredentialReads != 0 || result.ToolCalls != 0 ||
		result.RepositoryAccesses != 0 || result.ProductionActions != 0 || result.OutboundActions != 0 || result.PhoneFarmAccesses != 0 ||
		result.ModelOrSeatSelections != 0 || !result.Valid {
		return fmt.Errorf("role/squad rehearsal aggregate drifted")
	}
	trace, err := json.Marshal(receipts)
	if err != nil {
		return err
	}
	traceSum := sha256.Sum256(trace)
	if result.TraceSHA256 != hex.EncodeToString(traceSum[:]) {
		return fmt.Errorf("role/squad rehearsal trace hash drifted")
	}
	lastSequence := map[string]int{}
	verdicts := map[string]int{}
	verdictSeen := map[string]bool{}
	sharedExecutorStage := map[string]int{}
	contributions := map[string]map[string]struct{}{}
	evidenceByRoute := map[string][]string{"A01": {}}
	cases, err := Cases()
	if err != nil {
		return err
	}
	for _, item := range cases {
		evidenceByRoute[item.ID] = item.RequiredEvidence
	}
	for _, receipt := range receipts {
		if verdictSeen[receipt.RouteID] {
			return fmt.Errorf("route %s has activity after its final verdict", receipt.RouteID)
		}
		if !receipt.ControllerBound || receipt.StageSequence < 1 || receipt.RoleID == "" || receipt.ArtifactBindingID == "" ||
			!validSHA256(receipt.InstructionSHA256) || !validSHA256(receipt.SkillSHA256) || receipt.Contribution == "" {
			return fmt.Errorf("role/squad rehearsal receipt is incomplete")
		}
		ref, err := resolve(receipt.RoleID)
		if err != nil || receipt.ArtifactVersion != ref.ArtifactVersion || receipt.ArtifactBindingID != ref.BindingID ||
			receipt.InstructionSHA256 != ref.InstructionSHA256 || receipt.SkillSHA256 != ref.SkillSHA256 ||
			receipt.Contribution != roleSquadContributionV1(receipt.RoleID) {
			return fmt.Errorf("route %s role %s artifact or contribution drifted", receipt.RouteID, receipt.RoleID)
		}
		if receipt.Mode == "participant_stage" {
			if !receipt.SeparateWorker || receipt.StageSequence <= lastSequence[receipt.RouteID] {
				return fmt.Errorf("route %s participant stages are not controller-ordered", receipt.RouteID)
			}
			lastSequence[receipt.RouteID] = receipt.StageSequence
			if receipt.RoleID == "shared_windows_executor" {
				sharedExecutorStage[receipt.RouteID] = receipt.StageSequence
			}
		} else if receipt.Mode == "shared_skill_attachment" {
			if receipt.SeparateWorker || receipt.IndependentVerdict || receipt.RoleID == "quality_checker" {
				return fmt.Errorf("route %s shared skill became a worker or reviewer", receipt.RouteID)
			}
			if sharedExecutorStage[receipt.RouteID] == 0 || receipt.StageSequence != sharedExecutorStage[receipt.RouteID] {
				return fmt.Errorf("route %s shared skill is not attached to the shared executor stage", receipt.RouteID)
			}
		} else {
			return fmt.Errorf("route %s has unknown rehearsal mode", receipt.RouteID)
		}
		if receipt.IndependentVerdict {
			if receipt.RoleID != "quality_checker" || receipt.Mode != "participant_stage" {
				return fmt.Errorf("route %s has an invalid independent verdict", receipt.RouteID)
			}
			if !slices.Equal(receipt.RequiredEvidence, evidenceByRoute[receipt.RouteID]) {
				return fmt.Errorf("route %s final verdict lost its exact evidence contract", receipt.RouteID)
			}
			verdicts[receipt.RouteID]++
			verdictSeen[receipt.RouteID] = true
		} else if len(receipt.RequiredEvidence) != 0 {
			return fmt.Errorf("route %s non-review receipt claimed the final evidence contract", receipt.RouteID)
		}
		if receipt.Mode == "participant_stage" && receipt.RoleID != "quality_checker" {
			if contributions[receipt.RouteID] == nil {
				contributions[receipt.RouteID] = map[string]struct{}{}
			}
			if _, duplicate := contributions[receipt.RouteID][receipt.Contribution]; duplicate {
				return fmt.Errorf("route %s has duplicate contributor evidence", receipt.RouteID)
			}
			contributions[receipt.RouteID][receipt.Contribution] = struct{}{}
		}
	}
	if len(verdicts) != 20 {
		return fmt.Errorf("role/squad rehearsal verdict coverage is incomplete")
	}
	for routeID, count := range verdicts {
		if count != 1 {
			return fmt.Errorf("route %s has %d final verdicts", routeID, count)
		}
	}
	return nil
}

func roleSquadRehearsalReceiptV1(routeID string, sequence int, mode string, ref ShadowRoleArtifactRefV1) RoleSquadRehearsalReceiptV1 {
	return RoleSquadRehearsalReceiptV1{
		RouteID:           routeID,
		StageSequence:     sequence,
		Mode:              mode,
		RoleID:            ref.RoleID,
		ArtifactVersion:   ref.ArtifactVersion,
		ArtifactBindingID: ref.BindingID,
		InstructionSHA256: ref.InstructionSHA256,
		SkillSHA256:       ref.SkillSHA256,
		Contribution:      roleSquadContributionV1(ref.RoleID),
		ControllerBound:   true,
	}
}

func roleSquadArtifactRefV1(roleID string) (ShadowRoleArtifactRefV1, error) {
	version := "v2"
	if slices.Contains([]string{"mika_owner_interface", "product_ui", "quality_checker", "shared_windows_executor"}, roleID) {
		version = "v1"
	}
	return ValidatedShadowRoleArtifactRefV1("silver-mediums.role-artifact." + version + "." + roleID)
}

func roleSquadContributionV1(roleID string) string {
	return map[string]string{
		"architecture_assurance":           "authority_contract_and_blast_radius_constraints",
		"company_auditor":                  "broad_truth_completeness_and_freshness_audit",
		"data_sync_integrity":              "first_divergence_and_source_destination_reconciliation",
		"mika_owner_interface":             "owner_intent_and_exception_binding",
		"phone_fleet_reliability":          "mac_device_bridge_connectivity_and_session_evidence",
		"phonefarm_application_specialist": "mac_application_behavior_and_release",
		"product_ui":                       "interaction_information_and_responsive_acceptance",
		"quality_checker":                  "independent_complete_outcome_verdict",
		"release_runtime_assurance":        "candidate_runtime_rollback_and_live_truth",
		"security_privacy_review":          "redacted_risk_and_capability_boundary",
		"shared_windows_executor":          "bounded_implementation_and_end_to_end_candidate",
	}[roleID]
}

type roleSquadArtifactResolverV1 func(string) (ShadowRoleArtifactRefV1, error)

// Share validated immutable artifact references only within one rehearsal call.
// Every independent public call gets a fresh resolver; no current-source check
// or receipt validation is cached across calls. References contain value fields.
func memoizedRoleSquadArtifactsV1(resolve roleSquadArtifactResolverV1) roleSquadArtifactResolverV1 {
	refs := map[string]ShadowRoleArtifactRefV1{}
	return func(roleID string) (ShadowRoleArtifactRefV1, error) {
		if ref, ok := refs[roleID]; ok {
			return ref, nil
		}
		ref, err := resolve(roleID)
		if err != nil {
			return ShadowRoleArtifactRefV1{}, err
		}
		refs[roleID] = ref
		return ref, nil
	}
}
