package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestChatIntakeDoesNotInventProjectWorker(t *testing.T) {
	ctx := context.Background()
	assistant, session, _, _ := setupDirectChatSession(t, ctx, "Project routing")
	generic := createHandlerTestAgent(t, "Generic template builder", nil)
	reviewer := createHandlerTestAgent(t, "Generic template reviewer", nil)
	squad := dbfx.Squad(t, "Generic workflow", generic)
	dbfx.InsertNoID(t, "squad_workflow_template", testutil.Cols{"squad_id": squad, "workspace_id": testWorkspaceID, "implementer_id": generic, "reviewer_id": reviewer, "requires_owner": false, "requires_deployment": false}, "squad_id=$1", squad)
	project := dbfx.Project(t, "Hardware-bound project")
	issue := dbfx.Issue(t, "Research the dashboard", testutil.Cols{"project_id": project})
	linkDeliveryIssue(t, session, issue)
	task := sendDirectChat(t, ctx, assistant, session, "Make a dashboard for this project.")
	markTaskRunning(t, ctx, task)
	req := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Research the dashboard."}))
	req.Header.Set("X-Actor-Source", "task_token")
	req.Header.Set("X-Agent-ID", assistant)
	req.Header.Set("X-Task-ID", task)
	testutil.Call(t, testHandler.CreateChatIntake, req).Want(http.StatusConflict)
	var assigned bool
	dbfx.QueryRow(t, `SELECT assignee_id IS NOT NULL FROM issue WHERE id=$1`, issue).Scan(&assigned)
	writes := dbfx.Count(t, `SELECT (SELECT count(*) FROM issue_workflow_plan WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)`, issue)
	if assigned || writes != 0 {
		t.Fatalf("generic route leaked: assigned=%v writes=%d", assigned, writes)
	}
	current, err := testHandler.Queries.GetIssue(ctx, parseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := service.ProposeIssueWorkflowPlan(ctx, testHandler.Queries, current); err != nil || ok {
		t.Fatalf("driver invented project routing: ok=%v err=%v", ok, err)
	}
	// The coordinator can recover the same issue using a real specialist;
	// intake must retain that worker and must not add a legacy controller.
	runtime := dbfx.Runtime(t, "Project host")
	specialist := dbfx.Agent(t, "Project specialist", runtime)
	retry := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Research the dashboard.", "agent_id": specialist}))
	retry.Header = req.Header.Clone()
	// Refused invocation and queue admission must roll back assignment too.
	dbfx.Exec(t, `UPDATE agent SET archived_at=now() WHERE id=$1`, specialist)
	testutil.Call(t, testHandler.CreateChatIntake, retry).Want(403)
	dbfx.Exec(t, `UPDATE agent SET archived_at=NULL WHERE id=$1`, specialist)
	previousAdmission := testHandler.TaskService.PrepareWorkflowGuardedAuthority
	t.Cleanup(func() { testHandler.TaskService.PrepareWorkflowGuardedAuthority = previousAdmission })
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = func(context.Context, *db.Queries, db.Issue, db.Agent, pgtype.UUID, string, []byte) ([]byte, bool, error) {
		return nil, false, errors.New("test authority refused")
	}
	retry = withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Research the dashboard.", "agent_id": specialist}))
	retry.Header = req.Header.Clone()
	testutil.Call(t, testHandler.CreateChatIntake, retry).Want(409)
	dbfx.QueryRow(t, `SELECT assignee_id IS NOT NULL FROM issue WHERE id=$1`, issue).Scan(&assigned)
	if assigned || dbfx.Count(t, `SELECT (SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)+(SELECT count(*) FROM inbox_item WHERE issue_id=$1)`, issue) != 0 {
		t.Fatal("refused worker selection leaked assignment or dispatch")
	}
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = previousAdmission
	retry = withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Research the dashboard.", "agent_id": specialist}))
	retry.Header = req.Header.Clone()
	testutil.Call(t, testHandler.CreateChatIntake, retry).Want(201)
	var actualAgent, actualRuntime string
	dbfx.QueryRow(t, `SELECT agent_id::text,runtime_id::text FROM agent_task_queue WHERE issue_id=$1`, issue).Scan(&actualAgent, &actualRuntime)
	if actualAgent != specialist || actualRuntime != runtime || dbfx.Count(t, `SELECT count(*) FROM issue_workflow_plan WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("explicit specialist route was replaced or given a legacy plan")
	}
	current, err = testHandler.Queries.GetIssue(ctx, parseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := service.ProposeIssueWorkflowPlan(ctx, testHandler.Queries, current); err != nil || ok {
		t.Fatalf("driver adopted admitted ordinary Chat: ok=%v err=%v", ok, err)
	}
	for _, selected := range []string{specialist, generic} {
		replay := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Research the dashboard.", "agent_id": selected}))
		replay.Header = req.Header.Clone()
		expected := 200
		if selected != specialist {
			expected = 409
		}
		testutil.Call(t, testHandler.CreateChatIntake, replay).Want(expected)
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 1 {
		t.Fatal("worker-selection replay duplicated work")
	}
	other := dbfx.Issue(t, "Keep assigned route", testutil.Cols{"project_id": project, "assignee_type": "agent", "assignee_id": generic})
	linkDeliveryIssue(t, session, other)
	replace := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": other, "request": "Keep this route.", "agent_id": specialist}))
	replace.Header = req.Header.Clone()
	testutil.Call(t, testHandler.CreateChatIntake, replace).Want(409)
}

func TestChatOpeningCorrectionKeepsExactInput(t *testing.T) {
	for _, provenance := range []string{"opening_turn", "current_turn", "unrelated_turn", "manual_link", "ambiguous", "decision", "stale_after_freeze"} {
		t.Run(provenance, func(t *testing.T) {
			ctx := context.Background()
			worker := newOrdinaryQuestionFixture(t)
			assistant, session, runtime, _ := setupDirectChatSession(t, ctx, "Opening correction race")
			opening := sendDirectChat(t, ctx, assistant, session, "Build a dashboard.")
			markTaskRunning(t, ctx, opening)
			words := "Show layout boxes first. Keep the actual columns and density."
			correction := sendDirectChat(t, ctx, assistant, session, words)
			markTaskRunning(t, ctx, correction)
			origin := opening
			if provenance == "current_turn" {
				origin = correction
			}
			if provenance == "unrelated_turn" {
				origin = dbfx.Task(t, assistant, testutil.Cols{"chat_session_id": session, "runtime_id": runtime, "status": "completed"})
			}
			dbfx.Exec(t, `UPDATE issue SET origin_type='agent_create',origin_id=$2,created_at=now() WHERE id=$1`, worker.issue, origin)
			linkDeliveryIssue(t, session, worker.issue)
			if provenance == "manual_link" {
				dbfx.Exec(t, `UPDATE chat_issue_link SET source='manual' WHERE issue_id=$1`, worker.issue)
			}
			if provenance == "ambiguous" {
				linkDeliveryIssue(t, session, dbfx.Issue(t, "Another new issue"))
			}
			if provenance == "stale_after_freeze" {
				stored, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(correction))
				if err != nil {
					t.Fatal(err)
				}
				cs, err := testHandler.Queries.GetChatSession(ctx, parseUUID(session))
				if err != nil {
					t.Fatal(err)
				}
				resolved, err := service.ResolveOpeningChatCorrection(ctx, testHandler.Queries, cs, stored)
				if err != nil {
					t.Fatal(err)
				}
				var target struct {
					Target service.ChatWorkTarget `json:"chat_work_target"`
				}
				_ = json.Unmarshal(resolved.Context, &target)
				if !target.Target.CorrectionOnly || target.Target.IssueID != worker.issue {
					t.Fatal("claim did not freeze opening correction")
				}
				dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, worker.issue)
			}
			req := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "correct"}))
			if provenance == "decision" {
				req = withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "approve"}))
			}
			req.Header.Set("X-Actor-Source", "task_token")
			req.Header.Set("X-Agent-ID", assistant)
			req.Header.Set("X-Task-ID", correction)
			if provenance != "opening_turn" && provenance != "current_turn" {
				testutil.Call(t, testHandler.SubmitChatWorkResponse, req).Want(409)
				if dbfx.Count(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, worker.issue) != 0 {
					t.Fatal("unbound correction wrote owner authority")
				}
				return
			}
			testutil.Call(t, testHandler.SubmitChatWorkResponse, req).WantOneOf(200, 201)
			var recorded string
			dbfx.QueryRow(t, `SELECT content FROM issue_owner_directive WHERE issue_id=$1`, worker.issue).Scan(&recorded)
			if recorded != words {
				t.Fatalf("owner input changed: %q", recorded)
			}
		})
	}
}

func TestChatBlockedWorkerReportIsNotReviewableResult(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Blocked worker")
	issue := dbfx.Issue(t, "Phone dashboard", testutil.Cols{"status": "blocked"})
	linkDeliveryIssue(t, session, issue)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "completed", "started_at": testutil.Raw("now()"), "completed_at": testutil.Raw("now()")})
	dbfx.Comment(t, issue, "I cannot proceed on this runtime.", testutil.Cols{"author_type": "agent", "author_id": agent, "source_task_id": task})
	deliverWork(t)
	rows := deliveryMessages(t, session)
	if len(rows) != 1 {
		t.Fatalf("updates=%d", len(rows))
	}
	var update service.ChatWorkUpdate
	if err := json.Unmarshal(rows[0].WorkUpdate, &update); err != nil {
		t.Fatal(err)
	}
	if update.Kind != "stopped" || strings.Contains(update.Body, "reviewable result") || !strings.Contains(update.Body, "cannot proceed") {
		t.Fatalf("blocked report mislabeled: %+v", update)
	}
	// Stable identity prevents a new stopped-kind receipt from duplicating an
	// update already delivered as result by the previous backend.
	dbfx.Exec(t, `UPDATE issue SET status='in_progress' WHERE id=$1`, issue)
	deliverWork(t)
	if len(deliveryMessages(t, session)) != 1 {
		t.Fatal("reclassification duplicated the immutable delivery")
	}
}

func TestChatBlockedWorkerKeepsArtifactsAndCustomStatus(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Blocked evidence")
	dbfx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": "hardware_wait", "name": "Hardware wait", "category": "blocked", "color": "#888888", "position": 0, "is_system": false})
	issue := dbfx.Issue(t, "Hardware evidence", testutil.Cols{"status": "hardware_wait"})
	linkDeliveryIssue(t, session, issue)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "completed", "completed_at": testutil.Raw("now()")})
	comment := dbfx.Comment(t, issue, "Logs attached; execution is blocked.", testutil.Cols{"author_type": "agent", "author_id": agent, "source_task_id": task})
	attachment := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": issue, "comment_id": comment, "uploader_type": "agent", "uploader_id": agent, "filename": "evidence.png", "url": "https://cdn.example/evidence.png", "content_type": "image/png", "size_bytes": 100})
	deliverWork(t)
	messages := deliveryMessages(t, session)
	var update service.ChatWorkUpdate
	if len(messages) != 1 || json.Unmarshal(messages[0].WorkUpdate, &update) != nil || update.Kind != "stopped" || update.Review == nil || len(update.Review.Artifacts) != 1 || update.Review.Artifacts[0].ID != attachment {
		t.Fatalf("blocked evidence lost: %+v", update)
	}
	// Neither a later block nor a newer task reclassifies older source evidence.
	dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "failed", "completed_at": testutil.Raw("now()")})
	rows, err := testHandler.Queries.ListUndeliveredChatWorkUpdates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.SourceID == parseUUID(task) {
			t.Fatal("older artifact report was redelivered")
		}
	}
}
