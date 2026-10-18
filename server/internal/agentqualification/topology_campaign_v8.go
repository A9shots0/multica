package agentqualification

import (
	"fmt"
	"reflect"
)

func NextTopologyV8Observation(state TopologyCampaignState, definition TopologyExperimentDefinition) (*TopologyExperimentObservation, error) {
	if err := ValidateTopologyExperimentV8(definition); err != nil {
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

func ClaimTopologyV8Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, observationID, claimID string) (TopologyCampaignState, error) {
	if claimID == "" || observationID == "" {
		return state, fmt.Errorf("topology v8 claim identity is incomplete")
	}
	next, err := NextTopologyV8Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != observationID {
		return state, fmt.Errorf("topology v8 observation is not currently assignable")
	}
	arm, ok := topologyArm(definition, next.ArmID)
	if !ok || state.TotalTokens+arm.TotalTokenReservation > definition.MaxTotalTokens || state.TotalCostMicros+arm.CostCeilingMicros > definition.MaxIncrementalSpendMicros {
		return state, fmt.Errorf("topology v8 prelaunch reservation exceeds campaign ceiling")
	}
	state.ActiveClaim = claimID
	state.Revision++
	return state, nil
}

func RecordTopologyV8Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, result TopologyExperimentResult) (TopologyCampaignState, error) {
	if err := ValidateTopologyExperimentV8(definition); err != nil {
		return state, err
	}
	if state.GlobalHaltReason != "" || state.ActiveClaim == "" || result.ClaimID != state.ActiveClaim {
		return state, fmt.Errorf("topology v8 result has no exclusive active claim")
	}
	for _, existing := range state.Observations {
		if existing.ObservationID == result.ObservationID {
			if reflect.DeepEqual(existing, result) {
				return state, nil
			}
			return state, fmt.Errorf("topology v8 observation was already recorded")
		}
	}
	next := definition.ExecutionOrder[len(state.Observations)]
	if result.ObservationID != next.ObservationID || result.CaseID != next.CaseID || result.Repeat != next.Repeat || result.ArmID != next.ArmID {
		return state, fmt.Errorf("topology v8 result identity drifted")
	}
	if err := validateTopologyV8Result(definition, result); err != nil {
		return state, err
	}
	for _, prior := range state.Observations {
		// R04's initial tree contains the intentionally unique run-family binding.
		// Matched-pair integrity is the canonical task plus frozen fixture contract.
		if prior.CaseID == result.CaseID && prior.Repeat == result.Repeat && prior.ArmID != result.ArmID && (prior.TaskSHA256 != result.TaskSHA256 || prior.FixtureSHA256 != result.FixtureSHA256) {
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

func validateTopologyV8Result(definition TopologyExperimentDefinition, result TopologyExperimentResult) error {
	if err := validateTopologyV8StageCaps(result); err != nil {
		return err
	}
	if result.Outcome == "invalid" && result.FailureClass == "runtime_preflight_invalid" && result.AccountingStatus == "exact" && result.ProviderCalls == 0 && result.TotalTokens == 0 && result.CostMicros == 0 && result.PostBatchCorrection != "" {
		if err := validateTopologyV8SafetyHashEnvelope(result); err != nil {
			return err
		}
		if len(result.StageReceipts) != 1 || result.StageReceipts[0].RunID != result.ClaimID+":execute" || result.StageReceipts[0].Stage != "execute" || result.StageReceipts[0].Correction || result.StageReceipts[0].ProviderCalls != 0 || result.StageReceipts[0].InputTokens != 0 || result.StageReceipts[0].OutputTokens != 0 || result.StageReceipts[0].ReasoningTokens != 0 || result.StageReceipts[0].CostMicros != 0 || result.FinalTreeSHA256 != result.InitialTreeSHA256 {
			return fmt.Errorf("topology v8 zero-call runtime preflight receipt is malformed")
		}
		return nil
	}
	if result.Outcome == "invalid" && result.FailureClass == "transport_or_harness_imperfection" && (result.AccountingStatus == "exact" || result.AccountingStatus == "ambiguous") && result.PostBatchCorrection != "" {
		if err := validateTopologyV8SafetyHashEnvelope(result); err != nil {
			return err
		}
		arm, ok := topologyArm(definition, result.ArmID)
		if !ok || len(result.StageReceipts) == 0 || result.StageReceipts[0].RunID != result.ClaimID+":execute" || result.StageReceipts[0].Stage != "execute" || result.StageReceipts[0].Correction || result.ProviderCalls < 1 || result.ProviderCalls > arm.MaxProviderTurns || result.TotalTokens > arm.TotalTokenReservation || result.CostMicros > arm.CostCeilingMicros {
			return fmt.Errorf("topology v8 paid harness failure boundary is invalid")
		}
		providerCalls, totalTokens, costMicros := 0, int64(0), int64(0)
		for _, stage := range result.StageReceipts {
			if stage.RunID == "" || stage.ProviderCalls < 0 || stage.InputTokens < 0 || stage.OutputTokens < 0 || stage.ReasoningTokens < 0 || stage.CostMicros < 0 || stage.WallTimeMillis < 0 || stage.Correction != (stage.Stage == "correction") {
				return fmt.Errorf("topology v8 paid harness failure receipt is invalid")
			}
			providerCalls += stage.ProviderCalls
			totalTokens += stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
			costMicros += stage.CostMicros
		}
		if providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || costMicros != result.CostMicros {
			return fmt.Errorf("topology v8 paid harness failure accounting is incomplete")
		}
		return nil
	}
	if result.Outcome != "fail" || result.FailureClass != "response_or_tool_protocol_failure" {
		return validateTopologyResult(definition, result)
	}
	if result.AccountingStatus != "exact" || result.ProviderCalls < 1 || result.TotalTokens < 1 || result.OwnerInterventions != 0 || result.ExternalSideEffects != 0 {
		return fmt.Errorf("topology v8 protocol failure accounting is invalid")
	}
	for _, hash := range []string{result.InitialTreeSHA256, result.TaskSHA256, result.FixtureSHA256, result.FinalTreeSHA256, result.HiddenGradeSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("topology v8 protocol failure evidence hash is invalid")
		}
	}
	arm, ok := topologyArm(definition, result.ArmID)
	if !ok || result.ProviderCalls > arm.MaxProviderTurns || result.TotalTokens > arm.TotalTokenReservation || result.CostMicros > arm.CostCeilingMicros {
		return fmt.Errorf("topology v8 protocol failure exceeded its arm or stage boundary")
	}
	wantStages := []string{"execute"}
	if len(result.StageReceipts) == 3 {
		wantStages = []string{"execute", "review_1", "correction"}
	} else if len(result.StageReceipts) != 1 {
		return fmt.Errorf("topology v8 protocol failure invoked an unexpected review stage")
	}
	providerCalls, totalTokens, costMicros := 0, int64(0), int64(0)
	for index, stage := range result.StageReceipts {
		if stage.Stage != wantStages[index] || stage.RunID == "" || stage.ProviderCalls < 0 || stage.InputTokens < 0 || stage.OutputTokens < 0 || stage.ReasoningTokens < 0 || stage.CostMicros < 0 || stage.Correction != (stage.Stage == "correction") {
			return fmt.Errorf("topology v8 protocol failure receipt is incomplete")
		}
		providerCalls += stage.ProviderCalls
		totalTokens += stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
		costMicros += stage.CostMicros
	}
	if providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || costMicros != result.CostMicros {
		return fmt.Errorf("topology v8 protocol failure receipt is incomplete")
	}
	return nil
}

func validateTopologyV8StageCaps(result TopologyExperimentResult) error {
	exhausted := false
	for _, stage := range result.StageReceipts {
		total := stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
		generated := stage.OutputTokens + stage.ReasoningTokens
		switch stage.Stage {
		case "execute":
			over := stage.ProviderCalls > 15 || total > 60000 || generated > 12000 || stage.CostMicros > 250000
			if stage.Correction || over && !(result.Outcome == "invalid" && result.FailureClass == "stage_budget_exhaustion") {
				return fmt.Errorf("topology v8 execute stage exceeded its observation-local cap")
			}
			exhausted = exhausted || over || stage.ProviderCalls == 15
		case "review_1", "review_2":
			zeroCallInvalid := result.Outcome == "invalid" && result.FailureClass == "transport_or_harness_imperfection" && stage.ProviderCalls == 0 && total == 0 && stage.CostMicros == 0
			over := stage.ProviderCalls > 1 || total > 20000 || generated > 5000 || stage.CostMicros > 100000
			if stage.Correction || !zeroCallInvalid && stage.ProviderCalls != 1 && !over || over && !(result.Outcome == "invalid" && result.FailureClass == "stage_budget_exhaustion") {
				return fmt.Errorf("topology v8 review stage exceeded its exact one-call cap")
			}
			exhausted = exhausted || over
		case "correction":
			over := stage.ProviderCalls > 8 || total > 40000 || generated > 10000 || stage.CostMicros > 200000
			if !stage.Correction || over && !(result.Outcome == "invalid" && result.FailureClass == "stage_budget_exhaustion") {
				return fmt.Errorf("topology v8 correction stage exceeded its observation-local cap")
			}
			exhausted = exhausted || over || stage.ProviderCalls == 8
		case "hidden_grade":
			if stage.Correction || stage.ProviderCalls != 0 || total != 0 || stage.CostMicros != 0 {
				return fmt.Errorf("topology v8 hidden grade has provider accounting")
			}
		default:
			return fmt.Errorf("topology v8 stage is not allowed")
		}
	}
	if result.Outcome == "invalid" && result.FailureClass == "stage_budget_exhaustion" && !exhausted {
		return fmt.Errorf("topology v8 stage exhaustion did not cross a stage cap")
	}
	return nil
}

func validateTopologyV8SafetyHashEnvelope(result TopologyExperimentResult) error {
	if result.OwnerInterventions != 0 || result.ExternalSideEffects != 0 || result.EndToEndMillis < 0 {
		return fmt.Errorf("topology v8 result safety envelope is invalid")
	}
	for _, hash := range []string{result.InitialTreeSHA256, result.TaskSHA256, result.FixtureSHA256, result.FinalTreeSHA256, result.HiddenGradeSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("topology v8 result evidence hash is invalid")
		}
	}
	return nil
}
