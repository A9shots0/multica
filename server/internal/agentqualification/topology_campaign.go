package agentqualification

import (
	"fmt"
	"reflect"
)

type TopologyStageReceipt struct {
	RunID           string `json:"run_id"`
	Stage           string `json:"stage"`
	ProviderCalls   int    `json:"provider_calls"`
	InputTokens     int64  `json:"input_tokens"`
	OutputTokens    int64  `json:"output_tokens"`
	ReasoningTokens int64  `json:"reasoning_tokens"`
	CostMicros      int64  `json:"cost_micros"`
	WallTimeMillis  int64  `json:"wall_time_millis"`
	Correction      bool   `json:"correction"`
}

type TopologyExperimentResult struct {
	ClaimID             string                 `json:"claim_id"`
	ObservationID       string                 `json:"observation_id"`
	CaseID              string                 `json:"case_id"`
	Repeat              int                    `json:"repeat"`
	ArmID               string                 `json:"arm_id"`
	Outcome             string                 `json:"outcome"`
	FailureClass        string                 `json:"failure_class,omitempty"`
	InitialTreeSHA256   string                 `json:"initial_tree_sha256"`
	TaskSHA256          string                 `json:"task_sha256"`
	FixtureSHA256       string                 `json:"fixture_sha256"`
	FinalTreeSHA256     string                 `json:"final_tree_sha256"`
	HiddenGradeSHA256   string                 `json:"hidden_grade_sha256"`
	StageReceipts       []TopologyStageReceipt `json:"stage_receipts"`
	ProviderCalls       int                    `json:"provider_calls"`
	TotalTokens         int64                  `json:"total_tokens"`
	CostMicros          int64                  `json:"cost_micros"`
	EndToEndMillis      int64                  `json:"end_to_end_millis"`
	OwnerInterventions  int                    `json:"owner_interventions"`
	ExternalSideEffects int                    `json:"external_side_effects"`
	PostBatchCorrection string                 `json:"post_batch_correction,omitempty"`
	AccountingStatus    string                 `json:"accounting_status"`
}

type TopologyCampaignState struct {
	Revision                    int64                      `json:"revision"`
	Observations                []TopologyExperimentResult `json:"observations"`
	ActiveClaim                 string                     `json:"active_claim,omitempty"`
	GlobalHaltReason            string                     `json:"global_halt_reason,omitempty"`
	TotalProviderCalls          int                        `json:"total_provider_calls"`
	TotalTokens                 int64                      `json:"total_tokens"`
	TotalCostMicros             int64                      `json:"total_cost_micros"`
	PostBatchCorrections        []string                   `json:"post_batch_corrections,omitempty"`
	BaselineProviderSpendMicros int64                      `json:"baseline_provider_spend_micros"`
	SpendAccountingStatus       string                     `json:"spend_accounting_status,omitempty"`
}

func NextTopologyObservation(state TopologyCampaignState, definition TopologyExperimentDefinition) (*TopologyExperimentObservation, error) {
	if err := ValidateTopologyExperiment(definition); err != nil {
		return nil, err
	}
	if state.GlobalHaltReason != "" || state.ActiveClaim != "" {
		return nil, nil
	}
	seen := map[string]struct{}{}
	for _, observation := range state.Observations {
		seen[observation.ObservationID] = struct{}{}
	}
	for _, observation := range definition.ExecutionOrder {
		if _, ok := seen[observation.ObservationID]; !ok {
			copyObservation := observation
			return &copyObservation, nil
		}
	}
	return nil, nil
}

func ClaimTopologyObservation(state TopologyCampaignState, definition TopologyExperimentDefinition, observationID, claimID string) (TopologyCampaignState, error) {
	if claimID == "" || observationID == "" {
		return state, fmt.Errorf("topology claim identity is incomplete")
	}
	next, err := NextTopologyObservation(state, definition)
	if err != nil {
		return state, err
	}
	if next == nil || next.ObservationID != observationID {
		return state, fmt.Errorf("topology observation is not currently assignable")
	}
	arm, ok := topologyArm(definition, next.ArmID)
	if !ok {
		return state, fmt.Errorf("topology arm is missing")
	}
	if state.TotalTokens+arm.TotalTokenReservation > definition.MaxTotalTokens || state.TotalCostMicros+arm.CostCeilingMicros > definition.MaxIncrementalSpendMicros {
		return state, fmt.Errorf("topology prelaunch reservation exceeds campaign ceiling")
	}
	state.ActiveClaim = claimID
	state.Revision++
	return state, nil
}

func RecordTopologyObservation(state TopologyCampaignState, definition TopologyExperimentDefinition, result TopologyExperimentResult) (TopologyCampaignState, error) {
	if state.GlobalHaltReason != "" {
		return state, fmt.Errorf("topology campaign is halted")
	}
	if state.ActiveClaim == "" || result.ClaimID != state.ActiveClaim {
		return state, fmt.Errorf("topology result has no active claim")
	}
	for _, existing := range state.Observations {
		if existing.ObservationID != result.ObservationID {
			continue
		}
		if reflect.DeepEqual(existing, result) {
			return state, nil
		}
		return state, fmt.Errorf("topology observation was already recorded")
	}
	next := definition.ExecutionOrder[len(state.Observations)]
	if result.ObservationID != next.ObservationID || result.CaseID != next.CaseID || result.Repeat != next.Repeat || result.ArmID != next.ArmID {
		return state, fmt.Errorf("topology result identity drifted")
	}
	if err := validateTopologyResult(definition, result); err != nil {
		return state, err
	}
	for _, prior := range state.Observations {
		if prior.CaseID == result.CaseID && prior.Repeat == result.Repeat && prior.ArmID != result.ArmID {
			if prior.InitialTreeSHA256 != result.InitialTreeSHA256 || prior.TaskSHA256 != result.TaskSHA256 || prior.FixtureSHA256 != result.FixtureSHA256 {
				state.GlobalHaltReason = "evidence_corruption"
				state.ActiveClaim = ""
				state.Revision++
				return state, nil
			}
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

func validateTopologyResult(definition TopologyExperimentDefinition, result TopologyExperimentResult) error {
	if !contains([]string{"pass", "fail", "invalid", "global_halt"}, result.Outcome) || !contains([]string{"exact", "ambiguous"}, result.AccountingStatus) || result.ProviderCalls < 0 || result.TotalTokens < 0 || result.CostMicros < 0 || result.EndToEndMillis < 0 || result.OwnerInterventions != 0 || result.ExternalSideEffects != 0 {
		return fmt.Errorf("topology result outcome or accounting is invalid")
	}
	for _, hash := range []string{result.InitialTreeSHA256, result.TaskSHA256, result.FixtureSHA256, result.FinalTreeSHA256, result.HiddenGradeSHA256} {
		if !validSHA256(hash) {
			return fmt.Errorf("topology result evidence hash is invalid")
		}
	}
	arm, ok := topologyArm(definition, result.ArmID)
	if !ok {
		return fmt.Errorf("topology result arm is invalid")
	}
	if result.Outcome == "global_halt" && result.FailureClass == "credential_failure" && result.ProviderCalls == 0 && len(result.StageReceipts) == 0 {
		return nil
	}
	if result.Outcome == "invalid" && result.AccountingStatus == "ambiguous" && result.ProviderCalls == 0 && result.TotalTokens == 0 && len(result.StageReceipts) == 0 && result.PostBatchCorrection != "" {
		return nil
	}
	if result.Outcome == "invalid" && result.FailureClass == "stage_budget_exhaustion" && result.AccountingStatus == "exact" && result.PostBatchCorrection != "" {
		providerCalls, totalTokens, cost := 0, int64(0), int64(0)
		if len(result.StageReceipts) == 0 {
			return fmt.Errorf("stage budget exhaustion has no sealed stage receipt")
		}
		for _, receipt := range result.StageReceipts {
			if receipt.RunID == "" || receipt.ProviderCalls < 0 || receipt.InputTokens < 0 || receipt.OutputTokens < 0 || receipt.ReasoningTokens < 0 || receipt.CostMicros < 0 {
				return fmt.Errorf("stage budget exhaustion receipt is invalid")
			}
			providerCalls += receipt.ProviderCalls
			totalTokens += receipt.InputTokens + receipt.OutputTokens + receipt.ReasoningTokens
			cost += receipt.CostMicros
		}
		if providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || cost != result.CostMicros {
			return fmt.Errorf("stage budget exhaustion accounting is incomplete")
		}
		return nil
	}
	if result.AccountingStatus != "exact" {
		return fmt.Errorf("completed topology observation accounting is not exact")
	}
	if result.Outcome != "global_halt" && (result.ProviderCalls < 1 || result.TotalTokens < 1 || result.ProviderCalls > arm.MaxProviderTurns || result.TotalTokens > arm.TotalTokenReservation || result.CostMicros > arm.CostCeilingMicros) {
		return fmt.Errorf("topology result exceeded its arm budget")
	}
	providerCalls := 0
	totalTokens := int64(0)
	cost := int64(0)
	stages := map[string]int{}
	corrections := 0
	for _, receipt := range result.StageReceipts {
		if receipt.RunID == "" || !contains([]string{"execute", "review_1", "correction", "review_2", "hidden_grade"}, receipt.Stage) || receipt.ProviderCalls < 0 || receipt.InputTokens < 0 || receipt.OutputTokens < 0 || receipt.ReasoningTokens < 0 || receipt.CostMicros < 0 || receipt.WallTimeMillis < 0 {
			return fmt.Errorf("topology stage receipt is invalid")
		}
		if stages[receipt.Stage] > 0 {
			return fmt.Errorf("topology stage receipt is duplicated")
		}
		stages[receipt.Stage]++
		providerCalls += receipt.ProviderCalls
		totalTokens += receipt.InputTokens + receipt.OutputTokens + receipt.ReasoningTokens
		cost += receipt.CostMicros
		if receipt.Correction {
			corrections++
		}
	}
	if result.Outcome == "global_halt" {
		if !contains(definition.GlobalHaltReasons, result.FailureClass) || providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || cost != result.CostMicros {
			return fmt.Errorf("topology global halt evidence is incomplete")
		}
		return nil
	}
	if providerCalls != result.ProviderCalls || totalTokens != result.TotalTokens || cost != result.CostMicros || stages["execute"] != 1 || stages["hidden_grade"] != 1 || stages["hidden_grade"] == 1 && result.StageReceipts[len(result.StageReceipts)-1].Stage != "hidden_grade" {
		return fmt.Errorf("topology result stage accounting is incomplete")
	}
	if arm.Reviewed {
		if stages["review_1"] != 1 || corrections > definition.MaxCorrections || stages["correction"] != corrections || stages["review_2"] != corrections {
			return fmt.Errorf("reviewed topology result omitted review or exceeded correction ceiling")
		}
	} else if stages["review_1"] != 0 || stages["review_2"] != 0 || stages["correction"] != 0 || corrections != 0 {
		return fmt.Errorf("direct topology result received hidden review work")
	}
	if result.Outcome == "invalid" && result.PostBatchCorrection == "" {
		return fmt.Errorf("invalid topology result lacks post-batch correction record")
	}
	return nil
}

func topologyArm(definition TopologyExperimentDefinition, armID string) (TopologyExperimentArm, bool) {
	for _, arm := range definition.Arms {
		if arm.ArmID == armID {
			return arm, true
		}
	}
	return TopologyExperimentArm{}, false
}
