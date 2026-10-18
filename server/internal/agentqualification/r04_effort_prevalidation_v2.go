package agentqualification

import "fmt"

type R04EffortV2Prevalidation struct {
	ExecutorPlanSHA256     string `json:"executor_plan_sha256"`
	ExecutorResolvedSHA256 string `json:"executor_resolved_sha256"`
	ProviderCalls          int    `json:"provider_calls"`
}

func PrevalidateR04EffortV2Routes(definition TopologyExperimentDefinition) (R04EffortV2Prevalidation, error) {
	var proof R04EffortV2Prevalidation
	if err := ValidateR04EffortExperimentV2(definition); err != nil {
		return proof, err
	}
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return proof, err
	}
	request := RealisticProviderRequest{ProviderCallID: "r04-effort-v2-preclaim", ProviderTurn: 1, CaseID: "R04", CaseVersion: "v1", RunFamilyID: "r04-effort-v2-preclaim", Attempt: 2, Task: "provider-free route validation", WorkspacePaths: []string{"TASK.md"}, AllowedActions: []string{"read", "search", "edit", "run_check", "checkpoint_and_interrupt", "finish"}, MaxSteps: 15}
	plan, err := BuildOpenCodeR04EffortActionPlanV2(definition.Executor, definition.Executor, request)
	if err != nil {
		return proof, err
	}
	if err := ValidateR04EffortV2PlanBinding(definition.Executor, plan, OpenCodeR04EffortActionTransportV2); err != nil {
		return proof, err
	}
	resolved, err := ValidateR04EffortV2ResolvedConfig(plan, plan.InlineConfig)
	if err != nil {
		return proof, err
	}
	proof.ExecutorPlanSHA256, proof.ExecutorResolvedSHA256 = plan.InlineConfigSHA256, resolved
	if !validSHA256(proof.ExecutorPlanSHA256) || !validSHA256(proof.ExecutorResolvedSHA256) {
		return R04EffortV2Prevalidation{}, fmt.Errorf("R04 effort v1 prevalidation hashes are incomplete")
	}
	return proof, nil
}
