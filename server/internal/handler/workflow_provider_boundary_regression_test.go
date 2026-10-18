package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowguard"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestWorkflowImplementationReadyArtifactEvidenceFailsClosed(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	valid := validRepositoryInventoryEvidence()
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "missing evidence", mutate: func(packet map[string]any) { delete(packet, "artifact_evidence") }},
		{name: "missing per-worktree records", mutate: func(packet map[string]any) {
			packet["artifact_evidence"].(map[string]any)["worktrees"] = []any{}
		}},
		{name: "duplicate worktree record", mutate: func(packet map[string]any) {
			evidence := packet["artifact_evidence"].(map[string]any)
			worktrees := evidence["worktrees"].([]any)
			worktrees[1].(map[string]any)["path"] = worktrees[0].(map[string]any)["path"]
		}},
		{name: "lying worktree count", mutate: func(packet map[string]any) {
			packet["artifact_evidence"].(map[string]any)["inventory"].(map[string]any)["worktree_count"] = 3
		}},
		{name: "insufficient path checks", mutate: func(packet map[string]any) {
			packet["artifact_evidence"].(map[string]any)["inventory"].(map[string]any)["completed_path_check_count"] = 3
		}},
		{name: "stale artifact hash", mutate: func(packet map[string]any) {
			packet["artifact_evidence"].(map[string]any)["artifacts"].([]any)[0].(map[string]any)["observed_sha256"] = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{name: "inconsistent dirty arithmetic", mutate: func(packet map[string]any) {
			packet["artifact_evidence"].(map[string]any)["roots"].([]any)[0].(map[string]any)["dirty"] = false
		}},
		{name: "inconsistent path arithmetic", mutate: func(packet map[string]any) {
			packet["artifact_evidence"].(map[string]any)["inventory"].(map[string]any)["present_path_count"] = 2
		}},
		{name: "altered record digest", mutate: func(packet map[string]any) {
			packet["artifact_evidence"].(map[string]any)["worktrees"].([]any)[0].(map[string]any)["branch"] = "altered"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newWorkflowControlFixture(t)
			testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
			packet := map[string]any{
				"event": workflowstate.ImplementationReady, "reason": "fixture evidence",
				"artifact_evidence": cloneJSONMap(t, valid),
			}
			tt.mutate(packet)
			req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", packet), "id", fx.IssueID)
			req.Header.Set("X-Agent-ID", fx.ImplementerID)
			req.Header.Set("X-Task-ID", fx.SourceTaskID)
			testutil.Call(t, testHandler.PostIssueWorkflowEvent, req).Want(http.StatusBadRequest)

			var events, transitions int
			if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_workflow_event WHERE issue_id = $1`, fx.IssueID).Scan(&events); err != nil {
				t.Fatal(err)
			}
			if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'workflow_event'`, fx.IssueID).Scan(&transitions); err != nil {
				t.Fatal(err)
			}
			if events != 0 || transitions != 0 {
				t.Fatalf("invalid evidence committed events/transitions = %d/%d, want 0/0", events, transitions)
			}
		})
	}
}

func TestWorkflowImplementationReadyAllowsSameWorktreePathAcrossDifferentRoots(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	valid := validRepositoryInventoryEvidence()
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var evidence workflowArtifactEvidencePacketV1
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	evidence.Worktrees[1].Path = evidence.Worktrees[0].Path
	evidence.InventoryRecordsSHA256, err = workflowInventoryRecordsSHA256(evidence.Roots, evidence.Worktrees)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"event": workflowstate.ImplementationReady, "reason": "complete overlapping worktree evidence",
		"artifact_evidence": evidence,
	}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	req.Header.Set("X-Agent-ID", fx.ImplementerID)
	req.Header.Set("X-Task-ID", fx.SourceTaskID)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, req).Want(http.StatusCreated)
}

func TestWorkflowImplementationReadyRejectsExactCompositeWorktreeDuplicate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	valid := validRepositoryInventoryEvidence()
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var evidence workflowArtifactEvidencePacketV1
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	evidence.Worktrees[1] = evidence.Worktrees[0]
	evidence.Inventory.PresentPathCount = 2
	evidence.Inventory.AbsentPathCount = 0
	evidence.InventoryRecordsSHA256, err = workflowInventoryRecordsSHA256(evidence.Roots, evidence.Worktrees)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"event": workflowstate.ImplementationReady, "reason": "exact composite duplicate must fail",
		"artifact_evidence": evidence,
	}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	req.Header.Set("X-Agent-ID", fx.ImplementerID)
	req.Header.Set("X-Task-ID", fx.SourceTaskID)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, req).Want(http.StatusBadRequest)

	var events int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_workflow_event WHERE issue_id = $1`, fx.IssueID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("exact composite duplicate committed %d workflow events", events)
	}
}

func TestWorkflowFailureChainUsesOneControllerBoundaryAndProjectsIncident(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	leaderTaskID := dbfx.Task(t, fx.LeaderID, testutil.Cols{
		"runtime_id": handlerTestRuntimeID(t), "status": "completed", "issue_id": fx.IssueID,
		"started_at": testutil.Raw("now() - interval '10 minutes'"), "completed_at": testutil.Raw("now() - interval '9 minutes'"),
		"squad_id": fx.SquadID, "is_leader_task": true,
	})
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET delegated_from_task_id = $2 WHERE id = $1`, fx.SourceTaskID, leaderTaskID); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)

	// A durable implementation handoff wins over the provider process's later
	// turn-cap/error result. No generic recovery signal may be created.
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusCreated)
	if _, err := testHandler.TaskService.FailTask(ctx, parseUUID(fx.SourceTaskID), "turn cap after durable handoff", "", "", "", "agent_error.turn_limit", false, "", ""); err != nil {
		t.Fatalf("fail implementation task: %v", err)
	}
	assertWorkflowRecoveryFanout(t, fx.IssueID, 0)

	reviewerTask := oneWorkflowTransitionTask(t, fx.IssueID, fx.ReviewerID)
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status = 'running', started_at = now() WHERE id = $1`, reviewerTask.ID); err != nil {
		t.Fatal(err)
	}
	// Review rejection now pauses for an AI failure investigation, covered by
	// TestWorkflowControlReviewFailureStartsInvestigationBeforeCorrection.
	// Exercise the technical-stop boundary with no durable review result.
	if _, err := testHandler.TaskService.FailTask(ctx, reviewerTask.ID, "review ended before a structured result", "", "", "", "agent_error.turn_limit", false, "", ""); err != nil {
		t.Fatalf("fail reviewer task: %v", err)
	}
	assertWorkflowRecoveryFanout(t, fx.IssueID, 0)

	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "blocked" {
		t.Fatalf("failed workflow issue status = %q, want blocked", status)
	}
	picture, err := testHandler.Queries.GetIssueCurrentPicture(ctx, db.GetIssueCurrentPictureParams{WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(fx.IssueID)})
	if err != nil {
		t.Fatalf("load projected failure: %v", err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	if summary.Stage != workflowprojection.OwnerStageTechnicalStop || summary.LatestMeaningfulChange == nil || summary.LatestMeaningfulChange.ResultCode != "failed" {
		t.Fatalf("failure summary = %#v, want technical stop with latest failed change", summary)
	}
	if summary.NextAction == nil || summary.NextAction.Kind != "inspect_incident" || summary.NextAction.Automatic {
		t.Fatalf("failure next action = %#v, want manual inspect_incident", summary.NextAction)
	}
	var incidents int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_incident'`, fx.IssueID).Scan(&incidents); err != nil {
		t.Fatal(err)
	}
	if incidents != 1 {
		t.Fatalf("workflow incident inbox items = %d, want 1", incidents)
	}

	// A stale legacy recovery task cannot start a second missing-event chain.
	legacyRecoveryID := dbfx.Task(t, fx.ReviewerID, testutil.Cols{
		"runtime_id": handlerTestRuntimeID(t), "status": "running", "issue_id": fx.IssueID,
		"started_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
		"delegated_from_task_id": reviewerTask.ID,
		"trigger_evidence_kind":  "delegated_failure", "trigger_evidence_ref_id": reviewerTask.ID,
	})
	if _, err := testHandler.TaskService.CompleteTask(ctx, parseUUID(legacyRecoveryID), []byte(`{"output":"recovery stopped"}`), "", "", "", false, "", ""); err != nil {
		t.Fatalf("complete stale recovery task: %v", err)
	}
	assertWorkflowRecoveryFanout(t, fx.IssueID, 1)
}

func TestWorkflowProviderTaskCeilingRejectsBeforeFakeProviderStart(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	// The plan's one-correction, non-deployment shape permits five provider-
	// backed starts including the intake leader. Seed that complete budget.
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, fx.SourceTaskID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		dbfx.Task(t, fx.ReviewerID, testutil.Cols{
			"runtime_id": handlerTestRuntimeID(t), "status": "completed", "issue_id": fx.IssueID,
			"started_at": testutil.Raw("now() - interval '1 minute'"), "completed_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
		})
	}
	candidateID := dbfx.Task(t, fx.ImplementerID, testutil.Cols{
		"runtime_id": handlerTestRuntimeID(t), "status": "dispatched", "issue_id": fx.IssueID,
		"dispatched_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
	})

	fakeProviderStarts := 0
	startProvider := func() error {
		if _, err := testHandler.TaskService.StartTask(ctx, parseUUID(candidateID)); err != nil {
			return err
		}
		fakeProviderStarts++
		return nil
	}
	err := startProvider()
	if err == nil || !strings.Contains(err.Error(), "provider task ceiling") {
		t.Fatalf("ceiling start error = %v, want provider task ceiling", err)
	}
	if fakeProviderStarts != 0 {
		t.Fatalf("fake provider starts = %d, want 0", fakeProviderStarts)
	}
	var taskStatus string
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, candidateID).Scan(&taskStatus); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "cancelled" {
		t.Fatalf("ceiling-rejected task status = %q, want cancelled", taskStatus)
	}
	var started int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND started_at IS NOT NULL`, fx.IssueID).Scan(&started); err != nil {
		t.Fatal(err)
	}
	if started != 5 {
		t.Fatalf("started provider-task reservations = %d, want 5", started)
	}
	plan, err := testHandler.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{
		IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentState != string(workflowstate.Blocked) {
		t.Fatalf("ceiling-exhausted plan state = %q, want blocked", plan.CurrentState)
	}
	var issueStatus string
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&issueStatus); err != nil {
		t.Fatal(err)
	}
	if issueStatus != "blocked" {
		t.Fatalf("ceiling-exhausted issue status = %q, want blocked", issueStatus)
	}
	picture, err := testHandler.Queries.GetIssueCurrentPicture(ctx, db.GetIssueCurrentPictureParams{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(fx.IssueID),
	})
	if err != nil {
		t.Fatalf("load ceiling technical stop: %v", err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	if summary.Stage != workflowprojection.OwnerStageTechnicalStop || summary.NextAction == nil || summary.NextAction.Kind != "inspect_incident" {
		t.Fatalf("ceiling summary = %#v, want owner-visible technical stop", summary)
	}
	var incidents int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_incident'`, fx.IssueID).Scan(&incidents); err != nil {
		t.Fatal(err)
	}
	if incidents != 1 {
		t.Fatalf("ceiling incident inbox items = %d, want 1", incidents)
	}
}

func TestWorkflowProviderTaskCeilingSerializesRacingStarts(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	// Leave one of the five start reservations available, then race two
	// dispatched candidates for it. The plan-row lock must admit exactly one.
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, fx.SourceTaskID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		dbfx.Task(t, fx.ReviewerID, testutil.Cols{
			"runtime_id": handlerTestRuntimeID(t), "status": "completed", "issue_id": fx.IssueID,
			"started_at": testutil.Raw("now() - interval '1 minute'"), "completed_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
		})
	}
	candidates := []string{
		dbfx.Task(t, fx.ImplementerID, testutil.Cols{
			"runtime_id": handlerTestRuntimeID(t), "status": "dispatched", "issue_id": fx.IssueID,
			"dispatched_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
		}),
		dbfx.Task(t, fx.ReviewerID, testutil.Cols{
			"runtime_id": handlerTestRuntimeID(t), "status": "dispatched", "issue_id": fx.IssueID,
			"dispatched_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
		}),
	}

	start := make(chan struct{})
	results := make(chan error, len(candidates))
	var fakeProviderStarts atomic.Int32
	for _, candidateID := range candidates {
		candidateID := candidateID
		go func() {
			<-start
			_, err := testHandler.TaskService.StartTask(ctx, parseUUID(candidateID))
			if err == nil {
				fakeProviderStarts.Add(1)
			}
			results <- err
		}()
	}
	close(start)
	var admitted, denied int
	for range candidates {
		err := <-results
		switch {
		case err == nil:
			admitted++
		case strings.Contains(err.Error(), "provider task ceiling"):
			denied++
		default:
			t.Fatalf("racing start returned unexpected error: %v", err)
		}
	}
	if admitted != 1 || denied != 1 || fakeProviderStarts.Load() != 1 {
		t.Fatalf("racing starts admitted/denied/provider = %d/%d/%d, want 1/1/1", admitted, denied, fakeProviderStarts.Load())
	}
	var running, cancelled, started int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'running'),
		       count(*) FILTER (WHERE status = 'cancelled')
		FROM agent_task_queue WHERE id = ANY($1::uuid[])
	`, candidates).Scan(&running, &cancelled); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND squad_id = $2 AND started_at IS NOT NULL`, fx.IssueID, fx.SquadID).Scan(&started); err != nil {
		t.Fatal(err)
	}
	if running != 1 || cancelled != 1 || started != 5 {
		t.Fatalf("racing task states running/cancelled/started = %d/%d/%d, want 1/1/5", running, cancelled, started)
	}
	plan, err := testHandler.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{
		IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentState == string(workflowstate.Blocked) {
		t.Fatal("ceiling race blocked the workflow despite an admitted active task")
	}
}

func TestWorkflowAcceptedEventRemainsAuthoritativeWithoutSuccessorRow(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusCreated)
	successor := oneWorkflowTransitionTask(t, fx.IssueID, fx.ReviewerID)
	dbfx.Exec(t, `DELETE FROM agent_task_queue WHERE id = $1`, successor.ID)

	advanced, err := testHandler.Queries.HasAdvancingIssueWorkflowEventForTask(ctx, parseUUID(fx.SourceTaskID))
	if err != nil || !advanced {
		t.Fatalf("accepted event finality = %v, err %v, want true", advanced, err)
	}
	if _, err := testHandler.TaskService.FailTask(ctx, parseUUID(fx.SourceTaskID), "provider ended after accepted handoff", "", "", "", "agent_error.unknown", false, "", ""); err != nil {
		t.Fatalf("fail source after accepted event: %v", err)
	}
	plan, err := testHandler.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil || plan.CurrentState != string(workflowstate.TechnicalReview) {
		t.Fatalf("plan after accepted event/source failure = %#v, err %v", plan, err)
	}
	var issueStatus string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&issueStatus)
	if issueStatus == "blocked" {
		t.Fatal("accepted event was overturned into a technical stop")
	}
}

func TestWorkflowProviderCeilingSurvivesRoleMutation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, fx.SourceTaskID)
	for i := 0; i < 4; i++ {
		dbfx.Task(t, fx.ReviewerID, testutil.Cols{
			"runtime_id": handlerTestRuntimeID(t), "status": "completed", "issue_id": fx.IssueID,
			"started_at": testutil.Raw("now() - interval '1 minute'"), "completed_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
		})
	}
	candidateID := dbfx.Task(t, fx.ReviewerID, testutil.Cols{
		"runtime_id": handlerTestRuntimeID(t), "status": "dispatched", "issue_id": fx.IssueID,
		"dispatched_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
	})
	// Simulate an owner changing the live reviewer after this task was queued.
	// Provenance stays issue+squad bound and must not make the start ceiling
	// disappear for the already-dispatched row.
	dbfx.Exec(t, `UPDATE issue_workflow_plan SET reviewer_id = $2 WHERE issue_id = $1`, fx.IssueID, fx.LeaderID)
	if _, err := testHandler.TaskService.StartTask(ctx, parseUUID(candidateID)); err == nil || !strings.Contains(err.Error(), "provider task ceiling") {
		t.Fatalf("role-mutated ceiling error = %v, want provider task ceiling", err)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id = $1`, candidateID).Scan(&status)
	if status != "cancelled" {
		t.Fatalf("role-mutated candidate status = %q, want cancelled", status)
	}
}

func TestWorkflowProviderStartFailsClosedWithoutTransactionStarter(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	candidateID := dbfx.Task(t, fx.ReviewerID, testutil.Cols{
		"runtime_id": handlerTestRuntimeID(t), "status": "dispatched", "issue_id": fx.IssueID,
		"dispatched_at": testutil.Raw("now()"), "squad_id": fx.SquadID,
	})
	svc := service.NewTaskService(
		testHandler.Queries,
		nil,
		testHandler.TaskService.Hub,
		testHandler.TaskService.Bus,
		testHandler.TaskService.Wakeup,
	)
	if _, err := svc.StartTask(context.Background(), parseUUID(candidateID)); err == nil || !strings.Contains(err.Error(), "transaction support") {
		t.Fatalf("nil transaction starter error = %v, want fail-closed transaction support error", err)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id = $1`, candidateID).Scan(&status)
	if status != "dispatched" {
		t.Fatalf("nil-transaction candidate status = %q, want dispatched", status)
	}
}

func TestWorkflowSweeperFailureUsesControllerStopWithoutRecoveryFanout(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'failed', error = 'sweeper terminal', failure_reason = 'agent_error.unknown', completed_at = now() WHERE id = $1`, fx.SourceTaskID)
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(fx.SourceTaskID))
	if err != nil {
		t.Fatal(err)
	}
	if retried := testHandler.TaskService.HandleFailedTasks(ctx, []db.AgentTaskQueue{task}); retried != 0 {
		t.Fatalf("workflow sweeper retries = %d, want 0", retried)
	}
	assertWorkflowRecoveryFanout(t, fx.IssueID, 0)
	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&status)
	if status != "blocked" {
		t.Fatalf("sweeper-controlled issue status = %q, want blocked", status)
	}
	var incidents int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_incident'`, fx.IssueID).Scan(&incidents)
	if incidents != 1 {
		t.Fatalf("sweeper incident inbox items = %d, want 1", incidents)
	}
}

func TestWorkflowTechnicalStopAndOwnerInboxRollBackTogether(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'failed', error = 'atomic projection fixture', failure_reason = 'agent_error.unknown', completed_at = now() WHERE id = $1`, fx.SourceTaskID)
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(fx.SourceTaskID))
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewTaskService(
		testHandler.Queries,
		testHandler.TaskService.TxStarter,
		testHandler.TaskService.Hub,
		testHandler.TaskService.Bus,
		testHandler.TaskService.Wakeup,
	)
	svc.AdmitLegacyWorkflowProjection = func(context.Context, *db.Queries, pgtype.UUID, pgtype.UUID, int64) (*db.InboxItem, error) {
		return nil, errors.New("injected owner Inbox admission failure")
	}
	svc.HandleFailedTasks(ctx, []db.AgentTaskQueue{task})
	plan, err := testHandler.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	var issueStatus string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, fx.IssueID).Scan(&issueStatus)
	var incidents int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_incident'`, fx.IssueID).Scan(&incidents)
	if plan.CurrentState == string(workflowstate.Blocked) || issueStatus == "blocked" || incidents != 0 {
		t.Fatalf("partial technical stop committed plan/status/incidents = %q/%q/%d", plan.CurrentState, issueStatus, incidents)
	}
}

func TestWorkflowTransitionFreezesBoundedAssignmentAndExactSkills(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusCreated)
	task := oneWorkflowTransitionTask(t, fx.IssueID, fx.ReviewerID)

	packet, assignment, present, err := workflowguard.ExtractLegacyAssignment(task.Context)
	if err != nil || !present {
		t.Fatalf("bounded assignment present/error = %v/%v: %s", present, err, task.Context)
	}
	if len(task.Context) > workflowguard.MaxLegacyAssignmentBytes || assignment.TaskKind != "review" ||
		assignment.IssueID != fx.IssueID || assignment.SourceTaskID != fx.SourceTaskID ||
		assignment.TargetAgentID != fx.ReviewerID || assignment.ContextPolicy.LoadIssue ||
		assignment.ContextPolicy.LoadComments || assignment.ContextPolicy.LoadRepositoryDocs || len(assignment.RequiredSkills) == 0 ||
		!task.ForceFreshSession || len(task.RuntimeMcpOverlay) != 0 || len(task.RuntimeConnectedApps) != 0 {
		t.Fatalf("bounded assignment = %#v", assignment)
	}
	for _, skill := range assignment.RequiredSkills {
		if !strings.HasPrefix(skill.SHA256, "sha256:") {
			t.Fatalf("skill binding lacks exact digest: %#v", skill)
		}
	}
	response := taskToResponse(task, testWorkspaceID)
	if string(response.WorkflowPacket) != string(packet) {
		t.Fatalf("claim packet = %s, want %s", response.WorkflowPacket, packet)
	}
}

func TestWorkflowTransitionAcceptanceMarkerCannotGrantBrokerAuthority(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, "bounded broker assignment", testutil.Cols{"provider": "claude"})
	fx := newWorkflowControlFixture(t, runtimeID)
	digest := "sha256:" + strings.Repeat("a", 64)
	dbfx.Exec(t, `UPDATE issue SET acceptance_criteria = $2::jsonb WHERE id = $1`, fx.IssueID,
		`["bounded-workflow-broker-v1 `+digest+`"]`)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusCreated)
	task := oneWorkflowTransitionTask(t, fx.IssueID, fx.ReviewerID)

	_, assignment, present, err := workflowguard.ExtractLegacyAssignment(task.Context)
	if err != nil || !present {
		t.Fatalf("broker assignment present/error = %v/%v: %s", present, err, task.Context)
	}
	if assignment.ExecutionPolicy.ToolPolicy != workflowguard.LegacyToolPolicy ||
		assignment.ExecutionPolicy.BrokerExecutableSHA256 != "" || assignment.RuntimeProvider != "claude" {
		t.Fatalf("editable acceptance marker granted execution authority: %#v", assignment.ExecutionPolicy)
	}
}

func TestWorkflowClaimEnforcesBoundedAssignmentBeforeCapabilityExposure(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	tests := []struct {
		name       string
		capability string
		mutate     func(t *testing.T, taskID string, fx workflowControlFixture)
		wantStatus int
	}{
		{
			name:       "exact assignment and skill snapshot",
			capability: protocol.DaemonCapabilitySkillBundlesV1 + "," + protocol.DaemonCapabilityPlatformSkillV1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "altered broker executable digest",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,execution_policy,broker_executable_sha256}', to_jsonb('sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'::text), true) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered assignment identity",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, fx workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,target_agent_id}', to_jsonb($2::text), false) WHERE id = $1`, taskID, fx.ImplementerID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered task kind",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,task_kind}', to_jsonb('correction'::text), false) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered issue description",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,issue_description}', to_jsonb('altered'::text), false) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered acceptance criteria",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,acceptance_criteria}', '["altered"]'::jsonb, false) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered event reason",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,workflow_event_reason}', to_jsonb('altered'::text), false) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered event payload",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,workflow_event_payload}', '{"altered":true}'::jsonb, false) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered model selector",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,model}', to_jsonb('altered/model'::text), false) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "current agent model changed after enqueue",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, _ string, fx workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent SET model = 'changed-after-enqueue' WHERE id = $1`, fx.ReviewerID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "altered skill digest",
			capability: protocol.DaemonCapabilitySkillBundlesV1,
			mutate: func(t *testing.T, taskID string, _ workflowControlFixture) {
				dbfx.Exec(t, `UPDATE agent_task_queue SET context = jsonb_set(context, '{workflow_assignment_v1,required_skills,0,sha256}', to_jsonb('sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'::text), false) WHERE id = $1`, taskID)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "daemon lacks exact skill capability",
			capability: "",
			wantStatus: http.StatusConflict,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeID := dbfx.Runtime(t, "bounded workflow claim "+tt.name, testutil.Cols{"provider": "claude"})
			fx := newWorkflowControlFixture(t, runtimeID)
			digest := "sha256:" + strings.Repeat("a", 64)
			dbfx.Exec(t, `UPDATE issue SET acceptance_criteria = $2::jsonb WHERE id = $1`, fx.IssueID,
				`["bounded-workflow-broker-v1 `+digest+`"]`)
			testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
			testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowEventRequest(fx, workflowstate.ImplementationReady)).Want(http.StatusCreated)
			task := oneWorkflowTransitionTask(t, fx.IssueID, fx.ReviewerID)
			dbfx.Exec(t, `
				UPDATE agent SET
				  custom_env = '{"AMBIENT_SECRET":"must-not-cross"}'::jsonb,
				  custom_args = '["--ambient-arg"]'::jsonb,
				  mcp_config = '{"mcpServers":{"ambient":{}}}'::jsonb,
				  runtime_config = '{"ambient":true}'::jsonb,
				  service_tier = 'ambient-tier'
				WHERE id = $1
			`, fx.ReviewerID)
			if tt.mutate != nil {
				tt.mutate(t, uuidToString(task.ID), fx)
			}

			req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "bounded-workflow-claim")
			req = withURLParam(req, "runtimeId", runtimeID)
			if tt.capability != "" {
				req.Header.Set("X-Client-Capabilities", tt.capability)
			}
			response := testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(tt.wantStatus)
			if tt.wantStatus != http.StatusOK {
				var status, failureReason string
				dbfx.QueryRow(t, `SELECT status, failure_reason FROM agent_task_queue WHERE id = $1`, task.ID).Scan(&status, &failureReason)
				if status != "failed" || failureReason == "" {
					t.Fatalf("denied workflow claim settled as status/reason %q/%q", status, failureReason)
				}
				return
			}

			var envelope struct {
				Task AgentTaskResponse `json:"task"`
			}
			if err := json.Unmarshal([]byte(response.Text()), &envelope); err != nil {
				t.Fatalf("decode bounded claim: %v", err)
			}
			claimed := envelope.Task
			if claimed.ID != uuidToString(task.ID) || claimed.Agent == nil || len(claimed.WorkflowPacket) == 0 || len(claimed.Agent.SkillRefs) == 0 {
				t.Fatalf("bounded claim omitted exact task/packet/skills: %+v", claimed)
			}
			if claimed.Agent.Instructions != "" || len(claimed.Agent.CustomEnv) != 0 || len(claimed.Agent.CustomArgs) != 0 ||
				len(claimed.Agent.McpConfig) != 0 || len(claimed.Agent.RuntimeConfig) != 0 || claimed.Agent.ServiceTier != "" ||
				len(claimed.ConnectedApps) != 0 || len(claimed.RemoteMCPConnections) != 0 || len(claimed.PluginHookTools) != 0 ||
				len(claimed.Repos) != 0 || claimed.ProjectID != "" || claimed.ProjectTitle != "" || claimed.ProjectDescription != "" ||
				len(claimed.ProjectResources) != 0 || claimed.WorkspaceContext != "" || len(claimed.IssueStatuses) != 0 ||
				claimed.IssueStatusesOmitted != 0 || claimed.WorkspaceSlug != "" || claimed.IssueIdentifier != "" || claimed.ThreadName != "" ||
				claimed.PriorSessionID != "" || claimed.PriorWorkDir != "" || claimed.PriorSessionResumeUnavailable ||
				claimed.TriggerCommentContent != "" || len(claimed.CoalescedComments) != 0 ||
				claimed.RequestingUserName != "" || claimed.InitiatorName != "" {
				t.Fatalf("bounded claim exposed mutable capability data: %+v", claimed)
			}
		})
	}
}

func assertWorkflowRecoveryFanout(t *testing.T, issueID string, expectedTasks int) {
	t.Helper()
	var comments, tasks int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM comment WHERE issue_id = $1 AND source_task_id IS NOT NULL AND author_type = 'system' AND type = 'progress_update'`, issueID).Scan(&comments); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind IN ('delegated_failure', 'workflow_missing_event')`, issueID).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if comments != 0 || tasks != expectedTasks {
		t.Fatalf("workflow recovery comments/tasks = %d/%d, want 0/%d", comments, tasks, expectedTasks)
	}
}

func oneWorkflowTransitionTask(t *testing.T, issueID, agentID string) db.AgentTaskQueue {
	t.Helper()
	rows, err := testHandler.Queries.ListAgentTasks(context.Background(), parseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range rows {
		if task.IssueID == parseUUID(issueID) && task.TriggerEvidenceKind.Valid && task.TriggerEvidenceKind.String == "workflow_event" && task.Status == "queued" {
			return task
		}
	}
	t.Fatalf("no queued workflow transition task for agent %s", agentID)
	return db.AgentTaskQueue{}
}

func cloneJSONMap(t *testing.T, source map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}
