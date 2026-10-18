package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func prepareOperatorCompletionFixture(t *testing.T) (workflowControlFixture, string, string) {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	testutil.Call(t, testHandler.PutIssueWorkflowPlan, putWorkflowPlanRequest(fx)).Want(http.StatusOK)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='failed',completed_at=now() WHERE id=$1`, fx.SourceTaskID)
	reviewOutput, _ := json.Marshal(map[string]any{"output": `{"summary":"Independent verification passed.","verdict":"pass","findings":[]}`})
	reviewID := dbfx.Task(t, fx.ReviewerID, testutil.Cols{
		"issue_id": fx.IssueID, "squad_id": fx.SquadID, "status": "completed", "runtime_id": fx.RuntimeID,
		"started_at": testutil.Raw("now()"), "completed_at": testutil.Raw("now()"), "result": reviewOutput,
		"accountable_user_id": testUserID,
	})
	commentID := dbfx.Comment(t, fx.IssueID, "Reviewed published artifact SHA256: "+strings.Repeat("a", 64))
	params, err := workflowprojection.LegacyTechnicalStopParams(parseUUID(testWorkspaceID), parseUUID(fx.IssueID), parseUUID(fx.SourceTaskID), "earlier publication failed", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_, err = testHandler.Queries.UpsertIssueCurrentPicture(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	_, err = projectOwnerInboxItem(context.Background(), testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(fx.IssueID), 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM inbox_item WHERE issue_id=$1`, fx.IssueID)
		testPool.Exec(context.Background(), `DELETE FROM issue_current_picture WHERE issue_id=$1`, fx.IssueID)
	})
	return fx, reviewID, commentID
}

func TestOperatorCompletionSettlesAllAuthoritativeViews(t *testing.T) {
	fx, reviewID, commentID := prepareOperatorCompletionFixture(t)
	// Reproduction: the existing manual status command commits Done but leaves
	// Current Picture (and therefore Now/Inbox/Board) at the earlier failure.
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest(http.MethodPatch, "/api/issues/"+fx.IssueID, map[string]any{"status": "done"}), "id", fx.IssueID)).Want(http.StatusOK)
	picture, err := testHandler.Queries.GetIssueCurrentPicture(context.Background(), db.GetIssueCurrentPictureParams{WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(fx.IssueID)})
	if err != nil {
		t.Fatal(err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	if summary.Stage != workflowprojection.OwnerStageTechnicalStop {
		t.Fatal("manual status unexpectedly erased the stop before acceptance")
	}
	var response operatorCompletionResponse
	testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusOK).JSON(&response)
	if response.Replay || response.PictureRevision != 2 || response.CompletionMode != "operator_assisted" {
		t.Fatalf("completion response: %#v", response)
	}
	var detail issueCurrentPictureResponse
	testutil.Call(t, testHandler.GetIssueCurrentPicture, withURLParam(newRequest(http.MethodGet, "/api/issues/"+fx.IssueID+"/current-picture", nil), "id", fx.IssueID)).Want(http.StatusOK).JSON(&detail)
	if detail.OwnerSummary.Stage != workflowprojection.OwnerStageComplete || detail.OutcomeOpen || detail.TerminalReason != "" || detail.OwnerSummary.NextAction != nil {
		t.Fatalf("completion picture: %#v", detail)
	}
	assertOperatorCompletionState(t, fx.IssueID, 1, "done", 2)
	var replay operatorCompletionResponse
	testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusOK).JSON(&replay)
	if !replay.Replay || replay.CompletionID != response.CompletionID {
		t.Fatal("exact replay duplicated completion")
	}
	assertOperatorCompletionState(t, fx.IssueID, 1, "done", 2)
	var event db.IssueWorkflowEvent
	event, err = testHandler.Queries.GetIssueWorkflowEvent(context.Background(), db.GetIssueWorkflowEventParams{ID: parseUUID(response.CompletionID), IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	var record operatorCompletionRecord
	if json.Unmarshal(event.Payload, &record) != nil || record.PreviousPicture == nil || record.PreviousPicture.CurrentMilestone != "technical_stop" || event.ActorID != parseUUID(testUserID) || record.ActorType != "member" {
		t.Fatal("operator provenance or previous stop was lost")
	}
	var failures, archived int
	testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='failed'`, fx.IssueID).Scan(&failures)
	testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_incident' AND archived`, fx.IssueID).Scan(&archived)
	if failures != 1 || archived != 1 {
		t.Fatalf("history changed or incident remains active: failures=%d archived=%d", failures, archived)
	}
}

func TestOperatorCompletionDirectOwnerReview(t *testing.T) {
	for _, scenario := range []string{"accepted", "different_author", "missing_originator", "different_issue", "different_trigger", "different_kind", "missing_trigger", "delegated", "later_comment"} {
		t.Run(scenario, func(t *testing.T) {
			fx, reviewID, commentID := prepareOperatorCompletionFixture(t)
			triggerID := dbfx.Comment(t, fx.IssueID, "Please independently review this issue's candidate.")
			dbfx.Exec(t, `UPDATE comment SET created_at=now()-interval '1 minute' WHERE id=$1`, triggerID)
			// Native direct-owner mentions retain comment provenance and no squad_id.
			dbfx.Exec(t, `UPDATE agent_task_queue SET squad_id=NULL,originator_user_id=$2,trigger_comment_id=$3,trigger_evidence_kind='comment',trigger_evidence_ref_id=$3 WHERE id=$1`, reviewID, testUserID, triggerID)
			switch scenario {
			case "different_author":
				dbfx.Exec(t, `UPDATE comment SET author_id=gen_random_uuid() WHERE id=$1`, triggerID)
			case "missing_originator":
				dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=NULL WHERE id=$1`, reviewID)
			case "different_issue":
				dbfx.Exec(t, `UPDATE comment SET issue_id=$2 WHERE id=$1`, triggerID, dbfx.Issue(t, "Other review"))
			case "different_trigger":
				dbfx.Exec(t, `UPDATE agent_task_queue SET trigger_evidence_ref_id=gen_random_uuid() WHERE id=$1`, reviewID)
			case "different_kind":
				dbfx.Exec(t, `UPDATE agent_task_queue SET trigger_evidence_kind='delegation' WHERE id=$1`, reviewID)
			case "missing_trigger":
				dbfx.Exec(t, `UPDATE agent_task_queue SET trigger_comment_id=NULL,trigger_evidence_ref_id=NULL WHERE id=$1`, reviewID)
			case "delegated":
				dbfx.Exec(t, `UPDATE comment SET author_type='agent',author_id=$2 WHERE id=$1`, triggerID, fx.ImplementerID)
			case "later_comment":
				dbfx.Exec(t, `UPDATE comment SET created_at=now()+interval '1 minute' WHERE id=$1`, triggerID)
			}
			if scenario != "accepted" {
				testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusConflict)
				assertOperatorCompletionState(t, fx.IssueID, 0, "working", 1)
				return
			}
			testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusOK)
			assertOperatorCompletionState(t, fx.IssueID, 1, "done", 2)
			var originalSquad pgtype.UUID
			if err := testPool.QueryRow(context.Background(), `SELECT squad_id FROM agent_task_queue WHERE id=$1`, reviewID).Scan(&originalSquad); err != nil || originalSquad.Valid {
				t.Fatalf("direct review history was rewritten: squad=%v error=%v", originalSquad, err)
			}
		})
	}
}

func operatorCompletionBody(reviewID, commentID string) map[string]any {
	return map[string]any{"expected_plan_revision": 1, "expected_picture_revision": 1, "review_task_id": reviewID, "evidence_comment_id": commentID, "evidence_comment_revision": 1, "candidate_sha256": strings.Repeat("a", 64), "reason": "Accepted the published candidate after independent review; operator assistance is explicit."}
}
func operatorCompletionHTTPRequest(fx workflowControlFixture, reviewID, commentID string) *http.Request {
	return withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/operator-completion", operatorCompletionBody(reviewID, commentID)), "id", fx.IssueID)
}
func assertOperatorCompletionState(t *testing.T, issueID string, wantEvents int, wantPlan string, wantRevision int64) {
	t.Helper()
	var events, completions int
	var plan, status string
	var revision int64
	err := testPool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM issue_workflow_event WHERE issue_id=$1 AND event_type='operator_completed'), (SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='task_completed' AND details->>'completion_mode'='operator_assisted'), p.current_state,i.status,c.revision FROM issue i JOIN issue_workflow_plan p ON p.issue_id=i.id JOIN issue_current_picture c ON c.issue_id=i.id WHERE i.id=$1`, issueID).Scan(&events, &completions, &plan, &status, &revision)
	if err != nil {
		t.Fatal(err)
	}
	if events != wantEvents || completions != wantEvents || plan != wantPlan || revision != wantRevision || (wantPlan == "done" && status != "done") {
		t.Fatalf("events/completions/plan/status/revision=%d/%d/%s/%s/%d", events, completions, plan, status, revision)
	}
}
func TestOperatorCompletionRejectsUnsafeOrStaleEvidence(t *testing.T) {
	for _, scenario := range []string{"plan_revision", "picture_revision", "comment_revision", "active_task", "reviewer", "failed_review", "wrong_squad", "release_required", "owner_decision", "wrong_comment", "missing_hash", "changed_requester", "machine", "normalized", "altered_replay", "dynamic_owner_state", "suspended", "blocked", "unanswered_event", "replay_picture_identity"} {
		t.Run(scenario, func(t *testing.T) {
			fx, reviewID, commentID := prepareOperatorCompletionFixture(t)
			body := operatorCompletionBody(reviewID, commentID)
			switch scenario {
			case "dynamic_owner_state":
				dbfx.Exec(t, `UPDATE issue_workflow_plan SET current_state='needs_owner',requires_owner=false WHERE issue_id=$1`, fx.IssueID)
			case "suspended":
				dbfx.Exec(t, `UPDATE issue_workflow_plan SET suspended_state='working',suspended_agent_id=$2 WHERE issue_id=$1`, fx.IssueID, fx.ImplementerID)
			case "blocked":
				dbfx.Exec(t, `UPDATE issue_workflow_plan SET current_state='blocked' WHERE issue_id=$1`, fx.IssueID)
			case "unanswered_event":
				dbfx.Exec(t, `INSERT INTO issue_workflow_event(id,workspace_id,issue_id,source_task_id,actor_id,event_type,reason,payload,decision) VALUES(gen_random_uuid(),$1,$2,$3,$4,'owner_decision_required','Unanswered question','{}','{"next":"needs_owner","notify_owner":true}')`, testWorkspaceID, fx.IssueID, fx.SourceTaskID, fx.ImplementerID)
			case "replay_picture_identity":
				testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusOK)
				dbfx.Exec(t, `UPDATE issue_current_picture SET source_revisions='[{"type":"legacy_workflow_event","id":"11111111-1111-4111-8111-111111111111","revision":1}]' WHERE issue_id=$1`, fx.IssueID)
			case "plan_revision":
				body["expected_plan_revision"] = 2
			case "picture_revision":
				body["expected_picture_revision"] = 0
			case "comment_revision":
				body["evidence_comment_revision"] = 2
			case "active_task":
				dbfx.Exec(t, `UPDATE agent_task_queue SET status='queued',completed_at=NULL WHERE id=$1`, fx.SourceTaskID)
			case "reviewer":
				dbfx.Exec(t, `UPDATE agent_task_queue SET agent_id=$2 WHERE id=$1`, reviewID, fx.ImplementerID)
			case "failed_review":
				dbfx.Exec(t, `UPDATE agent_task_queue SET result='{"output":"{\"summary\":\"Review failed\",\"verdict\":\"fail\",\"findings\":[\"bad\"]}"}' WHERE id=$1`, reviewID)
			case "wrong_squad":
				dbfx.Exec(t, `UPDATE agent_task_queue SET squad_id=gen_random_uuid() WHERE id=$1`, reviewID)
			case "release_required":
				dbfx.Exec(t, `UPDATE issue_workflow_plan SET requires_deployment=true WHERE issue_id=$1`, fx.IssueID)
			case "owner_decision":
				dbfx.Exec(t, `UPDATE issue_workflow_plan SET requires_owner=true WHERE issue_id=$1`, fx.IssueID)
			case "wrong_comment":
				body["evidence_comment_id"] = dbfx.Comment(t, dbfx.Issue(t, "other issue"), "other "+strings.Repeat("a", 64))
			case "missing_hash":
				body["candidate_sha256"] = strings.Repeat("b", 64)
			case "changed_requester":
				dbfx.Exec(t, `UPDATE issue SET creator_id=gen_random_uuid() WHERE id=$1`, fx.IssueID)
			case "normalized":
				dbfx.Exec(t, `UPDATE issue_current_picture SET scope_revision=1 WHERE issue_id=$1`, fx.IssueID)
			case "altered_replay":
				testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusOK)
				body["reason"] = "Changed after completion"
			}
			request := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/operator-completion", body), "id", fx.IssueID)
			want := http.StatusConflict
			if scenario == "machine" {
				request.Header.Set("X-Actor-Source", "task_token")
				want = http.StatusForbidden
			}
			if scenario == "changed_requester" {
				want = http.StatusForbidden
			}
			testutil.Call(t, testHandler.PostIssueOperatorCompletion, request).Want(want)
			switch scenario {
			case "altered_replay", "replay_picture_identity":
				assertOperatorCompletionState(t, fx.IssueID, 1, "done", 2)
			case "dynamic_owner_state":
				assertOperatorCompletionState(t, fx.IssueID, 0, "needs_owner", 1)
			case "blocked":
				assertOperatorCompletionState(t, fx.IssueID, 0, "blocked", 1)
			default:
				assertOperatorCompletionState(t, fx.IssueID, 0, "working", 1)
			}
		})
	}
}
func TestOperatorCompletionRollsBackWhenInboxFails(t *testing.T) {
	fx, reviewID, commentID := prepareOperatorCompletionFixture(t)
	original := testHandler.TaskService.AdmitLegacyWorkflowProjection
	testHandler.TaskService.AdmitLegacyWorkflowProjection = func(context.Context, *db.Queries, pgtype.UUID, pgtype.UUID, int64) (*db.InboxItem, error) {
		return nil, errors.New("injected admission failure")
	}
	t.Cleanup(func() { testHandler.TaskService.AdmitLegacyWorkflowProjection = original })
	testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusConflict)
	assertOperatorCompletionState(t, fx.IssueID, 0, "working", 1)
	var active int
	testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_incident' AND NOT archived`, fx.IssueID).Scan(&active)
	if active != 1 {
		t.Fatal("incident archived despite transaction rollback")
	}
}
func TestOperatorCompletionConcurrentReplayIsSingleCommit(t *testing.T) {
	fx, reviewID, commentID := prepareOperatorCompletionFixture(t)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testutil.Call(t, testHandler.PostIssueOperatorCompletion, operatorCompletionHTTPRequest(fx, reviewID, commentID)).Want(http.StatusOK)
		}()
	}
	wg.Wait()
	assertOperatorCompletionState(t, fx.IssueID, 1, "done", 2)
}
