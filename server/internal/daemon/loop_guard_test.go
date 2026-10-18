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

	"github.com/multica-ai/multica/server/internal/loopguard"
	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestExecuteAndDrainReportsThirdRepeatedToolCycleInShadow(t *testing.T) {
	received := make(chan struct {
		TaskAttempt     int
		ProviderAttempt int
		Observation     loopguard.Observation
	}, 1)
	client := &Client{baseURL: "http://shadow.test", client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/daemon/tasks/task-loop/loop-observations" {
			var body struct {
				TaskAttempt       int             `json:"task_attempt"`
				ProviderAttempt   int             `json:"provider_attempt"`
				Observation       json.RawMessage `json:"observation"`
				ObservationSHA256 string          `json:"observation_sha256"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
			}
			observation, err := loopguard.Parse(body.Observation)
			if err != nil {
				t.Errorf("parse observation: %v", err)
			}
			digest, _ := observation.Digest()
			if digest != body.ObservationSHA256 {
				t.Errorf("digest = %q, want %q", body.ObservationSHA256, digest)
			}
			received <- struct {
				TaskAttempt     int
				ProviderAttempt int
				Observation     loopguard.Observation
			}{body.TaskAttempt, body.ProviderAttempt, observation}
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"status":"ok"}`))}, nil
	})}}

	d := &Daemon{client: client}
	result, _, err := d.executeAndDrainProviderAttempt(context.Background(), repeatedToolBackend{}, "prompt", agent.ExecOptions{}, slog.Default(), "task-loop", 4, 2, "codex", "", new(atomic.Int32), true)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	select {
	case got := <-received:
		if got.TaskAttempt != 4 || got.ProviderAttempt != 2 || got.Observation.Provider != "codex" || got.Observation.RepeatedCycles != 3 {
			t.Fatalf("reported = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("loop shadow observation was not reported")
	}
}

func TestObserveLoopGuardShadowFailureDoesNotBlockCompletion(t *testing.T) {
	client := &Client{baseURL: "http://shadow.test", client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("no"))}, nil
	})}}
	detector := loopguard.NewDetector("codex")
	var observation *loopguard.Observation
	for i := 0; i < 3; i++ {
		id := string(rune('a' + i))
		detector.ObserveToolUse(id, "read", map[string]any{"path": "same"})
		observation = detector.ObserveToolResult(id, "", "same", "completed")
	}
	input := make(chan loopguard.Observation, 1)
	input <- *observation
	close(input)
	select {
	case <-(&Daemon{client: client}).observeLoopGuardShadow(true, "task-1", 1, 1, input, slog.Default()):
	case <-time.After(5 * time.Second):
		t.Fatal("failed shadow report blocked completion")
	}
}

type repeatedToolBackend struct{}

func (repeatedToolBackend) Execute(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message, 12)
	result := make(chan agent.Result, 1)
	for i := 0; i < 3; i++ {
		id := string(rune('a' + i))
		messages <- agent.Message{Type: agent.MessageToolUse, Tool: "read_file", CallID: id, Input: map[string]any{"path": "README.md"}}
		messages <- agent.Message{Type: agent.MessageToolResult, Tool: "read_file", CallID: id, Output: "same", Status: "completed"}
		// Narration is not evidence that the repeated operation progressed and
		// must not reset the completed-cycle detector.
		messages <- agent.Message{Type: agent.MessageThinking, Content: "trying again"}
		messages <- agent.Message{Type: agent.MessageText, Content: "still working"}
	}
	close(messages)
	result <- agent.Result{Status: "completed"}
	close(result)
	return &agent.Session{Messages: messages, Result: result}, nil
}
