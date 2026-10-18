package daemon

import (
	"context"
	"log/slog"
	"time"

	"github.com/multica-ai/multica/server/internal/loopguard"
)

// observeLoopGuardShadow stores passive loop evidence without allowing a
// failed or slow evidence endpoint to affect provider execution.
func (d *Daemon) observeLoopGuardShadow(enabled bool, taskID string, taskAttempt, providerAttempt int, observations <-chan loopguard.Observation, taskLog *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if observations == nil || d.client == nil {
			return
		}
		if !enabled {
			for range observations {
			}
			return
		}
		for observation := range observations {
			reportCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err := d.client.ReportTaskLoopObservation(reportCtx, taskID, taskAttempt, providerAttempt, observation)
			cancel()
			if err != nil {
				taskLog.Warn("loop guard shadow report failed; task continues", "provider_attempt", providerAttempt, "error", err)
			}
		}
	}()
	return done
}
