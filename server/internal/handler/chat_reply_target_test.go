package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/taskauthority"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func insertReplyTarget(t *testing.T, ctx context.Context, sessionID, role, kind, content string, workUpdate any) string {
	t.Helper()
	var raw []byte
	if workUpdate != nil {
		var err error
		raw, err = json.Marshal(workUpdate)
		if err != nil {
			t.Fatal(err)
		}
	}
	var id string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_message (chat_session_id, role, content, message_kind, work_update)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, sessionID, role, content, kind, raw).Scan(&id); err != nil {
		t.Fatalf("insert reply target: %v", err)
	}
	return id
}

func sendReply(t *testing.T, ctx context.Context, agentID, sessionID, content string, target service.ChatReplyTarget) (*service.DirectChatSendResult, error) {
	t.Helper()
	session, err := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	agent, err := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	return testHandler.TaskService.SendDirectChatMessageWithOptions(ctx, session, agent, parseUUID(testUserID), content, nil, "member", parseUUID(testUserID), "", service.DirectChatSendOptions{ReplyTarget: target})
}

func TestDirectChatReplyFreezesExactTargetDespiteInterveningMessage(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "exact reply")
	targetID := insertReplyTarget(t, ctx, sessionID, "assistant", "message", "First answer", nil)
	_ = insertReplyTarget(t, ctx, sessionID, "assistant", "message", "Newer answer", nil)

	sent, err := sendReply(t, ctx, agentID, sessionID, "About the first one", service.ChatReplyTarget{EventID: targetID})
	if err != nil {
		t.Fatal(err)
	}
	if got := uuidToString(sent.Message.ReplyToMessageID); got != targetID {
		t.Fatalf("reply target = %q, want %q", got, targetID)
	}
	var snapshot service.ChatReplyContext
	if err := json.Unmarshal(sent.Message.ReplyContext, &snapshot); err != nil {
		t.Fatalf("decode reply snapshot: %v", err)
	}
	if snapshot.EventID != targetID || snapshot.Content != "First answer" || snapshot.Actor != "assistant" ||
		snapshot.ContentSHA256 != taskauthority.ContentSHA256("First answer") || snapshot.ContentTruncated {
		t.Fatalf("wrong frozen target: %+v", snapshot)
	}

	pageRequest := withURLParam(withChatTestWorkspaceCtx(t, newRequest("GET", "/messages", nil)), "sessionId", sessionID)
	var page ChatMessagesPageResponse
	testutil.Call(t, testHandler.ListChatMessagesPage, pageRequest).Want(200).JSON(&page)
	found := false
	for _, message := range page.Messages {
		if message.ID == uuidToString(sent.Message.ID) {
			found = message.ReplyToMessageID != nil && *message.ReplyToMessageID == targetID && len(message.ReplyContext) > 0
		}
	}
	if !found {
		t.Fatal("reply identity did not survive message readback")
	}
}

func TestDirectChatReplyRejectsInvisibleTargetWithoutPartialTurn(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "reply sender")
	otherSessionID := dbfx.ChatSession(t, agentID, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
	crossID := insertReplyTarget(t, ctx, otherSessionID, "assistant", "message", "Private to another Chat", nil)
	hiddenID := insertReplyTarget(t, ctx, sessionID, "user", "onboarding_kickoff", "Hidden control context", nil)

	for _, targetID := range []string{crossID, hiddenID, "not-a-uuid"} {
		var beforeMessages, beforeTasks int
		if err := testPool.QueryRow(ctx, `SELECT count(*) FROM chat_message WHERE chat_session_id=$1`, sessionID).Scan(&beforeMessages); err != nil {
			t.Fatal(err)
		}
		if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE chat_session_id=$1`, sessionID).Scan(&beforeTasks); err != nil {
			t.Fatal(err)
		}
		_, err := sendReply(t, ctx, agentID, sessionID, "Do not persist this", service.ChatReplyTarget{EventID: targetID})
		if !errors.Is(err, service.ErrChatReplyTargetChanged) {
			t.Fatalf("target %q error = %v", targetID, err)
		}
		var afterMessages, afterTasks int
		_ = testPool.QueryRow(ctx, `SELECT count(*) FROM chat_message WHERE chat_session_id=$1`, sessionID).Scan(&afterMessages)
		_ = testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE chat_session_id=$1`, sessionID).Scan(&afterTasks)
		if afterMessages != beforeMessages || afterTasks != beforeTasks {
			t.Fatalf("invalid reply partially persisted: messages %d->%d tasks %d->%d", beforeMessages, afterMessages, beforeTasks, afterTasks)
		}
	}
}

func TestDirectChatReplyValidatesArtifactSubtarget(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "artifact reply")
	update := service.ChatWorkUpdate{
		Kind: "result", IssueID: "b101db55-0e8e-4dbd-8a58-d513c140f754", SourceID: "source-1", RequestID: "request-1",
		Review: &service.ChatWorkReview{SchemaVersion: 1, SourceTaskID: "source-1", Artifacts: []service.ChatWorkArtifact{{ID: "artifact-1", Label: "Checkout page", Kind: "page"}}},
	}
	targetID := insertReplyTarget(t, ctx, sessionID, "assistant", "work_update", "Checkout result", update)

	sent, err := sendReply(t, ctx, agentID, sessionID, "Change this page", service.ChatReplyTarget{EventID: targetID, BlockID: "request-1", ArtifactID: "artifact-1"})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot service.ChatReplyContext
	if err := json.Unmarshal(sent.Message.ReplyContext, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.ArtifactID != "artifact-1" || len(snapshot.Artifacts) != 1 || snapshot.Artifacts[0].Label != "Checkout page" {
		t.Fatalf("artifact snapshot = %+v", snapshot)
	}

	_, err = sendReply(t, ctx, agentID, sessionID, "Wrong artifact", service.ChatReplyTarget{EventID: targetID, ArtifactID: "artifact-missing"})
	if !errors.Is(err, service.ErrChatReplyTargetChanged) {
		t.Fatalf("stale artifact error = %v", err)
	}
}

func TestEditChatReplyPreservesExactTarget(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "edit exact reply")
	targetID := insertReplyTarget(t, ctx, sessionID, "assistant", "message", "Keep this exact answer", nil)

	sent, err := sendReply(t, ctx, agentID, sessionID, "First wording", service.ChatReplyTarget{EventID: targetID})
	if err != nil {
		t.Fatal(err)
	}
	edited, err := editChatMessage(t, ctx, agentID, sessionID, uuidToString(sent.Message.ID), "Clearer wording")
	if err != nil {
		t.Fatal(err)
	}
	if got := uuidToString(edited.Message.ReplyToMessageID); got != targetID {
		t.Fatalf("edited reply target = %q, want %q", got, targetID)
	}
	var snapshot service.ChatReplyContext
	if err := json.Unmarshal(edited.Message.ReplyContext, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.EventID != targetID || snapshot.Content != "Keep this exact answer" {
		t.Fatalf("edited reply snapshot changed: %+v", snapshot)
	}
}
