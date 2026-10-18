package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatResponseFormatRoundTrip(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, runtimeID, daemonID := setupDirectChatSession(t, ctx, "Format review")
	request := withURLParam(withChatTestWorkspaceCtx(t, newRequest("POST", "/messages", map[string]string{"content": "Compare option A and B", "response_format": "compare"})), "sessionId", sessionID)
	var sent SendChatMessageResponse
	testutil.Call(t, testHandler.SendChatMessage, request).Want(201).JSON(&sent)
	if sent.ResponseFormat != "compare" {
		t.Fatalf("format not acknowledged: %+v", sent)
	}
	message, err := testHandler.Queries.GetChatMessage(ctx, parseUUID(sent.MessageID))
	if err != nil || message.Content != "Compare option A and B" {
		t.Fatalf("owner text was modified: %+v %v", message, err)
	}
	claimed := claimTaskForRuntimeGuard(t, runtimeID, daemonID)
	if !strings.Contains(claimed.ChatMessage, "concise table") || !strings.Contains(claimed.ChatMessage, "Compare option A and B") {
		t.Fatalf("format missing from worker brief: %q", claimed.ChatMessage)
	}
	session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	formats, err := testHandler.chatMessageResponseFormats(ctx, session, []pgtype.UUID{message.ID})
	if err != nil || formats[sent.MessageID] != "compare" {
		t.Fatalf("format readback: %v %v", formats, err)
	}
	other := session
	other.ID = parseUUID(dbfx.ChatSession(t, agentID))
	formats, err = testHandler.chatMessageResponseFormats(ctx, other, []pgtype.UUID{message.ID})
	if err != nil || len(formats) != 0 {
		t.Fatalf("cross-session format leaked: %v %v", formats, err)
	}
	// Editing retains the chosen format, but the next ordinary turn does not.
	edited, err := editChatMessage(t, ctx, agentID, sessionID, sent.MessageID, "Compare A and C")
	if err != nil {
		t.Fatal(err)
	}
	task, _ := testHandler.Queries.GetAgentTask(ctx, edited.Task.ID)
	if service.ChatResponseFormatFromContext(task.Context) != "compare" {
		t.Fatal("edit lost response format")
	}
	// Durable cancellation recovery fetches the preference from its original task,
	// even after the visible message is removed.
	restoreID := seedDraftRestore(t, sessionID, "Compare A and C", nil)
	dbfx.Exec(t, `UPDATE chat_draft_restore SET task_id=$1 WHERE id=$2`, task.ID, parseUUID(restoreID))
	readRestore := withURLParam(withChatTestWorkspaceCtx(t, newRequest("GET", "/draft-restores", nil)), "sessionId", sessionID)
	var restores ChatDraftRestoresResponse
	testutil.Call(t, testHandler.ListChatDraftRestores, readRestore).Want(200).JSON(&restores)
	if len(restores.Restores) != 1 || restores.Restores[0].ResponseFormat != "compare" {
		t.Fatalf("draft recovery lost format: %+v", restores)
	}
	general := sendDirectChat(t, ctx, agentID, sessionID, "Thanks")
	generalTask, _ := testHandler.Queries.GetAgentTask(ctx, parseUUID(general))
	if service.ChatResponseFormatFromContext(generalTask.Context) != "" {
		t.Fatal("format leaked into the next turn")
	}
	// Invalid options are rejected at the boundary rather than silently ignored.
	bad := withURLParam(withChatTestWorkspaceCtx(t, newRequest("POST", "/messages", map[string]string{"content": "Hi", "response_format": "execute"})), "sessionId", sessionID)
	testutil.Call(t, testHandler.SendChatMessage, bad).Want(400)
}
