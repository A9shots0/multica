package handler

import (
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOperatorCompletionPreservesDynamicOwnerQuestion(t *testing.T) {
	fx, reviewID, commentID := prepareOperatorCompletionFixture(t)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='running',completed_at=NULL WHERE id=$1`, fx.SourceTaskID)
	request := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", map[string]any{"event": "owner_decision_required", "reason": "Need a genuine owner choice", "owner_decision": ownerDecisionPacket()}), "id", fx.IssueID)
	request.Header.Set("X-Agent-ID", fx.ImplementerID)
	request.Header.Set("X-Task-ID", fx.SourceTaskID)
	testutil.Call(t, testHandler.PostIssueWorkflowEvent, request).Want(http.StatusCreated)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, fx.SourceTaskID)
	var before string
	testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id=$1`, fx.IssueID).Scan(&before)
	body := operatorCompletionBody(reviewID, commentID)
	body["expected_plan_revision"] = 2
	testutil.Call(t, testHandler.PostIssueOperatorCompletion, withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/operator-completion", body), "id", fx.IssueID)).Want(http.StatusConflict)
	assertOperatorCompletionState(t, fx.IssueID, 0, "needs_owner", 1)
	var pending int
	var after string
	var ownerFlag bool
	if err := testPool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_owner_decision' AND NOT archived),i.status,p.requires_owner FROM issue i JOIN issue_workflow_plan p ON p.issue_id=i.id WHERE i.id=$1`, fx.IssueID).Scan(&pending, &after, &ownerFlag); err != nil {
		t.Fatal(err)
	}
	if pending != 1 || after != before || ownerFlag {
		t.Fatal("unanswered dynamic owner question was altered")
	}
}
func TestOperatorCompletionYieldsToExistingLegacyWriters(t *testing.T) {
	for _, kind := range []string{"native_event", "failure_projection", "owner_decision"} {
		t.Run(kind, func(t *testing.T) {
			fx, reviewID, commentID := prepareOperatorCompletionFixture(t)
			dbfx.Exec(t, `UPDATE agent_task_queue SET status='running',completed_at=NULL WHERE id=$1`, fx.SourceTaskID)
			gate := &contextLockTestStarter{pool: testPool, pauseAfter: "-- name: GetIssueWorkflowPlanForUpdate :one", paused: make(chan struct{}), release: make(chan struct{})}
			priorHandler, priorService := testHandler.TxStarter, testHandler.TaskService.TxStarter
			testHandler.TxStarter = gate
			testHandler.TaskService.TxStarter = gate
			defer func() { testHandler.TxStarter = priorHandler; testHandler.TaskService.TxStarter = priorService }()
			released := false
			defer func() {
				if !released {
					close(gate.release)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			done := make(chan error, 1)
			if kind == "failure_projection" {
				go func() {
					_, err := testHandler.TaskService.FailTask(ctx, parseUUID(fx.SourceTaskID), "Synthetic publication failure", "", "", "", "", false, "", "")
					done <- err
				}()
			} else {
				event := workflowstate.ImplementationReady
				request := issueWorkflowEventRequest{Event: event, Reason: "Concurrent native transition"}
				if kind == "owner_decision" {
					event = workflowstate.OwnerDecisionRequired
					request.Event = event
					raw, _ := json.Marshal(ownerDecisionPacket())
					var packet workflowOwnerDecisionPacket
					json.Unmarshal(raw, &packet)
					request.OwnerDecision = &packet
				} else {
					raw, _ := json.Marshal(validRepositoryInventoryEvidence())
					var packet workflowArtifactEvidencePacketV1
					json.Unmarshal(raw, &packet)
					request.ArtifactEvidence = &packet
				}
				issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(fx.IssueID))
				if err != nil {
					t.Fatal(err)
				}
				task, err := testHandler.Queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: parseUUID(fx.SourceTaskID), WorkspaceID: parseUUID(testWorkspaceID)})
				if err != nil {
					t.Fatal(err)
				}
				go func() {
					_, _, err := testHandler.applyIssueWorkflowEvent(workflowEventRequest(fx, event).WithContext(ctx), issue, task, request)
					done <- err
				}()
			}
			select {
			case <-gate.paused:
			case err := <-done:
				t.Fatalf("writer did not reach locked plan: %v", err)
			case <-ctx.Done():
				t.Fatal("writer lock not reached")
			}
			completion := make(chan int, 1)
			go func() {
				response := httptest.NewRecorder()
				testHandler.PostIssueOperatorCompletion(response, withURLParam(operatorCompletionHTTPRequest(fx, reviewID, commentID).WithContext(ctx), "id", fx.IssueID))
				completion <- response.Code
			}()
			select {
			case status := <-completion:
				if status != http.StatusConflict {
					t.Fatalf("contention returned %d", status)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("completion waited while holding issue lock")
			}
			close(gate.release)
			released = true
			select {
			case err := <-done:
				if err != nil {
					t.Fatal("native writer was disrupted", err)
				}
			case <-ctx.Done():
				t.Fatal("native writer could not finish after completion yielded")
			}
			var completions int
			if err := testPool.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_event WHERE issue_id=$1 AND event_type='operator_completed'`, fx.IssueID).Scan(&completions); err != nil || completions != 0 {
				t.Fatal("contended completion wrote an event", err)
			}
			plan, err := testHandler.Queries.GetIssueWorkflowPlanForUpdate(ctx, db.GetIssueWorkflowPlanForUpdateParams{IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID)})
			if err != nil {
				t.Fatal(err)
			}
			expected := map[string]string{"native_event": "technical_review", "failure_projection": "blocked", "owner_decision": "needs_owner"}[kind]
			if plan.CurrentState != expected {
				t.Fatalf("native writer plan=%s want %s", plan.CurrentState, expected)
			}
		})
	}
}
