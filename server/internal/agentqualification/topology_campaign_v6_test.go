package agentqualification

import (
	"strings"
	"testing"
)

func TestTopologyV6CampaignEnforcesOrderExclusiveClaimAndPairIntegrity(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV6()
	first, second := definition.ExecutionOrder[0], definition.ExecutionOrder[1]
	state, err := ClaimTopologyV6Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimTopologyV6Observation(state, definition, first.ObservationID, "claim-2"); err == nil {
		t.Fatal("concurrent claim was accepted")
	}
	firstResult := validTopologyResult(definition, first, "claim-1")
	state, err = RecordTopologyV6Observation(state, definition, firstResult)
	if err != nil {
		t.Fatal(err)
	}
	state, err = ClaimTopologyV6Observation(state, definition, second.ObservationID, "claim-2")
	if err != nil {
		t.Fatal(err)
	}
	secondResult := validTopologyResult(definition, second, "claim-2")
	secondResult.InitialTreeSHA256 = strings.Repeat("b", 64)
	state, err = RecordTopologyV6Observation(state, definition, secondResult)
	if err != nil || state.GlobalHaltReason != "evidence_corruption" {
		t.Fatalf("mismatched pair did not fail closed: state=%+v err=%v", state, err)
	}
}

func TestTopologyV6CampaignReservationFailsBeforeLaunch(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV6()
	state := TopologyCampaignState{TotalTokens: definition.MaxTotalTokens - 74999}
	if _, err := ClaimTopologyV6Observation(state, definition, definition.ExecutionOrder[0].ObservationID, "claim"); err == nil {
		t.Fatal("campaign admitted an observation beyond its token ceiling")
	}
	state = TopologyCampaignState{TotalCostMicros: definition.MaxIncrementalSpendMicros - 199999}
	if _, err := ClaimTopologyV6Observation(state, definition, definition.ExecutionOrder[0].ObservationID, "claim"); err == nil {
		t.Fatal("campaign admitted an observation beyond its cost ceiling")
	}
}

func TestTopologyV6ProtocolFailureIsCandidateLocalAndNeedsNoReviewerReceipt(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV6()
	observation := definition.ExecutionOrder[0]
	state, err := ClaimTopologyV6Observation(TopologyCampaignState{}, definition, observation.ObservationID, "claim-protocol")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("c", 64)
	result := TopologyExperimentResult{
		ClaimID: "claim-protocol", ObservationID: observation.ObservationID, CaseID: observation.CaseID,
		Repeat: observation.Repeat, ArmID: observation.ArmID, Outcome: "fail", FailureClass: "response_or_tool_protocol_failure",
		InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts: []TopologyStageReceipt{{RunID: "claim-protocol:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 12, OutputTokens: 2, CostMicros: 3}},
		ProviderCalls: 1, TotalTokens: 14, CostMicros: 3, AccountingStatus: "exact",
	}
	state, err = RecordTopologyV6Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "" || len(state.Observations) != 1 {
		t.Fatalf("candidate-local protocol failure halted or was rejected: state=%+v err=%v", state, err)
	}
	next, err := NextTopologyV6Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != definition.ExecutionOrder[1].ObservationID {
		t.Fatalf("unaffected observation did not continue: next=%+v err=%v", next, err)
	}
}

func TestTopologyV6CorrectionProtocolFailureStopsBeforeReviewTwo(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV6()
	first, reviewed := definition.ExecutionOrder[0], definition.ExecutionOrder[1]
	state, err := ClaimTopologyV6Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-first")
	if err != nil {
		t.Fatal(err)
	}
	state, err = RecordTopologyV6Observation(state, definition, validTopologyResult(definition, first, "claim-first"))
	if err != nil {
		t.Fatal(err)
	}
	state, err = ClaimTopologyV6Observation(state, definition, reviewed.ObservationID, "claim-correction-protocol")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	stages := []TopologyStageReceipt{
		{RunID: "claim-correction-protocol:execute", Stage: "execute", ProviderCalls: 1, InputTokens: 10, OutputTokens: 2, CostMicros: 1},
		{RunID: "claim-correction-protocol:review-1", Stage: "review_1", ProviderCalls: 1, InputTokens: 5, OutputTokens: 1, CostMicros: 1},
		{RunID: "claim-correction-protocol:correction", Stage: "correction", ProviderCalls: 1, InputTokens: 7, OutputTokens: 1, CostMicros: 1, Correction: true},
	}
	result := TopologyExperimentResult{
		ClaimID: "claim-correction-protocol", ObservationID: reviewed.ObservationID, CaseID: reviewed.CaseID,
		Repeat: reviewed.Repeat, ArmID: reviewed.ArmID, Outcome: "fail", FailureClass: "response_or_tool_protocol_failure",
		InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts: stages, ProviderCalls: 3, TotalTokens: 26, CostMicros: 3, AccountingStatus: "exact",
	}
	state, err = RecordTopologyV6Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "" || len(state.Observations) != 2 {
		t.Fatalf("correction protocol failure halted or was rejected: state=%+v err=%v", state, err)
	}
}

func TestTopologyV6HarnessImperfectionInvalidatesOnlyAffectedObservation(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV6()
	first := definition.ExecutionOrder[0]
	state, err := ClaimTopologyV6Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-invalid")
	if err != nil {
		t.Fatal(err)
	}
	result := validTopologyResult(definition, first, "claim-invalid")
	result.Outcome = "invalid"
	result.FailureClass = "transport_or_harness_imperfection"
	result.PostBatchCorrection = "Preserve this affected observation and continue unaffected candidates."
	state, err = RecordTopologyV6Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "" || len(state.PostBatchCorrections) != 1 {
		t.Fatalf("local harness imperfection halted the campaign: state=%+v err=%v", state, err)
	}
	next, err := NextTopologyV6Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != definition.ExecutionOrder[1].ObservationID {
		t.Fatalf("unaffected observation did not continue: next=%+v err=%v", next, err)
	}
}
