package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const (
	testIssue       = "11111111-1111-4111-8111-111111111111"
	testSquad       = "22222222-2222-4222-8222-222222222222"
	testLeader      = "33333333-3333-4333-8333-333333333333"
	testImplementer = "44444444-4444-4444-8444-444444444444"
	testReviewer    = "55555555-5555-4555-8555-555555555555"
	testTask        = "66666666-6666-4666-8666-666666666666"
)

type fakeServer struct {
	mu       sync.Mutex
	comments []string
	runs     []map[string]any
}

func (f *fakeServer) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/squads/"+testSquad:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": testSquad, "leader_id": testLeader})
		case r.Method == http.MethodGet && r.URL.Path == "/api/issues/"+testIssue+"/task-runs":
			runs := f.runs
			if r.URL.Query().Get("active") == "true" {
				// brokerreply reads the current running task before commenting.
				runs = append([]map[string]any{{"id": testTask, "issue_id": testIssue, "agent_id": testLeader, "status": "running"}}, runs...)
			}
			_ = json.NewEncoder(w).Encode(runs)
		case r.Method == http.MethodPost && r.URL.Path == "/api/issues/"+testIssue+"/comments":
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			content, _ := payload["content"].(string)
			f.comments = append(f.comments, content)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func testConfig(serverURL, agentID string) config {
	return config{ServerURL: serverURL, Token: "mat_test", AgentID: agentID, TaskID: testTask}
}

func testPlan(state string) workflowPlan {
	return workflowPlan{CurrentState: state, SquadID: testSquad, ImplementerID: testImplementer, ReviewerID: testReviewer}
}

func TestLeaderDelegatesToTheImplementerOnce(t *testing.T) {
	fake := &fakeServer{}
	srv := httptest.NewServer(fake.handler(t))
	defer srv.Close()

	message, err := lead(context.Background(), testConfig(srv.URL, testLeader), testIssue, testPlan("working"))
	if err != nil {
		t.Fatalf("lead: %v", err)
	}
	if len(fake.comments) != 1 || !strings.Contains(fake.comments[0], "mention://agent/"+testImplementer) {
		t.Fatalf("comments = %q, want one mention of the implementer", fake.comments)
	}
	if !strings.Contains(message, "delegated") {
		t.Fatalf("message = %q", message)
	}

	// Once the implementer has a task, a later leader wake-up routes nothing.
	fake.runs = []map[string]any{{"id": "77777777-7777-4777-8777-777777777777", "agent_id": testImplementer, "status": "completed"}}
	if _, err := lead(context.Background(), testConfig(srv.URL, testLeader), testIssue, testPlan("technical_review")); err != nil {
		t.Fatalf("second lead: %v", err)
	}
	if len(fake.comments) != 1 {
		t.Fatalf("a later leader wake-up commented again: %q", fake.comments)
	}
}

func TestLeaderRoutesNothingOutsideTheWorkingState(t *testing.T) {
	fake := &fakeServer{}
	srv := httptest.NewServer(fake.handler(t))
	defer srv.Close()
	if _, err := lead(context.Background(), testConfig(srv.URL, testLeader), testIssue, testPlan("blocked")); err != nil {
		t.Fatalf("lead: %v", err)
	}
	if len(fake.comments) != 0 {
		t.Fatalf("leader commented on a blocked workflow: %q", fake.comments)
	}
}

func TestFailureReasonNamesTheReviewFindingWhenTheLaneGivesNone(t *testing.T) {
	dir := t.TempDir()
	if got := failureReason("a regression check fails with the change applied", dir); got != "a regression check fails with the change applied" {
		t.Fatalf("harness failure was replaced: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "02-reviewer.verdict.json"), []byte(`{"verdict":"fail","findings":["first review finding"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "04-review-after-correction.verdict.json"), []byte(`{"verdict":"fail","findings":["the test never calls ApiClient.listAgents"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := failureReason("", dir); !strings.Contains(got, "never calls ApiClient.listAgents") {
		t.Fatalf("reason = %q, want the last review finding", got)
	}
	if got := failureReason("", t.TempDir()); got == "" {
		t.Fatal("an empty reason reached the owner")
	}
}

func TestAnAgentOutsideTheWorkflowIsRefused(t *testing.T) {
	fake := &fakeServer{}
	srv := httptest.NewServer(fake.handler(t))
	defer srv.Close()
	stranger := "88888888-8888-4888-8888-888888888888"
	if _, err := lead(context.Background(), testConfig(srv.URL, stranger), testIssue, testPlan("working")); err == nil {
		t.Fatal("an agent that is not the squad leader was allowed to route the workflow")
	}
	if len(fake.comments) != 0 {
		t.Fatalf("refused agent commented: %q", fake.comments)
	}
}
