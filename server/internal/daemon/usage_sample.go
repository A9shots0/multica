package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/multica-ai/multica/server/internal/usagesample"
	"github.com/multica-ai/multica/server/pkg/agent"
)

type usageSampleObservation struct {
	Done                <-chan struct{}
	CheckpointRequested <-chan struct{}
}

// observeUsageSamples coalesces live telemetry to at most one report per
// second. Reporting is best-effort. It exposes one checkpoint request only
// when both the authenticated claim plan and the persisted server
// acknowledgement authorize it.
func (d *Daemon) observeUsageSamples(enabled bool, checkpointPlan *ContinuityCheckpointPlan, taskID string, taskAttempt, providerAttempt int, telemetry <-chan agent.UsageSample, taskLog *slog.Logger) usageSampleObservation {
	done := make(chan struct{})
	checkpointRequested := make(chan struct{}, 1)
	checkpointAuthorized := enabled && checkpointPlan != nil && validateContinuityCheckpointPlan(checkpointPlan) == nil
	go func() {
		defer close(done)
		if telemetry == nil || d.client == nil {
			return
		}
		if !enabled {
			for range telemetry {
			}
			return
		}
		var latest *agent.UsageSample
		checkpointSignaled := false
		flush := func() {
			if latest == nil {
				return
			}
			sample, err := usagesample.New(fmt.Sprintf("%s:%d", taskID, taskAttempt), providerAttempt, *latest)
			latest = nil
			if err != nil {
				taskLog.Warn("live usage shadow sample rejected locally; task continues", "provider_attempt", providerAttempt, "error", err)
				return
			}
			reportCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			acknowledgement, err := d.client.ReportTaskUsageSample(reportCtx, taskID, sample)
			cancel()
			if err != nil {
				taskLog.Warn("live usage shadow report failed; task continues", "provider_attempt", providerAttempt, "error", err)
				return
			}
			if checkpointAuthorized && acknowledgement.CheckpointRequested && !checkpointSignaled {
				checkpointSignaled = true
				checkpointRequested <- struct{}{}
			}
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case sample, ok := <-telemetry:
				if !ok {
					flush()
					return
				}
				copy := sample
				latest = &copy
				// Never let a later post-compaction/lower snapshot overwrite an
				// exact threshold crossing inside the one-second coalescing window.
				// This local decision only prioritizes reporting; the daemon still
				// acts solely on the server's persisted matching acknowledgement.
				if candidate, candidateErr := usagesample.New(fmt.Sprintf("%s:%d", taskID, taskAttempt), providerAttempt, copy); candidateErr == nil {
					decision, _ := usagesample.ShadowDecision(candidate)
					if decision == "would_request_checkpoint" || decision == "would_stop" {
						flush()
					}
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
	return usageSampleObservation{Done: done, CheckpointRequested: checkpointRequested}
}

// observeUsageSamplesShadow preserves the observation-only call surface while
// checkpoint interruption is integrated by the task runner.
func (d *Daemon) observeUsageSamplesShadow(enabled bool, taskID string, taskAttempt, providerAttempt int, telemetry <-chan agent.UsageSample, taskLog *slog.Logger) <-chan struct{} {
	return d.observeUsageSamples(enabled, nil, taskID, taskAttempt, providerAttempt, telemetry, taskLog).Done
}
