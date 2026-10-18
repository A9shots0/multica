package handler

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestStalledWorkDetectorReportsOnlyThresholdedUnactionableIssues(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	if err := testHandler.Queries.SeedIssueStatusEntries(ctx, parseUUID(testWorkspaceID)); err != nil {
		t.Fatalf("seed issue statuses: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-time.Hour)

	parentID := dbfx.Issue(t, "stalled detector parent", testutil.Cols{
		"status": "in_progress", "updated_at": old,
	})
	stalledChildID := dbfx.Issue(t, "stalled detector in-review child", testutil.Cols{
		"status": "in_review", "parent_issue_id": parentID, "stage": 1, "updated_at": old,
	})
	runningIssueID := dbfx.Issue(t, "stalled detector running issue", testutil.Cols{
		"status": "in_progress", "updated_at": old,
	})
	doneID := dbfx.Issue(t, "stalled detector done issue", testutil.Cols{
		"status": "done", "updated_at": old,
	})
	cancelledID := dbfx.Issue(t, "stalled detector cancelled issue", testutil.Cols{
		"status": "cancelled", "updated_at": old,
	})
	freshID := dbfx.Issue(t, "stalled detector fresh issue", testutil.Cols{
		"status": "in_progress", "updated_at": now,
	})

	agentID := dbfx.Agent(t, "stalled detector running agent", handlerTestRuntimeID(t))
	reviewerID := dbfx.Agent(t, "stalled detector reviewer", handlerTestRuntimeID(t))
	squadID := dbfx.Squad(t, "stalled detector squad", agentID)
	nonActionablePlanID := dbfx.Issue(t, "stalled detector plan without next step", testutil.Cols{
		"status": "in_review", "updated_at": old,
	})
	actionablePlanID := dbfx.Issue(t, "stalled detector actionable plan", testutil.Cols{
		"status": "in_review", "updated_at": old,
	})
	for issueID, state := range map[string]string{nonActionablePlanID: "needs_owner", actionablePlanID: "technical_review"} {
		dbfx.InsertNoID(t, "issue_workflow_plan", testutil.Cols{
			"issue_id": issueID, "workspace_id": testWorkspaceID, "squad_id": squadID,
			"implementer_id": agentID, "reviewer_id": reviewerID,
			"current_state": state, "updated_at": old,
		}, "issue_id = $1", issueID)
	}
	dbfx.Task(t, agentID, testutil.Cols{
		"issue_id": runningIssueID, "runtime_id": handlerTestRuntimeID(t),
		"status": "running", "started_at": old,
	})

	issueIDs := []string{
		parentID, stalledChildID, runningIssueID, doneID, cancelledID, freshID, nonActionablePlanID, actionablePlanID,
	}
	beforeTasks := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = ANY($1::uuid[])`, issueIDs)
	beforeNotifications := dbfx.Count(t, `SELECT count(*) FROM owner_notification WHERE issue_id = ANY($1::uuid[])`, issueIDs)
	detector := service.StalledWorkDetector{
		Queries: testHandler.Queries, Threshold: 15 * time.Minute, Limit: 100,
		Now: func() time.Time { return now },
	}
	report, err := detector.DetectOnce(ctx)
	if err != nil {
		t.Fatalf("detect stalled work: %v", err)
	}

	if report.Query != "ListStalledWork" || report.Threshold != "15m0s" {
		t.Fatalf("report provenance = query %q threshold %q", report.Query, report.Threshold)
	}
	if report.Counts.Matched < 2 || report.Counts.Returned != len(report.Issues) || report.Truncated {
		t.Fatalf("report counts = %+v truncated=%v issues=%d", report.Counts, report.Truncated, len(report.Issues))
	}
	if len(report.CannotSee) == 0 {
		t.Fatal("report omitted detector visibility limits")
	}
	ids := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		ids = append(ids, issue.IssueID)
	}
	if !slices.Contains(ids, stalledChildID) {
		t.Fatalf("in_review child without a controller was not detected: %v", ids)
	}
	if !slices.Contains(ids, nonActionablePlanID) {
		t.Fatalf("issue whose workflow plan has no actionable step was not detected: %v", ids)
	}
	for _, issue := range report.Issues {
		if issue.IssueID == stalledChildID && (issue.HealthState != "stalled" || issue.Reason != "workflow_plan_missing") {
			t.Fatalf("in_review child health = %+v, want explicit stalled/missing-plan report", issue)
		}
	}
	for _, excluded := range []string{runningIssueID, doneID, cancelledID, freshID, actionablePlanID} {
		if slices.Contains(ids, excluded) {
			t.Errorf("ineligible issue %s was reported stalled", excluded)
		}
	}

	expectedStatuses := map[string]string{
		parentID:            "in_progress",
		stalledChildID:      "in_review",
		runningIssueID:      "in_progress",
		doneID:              "done",
		cancelledID:         "cancelled",
		freshID:             "in_progress",
		nonActionablePlanID: "in_review",
		actionablePlanID:    "in_review",
	}
	for issueID, expected := range expectedStatuses {
		var actual string
		dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&actual)
		if actual != expected {
			t.Errorf("detector changed issue %s status from %q to %q", issueID, expected, actual)
		}
	}
	afterTasks := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = ANY($1::uuid[])`, issueIDs)
	afterNotifications := dbfx.Count(t, `SELECT count(*) FROM owner_notification WHERE issue_id = ANY($1::uuid[])`, issueIDs)
	if afterTasks != beforeTasks || afterNotifications != beforeNotifications {
		t.Fatalf("detector produced side effects: tasks %d -> %d, notifications %d -> %d",
			beforeTasks, afterTasks, beforeNotifications, afterNotifications)
	}
}
