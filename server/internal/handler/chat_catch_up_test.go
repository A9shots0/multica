package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func catchUpRead(t *testing.T, session string) service.ChatWork {
	t.Helper()
	var work service.ChatWork
	r := withURLParam(withChatTestWorkspaceCtx(t, newRequest("GET", "/work", nil)), "sessionId", session)
	testutil.Call(t, testHandler.GetChatWork, r).Want(200).JSON(&work)
	return work
}

func TestChatCatchUpExactAcknowledgementAndRestart(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Catch up across visits")
	issue := dbfx.Issue(t, "Audit result")
	linkDeliveryIssue(t, session, issue)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "started_at": testutil.Raw("now()")})
	deliverWork(t)
	before := catchUpRead(t, session)
	if len(before.CatchUp) != 1 {
		t.Fatalf("missing start: %+v", before.CatchUp)
	}
	if err := testHandler.Queries.MarkChatSessionRead(ctx, parseUUID(session)); err != nil {
		t.Fatal(err)
	}
	if len(catchUpRead(t, session).CatchUp) != 1 {
		t.Fatal("opening chat discarded unseen work")
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now(),result='{"output":"The audit is ready."}' WHERE id=$1`, task)
	deliverWork(t)
	ack := func(ids ...string) *http.Request {
		return withURLParam(withChatTestWorkspaceCtx(t, newRequest("POST", "/work/acknowledge", map[string]any{"message_ids": ids})), "sessionId", session)
	}
	// Concurrent arrival plus retry: an old view cannot acknowledge the result.
	for range 2 {
		testutil.Call(t, testHandler.AcknowledgeChatWork, ack(before.CatchUp[0].MessageID)).Want(204)
	}
	after := catchUpRead(t, session)
	if len(after.CatchUp) != 1 || after.CatchUp[0].Kind != "result" {
		t.Fatalf("lost arrival: %+v", after.CatchUp)
	}
	freshQueries := db.New(testPool)
	saved, err := freshQueries.ListUnseenChatWorkUpdates(ctx, db.ListUnseenChatWorkUpdatesParams{WorkspaceID: parseUUID(testWorkspaceID), ChatSessionID: parseUUID(session)})
	if err != nil || len(saved) != 1 {
		t.Fatalf("not durable: %v %d", err, len(saved))
	}
	actor := ack(after.CatchUp[0].MessageID)
	actor.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.AcknowledgeChatWork, actor).Want(403)
	other := createHandlerTestChatSession(t, agent)
	otherAck := withURLParam(withChatTestWorkspaceCtx(t, newRequest("POST", "/work/acknowledge", map[string]any{"message_ids": []string{after.CatchUp[0].MessageID}})), "sessionId", other)
	testutil.Call(t, testHandler.AcknowledgeChatWork, otherAck).Want(204)
	if len(catchUpRead(t, session).CatchUp) != 1 {
		t.Fatal("another chat acknowledged private evidence")
	}
	testutil.Call(t, testHandler.AcknowledgeChatWork, ack("bad-id")).Want(400)
	dbfx.Exec(t, `DELETE FROM chat_issue_link WHERE chat_session_id=$1 AND issue_id=$2`, session, issue)
	if len(catchUpRead(t, session).CatchUp) != 0 {
		t.Fatal("unlinked evidence exposed")
	}
}

func TestChatCatchUpBatchesAndDecisionIndependence(t *testing.T) {
	ctx := context.Background()
	question := newOrdinaryQuestionFixture(t)
	if result := question.report(question.issue, ownerDecisionPacket()); result.Code != 201 {
		t.Fatal(result.Body)
	}
	_, session, _, _ := setupDirectChatSession(t, ctx, "Pending decision")
	linkDeliveryIssue(t, session, question.issue)
	deliverWork(t)
	work := catchUpRead(t, session)
	if len(work.CatchUp) != 1 || len(work.Requests) != 1 {
		t.Fatalf("question missing: %+v", work)
	}
	request := withURLParam(withChatTestWorkspaceCtx(t, newRequest("POST", "/work/acknowledge", map[string]any{"message_ids": []string{work.CatchUp[0].MessageID}})), "sessionId", session)
	testutil.Call(t, testHandler.AcknowledgeChatWork, request).Want(204)
	work = catchUpRead(t, session)
	if len(work.CatchUp) != 0 || len(work.Requests) != 1 || work.Issues[0].State != "needs_you" {
		t.Fatal("reading must not resolve a decision")
	}
	for i := 0; i < 101; i++ {
		dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "assistant", "content": "Historical update", "message_kind": "work_update", "work_issue_id": question.issue, "work_update": fmt.Sprintf(`{"kind":"result","issue_id":%q,"title":"Result","occurred_at":"2026-09-20T00:00:00Z","body":"Evidence"}`, question.issue)})
	}
	work = catchUpRead(t, session)
	if !work.HasMoreCatchUp || len(work.CatchUp) != 100 {
		t.Fatalf("wrong batch: %d %v", len(work.CatchUp), work.HasMoreCatchUp)
	}
	ids := []string{}
	for _, update := range work.CatchUp {
		ids = append(ids, update.MessageID)
	}
	request = withURLParam(withChatTestWorkspaceCtx(t, newRequest("POST", "/work/acknowledge", map[string]any{"message_ids": ids})), "sessionId", session)
	testutil.Call(t, testHandler.AcknowledgeChatWork, request).Want(204)
	work = catchUpRead(t, session)
	if work.HasMoreCatchUp || len(work.CatchUp) != 1 {
		t.Fatal("older-source late delivery lost between batches")
	}
}
