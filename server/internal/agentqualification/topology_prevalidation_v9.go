package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type TopologyV9Prevalidation struct {
	ExecutorPlanSHA256     string `json:"executor_plan_sha256"`
	ExecutorResolvedSHA256 string `json:"executor_resolved_sha256"`
	ReviewerPlanSHA256     string `json:"reviewer_plan_sha256"`
	ReviewerResolvedSHA256 string `json:"reviewer_resolved_sha256"`
	ProviderCalls          int    `json:"provider_calls"`
}

func PrevalidateTopologyV9Routes(definition TopologyExperimentDefinition) (TopologyV9Prevalidation, error) {
	var proof TopologyV9Prevalidation
	if err := ValidateTopologyExperimentV9(definition); err != nil {
		return proof, err
	}
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return proof, err
	}
	executorRequest := RealisticProviderRequest{ProviderCallID: "v9-preclaim-executor", ProviderTurn: 1, CaseID: "R04", CaseVersion: "v1", RunFamilyID: "v9-preclaim", Attempt: 2, Task: "provider-free route validation", WorkspacePaths: []string{"TASK.md"}, AllowedActions: []string{"read", "search", "edit", "run_check", "checkpoint_and_interrupt", "finish"}, MaxSteps: 15}
	executorPlan, err := BuildOpenCodeTopologyActionPlanForFrozenExecutorV9(definition.Executor, definition.Executor, executorRequest)
	if err != nil {
		return proof, err
	}
	if err := ValidateTopologyV9PlanBinding(definition.Executor, executorPlan, OpenCodeTopologyActionTransportV9); err != nil {
		return proof, err
	}
	executorResolved, err := ValidateTopologyV9ResolvedConfig(executorPlan, executorPlan.InlineConfig)
	if err != nil {
		return proof, err
	}
	evidence := "provider-free representative reviewer evidence"
	evidenceHash := sha256.Sum256([]byte(evidence))
	reviewRequest, err := BuildTopologyReviewRequest("v9-preclaim", "v9-preclaim:review-1", "R04", hex.EncodeToString(evidenceHash[:]), "provider-free representative review", []string{"EVIDENCE.md"}, []string{"preclaim_evidence"}, []TopologyEvidenceItem{{EvidenceID: "preclaim_evidence", Kind: "preclaim", Content: evidence, SHA256: hex.EncodeToString(evidenceHash[:])}})
	if err != nil {
		return proof, err
	}
	reviewerPlan, err := BuildOpenCodeTopologyV9ReviewPlanForFrozenReviewer(definition.Reviewer, definition.Reviewer, reviewRequest)
	if err != nil {
		return proof, err
	}
	if err := ValidateTopologyV9PlanBinding(definition.Reviewer, reviewerPlan, OpenCodeTopologyReviewTransportV9); err != nil {
		return proof, err
	}
	reviewerResolved, err := ValidateTopologyV9ResolvedConfig(reviewerPlan, reviewerPlan.InlineConfig)
	if err != nil {
		return proof, err
	}
	proof.ExecutorPlanSHA256 = executorPlan.InlineConfigSHA256
	proof.ExecutorResolvedSHA256 = executorResolved
	proof.ReviewerPlanSHA256 = reviewerPlan.InlineConfigSHA256
	proof.ReviewerResolvedSHA256 = reviewerResolved
	if !validSHA256(proof.ExecutorPlanSHA256) || !validSHA256(proof.ExecutorResolvedSHA256) || !validSHA256(proof.ReviewerPlanSHA256) || !validSHA256(proof.ReviewerResolvedSHA256) {
		return TopologyV9Prevalidation{}, fmt.Errorf("topology v9 prevalidation hashes are incomplete")
	}
	return proof, nil
}
