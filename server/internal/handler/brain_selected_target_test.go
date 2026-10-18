package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/brain"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestBrainSelectedChatTargetOverridesConversationProject(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, runtimeID, _ := setupDirectChatSession(t, ctx, "Selected brain target")
	conversationProject := dbfx.Project(t, "Conversation project")
	selectedProject := dbfx.Project(t, "Selected issue project")
	issueID := dbfx.Issue(t, "Selected issue", testutil.Cols{"project_id": selectedProject})
	dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": sessionID, "issue_id": issueID, "source": "manual"}, "chat_session_id=$1 AND issue_id=$2", sessionID, issueID)
	session, err := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	contextJSON, _ := json.Marshal(map[string]any{"chat_work_target": target})
	calls := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, 200, map[string]any{"schemaVersion": "silvermediums.brain.v2", "servedAt": "now", "project": map[string]any{"id": "selected-project"}})
	}))
	t.Cleanup(source.Close)
	dir := t.TempDir()
	tokenPath, configPath := filepath.Join(dir, "token"), filepath.Join(dir, "config.json")
	if err := os.WriteFile(tokenPath, []byte("fixture-only"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(brain.Config{URL: source.URL, TokenFile: tokenPath, Workspaces: map[string]brain.Workspace{testWorkspaceID: {Projects: map[string]string{conversationProject: "conversation-project", selectedProject: "selected-project"}}}})
	if err := os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := brain.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	old := testHandler.SharedBrain
	testHandler.SharedBrain = &brain.Service{Client: client, Store: testHandler.Queries}
	t.Cleanup(func() { testHandler.SharedBrain = old })
	claim := func() map[string]any {
		t.Helper()
		taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "chat_session_id": sessionID, "context": string(contextJSON), "attempt": 1})
		dbfx.Cleanup(t, "DELETE FROM brain_task_link WHERE task_id=$1", taskID)
		dbfx.Cleanup(t, "DELETE FROM brain_event_outbox WHERE workspace_id=$1", testWorkspaceID)
		task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
		if err != nil {
			t.Fatal(err)
		}
		ref := testHandler.loadBrainReference(ctx, AgentTaskResponse{WorkspaceID: testWorkspaceID, ProjectID: conversationProject}, task)
		var result map[string]any
		if err := json.Unmarshal(ref, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	ref := claim()
	if ref["project"] != "selected-project" || ref["lifecycle"] != "durable_source_bound" || calls != 1 {
		t.Fatalf("selected issue did not control reference scope: %v, calls=%d", ref, calls)
	}
	dbfx.Exec(t, "UPDATE issue SET revision=revision+1 WHERE id=$1", issueID)
	ref = claim()
	if ref["state"] != "unavailable" || ref["lifecycle"] != "not_recorded" || calls != 1 {
		t.Fatalf("stale selected target contacted shared service: %v, calls=%d", ref, calls)
	}
}
