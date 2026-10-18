package agentqualification

import (
	"context"
	"fmt"
)

// RunTopologyCandidateV6 reuses the sealed controller and paid-response
// accounting without weakening the separate topology-v6 admission gate.
func RunTopologyCandidateV6(ctx context.Context, definition TopologyExperimentDefinition, workspace RealisticCaseWorkspace, runFamilyID string, task []byte, maxCalls int, maxTotalTokens, maxGeneratedTokens int64, provider RealisticProvider, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	if err := ValidateTopologyExperimentV6(definition); err != nil || !TopologyExperimentV6ProviderReady(definition) {
		return RealisticControllerResult{}, fmt.Errorf("topology v6 provider execution is locked until independent review and freeze")
	}
	if len(task) == 0 || maxCalls < 1 || maxCalls > 15 || maxTotalTokens < 1 || maxGeneratedTokens < 1 || maxGeneratedTokens > maxTotalTokens {
		return RealisticControllerResult{}, fmt.Errorf("topology v6 task or provider-call ceiling is invalid")
	}
	wrapped := &topologyTaskProvider{inner: provider, task: string(task), maxCalls: maxCalls, maxTotalTokens: maxTotalTokens, maxGeneratedTokens: maxGeneratedTokens}
	return runRealisticCandidateFrozen(ctx, workspace, runFamilyID, 1, wrapped, nil, checkpoint)
}
