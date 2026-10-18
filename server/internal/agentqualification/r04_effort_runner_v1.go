package agentqualification

import (
	"context"
	"errors"
	"fmt"
)

type r04EffortV1StageProvider struct {
	inner RealisticProvider
	task  string
	calls int
	cost  int64
}

func (provider *r04EffortV1StageProvider) Next(ctx context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	if provider.inner == nil {
		return RealisticProviderResponse{}, fmt.Errorf("R04 effort v1 provider is unavailable")
	}
	if provider.calls >= 15 || provider.cost >= 350000 {
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

// RunR04EffortCandidateV1 owns the frozen zero-call attempt-1 interruption and
// same-family attempt-2 continuation. It performs no retry or review.
func RunR04EffortCandidateV1(ctx context.Context, definition TopologyExperimentDefinition, workspace RealisticCaseWorkspace, runFamilyID string, task []byte, provider RealisticProvider, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	if err := ValidateR04EffortExperimentV1(definition); err != nil || !R04EffortExperimentV1ProviderReady(definition) {
		return RealisticControllerResult{}, fmt.Errorf("R04 effort v1 provider execution is locked until independent review and freeze")
	}
	if workspace.CaseID != "R04" || workspace.CaseVersion != "v1" || len(task) == 0 || runFamilyID == "" {
		return RealisticControllerResult{}, fmt.Errorf("R04 effort v1 execution contract is invalid")
	}
	wrapped := &r04EffortV1StageProvider{inner: provider, task: string(task)}
	stageExceeded := false
	stageCheckpoint := func(result RealisticControllerResult) error {
		if checkpoint != nil {
			if err := checkpoint(result); err != nil {
				return err
			}
		}
		if result.InputTokens+result.OutputTokens+result.ReasoningTokens > 75000 || result.OutputTokens+result.ReasoningTokens > 30000 {
			stageExceeded = true
			return ErrTopologyStageBudgetExceeded
		}
		return nil
	}
	interrupted, err := runRealisticCandidateFrozen(ctx, workspace, runFamilyID, 1, wrapped, nil, stageCheckpoint)
	if err != nil || interrupted.TerminalState != "interrupted" || interrupted.ProviderCalls != 0 || interrupted.Attempt != 1 || len(interrupted.AttemptHistory) != 1 {
		if err == nil {
			err = fmt.Errorf("R04 effort v1 forced interruption contract drifted")
		}
		return interrupted, err
	}
	result, err := resumeRealisticCandidateFrozen(ctx, workspace, interrupted, wrapped, nil, stageCheckpoint)
	if result.CostMicros > 350000 {
		stageExceeded = true
	}
	if stageExceeded || err != nil && errors.Is(err, ErrTopologyStageBudgetExceeded) {
		return result, ErrTopologyStageBudgetExceeded
	}
	return result, err
}
