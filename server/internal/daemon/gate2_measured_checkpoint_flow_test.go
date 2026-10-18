package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/usagesample"
	"github.com/multica-ai/multica/server/pkg/agent"
)

// TestMeasuredCheckpointFlowThroughTypedCompletion exercises the complete
// daemon-side Gate 2 seam with one fake provider: an exact 70% sample is
// durably acknowledged, the active turn yields only after its in-flight tool,
// the exact same provider session performs a checkpoint-only turn, the receipt
// is required and stripped from owner-visible output, both turns' usage is
// retained, and the terminal callback carries the typed receipt. The server's
// atomic typed-completion -> forced-fresh-successor transaction is covered by
// TestCompleteTaskWithContinuationCheckpointIsAtomicAndIdempotent in service.
func TestMeasuredCheckpointFlowThroughTypedCompletion(t *testing.T) {
	const taskID = "22222222-2222-2222-2222-222222222222"
	backend := &gate2FlowBackend{toolDelay: 40 * time.Millisecond}
	transport := &gate2FlowTransport{t: t, taskID: taskID}
	d := &Daemon{client: &Client{
		baseURL: "http://gate2.test",
		client:  &http.Client{Transport: transport},
	}}

	ctx, control := withMeasuredCheckpointControl(context.Background(), testContinuityPlan())
	started := time.Now()
	initial, tools, err := d.executeAndDrainProviderAttempt(
		ctx, backend, "ordinary implementation prompt", agent.ExecOptions{}, slog.Default(),
		taskID, 3, 1, "codex", "", new(atomic.Int32), true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !control.requested.Load() {
		t.Fatal("exact 70% acknowledgement did not authorize the measured checkpoint")
	}
	if !backend.toolResultSent.Load() || time.Since(started) < backend.toolDelay {
		t.Fatal("provider was interrupted before the in-flight tool reached its result boundary")
	}
	if initial.Status != "aborted" || initial.SessionID != "session-gate2" || tools != 1 {
		t.Fatalf("initial result=%+v tools=%d", initial, tools)
	}

	checkpointed, tools, err := d.runMeasuredCheckpointTurn(
		context.Background(), backend, initial, tools, agent.ExecOptions{}, testContinuityPlan(),
		slog.Default(), taskID, 3, 1, "codex", "", new(atomic.Int32),
	)
	if err != nil {
		t.Fatal(err)
	}
	if backend.calls.Load() != 2 {
		t.Fatalf("provider calls=%d, want ordinary turn plus one checkpoint turn", backend.calls.Load())
	}
	if backend.checkpointResumeID != "session-gate2" || checkpointed.SessionID != "session-gate2" {
		t.Fatalf("checkpoint session=(requested %q, returned %q), want exact interrupted session", backend.checkpointResumeID, checkpointed.SessionID)
	}
	if !strings.Contains(backend.checkpointPrompt, "Stop starting new implementation work") ||
		strings.Contains(backend.checkpointPrompt, "ordinary implementation prompt") {
		t.Fatalf("checkpoint prompt was not narrow: %q", backend.checkpointPrompt)
	}
	if tools != 2 || checkpointed.ProviderTurns != 3 {
		t.Fatalf("combined tools=%d turns=%d, want 2 tools and 3 turns", tools, checkpointed.ProviderTurns)
	}
	if got := checkpointed.Usage["gpt-a"]; got.InputTokens != 13 || got.OutputTokens != 6 || got.CacheReadTokens != 4 || got.CacheWriteTokens != 2 || got.CostUSDTicks != 12 {
		t.Fatalf("combined gpt-a usage=%+v", got)
	}
	if got := checkpointed.Usage["gpt-b"]; got.InputTokens != 17 || got.OutputTokens != 3 {
		t.Fatalf("checkpoint-only model usage=%+v", got)
	}

	receipt, visible, err := parseTaskContinuityCheckpoint(checkpointed.Output, testContinuityPlan(), true)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.PackageStep != 1 || visible != "Package one is safely parked." {
		t.Fatalf("receipt=%+v visible=%q", receipt, visible)
	}
	d.reportTaskResult(context.Background(), taskID, TaskResult{
		Status: "completed", Comment: visible, SessionID: checkpointed.SessionID,
		WorkDir: "/tmp/gate2-durable", ContinuityCheckpoint: receipt,
	}, slog.Default())

	request := transport.completedRequest()
	if request.Output != visible || request.SessionID != "session-gate2" || request.ContinuityCheckpoint == nil {
		t.Fatalf("typed completion=%+v", request)
	}
	if request.ContinuityCheckpoint.PackageStep != 1 || request.ContinuityCheckpoint.NextAction != "Run the integration path." {
		t.Fatalf("typed checkpoint=%+v", request.ContinuityCheckpoint)
	}
	if strings.Contains(request.Output, continuityCheckpointOpenTag) {
		t.Fatalf("machine receipt leaked into owner-visible output: %q", request.Output)
	}
	if got := transport.usageReports.Load(); got != 1 {
		t.Fatalf("usage reports=%d, want exactly one authenticated threshold report", got)
	}
}

type gate2FlowBackend struct {
	calls              atomic.Int32
	toolDelay          time.Duration
	toolResultSent     atomic.Bool
	mu                 sync.Mutex
	checkpointPrompt   string
	checkpointResumeID string
}

func (b *gate2FlowBackend) Execute(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
	call := b.calls.Add(1)
	if call == 1 {
		messages := make(chan agent.Message, 2)
		telemetry := make(chan agent.UsageSample, 1)
		result := make(chan agent.Result, 1)
		messages <- agent.Message{Type: agent.MessageToolUse, Tool: "write_file", CallID: "write-1"}
		telemetry <- validExactLiveUsageSample(1, 140_000)
		close(telemetry)
		go func() {
			time.Sleep(b.toolDelay)
			messages <- agent.Message{Type: agent.MessageToolResult, Tool: "write_file", CallID: "write-1", Status: "completed"}
			b.toolResultSent.Store(true)
			<-ctx.Done()
			close(messages)
			result <- agent.Result{
				Status: "aborted", SessionID: "session-gate2", ProviderModels: []string{"gpt-a"}, ProviderTurns: 2,
				Usage: map[string]agent.TokenUsage{"gpt-a": {
					InputTokens: 10, OutputTokens: 5, CacheReadTokens: 4, CacheWriteTokens: 1, CostUSDTicks: 7,
				}},
			}
			close(result)
		}()
		return &agent.Session{Messages: messages, Telemetry: telemetry, Result: result}, nil
	}

	b.mu.Lock()
	b.checkpointPrompt = prompt
	b.checkpointResumeID = opts.ResumeSessionID
	b.mu.Unlock()
	messages := make(chan agent.Message, 2)
	result := make(chan agent.Result, 1)
	messages <- agent.Message{Type: agent.MessageToolUse, Tool: "test", CallID: "test-1"}
	messages <- agent.Message{Type: agent.MessageToolResult, Tool: "test", CallID: "test-1", Status: "completed"}
	close(messages)
	result <- agent.Result{
		Status: "completed", SessionID: "session-gate2", Output: validContinuityOutput(),
		ProviderModels: []string{"gpt-a", "gpt-b"}, ProviderTurns: 1,
		Usage: map[string]agent.TokenUsage{
			"gpt-a": {InputTokens: 3, OutputTokens: 1, CacheWriteTokens: 1, CostUSDTicks: 5},
			"gpt-b": {InputTokens: 17, OutputTokens: 3},
		},
	}
	close(result)
	return &agent.Session{Messages: messages, Result: result}, nil
}

type gate2FlowCompletionRequest struct {
	Output               string                       `json:"output"`
	SessionID            string                       `json:"session_id"`
	ContinuityCheckpoint *ContinuityCheckpointReceipt `json:"continuity_checkpoint"`
}

type gate2FlowTransport struct {
	t             *testing.T
	taskID        string
	usageReports  atomic.Int32
	mu            sync.Mutex
	completionReq gate2FlowCompletionRequest
}

func (t *gate2FlowTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response := func(body string) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	if strings.HasSuffix(r.URL.Path, "/usage-samples") {
		wantPath := "/api/daemon/tasks/" + t.taskID + "/usage-samples"
		if r.URL.Path != wantPath {
			t.t.Fatalf("usage report path=%q, want %q", r.URL.Path, wantPath)
		}
		var body struct {
			Sample json.RawMessage `json:"sample"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.t.Fatal(err)
		}
		sample, err := usagesample.Parse(body.Sample)
		if err != nil {
			t.t.Fatal(err)
		}
		if sample.TaskAttemptID != t.taskID+":3" || sample.ProviderAttempt != 1 || sample.ContextTokens != 140_000 {
			t.t.Fatalf("threshold sample targeted wrong execution: %+v", sample)
		}
		digest, _ := sample.Digest()
		decision, _ := usagesample.ShadowDecision(sample)
		if decision != "would_request_checkpoint" {
			t.t.Fatalf("threshold decision=%q", decision)
		}
		t.usageReports.Add(1)
		encoded, _ := json.Marshal(map[string]any{
			"sample_sha256": digest, "shadow_decision": decision, "checkpoint_requested": true,
		})
		return response(string(encoded))
	}
	if strings.HasSuffix(r.URL.Path, "/complete") {
		wantPath := "/api/daemon/tasks/" + t.taskID + "/complete"
		if r.URL.Path != wantPath {
			t.t.Fatalf("completion path=%q, want %q", r.URL.Path, wantPath)
		}
		var body gate2FlowCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.t.Fatal(err)
		}
		t.mu.Lock()
		t.completionReq = body
		t.mu.Unlock()
		return response(`{}`)
	}
	// Transcript batches are part of the real drain path but not the subject of
	// this regression. Accept them so the test still crosses that boundary.
	return response(`{}`)
}

func (t *gate2FlowTransport) completedRequest() gate2FlowCompletionRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.completionReq
}
