package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/usagesample"
	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestObserveUsageSamplesShadowCoalescesAndReportsLatest(t *testing.T) {
	received := make(chan usagesample.Sample, 1)
	client := &Client{baseURL: "http://shadow.test", client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/daemon/tasks/task-1/usage-samples" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body struct {
			Sample json.RawMessage `json:"sample"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		sample, err := usagesample.Parse(body.Sample)
		if err != nil {
			t.Errorf("parse sample: %v", err)
		}
		received <- sample
		digest, _ := sample.Digest()
		decision, _ := usagesample.ShadowDecision(sample)
		responseBody, _ := json.Marshal(map[string]any{"status": "ok", "sample_sha256": digest, "shadow_decision": decision, "checkpoint_requested": false})
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(responseBody)))}, nil
	})}}

	d := &Daemon{client: client}
	telemetry := make(chan agent.UsageSample, 2)
	telemetry <- validLiveUsageSample(1, 10)
	telemetry <- validLiveUsageSample(2, 20)
	close(telemetry)
	select {
	case <-d.observeUsageSamplesShadow(true, "task-1", 3, 2, telemetry, slog.Default()):
	case <-time.After(5 * time.Second):
		t.Fatal("usage observer did not finish")
	}

	select {
	case sample := <-received:
		if sample.TaskAttemptID != "task-1:3" || sample.ProviderAttempt != 2 || sample.BackendAttempt != 1 || sample.Sequence != 2 || sample.InputTokens != 20 {
			t.Fatalf("reported sample = %#v", sample)
		}
	default:
		t.Fatal("usage sample was not reported")
	}
}

func TestReportTaskUsageSampleValidatesPersistedAcknowledgement(t *testing.T) {
	for _, tc := range []struct {
		name          string
		contextTokens int64
		response      func(string) string
	}{
		{"digest mismatch", 140_000, func(string) string {
			return `{"sample_sha256":"sha256:wrong","shadow_decision":"would_request_checkpoint","checkpoint_requested":true}`
		}},
		{"decision mismatch", 140_000, func(digest string) string {
			return `{"sample_sha256":"` + digest + `","shadow_decision":"continue","checkpoint_requested":true}`
		}},
		{"request below threshold", 100_000, func(digest string) string {
			return `{"sample_sha256":"` + digest + `","shadow_decision":"continue","checkpoint_requested":true}`
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			durable, err := usagesample.New("task-ack:1", 1, validExactLiveUsageSample(1, tc.contextTokens))
			if err != nil {
				t.Fatal(err)
			}
			digest, _ := durable.Digest()
			body := tc.response(digest)
			client := &Client{baseURL: "http://ack.test", client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}}
			if _, err := client.ReportTaskUsageSample(context.Background(), "task-ack", durable); err == nil {
				t.Fatal("mismatched acknowledgement was accepted")
			}
		})
	}
}

func TestObserveUsageSamplesSignalsCheckpointOnceFromAuthenticatedAck(t *testing.T) {
	client := acknowledgedUsageSampleClient(t, true)
	telemetry := make(chan agent.UsageSample, 2)
	telemetry <- validExactLiveUsageSample(1, 140_000)
	telemetry <- validExactLiveUsageSample(2, 150_000)
	close(telemetry)

	observation := (&Daemon{client: client}).observeUsageSamples(true, testContinuityPlan(), "task-checkpoint", 1, 1, telemetry, slog.Default())
	select {
	case <-observation.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("usage observer did not finish")
	}
	select {
	case <-observation.CheckpointRequested:
	default:
		t.Fatal("authenticated checkpoint acknowledgement did not signal")
	}
	select {
	case <-observation.CheckpointRequested:
		t.Fatal("checkpoint acknowledgement signaled more than once")
	default:
	}
}

func TestObserveUsageSamplesDoesNotLoseThresholdToLaterLowerSnapshot(t *testing.T) {
	telemetry := make(chan agent.UsageSample, 2)
	telemetry <- validExactLiveUsageSample(1, 140_000)
	telemetry <- validExactLiveUsageSample(2, 50_000)
	close(telemetry)

	observation := (&Daemon{client: acknowledgedUsageSampleClient(t, true)}).observeUsageSamples(true, testContinuityPlan(), "task-precompact", 1, 1, telemetry, slog.Default())
	<-observation.Done
	select {
	case <-observation.CheckpointRequested:
	default:
		t.Fatal("later lower snapshot erased the exact threshold crossing")
	}
}

func TestObserveUsageSamplesKeepsCheckpointShadowOnlyWithoutPlan(t *testing.T) {
	telemetry := make(chan agent.UsageSample, 1)
	telemetry <- validExactLiveUsageSample(1, 140_000)
	close(telemetry)
	observation := (&Daemon{client: acknowledgedUsageSampleClient(t, true)}).observeUsageSamples(true, nil, "task-shadow", 1, 1, telemetry, slog.Default())
	<-observation.Done
	select {
	case <-observation.CheckpointRequested:
		t.Fatal("checkpoint signaled without authenticated plan")
	default:
	}
}

func acknowledgedUsageSampleClient(t *testing.T, checkpointRequested bool) *Client {
	t.Helper()
	return &Client{baseURL: "http://ack.test", client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/usage-samples") {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"status":"ok"}`))}, nil
		}
		var request struct {
			Sample json.RawMessage `json:"sample"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode usage request: %v", err)
		}
		sample, err := usagesample.Parse(request.Sample)
		if err != nil {
			t.Fatalf("parse usage request: %v", err)
		}
		digest, _ := sample.Digest()
		decision, _ := usagesample.ShadowDecision(sample)
		body, _ := json.Marshal(map[string]any{
			"status": "ok", "sample_sha256": digest, "shadow_decision": decision, "checkpoint_requested": checkpointRequested,
		})
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}}
}

func TestObserveUsageSamplesShadowFailureDoesNotBlockCompletion(t *testing.T) {
	client := &Client{baseURL: "http://shadow.test", client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("no"))}, nil
	})}}
	d := &Daemon{client: client}
	telemetry := make(chan agent.UsageSample, 1)
	telemetry <- validLiveUsageSample(1, 10)
	close(telemetry)
	select {
	case <-d.observeUsageSamplesShadow(true, "task-1", 1, 1, telemetry, slog.Default()):
	case <-time.After(5 * time.Second):
		t.Fatal("failed shadow report blocked completion")
	}
}

func TestObserveUsageSamplesShadowDisabledDrainsWithoutReporting(t *testing.T) {
	var requests atomic.Int32
	client := &Client{baseURL: "http://shadow.test", client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}}
	telemetry := make(chan agent.UsageSample, 1)
	telemetry <- validLiveUsageSample(1, 10)
	close(telemetry)
	select {
	case <-(&Daemon{client: client}).observeUsageSamplesShadow(false, "task-1", 1, 1, telemetry, slog.Default()):
	case <-time.After(time.Second):
		t.Fatal("disabled usage observer did not drain")
	}
	if requests.Load() != 0 {
		t.Fatalf("disabled usage observer made %d endpoint calls", requests.Load())
	}
}

func TestLiveUsageTelemetryDoesNotResetIdleWatchdog(t *testing.T) {
	d := &Daemon{cfg: Config{AgentIdleWatchdog: 40 * time.Millisecond}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, _, err := d.executeAndDrain(ctx, telemetryOnlyBackend{}, "prompt", agent.ExecOptions{}, slog.Default(), "task-telemetry-idle", "", new(atomic.Int32))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "idle_watchdog" {
		t.Fatalf("status = %q, want idle_watchdog", result.Status)
	}
}

type telemetryOnlyBackend struct{}

func (telemetryOnlyBackend) Execute(ctx context.Context, _ string, _ agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message)
	telemetry := make(chan agent.UsageSample, 1)
	result := make(chan agent.Result, 1)
	go func() {
		defer close(messages)
		defer close(telemetry)
		defer close(result)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		var sequence int64
		for {
			select {
			case <-ctx.Done():
				result <- agent.Result{Status: "aborted"}
				return
			case <-ticker.C:
				sequence++
				sample := validLiveUsageSample(sequence, sequence)
				select {
				case telemetry <- sample:
				default:
					select {
					case <-telemetry:
					default:
					}
					select {
					case telemetry <- sample:
					default:
					}
				}
			}
		}
	}()
	return &agent.Session{Messages: messages, Telemetry: telemetry, Result: result}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func validLiveUsageSample(sequence, input int64) agent.UsageSample {
	return agent.UsageSample{
		Sequence: sequence, BackendAttempt: 1, Provider: "codex", Model: "gpt-test", ProviderCalls: sequence,
		SourceEvent: "thread/tokenUsage/updated", Usage: agent.TokenUsage{InputTokens: input},
		ModelContextWindow: 200_000, ModelContextWindowKind: agent.ContextMeasurementExact,
		ContextTokens: input, ContextTokensMeasurementKind: agent.ContextMeasurementProxy,
	}
}

func validExactLiveUsageSample(sequence, contextTokens int64) agent.UsageSample {
	sample := validLiveUsageSample(sequence, contextTokens)
	sample.ContextTokens = contextTokens
	sample.ContextTokensMeasurementKind = agent.ContextMeasurementExact
	return sample
}
