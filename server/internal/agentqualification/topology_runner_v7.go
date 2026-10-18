package agentqualification

import (
	"context"
	"errors"
	"fmt"
)

// topologyV7StageProvider owns the v7 provider-call boundary. A call-ceiling
// rejection is also a provider preflight rejection so the sealed controller
// removes its virtual in-flight call instead of recording a phantom call.
type topologyV7StageProvider struct {
	inner    RealisticProvider
	task     string
	maxCalls int
	calls    int
}

func (provider *topologyV7StageProvider) Next(ctx context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	if provider.inner == nil || provider.maxCalls < 1 {
		return RealisticProviderResponse{}, fmt.Errorf("topology v7 stage provider is invalid")
	}
	if provider.calls >= provider.maxCalls {
		return RealisticProviderResponse{}, fmt.Errorf("%w: %w", ErrRealisticProviderPreflight, ErrTopologyStageBudgetExceeded)
	}
	request.Task = provider.task
	response, err := provider.inner.Next(ctx, request)
	if err == nil {
		provider.calls++
	}
	return response, err
}

// RunTopologyCandidateV7 reuses the sealed controller and paid-response
// accounting without weakening the separate topology-v7 admission gate.
func RunTopologyCandidateV7(ctx context.Context, definition TopologyExperimentDefinition, workspace RealisticCaseWorkspace, runFamilyID string, task []byte, maxCalls int, maxTotalTokens, maxGeneratedTokens int64, provider RealisticProvider, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	if err := ValidateTopologyExperimentV7(definition); err != nil || !TopologyExperimentV7ProviderReady(definition) {
		return RealisticControllerResult{}, fmt.Errorf("topology v7 provider execution is locked until independent review and freeze")
	}
	if len(task) == 0 || maxCalls < 1 || maxCalls > 15 || maxTotalTokens < 1 || maxGeneratedTokens < 1 || maxGeneratedTokens > maxTotalTokens {
		return RealisticControllerResult{}, fmt.Errorf("topology v7 task or provider-call ceiling is invalid")
	}
	wrapped := &topologyV7StageProvider{inner: provider, task: string(task), maxCalls: maxCalls}
	stageBudgetExceeded := false
	stageCheckpoint := func(result RealisticControllerResult) error {
		if checkpoint != nil {
			if err := checkpoint(result); err != nil {
				return err
			}
		}
		if result.InputTokens+result.OutputTokens+result.ReasoningTokens > maxTotalTokens || result.OutputTokens+result.ReasoningTokens > maxGeneratedTokens {
			stageBudgetExceeded = true
			return ErrTopologyStageBudgetExceeded
		}
		return nil
	}
	result, err := runRealisticCandidateFrozen(ctx, workspace, runFamilyID, 1, wrapped, nil, stageCheckpoint)
	if stageBudgetExceeded || err != nil && errors.Is(err, ErrTopologyStageBudgetExceeded) {
		return result, ErrTopologyStageBudgetExceeded
	}
	return result, err
}
