package agentqualification

import (
	"context"
	"errors"
	"fmt"
)

var ErrTopologyStageBudgetExceeded = errors.New("topology stage budget exhausted")

// topologyTaskProvider replaces only the task field before the provider sees
// it. The deterministic controller, workspace, action allowlist, and receipt
// path remain the already-frozen realistic implementation. A fresh controller
// result means a correction receives no executor conversation or tool history.
type topologyTaskProvider struct {
	inner              RealisticProvider
	task               string
	maxCalls           int
	calls              int
	maxTotalTokens     int64
	maxGeneratedTokens int64
	totalTokens        int64
	generatedTokens    int64
	budgetExhausted    bool
}

func (provider *topologyTaskProvider) Next(ctx context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	if provider.inner == nil || provider.maxCalls < 1 {
		return RealisticProviderResponse{}, fmt.Errorf("topology stage provider is invalid")
	}
	if provider.budgetExhausted || provider.calls >= provider.maxCalls {
		return RealisticProviderResponse{}, ErrTopologyStageBudgetExceeded
	}
	provider.calls++
	request.Task = provider.task
	response, err := provider.inner.Next(ctx, request)
	if err != nil {
		return response, err
	}
	provider.totalTokens += response.InputTokens + response.OutputTokens + response.ReasoningTokens
	provider.generatedTokens += response.OutputTokens + response.ReasoningTokens
	if provider.totalTokens > provider.maxTotalTokens || provider.generatedTokens > provider.maxGeneratedTokens {
		// The paid response must be returned so the controller durably accounts
		// and seals it. The next requested turn ends only this stage without a
		// provider call.
		provider.budgetExhausted = true
	}
	return response, nil
}

// RunTopologyCandidate is the sole topology executor entrypoint. It reuses the
// reviewed realistic controller without editing any frozen realistic source.
func RunTopologyCandidate(ctx context.Context, workspace RealisticCaseWorkspace, runFamilyID string, task []byte, maxCalls int, maxTotalTokens, maxGeneratedTokens int64, provider RealisticProvider, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	definition, err := CurrentTopologyExperiment()
	if err != nil || !TopologyExperimentProviderReady(definition) {
		return RealisticControllerResult{}, fmt.Errorf("topology experiment is not reviewed and frozen")
	}
	if len(task) == 0 || maxCalls < 1 || maxCalls > 15 || maxTotalTokens < 1 || maxGeneratedTokens < 1 || maxGeneratedTokens > maxTotalTokens {
		return RealisticControllerResult{}, fmt.Errorf("topology task or provider-call ceiling is invalid")
	}
	wrapped := &topologyTaskProvider{inner: provider, task: string(task), maxCalls: maxCalls, maxTotalTokens: maxTotalTokens, maxGeneratedTokens: maxGeneratedTokens}
	return runRealisticCandidateFrozen(ctx, workspace, runFamilyID, 1, wrapped, nil, checkpoint)
}
