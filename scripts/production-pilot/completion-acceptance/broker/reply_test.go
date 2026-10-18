package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPublicationUsesCurrentNativeParentAndExactSuppression(t *testing.T) {
	oldIssue, oldRoles, oldTransport := pilotIssueID, suppressedAgentIDs, http.DefaultTransport
	defer func() { pilotIssueID = oldIssue; suppressedAgentIDs = oldRoles; http.DefaultTransport = oldTransport }()
	pilotIssueID = "11111111-1111-4111-8111-111111111111"
	agent := "22222222-2222-4222-8222-222222222222"
	task := "33333333-3333-4333-8333-333333333333"
	parent := "44444444-4444-4444-8444-444444444444"
	suppressedAgentIDs = agent + ",55555555-5555-4555-8555-555555555555,66666666-6666-4666-8666-666666666666"
	t.Setenv("MULTICA_ISSUE_ID", pilotIssueID)
	t.Setenv("MULTICA_AGENT_ID", agent)
	t.Setenv("MULTICA_TASK_ID", task)
	t.Setenv("MULTICA_TOKEN", "mat_fake_only")
	t.Setenv("MULTICA_SERVER_URL", "http://127.0.0.1:8080")
	posts := 0
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		body := "{}"
		if r.Method == http.MethodGet && r.URL.Path == "/api/issues/"+pilotIssueID+"/task-runs" && r.URL.RawQuery == "active=true" {
			raw, _ := json.Marshal([]map[string]any{{"id": task, "issue_id": pilotIssueID, "agent_id": agent, "status": "running", "trigger_comment_id": parent}})
			body = string(raw)
		} else if r.Method == http.MethodPost && r.URL.Path == "/api/issues/"+pilotIssueID+"/comments" {
			posts++
			var data map[string]any
			if json.NewDecoder(r.Body).Decode(&data) != nil {
				t.Fatal("bad publication")
			}
			if data["parent_id"] != parent || len(data["suppress_agent_ids"].([]any)) != 3 {
				t.Fatal("reply or suppression lost")
			}
		} else {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	if err := postBoundComment(context.Background(), "Preserved publication"); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatal("publication count")
	}
	suppressedAgentIDs = agent + "," + agent + ",66666666-6666-4666-8666-666666666666"
	if err := postBoundComment(context.Background(), "duplicate role"); err == nil || posts != 1 {
		t.Fatal("invalid roles posted")
	}
}
