package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type TopologyV7Prevalidation struct {
	ExecutorPlanSHA256     string `json:"executor_plan_sha256"`
	ExecutorResolvedSHA256 string `json:"executor_resolved_sha256"`
	ReviewerPlanSHA256     string `json:"reviewer_plan_sha256"`
	ReviewerResolvedSHA256 string `json:"reviewer_resolved_sha256"`
	ProviderCalls          int    `json:"provider_calls"`
}

func PrevalidateTopologyV7Routes(definition TopologyExperimentDefinition) (TopologyV7Prevalidation, error) {
	var proof TopologyV7Prevalidation
	if err := ValidateTopologyExperimentV7(definition); err != nil {
		return proof, err
	}
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return proof, err
	}
	executorRequest := RealisticProviderRequest{ProviderCallID: "v7-preclaim-executor", ProviderTurn: 1, CaseID: "R03", CaseVersion: "v2", RunFamilyID: "v7-preclaim", Attempt: 1, Task: "provider-free route validation", WorkspacePaths: []string{"TASK.md"}, AllowedActions: []string{"read", "search", "edit", "run_check", "finish"}, MaxSteps: 15}
	executorPlan, err := BuildOpenCodeTopologyActionPlanForFrozenExecutor(definition.Executor, definition.Executor, executorRequest)
	if err != nil {
		return proof, err
	}
	if err := ValidateTopologyV7PlanBinding(definition.Executor, executorPlan, OpenCodeTopologyActionTransportV7); err != nil {
		return proof, err
	}
	executorResolved, err := ValidateTopologyV7ResolvedConfig(executorPlan, executorPlan.InlineConfig)
	if err != nil {
		return proof, err
	}
	evidence := "provider-free representative reviewer evidence"
	evidenceHash := sha256.Sum256([]byte(evidence))
	reviewRequest, err := BuildTopologyReviewRequest("v7-preclaim", "v7-preclaim:review-1", "R03", hex.EncodeToString(evidenceHash[:]), "provider-free representative review", []string{"EVIDENCE.md"}, []string{"preclaim_evidence"}, []TopologyEvidenceItem{{EvidenceID: "preclaim_evidence", Kind: "preclaim", Content: evidence, SHA256: hex.EncodeToString(evidenceHash[:])}})
	if err != nil {
		return proof, err
	}
	reviewerPlan, err := BuildOpenCodeTopologyV7ReviewPlanForFrozenReviewer(definition.Reviewer, definition.Reviewer, reviewRequest)
	if err != nil {
		return proof, err
	}
	if err := ValidateTopologyV7PlanBinding(definition.Reviewer, reviewerPlan, OpenCodeTopologyReviewTransportV7); err != nil {
		return proof, err
	}
	reviewerResolved, err := ValidateTopologyV7ResolvedConfig(reviewerPlan, reviewerPlan.InlineConfig)
	if err != nil {
		return proof, err
	}
	proof.ExecutorPlanSHA256 = executorPlan.InlineConfigSHA256
	proof.ExecutorResolvedSHA256 = executorResolved
	proof.ReviewerPlanSHA256 = reviewerPlan.InlineConfigSHA256
	proof.ReviewerResolvedSHA256 = reviewerResolved
	if !validSHA256(proof.ExecutorPlanSHA256) || !validSHA256(proof.ExecutorResolvedSHA256) || !validSHA256(proof.ReviewerPlanSHA256) || !validSHA256(proof.ReviewerResolvedSHA256) {
		return TopologyV7Prevalidation{}, fmt.Errorf("topology v7 prevalidation hashes are incomplete")
	}
	return proof, nil
}
