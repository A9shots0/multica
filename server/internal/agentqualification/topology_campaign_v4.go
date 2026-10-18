package agentqualification

import (
	"fmt"
	"reflect"
)

func NextTopologyV4Observation(state TopologyCampaignState, definition TopologyExperimentDefinition) (*TopologyExperimentObservation, error) {
	if err := ValidateTopologyExperimentV4(definition); err != nil {
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

func ClaimTopologyV4Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, observationID, claimID string) (TopologyCampaignState, error) {
	if claimID == "" || observationID == "" {
		return state, fmt.Errorf("topology v4 claim identity is incomplete")
	}
	next, err := NextTopologyV4Observation(state, definition)
	if err != nil || next == nil || next.ObservationID != observationID {
		return state, fmt.Errorf("topology v4 observation is not currently assignable")
	}
	arm, ok := topologyArm(definition, next.ArmID)
	if !ok || state.TotalTokens+arm.TotalTokenReservation > definition.MaxTotalTokens || state.TotalCostMicros+arm.CostCeilingMicros > definition.MaxIncrementalSpendMicros {
		return state, fmt.Errorf("topology v4 prelaunch reservation exceeds campaign ceiling")
	}
	state.ActiveClaim = claimID
	state.Revision++
	return state, nil
}

func RecordTopologyV4Observation(state TopologyCampaignState, definition TopologyExperimentDefinition, result TopologyExperimentResult) (TopologyCampaignState, error) {
	if err := ValidateTopologyExperimentV4(definition); err != nil {
		return state, err
	}
	if state.GlobalHaltReason != "" || state.ActiveClaim == "" || result.ClaimID != state.ActiveClaim {
		return state, fmt.Errorf("topology v4 result has no exclusive active claim")
	}
	for _, existing := range state.Observations {
		if existing.ObservationID == result.ObservationID {
			if reflect.DeepEqual(existing, result) {
				return state, nil
			}
			return state, fmt.Errorf("topology v4 observation was already recorded")
		}
	}
	next := definition.ExecutionOrder[len(state.Observations)]
	if result.ObservationID != next.ObservationID || result.CaseID != next.CaseID || result.Repeat != next.Repeat || result.ArmID != next.ArmID {
		return state, fmt.Errorf("topology v4 result identity drifted")
	}
	if err := validateTopologyResult(definition, result); err != nil {
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
