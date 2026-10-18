package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type workflowControlFixture struct {
	IssueID       string
	SquadID       string
	RuntimeID     string
	LeaderID      string
	ImplementerID string
	ReviewerID    string
	SourceTaskID  string
}

func newWorkflowControlFixture(t *testing.T, runtimeIDs ...string) workflowControlFixture {
	t.Helper()
	runtimeID := handlerTestRuntimeID(t)
	if len(runtimeIDs) > 0 {
		runtimeID = runtimeIDs[0]
	}
	leaderID := createHandlerTestAgent(t, "Workflow Leader", nil)
	implementerID := createHandlerTestAgent(t, "Workflow Implementer", nil)
	reviewerID := createHandlerTestAgent(t, "Workflow Reviewer", nil)
	for _, agentID := range []string{leaderID, implementerID, reviewerID} {
		dbfx.Exec(t, `UPDATE agent SET runtime_id = $2, model = 'fixture/model', thinking_level = 'medium' WHERE id = $1`, agentID, runtimeID)
	}
	squadID := dbfx.Squad(t, "Workflow Squad", leaderID)
	dbfx.SquadMember(t, squadID, "agent", leaderID, testutil.Cols{"role": "leader"})
	dbfx.SquadMember(t, squadID, "agent", implementerID, testutil.Cols{"role": "implementer"})
	dbfx.SquadMember(t, squadID, "agent", reviewerID, testutil.Cols{"role": "reviewer"})
	issueID := dbfx.Issue(t, "deterministic workflow", testutil.Cols{
		"assignee_type": "squad",
		"assignee_id":   squadID,
	})
	sourceTaskID := dbfx.Task(t, implementerID, testutil.Cols{
		"runtime_id":          runtimeID,
		"status":              "running",
		"issue_id":            issueID,
		"started_at":          testutil.Raw("now()"),
		"squad_id":            squadID,
		"originator_user_id":  testUserID,
		"accountable_user_id": testUserID,
	})
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue_workflow_event WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue_workflow_plan WHERE issue_id = $1`, issueID)
	})
	return workflowControlFixture{
		IssueID: issueID, SquadID: squadID, RuntimeID: runtimeID, LeaderID: leaderID,
		ImplementerID: implementerID, ReviewerID: reviewerID, SourceTaskID: sourceTaskID,
	}
}

func TestEnsureWorkflowControllerFailsClosedWhenSquadTemplateIsMissing(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(fx.IssueID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	configured, err := service.EnsureWorkflowController(context.Background(), testHandler.Queries, testHandler.Bus, issue)
	if configured || err == nil || !strings.Contains(err.Error(), "no workflow controller template") {
		t.Fatalf("missing squad template did not fail closed: configured=%v err=%v", configured, err)
	}
	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatalf("read blocked issue status: %v", err)
	}
	if status != "blocked" {
		t.Fatalf("issue status = %q, want blocked", status)
	}
	var notices int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM inbox_item
		WHERE issue_id = $1 AND type = 'workflow_controller_unavailable' AND archived = false
	`, fx.IssueID).Scan(&notices); err != nil {
		t.Fatalf("count recovery notices: %v", err)
	}
	if notices != 1 {
		t.Fatalf("recovery notices = %d, want 1", notices)
	}
}

func putWorkflowPlanRequest(fx workflowControlFixture) *http.Request {
	return withURLParam(newRequest(http.MethodPut, "/api/issues/"+fx.IssueID+"/workflow-plan", map[string]any{
		"squad_id": fx.SquadID, "implementer_id": fx.ImplementerID, "reviewer_id": fx.ReviewerID,
	}), "id", fx.IssueID)
}

func workflowEventRequest(fx workflowControlFixture, event workflowstate.Event) *http.Request {
	return workflowEventRequestFor(fx, event, fx.ImplementerID, fx.SourceTaskID, nil)
}

func workflowEventRequestFor(fx workflowControlFixture, event workflowstate.Event, agentID, taskID string, correction map[string]any) *http.Request {
	body := map[string]any{"event": event, "reason": "scenario evidence"}
	if event == workflowstate.ImplementationReady {
		body["artifact_evidence"] = validRepositoryInventoryEvidence()
	}
	if event == workflowstate.ReviewPassed {
		body["test_evidence"] = validWorkflowTestEvidence()
		body["blast_radius_review"] = validWorkflowBlastRadiusReview()
	}
	if correction != nil {
		body["correction"] = correction
	}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Task-ID", taskID)
	return req
}

func validRepositoryInventoryEvidence() map[string]any {
	roots := []workflowInventoryRootV1{
		{Path: `C:\Dev\CRM`, Branch: "main", HeadSHA: strings.Repeat("a", 40), TrackedChangeCount: 2, UntrackedFileCount: 1, Dirty: true},
		{Path: `C:\Dev\linkaggregator`, Branch: "main", HeadSHA: strings.Repeat("b", 40), Dirty: false},
	}
	worktrees := []workflowInventoryWorktreeV1{
		{RootPath: roots[0].Path, Path: roots[0].Path, Branch: roots[0].Branch, HeadSHA: roots[0].HeadSHA, PathChecked: true, PathPresent: true},
		{RootPath: roots[1].Path, Path: `C:\Dev\linkaggregator-review`, Branch: "review", HeadSHA: strings.Repeat("c", 40), PathChecked: true, PathPresent: false},
	}
	recordsSHA256, err := workflowInventoryRecordsSHA256(roots, worktrees)
	if err != nil {
		panic(err)
	}
	packet := workflowArtifactEvidencePacketV1{
		SchemaVersion: 1,
		Contract:      "repository_worktree_inventory.v1",
		Artifacts: []workflowArtifactDigestV1{{
			Path: "inventory.json", DeclaredSHA256: strings.Repeat("a", 64), ObservedSHA256: strings.Repeat("a", 64),
		}},
		Inventory: workflowInventoryEvidenceV1{
			RootCount: 2, DirtyRootCount: 1, CleanRootCount: 1,
			WorktreeCount: 2, PerWorktreeRecordCount: 2,
			RequiredPathCheckCount: 2, CompletedPathCheckCount: 2,
			PresentPathCount: 1, AbsentPathCount: 1,
		},
		Roots: roots, Worktrees: worktrees, InventoryRecordsSHA256: recordsSHA256,
	}
	raw, err := json.Marshal(packet)
	if err != nil {
		panic(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		panic(err)
	}
	return result
}

func correctionPacket() map[string]any {
	return map[string]any{
		"failed_criteria":     []string{"output must equal APPROVED"},
		"evidence":            "candidate returned DRAFT",
		"required_correction": "change only the output to APPROVED",
		"candidate":           "candidate-1",
	}
}

func ownerDecisionPacket() map[string]any {
	return map[string]any{
		"question": "Which copy should ship?", "recommendation": "Use A because it is reversible",
		"consequence": "A resumes this worker; B keeps the issue paused", "options": []string{"A", "B"},
	}
}

func TestSquadWorkflowTemplateInstallsPlanBeforeDelegation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	req := withURLParams(newRequest(http.MethodPut, "/api/squads/"+fx.SquadID+"/workflow-template", map[string]any{
		"implementer_id":  fx.ImplementerID,
		"reviewer_id":     fx.ReviewerID,
		"max_corrections": 1,
	}), "id", fx.SquadID, "workspaceId", testWorkspaceID)
	testutil.Call(t, testHandler.PutSquadWorkflowTemplate, req).Want(http.StatusOK)

	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(fx.IssueID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	configured, err := service.EnsureSquadWorkflowPlan(context.Background(), testHandler.Queries, issue)
	if err != nil {
		t.Fatalf("install plan: %v", err)
	}
	if !configured {
		t.Fatal("configured squad was treated as stock routing")
	}
	plan, err := testHandler.Queries.GetIssueWorkflowPlan(context.Background(), db.GetIssueWorkflowPlanParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		t.Fatalf("load installed plan: %v", err)
	}
	if uuidToString(plan.ImplementerID) != fx.ImplementerID || uuidToString(plan.ReviewerID) != fx.ReviewerID {
		t.Fatalf("installed plan does not match template: %+v", plan)
	}

	if _, err := testPool.Exec(context.Background(), `
		UPDATE issue_workflow_plan SET correction_count = 1 WHERE issue_id = $1
	`, fx.IssueID); err != nil {
		t.Fatalf("advance installed plan: %v", err)
	}
	if _, err := service.EnsureSquadWorkflowPlan(context.Background(), testHandler.Queries, issue); err != nil {
		t.Fatalf("re-ensure plan: %v", err)
	}
	plan, err = testHandler.Queries.GetIssueWorkflowPlan(context.Background(), db.GetIssueWorkflowPlanParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil || plan.CorrectionCount != 1 {
		t.Fatalf("existing issue progress was reset: plan=%+v err=%v", plan, err)
	}
}

func TestWorkflowPlanReconcilerRequeuesExistingPlanWithoutCarrier(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	if err := testHandler.Queries.SeedIssueStatusEntries(ctx, parseUUID(testWorkspaceID)); err != nil {
		t.Fatalf("seed issue status catalog: %v", err)
	}
	req := withURLParams(newRequest(http.MethodPut, "/api/squads/"+fx.SquadID+"/workflow-template", map[string]any{
		"implementer_id":  fx.ImplementerID,
		"reviewer_id":     fx.ReviewerID,
		"max_corrections": 1,
	}), "id", fx.SquadID, "workspaceId", testWorkspaceID)
	testutil.Call(t, testHandler.PutSquadWorkflowTemplate, req).Want(http.StatusOK)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	if _, err := testPool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'completed', completed_at = now()
		WHERE id = $1
	`, fx.SourceTaskID); err != nil {
		t.Fatalf("settle old carrier: %v", err)
	}

	reconciler := service.WorkflowPlanReconciler{
		Queries: testHandler.Queries, TaskService: testHandler.TaskService, Bus: testHandler.Bus,
	}
	candidates, err := testHandler.Queries.ListIssuesMissingWorkflowPlan(ctx, 100)
	if err != nil {
		t.Fatalf("list stranded plans: %v", err)
	}
	found := false
	for _, candidate := range candidates {
		if uuidToString(candidate.ID) == fx.IssueID {
			found = true
			break
		}
	}
	if !found {
		var status, category, planState string
		var active bool
		if err := testPool.QueryRow(ctx, `
			SELECT issue.status, status_entry.category, plan.current_state,
			       EXISTS (
			         SELECT 1 FROM agent_task_queue task
			         WHERE task.issue_id = issue.id
			           AND task.status IN ('queued', 'dispatched', 'running', 'waiting_local_directory')
			       )
			FROM issue
			JOIN issue_status status_entry
			  ON status_entry.workspace_id = issue.workspace_id AND status_entry.key = issue.status
			JOIN issue_workflow_plan plan
			  ON plan.issue_id = issue.id AND plan.workspace_id = issue.workspace_id
			WHERE issue.id = $1
		`, fx.IssueID).Scan(&status, &category, &planState, &active); err != nil {
			t.Fatalf("diagnose missing reconciliation candidate: %v", err)
		}
		t.Fatalf("stranded plan was absent from %d reconciliation candidates: status=%s category=%s plan=%s active=%v", len(candidates), status, category, planState, active)
	}
	if err := reconciler.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile stranded plan: %v", err)
	}
	var tasks int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM agent_task_queue
		WHERE issue_id = $1
		  AND agent_id = $2
		  AND is_leader_task
		  AND status IN ('queued', 'dispatched', 'running', 'waiting_local_directory')
	`, fx.IssueID, fx.LeaderID).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if tasks != 1 {
		t.Fatalf("active recovery leader tasks = %d, want 1", tasks)
	}
	if err := reconciler.ReconcileOnce(ctx); err != nil {
		t.Fatalf("repeat reconciliation: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM agent_task_queue
		WHERE issue_id = $1
		  AND agent_id = $2
		  AND is_leader_task
		  AND status IN ('queued', 'dispatched', 'running', 'waiting_local_directory')
	`, fx.IssueID, fx.LeaderID).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if tasks != 1 {
		t.Fatalf("active recovery leader tasks after replay = %d, want 1", tasks)
	}
}

func TestSquadWorkflowTemplateFailsClosedWhenReviewerLeavesSquad(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	req := withURLParams(newRequest(http.MethodPut, "/api/squads/"+fx.SquadID+"/workflow-template", map[string]any{
		"implementer_id": fx.ImplementerID,
		"reviewer_id":    fx.ReviewerID,
	}), "id", fx.SquadID, "workspaceId", testWorkspaceID)
	testutil.Call(t, testHandler.PutSquadWorkflowTemplate, req).Want(http.StatusOK)
	if _, err := testPool.Exec(context.Background(), `
		DELETE FROM squad_member
		WHERE squad_id = $1 AND member_type = 'agent' AND member_id = $2
	`, fx.SquadID, fx.ReviewerID); err != nil {
		t.Fatalf("remove reviewer: %v", err)
	}
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(fx.IssueID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	configured, err := service.EnsureSquadWorkflowPlan(context.Background(), testHandler.Queries, issue)
	if !configured || err == nil {
		t.Fatalf("stale configured route did not fail closed: configured=%v err=%v", configured, err)
	}
}

func TestWorkflowControlImplementationReadyQueuesReviewerExactlyOnce(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	first := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusCreated)
	var firstBody issueWorkflowEventResponse
	if err := json.Unmarshal([]byte(first.Text()), &firstBody); err != nil {
		t.Fatalf("decode first event: %v", err)
	}
	if firstBody.Decision.Next != workflowstate.TechnicalReview || firstBody.Decision.EnqueueAgentID != fx.ReviewerID || firstBody.Dispatch != "queued" {
		t.Fatalf("unexpected first decision: %+v", firstBody)
	}

	second := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusCreated)
	var secondBody issueWorkflowEventResponse
	if err := json.Unmarshal([]byte(second.Text()), &secondBody); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	if !secondBody.Replay || secondBody.EventID != firstBody.EventID {
		t.Fatalf("event replay was not idempotent: first=%+v second=%+v", firstBody, secondBody)
	}

	var count int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE trigger_evidence_kind = 'workflow_event'
		  AND trigger_evidence_ref_id = $1
	`, firstBody.EventID).Scan(&count); err != nil {
		t.Fatalf("count workflow tasks: %v", err)
	}
	if count != 1 {
		t.Fatalf("workflow event created %d reviewer tasks, want exactly 1", count)
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatalf("load projected issue status: %v", err)
	}
	if status != "in_review" {
		t.Fatalf("implementation-ready board status = %q, want in_review", status)
	}

	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(fx.IssueID))
	if err != nil {
		t.Fatalf("load issue for trigger check: %v", err)
	}
	triggers, _ := testHandler.computeCommentAgentTriggers(context.Background(), issue, "implementation evidence", nil, "agent", fx.ImplementerID, commentTriggerComputeOptions{
		SourceTaskID: parseUUID(fx.SourceTaskID),
	})
	if triggersContainIssueAssigneeSquadLeader(triggers) {
		t.Fatal("structured implementation-ready event still woke squad leader as a relay")
	}
}

func TestWorkflowControlReviewPassedAtomicallyProjectsCompleteOwnerPicture(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx, reviewerTaskID := prepareNonLiveReviewFixture(t)

	result := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewPassed, fx.ReviewerID, reviewerTaskID, nil),
	).Want(http.StatusCreated)
	var response issueWorkflowEventResponse
	result.JSON(&response)
	if response.Decision.Next != workflowstate.Done || response.Plan.CurrentState != string(workflowstate.Done) {
		t.Fatalf("review pass response = %#v", response)
	}

	picture, err := testHandler.Queries.GetIssueCurrentPicture(context.Background(), db.GetIssueCurrentPictureParams{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(fx.IssueID),
	})
	if err != nil {
		t.Fatalf("load legacy success picture: %v", err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	if summary.Stage != workflowprojection.OwnerStageComplete || summary.NextAction != nil || summary.LatestMeaningfulChange == nil || summary.LatestMeaningfulChange.ResultCode != "succeeded" {
		t.Fatalf("legacy success owner summary = %#v", summary)
	}
	var detail issueCurrentPictureResponse
	testutil.Call(t, testHandler.GetIssueCurrentPicture,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+fx.IssueID+"/current-picture", nil), "id", fx.IssueID)).
		Want(http.StatusOK).JSON(&detail)
	if detail.OwnerSummary.Stage != workflowprojection.OwnerStageComplete || detail.ScopeRevision == nil || *detail.ScopeRevision != 1 {
		t.Fatalf("legacy success detail = %#v", detail)
	}
	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Fatalf("legacy success issue status = %q, want done", status)
	}
	assertLegacySuccessProjectionCounts(t, fx.IssueID, 1, 1, 1)

	// The source task remains running in this controller-level fixture, so an
	// exact replay reaches the idempotency branch. It must not duplicate the
	// picture, event, or owner Inbox item.
	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewPassed, fx.ReviewerID, reviewerTaskID, nil),
	).Want(http.StatusCreated)
	assertLegacySuccessProjectionCounts(t, fx.IssueID, 1, 1, 1)
}

func TestWorkflowControlReviewPassedRollsBackWhenOwnerInboxAdmissionFails(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx, reviewerTaskID := prepareNonLiveReviewFixture(t)
	originalAdmission := testHandler.TaskService.AdmitLegacyWorkflowProjection
	testHandler.TaskService.AdmitLegacyWorkflowProjection = func(context.Context, *db.Queries, pgtype.UUID, pgtype.UUID, int64) (*db.InboxItem, error) {
		return nil, errors.New("injected legacy success Inbox admission failure")
	}
	t.Cleanup(func() { testHandler.TaskService.AdmitLegacyWorkflowProjection = originalAdmission })

	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewPassed, fx.ReviewerID, reviewerTaskID, nil),
	).Want(http.StatusConflict)

	plan, err := testHandler.Queries.GetIssueWorkflowPlan(context.Background(), db.GetIssueWorkflowPlanParams{
		IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if plan.CurrentState != string(workflowstate.TechnicalReview) || status != "in_review" {
		t.Fatalf("partial success state committed plan/status = %q/%q", plan.CurrentState, status)
	}
	assertLegacySuccessProjectionCounts(t, fx.IssueID, 0, 0, 0)
}

func TestWorkflowControlReviewPassedDoesNotReplaceExistingCurrentPicture(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx, reviewerTaskID := prepareNonLiveReviewFixture(t)
	params, err := workflowprojection.LegacyTechnicalStopParams(
		parseUUID(testWorkspaceID), parseUUID(fx.IssueID), parseUUID(reviewerTaskID),
		"existing authoritative stop", time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := testHandler.Queries.UpsertIssueCurrentPicture(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}

	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewPassed, fx.ReviewerID, reviewerTaskID, nil),
	).Want(http.StatusConflict)
	retained, err := testHandler.Queries.GetIssueCurrentPicture(context.Background(), db.GetIssueCurrentPictureParams{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(fx.IssueID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if retained.Revision != existing.Revision || !bytes.Equal(retained.Document, existing.Document) {
		t.Fatal("legacy success replaced existing Current Picture truth")
	}
	assertLegacySuccessProjectionCounts(t, fx.IssueID, 0, 1, 0)
}

func prepareNonLiveReviewFixture(t *testing.T) (workflowControlFixture, string) {
	t.Helper()
	fx := newWorkflowControlFixture(t)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM inbox_item WHERE issue_id = $1`, fx.IssueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_current_picture WHERE issue_id = $1`, fx.IssueID)
	})
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	ready := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequest(fx, workflowstate.ImplementationReady),
	).Want(http.StatusCreated)
	var readyResponse issueWorkflowEventResponse
	ready.JSON(&readyResponse)
	var reviewerTaskID string
	if err := testPool.QueryRow(context.Background(), `
		UPDATE agent_task_queue SET status = 'running', started_at = now()
		WHERE trigger_evidence_kind = 'workflow_event' AND trigger_evidence_ref_id = $1
		RETURNING id
	`, readyResponse.EventID).Scan(&reviewerTaskID); err != nil {
		t.Fatalf("start reviewer task: %v", err)
	}
	return fx, reviewerTaskID
}

func assertLegacySuccessProjectionCounts(t *testing.T, issueID string, events, pictures, completions int) {
	t.Helper()
	var gotEvents, gotPictures, gotCompletions int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_workflow_event WHERE issue_id = $1 AND event_type = 'review_passed'`, issueID).Scan(&gotEvents); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_current_picture WHERE issue_id = $1`, issueID).Scan(&gotPictures); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'task_completed' AND details->>'owner_category' = 'completion'`, issueID).Scan(&gotCompletions); err != nil {
		t.Fatal(err)
	}
	if gotEvents != events || gotPictures != pictures || gotCompletions != completions {
		t.Fatalf("legacy success event/picture/completion counts = %d/%d/%d, want %d/%d/%d", gotEvents, gotPictures, gotCompletions, events, pictures, completions)
	}
}

func TestWorkflowControlRejectsLegacyEventFromNormalizedWorkflowTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue SET workflow_action_id = gen_random_uuid() WHERE id = $1
	`, fx.SourceTaskID); err != nil {
		t.Fatalf("bind normalized workflow action: %v", err)
	}

	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusConflict)

	var events int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM issue_workflow_event WHERE issue_id = $1
	`, fx.IssueID).Scan(&events); err != nil {
		t.Fatalf("count legacy workflow events: %v", err)
	}
	if events != 0 {
		t.Fatalf("normalized task created %d legacy workflow events, want zero", events)
	}
}

func TestWorkflowControlRejectsSelfReviewPlan(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	req := withURLParam(newRequest(http.MethodPut, "/api/issues/"+fx.IssueID+"/workflow-plan", map[string]any{
		"squad_id": fx.SquadID, "implementer_id": fx.ImplementerID, "reviewer_id": fx.ImplementerID,
	}), "id", fx.IssueID)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, req).Want(http.StatusBadRequest)
}

func TestWorkflowControlRejectsEventFromCompletedTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue SET status = 'completed', completed_at = now()
		WHERE id = $1
	`, fx.SourceTaskID); err != nil {
		t.Fatalf("complete source task: %v", err)
	}
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusConflict)
}

func TestWorkflowControlOwnerDecisionAlertsOnceAndResumesExactWorker(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	body := map[string]any{"event": workflowstate.OwnerDecisionRequired, "reason": "one product choice is required", "owner_decision": ownerDecisionPacket()}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	req.Header.Set("X-Agent-ID", fx.ImplementerID)
	req.Header.Set("X-Task-ID", fx.SourceTaskID)
	result := testutil.Call(t, testHandler.PostIssueWorkflowEvent, req).Want(http.StatusCreated)
	var waiting issueWorkflowEventResponse
	result.JSON(&waiting)
	if waiting.Decision.Next != workflowstate.NeedsOwner || !waiting.Decision.NotifyOwner {
		t.Fatalf("owner request decision = %+v", waiting.Decision)
	}

	var alertCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_owner_decision'`, fx.IssueID).Scan(&alertCount); err != nil {
		t.Fatal(err)
	}
	if alertCount != 1 {
		t.Fatalf("owner alerts = %d, want 1", alertCount)
	}

	answerReq := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-owner-decision", map[string]any{"request_event_id": waiting.EventID, "answer": "A"}), "id", fx.IssueID)
	answered := testutil.Call(t, testHandler.PostIssueWorkflowOwnerDecision, answerReq).Want(http.StatusCreated)
	var resumed issueWorkflowEventResponse
	answered.JSON(&resumed)
	if resumed.Decision.Next != workflowstate.Working || resumed.Decision.EnqueueAgentID != fx.ImplementerID || resumed.Dispatch != "queued" {
		t.Fatalf("owner answer did not resume exact worker: %+v", resumed)
	}
	if resumed.Plan.SuspendedState.Valid || resumed.Plan.SuspendedAgentID.Valid {
		t.Fatalf("suspended fields not cleared: %+v", resumed.Plan)
	}

	var resumedCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='workflow_event' AND trigger_evidence_ref_id=$1 AND agent_id=$2`, resumed.EventID, fx.ImplementerID).Scan(&resumedCount); err != nil {
		t.Fatal(err)
	}
	if resumedCount != 1 {
		t.Fatalf("resumed worker tasks = %d, want 1", resumedCount)
	}
	replayReq := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-owner-decision", map[string]any{"request_event_id": waiting.EventID, "answer": "A"}), "id", fx.IssueID)
	testutil.Call(t, testHandler.PostIssueWorkflowOwnerDecision, replayReq).Want(http.StatusConflict)
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_owner_decision'`, fx.IssueID).Scan(&alertCount); err != nil {
		t.Fatal(err)
	}
	if alertCount != 1 {
		t.Fatalf("owner alerts after replay = %d, want 1", alertCount)
	}
}

// Characterization only: free-text answers do not encode an outcome today.
// Even an explicit rejection is treated as OwnerDecisionRecorded and follows
// the same resume path as approval. Workflow contract v2 must replace this
// behavior with a typed response; this test must change when that lands.
func TestWorkflowControlCharacterizationNonemptyRejectionStillResumes(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	body := map[string]any{
		"event": workflowstate.OwnerDecisionRequired, "reason": "one product choice is required",
		"owner_decision": ownerDecisionPacket(),
	}
	request := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	request.Header.Set("X-Agent-ID", fx.ImplementerID)
	request.Header.Set("X-Task-ID", fx.SourceTaskID)
	var waiting issueWorkflowEventResponse
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, request).Want(http.StatusCreated).JSON(&waiting)

	answer := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-owner-decision", map[string]any{
		"request_event_id": waiting.EventID,
		"answer":           "Reject",
	}), "id", fx.IssueID)
	var rejected issueWorkflowEventResponse
	testutil.Call(t, testHandler.PostIssueWorkflowOwnerDecision, answer).Want(http.StatusCreated).JSON(&rejected)
	if rejected.Decision.Next != workflowstate.Working || rejected.Decision.EnqueueAgentID != fx.ImplementerID || rejected.Dispatch != "queued" {
		t.Fatalf("characterization changed: free-text rejection no longer followed the approval resume path: %+v", rejected)
	}
}

// Characterization only: OwnerDecisionRequired currently validates that the
// caller owns a running task on the issue, but it does not require that task's
// agent to hold any role in the workflow plan. Contract v2 must reject this.
func TestWorkflowControlCharacterizationUnrelatedRunningTaskCanRequestOwnerDecision(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	outsiderID := createHandlerTestAgent(t, "Workflow Outsider", nil)
	outsiderTaskID := dbfx.Task(t, outsiderID, testutil.Cols{
		"runtime_id":          handlerTestRuntimeID(t),
		"status":              "running",
		"issue_id":            fx.IssueID,
		"started_at":          testutil.Raw("now()"),
		"squad_id":            fx.SquadID,
		"originator_user_id":  testUserID,
		"accountable_user_id": testUserID,
	})
	body := map[string]any{
		"event": workflowstate.OwnerDecisionRequired, "reason": "unrelated task requested owner authority",
		"owner_decision": ownerDecisionPacket(),
	}
	request := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	request.Header.Set("X-Agent-ID", outsiderID)
	request.Header.Set("X-Task-ID", outsiderTaskID)
	var got issueWorkflowEventResponse
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, request).Want(http.StatusCreated).JSON(&got)
	if got.Decision.Next != workflowstate.NeedsOwner || !got.Decision.NotifyOwner || uuidToString(got.Plan.SuspendedAgentID) != outsiderID {
		t.Fatalf("characterization changed: unrelated running task was not accepted as the suspended owner-decision source: %+v", got)
	}
}

// Characterization only: duplicate concurrent submissions have one durable
// logical effect, but the losing request can be either a replay or a conflict
// depending on whether its initial lookup races the winner's commit. Contract
// v2 must return one stable idempotent receipt instead of this response race.
func TestWorkflowControlCharacterizationConcurrentDuplicateHasOneLogicalEffect(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	type result struct {
		code int
		body string
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for range 2 {
		request := workflowEventRequest(fx, workflowstate.ImplementationReady)
		go func() {
			<-start
			recorder := httptest.NewRecorder()
			testHandler.PostIssueWorkflowEvent(recorder, request)
			results <- result{code: recorder.Code, body: recorder.Body.String()}
		}()
	}
	close(start)

	created := 0
	replayed := 0
	conflicted := 0
	eventID := ""
	for range 2 {
		got := <-results
		switch got.code {
		case http.StatusCreated:
			var response issueWorkflowEventResponse
			if err := json.Unmarshal([]byte(got.body), &response); err != nil {
				t.Fatalf("decode concurrent response: %v: %s", err, got.body)
			}
			if response.Replay {
				replayed++
			} else {
				created++
			}
			if eventID != "" && response.EventID != eventID {
				t.Fatalf("concurrent duplicate returned two event ids: %s and %s", eventID, response.EventID)
			}
			eventID = response.EventID
		case http.StatusConflict:
			conflicted++
		default:
			t.Fatalf("concurrent duplicate returned unexpected status %d: %s", got.code, got.body)
		}
	}
	if created != 1 || replayed+conflicted != 1 {
		t.Fatalf("characterization changed: created=%d replayed=%d conflicted=%d", created, replayed, conflicted)
	}

	var eventCount, taskCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_workflow_event WHERE source_task_id=$1 AND event_type=$2`, fx.SourceTaskID, workflowstate.ImplementationReady).Scan(&eventCount); err != nil {
		t.Fatalf("count concurrent workflow events: %v", err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='workflow_event' AND trigger_evidence_ref_id=$1`, eventID).Scan(&taskCount); err != nil {
		t.Fatalf("count concurrent workflow tasks: %v", err)
	}
	if eventCount != 1 || taskCount != 1 {
		t.Fatalf("concurrent duplicate produced event/task counts %d/%d, want 1/1", eventCount, taskCount)
	}
}

// An agent's role identity alone must not promote an ordinary task into a
// legacy workflow. Such a task cannot recover a failed enqueue at completion.
func TestWorkflowControlRejectsUnboundLegacySourceBeforeAdvancing(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	for _, squad := range []string{"NULL", "'00000000-0000-4000-8000-000000000001'::uuid"} {
		t.Run(squad, func(t *testing.T) {
			fx := newWorkflowControlFixture(t)
			testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
			dbfx.Exec(t, `UPDATE agent_task_queue SET squad_id = `+squad+` WHERE id = $1`, fx.SourceTaskID)
			refused := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusConflict)
			if !strings.Contains(refused.Text(), "ordinary tasks cannot advance a legacy squad workflow") {
				t.Fatalf("missing actionable ordinary route refusal: %s", refused.Text())
			}
			assertWorkflowAdmissionUnchanged(t, fx)
		})
	}
}

func TestWorkflowControlRejectsDefaultLegacySuccessorBeforeAdvancing(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	for _, field := range []string{"model", "thinking_level"} {
		t.Run(field, func(t *testing.T) {
			fx := newWorkflowControlFixture(t)
			testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
			dbfx.Exec(t, `UPDATE agent SET `+field+` = NULL WHERE id = $1`, fx.ReviewerID)
			refused := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusConflict)
			if !strings.Contains(refused.Text(), "legacy workflow successor requires an explicit") {
				t.Fatalf("missing actionable configuration refusal: %s", refused.Text())
			}
			assertWorkflowAdmissionUnchanged(t, fx)
		})
	}
}

func assertWorkflowAdmissionUnchanged(t *testing.T, fx workflowControlFixture) {
	t.Helper()
	var state string
	var revision int64
	var events, successors int
	if err := testPool.QueryRow(context.Background(), `SELECT current_state, revision FROM issue_workflow_plan WHERE issue_id=$1`, fx.IssueID).Scan(&state, &revision); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_workflow_event WHERE issue_id=$1`, fx.IssueID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id<>$2`, fx.IssueID, fx.SourceTaskID).Scan(&successors); err != nil {
		t.Fatal(err)
	}
	if state != string(workflowstate.Working) || revision != 1 || events != 0 || successors != 0 {
		t.Fatalf("refused handoff changed state/revision/events/successors: %s/%d/%d/%d", state, revision, events, successors)
	}
}

// Characterization only: the workflow event and state commit before dispatch.
// On dispatch failure, "pending_retry" exists only in this HTTP response; no
// durable task or retry record is admitted for an automatic worker to claim.
func TestWorkflowControlCharacterizationPostCommitDispatchFailureHasNoDurableRetry(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	if _, err := testPool.Exec(context.Background(), `UPDATE agent SET archived_at=now() WHERE id=$1`, fx.ReviewerID); err != nil {
		t.Fatalf("archive dispatch target: %v", err)
	}

	var pending issueWorkflowEventResponse
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusAccepted).JSON(&pending)
	if pending.Dispatch != "pending_retry" || pending.Decision.Next != workflowstate.TechnicalReview || pending.Replay {
		t.Fatalf("characterization changed: post-commit dispatch failure response = %+v", pending)
	}

	var eventCount, taskCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_workflow_event WHERE id=$1`, pending.EventID).Scan(&eventCount); err != nil {
		t.Fatalf("count committed workflow event: %v", err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='workflow_event' AND trigger_evidence_ref_id=$1`, pending.EventID).Scan(&taskCount); err != nil {
		t.Fatalf("count failed-dispatch workflow tasks: %v", err)
	}
	if eventCount != 1 || taskCount != 0 {
		t.Fatalf("post-commit dispatch failure left event/task counts %d/%d, want 1/0", eventCount, taskCount)
	}

	var replay issueWorkflowEventResponse
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusAccepted).JSON(&replay)
	if !replay.Replay || replay.EventID != pending.EventID || replay.Dispatch != "pending_retry" {
		t.Fatalf("manual replay did not expose the same undelivered event: first=%+v replay=%+v", pending, replay)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='workflow_event' AND trigger_evidence_ref_id=$1`, pending.EventID).Scan(&taskCount); err != nil {
		t.Fatalf("recount failed-dispatch workflow tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("manual replay unexpectedly created %d durable retry tasks, want 0", taskCount)
	}
}

func TestWorkflowControlReviewFailureRequiresStructuredCorrection(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewFailed, fx.ReviewerID, fx.SourceTaskID, nil),
	).Want(http.StatusBadRequest)
}

// legacyWorkflowControlCorrectionLoopIsBoundedAndDoesNotRelayThroughMika is
// retained as executable historical test logic while the investigation-first
// contract replaces direct correction dispatch. It is deliberately not a Test.
func legacyWorkflowControlCorrectionLoopIsBoundedAndDoesNotRelayThroughMika(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	ready := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequest(fx, workflowstate.ImplementationReady),
	).Want(http.StatusCreated)
	var readyBody issueWorkflowEventResponse
	ready.JSON(&readyBody)

	var firstReviewerTask string
	if err := testPool.QueryRow(context.Background(), `
		UPDATE agent_task_queue SET status = 'running', started_at = now()
		WHERE trigger_evidence_kind = 'workflow_event' AND trigger_evidence_ref_id = $1
		RETURNING id
	`, readyBody.EventID).Scan(&firstReviewerTask); err != nil {
		t.Fatalf("start first reviewer: %v", err)
	}
	firstFailure := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewFailed, fx.ReviewerID, firstReviewerTask, correctionPacket()),
	).Want(http.StatusCreated)
	var firstFailureBody issueWorkflowEventResponse
	firstFailure.JSON(&firstFailureBody)
	if firstFailureBody.Decision.Next != workflowstate.Correction || firstFailureBody.Decision.EnqueueAgentID != fx.ImplementerID || firstFailureBody.Decision.WakeLeader {
		t.Fatalf("first failure did not route directly to correction: %+v", firstFailureBody)
	}
	if firstFailureBody.Plan.CorrectionCount != 1 {
		t.Fatalf("correction count = %d, want 1", firstFailureBody.Plan.CorrectionCount)
	}

	var stored workflowCorrectionPacket
	var payload []byte
	if err := testPool.QueryRow(context.Background(), `SELECT payload FROM issue_workflow_event WHERE id = $1`, firstFailureBody.EventID).Scan(&payload); err != nil {
		t.Fatalf("load correction payload: %v", err)
	}
	if err := json.Unmarshal(payload, &stored); err != nil || stored.RequiredCorrection != "change only the output to APPROVED" {
		t.Fatalf("stored correction packet = %+v, err=%v", stored, err)
	}

	var correctionTask string
	if err := testPool.QueryRow(context.Background(), `
		UPDATE agent_task_queue SET status = 'running', started_at = now()
		WHERE trigger_evidence_kind = 'workflow_event' AND trigger_evidence_ref_id = $1
		RETURNING id
	`, firstFailureBody.EventID).Scan(&correctionTask); err != nil {
		t.Fatalf("start correction worker: %v", err)
	}
	corrected := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ImplementationReady, fx.ImplementerID, correctionTask, nil),
	).Want(http.StatusCreated)
	var correctedBody issueWorkflowEventResponse
	corrected.JSON(&correctedBody)

	var secondReviewerTask string
	if err := testPool.QueryRow(context.Background(), `
		UPDATE agent_task_queue SET status = 'running', started_at = now()
		WHERE trigger_evidence_kind = 'workflow_event' AND trigger_evidence_ref_id = $1
		RETURNING id
	`, correctedBody.EventID).Scan(&secondReviewerTask); err != nil {
		t.Fatalf("start second reviewer: %v", err)
	}
	secondFailure := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewFailed, fx.ReviewerID, secondReviewerTask, correctionPacket()),
	).Want(http.StatusCreated)
	var secondFailureBody issueWorkflowEventResponse
	secondFailure.JSON(&secondFailureBody)
	if secondFailureBody.Decision.Next != workflowstate.Blocked || !secondFailureBody.Decision.WakeLeader || secondFailureBody.Decision.EnqueueAgentID != fx.LeaderID {
		t.Fatalf("second failure did not stop and escalate: %+v", secondFailureBody)
	}
	if secondFailureBody.Plan.CorrectionCount != 1 {
		t.Fatalf("bounded correction count = %d, want 1", secondFailureBody.Plan.CorrectionCount)
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatalf("load final issue status: %v", err)
	}
	if status != "blocked" {
		t.Fatalf("issue status = %q, want blocked", status)
	}
	var leaderTaskCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE trigger_evidence_kind = 'workflow_event'
		  AND trigger_evidence_ref_id = $1
		  AND agent_id = $2
		  AND is_leader_task = true
	`, secondFailureBody.EventID, fx.LeaderID).Scan(&leaderTaskCount); err != nil {
		t.Fatalf("count leader escalation task: %v", err)
	}
	if leaderTaskCount != 1 {
		t.Fatalf("leader escalation tasks = %d, want 1", leaderTaskCount)
	}
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(fx.IssueID))
	if err != nil {
		t.Fatalf("load issue for escalation trigger check: %v", err)
	}
	triggers, _ := testHandler.computeCommentAgentTriggers(context.Background(), issue, "second review failure", nil, "agent", fx.ReviewerID, commentTriggerComputeOptions{
		SourceTaskID: parseUUID(secondReviewerTask),
	})
	if triggersContainIssueAssigneeSquadLeader(triggers) {
		t.Fatal("controller-dispatched leader escalation still created a second comment fallback")
	}
}

func TestWorkflowControlReviewFailureStartsInvestigationBeforeCorrection(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	ready := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequest(fx, workflowstate.ImplementationReady),
	).Want(http.StatusCreated)
	var readyBody issueWorkflowEventResponse
	ready.JSON(&readyBody)

	var reviewerTask string
	if err := testPool.QueryRow(context.Background(), `
		UPDATE agent_task_queue SET status = 'running', started_at = now()
		WHERE trigger_evidence_kind = 'workflow_event' AND trigger_evidence_ref_id = $1
		RETURNING id
	`, readyBody.EventID).Scan(&reviewerTask); err != nil {
		t.Fatalf("start reviewer: %v", err)
	}

	failed := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewFailed, fx.ReviewerID, reviewerTask, correctionPacket()),
	).Want(http.StatusCreated)
	var failedBody issueWorkflowEventResponse
	failed.JSON(&failedBody)
	if failedBody.Decision.Next != workflowstate.Correction || failedBody.Dispatch != "investigation_queued" {
		t.Fatalf("review failure did not pause at investigation: %+v", failedBody)
	}

	var correctionTasks, investigations int
	if err := testPool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='workflow_event' AND trigger_evidence_ref_id=$1),
			(SELECT count(*) FROM ai_failure_investigation WHERE trigger_kind='qa_rejection' AND trigger_id=$1)
	`, failedBody.EventID).Scan(&correctionTasks, &investigations); err != nil {
		t.Fatalf("read investigation boundary: %v", err)
	}
	if correctionTasks != 0 || investigations != 1 {
		t.Fatalf("review rejection created %d correction tasks and %d investigations, want 0/1", correctionTasks, investigations)
	}

	replay := testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequestFor(fx, workflowstate.ReviewFailed, fx.ReviewerID, reviewerTask, correctionPacket()),
	).Want(http.StatusCreated)
	var replayBody issueWorkflowEventResponse
	replay.JSON(&replayBody)
	if !replayBody.Replay || replayBody.EventID != failedBody.EventID {
		t.Fatalf("review rejection replay drifted: first=%+v replay=%+v", failedBody, replayBody)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM ai_failure_investigation WHERE trigger_kind='qa_rejection' AND trigger_id=$1`, failedBody.EventID).Scan(&investigations); err != nil || investigations != 1 {
		t.Fatalf("replayed rejection investigations = %d, err=%v, want 1", investigations, err)
	}
}

// seedAcceptedEventWithoutDispatch records an accepted implementation_ready
// event whose successor enqueue never happened, which is the state left behind
// when the enqueue fails after the event commits.
func seedAcceptedEventWithoutDispatch(t *testing.T, fx workflowControlFixture) string {
	t.Helper()
	var eventID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue_workflow_event (
			id, workspace_id, issue_id, source_task_id, actor_id,
			event_type, reason, decision
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4,
			'implementation_ready', 'dispatch deliberately absent',
			jsonb_build_object('next', 'technical_review', 'enqueue_agent_id', $5::text)
		)
		RETURNING id
	`, testWorkspaceID, fx.IssueID, fx.SourceTaskID, fx.ImplementerID, fx.ReviewerID).Scan(&eventID); err != nil {
		t.Fatalf("seed event without dispatched task: %v", err)
	}
	return eventID
}

func countWorkflowTasks(t *testing.T, kind, refID string) int {
	t.Helper()
	var count int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE trigger_evidence_kind = $1 AND trigger_evidence_ref_id = $2
	`, kind, refID).Scan(&count); err != nil {
		t.Fatalf("count %s tasks: %v", kind, err)
	}
	return count
}

// The accepted rule (ffec7d703): an accepted advancing event is authoritative
// even when its successor enqueue failed after commit. The running task
// recovers the hand-off by resending the same event, which replays it and
// enqueues the successor idempotently; completing the task never wakes the
// leader for it. This replaced the earlier leader fallback that this test used
// to expect.
func TestWorkflowControlCommentBeforeMissingDispatchDoesNotRaceLeaderFallback(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	eventID := seedAcceptedEventWithoutDispatch(t, fx)
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(fx.IssueID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	triggers, _ := testHandler.computeCommentAgentTriggers(context.Background(), issue, "dispatch failed", nil, "agent", fx.ImplementerID, commentTriggerComputeOptions{
		SourceTaskID: parseUUID(fx.SourceTaskID),
	})
	if triggersContainIssueAssigneeSquadLeader(triggers) {
		t.Fatal("workflow-role comment raced the terminal recovery boundary")
	}

	// Resending the same event while the task is still running replays it and
	// recovers the missing hand-off.
	var replay issueWorkflowEventResponse
	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequest(fx, workflowstate.ImplementationReady),
	).Want(http.StatusCreated).JSON(&replay)
	if !replay.Replay || replay.EventID != eventID || replay.Dispatch != "queued" {
		t.Fatalf("resend = replay %v event %s dispatch %q, want replay of %s queued", replay.Replay, replay.EventID, replay.Dispatch, eventID)
	}
	if got := countWorkflowTasks(t, "workflow_event", eventID); got != 1 {
		t.Fatalf("reviewer tasks after resend = %d, want exactly 1", got)
	}

	if _, err := testHandler.TaskService.CompleteTask(context.Background(), parseUUID(fx.SourceTaskID), []byte(`{"output":"implementation finished"}`), "", "", "", false, "", ""); err != nil {
		t.Fatalf("complete workflow task: %v", err)
	}
	if got := countWorkflowTasks(t, "workflow_missing_event", fx.SourceTaskID); got != 0 {
		t.Fatalf("leader recovery tasks after completion = %d, want 0 under the accepted rule", got)
	}
}

// A hand-off whose successor enqueue failed and whose task then completed
// before anyone resent the event cannot be resent (only the running task may
// post workflow events). Completion repairs it: exactly one reviewer task is
// created for the accepted event, no leader is woken, and a second pass finds
// nothing left to dispatch.
func TestWorkflowControlMissingDispatchIsRecoveredAtCompletion(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	eventID := seedAcceptedEventWithoutDispatch(t, fx)

	if _, err := testHandler.TaskService.CompleteTask(context.Background(), parseUUID(fx.SourceTaskID), []byte(`{"output":"implementation finished"}`), "", "", "", false, "", ""); err != nil {
		t.Fatalf("complete workflow task: %v", err)
	}
	if got := countWorkflowTasks(t, "workflow_event", eventID); got != 1 {
		t.Fatalf("reviewer tasks after completion = %d, want exactly 1 recovered hand-off", got)
	}
	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT agent_id FROM agent_task_queue
		WHERE trigger_evidence_kind = 'workflow_event' AND trigger_evidence_ref_id = $1
	`, eventID).Scan(&agentID); err != nil {
		t.Fatalf("load recovered task: %v", err)
	}
	if agentID != fx.ReviewerID {
		t.Fatalf("recovered task agent = %s, want reviewer %s", agentID, fx.ReviewerID)
	}
	if got := countWorkflowTasks(t, "workflow_missing_event", fx.SourceTaskID); got != 0 {
		t.Fatalf("leader recovery tasks = %d, want 0", got)
	}
	remaining, err := testHandler.Queries.ListUndispatchedIssueWorkflowEventsForTask(context.Background(), parseUUID(fx.SourceTaskID))
	if err != nil {
		t.Fatalf("list undispatched events: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("undispatched events after recovery = %d, want 0 so a repeat pass is a no-op", len(remaining))
	}
	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status == "blocked" {
		t.Fatal("recovered hand-off still stopped the workflow")
	}
	// A late resend from the finished task is still refused and cannot add a
	// second reviewer task.
	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequest(fx, workflowstate.ImplementationReady),
	).Want(http.StatusConflict)
	if got := countWorkflowTasks(t, "workflow_event", eventID); got != 1 {
		t.Fatalf("reviewer tasks after late resend = %d, want 1", got)
	}
}

// When the missing hand-off cannot be recovered (here the reviewer was archived
// after the event was accepted), the workflow must not stall silently: the
// issue stops with a technical stop on its Current Picture and an owner Inbox
// incident.
func TestWorkflowControlUnrecoverableMissingDispatchRaisesIncident(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	eventID := seedAcceptedEventWithoutDispatch(t, fx)
	if _, err := testPool.Exec(ctx, `UPDATE agent SET archived_at = now() WHERE id = $1`, fx.ReviewerID); err != nil {
		t.Fatalf("archive reviewer: %v", err)
	}

	if _, err := testHandler.TaskService.CompleteTask(ctx, parseUUID(fx.SourceTaskID), []byte(`{"output":"implementation finished"}`), "", "", "", false, "", ""); err != nil {
		t.Fatalf("complete workflow task: %v", err)
	}
	if got := countWorkflowTasks(t, "workflow_event", eventID); got != 0 {
		t.Fatalf("reviewer tasks = %d, want 0 for an archived reviewer", got)
	}
	if got := countWorkflowTasks(t, "workflow_missing_event", fx.SourceTaskID); got != 0 {
		t.Fatalf("leader recovery tasks = %d, want 0", got)
	}
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "blocked" {
		t.Fatalf("issue status = %q, want blocked", status)
	}
	picture, err := testHandler.Queries.GetIssueCurrentPicture(ctx, db.GetIssueCurrentPictureParams{WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(fx.IssueID)})
	if err != nil {
		t.Fatalf("load current picture: %v", err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	if summary.Stage != workflowprojection.OwnerStageTechnicalStop {
		t.Fatalf("current picture stage = %q, want technical stop", summary.Stage)
	}
	if summary.NextAction == nil || summary.NextAction.Kind != "inspect_incident" || summary.NextAction.Automatic {
		t.Fatalf("next action = %#v, want manual inspect_incident", summary.NextAction)
	}
	var incidents int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_incident' AND NOT archived`, fx.IssueID).Scan(&incidents); err != nil {
		t.Fatal(err)
	}
	if incidents != 1 {
		t.Fatalf("workflow incident inbox items = %d, want 1", incidents)
	}
}

func TestWorkflowControlSuccessfulDispatchDoesNotWakeRecoveryLeader(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent,
		workflowEventRequest(fx, workflowstate.ImplementationReady),
	).Want(http.StatusCreated)

	if _, err := testHandler.TaskService.CompleteTask(context.Background(), parseUUID(fx.SourceTaskID), []byte(`{"output":"implementation finished"}`), "", "", "", false, "", ""); err != nil {
		t.Fatalf("complete workflow task: %v", err)
	}
	var recoveryCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE trigger_evidence_kind = 'workflow_missing_event'
		  AND trigger_evidence_ref_id = $1
	`, fx.SourceTaskID).Scan(&recoveryCount); err != nil {
		t.Fatalf("count terminal recovery tasks: %v", err)
	}
	if recoveryCount != 0 {
		t.Fatalf("successful workflow dispatch created %d recovery tasks, want 0", recoveryCount)
	}
	// Completion recovery leaves a successful hand-off alone: still exactly one
	// reviewer task and no incident.
	var reviewerTasks, incidents int
	if err := testPool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2),
			(SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_incident')
	`, fx.IssueID, fx.ReviewerID).Scan(&reviewerTasks, &incidents); err != nil {
		t.Fatalf("count reviewer tasks and incidents: %v", err)
	}
	if reviewerTasks != 1 || incidents != 0 {
		t.Fatalf("successful dispatch left %d reviewer tasks and %d incidents, want 1 and 0", reviewerTasks, incidents)
	}
}

func TestWorkflowControlRecoveryTaskWithoutEventRaisesTechnicalStop(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	if _, err := testHandler.TaskService.CompleteTask(ctx, parseUUID(fx.SourceTaskID), []byte(`{"output":"finished without workflow event"}`), "", "", "", false, "", ""); err != nil {
		t.Fatalf("complete source task: %v", err)
	}
	var recoveryTaskID string
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM agent_task_queue
		WHERE issue_id = $1
		  AND trigger_evidence_kind = 'workflow_missing_event'
		  AND trigger_evidence_ref_id = $2
	`, fx.IssueID, fx.SourceTaskID).Scan(&recoveryTaskID); err != nil {
		t.Fatalf("load recovery task: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'running', started_at = now()
		WHERE id = $1
	`, recoveryTaskID); err != nil {
		t.Fatalf("start recovery task: %v", err)
	}
	if _, err := testHandler.TaskService.CompleteTask(ctx, parseUUID(recoveryTaskID), []byte(`{"output":"recovery also finished without workflow event"}`), "", "", "", false, "", ""); err != nil {
		t.Fatalf("complete recovery task: %v", err)
	}

	plan, err := testHandler.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{
		IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load stopped plan: %v", err)
	}
	if plan.CurrentState != string(workflowstate.Blocked) {
		t.Fatalf("plan state = %q, want blocked", plan.CurrentState)
	}
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "blocked" {
		t.Fatalf("issue status = %q, want blocked", status)
	}
	picture, err := testHandler.Queries.GetIssueCurrentPicture(ctx, db.GetIssueCurrentPictureParams{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(fx.IssueID),
	})
	if err != nil {
		t.Fatalf("load technical-stop picture: %v", err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	if summary.Stage != workflowprojection.OwnerStageTechnicalStop {
		t.Fatalf("current picture stage = %q, want technical stop", summary.Stage)
	}
	var incidents int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM inbox_item
		WHERE issue_id = $1 AND type = 'workflow_incident' AND NOT archived
	`, fx.IssueID).Scan(&incidents); err != nil {
		t.Fatal(err)
	}
	if incidents != 1 {
		t.Fatalf("workflow incident inbox items = %d, want 1", incidents)
	}
	if got := countWorkflowTasks(t, "workflow_missing_event", recoveryTaskID); got != 0 {
		t.Fatalf("nested recovery tasks = %d, want 0", got)
	}
}
