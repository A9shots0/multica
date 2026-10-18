package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Regression tests for two silent WorkflowDriver gaps found by real empirical
// testing against a throwaway Postgres:
//
// Gap A (no-squad-template stall): an unassigned issue in a workspace with no
// squad workflow template is picked up by drivableIssuesSQL every sweep,
// EnsureIssueWorkflowPlan finds no squad, the action becomes `needs_plan`
// (DriveOnce's default: report.Skipped++ branch), and nothing else ever
// happens — no plan, no task, no inbox item, forever.
//
// Gap B (silent squad-template auto-assign): same unassigned issue, but in a
// workspace that DOES have a squad workflow template. ProposeIssueWorkflowPlan
// (workflow_plan_ensure.go:103-107) falls back to the template's own default
// implementer even though the issue itself has no assignee. Pass 1 installs
// the plan, pass 2 the driver assigns the issue to that agent and dispatches a
// real task — all without ever telling the owner.
//
// Both are fixed by workflow_driver.go raising a one-time, deduplicated
// inbox_item (workflowNeedsPlanInboxType / workflowAutoAssignedInboxType),
// using the exact dedup shape (issue_id, type, archived=false) that escalate()
// and surfaceOwnerWaits()/surfaceStoppedWorkflows() already use elsewhere in
// this file.

// newWorkflowDriverVisibilityFixture builds a workspace with one owner member,
// ready for either gap's issue to be seeded on top of it.
type workflowDriverVisibilityWorkspace struct {
	workspaceID pgtype.UUID
	userID      pgtype.UUID
}

func seedWorkflowDriverVisibilityWorkspace(t *testing.T, pool *pgxpool.Pool, label string) workflowDriverVisibilityWorkspace {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()

	var userID, workspaceID string
	if err := pool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ($1, $2) RETURNING id`,
		"WFD Visibility "+label, fmt.Sprintf("wfd-vis-%s-%d@multica.test", label, suffix)).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })

	if err := pool.QueryRow(ctx, `INSERT INTO workspace (name, slug) VALUES ($1, $2) RETURNING id`,
		"wfd vis "+label, fmt.Sprintf("wfd-vis-%s-%d", label, suffix)).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID) })

	if _, err := pool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`,
		workspaceID, userID); err != nil {
		t.Fatalf("seed member: %v", err)
	}

	return workflowDriverVisibilityWorkspace{
		workspaceID: util.MustParseUUID(workspaceID),
		userID:      util.MustParseUUID(userID),
	}
}

// seedUnassignedIssue creates an issue with no assignee, backdated well past
// any driver quiet period, so it reads as drivable on every sweep without
// racing a "just changed" guard.
func seedUnassignedIssue(t *testing.T, pool *pgxpool.Pool, ws workflowDriverVisibilityWorkspace, title string) pgtype.UUID {
	t.Helper()
	ctx := context.Background()
	var issueID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, creator_type, creator_id)
		VALUES ($1, $2, 'member', $3)
		RETURNING id
	`, util.UUIDToString(ws.workspaceID), title, util.UUIDToString(ws.userID)).Scan(&issueID); err != nil {
		t.Fatalf("seed issue: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE issue SET updated_at = now() - interval '2 hours' WHERE id = $1`, issueID); err != nil {
		t.Fatalf("backdate issue: %v", err)
	}
	return util.MustParseUUID(issueID)
}

func countInboxItems(t *testing.T, pool *pgxpool.Pool, issueID pgtype.UUID, itemType string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = $2`,
		issueID, itemType).Scan(&n); err != nil {
		t.Fatalf("count inbox items: %v", err)
	}
	return n
}

// newVisibilityDriver builds a WorkflowDriver with a tiny quiet period so
// consecutive DriveOnce calls in a test can each see the issue as drivable,
// the way three real 60-second production sweeps would.
func newVisibilityDriver(pool *pgxpool.Pool, q *db.Queries, svc *TaskService) *WorkflowDriver {
	return &WorkflowDriver{
		Pool:        pool,
		Queries:     q,
		TaskService: svc,
		Bus:         events.New(),
		QuietPeriod: time.Millisecond,
	}
}

// TestWorkflowDriverGapA_NeedsPlanStallRaisesOwnerInboxItemOnce is the RED/GREEN
// pin for Gap A. Before the fix, three consecutive real DriveOnce sweeps over
// an unassigned issue in a workspace with no squad workflow template produced
// an identical needs_plan outcome every time and zero inbox_item rows. After
// the fix, the first sweep raises exactly one owner-visible inbox item, and
// sweeps two and three must not duplicate it.
func TestWorkflowDriverGapA_NeedsPlanStallRaisesOwnerInboxItemOnce(t *testing.T) {
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	q := db.New(pool)

	ws := seedWorkflowDriverVisibilityWorkspace(t, pool, "gapa")
	issueID := seedUnassignedIssue(t, pool, ws, "gap A: nobody can work this")
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM inbox_item WHERE issue_id = $1`, issueID)
		pool.Exec(context.Background(), `DELETE FROM issue_workflow_plan WHERE issue_id = $1`, issueID)
		pool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	// A TaskService is required by DriveOnce whenever DryRun is false, even
	// though Gap A's path never reaches dispatch.
	svc := &TaskService{Queries: q, TxStarter: pool, Bus: events.New()}
	driver := newVisibilityDriver(pool, q, svc)

	for sweep := 1; sweep <= 3; sweep++ {
		report, err := driver.DriveOnce(ctx)
		if err != nil {
			t.Fatalf("sweep %d: DriveOnce: %v", sweep, err)
		}
		var sawIssue bool
		var action DriveAction
		for _, a := range report.Actions {
			if a.IssueID == util.UUIDToString(issueID) {
				sawIssue, action = true, a
			}
		}
		if !sawIssue {
			t.Fatalf("sweep %d: issue was not scanned at all (report=%+v)", sweep, report)
		}
		if action.Action != "needs_plan" {
			t.Fatalf("sweep %d: action = %q, want needs_plan", sweep, action.Action)
		}

		got := countInboxItems(t, pool, issueID, workflowNeedsPlanInboxType)
		if got != 1 {
			t.Fatalf("sweep %d: workflow_needs_plan inbox items = %d, want exactly 1 (no spam, no gap)", sweep, got)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The item must actually be addressed to the workspace owner and carry
	// enough for the Inbox "needs you" surface to group it correctly.
	var recipientID, title, body, requiredAction string
	if err := pool.QueryRow(ctx, `
		SELECT recipient_id::text, title, coalesce(body,''), details->>'required_action'
		FROM inbox_item WHERE issue_id = $1 AND type = $2
	`, issueID, workflowNeedsPlanInboxType).Scan(&recipientID, &title, &body, &requiredAction); err != nil {
		t.Fatalf("read raised inbox item: %v", err)
	}
	if recipientID != util.UUIDToString(ws.userID) {
		t.Errorf("recipient = %s, want workspace owner %s", recipientID, util.UUIDToString(ws.userID))
	}
	if requiredAction == "" {
		t.Errorf("required_action detail is empty; the owner Inbox needs-you surface keys on it")
	}
	if body == "" {
		t.Errorf("inbox item body is empty")
	}
}

// TestWorkflowDriverGapB_SquadDefaultAutoAssignRaisesOwnerInboxItemOnce is the
// RED/GREEN pin for Gap B. An unassigned issue in a workspace that DOES have a
// squad workflow template gets a plan installed on pass 1 (received state,
// implementer = the template's default agent per workflow_plan_ensure.go:
// 103-107), then on pass 2 the driver assigns the issue to that agent and
// dispatches a real task. Before the fix that hand-off was completely silent.
// After the fix it raises exactly one owner-visible inbox item.
func TestWorkflowDriverGapB_SquadDefaultAutoAssignRaisesOwnerInboxItemOnce(t *testing.T) {
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	q := db.New(pool)

	ws := seedWorkflowDriverVisibilityWorkspace(t, pool, "gapb")

	var runtimeID, implementerID, reviewerID, squadID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (workspace_id, name, runtime_mode, provider, status, device_info, metadata, owner_id)
		VALUES ($1, 'gapb-runtime', 'cloud', 'codex', 'online', '', '{}'::jsonb, $2)
		RETURNING id
	`, util.UUIDToString(ws.workspaceID), util.UUIDToString(ws.userID)).Scan(&runtimeID); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, runtime_mode, runtime_config, runtime_id, visibility,
			max_concurrent_tasks, owner_id, instructions, custom_env, custom_args)
		VALUES ($1, 'gapb-implementer', 'cloud', '{}'::jsonb, $2, 'workspace', 1, $3, '', '{}'::jsonb, '[]'::jsonb)
		RETURNING id
	`, util.UUIDToString(ws.workspaceID), runtimeID, util.UUIDToString(ws.userID)).Scan(&implementerID); err != nil {
		t.Fatalf("seed implementer agent: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, runtime_mode, runtime_config, runtime_id, visibility,
			max_concurrent_tasks, owner_id, instructions, custom_env, custom_args)
		VALUES ($1, 'gapb-reviewer', 'cloud', '{}'::jsonb, $2, 'workspace', 1, $3, '', '{}'::jsonb, '[]'::jsonb)
		RETURNING id
	`, util.UUIDToString(ws.workspaceID), runtimeID, util.UUIDToString(ws.userID)).Scan(&reviewerID); err != nil {
		t.Fatalf("seed reviewer agent: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO squad (workspace_id, name, leader_id, creator_id)
		VALUES ($1, 'gapb-squad', $2, $3)
		RETURNING id
	`, util.UUIDToString(ws.workspaceID), reviewerID, util.UUIDToString(ws.userID)).Scan(&squadID); err != nil {
		t.Fatalf("seed squad: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO squad_workflow_template (squad_id, workspace_id, implementer_id, reviewer_id)
		VALUES ($1, $2, $3, $4)
	`, squadID, util.UUIDToString(ws.workspaceID), implementerID, reviewerID); err != nil {
		t.Fatalf("seed squad workflow template: %v", err)
	}

	issueID := seedUnassignedIssue(t, pool, ws, "gap B: silently auto-assigned")
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM inbox_item WHERE issue_id = $1`, issueID)
		pool.Exec(c, `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
		pool.Exec(c, `DELETE FROM issue_workflow_plan WHERE issue_id = $1`, issueID)
		pool.Exec(c, `DELETE FROM issue WHERE id = $1`, issueID)
		pool.Exec(c, `DELETE FROM squad_workflow_template WHERE squad_id = $1`, squadID)
		pool.Exec(c, `DELETE FROM squad WHERE id = $1`, squadID)
		pool.Exec(c, `DELETE FROM agent WHERE id = ANY($1)`, []string{implementerID, reviewerID})
		pool.Exec(c, `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})

	svc := &TaskService{Queries: q, TxStarter: pool, Bus: events.New()}
	driver := newVisibilityDriver(pool, q, svc)

	// Pass 1: installs the controller. No assignment, no dispatch, no notice yet.
	report1, err := driver.DriveOnce(ctx)
	if err != nil {
		t.Fatalf("pass 1: DriveOnce: %v", err)
	}
	var action1 DriveAction
	for _, a := range report1.Actions {
		if a.IssueID == util.UUIDToString(issueID) {
			action1 = a
		}
	}
	if action1.Action != "planned" {
		t.Fatalf("pass 1: action = %q, want planned (report=%+v)", action1.Action, report1)
	}
	if got := countInboxItems(t, pool, issueID, workflowAutoAssignedInboxType); got != 0 {
		t.Fatalf("pass 1: workflow_auto_assigned inbox items = %d, want 0 (nothing was assigned yet)", got)
	}

	var assigneeAfterPlan pgtype.UUID
	if err := pool.QueryRow(ctx, `SELECT assignee_id FROM issue WHERE id = $1`, util.UUIDToString(issueID)).Scan(&assigneeAfterPlan); err != nil {
		t.Fatalf("read assignee after plan: %v", err)
	}
	if assigneeAfterPlan.Valid {
		t.Fatalf("pass 1: issue already has an assignee (%s); the auto-assign has not happened yet, so this fixture is wrong", util.UUIDToString(assigneeAfterPlan))
	}

	time.Sleep(5 * time.Millisecond)

	// Pass 2: the driver assigns the issue to the template's default implementer
	// and dispatches a real task.
	report2, err := driver.DriveOnce(ctx)
	if err != nil {
		t.Fatalf("pass 2: DriveOnce: %v", err)
	}
	var action2 DriveAction
	var sawIssue2 bool
	for _, a := range report2.Actions {
		if a.IssueID == util.UUIDToString(issueID) {
			sawIssue2, action2 = true, a
		}
	}
	if !sawIssue2 {
		t.Fatalf("pass 2: issue was not scanned (report=%+v)", report2)
	}
	if action2.Action != "resumed" {
		t.Fatalf("pass 2: action = %q, want resumed (a real task should have been dispatched); detail=%q", action2.Action, action2.Detail)
	}

	var assigneeID pgtype.UUID
	if err := pool.QueryRow(ctx, `SELECT assignee_id FROM issue WHERE id = $1`, util.UUIDToString(issueID)).Scan(&assigneeID); err != nil {
		t.Fatalf("read assignee after pass 2: %v", err)
	}
	if !assigneeID.Valid || util.UUIDToString(assigneeID) != implementerID {
		t.Fatalf("assignee after pass 2 = %s, want the template's default implementer %s", util.UUIDToString(assigneeID), implementerID)
	}

	var taskCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, util.UUIDToString(issueID)).Scan(&taskCount); err != nil {
		t.Fatalf("count dispatched tasks: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("dispatched task count = %d, want 1 (a real task was dispatched to the auto-assigned agent)", taskCount)
	}

	got := countInboxItems(t, pool, issueID, workflowAutoAssignedInboxType)
	if got != 1 {
		t.Fatalf("workflow_auto_assigned inbox items after auto-assign = %d, want exactly 1", got)
	}

	var recipientID, body, requiredAction, assignedAgentDetail string
	if err := pool.QueryRow(ctx, `
		SELECT recipient_id::text, coalesce(body,''), details->>'required_action', coalesce(details->>'assigned_agent_id','')
		FROM inbox_item WHERE issue_id = $1 AND type = $2
	`, issueID, workflowAutoAssignedInboxType).Scan(&recipientID, &body, &requiredAction, &assignedAgentDetail); err != nil {
		t.Fatalf("read raised inbox item: %v", err)
	}
	if recipientID != util.UUIDToString(ws.userID) {
		t.Errorf("recipient = %s, want workspace owner %s", recipientID, util.UUIDToString(ws.userID))
	}
	if requiredAction == "" {
		t.Errorf("required_action detail is empty; the owner Inbox needs-you surface keys on it")
	}
	if assignedAgentDetail != implementerID {
		t.Errorf("assigned_agent_id detail = %q, want %q", assignedAgentDetail, implementerID)
	}
	if body == "" {
		t.Errorf("inbox item body is empty")
	}

	// Pass 3: the issue now has a live dispatched task, so it drops out of the
	// drivable set entirely — the strongest possible guarantee against a
	// duplicate notice (the condition that triggers it structurally cannot
	// recur once the issue has a real assignee).
	time.Sleep(5 * time.Millisecond)
	report3, err := driver.DriveOnce(ctx)
	if err != nil {
		t.Fatalf("pass 3: DriveOnce: %v", err)
	}
	for _, a := range report3.Actions {
		if a.IssueID == util.UUIDToString(issueID) {
			t.Fatalf("pass 3: issue was scanned again while its task is still live (action=%+v)", a)
		}
	}
	if got := countInboxItems(t, pool, issueID, workflowAutoAssignedInboxType); got != 1 {
		t.Fatalf("pass 3: workflow_auto_assigned inbox items = %d, want still exactly 1", got)
	}
}
