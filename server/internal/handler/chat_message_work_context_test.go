package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatMessageWorkContextSurvivesReadback(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "Reply context")
	issueID := dbfx.Issue(t, "Audit pipeline")
	dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": sessionID, "issue_id": issueID, "source": "manual"}, "chat_session_id=$1 AND issue_id=$2", sessionID, issueID)
	session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	agent, _ := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
	target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	sent, err := testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Explain the result", nil, "member", parseUUID(testUserID), "", target)
	if err != nil {
		t.Fatal(err)
	}
	general, err := testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Hello", nil, "member", parseUUID(testUserID), "")
	if err != nil {
		t.Fatal(err)
	}
	// The inferred candidate keeps exact same-issue corrections recoverable,
	// but is not an explicit owner selection or an instruction to dispatch.
	stored, err := testHandler.Queries.GetAgentTask(ctx, general.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var captured struct {
		Target service.ChatWorkTarget `json:"chat_work_target"`
	}
	if json.Unmarshal(stored.Context, &captured) != nil || !captured.Target.CorrectionOnly || captured.Target.IssueID != issueID {
		t.Fatal("general message lost its guarded conversation candidate")
	}
	var directives, workers int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, issueID).Scan(&directives)
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID).Scan(&workers)
	if directives != 0 || workers != 0 {
		t.Fatal("ordinary discussion dispatched an issue correction")
	}
	// Queued follow-ups normally stay out of visible history until promoted.
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=ANY($1::uuid[])`, []pgtype.UUID{sent.Task.ID, general.Task.ID})
	read := withURLParam(withChatTestWorkspaceCtx(t, newRequest("GET", "/messages", nil)), "sessionId", sessionID)
	var page ChatMessagesPageResponse
	testutil.Call(t, testHandler.ListChatMessagesPage, read).Want(200).JSON(&page)
	found := false
	for _, message := range page.Messages {
		if message.ID == uuidToString(sent.Message.ID) {
			found = true
			if message.WorkContext == nil || message.WorkContext.IssueID != issueID || message.WorkContext.Title != "Audit pipeline" {
				t.Fatalf("missing saved context: %+v", message.WorkContext)
			}
		}
		if message.ID == uuidToString(general.Message.ID) && message.WorkContext != nil {
			t.Fatal("general message inherited a neighboring target")
		}
	}
	if !found {
		t.Fatal("targeted message missing")
	}
	other := session
	other.ID = parseUUID(dbfx.ChatSession(t, agentID))
	contexts, err := testHandler.chatMessageWorkContexts(ctx, other, []pgtype.UUID{sent.Message.ID})
	if err != nil || len(contexts) != 0 {
		t.Fatalf("cross-session label leaked: %v %v", contexts, err)
	}
	// Editing a scoped message must not silently turn it into general chat.
	edited, err := editChatMessage(t, ctx, agentID, sessionID, uuidToString(sent.Message.ID), "Explain briefly")
	if err != nil {
		t.Fatal(err)
	}
	contexts, err = testHandler.chatMessageWorkContexts(ctx, session, []pgtype.UUID{edited.Message.ID})
	if err != nil || contexts[uuidToString(edited.Message.ID)] == nil {
		t.Fatalf("edit lost context: %v %v", contexts, err)
	}
}
