package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/multica-ai/multica/server/pkg/agent"
)

type measuredCheckpointControl struct {
	plan      *ContinuityCheckpointPlan
	requested atomic.Bool
	sessionID atomic.Pointer[string]
}

func (c *measuredCheckpointControl) rememberSessionID(sessionID string) {
	if c == nil || sessionID == "" {
		return
	}
	value := sessionID
	c.sessionID.Store(&value)
}

func (c *measuredCheckpointControl) providerSessionID() string {
	if c == nil {
		return ""
	}
	value := c.sessionID.Load()
	if value == nil {
		return ""
	}
	return *value
}

type measuredCheckpointControlKey struct{}

func withMeasuredCheckpointControl(ctx context.Context, plan *ContinuityCheckpointPlan) (context.Context, *measuredCheckpointControl) {
	control := &measuredCheckpointControl{plan: plan}
	return context.WithValue(ctx, measuredCheckpointControlKey{}, control), control
}

func measuredCheckpointControlFrom(ctx context.Context) *measuredCheckpointControl {
	control, _ := ctx.Value(measuredCheckpointControlKey{}).(*measuredCheckpointControl)
	return control
}

func (d *Daemon) runMeasuredCheckpointTurn(
	ctx context.Context,
	backend agent.Backend,
	initial agent.Result,
	initialTools int32,
	opts agent.ExecOptions,
	plan *ContinuityCheckpointPlan,
	taskLog *slog.Logger,
	taskID string,
	taskAttempt, providerAttempt int,
	provider, codexHome string,
	msgSeq *atomic.Int32,
) (agent.Result, int32, error) {
	if initial.SessionID == "" {
		return initial, initialTools, fmt.Errorf("measured continuity checkpoint requires the interrupted provider session")
	}
	prompt, err := buildMeasuredContinuityCheckpointPrompt(plan)
	if err != nil {
		return initial, initialTools, fmt.Errorf("build measured continuity checkpoint prompt: %w", err)
	}
	checkpointOpts := opts
	checkpointOpts.ResumeSessionID = initial.SessionID
	checkpointOpts.ResumeExpected = true
	checkpointOpts.ResumeContinuityNotice = ""
	checkpointResult, checkpointTools, err := d.executeAndDrainProviderAttempt(
		ctx, backend, prompt, checkpointOpts, taskLog, taskID, taskAttempt,
		providerAttempt+1, provider, codexHome, msgSeq, false,
	)
	if err != nil {
		if ctx.Err() != nil {
			initial.Status = "cancelled"
			initial.Error = "task cancelled during measured continuity checkpoint"
			return initial, initialTools + checkpointTools, nil
		}
		return initial, initialTools + checkpointTools, fmt.Errorf("run measured continuity checkpoint turn: %w", err)
	}
	checkpointResult.Usage = mergeUsage(initial.Usage, checkpointResult.Usage)
	checkpointResult.ProviderTurns += initial.ProviderTurns
	checkpointResult.ProviderModels = mergeProviderModels(initial.ProviderModels, checkpointResult.ProviderModels)
	if checkpointResult.Status == "cancelled" && ctx.Err() != nil {
		return checkpointResult, initialTools + checkpointTools, nil
	}
	if checkpointResult.SessionID == "" || checkpointResult.SessionID != initial.SessionID {
		return checkpointResult, initialTools + checkpointTools, fmt.Errorf("measured continuity checkpoint did not resume the exact interrupted session")
	}
	if checkpointResult.Status != "completed" {
		return checkpointResult, initialTools + checkpointTools, fmt.Errorf("measured continuity checkpoint ended with status %q: %s", checkpointResult.Status, checkpointResult.Error)
	}
	return checkpointResult, initialTools + checkpointTools, nil
}

func mergeProviderModels(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	merged := make([]string, 0, len(a)+len(b))
	for _, models := range [][]string{a, b} {
		for _, model := range models {
			if model == "" {
				continue
			}
			if _, ok := seen[model]; ok {
				continue
			}
			seen[model] = struct{}{}
			merged = append(merged, model)
		}
	}
	return merged
}
