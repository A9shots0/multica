package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestRoleStageEvidenceContractV1CoversEveryResponseAndHandoff(t *testing.T) {
	contract, err := historicalContracts(t).CurrentRoleStageEvidenceContractV1()
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.RoleContracts) != 11 || contract.RouteCount != 20 || contract.ParticipantStageCount != 59 || contract.SharedSkillAttachmentCount != 10 {
		t.Fatalf("unexpected role stage coverage: %+v", contract)
	}
}

func TestRoleStageEvidenceContractV1RejectsMutations(t *testing.T) {
	base, err := historicalContracts(t).CurrentRoleStageEvidenceContractV1()
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*RoleStageEvidenceContractV1){
		"provider":            func(v *RoleStageEvidenceContractV1) { v.ProviderCallsAllowed = true },
		"model selection":     func(v *RoleStageEvidenceContractV1) { v.ModelSelectionAllowed = true },
		"direct handoff":      func(v *RoleStageEvidenceContractV1) { v.DirectRoleToRoleHandoffAllowed = true },
		"authority expansion": func(v *RoleStageEvidenceContractV1) { v.SpecialistEvidenceCanExpandAuthority = true },
		"missing packet":      func(v *RoleStageEvidenceContractV1) { v.ControllerReissuesCompletePacket = false },
		"nonliteral edit": func(v *RoleStageEvidenceContractV1) {
			v.ControllerActionProtocol.EditContentsSemantics = "implementation_advice"
		},
		"automatic replay": func(v *RoleStageEvidenceContractV1) { v.ControllerActionProtocol.AutomaticRepairOrReplayAllowed = true },
		"missing field": func(v *RoleStageEvidenceContractV1) {
			v.RoleContracts[0].TopLevelFields = v.RoleContracts[0].TopLevelFields[1:]
		},
		"Phone Farm class": func(v *RoleStageEvidenceContractV1) { v.RoleContracts[4].ResponseClass = "windows_executor" },
		"trial result":     func(v *RoleStageEvidenceContractV1) { v.TrialFindings.SeparateProductUIObservationsPassed = 1 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := cloneRoleStageEvidenceContractV1(t, base)
			mutate(&changed)
			if err := historicalContracts(t).ValidateRoleStageEvidenceContractV1(changed); err == nil {
				t.Fatal("mutated role stage evidence contract passed")
			}
		})
	}
}

func cloneRoleStageEvidenceContractV1(t *testing.T, value RoleStageEvidenceContractV1) RoleStageEvidenceContractV1 {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone RoleStageEvidenceContractV1
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestPrimarySubscriptionRoleTrialResultV1PreservesHeldSelection(t *testing.T) {
	result, err := CurrentPrimarySubscriptionRoleTrialResultV1()
	if err != nil {
		t.Fatal(err)
	}
	if result.Passes != 5 || result.ConfigurationSelection != "held_no_configuration_qualified" || result.PermanentSeatSelection != "held_no_permanent_seat_selected" {
		t.Fatalf("trial result boundary drifted: %+v", result)
	}
}
