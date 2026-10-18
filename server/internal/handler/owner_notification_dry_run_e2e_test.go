package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/ownernotification"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestOwnerNotificationProjectionAdmissionDryRunAndSemanticDeduplication(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	workspaceID := parseUUID(testWorkspaceID)
	recipientID := parseUUID(testUserID)
	issueID := parseUUID(dbfx.Issue(t, "provider-free notification shadow"))
	destinationKey := "item14-provider-free-destination"
	dbfx.Cleanup(t, `DELETE FROM owner_notification_attempt WHERE notification_id IN (SELECT id FROM owner_notification WHERE issue_id=$1)`, issueID)
	dbfx.Cleanup(t, `DELETE FROM owner_notification WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM owner_notification_binding WHERE workspace_id=$1`, workspaceID)
	if _, err := testHandler.Queries.UpsertOwnerNotificationBinding(ctx, db.UpsertOwnerNotificationBindingParams{
		WorkspaceID: workspaceID, RecipientID: recipientID, DestinationKey: destinationKey,
		DestinationFingerprint: "item14-provider-free", DeliveryMode: "dry_run", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC)
	document, err := json.Marshal(map[string]any{
		"contract_version": 2, "owner_summary_schema_version": 1,
		"outcome":   map[string]any{"id": "outcome-1", "revision": 1, "state": "open"},
		"scope":     map[string]any{"id": "scope-1", "revision": 1, "requires_deployment": false},
		"execution": map[string]any{"id": "execution-1", "revision": 1, "state": "paused"},
		"release":   map[string]any{"id": "release-1", "revision": 1, "state": "not_required"},
		"health":    "unverified", "attention": map[string]any{},
		"unresolved_decisions": []map[string]any{{"id": "decision-1", "revision": 1}},
		"unresolved_incidents": []map[string]any{}, "unresolved_dependencies": []map[string]any{},
		"next_automatic_action": map[string]any{}, "active_actor": map[string]any{}, "latest_meaningful_change": map[string]any{},
		"terminal_reason": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	picture := db.IssueCurrentPicture{
		WorkspaceID: workspaceID, IssueID: issueID, ContractVersion: 2, Revision: 1,
		Document: document, GeneratedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}

	admit := func() int {
		count := 0
		err := workflowstore.InIssueWriteTransaction(ctx, testPool, testHandler.Queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
			rows, err := ownernotification.AdmitProjectionInWrite(ctx, write, workspaceID, issueID, picture, "https://item14.invalid")
			count = len(rows)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return count
	}
	if got := admit(); got != 1 {
		t.Fatalf("first admission count = %d, want 1", got)
	}
	if got := admit(); got != 0 {
		t.Fatalf("semantic replay admission count = %d, want 0", got)
	}
	rows, err := testHandler.Queries.ListOwnerNotificationsByIssue(ctx, db.ListOwnerNotificationsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil || len(rows) != 1 || rows[0].State != "admitted" {
		t.Fatalf("admitted rows = %#v, error=%v", rows, err)
	}

	worker := ownernotification.Worker{
		Queries: testHandler.Queries, Mode: "dry_run", DestinationKey: destinationKey,
		Now: func() time.Time { return now },
	}
	if err := worker.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err = testHandler.Queries.ListOwnerNotificationsByIssue(ctx, db.ListOwnerNotificationsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil || len(rows) != 1 || rows[0].State != "delivered" || rows[0].ProviderState.String != "dry_run_delivered" || rows[0].AttemptCount != 1 {
		t.Fatalf("dry-run delivery rows = %#v, error=%v", rows, err)
	}
	attempts, err := testHandler.Queries.ListOwnerNotificationAttempts(ctx, rows[0].ID)
	if err != nil || len(attempts) != 2 || attempts[0].State != "started" || attempts[1].State != "dry_run_delivered" {
		t.Fatalf("dry-run attempts = %#v, error=%v", attempts, err)
	}
	if err := worker.ProcessOne(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("duplicate dry-run process error = %v, want no rows", err)
	}
	again, err := testHandler.Queries.ListOwnerNotificationAttempts(ctx, rows[0].ID)
	if err != nil || len(again) != 2 {
		t.Fatalf("duplicate dry-run attempts = %#v, error=%v", again, err)
	}
}
