package agentqualification

import (
	"strings"
	"testing"
)

func TestTopologyV4CampaignEnforcesOrderExclusiveClaimAndPairIntegrity(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV4()
	first, second := definition.ExecutionOrder[0], definition.ExecutionOrder[1]
	state, err := ClaimTopologyV4Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimTopologyV4Observation(state, definition, first.ObservationID, "claim-2"); err == nil {
		t.Fatal("concurrent claim was accepted")
	}
	firstResult := validTopologyResult(definition, first, "claim-1")
	state, err = RecordTopologyV4Observation(state, definition, firstResult)
	if err != nil {
		t.Fatal(err)
	}
	state, err = ClaimTopologyV4Observation(state, definition, second.ObservationID, "claim-2")
	if err != nil {
		t.Fatal(err)
	}
	secondResult := validTopologyResult(definition, second, "claim-2")
	secondResult.InitialTreeSHA256 = strings.Repeat("b", 64)
	state, err = RecordTopologyV4Observation(state, definition, secondResult)
	if err != nil || state.GlobalHaltReason != "evidence_corruption" {
		t.Fatalf("mismatched pair did not fail closed: state=%+v err=%v", state, err)
	}
}

func TestTopologyV4CampaignReservationFailsBeforeLaunch(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV4()
	state := TopologyCampaignState{TotalTokens: definition.MaxTotalTokens - 74999}
	if _, err := ClaimTopologyV4Observation(state, definition, definition.ExecutionOrder[0].ObservationID, "claim"); err == nil {
		t.Fatal("campaign admitted an observation beyond its token ceiling")
	}
	state = TopologyCampaignState{TotalCostMicros: definition.MaxIncrementalSpendMicros - 299999}
	if _, err := ClaimTopologyV4Observation(state, definition, definition.ExecutionOrder[0].ObservationID, "claim"); err == nil {
		t.Fatal("campaign admitted an observation beyond its cost ceiling")
	}
}
