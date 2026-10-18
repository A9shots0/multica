package agentqualification

import "testing"

func TestCapabilityCaseGroupPinsValidatedPackets(t *testing.T) {
	group, err := CurrentCapabilityCaseGroup()
	if err != nil {
		t.Fatal(err)
	}
	if len(group.Cases) != 5 || group.Status != "frozen_reviewed" {
		t.Fatalf("unexpected group: %+v", group)
	}
	changed := group
	changed.Cases = append([]ScreeningCase(nil), group.Cases...)
	changed.Cases[2].GraderPacketSHA256 = "changed"
	if err := ValidateCapabilityCaseGroup(changed); err == nil {
		t.Fatal("changed packet hash was accepted")
	}
}
