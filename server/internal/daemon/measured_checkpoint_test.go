package daemon

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestExecuteAndDrainMeasuredCheckpointWaitsForToolBoundary(t *testing.T) {
	backend := &measuredBoundaryBackend{toolStartDelay: 10 * time.Millisecond, toolDelay: 75 * time.Millisecond}
	d := &Daemon{client: acknowledgedUsageSampleClient(t, true)}
	ctx, control := withMeasuredCheckpointControl(context.Background(), testContinuityPlan())
	started := time.Now()
	result, tools, err := d.executeAndDrainProviderAttempt(ctx, backend, "ordinary prompt", agent.ExecOptions{}, slog.Default(), "task-measured", 1, 1, "codex", "", new(atomic.Int32), true)
	if err != nil {
		t.Fatal(err)
	}
	if !control.requested.Load() {
		t.Fatal("authenticated exact-context request did not mark the provider attempt")
	}
	if result.Status != "aborted" || result.SessionID != "session-measured" || tools != 1 {
		t.Fatalf("result=%+v tools=%d", result, tools)
	}
	if elapsed := time.Since(started); elapsed < backend.toolStartDelay+backend.toolDelay {
		t.Fatalf("provider interrupted after %s, before delayed tool boundary %s", elapsed, backend.toolStartDelay+backend.toolDelay)
	}
	if !backend.toolResultSent.Load() {
		t.Fatal("provider was interrupted before the in-flight tool result")
	}
}

func TestExecuteAndDrainNaturalTerminalWinsCheckpointRace(t *testing.T) {
	backend := &measuredTerminalBackend{}
	d := &Daemon{client: acknowledgedUsageSampleClient(t, true)}
	ctx, control := withMeasuredCheckpointControl(context.Background(), testContinuityPlan())
	result, _, err := d.executeAndDrainProviderAttempt(ctx, backend, "ordinary prompt", agent.ExecOptions{}, slog.Default(), "task-terminal", 1, 1, "codex", "", new(atomic.Int32), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.Output != "done" {
		t.Fatalf("natural result changed: %+v", result)
	}
	if control.requested.Load() {
		t.Fatal("naturally terminal run was converted into a checkpoint interrupt")
	}
}

func TestExecuteAndDrainCompletionAfterCheckpointSignalSuppressesFollowUp(t *testing.T) {
	backend := &measuredCompletionOnCancelBackend{}
	d := &Daemon{client: acknowledgedUsageSampleClient(t, true)}
	ctx, control := withMeasuredCheckpointControl(context.Background(), testContinuityPlan())
	result, _, err := d.executeAndDrainProviderAttempt(ctx, backend, "ordinary prompt", agent.ExecOptions{}, slog.Default(), "task-terminal-race", 1, 1, "codex", "", new(atomic.Int32), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.Output != "naturally done" {
		t.Fatalf("natural result changed: %+v", result)
	}
	if control.requested.Load() {
		t.Fatal("terminal completion after threshold signal incorrectly requested a checkpoint follow-up")
	}
}

func TestExecuteAndDrainOwnerCancellationIsNotCheckpointRequest(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	ctx, control := withMeasuredCheckpointControl(baseCtx, testContinuityPlan())
	d := &Daemon{client: acknowledgedUsageSampleClient(t, false)}
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	result, _, err := d.executeAndDrainProviderAttempt(ctx, &measuredCancellationBackend{}, "ordinary prompt", agent.ExecOptions{}, slog.Default(), "task-cancel", 1, 1, "codex", "", new(atomic.Int32), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "cancelled" {
		t.Fatalf("cancelled result = %+v", result)
	}
	if control.requested.Load() {
		t.Fatal("owner/system cancellation was mislabeled as a measured checkpoint request")
	}
}

func TestRunMeasuredCheckpointTurnResumesExactSessionWithNarrowPrompt(t *testing.T) {
	backend := &capturingCheckpointBackend{}
	d := &Daemon{client: acknowledgedUsageSampleClient(t, false)}
	initial := agent.Result{
		Status: "aborted", SessionID: "session-1", ProviderModels: []string{"gpt-a"}, ProviderTurns: 2,
		Usage: map[string]agent.TokenUsage{"gpt-a": {InputTokens: 10}},
	}
	result, tools, err := d.runMeasuredCheckpointTurn(context.Background(), backend, initial, 3, agent.ExecOptions{ResumeSessionID: "old-session"}, testContinuityPlan(), slog.Default(), "task-1", 1, 1, "codex", "", new(atomic.Int32))
	if err != nil {
		t.Fatal(err)
	}
	if backend.opts.ResumeSessionID != "session-1" || !backend.opts.ResumeExpected {
		t.Fatalf("checkpoint resume options = %+v", backend.opts)
	}
	if !strings.Contains(backend.prompt, "Stop starting new implementation work") || strings.Contains(backend.prompt, "ordinary prompt") {
		t.Fatalf("checkpoint prompt was not narrow: %q", backend.prompt)
	}
	if result.SessionID != "session-1" || result.Status != "completed" || tools != 5 || result.ProviderTurns != 3 || result.Usage["gpt-a"].InputTokens != 15 {
		t.Fatalf("checkpoint result=%+v tools=%d", result, tools)
	}
}

func TestRunMeasuredCheckpointTurnPreservesOwnerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Daemon{client: acknowledgedUsageSampleClient(t, false)}
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	result, _, err := d.runMeasuredCheckpointTurn(ctx, &measuredCheckpointCancellationBackend{}, agent.Result{Status: "aborted", SessionID: "session-1"}, 0, agent.ExecOptions{}, testContinuityPlan(), slog.Default(), "task-1", 1, 1, "codex", "", new(atomic.Int32))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "cancelled" {
		t.Fatalf("checkpoint cancellation became %+v", result)
	}
}

type measuredBoundaryBackend struct {
	toolStartDelay time.Duration
	toolDelay      time.Duration
	toolResultSent atomic.Bool
}

func (b *measuredBoundaryBackend) Execute(ctx context.Context, _ string, _ agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message, 3)
	telemetry := make(chan agent.UsageSample, 1)
	result := make(chan agent.Result, 1)
	telemetry <- validExactLiveUsageSample(1, 140_000)
	close(telemetry)
	go func() {
		messages <- agent.Message{Type: agent.MessageStatus, SessionID: "session-measured"}
		time.Sleep(b.toolStartDelay)
		messages <- agent.Message{Type: agent.MessageToolUse, Tool: "write_file", CallID: "tool-1"}
		time.Sleep(b.toolDelay)
		messages <- agent.Message{Type: agent.MessageToolResult, Tool: "write_file", CallID: "tool-1", Status: "completed"}
		b.toolResultSent.Store(true)
		<-ctx.Done()
		close(messages)
		result <- agent.Result{Status: "aborted", SessionID: "session-measured"}
		close(result)
	}()
	return &agent.Session{Messages: messages, Telemetry: telemetry, Result: result}, nil
}

type measuredCompletionOnCancelBackend struct{}

func (*measuredCompletionOnCancelBackend) Execute(ctx context.Context, _ string, _ agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message)
	telemetry := make(chan agent.UsageSample, 1)
	result := make(chan agent.Result, 1)
	telemetry <- validExactLiveUsageSample(1, 140_000)
	close(telemetry)
	go func() {
		<-ctx.Done()
		close(messages)
		result <- agent.Result{Status: "completed", Output: "naturally done", SessionID: "session-race"}
		close(result)
	}()
	return &agent.Session{Messages: messages, Telemetry: telemetry, Result: result}, nil
}

type measuredTerminalBackend struct{}

func (*measuredTerminalBackend) Execute(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message)
	telemetry := make(chan agent.UsageSample, 1)
	result := make(chan agent.Result, 1)
	telemetry <- validExactLiveUsageSample(1, 140_000)
	close(telemetry)
	close(messages)
	result <- agent.Result{Status: "completed", Output: "done", SessionID: "session-terminal"}
	close(result)
	return &agent.Session{Messages: messages, Telemetry: telemetry, Result: result}, nil
}

type measuredCancellationBackend struct{}

func (*measuredCancellationBackend) Execute(ctx context.Context, _ string, _ agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message)
	telemetry := make(chan agent.UsageSample, 1)
	result := make(chan agent.Result, 1)
	telemetry <- validExactLiveUsageSample(1, 100_000)
	close(telemetry)
	go func() {
		<-ctx.Done()
		close(messages)
		result <- agent.Result{Status: "aborted", SessionID: "session-cancel"}
		close(result)
	}()
	return &agent.Session{Messages: messages, Telemetry: telemetry, Result: result}, nil
}

type capturingCheckpointBackend struct {
	mu     sync.Mutex
	prompt string
	opts   agent.ExecOptions
}

type measuredCheckpointCancellationBackend struct{}

func (*measuredCheckpointCancellationBackend) Execute(ctx context.Context, _ string, _ agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message)
	result := make(chan agent.Result, 1)
	go func() {
		<-ctx.Done()
		close(messages)
		result <- agent.Result{Status: "cancelled"}
		close(result)
	}()
	return &agent.Session{Messages: messages, Result: result}, nil
}

func (b *capturingCheckpointBackend) Execute(_ context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
	b.mu.Lock()
	b.prompt, b.opts = prompt, opts
	b.mu.Unlock()
	messages := make(chan agent.Message, 4)
	result := make(chan agent.Result, 1)
	messages <- agent.Message{Type: agent.MessageToolUse, Tool: "read_file", CallID: "a"}
	messages <- agent.Message{Type: agent.MessageToolResult, Tool: "read_file", CallID: "a", Status: "completed"}
	messages <- agent.Message{Type: agent.MessageToolUse, Tool: "test", CallID: "b"}
	messages <- agent.Message{Type: agent.MessageToolResult, Tool: "test", CallID: "b", Status: "completed"}
	close(messages)
	result <- agent.Result{
		Status: "completed", SessionID: "session-1", Output: validContinuityOutput(),
		ProviderModels: []string{"gpt-a"}, ProviderTurns: 1,
		Usage: map[string]agent.TokenUsage{"gpt-a": {InputTokens: 5}},
	}
	close(result)
	return &agent.Session{Messages: messages, Result: result}, nil
}
