package main

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowCompleteUsesSupportedHumanEndpoint(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusConflict} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			posts := 0
			packet := `{"expected_plan_revision":11,"expected_picture_revision":1,"review_task_id":"review-id","evidence_comment_id":"comment-id","evidence_comment_revision":1,"candidate_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","reason":"Explicitly accepted with operator assistance"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/issues/CHECK-1" {
					json.NewEncoder(w).Encode(map[string]string{"id": "11111111-1111-4111-8111-111111111111", "identifier": "CHECK-1"})
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != "/api/issues/11111111-1111-4111-8111-111111111111/operator-completion" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					http.NotFound(w, r)
					return
				}
				posts++
				if r.Header.Get("Authorization") != "Bearer fixture-human-session" {
					t.Error("human session was not passed")
				}
				var got, want map[string]any
				json.NewDecoder(r.Body).Decode(&got)
				json.Unmarshal([]byte(packet), &want)
				encoded, _ := json.Marshal(got)
				expected, _ := json.Marshal(want)
				if string(encoded) != string(expected) {
					t.Error("completion packet changed")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status == http.StatusOK {
					w.Write([]byte(`{"completion_id":"22222222-2222-4222-8222-222222222222","picture_revision":2,"completion_mode":"operator_assisted","replay":false}`))
				} else {
					w.Write([]byte(`{"error":"workflow changed; refresh before retry"}`))
				}
			}))
			defer server.Close()
			t.Chdir(t.TempDir())
			t.Setenv("MULTICA_SERVER_URL", server.URL)
			t.Setenv("MULTICA_WORKSPACE_ID", "workspace-fixture")
			t.Setenv("MULTICA_TOKEN", "fixture-human-session")
			t.Setenv("MULTICA_AGENT_ID", "")
			t.Setenv("MULTICA_TASK_ID", "")
			t.Setenv("MULTICA_TASK_CONFIG_ROOT", t.TempDir())
			path := filepath.Join(t.TempDir(), "completion.json")
			if err := os.WriteFile(path, []byte(packet), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := &cobra.Command{}
			cmd.Flags().String("evidence-file", path, "")
			err := issueWorkflowCompleteCmd.RunE(cmd, []string{"CHECK-1"})
			if (err == nil) != (status == http.StatusOK) {
				t.Fatalf("unexpected result %v", err)
			}
			if posts != 1 {
				t.Fatalf("command made %d posts", posts)
			}
		})
	}
}
