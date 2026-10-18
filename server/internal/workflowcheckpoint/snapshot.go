// Package workflowcheckpoint builds and verifies the immutable, hash-linked
// representation of a workflow execution revision.
package workflowcheckpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const ContractVersion = 1

const maxStructuredFieldBytes = 1 << 20

type document struct {
	ContractVersion        int             `json:"contract_version"`
	WorkspaceID            string          `json:"workspace_id"`
	IssueID                string          `json:"issue_id"`
	ExecutionID            string          `json:"execution_id"`
	ExecutionRevision      int64           `json:"execution_revision"`
	BranchKey              string          `json:"branch_key"`
	OperationType          string          `json:"operation_type"`
	State                  string          `json:"state"`
	ScopeID                string          `json:"scope_id"`
	ScopeRevision          int64           `json:"scope_revision"`
	AuthorityID            *string         `json:"authority_id"`
	AuthorityRevision      *int64          `json:"authority_revision"`
	CandidateID            *string         `json:"candidate_id"`
	TargetKey              *string         `json:"target_key"`
	ActorType              *string         `json:"actor_type"`
	ActorID                *string         `json:"actor_id"`
	TaskID                 *string         `json:"task_id"`
	Checkpoint             json.RawMessage `json:"checkpoint"`
	ExpectedEvent          json.RawMessage `json:"expected_event"`
	WaitReason             json.RawMessage `json:"wait_reason"`
	NextActionAt           *string         `json:"next_action_at"`
	AttemptCount           int32           `json:"attempt_count"`
	TerminalReason         string          `json:"terminal_reason"`
	SupersededBy           *string         `json:"superseded_by"`
	SupersededAt           *string         `json:"superseded_at"`
	CreatedAt              string          `json:"created_at"`
	UpdatedAt              string          `json:"updated_at"`
	PreviousRevision       *int64          `json:"previous_revision"`
	PreviousSnapshotSHA256 *string         `json:"previous_snapshot_sha256"`
	ChainAnchor            bool            `json:"chain_anchor"`
}

// Built is the canonical snapshot document and its lowercase SHA-256 digest.
type Built struct {
	Document               []byte
	Digest                 string
	Checkpoint             []byte
	PreviousRevision       pgtype.Int8
	PreviousSnapshotSHA256 pgtype.Text
	ChainAnchor            bool
}

// Verify checks a snapshot after reading it from the database. JSONB does not
// preserve object key order, so verification strictly decodes the document,
// re-encodes its canonical struct representation, and hashes those bytes.
func Verify(snapshot db.IssueWorkflowExecutionCheckpointSnapshot) error {
	if !snapshot.WorkspaceID.Valid || !snapshot.IssueID.Valid || !snapshot.ExecutionID.Valid || !snapshot.ScopeID.Valid || snapshot.ExecutionRevision <= 0 || snapshot.ScopeRevision <= 0 {
		return errors.New("workflow checkpoint: snapshot row identity is incomplete")
	}
	decoder := json.NewDecoder(bytes.NewReader(snapshot.SnapshotDocument))
	decoder.DisallowUnknownFields()
	var doc document
	if err := decoder.Decode(&doc); err != nil {
		return fmt.Errorf("workflow checkpoint: decode snapshot document: %w", err)
	}
	if err := expectEOF(decoder); err != nil {
		return fmt.Errorf("workflow checkpoint: decode snapshot document: %w", err)
	}
	documentCheckpoint, err := canonicalObject("document checkpoint", doc.Checkpoint)
	if err != nil {
		return err
	}
	documentExpectedEvent, err := canonicalObject("document expected_event", doc.ExpectedEvent)
	if err != nil {
		return err
	}
	documentWaitReason, err := canonicalObject("document wait_reason", doc.WaitReason)
	if err != nil {
		return err
	}
	// PostgreSQL JSONB has its own key ordering. Restore the same recursive
	// canonical form Build hashed before checking the document digest.
	doc.Checkpoint = documentCheckpoint
	doc.ExpectedEvent = documentExpectedEvent
	doc.WaitReason = documentWaitReason
	canonical, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("workflow checkpoint: encode snapshot document: %w", err)
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != snapshot.SnapshotSha256 {
		return errors.New("workflow checkpoint: snapshot document digest mismatch")
	}
	checkpoint, err := canonicalObject("checkpoint", snapshot.Checkpoint)
	if err != nil {
		return err
	}
	if !bytes.Equal(checkpoint, documentCheckpoint) {
		return errors.New("workflow checkpoint: checkpoint column does not match snapshot document")
	}
	if doc.ContractVersion != ContractVersion || doc.WorkspaceID != uuidString(snapshot.WorkspaceID) || doc.IssueID != uuidString(snapshot.IssueID) || doc.ExecutionID != uuidString(snapshot.ExecutionID) || doc.ExecutionRevision != snapshot.ExecutionRevision {
		return errors.New("workflow checkpoint: snapshot document execution identity does not match row")
	}
	if doc.BranchKey != snapshot.BranchKey || doc.State != snapshot.ExecutionState || doc.ScopeID != uuidString(snapshot.ScopeID) || doc.ScopeRevision != snapshot.ScopeRevision {
		return errors.New("workflow checkpoint: snapshot document execution state does not match row")
	}
	if !equalStringPointer(doc.AuthorityID, nullableUUID(snapshot.AuthorityID)) || !equalInt64Pointer(doc.AuthorityRevision, nullableInt64(snapshot.AuthorityRevision)) ||
		!equalStringPointer(doc.ActorID, nullableUUID(snapshot.ActorID)) || !equalStringPointer(doc.TaskID, nullableUUID(snapshot.TaskID)) {
		return errors.New("workflow checkpoint: snapshot document bindings do not match row")
	}
	if doc.ChainAnchor != snapshot.ChainAnchor || !equalInt64Pointer(doc.PreviousRevision, nullableInt64(snapshot.PreviousRevision)) || !equalStringPointer(doc.PreviousSnapshotSHA256, nullableText(snapshot.PreviousSnapshotSha256)) {
		return errors.New("workflow checkpoint: snapshot document chain link does not match row")
	}
	if snapshot.ChainAnchor {
		if snapshot.PreviousRevision.Valid || snapshot.PreviousSnapshotSha256.Valid {
			return errors.New("workflow checkpoint: anchor has a predecessor")
		}
	} else if !snapshot.PreviousRevision.Valid || snapshot.PreviousRevision.Int64 != snapshot.ExecutionRevision-1 || !snapshot.PreviousSnapshotSha256.Valid || len(snapshot.PreviousSnapshotSha256.String) != 64 {
		return errors.New("workflow checkpoint: non-anchor predecessor is malformed")
	}
	return nil
}

// Build snapshots every field of an execution row that can affect its meaning.
// A nil predecessor deliberately starts an anchor, including when an execution
// predates this ledger and its current revision is greater than one.
func Build(execution db.IssueWorkflowExecution, predecessor *db.IssueWorkflowExecutionCheckpointSnapshot) (Built, error) {
	if !execution.ID.Valid || !execution.WorkspaceID.Valid || !execution.IssueID.Valid || !execution.ScopeID.Valid {
		return Built{}, errors.New("workflow checkpoint: execution identity is incomplete")
	}
	if execution.Revision <= 0 || execution.ScopeRevision <= 0 {
		return Built{}, errors.New("workflow checkpoint: revisions must be positive")
	}
	if !execution.CreatedAt.Valid || !execution.UpdatedAt.Valid {
		return Built{}, errors.New("workflow checkpoint: execution timestamps are incomplete")
	}
	if execution.AuthorityID.Valid != execution.AuthorityRevision.Valid {
		return Built{}, errors.New("workflow checkpoint: authority identity and revision must both be present or absent")
	}
	checkpoint, err := canonicalObject("checkpoint", execution.Checkpoint)
	if err != nil {
		return Built{}, err
	}
	expectedEvent, err := canonicalObject("expected_event", execution.ExpectedEvent)
	if err != nil {
		return Built{}, err
	}
	waitReason, err := canonicalObject("wait_reason", execution.WaitReason)
	if err != nil {
		return Built{}, err
	}

	chainAnchor := predecessor == nil
	var previousRevision *int64
	var previousDigest *string
	if predecessor != nil {
		if predecessor.WorkspaceID != execution.WorkspaceID || predecessor.IssueID != execution.IssueID || predecessor.ExecutionID != execution.ID {
			return Built{}, errors.New("workflow checkpoint: predecessor belongs to another execution")
		}
		if predecessor.ExecutionRevision != execution.Revision-1 {
			return Built{}, fmt.Errorf("workflow checkpoint: predecessor revision is %d, want %d", predecessor.ExecutionRevision, execution.Revision-1)
		}
		if len(predecessor.SnapshotSha256) != 64 {
			return Built{}, errors.New("workflow checkpoint: predecessor digest is malformed")
		}
		previousRevision = pointer(predecessor.ExecutionRevision)
		previousDigest = pointer(predecessor.SnapshotSha256)
	}

	doc := document{
		ContractVersion: ContractVersion, WorkspaceID: uuidString(execution.WorkspaceID), IssueID: uuidString(execution.IssueID),
		ExecutionID: uuidString(execution.ID), ExecutionRevision: execution.Revision,
		BranchKey: execution.BranchKey, OperationType: execution.OperationType, State: execution.State,
		ScopeID: uuidString(execution.ScopeID), ScopeRevision: execution.ScopeRevision,
		AuthorityID: nullableUUID(execution.AuthorityID), AuthorityRevision: nullableInt64(execution.AuthorityRevision),
		CandidateID: nullableUUID(execution.CandidateID), TargetKey: nullableText(execution.TargetKey),
		ActorType: nullableText(execution.ActorType), ActorID: nullableUUID(execution.ActorID), TaskID: nullableUUID(execution.TaskID),
		Checkpoint: checkpoint, ExpectedEvent: expectedEvent, WaitReason: waitReason,
		NextActionAt: nullableTime(execution.NextActionAt), AttemptCount: execution.AttemptCount, TerminalReason: execution.TerminalReason,
		SupersededBy: nullableUUID(execution.SupersededBy), SupersededAt: nullableTime(execution.SupersededAt),
		CreatedAt: requiredTime(execution.CreatedAt), UpdatedAt: requiredTime(execution.UpdatedAt),
		PreviousRevision: previousRevision, PreviousSnapshotSHA256: previousDigest, ChainAnchor: chainAnchor,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return Built{}, fmt.Errorf("workflow checkpoint: encode document: %w", err)
	}
	digest := sha256.Sum256(raw)
	built := Built{Document: raw, Digest: hex.EncodeToString(digest[:]), Checkpoint: checkpoint, ChainAnchor: chainAnchor}
	if previousRevision != nil {
		built.PreviousRevision = pgtype.Int8{Int64: *previousRevision, Valid: true}
		built.PreviousSnapshotSHA256 = pgtype.Text{String: *previousDigest, Valid: true}
	}
	return built, nil
}

func canonicalObject(name string, raw []byte) ([]byte, error) {
	if len(raw) > maxStructuredFieldBytes {
		return nil, fmt.Errorf("workflow checkpoint: %s exceeds %d bytes", name, maxStructuredFieldBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("workflow checkpoint: decode %s: %w", name, err)
	}
	if err := expectEOF(decoder); err != nil {
		return nil, fmt.Errorf("workflow checkpoint: decode %s: %w", name, err)
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("workflow checkpoint: %s must be an object", name)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("workflow checkpoint: encode %s: %w", name, err)
	}
	if len(canonical) > maxStructuredFieldBytes {
		return nil, fmt.Errorf("workflow checkpoint: canonical %s exceeds %d bytes", name, maxStructuredFieldBytes)
	}
	return canonical, nil
}

func expectEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("trailing JSON value")
	}
	return err
}

func uuidString(value pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", value.Bytes[0:4], value.Bytes[4:6], value.Bytes[6:8], value.Bytes[8:10], value.Bytes[10:16])
}
func nullableUUID(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	return pointer(uuidString(value))
}
func nullableText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return pointer(value.String)
}
func nullableInt64(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	return pointer(value.Int64)
}
func nullableTime(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	return pointer(value.Time.UTC().Format(time.RFC3339Nano))
}
func requiredTime(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}
func pointer[T any](value T) *T { return &value }

func equalStringPointer(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalInt64Pointer(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}
