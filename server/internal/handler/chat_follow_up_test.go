package handler

import (
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"net/http"
	"strings"
	"testing"
)

func TestChatFollowUpCoalescesAndNeverReplaysOwnerInput(t *testing.T) {
	ctx := context.Background()
	_, session, runtime, daemon := setupDirectChatSession(t, ctx, "Automatic follow-up")
	dbfx.Exec(t, `UPDATE agent_runtime SET provider='codex',metadata='{"capabilities":["chat-follow-up-v1"]}' WHERE id=$1`, runtime)
	dbfx.Exec(t, `UPDATE chat_session SET session_id='owner-session',work_dir='/owner-work',runtime_id=$2 WHERE id=$1`, session, runtime)
	old := dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "user", "content": "Old approval must never be replayed"})
	issue := dbfx.Issue(t, "Audit outcome")
	linkDeliveryIssue(t, session, issue)
	add := func(body string) string {
		return dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "assistant", "content": body, "message_kind": "work_update", "work_issue_id": issue, "work_update": `{"kind":"result","issue_id":"` + issue + `","body":"` + body + `"}`, "created_at": testutil.Raw("now()-interval '20 seconds'")})
	}
	add("First result")
	add("Second result")
	for i := 0; i < 2; i++ {
		n, err := testHandler.TaskService.EnqueueChatFollowUps(ctx)
		if err != nil || n != 1-i {
			t.Fatalf("enqueue %d: %d %v", i, n, err)
		}
	}
	var taskID string
	dbfx.QueryRow(t, `SELECT id FROM agent_task_queue WHERE chat_session_id=$1 AND context ? 'chat_follow_up'`, session).Scan(&taskID)
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	if !service.IsChatFollowUp(task) || task.ChatInputTaskID.Valid || !task.ForceFreshSession || task.MaxAttempts != 1 {
		t.Fatalf("unsafe report: %+v", task)
	}
	var sealed int
	dbfx.QueryRow(t, `SELECT count(*) FROM chat_message WHERE work_follow_up_task_id=$1`, taskID).Scan(&sealed)
	if sealed != 2 {
		t.Fatal("batch did not seal both events")
	}
	r := newDaemonTokenRequest(http.MethodPost, "/claim", nil, testWorkspaceID, daemon)
	r.Header.Set("X-Client-Capabilities", "chat-follow-up-v1")
	r = testutil.WithURLParams(r, "runtimeId", runtime)
	var claim struct {
		Task *struct {
			ChatMessage    string `json:"chat_message"`
			ChatFollowUp   string `json:"chat_follow_up"`
			PriorSessionID string `json:"prior_session_id"`
		} `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, r).Want(200).JSON(&claim)
	if claim.Task == nil || claim.Task.ChatMessage != "" || claim.Task.PriorSessionID != "" || !strings.Contains(claim.Task.ChatFollowUp, "Second result") {
		t.Fatalf("report reused input or lost evidence: %+v", claim.Task)
	}
	// A new result arriving mid-report gets its own later batch, never lost to a cursor.
	add("Late result")
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, taskID)
	_, err = testHandler.TaskService.CompleteTask(ctx, parseUUID(taskID), json.RawMessage(`{"output":"The audit is ready for review."}`), "report-session", "/report-work", "", false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(session))
	if saved.SessionID.String != "owner-session" {
		t.Fatal("report replaced owner's conversation")
	}
	var owned bool
	dbfx.QueryRow(t, `SELECT task_id IS NOT NULL FROM chat_message WHERE id=$1`, old).Scan(&owned)
	if owned {
		t.Fatal("report stole human input")
	}
	n, err := testHandler.TaskService.EnqueueChatFollowUps(ctx)
	if err != nil || n != 1 {
		t.Fatalf("late event lost: %d %v", n, err)
	}
	// Removing a link prevents new work evidence from being scheduled.
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE chat_session_id=$1`, session)
	add("Unlinked result")
	dbfx.Exec(t, `DELETE FROM chat_issue_link WHERE chat_session_id=$1 AND issue_id=$2`, session, issue)
	n, err = testHandler.TaskService.EnqueueChatFollowUps(ctx)
	if err != nil || n != 0 {
		t.Fatalf("unlinked work delivered: %d %v", n, err)
	}
}

func TestChatFollowUpTokenCannotMutate(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Report token")
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "chat_session_id": session, "status": "running", "context": `{"chat_follow_up":[]}`})
	token := "mat_local_report_test"
	dbfx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(token), "task_id": task, "agent_id": agent, "workspace_id": testWorkspaceID, "user_id": testUserID, "expires_at": testutil.Raw("now()+interval '1 minute'")})
	reached := false
	gate := middleware.Auth(testHandler.Queries, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(204) }))
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		req := newRequest(method, "/api/chat/work/respond", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		testutil.Call(t, gate.ServeHTTP, req).Want(403)
	}
	if reached {
		t.Fatal("automatic report reached a mutation")
	}
	req := newRequest("GET", "/api/chat/work", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testutil.Call(t, gate.ServeHTTP, req).Want(204)
}

func TestChatFollowUpSkipsSupersededWorkerEvents(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Current revision reports")
	dbfx.Exec(t, `UPDATE agent_runtime SET provider='codex',metadata='{"capabilities":["chat-follow-up-v1"]}' WHERE id=$1`, runtime)
	issue := dbfx.Issue(t, "Revised design")
	linkDeliveryIssue(t, session, issue)
	old := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "cancelled", "created_at": testutil.Raw("now()-interval '2 minutes'")})
	current := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "created_at": testutil.Raw("now()-interval '1 minute'")})
	add := func(kind, source string) string {
		return dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "assistant", "content": "Historical worker event", "message_kind": "work_update", "work_issue_id": issue, "work_update": `{"kind":"` + kind + `","issue_id":"` + issue + `","source_id":"` + source + `"}`, "created_at": testutil.Raw("now()-interval '20 seconds'")})
	}
	for _, kind := range []string{"result", "stopped"} {
		add(kind, old)
	}
	if n, err := testHandler.TaskService.EnqueueChatFollowUps(ctx); err != nil || n != 0 {
		t.Fatalf("superseded event generated an automatic report: %d %v", n, err)
	}
	// The immutable events stay in Chat history, but may not promote the old
	// deliverable when a correction or review run has replaced its work.
	var history int
	dbfx.QueryRow(t, `SELECT count(*) FROM chat_message WHERE chat_session_id=$1 AND message_kind='work_update'`, session).Scan(&history)
	if history != 2 {
		t.Fatalf("historical evidence was removed: %d", history)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, current)
	latest := add("result", current)
	if n, err := testHandler.TaskService.EnqueueChatFollowUps(ctx); err != nil || n != 1 {
		t.Fatalf("current result not reported: %d %v", n, err)
	}
	var sealed int
	dbfx.QueryRow(t, `SELECT count(*) FROM chat_message WHERE chat_session_id=$1 AND work_follow_up_task_id IS NOT NULL AND id<>$2`, session, latest).Scan(&sealed)
	if sealed != 0 {
		t.Fatal("current report borrowed superseded events")
	}
}
