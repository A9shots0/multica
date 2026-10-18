package workflowstore

import (
	"context"
	"os"
	"strings"
	"testing"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowcheckpoint"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestExecutionCheckpointSnapshotsCommitAtomically(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("integration test requires DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration database unavailable: %v", err)
	}

	fixture := testutil.New(pool, "", "")
	userID := fixture.User(t, "Checkpoint Test", "checkpoint-"+googleuuid.NewString()+"@multica.invalid")
	workspaceID := fixture.Workspace(t, "Checkpoint Test", "checkpoint-"+googleuuid.NewString())
	fixture.WorkspaceID, fixture.UserID = workspaceID, userID
	issueID := fixture.Issue(t, "verified checkpoint chain")
	fixture.Cleanup(t, `DELETE FROM issue_workflow_execution_checkpoint_snapshot WHERE workspace_id=$1`, workspaceID)
	fixture.Cleanup(t, `DELETE FROM issue_workflow_execution WHERE workspace_id=$1`, workspaceID)

	workspaceUUID, issueUUID := mustPGUUID(t, workspaceID), mustPGUUID(t, issueID)
	executionID, scopeID := pgUUID(googleuuid.New()), pgUUID(googleuuid.New())
	queries := db.New(pool)
	create := db.CreateIssueWorkflowExecutionParams{
		ID: executionID, WorkspaceID: workspaceUUID, IssueID: issueUUID,
		BranchKey: "main", OperationType: "implementation", State: "running",
		ScopeID: scopeID, ScopeRevision: 1, Checkpoint: []byte(`{"alphabet":1,"z":2}`),
		ExpectedEvent: []byte(`{}`), WaitReason: []byte(`{}`), AttemptCount: 1,
	}
	if err := InIssueWriteTransaction(ctx, pool, queries, workspaceUUID, issueUUID, func(write *IssueWrite) error {
		_, createErr := write.CreateExecution(ctx, create)
		return createErr
	}); err != nil {
		t.Fatal(err)
	}

	first, err := queries.GetLatestIssueWorkflowExecutionCheckpointSnapshot(ctx, db.GetLatestIssueWorkflowExecutionCheckpointSnapshotParams{
		WorkspaceID: workspaceUUID, IssueID: issueUUID, ExecutionID: executionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ExecutionRevision != 1 || !first.ChainAnchor || first.PreviousRevision.Valid || first.PreviousSnapshotSha256.Valid {
		t.Fatalf("first snapshot linkage = %+v", first)
	}
	if strings.Contains(string(first.SnapshotDocument), "lease_token") || strings.Contains(string(first.SnapshotDocument), "lease_expires_at") {
		t.Fatalf("snapshot leaked transient lease data: %s", first.SnapshotDocument)
	}

	update := db.UpdateIssueWorkflowExecutionCASParams{
		WorkspaceID: workspaceUUID, IssueID: issueUUID, ID: executionID, ExpectedRevision: 1,
		State: "waiting", Checkpoint: []byte(`{"goal":"finish","done":["audit"]}`),
		ExpectedEvent: []byte(`{"type":"resume"}`), WaitReason: []byte(`{"reason":"rotation"}`), AttemptCount: 1,
	}
	if err := InIssueWriteTransaction(ctx, pool, queries, workspaceUUID, issueUUID, func(write *IssueWrite) error {
		_, updateErr := write.UpdateExecution(ctx, update)
		return updateErr
	}); err != nil {
		t.Fatal(err)
	}
	second, err := queries.GetLatestIssueWorkflowExecutionCheckpointSnapshot(ctx, db.GetLatestIssueWorkflowExecutionCheckpointSnapshotParams{
		WorkspaceID: workspaceUUID, IssueID: issueUUID, ExecutionID: executionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ExecutionRevision != 2 || second.ChainAnchor || !second.PreviousRevision.Valid || second.PreviousRevision.Int64 != 1 || !second.PreviousSnapshotSha256.Valid || second.PreviousSnapshotSha256.String != first.SnapshotSha256 {
		t.Fatalf("second snapshot linkage = %+v, first digest %q", second, first.SnapshotSha256)
	}
	if err := workflowcheckpoint.Verify(second); err != nil {
		t.Fatalf("verify JSONB round trip: %v", err)
	}

	update.ExpectedRevision = 2
	update.Checkpoint = []byte(`{"value":"` + strings.Repeat("x", 1<<20) + `"}`)
	if err := InIssueWriteTransaction(ctx, pool, queries, workspaceUUID, issueUUID, func(write *IssueWrite) error {
		_, updateErr := write.UpdateExecution(ctx, update)
		return updateErr
	}); err == nil {
		t.Fatal("oversized checkpoint update committed")
	}
	current, err := queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{WorkspaceID: workspaceUUID, IssueID: issueUUID, ID: executionID})
	if err != nil {
		t.Fatal(err)
	}
	if current.Revision != 2 {
		t.Fatalf("execution revision after rejected snapshot = %d, want 2", current.Revision)
	}
	if got := fixture.Count(t, `SELECT count(*) FROM issue_workflow_execution_checkpoint_snapshot WHERE workspace_id=$1 AND issue_id=$2 AND execution_id=$3`, workspaceID, issueID, googleuuid.UUID(executionID.Bytes)); got != 2 {
		t.Fatalf("snapshot count = %d, want 2", got)
	}

	legacyID := googleuuid.NewString()
	fixture.Insert(t, "issue_workflow_execution", testutil.Cols{
		"id": legacyID, "workspace_id": workspaceID, "issue_id": issueID,
		"branch_key": "legacy", "operation_type": "implementation", "state": "running", "revision": 4,
		"scope_id": googleuuid.UUID(scopeID.Bytes), "scope_revision": 1,
		"checkpoint": testutil.Raw(`'{"goal":"legacy"}'::jsonb`), "expected_event": testutil.Raw(`'{}'::jsonb`), "wait_reason": testutil.Raw(`'{}'::jsonb`),
	})
	legacyUUID := mustPGUUID(t, legacyID)
	legacyUpdate := db.UpdateIssueWorkflowExecutionCASParams{
		WorkspaceID: workspaceUUID, IssueID: issueUUID, ID: legacyUUID, ExpectedRevision: 4,
		State: "waiting", Checkpoint: []byte(`{"goal":"legacy","resumed":true}`), ExpectedEvent: []byte(`{}`), WaitReason: []byte(`{}`),
	}
	if err := InIssueWriteTransaction(ctx, pool, queries, workspaceUUID, issueUUID, func(write *IssueWrite) error {
		_, updateErr := write.UpdateExecution(ctx, legacyUpdate)
		return updateErr
	}); err != nil {
		t.Fatal(err)
	}
	legacyAnchor, err := queries.GetIssueWorkflowExecutionCheckpointSnapshot(ctx, db.GetIssueWorkflowExecutionCheckpointSnapshotParams{
		WorkspaceID: workspaceUUID, IssueID: issueUUID, ExecutionID: legacyUUID, ExecutionRevision: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !legacyAnchor.ChainAnchor {
		t.Fatal("pre-ledger execution was not explicitly anchored at its first observed revision")
	}

	correctionID := googleuuid.NewString()
	fixture.Insert(t, "issue_workflow_execution", testutil.Cols{
		"id": correctionID, "workspace_id": workspaceID, "issue_id": issueID,
		"branch_key": "correction", "operation_type": "implementation", "state": "correction",
		"scope_id": googleuuid.UUID(scopeID.Bytes), "scope_revision": 1,
		"checkpoint": testutil.Raw(`'{"goal":"correct"}'::jsonb`), "expected_event": testutil.Raw(`'{}'::jsonb`), "wait_reason": testutil.Raw(`'{}'::jsonb`),
	})
	correctionUUID := mustPGUUID(t, correctionID)
	if err := InIssueWriteTransaction(ctx, pool, queries, workspaceUUID, issueUUID, func(write *IssueWrite) error {
		_, updateErr := write.UpdateCorrectionCandidate(ctx, db.UpdateIssueWorkflowExecutionCorrectionCandidateCASParams{
			WorkspaceID: workspaceUUID, IssueID: issueUUID, ID: correctionUUID, ExpectedRevision: 1,
			State: "working", CandidateID: pgUUID(googleuuid.New()), ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: pgUUID(googleuuid.New()),
		})
		return updateErr
	}); err != nil {
		t.Fatal(err)
	}
	if got := fixture.Count(t, `SELECT count(*) FROM issue_workflow_execution_checkpoint_snapshot WHERE workspace_id=$1 AND issue_id=$2 AND execution_id=$3`, workspaceID, issueID, correctionID); got != 2 {
		t.Fatalf("correction snapshot count = %d, want 2", got)
	}

	if err := queries.DeleteIssue(ctx, db.DeleteIssueParams{ID: issueUUID, WorkspaceID: workspaceUUID}); err != nil {
		t.Fatal(err)
	}
	if got := fixture.Count(t, `SELECT count(*) FROM issue_workflow_execution_checkpoint_snapshot WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID); got != 0 {
		t.Fatalf("issue deletion left %d checkpoint snapshots", got)
	}

	workspaceDeleteIssueID := fixture.Issue(t, "workspace snapshot cleanup")
	workspaceDeleteIssueUUID := mustPGUUID(t, workspaceDeleteIssueID)
	workspaceDeleteExecutionID := pgUUID(googleuuid.New())
	create.ID, create.IssueID = workspaceDeleteExecutionID, workspaceDeleteIssueUUID
	if err := InIssueWriteTransaction(ctx, pool, queries, workspaceUUID, workspaceDeleteIssueUUID, func(write *IssueWrite) error {
		_, createErr := write.CreateExecution(ctx, create)
		return createErr
	}); err != nil {
		t.Fatal(err)
	}
	if err := queries.DeleteWorkspaceIssueRoots(ctx, workspaceUUID); err != nil {
		t.Fatal(err)
	}
	if got := fixture.Count(t, `SELECT count(*) FROM issue_workflow_execution_checkpoint_snapshot WHERE workspace_id=$1`, workspaceID); got != 0 {
		t.Fatalf("workspace issue-root deletion left %d checkpoint snapshots", got)
	}
}

func mustPGUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	parsed, err := googleuuid.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return pgUUID(parsed)
}

func pgUUID(value googleuuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}
