package agentqualification

import (
	"strings"
	"testing"
)

func TestTopologyV7CampaignEnforcesOrderExclusiveClaimAndPairIntegrity(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	first, second := definition.ExecutionOrder[0], definition.ExecutionOrder[1]
	state, err := ClaimTopologyV7Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimTopologyV7Observation(state, definition, first.ObservationID, "claim-2"); err == nil {
		t.Fatal("concurrent claim was accepted")
	}
	firstResult := validTopologyResult(definition, first, "claim-1")
	state, err = RecordTopologyV7Observation(state, definition, firstResult)
	if err != nil {
		t.Fatal(err)
	}
	state, err = ClaimTopologyV7Observation(state, definition, second.ObservationID, "claim-2")
	if err != nil {
		t.Fatal(err)
	}
	secondResult := validTopologyResult(definition, second, "claim-2")
	secondResult.InitialTreeSHA256 = strings.Repeat("b", 64)
	state, err = RecordTopologyV7Observation(state, definition, secondResult)
	if err != nil || state.GlobalHaltReason != "evidence_corruption" {
		t.Fatalf("mismatched pair did not fail closed: state=%+v err=%v", state, err)
	}
}

func TestTopologyV7CampaignReservationFailsBeforeLaunch(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	state := TopologyCampaignState{TotalTokens: definition.MaxTotalTokens - 74999}
	if _, err := ClaimTopologyV7Observation(state, definition, definition.ExecutionOrder[0].ObservationID, "claim"); err == nil {
		t.Fatal("campaign admitted an observation beyond its token ceiling")
	}
	state = TopologyCampaignState{TotalCostMicros: definition.MaxIncrementalSpendMicros - 199999}
	if _, err := ClaimTopologyV7Observation(state, definition, definition.ExecutionOrder[0].ObservationID, "claim"); err == nil {
		t.Fatal("campaign admitted an observation beyond its cost ceiling")
	}
}

func TestTopologyV7ProtocolFailureIsCandidateLocalAndNeedsNoReviewerReceipt(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	observation := definition.ExecutionOrder[0]
	state, err := ClaimTopologyV7Observation(TopologyCampaignState{}, definition, observation.ObservationID, "claim-protocol")
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
	state, err = RecordTopologyV7Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "" || len(state.Observations) != 1 {
		t.Fatalf("candidate-local protocol failure halted or was rejected: state=%+v err=%v", state, err)
	}
	next, err := NextTopologyV7Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != definition.ExecutionOrder[1].ObservationID {
		t.Fatalf("unaffected observation did not continue: next=%+v err=%v", next, err)
	}
}

func TestTopologyV7CorrectionProtocolFailureStopsBeforeReviewTwo(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	first, reviewed := definition.ExecutionOrder[0], definition.ExecutionOrder[1]
	state, err := ClaimTopologyV7Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-first")
	if err != nil {
		t.Fatal(err)
	}
	state, err = RecordTopologyV7Observation(state, definition, validTopologyResult(definition, first, "claim-first"))
	if err != nil {
		t.Fatal(err)
	}
	state, err = ClaimTopologyV7Observation(state, definition, reviewed.ObservationID, "claim-correction-protocol")
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
	state, err = RecordTopologyV7Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "" || len(state.Observations) != 2 {
		t.Fatalf("correction protocol failure halted or was rejected: state=%+v err=%v", state, err)
	}
}

func TestTopologyV7HarnessImperfectionInvalidatesOnlyAffectedObservation(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	first := definition.ExecutionOrder[0]
	state, err := ClaimTopologyV7Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-invalid")
	if err != nil {
		t.Fatal(err)
	}
	result := validTopologyV7PaidHarnessInvalid(first, "claim-invalid")
	state, err = RecordTopologyV7Observation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "" || len(state.PostBatchCorrections) != 1 {
		t.Fatalf("local harness imperfection halted the campaign: state=%+v err=%v", state, err)
	}
	next, err := NextTopologyV7Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != definition.ExecutionOrder[1].ObservationID {
		t.Fatalf("unaffected observation did not continue: next=%+v err=%v", next, err)
	}
}

func validTopologyV7PaidHarnessInvalid(observation TopologyExperimentObservation, claimID string) TopologyExperimentResult {
	hash := strings.Repeat("f", 64)
	return TopologyExperimentResult{
		ClaimID: claimID, ObservationID: observation.ObservationID, CaseID: observation.CaseID,
		Repeat: observation.Repeat, ArmID: observation.ArmID, Outcome: "invalid", FailureClass: "transport_or_harness_imperfection",
		InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts: []TopologyStageReceipt{{RunID: claimID + ":execute", Stage: "execute", ProviderCalls: 1, InputTokens: 10, OutputTokens: 2, CostMicros: 3}},
		ProviderCalls: 1, TotalTokens: 12, CostMicros: 3, AccountingStatus: "exact",
		PostBatchCorrection: "Preserve this affected observation and continue unaffected candidates.",
	}
}

func validTopologyV7ZeroCallInvalid(definition TopologyExperimentDefinition, observation TopologyExperimentObservation, claimID string) TopologyExperimentResult {
	hash := strings.Repeat("e", 64)
	return TopologyExperimentResult{
		ClaimID: claimID, ObservationID: observation.ObservationID, CaseID: observation.CaseID,
		Repeat: observation.Repeat, ArmID: observation.ArmID, Outcome: "invalid", FailureClass: "runtime_preflight_invalid",
		InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts:    []TopologyStageReceipt{{RunID: claimID + ":execute", Stage: "execute"}},
		AccountingStatus: "exact", PostBatchCorrection: "preserved zero-call preflight invalidity",
	}
}

func TestTopologyV7ZeroCallRuntimeInvalidClearsClaimAndContinues(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	first := definition.ExecutionOrder[0]
	state, err := ClaimTopologyV7Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-zero")
	if err != nil {
		t.Fatal(err)
	}
	state, err = RecordTopologyV7Observation(state, definition, validTopologyV7ZeroCallInvalid(definition, first, "claim-zero"))
	if err != nil || state.ActiveClaim != "" || state.GlobalHaltReason != "" || state.TotalProviderCalls != 0 || state.TotalTokens != 0 || state.TotalCostMicros != 0 || len(state.PostBatchCorrections) != 1 {
		t.Fatalf("zero-call invalidity was not durably local: state=%+v err=%v", state, err)
	}
	next, err := NextTopologyV7Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != definition.ExecutionOrder[1].ObservationID {
		t.Fatalf("unaffected observation did not continue: next=%+v err=%v", next, err)
	}
}

func TestTopologyV7MalformedZeroCallRuntimeInvalidIsRejected(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	first := definition.ExecutionOrder[0]
	mutations := map[string]func(*TopologyExperimentResult){
		"missing stage": func(result *TopologyExperimentResult) { result.StageReceipts = nil },
		"paid call": func(result *TopologyExperimentResult) {
			result.StageReceipts[0].ProviderCalls = 1
			result.ProviderCalls = 1
		},
		"token accounting": func(result *TopologyExperimentResult) {
			result.StageReceipts[0].InputTokens = 1
			result.TotalTokens = 1
		},
		"missing correction":   func(result *TopologyExperimentResult) { result.PostBatchCorrection = "" },
		"malformed evidence":   func(result *TopologyExperimentResult) { result.FinalTreeSHA256 = "bad" },
		"ambiguous accounting": func(result *TopologyExperimentResult) { result.AccountingStatus = "ambiguous" },
		"owner intervention":   func(result *TopologyExperimentResult) { result.OwnerInterventions = 1 },
		"external side effect": func(result *TopologyExperimentResult) { result.ExternalSideEffects = 1 },
		"negative duration":    func(result *TopologyExperimentResult) { result.EndToEndMillis = -1 },
		"wrong execute run id": func(result *TopologyExperimentResult) { result.StageReceipts[0].RunID = "wrong" },
		"execute correction":   func(result *TopologyExperimentResult) { result.StageReceipts[0].Correction = true },
		"changed final tree":   func(result *TopologyExperimentResult) { result.FinalTreeSHA256 = strings.Repeat("d", 64) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			state, err := ClaimTopologyV7Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-bad")
			if err != nil {
				t.Fatal(err)
			}
			result := validTopologyV7ZeroCallInvalid(definition, first, "claim-bad")
			mutate(&result)
			if _, err := RecordTopologyV7Observation(state, definition, result); err == nil {
				t.Fatal("malformed zero-call invalid receipt was accepted")
			}
		})
	}
}

func TestTopologyV7PaidHarnessInvalidRejectsSafetyAndExecuteEnvelopeMutations(t *testing.T) {
	definition, _ := CurrentTopologyExperimentV7()
	first := definition.ExecutionOrder[0]
	mutations := map[string]func(*TopologyExperimentResult){
		"owner intervention":   func(result *TopologyExperimentResult) { result.OwnerInterventions = 1 },
		"external side effect": func(result *TopologyExperimentResult) { result.ExternalSideEffects = 1 },
		"negative duration":    func(result *TopologyExperimentResult) { result.EndToEndMillis = -1 },
		"malformed hash":       func(result *TopologyExperimentResult) { result.TaskSHA256 = "bad" },
		"wrong execute run id": func(result *TopologyExperimentResult) { result.StageReceipts[0].RunID = "wrong" },
		"execute correction":   func(result *TopologyExperimentResult) { result.StageReceipts[0].Correction = true },
		"negative stage time":  func(result *TopologyExperimentResult) { result.StageReceipts[0].WallTimeMillis = -1 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			state, err := ClaimTopologyV7Observation(TopologyCampaignState{}, definition, first.ObservationID, "claim-paid-bad")
			if err != nil {
				t.Fatal(err)
			}
			result := validTopologyV7PaidHarnessInvalid(first, "claim-paid-bad")
			mutate(&result)
			if _, err := RecordTopologyV7Observation(state, definition, result); err == nil {
				t.Fatal("malformed paid harness invalid receipt was accepted")
			}
		})
	}
}
