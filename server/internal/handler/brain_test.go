package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/brain"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestBrainTaskToolsScopeAndDurableHandoff(t *testing.T) {
	if testPool == nil {
		t.Fatal("isolated test database required")
	}
	ctx := context.Background()
	calls := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture-only" {
			t.Error("server credential missing")
		}
		if strings.HasSuffix(r.URL.Path, "/events") {
			if r.URL.Query().Get("project") != "crm" || r.URL.Query().Get("taskId") != "prior-task" {
				t.Error("history escaped bound project/task")
			}
			writeJSON(w, 200, map[string]any{"schemaVersion": "silvermediums.brain.v2", "servedAt": "now", "events": []any{map[string]any{"project": "crm", "task_id": "prior-task", "sequence": 1}}, "nextCursor": "1", "hasMore": false})
			return
		}
		if strings.Contains(r.URL.Path, "/evidence/") {
			writeJSON(w, 200, map[string]any{"schemaVersion": "silvermediums.brain.v2", "servedAt": "now", "evidence": map[string]any{"id": "33333333-3333-4333-8333-333333333333", "project": "outside", "revision": strings.Repeat("a", 64), "observation": map[string]any{"private": true}}})
			return
		}
		if strings.Contains(r.URL.Path, "documents/") {
			writeJSON(w, 200, map[string]any{"schemaVersion": "silvermediums.brain.v2", "servedAt": "now", "document": map[string]any{"id": uuid.Nil.String(), "revision": strings.Repeat("a", 64), "project": "outside", "body": "private"}})
			return
		}
		writeJSON(w, 200, map[string]any{"schemaVersion": "silvermediums.brain.v2", "servedAt": "now", "project": map[string]any{"id": "crm"}})
	}))
	t.Cleanup(source.Close)
	dir := t.TempDir()
	token := filepath.Join(dir, "token")
	config := filepath.Join(dir, "config.json")
	if err := os.WriteFile(token, []byte("fixture-only"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(brain.Config{URL: source.URL, TokenFile: token, Workspaces: map[string]brain.Workspace{testWorkspaceID: {DefaultProject: "crm"}}})
	if err := os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := brain.Load(config)
	if err != nil {
		t.Fatal(err)
	}
	old := testHandler.SharedBrain
	testHandler.SharedBrain = &brain.Service{Client: client, Store: testHandler.Queries}
	t.Cleanup(func() { testHandler.SharedBrain = old })
	runtime := dbfx.Runtime(t, "Brain tool runtime", testutil.Cols{"daemon_id": "brain-tool-daemon"})
	agent := dbfx.Agent(t, "Brain tool worker", runtime)
	issue := dbfx.Issue(t, "Brain tool proof")
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "running", "attempt": 1, "started_at": testutil.Raw("now()")})
	dbfx.Cleanup(t, "DELETE FROM brain_task_link WHERE task_id=$1", task)
	dbfx.Cleanup(t, "DELETE FROM brain_event_outbox WHERE workspace_id=$1", testWorkspaceID)
	_, err = testHandler.Queries.BindBrainTask(ctx, db.BindBrainTaskParams{WorkspaceID: parseUUID(testWorkspaceID), TaskID: parseUUID(task), Attempt: 1, ProjectID: "crm"})
	if err != nil {
		t.Fatal(err)
	}
	call := func(daemon, workspace, op string, input any, status int) *testutil.Response {
		req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+task+"/brain", map[string]any{"operation": op, "input": input}, workspace, daemon)
		return testutil.Call(t, testHandler.InvokeAgentBrain, withURLParam(req, "id", task)).Want(status)
	}
	call("other-daemon", testWorkspaceID, "context", map[string]any{}, 403)
	call("brain-tool-daemon", uuid.NewString(), "context", map[string]any{}, 404)
	if calls != 0 {
		t.Fatal("unauthorized scope contacted brain")
	}
	var result map[string]any
	call("brain-tool-daemon", testWorkspaceID, "context", map[string]any{}, 200).JSON(&result)
	call("brain-tool-daemon", testWorkspaceID, "read", map[string]any{"id": uuid.Nil.String(), "revision": strings.Repeat("a", 64)}, 502)
	call("brain-tool-daemon", testWorkspaceID, "events", map[string]any{"taskId": "prior-task", "limit": 1}, 200)
	call("brain-tool-daemon", testWorkspaceID, "events", map[string]any{"project": "outside"}, 400)
	call("brain-tool-daemon", testWorkspaceID, "evidence", map[string]any{"id": "33333333-3333-4333-8333-333333333333", "revision": strings.Repeat("a", 64)}, 502)
	input := map[string]any{"eventId": uuid.NewString(), "summary": "Changed local code; runtime unverified.", "evidenceRefs": []string{"git:fixture-sha"}}
	for i := 0; i < 2; i++ {
		call("brain-tool-daemon", testWorkspaceID, "handoff", input, 200).JSON(&result)
		if result["queued"] != true || result["accepted"] != false {
			t.Fatal("queued handoff reported delivered or accepted")
		}
	}
	input["summary"] = "different"
	call("brain-tool-daemon", testWorkspaceID, "handoff", input, 409)
	if got := dbfx.Count(t, "SELECT count(*) FROM brain_event_outbox WHERE workspace_id=$1 AND event_id LIKE '%:handoff:%'", testWorkspaceID); got != 1 {
		t.Fatalf("handoff replay duplicated: %d", got)
	}
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", task)
	call("brain-tool-daemon", testWorkspaceID, "context", map[string]any{}, 403)
}
