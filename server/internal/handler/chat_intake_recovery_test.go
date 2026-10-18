package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestChatIntakeSystemFailureRecovery(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Fatal("isolated test database required")
	}
	ctx := context.Background()
	for _, mode := range []string{"resume", "explicit block", "external wait", "foreign failure", "workflow plan", "owner decision"} {
		t.Run(mode, func(t *testing.T) {
			worker := createHandlerTestAgent(t, "Recovery worker", nil)
			issue := dbfx.Issue(t, "Recover existing Chat request", testutil.Cols{"status": "blocked", "assignee_type": "agent", "assignee_id": worker})
			failed := dbfx.Task(t, worker, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "issue_id": issue, "status": "failed", "error": "old provider failure", "created_at": testutil.Raw("now()-interval '2 hours'"), "completed_at": testutil.Raw("now()-interval '1 hour'")})
			metadata := map[string]any{"terminal_task_failure_wait": map[string]string{"task_id": failed}}
			switch mode {
			case "explicit block":
				dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": issue, "actor_type": "member", "actor_id": testUserID, "action": "status_changed", "details": `{"from":"in_progress","to":"blocked"}`})
			case "external wait":
				metadata["recovery.wait_kind"] = "external_dependency"
			case "foreign failure":
				foreign := dbfx.Issue(t, "Different failure")
				if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET issue_id=$2 WHERE id=$1`, failed, foreign); err != nil {
					t.Fatal(err)
				}
			case "workflow plan":
				reviewer := createHandlerTestAgent(t, "Recovery reviewer", nil)
				squad := dbfx.Squad(t, "Recovery squad", worker)
				dbfx.InsertNoID(t, "issue_workflow_plan", testutil.Cols{"issue_id": issue, "workspace_id": testWorkspaceID, "squad_id": squad, "implementer_id": worker, "reviewer_id": reviewer}, "issue_id=$1", issue)
			case "owner decision":
				dbfx.Insert(t, "issue_owner_directive", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": issue, "directive_type": "change_scope", "expected_issue_revision": 1, "content": "Wait for a new scope decision", "state": "recorded", "actor_id": testUserID})
			}
			encoded, _ := json.Marshal(metadata)
			if _, err := testPool.Exec(ctx, `UPDATE issue SET metadata=$2 WHERE id=$1`, issue, encoded); err != nil {
				t.Fatal(err)
			}
			// A completed independent attempt is not itself owner authority to
			// dismiss any older failure. Only the typed continuation below is.
			dbfx.Task(t, worker, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "issue_id": issue, "status": "completed", "created_at": testutil.Raw("now()-interval '30 minutes'"), "completed_at": testutil.Raw("now()-interval '20 minutes'")})
			assistant := createHandlerTestAgent(t, "Recovery Chat", nil)
			session := dbfx.ChatSession(t, assistant, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
			chatTask := dbfx.Task(t, assistant, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "chat_session_id": session, "status": "running", "started_at": testutil.Raw("now()")})
			bind := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Continue this existing issue after the failed run."}))
			bind.Header.Set("X-Actor-Source", "task_token")
			bind.Header.Set("X-Agent-ID", assistant)
			bind.Header.Set("X-Task-ID", chatTask)
			var revision int64
			if err := testPool.QueryRow(ctx, `SELECT revision FROM issue WHERE id=$1`, issue).Scan(&revision); err != nil {
				t.Fatal(err)
			}
			if mode != "resume" {
				bind := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Continue this existing issue after the failed run."}))
				bind.Header.Set("X-Actor-Source", "task_token")
				bind.Header.Set("X-Agent-ID", assistant)
				bind.Header.Set("X-Task-ID", chatTask)
				w := httptest.NewRecorder()
				testHandler.CreateChatIntake(w, bind)
				if w.Code != http.StatusConflict {
					t.Fatalf("explicit or ambiguous wait admitted: %d %s", w.Code, w.Body)
				}
				var queued int
				if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'`, issue).Scan(&queued); err != nil || queued != 0 {
					t.Fatalf("denial queued work: count=%d error=%v", queued, err)
				}
				return
			}
			// A refused admission must roll back the attempted recovery, including
			// the owner direction and system failure marker removal.
			previous := testHandler.TaskService.PrepareWorkflowGuardedAuthority
			testHandler.TaskService.PrepareWorkflowGuardedAuthority = func(context.Context, *db.Queries, db.Issue, db.Agent, pgtype.UUID, string, []byte) ([]byte, bool, error) {
				return nil, false, errors.New("isolated admission refusal")
			}
			denied := httptest.NewRecorder()
			testHandler.CreateChatIntake(denied, bind)
			testHandler.TaskService.PrepareWorkflowGuardedAuthority = previous
			if denied.Code != http.StatusConflict {
				t.Fatalf("admission refusal: %d %s", denied.Code, denied.Body)
			}
			var unchanged bool
			var writes int
			if err := testPool.QueryRow(ctx, `SELECT status='blocked' AND metadata ? 'terminal_task_failure_wait' AND revision=$2,(SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued') FROM issue WHERE id=$1`, issue, revision).Scan(&unchanged, &writes); err != nil || !unchanged || writes != 0 {
				t.Fatalf("failed recovery leaked state: unchanged=%v writes=%d error=%v", unchanged, writes, err)
			}
			var wg sync.WaitGroup
			responses := make(chan *httptest.ResponseRecorder, 2)
			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					r := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Continue this existing issue after the failed run."}))
					r.Header.Set("X-Actor-Source", "task_token")
					r.Header.Set("X-Agent-ID", assistant)
					r.Header.Set("X-Task-ID", chatTask)
					w := httptest.NewRecorder()
					testHandler.CreateChatIntake(w, r)
					responses <- w
				}()
			}
			wg.Wait()
			close(responses)
			var receipt map[string]any
			created := 0
			for w := range responses {
				if w.Code == http.StatusCreated {
					created++
				} else if w.Code != http.StatusOK {
					t.Fatalf("resume: %d %s", w.Code, w.Body)
				}
				if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
					t.Fatal(err)
				}
			}
			if created != 1 || receipt["task_state"] != "queued" {
				t.Fatalf("continuation did not queue exactly once: %v", receipt)
			}
			var status string
			var marked bool
			var count int
			if err := testPool.QueryRow(ctx, `SELECT status,metadata ? 'terminal_task_failure_wait',revision FROM issue WHERE id=$1`, issue).Scan(&status, &marked, &revision); err != nil {
				t.Fatal(err)
			}
			if status != "in_progress" || marked || receipt["issue_revision"] != float64(revision) {
				t.Fatalf("resume projection stale: status=%s marked=%v receipt=%v revision=%d", status, marked, receipt, revision)
			}
			if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'`, issue).Scan(&count); err != nil || count != 1 {
				t.Fatalf("queued count=%d error=%v", count, err)
			}
			if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, receipt["task_id"]); err != nil {
				t.Fatal(err)
			}
			// Reproduce a stale marker already persisted by an older server after
			// an earlier successful intake. Its applied correction is not a pause.
			dbfx.Exec(t, `UPDATE issue SET status='blocked',metadata=$2 WHERE id=$1`, issue, encoded)
			canResume, err := testHandler.Queries.CanResumeIssueFailureForChatIntake(ctx, db.CanResumeIssueFailureForChatIntakeParams{ID: parseUUID(issue), WorkspaceID: parseUUID(testWorkspaceID)})
			if err != nil || !canResume {
				t.Fatalf("prior continued intake mistaken for an owner block: %v %v", canResume, err)
			}
			dbfx.Exec(t, `UPDATE issue SET status='in_progress',metadata='{}' WHERE id=$1`, issue)
			// Exercise both the scanner and the locked final mutation, including
			// a repair candidate selected before the owner continuation committed.
			if _, err := testHandler.TaskService.ReconcileTerminalFailedIssues(ctx, 100); err != nil {
				t.Fatal(err)
			}
			_, err = testHandler.Queries.BlockIssueForTerminalTaskFailure(ctx, db.BlockIssueForTerminalTaskFailureParams{ID: parseUUID(issue), WorkspaceID: parseUUID(testWorkspaceID), TaskID: parseUUID(failed)})
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("old failure was re-projected: %v", err)
			}
			if err := testPool.QueryRow(ctx, `SELECT status,metadata ? 'terminal_task_failure_wait' FROM issue WHERE id=$1`, issue).Scan(&status, &marked); err != nil || status != "in_progress" || marked {
				t.Fatalf("post-completion projection changed: %s %v %v", status, marked, err)
			}
			// The continued worker's own later failure remains actionable.
			if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status='failed',error='new failure' WHERE id=$1`, receipt["task_id"]); err != nil {
				t.Fatal(err)
			}
			if _, err := testHandler.TaskService.ReconcileTerminalFailedIssues(ctx, 100); err != nil {
				t.Fatal(err)
			}
			var marker string
			if err := testPool.QueryRow(ctx, `SELECT metadata->'terminal_task_failure_wait'->>'task_id' FROM issue WHERE id=$1`, issue).Scan(&marker); err != nil || marker != receipt["task_id"] {
				t.Fatalf("new failure suppressed: %s %v", marker, err)
			}
		})
	}
}
