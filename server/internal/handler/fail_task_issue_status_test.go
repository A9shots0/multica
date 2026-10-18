package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type failTaskQueryFaultStarter struct {
	pool  *pgxpool.Pool
	match string
	err   error
}

func (s failTaskQueryFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &failTaskQueryFaultTx{Tx: tx, match: s.match, err: s.err}, nil
}

type failTaskQueryFaultTx struct {
	pgx.Tx
	match string
	err   error
}

func (tx *failTaskQueryFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, tx.match) {
		return failTaskFaultRow{err: tx.err}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

type failTaskFaultRow struct{ err error }

func (row failTaskFaultRow) Scan(...any) error { return row.err }

func TestFailTaskTerminalFailureBlocksIdleIssueWithDurableWait(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()

	runtimeID := dbfx.Runtime(t, "fail-status-reset-runtime")
	agentID := dbfx.Agent(t, "fail-status-reset-agent", runtimeID)
	issueID := dbfx.Issue(t, "terminal failure leaves no active carrier", testutil.Cols{
		"status":        "in_progress",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	runningID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id":     issueID,
		"runtime_id":   runtimeID,
		"status":       "running",
		"attempt":      1,
		"max_attempts": 2,
	})

	if _, err := testHandler.TaskService.FailTask(ctx, parseUUID(runningID),
		"worker exited", "", "", "", "agent_error.process_failure", false, "", ""); err != nil {
		t.Fatalf("FailTask: %v", err)
	}

	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(issueID))
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Status != "blocked" {
		t.Fatalf("terminal failure with no active carrier left issue status = %q, want blocked", issue.Status)
	}
	var markerTaskID string
	if err := testPool.QueryRow(ctx, `SELECT metadata->'terminal_task_failure_wait'->>'task_id' FROM issue WHERE id = $1`, parseUUID(issueID)).Scan(&markerTaskID); err != nil {
		t.Fatalf("load terminal failure wait marker: %v", err)
	}
	if markerTaskID != runningID {
		t.Fatalf("terminal failure wait task = %q, want %q", markerTaskID, runningID)
	}
}

func TestFailTaskKeepsIssueActiveWhileRetryIsPending(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()

	runtimeID := dbfx.Runtime(t, "fail-status-retry-runtime")
	agentID := dbfx.Agent(t, "fail-status-retry-agent", runtimeID)
	issueID := dbfx.Issue(t, "retry remains an active carrier", testutil.Cols{
		"status":        "in_progress",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	runningID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id":     issueID,
		"runtime_id":   runtimeID,
		"status":       "running",
		"attempt":      1,
		"max_attempts": 2,
	})

	if _, err := testHandler.TaskService.FailTask(ctx, parseUUID(runningID),
		"task timed out", "", "", "", "timeout", false, "", ""); err != nil {
		t.Fatalf("FailTask: %v", err)
	}

	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(issueID))
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Status != "in_progress" {
		t.Fatalf("retry-pending issue status = %q, want in_progress", issue.Status)
	}
	var retries int
	if err := testPool.QueryRow(ctx,
		`SELECT count(*) FROM agent_task_queue WHERE parent_task_id = $1 AND status IN ('queued', 'deferred')`,
		parseUUID(runningID)).Scan(&retries); err != nil {
		t.Fatalf("count retries: %v", err)
	}
	if retries != 1 {
		t.Fatalf("pending retries = %d, want 1", retries)
	}
}

func TestFailTaskKeepsIssueActiveWhileAnotherCarrierRuns(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()

	runtimeID := dbfx.Runtime(t, "fail-status-sibling-runtime")
	agentID := dbfx.Agent(t, "fail-status-sibling-agent", runtimeID)
	issueID := dbfx.Issue(t, "another carrier remains active", testutil.Cols{
		"status":        "in_progress",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	runningID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id":   issueID,
		"runtime_id": runtimeID,
		"status":     "running",
	})
	dbfx.Task(t, agentID, testutil.Cols{
		"issue_id":   issueID,
		"runtime_id": runtimeID,
		"status":     "running",
	})

	if _, err := testHandler.TaskService.FailTask(ctx, parseUUID(runningID),
		"worker exited", "", "", "", "agent_error.process_failure", false, "", ""); err != nil {
		t.Fatalf("FailTask: %v", err)
	}

	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(issueID))
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Status != "in_progress" {
		t.Fatalf("issue with another active carrier status = %q, want in_progress", issue.Status)
	}
}

func TestFailTaskReconciliationErrorRollsBackTerminalTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	tests := []struct {
		name  string
		match string
	}{
		{name: "active carrier query", match: "-- name: HasActiveTaskForIssue :one"},
		{name: "issue status update", match: "-- name: BlockIssueForTerminalTaskFailure :one"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			runtimeID := dbfx.Runtime(t, "fail-reconcile-fault-runtime-"+strings.ReplaceAll(tc.name, " ", "-"))
			agentID := dbfx.Agent(t, "fail-reconcile-fault-agent-"+strings.ReplaceAll(tc.name, " ", "-"), runtimeID)
			issueID := dbfx.Issue(t, "terminal cleanup fault must roll back "+tc.name, testutil.Cols{
				"status":        "in_progress",
				"assignee_type": "agent",
				"assignee_id":   agentID,
			})
			runningID := dbfx.Task(t, agentID, testutil.Cols{
				"issue_id":   issueID,
				"runtime_id": runtimeID,
				"status":     "running",
			})

			injected := errors.New("injected reconciliation failure: " + tc.name)
			origStarter := testHandler.TaskService.TxStarter
			testHandler.TaskService.TxStarter = failTaskQueryFaultStarter{pool: testPool, match: tc.match, err: injected}
			_, failErr := testHandler.TaskService.FailTask(ctx, parseUUID(runningID),
				"worker exited", "", "", "", "agent_error.process_failure", false, "", "")
			testHandler.TaskService.TxStarter = origStarter
			if !errors.Is(failErr, injected) {
				t.Fatalf("FailTask error = %v, want injected reconciliation failure", failErr)
			}

			var taskStatus, issueStatus string
			if err := testPool.QueryRow(ctx,
				`SELECT status FROM agent_task_queue WHERE id = $1`, parseUUID(runningID)).Scan(&taskStatus); err != nil {
				t.Fatalf("load task after rollback: %v", err)
			}
			if err := testPool.QueryRow(ctx,
				`SELECT status FROM issue WHERE id = $1`, parseUUID(issueID)).Scan(&issueStatus); err != nil {
				t.Fatalf("load issue after rollback: %v", err)
			}
			if taskStatus != "running" || issueStatus != "in_progress" {
				t.Fatalf("cleanup fault committed partial terminal state: task=%q issue=%q, want running/in_progress", taskStatus, issueStatus)
			}
		})
	}
}

func TestRecoverOrphanedTasksRollsBackTerminalRowsWhenIssueReconciliationFails(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := dbfx.Runtime(t, "bulk-terminal-atomic-runtime")
	agentID := dbfx.Agent(t, "bulk-terminal-atomic-agent", runtimeID)
	issueID := dbfx.Issue(t, "bulk terminalization is atomic with issue reconciliation", testutil.Cols{
		"status": "in_progress", "assignee_type": "agent", "assignee_id": agentID,
	})
	runningID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id": issueID, "runtime_id": runtimeID, "status": "running",
	})

	injected := errors.New("injected bulk issue reconciliation failure")
	origStarter := testHandler.TaskService.TxStarter
	testHandler.TaskService.TxStarter = failTaskQueryFaultStarter{
		pool: testPool, match: "-- name: BlockIssueForTerminalTaskFailure :one", err: injected,
	}
	_, recoverErr := testHandler.TaskService.RecoverOrphanedTasksForRuntime(ctx, parseUUID(runtimeID))
	testHandler.TaskService.TxStarter = origStarter
	if !errors.Is(recoverErr, injected) {
		t.Fatalf("RecoverOrphanedTasksForRuntime error = %v, want injected failure", recoverErr)
	}

	var taskStatus, issueStatus string
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, parseUUID(runningID)).Scan(&taskStatus); err != nil {
		t.Fatalf("load task after rollback: %v", err)
	}
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, parseUUID(issueID)).Scan(&issueStatus); err != nil {
		t.Fatalf("load issue after rollback: %v", err)
	}
	if taskStatus != "running" || issueStatus != "in_progress" {
		t.Fatalf("bulk cleanup fault committed partial state: task=%q issue=%q, want running/in_progress", taskStatus, issueStatus)
	}
}

func TestReconcileTerminalFailedIssuesRepairsCommittedCrashWindow(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := dbfx.Runtime(t, "terminal-restart-repair-runtime")
	agentID := dbfx.Agent(t, "terminal-restart-repair-agent", runtimeID)
	issueID := dbfx.Issue(t, "restart repairs committed failure projection", testutil.Cols{
		"status": "in_progress", "assignee_type": "agent", "assignee_id": agentID,
	})
	failedID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id": issueID, "runtime_id": runtimeID, "status": "failed",
		"error": "process exited before issue reconciliation", "failure_reason": "runtime_recovery",
	})

	reconciled, err := testHandler.TaskService.ReconcileTerminalFailedIssues(ctx, 100)
	if err != nil {
		t.Fatalf("ReconcileTerminalFailedIssues: %v", err)
	}
	if reconciled < 1 {
		t.Fatalf("reconciled = %d, want at least 1", reconciled)
	}
	var status, markerTaskID string
	if err := testPool.QueryRow(ctx, `SELECT status, metadata->'terminal_task_failure_wait'->>'task_id' FROM issue WHERE id = $1`, parseUUID(issueID)).Scan(&status, &markerTaskID); err != nil {
		t.Fatalf("load repaired issue: %v", err)
	}
	if status != "blocked" || markerTaskID != failedID {
		t.Fatalf("repaired issue = status %q marker %q, want blocked/%q", status, markerTaskID, failedID)
	}
	second, err := testHandler.TaskService.ReconcileTerminalFailedIssues(ctx, 100)
	if err != nil {
		t.Fatalf("second ReconcileTerminalFailedIssues: %v", err)
	}
	if second != 0 {
		t.Fatalf("second reconciliation = %d, want idempotent 0", second)
	}
}

func TestConcurrentTerminalFailureAndTaskAdmissionPreserveActiveIssue(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := dbfx.Runtime(t, "terminal-admission-race-runtime")
	agentID := dbfx.Agent(t, "terminal-admission-race-agent", runtimeID)
	issueID := dbfx.Issue(t, "terminal failure racing task admission", testutil.Cols{
		"status": "in_progress", "assignee_type": "agent", "assignee_id": agentID,
	})
	runningID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id": issueID, "runtime_id": runtimeID, "status": "running", "attempt": 1, "max_attempts": 1,
	})
	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(issueID))
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}

	barrier, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin race barrier: %v", err)
	}
	if _, err := barrier.Exec(ctx, `SELECT id FROM issue WHERE id = $1 FOR UPDATE`, parseUUID(issueID)); err != nil {
		_ = barrier.Rollback(ctx)
		t.Fatalf("lock issue race barrier: %v", err)
	}
	failDone := make(chan error, 1)
	go func() {
		_, failErr := testHandler.TaskService.FailTask(context.Background(), parseUUID(runningID),
			"worker exited", "", "", "", "agent_error.process_failure", false, "", "")
		failDone <- failErr
	}()
	time.Sleep(50 * time.Millisecond)
	enqueueDone := make(chan error, 1)
	go func() {
		_, enqueueErr := testHandler.TaskService.EnqueueTaskForIssue(context.Background(), issue)
		enqueueDone <- enqueueErr
	}()
	time.Sleep(50 * time.Millisecond)
	if err := barrier.Commit(ctx); err != nil {
		t.Fatalf("release race barrier: %v", err)
	}
	for name, done := range map[string]<-chan error{"failure": failDone, "enqueue": enqueueDone} {
		select {
		case opErr := <-done:
			if opErr != nil {
				t.Fatalf("%s side of race: %v", name, opErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s side of race timed out", name)
		}
	}
	var status string
	var hasMarker bool
	var active int
	if err := testPool.QueryRow(ctx, `
		SELECT i.status, i.metadata ? 'terminal_task_failure_wait',
		       count(t.id) FILTER (WHERE t.status IN ('queued','dispatched','running','waiting_local_directory'))
		FROM issue i LEFT JOIN agent_task_queue t ON t.issue_id = i.id
		WHERE i.id = $1 GROUP BY i.id
	`, parseUUID(issueID)).Scan(&status, &hasMarker, &active); err != nil {
		t.Fatalf("load race result: %v", err)
	}
	if status != "in_progress" || hasMarker || active == 0 {
		t.Fatalf("race result status=%q marker=%v active=%d, want in_progress/false/>0", status, hasMarker, active)
	}
}
