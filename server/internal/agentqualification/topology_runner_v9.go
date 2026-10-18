package agentqualification

import (
	"context"
	"errors"
	"fmt"
)

// topologyV9StageProvider owns the v9 provider-call boundary. A call-ceiling
// rejection is also a provider preflight rejection so the sealed controller
// removes its virtual in-flight call instead of recording a phantom call.
type topologyV9StageProvider struct {
	inner    RealisticProvider
	task     string
	maxCalls int
	maxCost  int64
	calls    int
	cost     int64
}

func (provider *topologyV9StageProvider) Next(ctx context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	if provider.inner == nil || provider.maxCalls < 1 || provider.maxCost < 1 {
		return RealisticProviderResponse{}, fmt.Errorf("topology v9 stage provider is invalid")
	}
	if provider.calls >= provider.maxCalls || provider.cost >= provider.maxCost {
		return RealisticProviderResponse{}, fmt.Errorf("%w: %w", ErrRealisticProviderPreflight, ErrTopologyStageBudgetExceeded)
	}
	request.Task = provider.task
	response, err := provider.inner.Next(ctx, request)
	if err == nil {
		provider.calls++
		provider.cost += response.CostMicros
	}
	return response, err
}

// RunTopologyCandidateV9 owns R04's forced attempt-1 checkpoint and resumes
// the same run family in attempt 2. The forced checkpoint is continuation,
// never a retry or provider call.
func RunTopologyCandidateV9(ctx context.Context, definition TopologyExperimentDefinition, workspace RealisticCaseWorkspace, runFamilyID string, task []byte, maxCalls int, maxTotalTokens, maxGeneratedTokens int64, provider RealisticProvider, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	if err := ValidateTopologyExperimentV9(definition); err != nil || !TopologyExperimentV9ProviderReady(definition) {
		return RealisticControllerResult{}, fmt.Errorf("topology v9 provider execution is locked until independent review and freeze")
	}
	if len(task) == 0 || maxCalls < 1 || maxCalls > 15 || maxTotalTokens < 1 || maxGeneratedTokens < 1 || maxGeneratedTokens > maxTotalTokens {
		return RealisticControllerResult{}, fmt.Errorf("topology v9 task or provider-call ceiling is invalid")
	}
	wrapped := &topologyV9StageProvider{inner: provider, task: string(task), maxCalls: maxCalls, maxCost: 250000}
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
	if workspace.CaseID != "R04" || workspace.CaseVersion != "v1" {
		return RealisticControllerResult{}, fmt.Errorf("topology v9 requires unchanged R04 v1")
	}
	interrupted, err := runRealisticCandidateFrozen(ctx, workspace, runFamilyID, 1, wrapped, nil, stageCheckpoint)
	if err != nil || interrupted.TerminalState != "interrupted" || interrupted.ProviderCalls != 0 || interrupted.Attempt != 1 || len(interrupted.AttemptHistory) != 1 {
		if err == nil {
			err = fmt.Errorf("topology v9 forced interruption contract drifted")
		}
		return interrupted, err
	}
	result, err := resumeRealisticCandidateFrozen(ctx, workspace, interrupted, wrapped, nil, stageCheckpoint)
	if result.CostMicros > 250000 {
		stageBudgetExceeded = true
	}
	if stageBudgetExceeded || err != nil && errors.Is(err, ErrTopologyStageBudgetExceeded) {
		return result, ErrTopologyStageBudgetExceeded
	}
	return result, err
}

// RunTopologyCorrectionV9 starts a bounded correction at attempt 2 so the
// already-proven R04 interruption is not replayed during the correction arm.
func RunTopologyCorrectionV9(ctx context.Context, definition TopologyExperimentDefinition, workspace RealisticCaseWorkspace, runFamilyID string, task []byte, maxCalls int, maxTotalTokens, maxGeneratedTokens int64, provider RealisticProvider, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	if err := ValidateTopologyExperimentV9(definition); err != nil || !TopologyExperimentV9ProviderReady(definition) {
		return RealisticControllerResult{}, fmt.Errorf("topology v9 provider execution is locked until independent review and freeze")
	}
	if workspace.CaseID != "R04" || workspace.CaseVersion != "v1" || len(task) == 0 || maxCalls < 1 || maxCalls > 8 || maxTotalTokens < 1 || maxGeneratedTokens < 1 || maxGeneratedTokens > maxTotalTokens {
		return RealisticControllerResult{}, fmt.Errorf("topology v9 correction contract is invalid")
	}
	wrapped := &topologyV9StageProvider{inner: provider, task: string(task), maxCalls: maxCalls, maxCost: 200000}
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
	result, err := runRealisticCandidateFrozen(ctx, workspace, runFamilyID, 2, wrapped, nil, stageCheckpoint)
	if result.CostMicros > 200000 {
		stageBudgetExceeded = true
	}
	if stageBudgetExceeded || err != nil && errors.Is(err, ErrTopologyStageBudgetExceeded) {
		return result, ErrTopologyStageBudgetExceeded
	}
	return result, err
}
