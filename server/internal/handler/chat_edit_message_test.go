package handler

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

// editChatMessage drives TaskService.EditChatMessageAndRegenerate directly —
// same reasoning as sendDirectChat in chat_input_ownership_test.go: exercise
// the transactional service path, not the HTTP layer, so these tests isolate
// the DB-state contract from request parsing/auth wiring already covered
// elsewhere.
func editChatMessage(t *testing.T, ctx context.Context, agentID, sessionID, messageID, content string) (*service.DirectChatSendResult, error) {
	t.Helper()
	sess, err := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	if err != nil {
		t.Fatalf("load chat session: %v", err)
	}
	ag, err := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
	if err != nil {
		t.Fatalf("load agent: %v", err)
	}
	msg, err := testHandler.Queries.GetChatMessage(ctx, parseUUID(messageID))
	if err != nil {
		t.Fatalf("load chat message: %v", err)
	}
	return testHandler.TaskService.EditChatMessageAndRegenerate(ctx, sess, ag, msg, parseUUID(testUserID), content, "member", parseUUID(testUserID))
}

func chatMessageIDsForSession(t *testing.T, ctx context.Context, sessionID string) []string {
	t.Helper()
	rows, err := testPool.Query(ctx, `SELECT id FROM chat_message WHERE chat_session_id = $1 ORDER BY created_at`, sessionID)
	if err != nil {
		t.Fatalf("list chat message ids: %v", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan chat message id: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func taskChatSessionID(t *testing.T, ctx context.Context, taskID string) *string {
	t.Helper()
	var sessionID *string
	if err := testPool.QueryRow(ctx, `SELECT chat_session_id FROM agent_task_queue WHERE id = $1`, taskID).Scan(&sessionID); err != nil {
		t.Fatalf("load task chat_session_id: %v", err)
	}
	return sessionID
}

func chatSessionPointer(t *testing.T, ctx context.Context, sessionID string) (sessionPointer, runtimePointer *string) {
	t.Helper()
	if err := testPool.QueryRow(ctx, `SELECT session_id, runtime_id FROM chat_session WHERE id = $1`, sessionID).Scan(&sessionPointer, &runtimePointer); err != nil {
		t.Fatalf("load chat session pointer: %v", err)
	}
	return sessionPointer, runtimePointer
}

func setChatSessionPointer(t *testing.T, ctx context.Context, sessionID, fakeSessionID, runtimeID string) {
	t.Helper()
	// Give the task a fake but real-shaped session_id, and give the chat
	// session a resume pointer at the SAME runtime the test's agent actually
	// carries — AdvanceCancelledChatSessionPointer-style writers always pair
	// session_id with the task's real runtime_id, so a test pointer has to
	// match that shape too.
	if _, err := testPool.Exec(ctx, `
		UPDATE chat_session SET session_id = $2, runtime_id = $3 WHERE id = $1
	`, sessionID, fakeSessionID, runtimeID); err != nil {
		t.Fatalf("seed chat session pointer: %v", err)
	}
}

// TestEditChatMessage_ReplacesEditedMessage verifies the basic case: editing
// a completed turn's message discards it and sends the new content as a
// fresh message, leaving exactly one (new) message in the session.
func TestEditChatMessage_ReplacesEditedMessage(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "edit test")

	taskID := sendDirectChat(t, ctx, agentID, sessionID, "original content")
	ids := chatMessageIDsForSession(t, ctx, sessionID)
	if len(ids) != 1 {
		t.Fatalf("expected 1 message before edit, got %d: %v", len(ids), ids)
	}
	originalMessageID := ids[0]

	result, err := editChatMessage(t, ctx, agentID, sessionID, originalMessageID, "edited content")
	if err != nil {
		t.Fatalf("EditChatMessageAndRegenerate: %v", err)
	}

	after := chatMessageIDsForSession(t, ctx, sessionID)
	if len(after) != 1 {
		t.Fatalf("expected exactly 1 message after edit, got %d: %v", len(after), after)
	}
	if after[0] == originalMessageID {
		t.Fatalf("edit should replace the message with a new row, got the same id back")
	}
	if result.Message.Content != "edited content" {
		t.Fatalf("new message content = %q, want %q", result.Message.Content, "edited content")
	}
	if result.Task.ID == parseUUID(taskID) {
		t.Fatalf("edit should create a NEW task, got the original task id back")
	}
}

// TestEditChatMessage_DiscardsEverythingAfter verifies that editing an
// earlier message in a multi-turn conversation removes it AND every message
// sent after it, not just the edited one.
func TestEditChatMessage_DiscardsEverythingAfter(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "edit truncation test")

	sendDirectChat(t, ctx, agentID, sessionID, "first")
	firstIDs := chatMessageIDsForSession(t, ctx, sessionID)
	firstMessageID := firstIDs[0]

	sendDirectChat(t, ctx, agentID, sessionID, "second")
	beforeEdit := chatMessageIDsForSession(t, ctx, sessionID)
	if len(beforeEdit) != 2 {
		t.Fatalf("expected 2 messages before edit, got %d: %v", len(beforeEdit), beforeEdit)
	}

	if _, err := editChatMessage(t, ctx, agentID, sessionID, firstMessageID, "first, edited"); err != nil {
		t.Fatalf("EditChatMessageAndRegenerate: %v", err)
	}

	after := chatMessageIDsForSession(t, ctx, sessionID)
	if len(after) != 1 {
		t.Fatalf("expected exactly 1 message after editing the first of two, got %d: %v", len(after), after)
	}
}

// TestEditChatMessage_CancelsRunningTask verifies that editing a message
// whose task is still running stops that task instead of leaving it to run
// to completion against a conversation that no longer contains its input.
func TestEditChatMessage_CancelsRunningTask(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "edit cancels running task")

	taskID := sendDirectChat(t, ctx, agentID, sessionID, "original")
	markTaskRunning(t, ctx, taskID)
	ids := chatMessageIDsForSession(t, ctx, sessionID)

	if _, err := editChatMessage(t, ctx, agentID, sessionID, ids[0], "edited"); err != nil {
		t.Fatalf("EditChatMessageAndRegenerate: %v", err)
	}

	if status := taskStatus(t, taskID); status != "cancelled" {
		t.Fatalf("original task status = %q, want %q", status, "cancelled")
	}
}

// TestEditChatMessage_DetachesDiscardedTaskFromSession verifies the discarded
// task is detached from the chat session (chat_session_id = NULL) — the same
// state a task ends up in when its whole session is deleted. This is what
// keeps GetLastChatTaskSession from ever finding it again.
func TestEditChatMessage_DetachesDiscardedTaskFromSession(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "edit detaches task")

	taskID := sendDirectChat(t, ctx, agentID, sessionID, "original")
	ids := chatMessageIDsForSession(t, ctx, sessionID)

	if _, err := editChatMessage(t, ctx, agentID, sessionID, ids[0], "edited"); err != nil {
		t.Fatalf("EditChatMessageAndRegenerate: %v", err)
	}

	sid := taskChatSessionID(t, ctx, taskID)
	if sid != nil {
		t.Fatalf("discarded task's chat_session_id = %v, want NULL", *sid)
	}
}

// TestEditChatMessage_ClearsResumePointer is the correctness property this
// feature exists to protect: without it, the fresh turn created by an edit
// would resume the discarded turn's runtime session and the agent would
// carry on with edited-away context intact. Seeds a resume pointer that
// LOOKS like it was written by the original (now-discarded) task, edits the
// message, and asserts the pointer no longer names that stale session.
func TestEditChatMessage_ClearsResumePointer(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, runtimeID, _ := setupDirectChatSession(t, ctx, "edit clears resume pointer")

	sendDirectChat(t, ctx, agentID, sessionID, "original")
	ids := chatMessageIDsForSession(t, ctx, sessionID)

	setChatSessionPointer(t, ctx, sessionID, "stale-runtime-session-id", runtimeID)
	seededSessionPtr, _ := chatSessionPointer(t, ctx, sessionID)
	if seededSessionPtr == nil || *seededSessionPtr != "stale-runtime-session-id" {
		t.Fatalf("test setup: session pointer did not seed as expected: %v", seededSessionPtr)
	}

	if _, err := editChatMessage(t, ctx, agentID, sessionID, ids[0], "edited"); err != nil {
		t.Fatalf("EditChatMessageAndRegenerate: %v", err)
	}

	sessionPtr, runtimePtr := chatSessionPointer(t, ctx, sessionID)
	if sessionPtr != nil {
		t.Fatalf("chat_session.session_id = %v after edit, want NULL (stale pointer must not survive)", *sessionPtr)
	}
	if runtimePtr != nil {
		t.Fatalf("chat_session.runtime_id = %v after edit, want NULL", *runtimePtr)
	}
}

// TestEditChatMessage_RejectsAssistantMessage verifies only a user's own
// ordinary message can be edited — not the agent's reply.
func TestEditChatMessage_RejectsAssistantMessage(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "edit rejects assistant message")

	taskID := sendDirectChat(t, ctx, agentID, sessionID, "original")
	var replyID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_message (chat_session_id, role, content, task_id, message_kind)
		VALUES ($1, 'assistant', 'a reply', $2, 'message')
		RETURNING id
	`, sessionID, taskID).Scan(&replyID); err != nil {
		t.Fatalf("seed assistant reply: %v", err)
	}

	_, err := editChatMessage(t, ctx, agentID, sessionID, replyID, "trying to edit the reply")
	if err == nil {
		t.Fatal("expected editing an assistant message to be rejected, got nil error")
	}
	if err != service.ErrChatMessageNotEditable {
		t.Fatalf("error = %v, want ErrChatMessageNotEditable", err)
	}
}
