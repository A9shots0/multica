package agentqualification

import (
	"strings"
	"testing"
)

func TestTopologyV3CampaignRequiresExclusiveClaimsAndMatchedPairHashes(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV3()
	first, second := definition.ExecutionOrder[0], definition.ExecutionOrder[1]
	state, err := ClaimTopologyV3Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimTopologyV3Observation(state, definition, first.ObservationID, "claim-2"); err == nil {
		t.Fatal("concurrent topology v3 claim was accepted")
	}
	firstResult := validTopologyResult(definition, first, "claim-1")
	state, err = RecordTopologyV3Observation(state, definition, firstResult)
	if err != nil {
		t.Fatal(err)
	}
	state, err = ClaimTopologyV3Observation(state, definition, second.ObservationID, "claim-2")
	if err != nil {
		t.Fatal(err)
	}
	secondResult := validTopologyResult(definition, second, "claim-2")
	secondResult.InitialTreeSHA256 = strings.Repeat("b", 64)
	state, err = RecordTopologyV3Observation(state, definition, secondResult)
	if err != nil || state.GlobalHaltReason != "evidence_corruption" {
		t.Fatalf("mismatched v3 pair did not fail closed: state=%+v err=%v", state, err)
	}
}
