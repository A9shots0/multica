package agentqualification

import "testing"

func TestRealisticNextGroupDeclaresOnlyDeliberateRemainingCases(t *testing.T) {
	group, err := CurrentRealisticNextGroup()
	if err != nil {
		t.Fatal(err)
	}
	if group.Status != "frozen_reviewed" || len(group.Cases) != 2 || group.Cases[0].CaseID != "R05" || group.Cases[1].CaseID != "R06" {
		t.Fatalf("unexpected realistic next group: %+v", group)
	}
	if group.ExecutionPolicy.ProviderCallsAllowedBeforeFreeze {
		t.Fatal("provider calls unlocked before complete independent review")
	}
}
