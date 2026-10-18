package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatConversationCorrectionStopsActiveWorkAndPreservesOwnerWords(t *testing.T) {
	ctx := context.Background()
	worker := newOrdinaryQuestionFixture(t)
	agent, session, _, _ := setupDirectChatSession(t, ctx, "Ordinary active correction")
	linkDeliveryIssue(t, session, worker.issue)
	// Ordinary design feedback plus screenshot references exceeds both former
	// 2 KB Chat and 4 KB directive limits. Preserve it rather than asking Mika
	// to paraphrase immutable owner input.
	words := "Keep this dashboard and its existing shell.\n" + strings.Repeat("Make missing account data say unavailable instead of zero. Readable typography matters.\n", 80) + "![owner screenshot](/uploads/owner-capture.png)"
	task := sendDirectChat(t, ctx, agent, session, words)
	markTaskRunning(t, ctx, task)
	currentImage, currentMessage := seedCorrectionImage(t, task, session)
	oldMessage := dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "user", "content": "Older unrelated picture"})
	oldImage := seedCorrectionImageForMessage(t, session, oldMessage)
	stored, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(task))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Target service.ChatWorkTarget `json:"chat_work_target"`
	}
	if json.Unmarshal(stored.Context, &snapshot) != nil || !snapshot.Target.CorrectionOnly || snapshot.Target.IssueID != worker.issue || snapshot.Target.ResultTaskID != "" {
		t.Fatalf("missing correction-only frozen target: %s", stored.Context)
	}
	request := func(action string) *http.Request {
		r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": action}))
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Agent-ID", agent)
		r.Header.Set("X-Task-ID", task)
		return r
	}
	for _, action := range []string{"approve", "reject", "answer", "cancel", "accept_result", "request_changes", "change_scope"} {
		testutil.Call(t, testHandler.SubmitChatWorkResponse, request(action)).Want(409)
	}
	var before int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&before)
	if before != 0 {
		t.Fatal("implicit target authorized another operation")
	}
	for i := 0; i < 2; i++ {
		testutil.Call(t, testHandler.SubmitChatWorkResponse, request("correct")).WantOneOf(200, 201)
	}
	var status, recorded string
	var receipts, replacements int
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, worker.task).Scan(&status)
	dbfx.QueryRow(t, `SELECT count(*),min(content) FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&receipts, &recorded)
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status IN ('queued','dispatched','running')`, worker.issue).Scan(&replacements)
	if status != "cancelled" || receipts != 1 || recorded != words || replacements != 1 {
		t.Fatalf("correction outcome old=%s receipts=%d words=%q replacements=%d", status, receipts, recorded, replacements)
	}
	var boundIssue, boundComment, preservedMessage, preservedSession string
	dbfx.QueryRow(t, `SELECT issue_id::text,comment_id::text,chat_message_id::text,chat_session_id::text FROM attachment WHERE id=$1`, currentImage).Scan(&boundIssue, &boundComment, &preservedMessage, &preservedSession)
	var directiveComment string
	dbfx.QueryRow(t, `SELECT comment_id::text FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&directiveComment)
	if boundIssue != worker.issue || boundComment != directiveComment || preservedMessage != currentMessage || preservedSession != session {
		t.Fatal("current evidence was not bound without losing its Chat provenance")
	}
	var oldBound bool
	dbfx.QueryRow(t, `SELECT issue_id IS NOT NULL OR comment_id IS NOT NULL FROM attachment WHERE id=$1`, oldImage).Scan(&oldBound)
	if oldBound {
		t.Fatal("older Chat evidence leaked into this correction")
	}
}

func TestChatConversationCorrectionFailsClosedOnChangedScope(t *testing.T) {
	for _, change := range []string{"multiple_links_at_send", "new_link", "new_question", "stale_revision"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			worker := newOrdinaryQuestionFixture(t)
			agent, session, _, _ := setupDirectChatSession(t, ctx, "Guarded correction")
			linkDeliveryIssue(t, session, worker.issue)
			if change == "multiple_links_at_send" {
				linkDeliveryIssue(t, session, dbfx.Issue(t, "Other work"))
			}
			task := sendDirectChat(t, ctx, agent, session, "Use the existing shell.")
			markTaskRunning(t, ctx, task)
			image, _ := seedCorrectionImage(t, task, session)
			switch change {
			case "new_link":
				linkDeliveryIssue(t, session, dbfx.Issue(t, "Newly linked work"))
			case "new_question":
				dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID, "issue_id": worker.issue, "type": "chat_intake", "severity": "action_required", "title": "Which version?", "details": `{"question_scope":"ordinary_task"}`})
			case "stale_revision":
				dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, worker.issue)
			}
			r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "correct"}))
			r.Header.Set("X-Actor-Source", "task_token")
			r.Header.Set("X-Agent-ID", agent)
			r.Header.Set("X-Task-ID", task)
			testutil.Call(t, testHandler.SubmitChatWorkResponse, r).Want(409)
			var status string
			var count int
			dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, worker.task).Scan(&status)
			dbfx.QueryRow(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&count)
			if status != "running" || count != 0 {
				t.Fatalf("refused correction mutated work: %s %d", status, count)
			}
			var bound bool
			dbfx.QueryRow(t, `SELECT issue_id IS NOT NULL OR comment_id IS NOT NULL FROM attachment WHERE id=$1`, image).Scan(&bound)
			if bound {
				t.Fatal("refused correction changed attachment scope")
			}
		})
	}
}

func seedCorrectionImage(t *testing.T, task, session string) (string, string) {
	t.Helper()
	var message string
	dbfx.QueryRow(t, `SELECT id::text FROM chat_message WHERE task_id=$1 AND role='user' AND message_kind='message' ORDER BY created_at,id LIMIT 1`, task).Scan(&message)
	return seedCorrectionImageForMessage(t, session, message), message
}

func TestChatConversationCorrectionRejectsAmbiguousResponseImages(t *testing.T) {
	ctx := context.Background()
	worker := newOrdinaryQuestionFixture(t)
	agent, session, _, _ := setupDirectChatSession(t, ctx, "Unmapped response evidence")
	linkDeliveryIssue(t, session, worker.issue)
	task := sendDirectChat(t, ctx, agent, session, "Review this screenshot.")
	markTaskRunning(t, ctx, task)
	image, _ := seedCorrectionImage(t, task, session)
	stored, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(task))
	if err != nil {
		t.Fatal(err)
	}
	var contextData struct {
		Target    service.ChatWorkTarget     `json:"chat_work_target"`
		Responses []service.ChatWorkResponse `json:"chat_work_responses"`
	}
	if err := json.Unmarshal(stored.Context, &contextData); err != nil {
		t.Fatal(err)
	}
	contextData.Responses = []service.ChatWorkResponse{{ResponseID: "selected", Target: contextData.Target, Content: "Review this screenshot."}}
	raw, err := json.Marshal(contextData)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET context=$2 WHERE id=$1`, task, string(raw))
	r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "correct", "response_id": "selected"}))
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Agent-ID", agent)
	r.Header.Set("X-Task-ID", task)
	testutil.Call(t, testHandler.SubmitChatWorkResponse, r).Want(409)
	var bound bool
	var receipts int
	dbfx.QueryRow(t, `SELECT issue_id IS NOT NULL OR comment_id IS NOT NULL FROM attachment WHERE id=$1`, image).Scan(&bound)
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&receipts)
	if bound || receipts != 0 {
		t.Fatal("ambiguous evidence was distributed to a selected target")
	}
}

func seedCorrectionImageForMessage(t *testing.T, session, message string) string {
	t.Helper()
	return dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": session, "chat_message_id": message, "uploader_type": "member", "uploader_id": testUserID, "filename": "owner-view.png", "url": "/uploads/owner-view.png", "content_type": "image/png", "size_bytes": 123})
}

func TestChatConversationCorrectionCannotStealEvidence(t *testing.T) {
	for _, scope := range []string{"another_issue", "another_uploader", "another_session"} {
		t.Run(scope, func(t *testing.T) {
			ctx := context.Background()
			worker := newOrdinaryQuestionFixture(t)
			agent, session, _, _ := setupDirectChatSession(t, ctx, "Evidence scope")
			linkDeliveryIssue(t, session, worker.issue)
			task := sendDirectChat(t, ctx, agent, session, "Review this exact screenshot.")
			markTaskRunning(t, ctx, task)
			image, _ := seedCorrectionImage(t, task, session)
			switch scope {
			case "another_issue":
				dbfx.Exec(t, `UPDATE attachment SET issue_id=$2 WHERE id=$1`, image, dbfx.Issue(t, "Unrelated evidence"))
			case "another_uploader":
				dbfx.Exec(t, `UPDATE attachment SET uploader_type='agent',uploader_id=$2 WHERE id=$1`, image, agent)
			case "another_session":
				other := dbfx.Insert(t, "chat_session", testutil.Cols{"workspace_id": testWorkspaceID, "agent_id": agent, "creator_id": testUserID, "title": "Other evidence", "status": "active"})
				dbfx.Exec(t, `UPDATE attachment SET chat_session_id=$2 WHERE id=$1`, image, other)
			}
			r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "correct"}))
			r.Header.Set("X-Actor-Source", "task_token")
			r.Header.Set("X-Agent-ID", agent)
			r.Header.Set("X-Task-ID", task)
			testutil.Call(t, testHandler.SubmitChatWorkResponse, r).Want(409)
			var status string
			var receipts int
			dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, worker.task).Scan(&status)
			dbfx.QueryRow(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&receipts)
			if status != "running" || receipts != 0 {
				t.Fatal("out-of-scope evidence changed work")
			}
		})
	}
}

func TestChatConversationCorrectionRejectsOversizeWithoutStoppingWork(t *testing.T) {
	ctx := context.Background()
	worker := newOrdinaryQuestionFixture(t)
	agent, session, _, _ := setupDirectChatSession(t, ctx, "Bounded correction")
	linkDeliveryIssue(t, session, worker.issue)
	// Unicode counts as bytes at the persisted directive boundary.
	words := strings.Repeat("é", maxOwnerDirectiveContentBytes/2) + "x"
	task := sendDirectChat(t, ctx, agent, session, words)
	markTaskRunning(t, ctx, task)
	r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "correct"}))
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Agent-ID", agent)
	r.Header.Set("X-Task-ID", task)
	testutil.Call(t, testHandler.SubmitChatWorkResponse, r).Want(400)
	var status string
	var receipts int
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, worker.task).Scan(&status)
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&receipts)
	if status != "running" || receipts != 0 {
		t.Fatalf("oversize correction changed work: status=%s receipts=%d", status, receipts)
	}
}
