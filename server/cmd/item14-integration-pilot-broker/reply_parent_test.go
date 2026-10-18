package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrokerPreservesCurrentReplyParent(t *testing.T) {
	const current = "11111111-1111-4111-8111-111111111111"
	const old = "22222222-2222-4222-8222-222222222222"
	for _, tc := range []struct {
		name, trigger, status, task string
		wantError                   bool
	}{
		{"assignment", "", "running", "task-1", false},
		{"current comment", current, "running", "task-1", false},
		{"resumed and coalesced", current, "running", "task-2", false},
		{"terminal task", current, "completed", "task-1", true},
		{"malformed parent", "not-a-uuid", "running", "task-1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var received map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/issues/issue-1/workflow-plan":
					json.NewEncoder(w).Encode(map[string]any{"implementer_id": "implementer-1"})
				case "/api/issues/issue-1/task-runs":
					if r.URL.Query().Get("active") != "true" {
						t.Error("lookup must request current tasks")
					}
					json.NewEncoder(w).Encode([]map[string]any{{"id": tc.task, "issue_id": "issue-1", "agent_id": "agent-1", "status": tc.status, "trigger_comment_id": tc.trigger, "coalesced_comment_ids": []string{old}, "trigger_thread_id": old}})
				case "/api/issues/issue-1/comments":
					json.NewDecoder(r.Body).Decode(&received)
					w.WriteHeader(http.StatusCreated)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			t.Setenv("MULTICA_SERVER_URL", server.URL)
			t.Setenv("MULTICA_TOKEN", "mat_fixture")
			t.Setenv("MULTICA_AGENT_ID", "agent-1")
			t.Setenv("MULTICA_TASK_ID", tc.task)
			t.Setenv("MULTICA_ISSUE_ID", "issue-1")
			err := routeToImplementer(context.Background())
			if tc.wantError {
				if err == nil || received != nil {
					t.Fatal("invalid context published a comment")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, _ := received["parent_id"].(string)
			if got != tc.trigger {
				t.Fatalf("reply parent = %q, want current trigger %q", got, tc.trigger)
			}
		})
	}
}
