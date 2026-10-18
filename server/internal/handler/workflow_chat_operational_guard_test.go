package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatTaskCannotMutateIssueCommentOrWorkflowEvent(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	agentID := createHandlerTestAgent(t, "Chat workflow boundary", nil)
	sessionID := dbfx.ChatSession(t, agentID)
	issueID := dbfx.Issue(t, "unchanged chat boundary issue")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id":      handlerTestRuntimeID(t),
		"status":          "running",
		"issue_id":        issueID,
		"chat_session_id": sessionID,
		"started_at":      testutil.Raw("now()"),
	})

	request := func(method, target string, body any) *http.Request {
		req := withURLParam(newRequest(method, target, body), "id", issueID)
		req.Header.Set("X-Actor-Source", "task_token")
		req.Header.Set("X-Task-ID", taskID)
		req.Header.Set("X-Agent-ID", agentID)
		req.Header.Set("X-User-ID", testUserID)
		return req
	}

	testutil.Call(t, testHandler.UpdateIssue, request(http.MethodPut, "/api/issues/"+issueID, map[string]any{"title": "altered"})).Want(http.StatusConflict)
	testutil.Call(t, testHandler.CreateComment, request(http.MethodPost, "/api/issues/"+issueID+"/comments", map[string]any{"content": "blocked"})).Want(http.StatusConflict)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, request(http.MethodPost, "/api/issues/"+issueID+"/workflow-events", map[string]any{"event": "implementation_started", "reason": "blocked"})).Want(http.StatusConflict)

	var title string
	if err := testPool.QueryRow(context.Background(), `SELECT title FROM issue WHERE id = $1`, issueID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "unchanged chat boundary issue" {
		t.Fatalf("chat task changed issue title to %q", title)
	}
	for table, query := range map[string]string{
		"comments":        `SELECT count(*) FROM comment WHERE issue_id = $1`,
		"workflow events": `SELECT count(*) FROM issue_workflow_event WHERE issue_id = $1`,
	} {
		var count int
		if err := testPool.QueryRow(context.Background(), query, issueID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("chat task created %d %s, want zero", count, table)
		}
	}
}
