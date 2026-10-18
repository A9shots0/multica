package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ordinaryQuestionFixture struct{ issue, agent, task string }

func newOrdinaryQuestionFixture(t *testing.T) ordinaryQuestionFixture {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Fatal("isolated test database is required")
	}
	agent := createHandlerTestAgent(t, "Question worker "+uuid.NewString(), nil)
	issue := dbfx.Issue(t, "A genuine owner choice", testutil.Cols{"status": "in_progress", "assignee_type": "agent", "assignee_id": agent})
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "issue_id": issue, "status": "running", "started_at": testutil.Raw("now()"), "accountable_user_id": testUserID, "originator_source": "member", "originator_user_id": testUserID})
	return ordinaryQuestionFixture{issue, agent, task}
}
func (f ordinaryQuestionFixture) report(target string, packet map[string]any) *httptest.ResponseRecorder {
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+target+"/workflow-events", map[string]any{"event": "owner_decision_required", "reason": "A real choice is needed", "owner_decision": packet}), "id", target)
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Task-ID", f.task)
	r.Header.Set("X-Agent-ID", f.agent)
	w := httptest.NewRecorder()
	testHandler.PostIssueWorkflowEvent(w, r)
	return w
}
func (f ordinaryQuestionFixture) answer(id, target, user string, revision int64) *httptest.ResponseRecorder {
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+target+"/owner-directives", map[string]any{"intake_id": id, "directive": "correct", "content": "Use A; preserve all unrelated work.", "expected_issue_revision": revision}), "id", target)
	r.Header.Set("X-User-ID", user)
	w := httptest.NewRecorder()
	testHandler.PostIssueOwnerDirective(w, r)
	return w
}
func TestOrdinaryTaskQuestionDurableAnswer(t *testing.T) {
	f := newOrdinaryQuestionFixture(t)
	ctx := context.Background()
	w := f.report(f.issue, ownerDecisionPacket())
	if w.Code != http.StatusCreated {
		t.Fatalf("report: %d %s", w.Code, w.Body)
	}
	var receipt map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	id := receipt["inbox_item_id"].(string)
	if repeat := f.report(f.issue, ownerDecisionPacket()); repeat.Code != http.StatusOK {
		t.Fatalf("report replay: %d %s", repeat.Code, repeat.Body)
	}
	changed := ownerDecisionPacket()
	changed["question"] = "Different question"
	if repeat := f.report(f.issue, changed); repeat.Code != http.StatusConflict {
		t.Fatalf("changed replay: %d", repeat.Code)
	}
	var revision int64
	if err := testPool.QueryRow(ctx, `SELECT revision FROM issue WHERE id=$1`, f.issue).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if active := f.answer(id, f.issue, testUserID, revision); active.Code != http.StatusConflict {
		t.Fatalf("active answer: %d %s", active.Code, active.Body)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, f.task)
	current, err := testHandler.Queries.GetIssue(ctx, parseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testHandler.TaskService.EnqueueTaskForIssue(ctx, current, pgtype.UUID{}); err == nil || !strings.Contains(err.Error(), "owner question") {
		t.Fatalf("ordinary admission bypassed the unanswered question: %v", err)
	}
	projected := testHandler.ordinaryTaskQuestionsByIssue(ctx, parseUUID(testWorkspaceID), []pgtype.UUID{parseUUID(f.issue)})
	if projected[f.issue] == nil || projected[f.issue].Question != "Which copy should ship?" {
		t.Fatalf("completed reporter lost question: %+v", projected)
	}
	otherMember := dbfx.User(t, "Other member", "question-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, testWorkspaceID, otherMember, "member")
	if wrong := f.answer(id, f.issue, otherMember, revision); wrong.Code == http.StatusCreated || wrong.Code == http.StatusOK {
		t.Fatalf("wrong recipient accepted")
	}
	other := dbfx.Issue(t, "Other question target")
	if wrong := f.answer(id, other, testUserID, revision); wrong.Code != http.StatusNotFound {
		t.Fatalf("wrong issue: %d %s", wrong.Code, wrong.Body)
	}
	if stale := f.answer(id, f.issue, testUserID, revision+1); stale.Code != http.StatusConflict {
		t.Fatalf("stale revision: %d", stale.Code)
	}

	prior := testHandler.TaskService.PrepareWorkflowGuardedAuthority
	t.Cleanup(func() { testHandler.TaskService.PrepareWorkflowGuardedAuthority = prior })
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = func(context.Context, *db.Queries, db.Issue, db.Agent, pgtype.UUID, string, []byte) ([]byte, bool, error) {
		return nil, false, errors.New("test admission refused")
	}
	if denied := f.answer(id, f.issue, testUserID, revision); denied.Code != http.StatusConflict {
		t.Fatalf("admission denial: %d %s", denied.Code, denied.Body)
	}
	var writes int
	var state, status string
	var afterRevision int64
	if err := testPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id<>$2)`, f.issue, f.task).Scan(&writes); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT details->>'intake_state' FROM inbox_item WHERE id=$1`, id).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT status,revision FROM issue WHERE id=$1`, f.issue).Scan(&status, &afterRevision); err != nil {
		t.Fatal(err)
	}
	if writes != 0 || state != "pending" || status != "in_progress" || afterRevision != revision {
		t.Fatalf("denial leaked: writes=%d state=%s status=%s revision=%d", writes, state, status, afterRevision)
	}
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = prior

	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- f.answer(id, f.issue, testUserID, revision) }()
	}
	wg.Wait()
	close(results)
	codes := map[int]int{}
	var child string
	for result := range results {
		codes[result.Code]++
		var body map[string]any
		_ = json.Unmarshal(result.Body.Bytes(), &body)
		if result.Code != http.StatusCreated && result.Code != http.StatusOK {
			t.Fatalf("answer: %d %s", result.Code, result.Body)
		}
		child, _ = body["task_id"].(string)
		if body["task_state"] != "queued" {
			t.Fatalf("answer isn't queued: %v", body)
		}
	}
	if codes[http.StatusCreated] != 1 || codes[http.StatusOK] != 1 || child == "" {
		t.Fatalf("not exactly one admission: %v child=%q", codes, child)
	}
	if err := testPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id<>$2)`, f.issue, f.task).Scan(&writes); err != nil {
		t.Fatal(err)
	}
	if writes != 3 {
		t.Fatalf("expected one comment, directive and successor: %d", writes)
	}
	var continuation, directiveID string
	if err := testPool.QueryRow(ctx, `SELECT c.content,d.id::text FROM agent_task_queue t JOIN comment c ON c.id=t.trigger_comment_id JOIN issue_owner_directive d ON d.comment_id=c.id WHERE t.id=$1`, child).Scan(&continuation, &directiveID); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{id, directiveID, "already saved this answer and closed this question", "do not submit the answer again", "Which copy should ship?", "Use A; preserve all unrelated work."} {
		if !strings.Contains(continuation, required) {
			t.Fatalf("continuation lost receipt, exact answer or handoff instruction %q: %s", required, continuation)
		}
	}
	projected = testHandler.ordinaryTaskQuestionsByIssue(ctx, parseUUID(testWorkspaceID), []pgtype.UUID{parseUUID(f.issue)})
	if projected[f.issue] != nil {
		t.Fatal("admitted answer did not clear Board question")
	}
	var category, action, body string
	if err := testPool.QueryRow(ctx, `SELECT details->>'owner_category',details->>'required_action',body FROM inbox_item WHERE id=$1`, id).Scan(&category, &action, &body); err != nil {
		t.Fatal(err)
	}
	if category != "update" || action != "follow_work" || !strings.Contains(body, "queued") {
		t.Fatalf("missing meaningful Inbox update: %s %s %s", category, action, body)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, child)
	if repeat := f.answer(id, f.issue, testUserID, revision); repeat.Code != http.StatusOK {
		t.Fatalf("completed successor replay: %d", repeat.Code)
	}
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id=$1`, f.issue).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status == "done" {
		t.Fatal("question or answer falsely completed issue")
	}
}

func TestOrdinaryTaskQuestionScopeAndSupersession(t *testing.T) {
	for _, mode := range []string{"different_issue", "wrong_workspace", "completed", "newer_task", "changed_assignee", "guarded", "autopilot", "squad"} {
		t.Run(mode, func(t *testing.T) {
			f := newOrdinaryQuestionFixture(t)
			target := f.issue
			switch mode {
			case "different_issue":
				target = dbfx.Issue(t, "Other parent")
			case "wrong_workspace":
				workspace := dbfx.Workspace(t, "Foreign question workspace", "question-"+uuid.NewString())
				runtime := dbfx.Runtime(t, "Foreign question runtime", testutil.Cols{"workspace_id": workspace})
				agent := dbfx.Agent(t, "Foreign worker", runtime, testutil.Cols{"workspace_id": workspace})
				foreign := dbfx.Issue(t, "Foreign issue", testutil.Cols{"workspace_id": workspace})
				f.task = dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": foreign, "status": "running"})
			case "completed":
				dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, f.task)
			case "newer_task":
				dbfx.Task(t, f.agent, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "issue_id": f.issue, "status": "completed", "created_at": testutil.Raw("now()+interval '1 second'")})
			case "changed_assignee":
				dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue, createHandlerTestAgent(t, "Other worker", nil))
			case "guarded":
				dbfx.Exec(t, `UPDATE agent_task_queue SET context='{"guarded_dispatch":{}}' WHERE id=$1`, f.task)
			case "autopilot":
				autopilot := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Question authority fixture", "assignee_id": f.agent, "execution_mode": "run_only", "created_by_type": "member", "created_by_id": testUserID})
				run := dbfx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": autopilot, "source": "manual", "status": "running"})
				dbfx.Exec(t, `UPDATE agent_task_queue SET autopilot_run_id=$2 WHERE id=$1`, f.task, run)
			case "squad":
				dbfx.Exec(t, `UPDATE agent_task_queue SET squad_id=$2 WHERE id=$1`, f.task, uuid.NewString())
			}
			w := f.report(target, ownerDecisionPacket())
			if w.Code < 400 {
				t.Fatalf("ineligible report accepted: %d %s", w.Code, w.Body)
			}
			var n int
			if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND details->>'question_scope'='ordinary_task'`, f.issue).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Fatal("refused report persisted question")
			}
		})
	}
	t.Run("contended_task", func(t *testing.T) {
		f := newOrdinaryQuestionFixture(t)
		ctx := context.Background()
		tx, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `SELECT id FROM agent_task_queue WHERE id=$1 FOR UPDATE`, f.task); err != nil {
			t.Fatal(err)
		}
		if w := f.report(f.issue, ownerDecisionPacket()); w.Code != http.StatusConflict {
			t.Fatalf("concurrent task mutation: %d %s", w.Code, w.Body)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if w := f.report(f.issue, ownerDecisionPacket()); w.Code != http.StatusCreated {
			t.Fatalf("safe retry after contention: %d %s", w.Code, w.Body)
		}
	})
	t.Run("superseded_answer", func(t *testing.T) {
		f := newOrdinaryQuestionFixture(t)
		w := f.report(f.issue, ownerDecisionPacket())
		if w.Code != 201 {
			t.Fatalf("report: %s", w.Body)
		}
		var receipt map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &receipt)
		dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, f.task)
		dbfx.Task(t, f.agent, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "issue_id": f.issue, "status": "completed", "created_at": testutil.Raw("now()+interval '1 second'")})
		var revision int64
		_ = testPool.QueryRow(context.Background(), `SELECT revision FROM issue WHERE id=$1`, f.issue).Scan(&revision)
		if w := f.answer(receipt["inbox_item_id"].(string), f.issue, testUserID, revision); w.Code != 409 {
			t.Fatalf("superseded answer: %d %s", w.Code, w.Body)
		}
	})
}

func TestOrdinaryQuestionBriefingRequiresDurableReceipt(t *testing.T) {
	text := ordinaryTaskQuestionBriefing("SILV-60")
	for _, required := range []string{"workflow-event SILV-60 owner_decision_required", "--question", "--recommendation", "--consequence", "--option", "inbox_item_id", "finish this turn", "does not create", "never mark the issue Done"} {
		if !strings.Contains(text, required) {
			t.Errorf("missing %q", required)
		}
	}
}

func TestOrdinaryQuestionInvalidationAllowsOnlyExplicitRecovery(t *testing.T) {
	for _, change := range []string{"failed", "cancelled", "superseded", "reassigned"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			f := newOrdinaryQuestionFixture(t)
			reported := f.report(f.issue, ownerDecisionPacket())
			if reported.Code != 201 {
				t.Fatalf("report: %d %s", reported.Code, reported.Body)
			}
			var receipt map[string]any
			_ = json.Unmarshal(reported.Body.Bytes(), &receipt)
			id := receipt["inbox_item_id"].(string)
			dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, f.task)
			dbfx.Exec(t, `UPDATE inbox_item SET archived=true WHERE id=$1`, id)
			current, err := testHandler.Queries.GetIssue(ctx, parseUUID(f.issue))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := testHandler.TaskService.EnqueueTaskForIssue(ctx, current, pgtype.UUID{}); err == nil || !strings.Contains(err.Error(), "owner question") {
				t.Fatalf("archiving authorized bypass: %v", err)
			}
			switch change {
			case "failed", "cancelled":
				dbfx.Exec(t, `UPDATE agent_task_queue SET status=$2 WHERE id=$1`, f.task, change)
				dbfx.Exec(t, `UPDATE issue SET status='blocked',metadata=jsonb_build_object('terminal_task_failure_wait',jsonb_build_object('task_id',$2::text)) WHERE id=$1`, f.issue, f.task)
			case "superseded":
				dbfx.Task(t, f.agent, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "issue_id": f.issue, "status": "completed", "created_at": testutil.Raw("now()-interval '1 millisecond'")})
			case "reassigned":
				dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue, createHandlerTestAgent(t, "Recovery worker", nil))
			}
			current, err = testHandler.Queries.GetIssue(ctx, parseUUID(f.issue))
			if err != nil {
				t.Fatal(err)
			}
			if stale := f.answer(id, f.issue, testUserID, current.Revision); stale.Code != 409 {
				t.Fatalf("stale answer accepted: %d %s", stale.Code, stale.Body)
			}
			open := testHandler.ordinaryTaskQuestionsByIssue(ctx, current.WorkspaceID, []pgtype.UUID{current.ID})
			if open[f.issue] != nil {
				t.Fatal("invalid question still claims Needs You")
			}
			item, err := testHandler.Queries.GetInboxItemInWorkspace(ctx, db.GetInboxItemInWorkspaceParams{ID: parseUUID(id), WorkspaceID: current.WorkspaceID})
			if err != nil {
				t.Fatal(err)
			}
			history := []InboxItemResponse{inboxToResponse(item)}
			testHandler.projectOrdinaryQuestionLifecycle(ctx, current.WorkspaceID, history)
			var projected map[string]string
			_ = json.Unmarshal(history[0].Details, &projected)
			if projected["intake_state"] != "invalidated" || projected["owner_category"] != "history" || history[0].Body == nil || !strings.Contains(*history[0].Body, "no longer current") {
				t.Fatalf("invalid question still actionable: %+v", history[0])
			}
			inboxRequest := withChatTestWorkspaceCtx(t, newRequest(http.MethodGet, "/api/inbox/archived", nil))
			inboxResponse := httptest.NewRecorder()
			testHandler.ListArchivedInbox(inboxResponse, inboxRequest)
			if inboxResponse.Code != 200 {
				t.Fatalf("archived Inbox: %d %s", inboxResponse.Code, inboxResponse.Body)
			}
			var visible []InboxItemResponse
			if err := json.Unmarshal(inboxResponse.Body.Bytes(), &visible); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range visible {
				if row.ID == id {
					var state map[string]string
					_ = json.Unmarshal(row.Details, &state)
					found = state["intake_state"] == "invalidated" && state["owner_category"] == "history"
				}
			}
			if !found {
				t.Fatalf("API did not expose stale question as history: %s", inboxResponse.Body)
			}
			var original map[string]string
			_ = json.Unmarshal(item.Details, &original)
			if original["intake_state"] != "pending" || original["directive_receipt"] != "" {
				t.Fatal("invalidation fabricated an answer or destroyed the receipt")
			}
			var active int
			_ = testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status IN ('queued','dispatched','running')`, f.issue).Scan(&active)
			if active != 0 {
				t.Fatal("question invalidation started work automatically")
			}

			// Use the real Chat binding and typed human continuation. No status reset,
			// raw comment dispatch or direct task insertion stands in for recovery.
			assistant := createHandlerTestAgent(t, "Recovery intake assistant", nil)
			session := dbfx.ChatSession(t, assistant, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
			chatTask := dbfx.Task(t, assistant, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "status": "running", "chat_session_id": session})
			makeRequest := func() *http.Request {
				request := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": f.issue, "request": "Recover this issue after inspecting its changed worker evidence."}))
				request.Header.Set("X-Actor-Source", "task_token")
				request.Header.Set("X-Task-ID", chatTask)
				request.Header.Set("X-Agent-ID", assistant)
				return request
			}
			bound := httptest.NewRecorder()
			testHandler.CreateChatIntake(bound, makeRequest())
			if bound.Code != 201 {
				t.Fatalf("new owner intake: %d %s", bound.Code, bound.Body)
			}
			var intake map[string]any
			_ = json.Unmarshal(bound.Body.Bytes(), &intake)
			if intake["state"] != "continued" || intake["task_id"] == "" {
				t.Fatalf("intake did not queue task directly: %v", intake)
			}
			// Replaying the same Chat intake request must be idempotent (status 200).
			repeat := httptest.NewRecorder()
			testHandler.CreateChatIntake(repeat, makeRequest())
			if repeat.Code != 200 {
				t.Fatalf("recovery replay: %d %s", repeat.Code, repeat.Body)
			}
			_ = testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'`, f.issue).Scan(&active)
			if active != 1 {
				t.Fatalf("expected exactly one authorized recovery: %d", active)
			}
		})
	}
}
