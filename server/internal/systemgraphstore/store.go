// Package systemgraphstore connects the provider-free system graph contracts
// to durable workflow evidence. It is an internal controller boundary; it is
// not exposed through HTTP, task context, agent prompts, or provider code.
package systemgraphstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var (
	ErrStoreUnavailable       = errors.New("system graph workflow evidence store is unavailable")
	ErrWorkflowEvidenceDenied = errors.New("workflow evidence is missing, stale, superseded, or contradicted")
)

type TxStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Store struct {
	starter TxStarter
	queries *db.Queries
}

func New(starter TxStarter, queries *db.Queries) *Store {
	return &Store{starter: starter, queries: queries}
}

// ConsequenceAuthorityIssueV1 contains only the controller-selected identity
// and route. The store derives the role, action, graph version, authenticated
// state, and immutable state instead of accepting those trust facts as data.
type ConsequenceAuthorityIssueV1 struct {
	ReceiptID    string
	ControllerID string
	RouteVersion string
}

type consequenceGate struct {
	proof systemgraph.ConsequenceCreationProofV1
}

func (g consequenceGate) AuthorizeAndLoadCurrentConsequenceV1(systemgraph.ConsequenceReceiptInputV1) (systemgraph.ConsequenceCreationProofV1, error) {
	return g.proof, nil
}

// CreateConsequenceReceipt locks both current issue/outcome snapshots and
// appends one receipt in the same transaction. Exact retries return the prior
// receipt even after the issues advance; altered reuse fails closed.
func (s *Store) CreateConsequenceReceipt(
	ctx context.Context,
	workspaceID string,
	authorityIssue ConsequenceAuthorityIssueV1,
	input systemgraph.ConsequenceReceiptInputV1,
	now time.Time,
) (systemgraph.ConsequenceReceiptV1, error) {
	if s == nil || s.starter == nil || s.queries == nil {
		return systemgraph.ConsequenceReceiptV1{}, ErrStoreUnavailable
	}
	workspaceUUID, err := parseUUID(workspaceID, "workspace id")
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	authorityIssue.ReceiptID = strings.TrimSpace(authorityIssue.ReceiptID)
	authorityIssue.ControllerID = strings.TrimSpace(authorityIssue.ControllerID)
	authorityIssue.RouteVersion = strings.TrimSpace(authorityIssue.RouteVersion)
	if authorityIssue.ReceiptID == "" || authorityIssue.ControllerID == "" || authorityIssue.RouteVersion == "" {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("%w: incomplete controller authority", systemgraph.ErrConsequenceReceiptInvalid)
	}
	if input.ControllerAuthorityReceiptID != authorityIssue.ReceiptID {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("%w: controller authority receipt mismatch", systemgraph.ErrConsequenceReceiptInvalid)
	}
	sourceIssueID, err := parseUUID(input.Source.IssueID, "source issue id")
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	affectedIssueID, err := parseUUID(input.Affected.IssueID, "affected issue id")
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	if sourceIssueID == affectedIssueID {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("%w: source and affected issues must be distinct", systemgraph.ErrConsequenceReceiptInvalid)
	}

	tx, err := s.starter.Begin(ctx)
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.queries.WithTx(tx)
	if _, err := qtx.LockSystemGraphEvidenceWorkspaceOwner(ctx, workspaceUUID); err != nil {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("lock workflow workspace owner: %w", err)
	}
	ordered := []pgtype.UUID{sourceIssueID, affectedIssueID}
	sort.Slice(ordered, func(i, j int) bool { return bytes.Compare(ordered[i].Bytes[:], ordered[j].Bytes[:]) < 0 })
	for _, issueID := range ordered {
		if _, err := qtx.LockSystemGraphEvidenceIssueOwner(ctx, db.LockSystemGraphEvidenceIssueOwnerParams{WorkspaceID: workspaceUUID, IssueID: issueID}); err != nil {
			return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("lock workflow issue owner: %w", err)
		}
	}
	if err := qtx.LockSystemGraphEvidenceWorkspace(ctx, workspaceID); err != nil {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("lock system graph evidence workspace: %w", err)
	}

	existing, err := qtx.GetIssueWorkflowConsequenceReceiptByIdempotency(ctx, db.GetIssueWorkflowConsequenceReceiptByIdempotencyParams{
		WorkspaceID: workspaceUUID, IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
	})
	if err == nil {
		receipt, decodeErr := decodeConsequenceRow(existing, workspaceUUID)
		if decodeErr != nil {
			return systemgraph.ConsequenceReceiptV1{}, decodeErr
		}
		if !consequenceRequestMatches(receipt, authorityIssue, input) {
			return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("%w: altered idempotency replay", systemgraph.ErrConsequenceReceiptConflict)
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("load consequence replay: %w", err)
	}

	rows := make(map[pgtype.UUID]db.LockCurrentIssueOutcomeSnapshotRow, 2)
	for _, issueID := range ordered {
		row, loadErr := qtx.LockCurrentIssueOutcomeSnapshot(ctx, db.LockCurrentIssueOutcomeSnapshotParams{WorkspaceID: workspaceUUID, IssueID: issueID})
		if loadErr != nil {
			return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("load current issue outcome snapshot: %w", loadErr)
		}
		rows[issueID] = row
	}
	source, err := currentSnapshot(rows[sourceIssueID])
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	affected, err := currentSnapshot(rows[affectedIssueID])
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	graph, err := systemgraph.Default()
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	authority := systemgraph.ConsequenceControllerAuthorityV1{
		ReceiptID: authorityIssue.ReceiptID, ControllerID: authorityIssue.ControllerID,
		IssuerRole: "deterministic_controller", Source: "controller_record",
		Action: "record_cross_issue_consequence", RouteVersion: authorityIssue.RouteVersion,
		GraphVersion: graph.Version, SourceIssueID: input.Source.IssueID,
		AffectedIssueID: input.Affected.IssueID, Authenticated: true, Immutable: true,
	}
	journal, err := systemgraph.NewConsequenceReceiptStoreV1(systemgraph.ConsequenceReceiptJournalV1{}, consequenceGate{proof: systemgraph.ConsequenceCreationProofV1{
		Authority: authority, Source: source, Affected: affected,
	}})
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	prepared, err := journal.Prepare(input)
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	receipt, err := journal.Commit(prepared)
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	sourceOutcomeID, _ := parseUUID(receipt.Source.OutcomeID, "source outcome id")
	affectedOutcomeID, _ := parseUUID(receipt.Affected.OutcomeID, "affected outcome id")
	row, err := qtx.CreateIssueWorkflowConsequenceReceipt(ctx, db.CreateIssueWorkflowConsequenceReceiptParams{
		ID: deterministicUUID("consequence", receipt.ReceiptID), WorkspaceID: workspaceUUID,
		SourceIssueID: sourceIssueID, AffectedIssueID: affectedIssueID,
		IdempotencyKey: receipt.IdempotencyKey, ReceiptID: receipt.ReceiptID,
		ControllerAuthorityReceiptID: receipt.ControllerAuthority.ReceiptID,
		ControllerID:                 receipt.ControllerAuthority.ControllerID, GraphVersion: receipt.ControllerAuthority.GraphVersion,
		SourceOutcomeID: sourceOutcomeID, AffectedOutcomeID: affectedOutcomeID,
		SourceScopeRevision: receipt.Source.ScopeRevision, AffectedScopeRevision: receipt.Affected.ScopeRevision,
		SourceAuthorityRevision: receipt.Source.AuthorityRevision, AffectedAuthorityRevision: receipt.Affected.AuthorityRevision,
		RelationshipKind: receipt.RelationshipKind, PayloadSha256: receipt.PayloadSHA256,
		Receipt: receiptJSON, CommittedAt: timestamptz(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = qtx.GetIssueWorkflowConsequenceReceiptByIdempotency(ctx, db.GetIssueWorkflowConsequenceReceiptByIdempotencyParams{WorkspaceID: workspaceUUID, IdempotencyKey: receipt.IdempotencyKey})
	}
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("store consequence receipt: %w", err)
	}
	stored, err := decodeConsequenceRow(row, workspaceUUID)
	if err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	if !reflect.DeepEqual(stored, receipt) {
		return systemgraph.ConsequenceReceiptV1{}, fmt.Errorf("%w: stored consequence differs", systemgraph.ErrConsequenceReceiptConflict)
	}
	if err := tx.Commit(ctx); err != nil {
		return systemgraph.ConsequenceReceiptV1{}, err
	}
	return stored, nil
}

func (s *Store) ListConsequenceReceipts(ctx context.Context, workspaceID, issueID string) ([]systemgraph.ConsequenceReceiptV1, error) {
	if s == nil || s.queries == nil {
		return nil, ErrStoreUnavailable
	}
	workspaceUUID, err := parseUUID(workspaceID, "workspace id")
	if err != nil {
		return nil, err
	}
	issueUUID, err := parseUUID(issueID, "issue id")
	if err != nil {
		return nil, err
	}
	issueID = uuidString(issueUUID)
	rows, err := s.queries.ListIssueWorkflowConsequenceReceipts(ctx, db.ListIssueWorkflowConsequenceReceiptsParams{WorkspaceID: workspaceUUID, IssueID: issueUUID})
	if err != nil {
		return nil, err
	}
	result := make([]systemgraph.ConsequenceReceiptV1, 0, len(rows))
	for _, row := range rows {
		receipt, decodeErr := decodeConsequenceRow(row, workspaceUUID)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if receipt.Source.IssueID != issueID && receipt.Affected.IssueID != issueID {
			return nil, fmt.Errorf("%w: consequence issue binding mismatch", systemgraph.ErrConsequenceReceiptInvalid)
		}
		result = append(result, receipt)
	}
	return result, nil
}

// ApplyRegressionChange requires every domain source reference to resolve to a
// current, non-contradicted workflow evidence row for the exact source issue.
// The complete accepted and rejected hash chains are replayed before the new
// attempt is evaluated and appended.
func (s *Store) ApplyRegressionChange(
	ctx context.Context,
	workspaceID, sourceIssueID string,
	proposal systemgraph.RegressionProposal,
	evidence systemgraph.RegressionDomainEvidence,
	approval systemgraph.RegressionApprovalReceipt,
	now time.Time,
) (systemgraph.RegressionChangeRecord, error) {
	if s == nil || s.starter == nil || s.queries == nil {
		return systemgraph.RegressionChangeRecord{}, ErrStoreUnavailable
	}
	workspaceUUID, err := parseUUID(workspaceID, "workspace id")
	if err != nil {
		return systemgraph.RegressionChangeRecord{}, err
	}
	issueUUID, err := parseUUID(sourceIssueID, "source issue id")
	if err != nil {
		return systemgraph.RegressionChangeRecord{}, err
	}
	now = canonicalTime(now)
	tx, err := s.starter.Begin(ctx)
	if err != nil {
		return systemgraph.RegressionChangeRecord{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.queries.WithTx(tx)
	if _, err := qtx.LockSystemGraphEvidenceWorkspaceOwner(ctx, workspaceUUID); err != nil {
		return systemgraph.RegressionChangeRecord{}, fmt.Errorf("lock workflow workspace owner: %w", err)
	}
	if _, err := qtx.LockSystemGraphEvidenceIssueOwner(ctx, db.LockSystemGraphEvidenceIssueOwnerParams{WorkspaceID: workspaceUUID, IssueID: issueUUID}); err != nil {
		return systemgraph.RegressionChangeRecord{}, fmt.Errorf("lock workflow issue owner: %w", err)
	}
	if err := qtx.LockSystemGraphEvidenceWorkspace(ctx, workspaceID); err != nil {
		return systemgraph.RegressionChangeRecord{}, fmt.Errorf("lock system graph evidence workspace: %w", err)
	}
	registry, err := loadRegressionRegistry(ctx, qtx, workspaceUUID)
	if err != nil {
		return systemgraph.RegressionChangeRecord{}, err
	}
	beforeHistory, beforeRejected := len(registry.History()), len(registry.RejectedAttempts())
	record, applyErr := registry.Apply(proposal, evidence, approval, now)
	history, rejected := registry.History(), registry.RejectedAttempts()
	if len(history) == beforeHistory && len(rejected) == beforeRejected {
		if err := tx.Commit(ctx); err != nil {
			return systemgraph.RegressionChangeRecord{}, err
		}
		return record, applyErr
	}

	evidenceIDs, err := workflowEvidenceIDs(evidence.SourceRefs)
	if err != nil {
		return systemgraph.RegressionChangeRecord{}, err
	}
	found, err := qtx.ListCurrentWorkflowEvidenceRefs(ctx, db.ListCurrentWorkflowEvidenceRefsParams{
		WorkspaceID: workspaceUUID, IssueID: issueUUID, EvidenceIds: evidenceIDs, Now: timestamptz(now),
	})
	if err != nil {
		return systemgraph.RegressionChangeRecord{}, fmt.Errorf("load regression workflow evidence: %w", err)
	}
	if !sameUUIDSet(evidenceIDs, found) {
		return systemgraph.RegressionChangeRecord{}, fmt.Errorf("%w: every source_ref must identify current evidence on the exact issue", ErrWorkflowEvidenceDenied)
	}

	if len(history) > beforeHistory {
		if err := appendRegressionRecord(ctx, qtx, workspaceUUID, issueUUID, history[len(history)-1]); err != nil {
			return systemgraph.RegressionChangeRecord{}, err
		}
	}
	if len(rejected) > beforeRejected {
		if err := appendRegressionRejected(ctx, qtx, workspaceUUID, issueUUID, rejected[len(rejected)-1]); err != nil {
			return systemgraph.RegressionChangeRecord{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return systemgraph.RegressionChangeRecord{}, err
	}
	return record, applyErr
}

func (s *Store) LoadRegressionRegistry(ctx context.Context, workspaceID string) (*systemgraph.RegressionRegistry, error) {
	if s == nil || s.starter == nil || s.queries == nil {
		return nil, ErrStoreUnavailable
	}
	workspaceUUID, err := parseUUID(workspaceID, "workspace id")
	if err != nil {
		return nil, err
	}
	tx, err := s.starter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := s.queries.WithTx(tx)
	if _, err := qtx.LockSystemGraphEvidenceWorkspaceOwner(ctx, workspaceUUID); err != nil {
		return nil, fmt.Errorf("lock workflow workspace owner: %w", err)
	}
	if err := qtx.LockSystemGraphEvidenceWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}
	return loadRegressionRegistry(ctx, qtx, workspaceUUID)
}

func (s *Store) ActiveRegressionsForWorkflow(ctx context.Context, workspaceID string, impact systemgraph.RegressionImpactScope, consumer string, now time.Time) (systemgraph.RegressionSelection, error) {
	registry, err := s.LoadRegressionRegistry(ctx, workspaceID)
	if err != nil {
		return systemgraph.RegressionSelection{}, err
	}
	return registry.ActiveFor(impact, consumer, canonicalTime(now))
}

func loadRegressionRegistry(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) (*systemgraph.RegressionRegistry, error) {
	rows, err := queries.ListIssueWorkflowRegressionChangeRecords(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("load regression history: %w", err)
	}
	history := make([]systemgraph.RegressionChangeRecord, 0, len(rows))
	for _, row := range rows {
		var record systemgraph.RegressionChangeRecord
		if err := decodeStrict(row.Record, &record); err != nil {
			return nil, fmt.Errorf("decode regression record: %w", err)
		}
		if !row.ID.Valid || !row.SourceIssueID.Valid || row.RegistryVersion < 1 || uint64(row.RegistryVersion) != record.RegistryVersion || row.ProposalID != record.Proposal.ID || row.ObligationID != record.Proposal.Obligation.ID || row.RecordHash != record.RecordHash || row.PreviousRecordHash != record.PreviousRecordHash || !row.AppliedAt.Valid || !row.AppliedAt.Time.Equal(record.AppliedAt) {
			return nil, fmt.Errorf("%w: durable regression record columns disagree with payload", systemgraph.ErrRegressionHistoryCorrupt)
		}
		history = append(history, record)
	}
	auditRows, err := queries.ListIssueWorkflowRegressionRejectedAttempts(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("load regression rejected audit: %w", err)
	}
	audit := make([]systemgraph.RegressionRejectedAttempt, 0, len(auditRows))
	for _, row := range auditRows {
		var attempt systemgraph.RegressionRejectedAttempt
		if err := decodeStrict(row.Attempt, &attempt); err != nil {
			return nil, fmt.Errorf("decode regression rejected attempt: %w", err)
		}
		if !row.ID.Valid || !row.SourceIssueID.Valid || row.Sequence < 1 || uint64(row.Sequence) != attempt.Sequence || row.RegistryVersion < 0 || uint64(row.RegistryVersion) != attempt.RegistryVersion || row.ProposalID != attempt.Proposal.ID || row.ObligationID != attempt.Proposal.Obligation.ID || row.SubmissionHash != attempt.SubmissionHash || row.AttemptHash != attempt.AttemptHash || row.PreviousAttemptHash != attempt.PreviousAttemptHash || !row.AttemptedAt.Valid || !row.AttemptedAt.Time.Equal(attempt.AttemptedAt) {
			return nil, fmt.Errorf("%w: durable rejected-attempt columns disagree with payload", systemgraph.ErrRegressionHistoryCorrupt)
		}
		audit = append(audit, attempt)
	}
	return systemgraph.ReplayRegressionRegistry(history, audit)
}

func appendRegressionRecord(ctx context.Context, queries *db.Queries, workspaceID, issueID pgtype.UUID, record systemgraph.RegressionChangeRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if record.RegistryVersion > uint64(^uint64(0)>>1) {
		return fmt.Errorf("%w: registry version overflows durable representation", systemgraph.ErrRegressionInvalid)
	}
	_, err = queries.CreateIssueWorkflowRegressionChangeRecord(ctx, db.CreateIssueWorkflowRegressionChangeRecordParams{
		ID: deterministicUUID("regression-change", record.RecordHash), WorkspaceID: workspaceID, SourceIssueID: issueID,
		RegistryVersion: int64(record.RegistryVersion), ProposalID: record.Proposal.ID,
		ObligationID: record.Proposal.Obligation.ID, RecordHash: record.RecordHash,
		PreviousRecordHash: record.PreviousRecordHash, Record: raw, AppliedAt: timestamptz(record.AppliedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: regression history append conflicted", systemgraph.ErrRegressionConflict)
	}
	if err != nil {
		return fmt.Errorf("append regression history: %w", err)
	}
	return nil
}

func appendRegressionRejected(ctx context.Context, queries *db.Queries, workspaceID, issueID pgtype.UUID, attempt systemgraph.RegressionRejectedAttempt) error {
	raw, err := json.Marshal(attempt)
	if err != nil {
		return err
	}
	if attempt.Sequence > uint64(^uint64(0)>>1) || attempt.RegistryVersion > uint64(^uint64(0)>>1) {
		return fmt.Errorf("%w: rejected audit version overflows durable representation", systemgraph.ErrRegressionInvalid)
	}
	_, err = queries.CreateIssueWorkflowRegressionRejectedAttempt(ctx, db.CreateIssueWorkflowRegressionRejectedAttemptParams{
		ID: deterministicUUID("regression-rejected", attempt.AttemptHash), WorkspaceID: workspaceID, SourceIssueID: issueID,
		Sequence: int64(attempt.Sequence), RegistryVersion: int64(attempt.RegistryVersion),
		ProposalID: attempt.Proposal.ID, ObligationID: attempt.Proposal.Obligation.ID,
		SubmissionHash: attempt.SubmissionHash, AttemptHash: attempt.AttemptHash,
		PreviousAttemptHash: attempt.PreviousAttemptHash, Attempt: raw, AttemptedAt: timestamptz(attempt.AttemptedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: rejected regression audit append conflicted", systemgraph.ErrRegressionConflict)
	}
	if err != nil {
		return fmt.Errorf("append rejected regression audit: %w", err)
	}
	return nil
}

func currentSnapshot(row db.LockCurrentIssueOutcomeSnapshotRow) (systemgraph.CurrentIssueOutcomeSnapshotV1, error) {
	if !row.WorkspaceID.Valid || !row.IssueID.Valid || !row.OutcomeID.Valid || !row.ScopeRevision.Valid || row.ScopeRevision.Int64 < 1 || !row.AuthorityRevision.Valid || row.AuthorityRevision.Int64 < 1 || row.StoreRevision < 1 || strings.TrimSpace(row.OutcomeState) == "" {
		return systemgraph.CurrentIssueOutcomeSnapshotV1{}, fmt.Errorf("%w: incomplete current workflow snapshot", systemgraph.ErrConsequenceReceiptInvalid)
	}
	snapshot := systemgraph.IssueOutcomeSnapshotV1{
		IssueID: uuidString(row.IssueID), OutcomeID: uuidString(row.OutcomeID),
		ScopeRevision: row.ScopeRevision.Int64, Status: row.OutcomeState,
		AuthorityRevision: row.AuthorityRevision.Int64,
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return systemgraph.CurrentIssueOutcomeSnapshotV1{}, err
	}
	digest := sha256.Sum256(raw)
	return systemgraph.CurrentIssueOutcomeSnapshotV1{
		Snapshot: snapshot,
		Provenance: systemgraph.IssueOutcomeSnapshotProvenanceV1{
			SnapshotReceiptID: "issue-current-picture:" + snapshot.IssueID + ":" + strconv.FormatInt(row.StoreRevision, 10),
			StoreRevision:     strconv.FormatInt(row.StoreRevision, 10), SnapshotSHA256: hex.EncodeToString(digest[:]),
		},
	}, nil
}

func decodeConsequenceRow(row db.IssueWorkflowConsequenceReceipt, workspaceID pgtype.UUID) (systemgraph.ConsequenceReceiptV1, error) {
	var receipt systemgraph.ConsequenceReceiptV1
	if err := decodeStrict(row.Receipt, &receipt); err != nil {
		return receipt, fmt.Errorf("decode consequence receipt: %w", err)
	}
	if err := systemgraph.ValidateConsequenceReceiptV1(receipt); err != nil {
		return receipt, err
	}
	sourceIssueID, sourceErr := parseUUID(receipt.Source.IssueID, "stored source issue id")
	affectedIssueID, affectedErr := parseUUID(receipt.Affected.IssueID, "stored affected issue id")
	sourceOutcomeID, sourceOutcomeErr := parseUUID(receipt.Source.OutcomeID, "stored source outcome id")
	affectedOutcomeID, affectedOutcomeErr := parseUUID(receipt.Affected.OutcomeID, "stored affected outcome id")
	if sourceErr != nil || affectedErr != nil || sourceOutcomeErr != nil || affectedOutcomeErr != nil || !row.ID.Valid || row.WorkspaceID != workspaceID || row.SourceIssueID != sourceIssueID || row.AffectedIssueID != affectedIssueID || row.SourceOutcomeID != sourceOutcomeID || row.AffectedOutcomeID != affectedOutcomeID || row.IdempotencyKey != receipt.IdempotencyKey || row.ReceiptID != receipt.ReceiptID || row.ControllerAuthorityReceiptID != receipt.ControllerAuthority.ReceiptID || row.ControllerID != receipt.ControllerAuthority.ControllerID || row.GraphVersion != receipt.ControllerAuthority.GraphVersion || row.SourceScopeRevision != receipt.Source.ScopeRevision || row.AffectedScopeRevision != receipt.Affected.ScopeRevision || row.SourceAuthorityRevision != receipt.Source.AuthorityRevision || row.AffectedAuthorityRevision != receipt.Affected.AuthorityRevision || row.RelationshipKind != receipt.RelationshipKind || row.PayloadSha256 != receipt.PayloadSHA256 || !row.CommittedAt.Valid {
		return receipt, fmt.Errorf("%w: durable consequence columns disagree with payload", systemgraph.ErrConsequenceReceiptInvalid)
	}
	return receipt, nil
}

func consequenceRequestMatches(receipt systemgraph.ConsequenceReceiptV1, authority ConsequenceAuthorityIssueV1, input systemgraph.ConsequenceReceiptInputV1) bool {
	return receipt.IdempotencyKey == strings.TrimSpace(input.IdempotencyKey) &&
		receipt.ControllerAuthority.ReceiptID == authority.ReceiptID &&
		receipt.ControllerAuthority.ControllerID == authority.ControllerID &&
		receipt.ControllerAuthority.RouteVersion == authority.RouteVersion &&
		receipt.Source == input.Source && receipt.Affected == input.Affected &&
		receipt.RelationshipKind == strings.TrimSpace(input.RelationshipKind) &&
		reflect.DeepEqual(receipt.ImpactSet.AffectedIssueIDs, normalizedStrings(input.ImpactSet.AffectedIssueIDs)) &&
		reflect.DeepEqual(receipt.ImpactSet.UnrelatedControlIssueIDs, normalizedStrings(input.ImpactSet.UnrelatedControlIssueIDs)) &&
		reflect.DeepEqual(receipt.SharedEvidenceRefs, normalizedStrings(input.SharedEvidenceRefs))
}

func workflowEvidenceIDs(refs []string) ([]pgtype.UUID, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("%w: source_refs are required", ErrWorkflowEvidenceDenied)
	}
	result := make([]pgtype.UUID, 0, len(refs))
	seen := make(map[pgtype.UUID]struct{}, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if !strings.HasPrefix(ref, "workflow-evidence:") {
			return nil, fmt.Errorf("%w: source_ref must use workflow-evidence:<uuid>", ErrWorkflowEvidenceDenied)
		}
		value := strings.TrimSpace(strings.TrimPrefix(ref, "workflow-evidence:"))
		id, err := parseUUID(value, "workflow evidence ref")
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrWorkflowEvidenceDenied, err)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return bytes.Compare(result[i].Bytes[:], result[j].Bytes[:]) < 0 })
	return result, nil
}

func sameUUIDSet(left, right []pgtype.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	want := make(map[pgtype.UUID]struct{}, len(left))
	for _, id := range left {
		want[id] = struct{}{}
	}
	for _, id := range right {
		if _, ok := want[id]; !ok {
			return false
		}
	}
	return true
}

func normalizedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func parseUUID(value, field string) (pgtype.UUID, error) {
	id, err := util.ParseUUID(strings.TrimSpace(value))
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid %s", field)
	}
	return id, nil
}

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return util.UUIDToString(value)
}

func deterministicUUID(domain, identity string) pgtype.UUID {
	sum := sha256.Sum256([]byte(domain + "\x00" + identity))
	var result pgtype.UUID
	copy(result.Bytes[:], sum[:16])
	result.Bytes[6] = (result.Bytes[6] & 0x0f) | 0x50
	result.Bytes[8] = (result.Bytes[8] & 0x3f) | 0x80
	result.Valid = true
	return result
}

func canonicalTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: canonicalTime(value), Valid: true}
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}
