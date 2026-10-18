package agentqualification

import (
	"strings"
	"testing"
)

func validTopologyResult(definition TopologyExperimentDefinition, observation TopologyExperimentObservation, claimID string) TopologyExperimentResult {
	hash := strings.Repeat("a", 64)
	stages := []TopologyStageReceipt{{RunID: observation.ObservationID + "-execute", Stage: "execute", ProviderCalls: 1, InputTokens: 10, OutputTokens: 2, CostMicros: 1}}
	if arm, _ := topologyArm(definition, observation.ArmID); arm.Reviewed {
		stages = append(stages, TopologyStageReceipt{RunID: observation.ObservationID + "-review", Stage: "review_1", ProviderCalls: 1, InputTokens: 5, OutputTokens: 1, CostMicros: 1})
	}
	stages = append(stages, TopologyStageReceipt{RunID: observation.ObservationID + "-grade", Stage: "hidden_grade"})
	calls, tokens, cost := 0, int64(0), int64(0)
	for _, stage := range stages {
		calls += stage.ProviderCalls
		tokens += stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
		cost += stage.CostMicros
	}
	return TopologyExperimentResult{ClaimID: claimID, ObservationID: observation.ObservationID, CaseID: observation.CaseID, Repeat: observation.Repeat, ArmID: observation.ArmID, Outcome: "pass", InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash, StageReceipts: stages, ProviderCalls: calls, TotalTokens: tokens, CostMicros: cost, AccountingStatus: "exact"}
}

func TestTopologyCampaignRunsCounterbalancedOrderAndContinuesAfterArmFailure(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	state := TopologyCampaignState{}
	for index, observation := range definition.ExecutionOrder {
		claimID := "claim-" + observation.ObservationID
		var err error
		state, err = ClaimTopologyObservation(state, definition, observation.ObservationID, claimID)
		if err != nil {
			t.Fatalf("claim %d failed: %v", index, err)
		}
		result := validTopologyResult(definition, observation, claimID)
		if index == 0 {
			result.Outcome = "fail"
			result.FailureClass = "competence_failure"
		}
		state, err = RecordTopologyObservation(state, definition, result)
		if err != nil {
			t.Fatalf("record %d failed: %v", index, err)
		}
	}
	if len(state.Observations) != 6 || state.GlobalHaltReason != "" {
		t.Fatalf("candidate-local failure halted unrelated comparisons: %+v", state)
	}
}

func TestTopologyCampaignPreservesInvalidAndContinues(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	first := definition.ExecutionOrder[0]
	state, _ := ClaimTopologyObservation(TopologyCampaignState{}, definition, first.ObservationID, "claim-invalid")
	result := validTopologyResult(definition, first, "claim-invalid")
	result.Outcome = "invalid"
	result.FailureClass = "grader"
	result.PostBatchCorrection = "phrase matcher rejected truthful machine field"
	state, err := RecordTopologyObservation(state, definition, result)
	if err != nil {
		t.Fatal(err)
	}
	next, err := NextTopologyObservation(state, definition)
	if err != nil || next == nil || next.ObservationID != definition.ExecutionOrder[1].ObservationID || len(state.PostBatchCorrections) != 1 {
		t.Fatalf("invalid observation stopped unaffected work: state=%+v next=%+v err=%v", state, next, err)
	}
}

func TestTopologyCampaignFailsClosedOnMismatchedPairOrHiddenUsage(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	first, second := definition.ExecutionOrder[0], definition.ExecutionOrder[1]
	state, _ := ClaimTopologyObservation(TopologyCampaignState{}, definition, first.ObservationID, "claim-1")
	state, _ = RecordTopologyObservation(state, definition, validTopologyResult(definition, first, "claim-1"))
	state, _ = ClaimTopologyObservation(state, definition, second.ObservationID, "claim-2")
	mismatch := validTopologyResult(definition, second, "claim-2")
	mismatch.InitialTreeSHA256 = strings.Repeat("b", 64)
	state, err := RecordTopologyObservation(state, definition, mismatch)
	if err != nil || state.GlobalHaltReason != "evidence_corruption" {
		t.Fatalf("mismatched pair did not halt corrupted evidence: state=%+v err=%v", state, err)
	}

	reviewed := validTopologyResult(definition, second, "claim-2")
	reviewed.ProviderCalls++
	if err := validateTopologyResult(definition, reviewed); err == nil {
		t.Fatal("unreceipted provider work was accepted")
	}
}

func TestTopologyCampaignAllowsOnlyNamedGlobalStops(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	observation := definition.ExecutionOrder[0]
	result := validTopologyResult(definition, observation, "claim")
	result.Outcome = "global_halt"
	result.FailureClass = "competence_failure"
	if err := validateTopologyResult(definition, result); err == nil {
		t.Fatal("candidate failure became a global halt")
	}
	result.FailureClass = "unsafe_external_action"
	if err := validateTopologyResult(definition, result); err != nil {
		t.Fatalf("named safety halt was rejected: %v", err)
	}
}

func TestTopologyCampaignPreservesZeroCallHarnessInvalidAndContinues(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	first := definition.ExecutionOrder[0]
	state, err := ClaimTopologyObservation(TopologyCampaignState{}, definition, first.ObservationID, "claim-zero")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	result := TopologyExperimentResult{ClaimID: "claim-zero", ObservationID: first.ObservationID, CaseID: first.CaseID, Repeat: first.Repeat, ArmID: first.ArmID, Outcome: "invalid", FailureClass: "transport_or_harness_imperfection", InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash, PostBatchCorrection: "provider preflight transport failed", AccountingStatus: "ambiguous"}
	state, err = RecordTopologyObservation(state, definition, result)
	if err != nil {
		t.Fatal(err)
	}
	next, err := NextTopologyObservation(state, definition)
	if err != nil || next == nil || next.ObservationID != definition.ExecutionOrder[1].ObservationID {
		t.Fatalf("unaffected observation did not continue: next=%+v err=%v", next, err)
	}
}

func TestTopologyCampaignAllowsZeroCallCredentialHaltOnly(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	first := definition.ExecutionOrder[0]
	state, _ := ClaimTopologyObservation(TopologyCampaignState{}, definition, first.ObservationID, "claim-credential")
	hash := strings.Repeat("b", 64)
	result := TopologyExperimentResult{ClaimID: "claim-credential", ObservationID: first.ObservationID, CaseID: first.CaseID, Repeat: first.Repeat, ArmID: first.ArmID, Outcome: "global_halt", FailureClass: "credential_failure", InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash, AccountingStatus: "ambiguous"}
	state, err := RecordTopologyObservation(state, definition, result)
	if err != nil || state.GlobalHaltReason != "credential_failure" {
		t.Fatalf("credential halt was not preserved: state=%+v err=%v", state, err)
	}
}

func TestTopologyCampaignAcceptsExactCandidateLocalStageExhaustion(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	observation := definition.ExecutionOrder[1]
	hash := strings.Repeat("c", 64)
	result := TopologyExperimentResult{
		ClaimID: "claim", ObservationID: observation.ObservationID, CaseID: observation.CaseID,
		Repeat: observation.Repeat, ArmID: observation.ArmID, Outcome: "invalid",
		FailureClass: "stage_budget_exhaustion", InitialTreeSHA256: hash, TaskSHA256: hash,
		FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts: []TopologyStageReceipt{{RunID: "claim:execute", Stage: "execute", ProviderCalls: 2, InputTokens: 59, OutputTokens: 3, CostMicros: 7}},
		ProviderCalls: 2, TotalTokens: 62, CostMicros: 7, AccountingStatus: "exact",
		PostBatchCorrection: "paid overrun response sealed; next call stopped before provider",
	}
	if err := validateTopologyResult(definition, result); err != nil {
		t.Fatalf("exact candidate-local stage exhaustion was rejected: %v", err)
	}
}
