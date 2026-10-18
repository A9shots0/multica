package workflowcheckpoint

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestBuildIsDeterministicAndLinksExactPredecessor(t *testing.T) {
	execution := testExecution()
	first, err := Build(execution, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ChainAnchor || first.PreviousRevision.Valid || first.PreviousSnapshotSHA256.Valid {
		t.Fatalf("first snapshot linkage = %#v, want anchor", first)
	}
	if len(first.Digest) != 64 {
		t.Fatalf("digest length = %d, want 64", len(first.Digest))
	}

	// JSONB may return semantically identical objects with different spacing
	// or key order. The snapshot must still hash to exactly the same bytes.
	execution.Checkpoint = []byte(`{ "alpha": 1, "goal": "ship" }`)
	again, err := Build(execution, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != again.Digest || !bytes.Equal(first.Document, again.Document) {
		t.Fatal("semantically identical execution produced a different snapshot")
	}

	predecessor := db.IssueWorkflowExecutionCheckpointSnapshot{
		WorkspaceID: execution.WorkspaceID, IssueID: execution.IssueID, ExecutionID: execution.ID,
		ExecutionRevision: execution.Revision, SnapshotSha256: first.Digest,
	}
	execution.Revision++
	execution.State = "waiting"
	execution.UpdatedAt = pgtype.Timestamptz{Time: execution.UpdatedAt.Time.Add(time.Minute), Valid: true}
	linked, err := Build(execution, &predecessor)
	if err != nil {
		t.Fatal(err)
	}
	if linked.ChainAnchor || !linked.PreviousRevision.Valid || linked.PreviousRevision.Int64 != 1 {
		t.Fatalf("linked previous revision = %#v", linked.PreviousRevision)
	}
	if !linked.PreviousSnapshotSHA256.Valid || linked.PreviousSnapshotSHA256.String != first.Digest {
		t.Fatalf("linked previous digest = %#v", linked.PreviousSnapshotSHA256)
	}
	var decoded map[string]any
	if err := json.Unmarshal(linked.Document, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["previous_snapshot_sha256"] != first.Digest || decoded["chain_anchor"] != false {
		t.Fatalf("document linkage = %#v", decoded)
	}
	if _, exists := decoded["lease_token"]; exists {
		t.Fatal("snapshot document persisted transient lease token")
	}
	if _, exists := decoded["lease_expires_at"]; exists {
		t.Fatal("snapshot document persisted transient lease expiry")
	}
}

func TestBuildRejectsInvalidExecutionAndPredecessor(t *testing.T) {
	tests := []struct {
		name string
		edit func(*db.IssueWorkflowExecution)
	}{
		{name: "checkpoint is not object", edit: func(row *db.IssueWorkflowExecution) { row.Checkpoint = []byte(`[]`) }},
		{name: "trailing json", edit: func(row *db.IssueWorkflowExecution) { row.ExpectedEvent = []byte(`{} {}`) }},
		{name: "authority pair incomplete", edit: func(row *db.IssueWorkflowExecution) { row.AuthorityRevision.Valid = false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execution := testExecution()
			tt.edit(&execution)
			if _, err := Build(execution, nil); err == nil {
				t.Fatal("Build succeeded, want error")
			}
		})
	}

	oversized := testExecution()
	oversized.Checkpoint = append([]byte(`{"value":"`), bytes.Repeat([]byte("x"), maxStructuredFieldBytes)...)
	oversized.Checkpoint = append(oversized.Checkpoint, []byte(`"}`)...)
	if _, err := Build(oversized, nil); err == nil {
		t.Fatal("Build accepted oversized checkpoint")
	}

	execution := testExecution()
	predecessor := db.IssueWorkflowExecutionCheckpointSnapshot{
		WorkspaceID: execution.WorkspaceID, IssueID: execution.IssueID, ExecutionID: execution.ID,
		ExecutionRevision: execution.Revision - 1, SnapshotSha256: "short",
	}
	if _, err := Build(execution, &predecessor); err == nil {
		t.Fatal("Build accepted malformed predecessor digest")
	}
}

func TestVerifyRejectsTamperedSnapshot(t *testing.T) {
	execution := testExecution()
	built, err := Build(execution, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := db.IssueWorkflowExecutionCheckpointSnapshot{
		WorkspaceID: execution.WorkspaceID, IssueID: execution.IssueID,
		ExecutionID: execution.ID, ExecutionRevision: execution.Revision,
		TaskID: execution.TaskID, ActorID: execution.ActorID,
		ScopeID: execution.ScopeID, ScopeRevision: execution.ScopeRevision,
		AuthorityID: execution.AuthorityID, AuthorityRevision: execution.AuthorityRevision,
		BranchKey: execution.BranchKey, ExecutionState: execution.State,
		Checkpoint: built.Checkpoint, SnapshotDocument: built.Document,
		ChainAnchor: built.ChainAnchor, SnapshotSha256: built.Digest,
	}
	if err := Verify(snapshot); err != nil {
		t.Fatalf("Verify(valid) = %v", err)
	}

	tests := []struct {
		name string
		edit func(*db.IssueWorkflowExecutionCheckpointSnapshot)
	}{
		{name: "digest", edit: func(row *db.IssueWorkflowExecutionCheckpointSnapshot) { row.SnapshotSha256 = strings.Repeat("0", 64) }},
		{name: "row identity", edit: func(row *db.IssueWorkflowExecutionCheckpointSnapshot) { row.ExecutionRevision++ }},
		{name: "checkpoint column", edit: func(row *db.IssueWorkflowExecutionCheckpointSnapshot) { row.Checkpoint = []byte(`{"other":true}`) }},
		{name: "unknown document field", edit: func(row *db.IssueWorkflowExecutionCheckpointSnapshot) {
			row.SnapshotDocument = append(bytes.TrimSuffix(row.SnapshotDocument, []byte("}")), []byte(`,"unknown":true}`)...)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := snapshot
			changed.Checkpoint = append([]byte(nil), snapshot.Checkpoint...)
			changed.SnapshotDocument = append([]byte(nil), snapshot.SnapshotDocument...)
			tt.edit(&changed)
			if err := Verify(changed); err == nil {
				t.Fatal("Verify accepted tampered snapshot")
			}
		})
	}
}

func testExecution() db.IssueWorkflowExecution {
	now := time.Date(2026, 10, 17, 12, 0, 0, 123, time.FixedZone("EDT", -4*60*60))
	return db.IssueWorkflowExecution{
		ID: testUUID(1), WorkspaceID: testUUID(2), IssueID: testUUID(3),
		BranchKey: "implementation", OperationType: "execute", State: "running", Revision: 1,
		ScopeID: testUUID(4), ScopeRevision: 2, AuthorityID: testUUID(5), AuthorityRevision: pgtype.Int8{Int64: 3, Valid: true},
		CandidateID: testUUID(6), TargetKey: pgtype.Text{String: "backend", Valid: true},
		ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: testUUID(7), TaskID: testUUID(8),
		Checkpoint: []byte(`{"goal":"ship","alpha":1}`), ExpectedEvent: []byte(`{"type":"worker_result"}`), WaitReason: []byte(`{}`),
		LeaseToken: testUUID(9), LeaseExpiresAt: pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
		NextActionAt: pgtype.Timestamptz{Time: now.Add(2 * time.Minute), Valid: true}, AttemptCount: 1,
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
}

func testUUID(last byte) pgtype.UUID {
	var value [16]byte
	value[15] = last
	return pgtype.UUID{Bytes: value, Valid: true}
}
