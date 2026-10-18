package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestClaimTaskByRuntimeAttachesCurrentPictureContinuityLedger(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Workflow continuity claim runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Workflow continuity claim agent")
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	document, err := json.Marshal(map[string]any{
		"contract_version": 2,
		"original_business_request": map[string]any{
			"available": true, "id": "owner-request-1", "payload": map[string]any{"goal": "ship the usable workflow"},
		},
		"outcome": map[string]any{"state": "open", "acceptance_version": 2},
		"scope": map[string]any{
			"acceptance_version": 2, "statement": map[string]any{"goal": "ship the usable workflow"},
		},
		"unresolved_dependencies": []map[string]any{{
			"id": "execution-1", "revision": 4, "state": "blocked_external",
			"actor_type": "external_provider", "actor_id": "provider-actor-1",
			"wait_reason":    map[string]any{"code": "external_receipt"},
			"expected_event": map[string]any{"type": "provider_readback", "after_receipt_revision": 4},
			"checkpoint":     map[string]any{"provider": "railway", "next_action": "wait for the experiment owner"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `INSERT INTO issue_current_picture (
		workspace_id, issue_id, contract_version, revision, outcome_state, outcome_open,
		execution_state, release_state, health_state, attention_state, source_revisions,
		document, generated_at
	) VALUES ($1,$2,2,12,'open',true,'blocked_external','pending','unknown','resolved','[]',$3,now())`,
		testWorkspaceID, issueID, document)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_current_picture WHERE issue_id=$1`, issueID)
	})

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "workflow-continuity-claim")
	req = withURLParam(req, "runtimeId", runtimeID)
	var response struct {
		Task *struct {
			ID                 string          `json:"id"`
			WorkflowContinuity json.RawMessage `json:"workflow_continuity"`
		} `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&response)
	if response.Task == nil || response.Task.ID != taskID {
		t.Fatalf("claimed task = %#v, want %s", response.Task, taskID)
	}
	var ledger struct {
		CurrentPictureRevision int64 `json:"current_picture_revision"`
		OriginalGoal           struct {
			AcceptanceVersion int64 `json:"acceptance_version"`
			AcceptedScope     struct {
				Text struct {
					Goal string `json:"goal"`
				} `json:"text"`
			} `json:"accepted_scope"`
		} `json:"original_goal"`
		Acceptance struct {
			Accepted                   bool `json:"accepted"`
			RequiresExternalAcceptance bool `json:"requires_external_acceptance"`
		} `json:"acceptance"`
		Blockers []struct {
			Fingerprint string `json:"fingerprint"`
			Owner       struct {
				System string `json:"system"`
			} `json:"owner"`
			ResumeCondition struct {
				Type string `json:"type"`
			} `json:"resume_condition"`
		} `json:"blockers"`
	}
	if err := json.Unmarshal(response.Task.WorkflowContinuity, &ledger); err != nil {
		t.Fatalf("continuity ledger = %s: %v", response.Task.WorkflowContinuity, err)
	}
	if ledger.CurrentPictureRevision != 12 || ledger.OriginalGoal.AcceptanceVersion != 2 ||
		ledger.OriginalGoal.AcceptedScope.Text.Goal != "ship the usable workflow" ||
		ledger.Acceptance.Accepted || !ledger.Acceptance.RequiresExternalAcceptance || len(ledger.Blockers) != 1 {
		t.Fatalf("continuity ledger = %#v", ledger)
	}
	if ledger.Blockers[0].Fingerprint == "" || ledger.Blockers[0].Owner.System != "railway" ||
		ledger.Blockers[0].ResumeCondition.Type != "current_picture_blocker_cleared" {
		t.Fatalf("canonical blocker = %#v", ledger.Blockers[0])
	}
}

func TestClaimTaskByRuntimeDoesNotGateOnInvalidContinuityProjection(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Invalid continuity claim runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Invalid continuity claim agent")
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	dbfx.Exec(t, `INSERT INTO issue_current_picture (
		workspace_id, issue_id, contract_version, revision, outcome_state, outcome_open,
		execution_state, release_state, health_state, attention_state, source_revisions,
		document, generated_at
	) VALUES ($1,$2,2,12,'open',true,'received','pending','unknown','resolved','[]','{"contract_version":99}',now())`,
		testWorkspaceID, issueID)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_current_picture WHERE issue_id=$1`, issueID)
	})

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "invalid-workflow-continuity-claim")
	req = withURLParam(req, "runtimeId", runtimeID)
	var response struct {
		Task *struct {
			ID                 string          `json:"id"`
			WorkflowContinuity json.RawMessage `json:"workflow_continuity"`
		} `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&response)
	if response.Task == nil || response.Task.ID != taskID {
		t.Fatalf("claimed task = %#v, want %s", response.Task, taskID)
	}
	if len(response.Task.WorkflowContinuity) != 0 {
		t.Fatalf("invalid continuity projection leaked into claim: %s", response.Task.WorkflowContinuity)
	}
}

func TestCurrentIssueOwnerAcceptanceDoesNotSurviveOutcomeReopenOrResatisfaction(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Workflow acceptance binding runtime")
	_, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Workflow acceptance binding agent")
	var outcomeID string
	dbfx.QueryRow(t, `
		INSERT INTO issue_workflow_outcome (
			id, workspace_id, issue_id, revision, state, acceptance_version,
			completion_evidence_id, terminal_reason, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, 1, 'satisfied', 1,
			gen_random_uuid(), 'initial delivery', now() - interval '3 minutes', now() - interval '2 minutes'
		) RETURNING id
	`, testWorkspaceID, issueID).Scan(&outcomeID)
	insertDirective := func(kind, state, age string) string {
		t.Helper()
		var id string
		dbfx.QueryRow(t, `
			INSERT INTO issue_owner_directive (
				workspace_id, issue_id, directive_type, expected_issue_revision,
				content, state, actor_id, created_at
			) VALUES ($1, $2, $3, 1, 'owner response', $4, $5, now() + ($6::interval))
			RETURNING id
		`, testWorkspaceID, issueID, kind, state, testUserID, age).Scan(&id)
		return id
	}
	firstReceipt := insertDirective("accept_result", "applied", "-1 minute")
	params := db.GetCurrentIssueOwnerAcceptanceParams{WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID)}
	accepted, err := testHandler.Queries.GetCurrentIssueOwnerAcceptance(ctx, params)
	if err != nil || uuidToString(accepted.ReceiptID) != firstReceipt || uuidToString(accepted.OutcomeID) != outcomeID || accepted.OutcomeRevision != 1 {
		t.Fatalf("initial acceptance binding = %#v, err=%v", accepted, err)
	}

	dbfx.Exec(t, `
		UPDATE issue_workflow_outcome
		SET revision=2, state='open', completion_evidence_id=NULL, terminal_reason='regression', updated_at=now()
		WHERE workspace_id=$1 AND issue_id=$2 AND id=$3
	`, testWorkspaceID, issueID, outcomeID)
	if _, err := testHandler.Queries.GetCurrentIssueOwnerAcceptance(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reopened outcome retained old acceptance: %v", err)
	}

	dbfx.Exec(t, `
		UPDATE issue_workflow_outcome
		SET revision=3, state='satisfied', completion_evidence_id=gen_random_uuid(), terminal_reason='redelivered', updated_at=now()
		WHERE workspace_id=$1 AND issue_id=$2 AND id=$3
	`, testWorkspaceID, issueID, outcomeID)
	if _, err := testHandler.Queries.GetCurrentIssueOwnerAcceptance(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("resatisfied outcome resurrected old acceptance: %v", err)
	}

	secondReceipt := insertDirective("accept_result", "applied", "1 minute")
	accepted, err = testHandler.Queries.GetCurrentIssueOwnerAcceptance(ctx, params)
	if err != nil || uuidToString(accepted.ReceiptID) != secondReceipt || accepted.OutcomeRevision != 3 {
		t.Fatalf("fresh acceptance binding = %#v, err=%v", accepted, err)
	}
	insertDirective("correct", "recorded", "2 minutes")
	if _, err := testHandler.Queries.GetCurrentIssueOwnerAcceptance(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("later correction retained old acceptance: %v", err)
	}
}
