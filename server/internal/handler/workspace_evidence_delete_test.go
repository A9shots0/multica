package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/developmentmemory"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeleteWorkspaceSettlesPendingEvidenceAndPreservesHistory(t *testing.T) {
	ws := dbfx.Insert(t, "workspace", testutil.Cols{"name": "Evidence deletion", "slug": "ci-evidence-deletion"})
	dbfx.Exec(t, `INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'owner')`, ws, testUserID)
	for _, id := range []string{ws, testWorkspaceID} {
		dbfx.Exec(t, `INSERT INTO brain_event_outbox(event_id,workspace_id,project_id,payload,delivered_at)
   VALUES($1,$2,'ci-delete-proof','{}',NULL),($3,$2,'ci-delete-proof','{}',now())`, "pending-"+id, id, "delivered-"+id)
		dbfx.Exec(t, `INSERT INTO development_memory_proposal(id,workspace_id,issue_id,task_id,workstream_id,agent_name,task_started_at,task_completed_at,task_status,binding,candidate,source_output_sha256,lease_token,delivered_at)
   SELECT gen_random_uuid(),$1,gen_random_uuid(),gen_random_uuid(),'ci-delete-proof','fixture',now(),now(),'complete','{}','{}','fixture',gen_random_uuid(),delivered
   FROM (VALUES(NULL::timestamptz),(now())) AS v(delivered)`, id)
		dbfx.Exec(t, `INSERT INTO ai_failure_investigation(id,workspace_id,issue_id,trigger_kind,trigger_id,summary,status,research_scope,findings)
   SELECT gen_random_uuid(),$1,gen_random_uuid(),'task_failed',gen_random_uuid(),'ci-delete-proof',status,'[]',findings
   FROM (VALUES('queued',NULL::jsonb),('completed','{}'::jsonb)) AS v(status,findings)`, id)
		dbfx.Exec(t, `INSERT INTO owner_review_feedback(id,workspace_id,user_id,issue_id,source_attachment_id,source_filename,source_content_type,marked_screenshot_attachment_id,marks,note)
   VALUES(gen_random_uuid(),$1,$2,gen_random_uuid(),gen_random_uuid(),'fixture.png','image/png',gen_random_uuid(),'[]','ci-delete-proof')`, id, testUserID)
		dbfx.Exec(t, `INSERT INTO brain_task_link(workspace_id,task_id,attempt,project_id) VALUES($1,gen_random_uuid(),1,'ci-delete-proof')`, id)
		dbfx.Exec(t, `INSERT INTO chat_library(workspace_id,owner_id) VALUES($1,$2)`, id, testUserID)
	}
	t.Cleanup(func() {
		for _, table := range []string{"brain_event_outbox", "development_memory_proposal", "ai_failure_investigation", "owner_review_feedback", "brain_task_link", "chat_library"} {
			_, err := testPool.Exec(context.Background(), `DELETE FROM `+table+` WHERE workspace_id=ANY($1::uuid[])`, []string{ws, testWorkspaceID})
			if err != nil {
				t.Errorf("cleanup %s: %v", table, err)
			}
		}
	})
	req := withURLParam(newRequest("DELETE", "/api/workspaces/"+ws, nil), "id", ws)
	testutil.Call(t, testHandler.DeleteWorkspace, req).Want(http.StatusNoContent)
	// A worker may already have fetched this event before deletion. Its late
	// transient-failure result must never reopen the deletion hold.
	queries := db.New(testPool)
	ctx := context.Background()
	if err := queries.MarkBrainEventFailure(ctx, db.MarkBrainEventFailureParams{EventID: "pending-" + ws, LastError: pgtype.Text{String: "late transient failure", Valid: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.EnqueueBrainHandoff(ctx, db.EnqueueBrainHandoffParams{EventID: "late-" + ws, WorkspaceID: parseUUID(ws), ProjectID: "ci-delete-proof", Payload: []byte(`{}`)}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("late handoff after deletion: %v", err)
	}
	pending, err := queries.ListPendingBrainEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range pending {
		if event.EventID == "pending-"+ws {
			t.Fatal("deleted workspace returned for delivery")
		}
	}
	store := developmentmemory.NewStore(testPool)
	var proposalID, issueID string
	dbfx.QueryRow(t, `SELECT id::text,issue_id::text FROM development_memory_proposal WHERE workspace_id=$1 AND delivered_at IS NULL`, ws).Scan(&proposalID, &issueID)
	if _, err := store.Approve(ctx, ws, issueID, proposalID, testUserID, developmentmemory.ProposalDraft{}, "late approval"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("late approval after deletion: %v", err)
	}
	dbfx.Exec(t, `UPDATE development_memory_proposal SET review_status='approved',approved_payload='{}',reviewed_at=now() WHERE id=$1`, proposalID)
	if _, err := store.RetryDelivery(ctx, ws, issueID, proposalID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("late retry after deletion: %v", err)
	}
	// Even a stale writer that clears the hold cannot make an orphan claimable.
	dbfx.Exec(t, `UPDATE development_memory_proposal SET dead_lettered_at=NULL,next_attempt_at=now() WHERE id=$1`, proposalID)
	if delivery, err := store.ClaimNext(ctx, time.Minute); err == nil && delivery.ProposalID == proposalID {
		t.Fatal("deleted workspace proposal claimed")
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE development_memory_proposal SET dead_lettered_at=now() WHERE id=$1`, proposalID)
	for _, id := range []string{ws, testWorkspaceID} {
		deleted := id == ws
		var blocked bool
		dbfx.QueryRow(t, `SELECT is_blocked FROM brain_event_outbox WHERE event_id=$1`, "pending-"+id).Scan(&blocked)
		if blocked != deleted {
			t.Fatalf("pending delivery workspace=%s blocked=%v", id, blocked)
		}
		dbfx.QueryRow(t, `SELECT is_blocked FROM brain_event_outbox WHERE event_id=$1`, "delivered-"+id).Scan(&blocked)
		if blocked {
			t.Fatal("delivered brain history changed")
		}
		var dead, leaseGone, unchanged bool
		dbfx.QueryRow(t, `SELECT dead_lettered_at IS NOT NULL,lease_token IS NULL FROM development_memory_proposal WHERE workspace_id=$1 AND workstream_id='ci-delete-proof' AND delivered_at IS NULL`, id).Scan(&dead, &leaseGone)
		if dead != deleted || leaseGone != deleted {
			t.Fatalf("pending memory workspace=%s dead=%v leaseGone=%v", id, dead, leaseGone)
		}
		dbfx.QueryRow(t, `SELECT dead_lettered_at IS NULL AND lease_token IS NOT NULL FROM development_memory_proposal WHERE workspace_id=$1 AND workstream_id='ci-delete-proof' AND delivered_at IS NOT NULL`, id).Scan(&unchanged)
		if !unchanged {
			t.Fatal("delivered memory history changed")
		}
		var status string
		dbfx.QueryRow(t, `SELECT status FROM ai_failure_investigation WHERE workspace_id=$1 AND summary='ci-delete-proof' AND findings IS NULL`, id).Scan(&status)
		want := "queued"
		if deleted {
			want = "investigator_failed"
		}
		if status != want {
			t.Fatalf("investigation status=%s want=%s", status, want)
		}
		var retained int
		dbfx.QueryRow(t, `SELECT count(*) FROM ai_failure_investigation WHERE workspace_id=$1 AND summary='ci-delete-proof' AND status='completed' AND findings='{}'`, id).Scan(&retained)
		if retained != 1 {
			t.Fatal("completed investigation history changed")
		}
		dbfx.QueryRow(t, `SELECT count(*) FROM owner_review_feedback WHERE workspace_id=$1 AND note='ci-delete-proof'`, id).Scan(&retained)
		if retained != 1 {
			t.Fatal("owner feedback history changed")
		}
		for _, table := range []string{"brain_task_link", "chat_library"} {
			var remaining int
			dbfx.QueryRow(t, `SELECT count(*) FROM `+table+` WHERE workspace_id=$1`, id).Scan(&remaining)
			want := 1
			if deleted {
				want = 0
			}
			if remaining != want {
				t.Fatalf("%s workspace=%s rows=%d want=%d", table, id, remaining, want)
			}
		}
	}
}
