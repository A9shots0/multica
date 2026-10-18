package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func legacyProjectionAdmissionFixture(t *testing.T) db.UpsertIssueCurrentPictureParams {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx := newWorkflowControlFixture(t)
	params, err := workflowprojection.LegacyTechnicalStopParams(parseUUID(testWorkspaceID), parseUUID(fx.IssueID), parseUUID(fx.SourceTaskID), "admission regression", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_current_picture WHERE issue_id=$1`, fx.IssueID)
	})
	return params
}

func TestLegacyProjectionAdmissionPreservesCallerTransaction(t *testing.T) {
	params := legacyProjectionAdmissionFixture(t)
	ctx := context.Background()
	lookup := db.GetIssueCurrentPictureParams{WorkspaceID: params.WorkspaceID, IssueID: params.IssueID}
	for _, commit := range []bool{false, true} {
		tx, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := workflowstore.UpsertLegacyCurrentPicture(ctx, tx, params); err != nil {
			t.Fatal(err)
		}
		if _, err := db.New(tx).GetIssueCurrentPicture(ctx, lookup); err != nil {
			t.Fatalf("projection missing inside caller transaction: %v", err)
		}
		if _, err := testHandler.Queries.GetIssueCurrentPicture(ctx, lookup); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("projection committed ahead of caller: %v", err)
		}
		if commit {
			err = tx.Commit(ctx)
		} else {
			err = tx.Rollback(ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		_, err = testHandler.Queries.GetIssueCurrentPicture(ctx, lookup)
		if (commit && err != nil) || (!commit && !errors.Is(err, pgx.ErrNoRows)) {
			t.Fatalf("caller commit=%v, readback=%v", commit, err)
		}
	}
}

func TestLegacyProjectionAdmissionRejectsMissingOrMismatchedOwner(t *testing.T) {
	params := legacyProjectionAdmissionFixture(t)
	ctx := context.Background()
	if _, err := workflowstore.UpsertLegacyCurrentPicture(ctx, nil, params); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
		t.Fatalf("nil transaction accepted: %v", err)
	}
	for _, missingWorkspace := range []bool{true, false} {
		tx, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		wrongScope := params
		if missingWorkspace {
			wrongScope.WorkspaceID = dbid.NewV7()
		} else {
			// An existing issue in this workspace cannot be addressed through
			// another tenant, even when both owner records exist.
			otherWorkspace := dbfx.Workspace(t, "Projection admission", "projection-"+uuidToString(dbid.NewV7()))
			wrongScope.WorkspaceID = parseUUID(otherWorkspace)
		}
		if _, err := workflowstore.UpsertLegacyCurrentPicture(ctx, tx, wrongScope); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			t.Fatalf("invalid owner accepted: %v", err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyProjectionAdmissionDoesNotWaitBehindArchival(t *testing.T) {
	for _, owner := range []string{"workspace", "issue"} {
		t.Run(owner, func(t *testing.T) {
			params := legacyProjectionAdmissionFixture(t)
			ctx := context.Background()
			archive, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Rollback(ctx)
			query, id := `SELECT id FROM workspace WHERE id=$1 FOR UPDATE`, params.WorkspaceID
			if owner == "issue" {
				query, id = `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, params.IssueID
			}
			if _, err := archive.Exec(ctx, query, id); err != nil {
				t.Fatal(err)
			}
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if owner == "workspace" {
				// Simulate the older controller having changed an issue before
				// admission. Waiting for workspace here would invert lock order.
				if _, err := tx.Exec(ctx, `UPDATE issue SET title='must roll back' WHERE id=$1`, params.IssueID); err != nil {
					t.Fatal(err)
				}
			}
			bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_, err = workflowstore.UpsertLegacyCurrentPicture(bounded, tx, params)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("expected immediate lock conflict, got %v", err)
			}
			if err := tx.Commit(ctx); !errors.Is(err, pgx.ErrTxCommitRollback) {
				t.Fatalf("failed admission could commit legacy writes: %v", err)
			}
			var title string
			if err := testPool.QueryRow(ctx, `SELECT title FROM issue WHERE id=$1`, params.IssueID).Scan(&title); err != nil || title == "must roll back" {
				t.Fatalf("legacy write survived conflict: title=%q err=%v", title, err)
			}
		})
	}
}

func TestLegacyProjectionAdmissionHoldsOwnerLocksUntilCallerFinishes(t *testing.T) {
	params := legacyProjectionAdmissionFixture(t)
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := workflowstore.UpsertLegacyCurrentPicture(ctx, tx, params); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"workspace", "issue"} {
		archive, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer archive.Rollback(ctx)
		query, id := `SELECT id FROM workspace WHERE id=$1 FOR UPDATE NOWAIT`, params.WorkspaceID
		if owner == "issue" {
			query, id = `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, params.IssueID
		}
		_, err = archive.Exec(ctx, query, id)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
			t.Fatalf("%s lock escaped transaction: %v", owner, err)
		}
		_ = archive.Rollback(ctx)
	}
}
