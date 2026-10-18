package agentqualification

import (
	"fmt"
	"reflect"
)

func NextR04EffortV1Observation(state TopologyCampaignState, definition TopologyExperimentDefinition) (*TopologyExperimentObservation, error) {
	if err := ValidateR04EffortExperimentV1(definition); err != nil {
		return nil, err
	}
	if state.GlobalHaltReason != "" || state.ActiveClaim != "" {
		return nil, nil
	}
	seen := map[string]bool{}
	for _, result := range state.Observations {
		seen[result.ObservationID] = true
	}
	for _, observation := range definition.ExecutionOrder {
		if !seen[observation.ObservationID] {
			copy := observation
			return &copy, nil
		}
	}
	return nil, nil
}

func ClaimR04EffortV1Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, observationID, claimID string) (TopologyCampaignState, error) {
	if claimID == "" {
		return state, fmt.Errorf("R04 effort v1 claim identity is incomplete")
	}
	next, err := NextR04EffortV1Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != observationID {
		return state, fmt.Errorf("R04 effort v1 observation is not assignable")
	}
	arm := definition.Arms[0]
	if state.TotalProviderCalls+arm.MaxProviderTurns > 30 || state.TotalTokens+arm.TotalTokenReservation > definition.MaxTotalTokens || state.TotalCostMicros+arm.CostCeilingMicros > definition.MaxIncrementalSpendMicros {
		return state, fmt.Errorf("R04 effort v1 reservation exceeds campaign ceiling")
	}
	state.ActiveClaim, state.Revision = claimID, state.Revision+1
	return state, nil
}

func RecordR04EffortV1Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, result TopologyExperimentResult) (TopologyCampaignState, error) {
	if err := ValidateR04EffortExperimentV1(definition); err != nil {
		return state, err
	}
	if state.GlobalHaltReason != "" || state.ActiveClaim == "" || result.ClaimID != state.ActiveClaim {
		return state, fmt.Errorf("R04 effort v1 result has no exclusive claim")
	}
	for _, prior := range state.Observations {
		if prior.ObservationID == result.ObservationID {
			if reflect.DeepEqual(prior, result) {
				return state, nil
			}
			return state, fmt.Errorf("R04 effort v1 observation already recorded")
		}
	}
	next := definition.ExecutionOrder[len(state.Observations)]
	if result.ObservationID != next.ObservationID || result.CaseID != "R04" || result.Repeat != next.Repeat || result.ArmID != next.ArmID {
		return state, fmt.Errorf("R04 effort v1 result identity drifted")
	}
	if err := validateR04EffortV1Result(definition, result); err != nil {
		return state, err
	}
	for _, prior := range state.Observations {
		if prior.TaskSHA256 != result.TaskSHA256 || prior.FixtureSHA256 != result.FixtureSHA256 {
			state.GlobalHaltReason, state.ActiveClaim, state.Revision = "evidence_corruption", "", state.Revision+1
			return state, nil
		}
	}
	state.Observations = append(state.Observations, result)
	state.ActiveClaim = ""
	state.TotalProviderCalls += result.ProviderCalls
	state.TotalTokens += result.TotalTokens
	state.TotalCostMicros += result.CostMicros
	if result.Outcome == "invalid" && result.PostBatchCorrection != "" {
		state.PostBatchCorrections = append(state.PostBatchCorrections, result.ObservationID+":"+result.PostBatchCorrection)
	}
	if result.Outcome == "global_halt" {
		state.GlobalHaltReason = result.FailureClass
	}
	if state.TotalProviderCalls > 30 || state.TotalTokens > 150000 || state.TotalCostMicros > 700000 {
		state.GlobalHaltReason = "uncontrolled_spending"
	}
	state.Revision++
	return state, nil
}

func validateR04EffortV1Result(definition TopologyExperimentDefinition, result TopologyExperimentResult) error {
	if result.OwnerInterventions != 0 || result.ExternalSideEffects != 0 || result.EndToEndMillis < 0 {
		return fmt.Errorf("R04 effort v1 safety envelope is invalid")
	}
	for _, hash := range []string{result.InitialTreeSHA256, result.TaskSHA256, result.FixtureSHA256, result.FinalTreeSHA256, result.HiddenGradeSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("R04 effort v1 evidence hash is invalid")
		}
	}
	if len(result.StageReceipts) < 1 || len(result.StageReceipts) > 2 || result.StageReceipts[0].Stage != "execute" || result.StageReceipts[0].Correction {
		return fmt.Errorf("R04 effort v1 stage receipt is invalid")
	}
	if len(result.StageReceipts) == 2 && (result.StageReceipts[1].Stage != "hidden_grade" || result.StageReceipts[1].ProviderCalls != 0) {
		return fmt.Errorf("R04 effort v1 hidden grade is invalid")
	}
	providerCalls, totalTokens, costMicros := 0, int64(0), int64(0)
	for _, stage := range result.StageReceipts {
		providerCalls += stage.ProviderCalls
		totalTokens += stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
		costMicros += stage.CostMicros
	}
	if providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || costMicros != result.CostMicros {
		return fmt.Errorf("R04 effort v1 accounting is incomplete")
	}
	if result.ProviderCalls > 15 || result.TotalTokens > 75000 || result.StageReceipts[0].OutputTokens+result.StageReceipts[0].ReasoningTokens > 30000 || result.CostMicros > 350000 {
		if result.Outcome != "invalid" || result.FailureClass != "stage_budget_exhaustion" {
			return fmt.Errorf("R04 effort v1 stage cap exceeded")
		}
	}
	globalHaltAllowed := false
	for _, reason := range definition.GlobalHaltReasons {
		if result.FailureClass == reason {
			globalHaltAllowed = true
			break
		}
	}
	allowed := result.Outcome == "pass" || result.Outcome == "fail" && (result.FailureClass == "competence_failure" || result.FailureClass == "response_or_tool_protocol_failure") || result.Outcome == "invalid" && (result.FailureClass == "runtime_preflight_invalid" || result.FailureClass == "transport_or_harness_imperfection" || result.FailureClass == "stage_budget_exhaustion") || result.Outcome == "global_halt" && globalHaltAllowed
	if !allowed {
		return fmt.Errorf("R04 effort v1 outcome classification is invalid")
	}
	if result.Outcome == "invalid" && result.PostBatchCorrection == "" {
		return fmt.Errorf("R04 effort v1 invalid result lacks preservation note")
	}
	return nil
}
