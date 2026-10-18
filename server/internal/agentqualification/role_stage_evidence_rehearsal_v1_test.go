package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestRoleStageEvidenceRehearsalV1CoversAllRoutesAndLocalShadow(t *testing.T) {
	result, err := historicalContracts(t).CurrentRoleStageEvidenceRehearsalV1()
	if err != nil {
		t.Fatal(err)
	}
	if result.AllRouteReceipts+result.LocalShadowReceipts != 91 || result.ResponseContractBindings != 89 || result.ControllerOnlyBindings != 2 || result.ControllerHandoffs != 79 || result.SelfContainedActionObligations != 20 || !result.Valid {
		t.Fatalf("unexpected integrated rehearsal: %+v", result)
	}
}

func TestRoleStageEvidenceRehearsalV1RejectsHandoffAndTraceDrift(t *testing.T) {
	result, receipts, err := historicalContracts(t).RunRoleStageEvidenceRehearsalV1()
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*RoleStageEvidenceRehearsalResultV1, []RoleStageEvidenceRehearsalReceiptV1){
		"provider": func(v *RoleStageEvidenceRehearsalResultV1, _ []RoleStageEvidenceRehearsalReceiptV1) {
			v.ProviderRequests = 1
		},
		"selection": func(v *RoleStageEvidenceRehearsalResultV1, _ []RoleStageEvidenceRehearsalReceiptV1) {
			v.ModelOrSeatSelections = 1
		},
		"response class": func(_ *RoleStageEvidenceRehearsalResultV1, r []RoleStageEvidenceRehearsalReceiptV1) {
			r[0].ResponseClass = "other"
		},
		"direct handoff": func(_ *RoleStageEvidenceRehearsalResultV1, r []RoleStageEvidenceRehearsalReceiptV1) {
			r[0].ControllerHandoff = false
		},
		"missing packet": func(_ *RoleStageEvidenceRehearsalResultV1, r []RoleStageEvidenceRehearsalReceiptV1) {
			r[0].CompletePacketReissued = false
		},
		"authority inheritance": func(_ *RoleStageEvidenceRehearsalResultV1, r []RoleStageEvidenceRehearsalReceiptV1) {
			r[0].PriorEvidenceAdvisoryOnly = false
		},
		"action omission": func(_ *RoleStageEvidenceRehearsalResultV1, r []RoleStageEvidenceRehearsalReceiptV1) {
			for i := range r {
				if r[i].RoleID == "shared_windows_executor" && !r[i].SharedSkillAttachment {
					r[i].SelfContainedActionRequired = false
					return
				}
			}
		},
		"shared skill promotion": func(_ *RoleStageEvidenceRehearsalResultV1, r []RoleStageEvidenceRehearsalReceiptV1) {
			for i := range r {
				if r[i].SharedSkillAttachment {
					r[i].ControllerHandoff = true
					return
				}
			}
		},
		"controller execution": func(_ *RoleStageEvidenceRehearsalResultV1, r []RoleStageEvidenceRehearsalReceiptV1) {
			for i := range r {
				if r[i].ControllerOnlyBinding {
					r[i].ControllerHandoff = true
					return
				}
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changedResult := result
			changedReceipts := cloneRoleStageReceiptsV1(t, receipts)
			mutate(&changedResult, changedReceipts)
			if err := historicalContracts(t).ValidateRoleStageEvidenceRehearsalV1(changedResult, changedReceipts); err == nil {
				t.Fatal("altered integrated rehearsal passed")
			}
		})
	}
}

func cloneRoleStageReceiptsV1(t *testing.T, value []RoleStageEvidenceRehearsalReceiptV1) []RoleStageEvidenceRehearsalReceiptV1 {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone []RoleStageEvidenceRehearsalReceiptV1
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
