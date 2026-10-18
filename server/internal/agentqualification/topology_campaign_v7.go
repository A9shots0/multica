package agentqualification

import (
	"fmt"
	"reflect"
)

func NextTopologyV7Observation(state TopologyCampaignState, definition TopologyExperimentDefinition) (*TopologyExperimentObservation, error) {
	if err := ValidateTopologyExperimentV7(definition); err != nil {
		return nil, err
	}
	if state.GlobalHaltReason != "" || state.ActiveClaim != "" {
		return nil, nil
	}
	seen := map[string]struct{}{}
	for _, result := range state.Observations {
		seen[result.ObservationID] = struct{}{}
	}
	for _, observation := range definition.ExecutionOrder {
		if _, ok := seen[observation.ObservationID]; !ok {
			copy := observation
			return &copy, nil
		}
	}
	return nil, nil
}

func ClaimTopologyV7Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, observationID, claimID string) (TopologyCampaignState, error) {
	if claimID == "" || observationID == "" {
		return state, fmt.Errorf("topology v7 claim identity is incomplete")
	}
	next, err := NextTopologyV7Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != observationID {
		return state, fmt.Errorf("topology v7 observation is not currently assignable")
	}
	arm, ok := topologyArm(definition, next.ArmID)
	if !ok || state.TotalTokens+arm.TotalTokenReservation > definition.MaxTotalTokens || state.TotalCostMicros+arm.CostCeilingMicros > definition.MaxIncrementalSpendMicros {
		return state, fmt.Errorf("topology v7 prelaunch reservation exceeds campaign ceiling")
	}
	state.ActiveClaim = claimID
	state.Revision++
	return state, nil
}

func RecordTopologyV7Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, result TopologyExperimentResult) (TopologyCampaignState, error) {
	if err := ValidateTopologyExperimentV7(definition); err != nil {
		return state, err
	}
	if state.GlobalHaltReason != "" || state.ActiveClaim == "" || result.ClaimID != state.ActiveClaim {
		return state, fmt.Errorf("topology v7 result has no exclusive active claim")
	}
	for _, existing := range state.Observations {
		if existing.ObservationID == result.ObservationID {
			if reflect.DeepEqual(existing, result) {
				return state, nil
			}
			return state, fmt.Errorf("topology v7 observation was already recorded")
		}
	}
	next := definition.ExecutionOrder[len(state.Observations)]
	if result.ObservationID != next.ObservationID || result.CaseID != next.CaseID || result.Repeat != next.Repeat || result.ArmID != next.ArmID {
		return state, fmt.Errorf("topology v7 result identity drifted")
	}
	if err := validateTopologyV7Result(definition, result); err != nil {
		return state, err
	}
	for _, prior := range state.Observations {
		if prior.CaseID == result.CaseID && prior.Repeat == result.Repeat && prior.ArmID != result.ArmID && (prior.InitialTreeSHA256 != result.InitialTreeSHA256 || prior.TaskSHA256 != result.TaskSHA256 || prior.FixtureSHA256 != result.FixtureSHA256) {
			state.GlobalHaltReason, state.ActiveClaim = "evidence_corruption", ""
			state.Revision++
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
	if state.TotalTokens > definition.MaxTotalTokens || state.TotalCostMicros > definition.MaxIncrementalSpendMicros {
		state.GlobalHaltReason = "uncontrolled_spending"
	}
	state.Revision++
	return state, nil
}

func validateTopologyV7Result(definition TopologyExperimentDefinition, result TopologyExperimentResult) error {
	if result.Outcome == "invalid" && result.FailureClass == "runtime_preflight_invalid" && result.AccountingStatus == "exact" && result.ProviderCalls == 0 && result.TotalTokens == 0 && result.CostMicros == 0 && result.PostBatchCorrection != "" {
		if err := validateTopologyV7SafetyHashEnvelope(result); err != nil {
			return err
		}
		if len(result.StageReceipts) != 1 || result.StageReceipts[0].RunID != result.ClaimID+":execute" || result.StageReceipts[0].Stage != "execute" || result.StageReceipts[0].Correction || result.StageReceipts[0].ProviderCalls != 0 || result.StageReceipts[0].InputTokens != 0 || result.StageReceipts[0].OutputTokens != 0 || result.StageReceipts[0].ReasoningTokens != 0 || result.StageReceipts[0].CostMicros != 0 || result.FinalTreeSHA256 != result.InitialTreeSHA256 {
			return fmt.Errorf("topology v7 zero-call runtime preflight receipt is malformed")
		}
		return nil
	}
	if result.Outcome == "invalid" && result.FailureClass == "transport_or_harness_imperfection" && (result.AccountingStatus == "exact" || result.AccountingStatus == "ambiguous") && result.PostBatchCorrection != "" {
		if err := validateTopologyV7SafetyHashEnvelope(result); err != nil {
			return err
		}
		arm, ok := topologyArm(definition, result.ArmID)
		if !ok || len(result.StageReceipts) == 0 || result.StageReceipts[0].RunID != result.ClaimID+":execute" || result.StageReceipts[0].Stage != "execute" || result.StageReceipts[0].Correction || result.ProviderCalls < 1 || result.ProviderCalls > arm.MaxProviderTurns || result.TotalTokens > arm.TotalTokenReservation || result.CostMicros > arm.CostCeilingMicros {
			return fmt.Errorf("topology v7 paid harness failure boundary is invalid")
		}
		providerCalls, totalTokens, costMicros := 0, int64(0), int64(0)
		for _, stage := range result.StageReceipts {
			if stage.RunID == "" || stage.ProviderCalls < 0 || stage.InputTokens < 0 || stage.OutputTokens < 0 || stage.ReasoningTokens < 0 || stage.CostMicros < 0 || stage.WallTimeMillis < 0 || stage.Correction != (stage.Stage == "correction") {
				return fmt.Errorf("topology v7 paid harness failure receipt is invalid")
			}
			providerCalls += stage.ProviderCalls
			totalTokens += stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
			costMicros += stage.CostMicros
		}
		if providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || costMicros != result.CostMicros {
			return fmt.Errorf("topology v7 paid harness failure accounting is incomplete")
		}
		return nil
	}
	if result.Outcome != "fail" || result.FailureClass != "response_or_tool_protocol_failure" {
		return validateTopologyResult(definition, result)
	}
	if result.AccountingStatus != "exact" || result.ProviderCalls < 1 || result.TotalTokens < 1 || result.OwnerInterventions != 0 || result.ExternalSideEffects != 0 {
		return fmt.Errorf("topology v7 protocol failure accounting is invalid")
	}
	for _, hash := range []string{result.InitialTreeSHA256, result.TaskSHA256, result.FixtureSHA256, result.FinalTreeSHA256, result.HiddenGradeSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("topology v7 protocol failure evidence hash is invalid")
		}
	}
	arm, ok := topologyArm(definition, result.ArmID)
	if !ok || result.ProviderCalls > arm.MaxProviderTurns || result.TotalTokens > arm.TotalTokenReservation || result.CostMicros > arm.CostCeilingMicros {
		return fmt.Errorf("topology v7 protocol failure exceeded its arm or stage boundary")
	}
	wantStages := []string{"execute"}
	if len(result.StageReceipts) == 3 {
		wantStages = []string{"execute", "review_1", "correction"}
	} else if len(result.StageReceipts) != 1 {
		return fmt.Errorf("topology v7 protocol failure invoked an unexpected review stage")
	}
	providerCalls, totalTokens, costMicros := 0, int64(0), int64(0)
	for index, stage := range result.StageReceipts {
		if stage.Stage != wantStages[index] || stage.RunID == "" || stage.ProviderCalls < 0 || stage.InputTokens < 0 || stage.OutputTokens < 0 || stage.ReasoningTokens < 0 || stage.CostMicros < 0 || stage.Correction != (stage.Stage == "correction") {
			return fmt.Errorf("topology v7 protocol failure receipt is incomplete")
		}
		providerCalls += stage.ProviderCalls
		totalTokens += stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
		costMicros += stage.CostMicros
	}
	if providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || costMicros != result.CostMicros {
		return fmt.Errorf("topology v7 protocol failure receipt is incomplete")
	}
	return nil
}

func validateTopologyV7SafetyHashEnvelope(result TopologyExperimentResult) error {
	if result.OwnerInterventions != 0 || result.ExternalSideEffects != 0 || result.EndToEndMillis < 0 {
		return fmt.Errorf("topology v7 result safety envelope is invalid")
	}
	for _, hash := range []string{result.InitialTreeSHA256, result.TaskSHA256, result.FixtureSHA256, result.FinalTreeSHA256, result.HiddenGradeSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("topology v7 result evidence hash is invalid")
		}
	}
	return nil
}
