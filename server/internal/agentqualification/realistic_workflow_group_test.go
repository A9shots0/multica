package agentqualification

import "testing"

func TestRealisticWorkflowGroupDefinesBoundedCapabilityStage(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	if group.Status != "frozen_reviewed" || len(group.Cases) != 4 || group.ExecutionPolicy.ProviderCallsAllowedBeforeFreeze {
		t.Fatalf("unexpected realistic workflow group: %+v", group)
	}
	for _, candidateCase := range group.Cases {
		if len(candidateCase.RequiredEvidence) == 0 || len(candidateCase.HardDisqualifiers) == 0 {
			t.Fatalf("case is not evidence gated: %+v", candidateCase)
		}
	}
}

func TestRealisticWorkflowGroupRejectsBroadenedAuthority(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	group.ExecutionPolicy.ForbiddenTools = append([]string(nil), group.ExecutionPolicy.ForbiddenTools...)
	group.ExecutionPolicy.ForbiddenTools = group.ExecutionPolicy.ForbiddenTools[1:]
	if err := ValidateRealisticWorkflowGroup(group); err == nil {
		t.Fatal("group accepted removal of a forbidden capability")
	}
	group, _ = CurrentRealisticWorkflowGroup()
	group.ExecutionPolicy.ProviderCallsAllowedBeforeFreeze = true
	if err := ValidateRealisticWorkflowGroup(group); err == nil {
		t.Fatal("group allowed provider calls before freeze")
	}
}

func TestRealisticWorkflowGroupRejectsMissingEvidenceGate(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	group.Cases = append([]RealisticWorkflowCase(nil), group.Cases...)
	group.Cases[0].RequiredEvidence = nil
	if err := ValidateRealisticWorkflowGroup(group); err == nil {
		t.Fatal("group accepted a case without evidence gates")
	}
}
