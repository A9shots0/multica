// Package workflowstore owns admission for normalized workflow writes.
package workflowstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowcheckpoint"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrInvalidIssueWriteCapability = errors.New("invalid or expired issue write capability")

// TxStarter is the minimum database capability needed for a workflow write.
type TxStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

// IssueWrite is capability-scoped to the lifetime of an admitted issue
// transaction. Its usable value can only be created by this package's admission APIs;
// every operation validates tenant scope and holds a lifetime lease through
// its complete database call.
type IssueWrite struct {
	state *issueWriteState
}

type issueWriteState struct {
	queries       *db.Queries
	workspaceID   pgtype.UUID
	issueID       pgtype.UUID
	mu            sync.RWMutex
	active        bool
	releaseActors map[pgtype.UUID]struct{}
}

func (write *IssueWrite) MatchesReleaseAdmission(operatorID, verifierID pgtype.UUID) bool {
	if write == nil || write.state == nil {
		return false
	}
	write.state.mu.RLock()
	defer write.state.mu.RUnlock()
	if !write.state.active || len(write.state.releaseActors) != 2 || operatorID == verifierID {
		return false
	}
	_, operator := write.state.releaseActors[operatorID]
	_, verifier := write.state.releaseActors[verifierID]
	return operator && verifier
}

type issueOperationLeaseKey struct{}

type issueOperationLease struct {
	state  *issueWriteState
	mu     sync.RWMutex
	active bool
}

// MatchesIssue proves that this opaque write capability was admitted for the
// exact tenant and issue. The identifiers cannot be supplied by callers.
func (write *IssueWrite) MatchesIssue(workspaceID, issueID pgtype.UUID) bool {
	if write == nil {
		return false
	}
	if write.state == nil {
		return false
	}
	write.state.mu.RLock()
	defer write.state.mu.RUnlock()
	return write.state.matchesIssueLocked(workspaceID, issueID)
}

func (state *issueWriteState) matchesIssueLocked(workspaceID, issueID pgtype.UUID) bool {
	return state.active && state.queries != nil && state.workspaceID.Valid && state.issueID.Valid && state.workspaceID == workspaceID && state.issueID == issueID
}

func (write *IssueWrite) beginOperation(ctx context.Context, workspaceID, issueID pgtype.UUID) (func(), error) {
	if write == nil || write.state == nil {
		return nil, ErrInvalidIssueWriteCapability
	}
	if lease, ok := ctx.Value(issueOperationLeaseKey{}).(*issueOperationLease); ok && lease.state == write.state {
		lease.mu.RLock()
		if !lease.active || write.state.workspaceID != workspaceID || write.state.issueID != issueID {
			lease.mu.RUnlock()
			return nil, ErrInvalidIssueWriteCapability
		}
		return lease.mu.RUnlock, nil
	}
	write.state.mu.RLock()
	if !write.state.matchesIssueLocked(workspaceID, issueID) {
		write.state.mu.RUnlock()
		return nil, ErrInvalidIssueWriteCapability
	}
	return write.state.mu.RUnlock, nil
}

// WithIssueOperation holds one operation-lifetime lease across a composite
// multi-query operation. Nested IssueWrite methods recognize the unforgeable
// context lease and remain covered without reacquiring the capability lock.
func (write *IssueWrite) WithIssueOperation(ctx context.Context, workspaceID, issueID pgtype.UUID, operation func(context.Context) error) error {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return err
	}
	defer finish()
	if lease, ok := ctx.Value(issueOperationLeaseKey{}).(*issueOperationLease); ok && lease.state == write.state {
		return operation(ctx)
	}
	lease := &issueOperationLease{state: write.state, active: true}
	leasedCtx := context.WithValue(ctx, issueOperationLeaseKey{}, lease)
	defer func() {
		lease.mu.Lock()
		lease.active = false
		lease.mu.Unlock()
	}()
	return operation(leasedCtx)
}

func (write *IssueWrite) ListProjectionInputs(ctx context.Context, params db.ListIssueWorkflowProjectionInputsParams) ([]db.ListIssueWorkflowProjectionInputsRow, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return nil, err
	}
	defer finish()
	return write.state.queries.ListIssueWorkflowProjectionInputs(ctx, params)
}

func (write *IssueWrite) ListConcernHeads(ctx context.Context, params db.ListIssueWorkflowConcernHeadsParams) ([]db.IssueWorkflowConcernHead, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return nil, err
	}
	defer finish()
	return write.state.queries.ListIssueWorkflowConcernHeads(ctx, params)
}

func (write *IssueWrite) InitializeConcernHead(ctx context.Context, params db.InitializeIssueWorkflowConcernHeadParams) (db.IssueWorkflowConcernHead, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowConcernHead{}, err
	}
	defer finish()
	return write.state.queries.InitializeIssueWorkflowConcernHead(ctx, params)
}

func (write *IssueWrite) AdvanceConcernHeadCAS(ctx context.Context, params db.AdvanceIssueWorkflowConcernHeadCASParams) (db.IssueWorkflowConcernHead, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowConcernHead{}, err
	}
	defer finish()
	return write.state.queries.AdvanceIssueWorkflowConcernHeadCAS(ctx, params)
}

func (write *IssueWrite) UpsertCurrentPictureCAS(ctx context.Context, params db.UpsertIssueCurrentPictureParams) (db.IssueCurrentPicture, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueCurrentPicture{}, err
	}
	defer finish()
	return write.state.queries.UpsertIssueCurrentPicture(ctx, params)
}

func (write *IssueWrite) GetIssueForNotification(ctx context.Context, workspaceID, issueID pgtype.UUID) (db.Issue, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.Issue{}, err
	}
	defer finish()
	return write.state.queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: workspaceID})
}

func (write *IssueWrite) ListTaskContinuationExecutionCandidates(ctx context.Context, params db.ListTaskContinuationExecutionCandidatesParams) ([]db.IssueWorkflowExecution, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return nil, err
	}
	defer finish()
	return write.state.queries.ListTaskContinuationExecutionCandidates(ctx, params)
}

func (write *IssueWrite) GetWorkspaceForNotification(ctx context.Context, workspaceID, issueID pgtype.UUID) (db.Workspace, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.Workspace{}, err
	}
	defer finish()
	return write.state.queries.GetWorkspace(ctx, workspaceID)
}

func (write *IssueWrite) GetOwnerNotificationBinding(ctx context.Context, workspaceID, issueID pgtype.UUID) (db.OwnerNotificationBinding, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.OwnerNotificationBinding{}, err
	}
	defer finish()
	return write.state.queries.GetOwnerNotificationBinding(ctx, workspaceID)
}

func (write *IssueWrite) StoreOwnerNotificationIntent(ctx context.Context, params db.AdmitOwnerNotificationParams) (db.OwnerNotification, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.OwnerNotification{}, err
	}
	defer finish()
	return write.state.queries.AdmitOwnerNotification(ctx, params)
}

func (write *IssueWrite) CreateCommand(ctx context.Context, params db.CreateIssueWorkflowCommandParams) (db.IssueWorkflowCommand, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowCommand{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowCommand(ctx, params)
}
func (write *IssueWrite) SetCommandResult(ctx context.Context, params db.SetIssueWorkflowCommandResultParams) (db.IssueWorkflowCommand, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowCommand{}, err
	}
	defer finish()
	return write.state.queries.SetIssueWorkflowCommandResult(ctx, params)
}
func (write *IssueWrite) CreateAction(ctx context.Context, params db.CreateIssueWorkflowActionReceiptParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowActionReceipt(ctx, params)
}
func (write *IssueWrite) UpdateExecution(ctx context.Context, params db.UpdateIssueWorkflowExecutionCASParams) (db.IssueWorkflowExecution, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	defer finish()
	current, err := write.state.queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{
		WorkspaceID: params.WorkspaceID, IssueID: params.IssueID, ID: params.ID,
	})
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	if current.Revision != params.ExpectedRevision {
		return db.IssueWorkflowExecution{}, pgx.ErrNoRows
	}
	if err := write.ensureExecutionSnapshot(ctx, current); err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	updated, err := write.state.queries.UpdateIssueWorkflowExecutionCAS(ctx, params)
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	if err := write.ensureExecutionSnapshot(ctx, updated); err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	return updated, nil
}
func (write *IssueWrite) CreateReview(ctx context.Context, params db.CreateIssueWorkflowReviewParams) (db.IssueWorkflowReview, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowReview{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowReview(ctx, params)
}
func (write *IssueWrite) GetReview(ctx context.Context, params db.GetIssueWorkflowReviewParams) (db.IssueWorkflowReview, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowReview{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowReview(ctx, params)
}
func (write *IssueWrite) GetRelease(ctx context.Context, params db.GetIssueWorkflowReleaseParams) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowRelease(ctx, params)
}

// GetWorkspaceReleaseByTargetGeneration reads the immutable predecessor bound
// to the admitted issue's workspace. The admitted issue ID is required solely
// to retain the capability/lifetime fence; the returned release may belong to
// another issue in the same workspace.
func (write *IssueWrite) GetWorkspaceReleaseByTargetGeneration(ctx context.Context, workspaceID, admittedIssueID pgtype.UUID, targetKey string, generation int64) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, workspaceID, admittedIssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowReleaseByTargetGeneration(ctx, db.GetIssueWorkflowReleaseByTargetGenerationParams{WorkspaceID: workspaceID, TargetKey: targetKey, TargetGeneration: generation})
}

func (write *IssueWrite) GetWorkspaceCandidate(ctx context.Context, workspaceID, admittedIssueID, id pgtype.UUID) (db.IssueWorkflowCandidate, error) {
	finish, err := write.beginOperation(ctx, workspaceID, admittedIssueID)
	if err != nil {
		return db.IssueWorkflowCandidate{}, err
	}
	defer finish()
	return write.state.queries.GetWorkspaceWorkflowCandidate(ctx, db.GetWorkspaceWorkflowCandidateParams{WorkspaceID: workspaceID, ID: id})
}

func (write *IssueWrite) GetWorkspaceReview(ctx context.Context, workspaceID, admittedIssueID, id pgtype.UUID) (db.IssueWorkflowReview, error) {
	finish, err := write.beginOperation(ctx, workspaceID, admittedIssueID)
	if err != nil {
		return db.IssueWorkflowReview{}, err
	}
	defer finish()
	return write.state.queries.GetWorkspaceWorkflowReview(ctx, db.GetWorkspaceWorkflowReviewParams{WorkspaceID: workspaceID, ID: id})
}

func (write *IssueWrite) ListWorkspaceReleaseEvidence(ctx context.Context, workspaceID, admittedIssueID, releaseID pgtype.UUID) ([]db.IssueWorkflowEvidence, error) {
	finish, err := write.beginOperation(ctx, workspaceID, admittedIssueID)
	if err != nil {
		return nil, err
	}
	defer finish()
	return write.state.queries.ListWorkspaceWorkflowReleaseEvidence(ctx, db.ListWorkspaceWorkflowReleaseEvidenceParams{WorkspaceID: workspaceID, ReleaseID: releaseID})
}
func (write *IssueWrite) UpdateRelease(ctx context.Context, params db.UpdateIssueWorkflowReleaseCASParams) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.UpdateIssueWorkflowReleaseCAS(ctx, params)
}

func (write *IssueWrite) RebindPendingReleaseCandidate(ctx context.Context, params db.RebindPendingIssueWorkflowReleaseCandidateCASParams) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.RebindPendingIssueWorkflowReleaseCandidateCAS(ctx, params)
}

func (write *IssueWrite) MarkTaskResultReconciliationFailed(ctx context.Context, params db.MarkWorkflowTaskResultReconciliationFailedParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.MarkWorkflowTaskResultReconciliationFailed(ctx, params)
}
func (write *IssueWrite) CreateDecision(ctx context.Context, params db.CreateIssueWorkflowDecisionParams) (db.IssueWorkflowDecision, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowDecision{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowDecision(ctx, params)
}
func (write *IssueWrite) UpdateDecision(ctx context.Context, params db.UpdateIssueWorkflowDecisionStateCASParams) (db.IssueWorkflowDecision, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowDecision{}, err
	}
	defer finish()
	return write.state.queries.UpdateIssueWorkflowDecisionStateCAS(ctx, params)
}
func (write *IssueWrite) CreateDecisionResponse(ctx context.Context, params db.CreateIssueWorkflowDecisionResponseParams) (db.IssueWorkflowDecisionResponse, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowDecisionResponse{}, err
	}
	defer finish()
	response, err := write.state.queries.CreateIssueWorkflowDecisionResponse(ctx, params)
	if err != nil {
		return response, err
	}
	// Keep the notification and canonical response atomic. Otherwise a saved
	// answer can leave Chat and Inbox asking the owner the same question again.
	err = write.state.queries.ArchiveInboxByDecision(ctx, db.ArchiveInboxByDecisionParams{
		WorkspaceID: params.WorkspaceID, IssueID: params.IssueID,
		OwnerID: params.ActorID, DecisionID: params.DecisionID.String(),
	})
	return response, err
}
func (write *IssueWrite) CreateEvidence(ctx context.Context, params db.CreateIssueWorkflowEvidenceParams) (db.IssueWorkflowEvidence, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowEvidence{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowEvidence(ctx, params)
}
func (write *IssueWrite) UpdateAttention(ctx context.Context, params db.UpdateIssueWorkflowAttentionCASParams) (db.IssueWorkflowAttention, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowAttention{}, err
	}
	defer finish()
	return write.state.queries.UpdateIssueWorkflowAttentionCAS(ctx, params)
}
func (write *IssueWrite) CreateAttention(ctx context.Context, params db.CreateIssueWorkflowAttentionParams) (db.IssueWorkflowAttention, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowAttention{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowAttention(ctx, params)
}
func (write *IssueWrite) UpdateOutcome(ctx context.Context, params db.UpdateIssueWorkflowOutcomeCASParams) (db.IssueWorkflowOutcome, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowOutcome{}, err
	}
	defer finish()
	return write.state.queries.UpdateIssueWorkflowOutcomeCAS(ctx, params)
}
func (write *IssueWrite) LockReviewerAuthority(ctx context.Context, params db.LockWorkflowReviewerCommandAuthorityParams) (bool, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return false, err
	}
	defer finish()
	return write.state.queries.LockWorkflowReviewerCommandAuthority(ctx, params)
}
func (write *IssueWrite) LockFinalizerAuthority(ctx context.Context, params db.LockWorkflowFinalizerCommandAuthorityParams) (bool, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return false, err
	}
	defer finish()
	return write.state.queries.LockWorkflowFinalizerCommandAuthority(ctx, params)
}

func (write *IssueWrite) CreateOutcome(ctx context.Context, params db.CreateIssueWorkflowOutcomeParams) (db.IssueWorkflowOutcome, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowOutcome{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowOutcome(ctx, params)
}
func (write *IssueWrite) GetCommandByKey(ctx context.Context, workspaceID, issueID pgtype.UUID, commandKey string) (db.IssueWorkflowCommand, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueWorkflowCommand{}, err
	}
	defer finish()
	row, err := write.state.queries.GetIssueWorkflowCommandByKey(ctx, db.GetIssueWorkflowCommandByKeyParams{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: commandKey})
	if err == nil && row.IssueID != issueID {
		return db.IssueWorkflowCommand{}, ErrInvalidIssueWriteCapability
	}
	return row, err
}
func (write *IssueWrite) GetActionByKey(ctx context.Context, workspaceID, issueID pgtype.UUID, actionKey string) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	row, err := write.state.queries.GetIssueWorkflowActionReceiptByKey(ctx, db.GetIssueWorkflowActionReceiptByKeyParams{WorkspaceID: workspaceID, ActionKey: actionKey})
	if err == nil && row.IssueID != issueID {
		return db.IssueWorkflowActionReceipt{}, ErrInvalidIssueWriteCapability
	}
	return row, err
}
func (write *IssueWrite) GetCurrentPicture(ctx context.Context, params db.GetIssueCurrentPictureParams) (db.IssueCurrentPicture, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueCurrentPicture{}, err
	}
	defer finish()
	return write.state.queries.GetIssueCurrentPicture(ctx, params)
}
func (write *IssueWrite) LockOwnerAuthority(ctx context.Context, workspaceID, issueID, actorID pgtype.UUID) (bool, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return false, err
	}
	defer finish()
	return write.state.queries.LockWorkflowOwnerCommandAuthority(ctx, db.LockWorkflowOwnerCommandAuthorityParams{WorkspaceID: workspaceID, ActorID: actorID})
}
func (write *IssueWrite) GetDecision(ctx context.Context, params db.GetIssueWorkflowDecisionParams) (db.IssueWorkflowDecision, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowDecision{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowDecision(ctx, params)
}
func (write *IssueWrite) ListEvidence(ctx context.Context, params db.ListIssueWorkflowEvidenceParams) ([]db.IssueWorkflowEvidence, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return nil, err
	}
	defer finish()
	return write.state.queries.ListIssueWorkflowEvidence(ctx, params)
}
func (write *IssueWrite) GetCandidate(ctx context.Context, params db.GetIssueWorkflowCandidateParams) (db.IssueWorkflowCandidate, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowCandidate{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowCandidate(ctx, params)
}

func (write *IssueWrite) CreateCandidate(ctx context.Context, params db.CreateIssueWorkflowCandidateParams) (db.IssueWorkflowCandidate, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowCandidate{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowCandidate(ctx, params)
}

func (write *IssueWrite) SupersedeCandidate(ctx context.Context, params db.SupersedeIssueWorkflowCandidateCASParams) (db.IssueWorkflowCandidate, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowCandidate{}, err
	}
	defer finish()
	return write.state.queries.SupersedeIssueWorkflowCandidateCAS(ctx, params)
}

func (write *IssueWrite) CreateRelease(ctx context.Context, params db.CreateIssueWorkflowReleaseParams) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowRelease(ctx, params)
}

func (write *IssueWrite) CreateReleaseAttempt(ctx context.Context, params db.CreateIssueWorkflowReleaseAttemptParams) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowReleaseAttempt(ctx, params)
}

func (write *IssueWrite) CreateRecoveryReleaseAttempt(ctx context.Context, params db.CreateIssueWorkflowRecoveryReleaseAttemptParams) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowRecoveryReleaseAttempt(ctx, params)
}

func (write *IssueWrite) AdmitPendingReleaseAttempt(ctx context.Context, params db.AdmitPendingIssueWorkflowReleaseAttemptCASParams) (db.IssueWorkflowRelease, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowRelease{}, err
	}
	defer finish()
	return write.state.queries.AdmitPendingIssueWorkflowReleaseAttemptCAS(ctx, params)
}

func (write *IssueWrite) CreateReleaseIncident(ctx context.Context, params db.CreateIssueWorkflowReleaseIncidentParams) (db.IssueWorkflowIncident, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowIncident{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowReleaseIncident(ctx, params)
}

func (write *IssueWrite) LockRecoveryIncident(ctx context.Context, params db.LockIssueWorkflowRecoveryIncidentParams) (db.IssueWorkflowIncident, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowIncident{}, err
	}
	defer finish()
	return write.state.queries.LockIssueWorkflowRecoveryIncident(ctx, params)
}

func (write *IssueWrite) UpdateIncident(ctx context.Context, params db.UpdateIssueWorkflowIncidentCASParams) (db.IssueWorkflowIncident, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowIncident{}, err
	}
	defer finish()
	return write.state.queries.UpdateIssueWorkflowIncidentCAS(ctx, params)
}

func (write *IssueWrite) LockReleaseTarget(ctx context.Context, workspaceID, issueID pgtype.UUID, targetKey string) error {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return err
	}
	defer finish()
	return write.state.queries.LockIssueWorkflowTarget(ctx, db.LockIssueWorkflowTargetParams{WorkspaceID: workspaceID, TargetKey: targetKey})
}

func (write *IssueWrite) LockReleaseTargetHead(ctx context.Context, workspaceID, issueID pgtype.UUID, targetKey string) (db.IssueWorkflowTargetHead, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueWorkflowTargetHead{}, err
	}
	defer finish()
	return write.state.queries.LockIssueWorkflowTargetHead(ctx, db.LockIssueWorkflowTargetHeadParams{WorkspaceID: workspaceID, TargetKey: targetKey})
}

func (write *IssueWrite) AdvanceReleaseTargetHead(ctx context.Context, params db.AdvanceIssueWorkflowTargetHeadCASParams) (db.IssueWorkflowTargetHead, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowTargetHead{}, err
	}
	defer finish()
	return write.state.queries.AdvanceIssueWorkflowTargetHeadCAS(ctx, params)
}

func (write *IssueWrite) UpdateReleaseTargetHeadState(ctx context.Context, params db.UpdateIssueWorkflowTargetHeadStateCASParams) (db.IssueWorkflowTargetHead, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowTargetHead{}, err
	}
	defer finish()
	return write.state.queries.UpdateIssueWorkflowTargetHeadStateCAS(ctx, params)
}

func (write *IssueWrite) UpdateCorrectionCandidate(ctx context.Context, params db.UpdateIssueWorkflowExecutionCorrectionCandidateCASParams) (db.IssueWorkflowExecution, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	defer finish()
	current, err := write.state.queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{
		WorkspaceID: params.WorkspaceID, IssueID: params.IssueID, ID: params.ID,
	})
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	if current.Revision != params.ExpectedRevision {
		return db.IssueWorkflowExecution{}, pgx.ErrNoRows
	}
	if err := write.ensureExecutionSnapshot(ctx, current); err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	updated, err := write.state.queries.UpdateIssueWorkflowExecutionCorrectionCandidateCAS(ctx, params)
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	if err := write.ensureExecutionSnapshot(ctx, updated); err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	return updated, nil
}

func (write *IssueWrite) LockCorrectionResultAuthority(ctx context.Context, params db.LockWorkflowCorrectionResultAuthorityParams) (db.LockWorkflowCorrectionResultAuthorityRow, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.LockWorkflowCorrectionResultAuthorityRow{}, err
	}
	defer finish()
	return write.state.queries.LockWorkflowCorrectionResultAuthority(ctx, params)
}

func (write *IssueWrite) LockReviewResultAuthority(ctx context.Context, params db.LockWorkflowReviewResultAuthorityParams) (db.LockWorkflowReviewResultAuthorityRow, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.LockWorkflowReviewResultAuthorityRow{}, err
	}
	defer finish()
	return write.state.queries.LockWorkflowReviewResultAuthority(ctx, params)
}

func (write *IssueWrite) LockReleaseVerificationResultAuthority(ctx context.Context, params db.LockWorkflowReleaseVerificationResultAuthorityParams) (db.LockWorkflowReleaseVerificationResultAuthorityRow, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.LockWorkflowReleaseVerificationResultAuthorityRow{}, err
	}
	defer finish()
	return write.state.queries.LockWorkflowReleaseVerificationResultAuthority(ctx, params)
}

func (write *IssueWrite) LockWorkflowPlan(ctx context.Context, workspaceID, issueID pgtype.UUID) (db.IssueWorkflowPlan, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueWorkflowPlan{}, err
	}
	defer finish()
	return write.state.queries.LockIssueWorkflowPlanForV2(ctx, db.LockIssueWorkflowPlanForV2Params{WorkspaceID: workspaceID, IssueID: issueID})
}

func (write *IssueWrite) GetScope(ctx context.Context, params db.GetIssueWorkflowScopeParams) (db.IssueWorkflowScope, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowScope{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowScope(ctx, params)
}

func (write *IssueWrite) GetLatestScope(ctx context.Context, workspaceID, issueID pgtype.UUID) (db.IssueWorkflowScope, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueWorkflowScope{}, err
	}
	defer finish()
	return write.state.queries.GetLatestIssueWorkflowScope(ctx, db.GetLatestIssueWorkflowScopeParams{WorkspaceID: workspaceID, IssueID: issueID})
}

func (write *IssueWrite) CreateScope(ctx context.Context, params db.CreateIssueWorkflowScopeParams) (db.IssueWorkflowScope, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowScope{}, err
	}
	defer finish()
	return write.state.queries.CreateIssueWorkflowScope(ctx, params)
}

func (write *IssueWrite) GetAuthority(ctx context.Context, params db.GetIssueWorkflowAuthorityParams) (db.IssueWorkflowAuthority, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowAuthority{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowAuthority(ctx, params)
}

func (write *IssueWrite) GetLatestAuthority(ctx context.Context, workspaceID, issueID pgtype.UUID) (db.IssueWorkflowAuthority, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueWorkflowAuthority{}, err
	}
	defer finish()
	return write.state.queries.GetLatestIssueWorkflowAuthority(ctx, db.GetLatestIssueWorkflowAuthorityParams{WorkspaceID: workspaceID, IssueID: issueID})
}

func (write *IssueWrite) LockReleaseBuildAuthority(ctx context.Context, params db.LockWorkflowReleaseBuildAuthorityParams) (db.LockWorkflowReleaseBuildAuthorityRow, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.LockWorkflowReleaseBuildAuthorityRow{}, err
	}
	defer finish()
	return write.state.queries.LockWorkflowReleaseBuildAuthority(ctx, params)
}
func (write *IssueWrite) GetConcernHead(ctx context.Context, params db.GetIssueWorkflowConcernHeadParams) (db.IssueWorkflowConcernHead, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowConcernHead{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowConcernHead(ctx, params)
}
func (write *IssueWrite) LockConcernHead(ctx context.Context, params db.LockIssueWorkflowConcernHeadParams) (db.IssueWorkflowConcernHead, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowConcernHead{}, err
	}
	defer finish()
	return write.state.queries.LockIssueWorkflowConcernHead(ctx, params)
}

type ReleaseSnapshotSources struct {
	Scope     db.IssueWorkflowScope
	Authority db.IssueWorkflowAuthority
	Execution db.IssueWorkflowExecution
	Review    db.IssueWorkflowReview
	Candidate db.IssueWorkflowCandidate
}

// LockReleaseSnapshotSources freezes the exact source rows selected by the
// already-locked concern heads. It is intentionally one capability operation
// so callback invalidation cannot interleave across the snapshot.
func (write *IssueWrite) LockReleaseSnapshotSources(ctx context.Context, workspaceID, issueID, scopeID, authorityID, executionID, reviewID, candidateID pgtype.UUID) (ReleaseSnapshotSources, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return ReleaseSnapshotSources{}, err
	}
	defer finish()
	q := write.state.queries
	var result ReleaseSnapshotSources
	if result.Scope, err = q.LockIssueWorkflowScopeRecord(ctx, db.LockIssueWorkflowScopeRecordParams{WorkspaceID: workspaceID, IssueID: issueID, ID: scopeID}); err != nil {
		return ReleaseSnapshotSources{}, err
	}
	if result.Authority, err = q.LockIssueWorkflowAuthorityRecord(ctx, db.LockIssueWorkflowAuthorityRecordParams{WorkspaceID: workspaceID, IssueID: issueID, ID: authorityID}); err != nil {
		return ReleaseSnapshotSources{}, err
	}
	if result.Execution, err = q.LockIssueWorkflowExecutionRecord(ctx, db.LockIssueWorkflowExecutionRecordParams{WorkspaceID: workspaceID, IssueID: issueID, ID: executionID}); err != nil {
		return ReleaseSnapshotSources{}, err
	}
	if result.Review, err = q.LockIssueWorkflowReviewRecord(ctx, db.LockIssueWorkflowReviewRecordParams{WorkspaceID: workspaceID, IssueID: issueID, ID: reviewID}); err != nil {
		return ReleaseSnapshotSources{}, err
	}
	if result.Candidate, err = q.LockIssueWorkflowCandidateRecord(ctx, db.LockIssueWorkflowCandidateRecordParams{WorkspaceID: workspaceID, IssueID: issueID, ID: candidateID}); err != nil {
		return ReleaseSnapshotSources{}, err
	}
	return result, nil
}

func (write *IssueWrite) LockEvidenceRecord(ctx context.Context, params db.LockIssueWorkflowEvidenceRecordParams) (db.IssueWorkflowEvidence, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowEvidence{}, err
	}
	defer finish()
	return write.state.queries.LockIssueWorkflowEvidenceRecord(ctx, params)
}
func (write *IssueWrite) GetExecution(ctx context.Context, params db.GetIssueWorkflowExecutionParams) (db.IssueWorkflowExecution, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowExecution(ctx, params)
}

func (write *IssueWrite) CreateExecution(ctx context.Context, params db.CreateIssueWorkflowExecutionParams) (db.IssueWorkflowExecution, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	defer finish()
	execution, err := write.state.queries.CreateIssueWorkflowExecution(ctx, params)
	if err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	if err := write.ensureExecutionSnapshot(ctx, execution); err != nil {
		return db.IssueWorkflowExecution{}, err
	}
	return execution, nil
}

func (write *IssueWrite) ensureExecutionSnapshot(ctx context.Context, execution db.IssueWorkflowExecution) error {
	q := write.state.queries
	latest, err := q.GetLatestIssueWorkflowExecutionCheckpointSnapshot(ctx, db.GetLatestIssueWorkflowExecutionCheckpointSnapshotParams{
		WorkspaceID: execution.WorkspaceID, IssueID: execution.IssueID, ExecutionID: execution.ID,
	})
	var predecessor *db.IssueWorkflowExecutionCheckpointSnapshot
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// An execution created before this ledger is intentionally anchored at
		// the first revision observed; history is never fabricated.
	case err != nil:
		return fmt.Errorf("load latest workflow checkpoint snapshot: %w", err)
	case latest.ExecutionRevision == execution.Revision-1:
		predecessor = &latest
	case latest.ExecutionRevision == execution.Revision:
		if !latest.ChainAnchor {
			if !latest.PreviousRevision.Valid || latest.PreviousRevision.Int64 != execution.Revision-1 || !latest.PreviousSnapshotSha256.Valid {
				return errors.New("workflow checkpoint: existing snapshot has malformed predecessor binding")
			}
			previous, getErr := q.GetIssueWorkflowExecutionCheckpointSnapshot(ctx, db.GetIssueWorkflowExecutionCheckpointSnapshotParams{
				WorkspaceID: execution.WorkspaceID, IssueID: execution.IssueID, ExecutionID: execution.ID,
				ExecutionRevision: latest.PreviousRevision.Int64,
			})
			if getErr != nil {
				return fmt.Errorf("load workflow checkpoint predecessor: %w", getErr)
			}
			if previous.SnapshotSha256 != latest.PreviousSnapshotSha256.String {
				return errors.New("workflow checkpoint: existing snapshot predecessor digest does not match")
			}
			predecessor = &previous
		}
		built, buildErr := workflowcheckpoint.Build(execution, predecessor)
		if buildErr != nil {
			return buildErr
		}
		if err := workflowcheckpoint.Verify(latest); err != nil {
			return err
		}
		if built.Digest != latest.SnapshotSha256 {
			return errors.New("workflow checkpoint: existing snapshot does not match execution revision")
		}
		return nil
	default:
		return fmt.Errorf("workflow checkpoint: latest revision is %d, cannot append execution revision %d", latest.ExecutionRevision, execution.Revision)
	}

	built, err := workflowcheckpoint.Build(execution, predecessor)
	if err != nil {
		return err
	}
	_, err = q.PutIssueWorkflowExecutionCheckpointSnapshot(ctx, db.PutIssueWorkflowExecutionCheckpointSnapshotParams{
		ID:          pgtype.UUID{Bytes: googleuuid.New(), Valid: true},
		WorkspaceID: execution.WorkspaceID, IssueID: execution.IssueID, ExecutionID: execution.ID,
		ExecutionRevision: execution.Revision, TaskID: execution.TaskID, ActorID: execution.ActorID,
		ScopeID: execution.ScopeID, ScopeRevision: execution.ScopeRevision,
		AuthorityID: execution.AuthorityID, AuthorityRevision: execution.AuthorityRevision,
		BranchKey: execution.BranchKey, ExecutionState: execution.State,
		Checkpoint: built.Checkpoint, SnapshotDocument: built.Document,
		PreviousRevision: built.PreviousRevision, PreviousSnapshotSha256: built.PreviousSnapshotSHA256,
		ChainAnchor: built.ChainAnchor, SnapshotSha256: built.Digest,
	})
	if err != nil {
		return fmt.Errorf("append workflow checkpoint snapshot: %w", err)
	}
	return nil
}

func (write *IssueWrite) GetExternalWorkReceiptByKey(ctx context.Context, workspaceID, issueID pgtype.UUID, idempotencyKey string) (db.IssueExternalWorkReceipt, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueExternalWorkReceipt{}, err
	}
	defer finish()
	return write.state.queries.GetExternalWorkReceiptByIdempotencyKey(ctx, db.GetExternalWorkReceiptByIdempotencyKeyParams{WorkspaceID: workspaceID, IssueID: issueID, IdempotencyKey: idempotencyKey})
}

func (write *IssueWrite) LockExternalWorkReceiptStream(ctx context.Context, workspaceID, issueID pgtype.UUID, provider, workID string) error {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return err
	}
	defer finish()
	return write.state.queries.LockExternalWorkReceiptStream(ctx, db.LockExternalWorkReceiptStreamParams{
		WorkspaceID: workspaceID, IssueID: issueID, Provider: provider, WorkID: workID,
	})
}

func (write *IssueWrite) GetLatestExternalWorkReceipt(ctx context.Context, workspaceID, issueID pgtype.UUID, provider, workID string) (db.IssueExternalWorkReceipt, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return db.IssueExternalWorkReceipt{}, err
	}
	defer finish()
	return write.state.queries.GetLatestExternalWorkReceiptForWork(ctx, db.GetLatestExternalWorkReceiptForWorkParams{WorkspaceID: workspaceID, IssueID: issueID, Provider: provider, WorkID: workID})
}

func (write *IssueWrite) AdmitExternalWorkReceipt(ctx context.Context, params db.CreateExternalWorkReceiptParams) (db.IssueExternalWorkReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueExternalWorkReceipt{}, err
	}
	defer finish()
	return write.state.queries.CreateExternalWorkReceipt(ctx, params)
}

func (write *IssueWrite) SetExternalWorkReceiptResult(ctx context.Context, params db.SetExternalWorkReceiptAdmissionResultParams) (db.IssueExternalWorkReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueExternalWorkReceipt{}, err
	}
	defer finish()
	return write.state.queries.SetExternalWorkReceiptAdmissionResult(ctx, params)
}
func (write *IssueWrite) GetAttention(ctx context.Context, params db.GetIssueWorkflowAttentionParams) (db.IssueWorkflowAttention, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowAttention{}, err
	}
	defer finish()
	return write.state.queries.GetIssueWorkflowAttention(ctx, params)
}
func (write *IssueWrite) GetLatestOutcome(ctx context.Context, params db.GetLatestIssueWorkflowOutcomeParams) (db.IssueWorkflowOutcome, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowOutcome{}, err
	}
	defer finish()
	return write.state.queries.GetLatestIssueWorkflowOutcome(ctx, params)
}

func (write *IssueWrite) ClaimAction(ctx context.Context, params db.ClaimIssueWorkflowActionExactParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.ClaimIssueWorkflowActionExact(ctx, params)
}

func (write *IssueWrite) DeferAction(ctx context.Context, params db.DeferIssueWorkflowActionExactParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.DeferIssueWorkflowActionExact(ctx, params)
}

func (write *IssueWrite) BeginActionAttempt(ctx context.Context, params db.BeginIssueWorkflowActionAttemptParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.BeginIssueWorkflowActionAttempt(ctx, params)
}

func (write *IssueWrite) RenewAction(ctx context.Context, params db.RenewIssueWorkflowActionLeaseParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.RenewIssueWorkflowActionLease(ctx, params)
}

func (write *IssueWrite) RecordActionObservationFailure(ctx context.Context, params db.RecordIssueWorkflowActionObservationFailureParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.RecordIssueWorkflowActionObservationFailure(ctx, params)
}

func (write *IssueWrite) SettleAction(ctx context.Context, params db.SettleIssueWorkflowActionParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.SettleIssueWorkflowAction(ctx, params)
}

func (write *IssueWrite) DeadLetterAction(ctx context.Context, params db.DeadLetterIssueWorkflowActionExactParams) (db.IssueWorkflowActionReceipt, error) {
	finish, err := write.beginOperation(ctx, params.WorkspaceID, params.IssueID)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	defer finish()
	return write.state.queries.DeadLetterIssueWorkflowActionExact(ctx, params)
}

// UpsertLegacyCurrentPicture admits a projection inside a caller-owned legacy
// transaction. It never starts or commits a separate transaction: the legacy
// plan, task, picture and Inbox receipt must succeed or roll back together.
// NOWAIT owner locks avoid inverting archival lock order when the caller already
// holds issue/plan/task locks. The caller must propagate every error and roll
// back; successful owner locks remain held until its transaction ends.
// Only this scoped projection operation is exposed, never a reusable capability.
func UpsertLegacyCurrentPicture(ctx context.Context, tx pgx.Tx, params db.UpsertIssueCurrentPictureParams) (db.IssueCurrentPicture, error) {
	if tx == nil || !params.WorkspaceID.Valid || !params.IssueID.Valid {
		return db.IssueCurrentPicture{}, ErrInvalidIssueWriteCapability
	}
	qtx := db.New(tx)
	if _, err := qtx.LockLegacyProjectionWorkspaceAdmission(ctx, params.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.IssueCurrentPicture{}, ErrInvalidIssueWriteCapability
		}
		return db.IssueCurrentPicture{}, err
	}
	if _, err := qtx.LockLegacyProjectionIssueAdmission(ctx, db.LockLegacyProjectionIssueAdmissionParams{WorkspaceID: params.WorkspaceID, IssueID: params.IssueID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.IssueCurrentPicture{}, ErrInvalidIssueWriteCapability
		}
		return db.IssueCurrentPicture{}, err
	}
	capability := &IssueWrite{state: &issueWriteState{queries: qtx, workspaceID: params.WorkspaceID, issueID: params.IssueID, active: true}}
	defer func() {
		capability.state.mu.Lock()
		capability.state.active = false
		capability.state.mu.Unlock()
	}()
	return capability.UpsertCurrentPictureCAS(ctx, params)
}

// InIssueWriteTransaction is the application admission API for normalized v2 issue
// workflow mutations. It locks workspace then issue and keeps both locks until
// the callback and commit finish. Escaped IssueWrite values are invalidated.
func InIssueWriteTransaction(
	ctx context.Context,
	starter TxStarter,
	queries *db.Queries,
	workspaceID pgtype.UUID,
	issueID pgtype.UUID,
	write func(*IssueWrite) error,
) error {
	tx, err := starter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := queries.WithTx(tx)
	if _, err := qtx.LockIssueWorkflowV2Admission(ctx, db.LockIssueWorkflowV2AdmissionParams{
		WorkspaceID: workspaceID,
		IssueID:     issueID,
	}); err != nil {
		return err
	}
	capability := &IssueWrite{state: &issueWriteState{queries: qtx, workspaceID: workspaceID, issueID: issueID, active: true}}
	invalidate := func() {
		capability.state.mu.Lock()
		capability.state.active = false
		capability.state.mu.Unlock()
	}
	defer invalidate()
	if err := write(capability); err != nil {
		return err
	}
	// The callback is the complete lifetime of the capability. Invalidate it
	// before commit starts so an escaped value cannot issue work while Commit
	// is blocked or otherwise in progress.
	invalidate()
	return tx.Commit(ctx)
}

// InExistingIssueWriteTransaction admits normalized workflow writes inside a
// transaction owned by a larger operation. It neither commits nor rolls back:
// the caller must propagate every error so its task mutation and the workflow
// snapshot have one outcome. The capability expires when write returns.
func InExistingIssueWriteTransaction(
	ctx context.Context,
	tx pgx.Tx,
	workspaceID pgtype.UUID,
	issueID pgtype.UUID,
	write func(*IssueWrite) error,
) error {
	if tx == nil || !workspaceID.Valid || !issueID.Valid || write == nil {
		return ErrInvalidIssueWriteCapability
	}
	qtx := db.New(tx)
	if _, err := qtx.LockIssueWorkflowV2Admission(ctx, db.LockIssueWorkflowV2AdmissionParams{
		WorkspaceID: workspaceID,
		IssueID:     issueID,
	}); err != nil {
		return err
	}
	capability := &IssueWrite{state: &issueWriteState{queries: qtx, workspaceID: workspaceID, issueID: issueID, active: true}}
	invalidate := func() {
		capability.state.mu.Lock()
		capability.state.active = false
		capability.state.mu.Unlock()
	}
	defer invalidate()
	if err := write(capability); err != nil {
		return err
	}
	invalidate()
	return nil
}

// InExistingTaskContinuationTransaction is the stricter caller-owned variant
// for daemon task rotation. It preserves the global workspace -> agent -> issue
// -> runtime lock order and proves the actor is still bound to the exact runtime
// before exposing the admitted issue capability.
func InExistingTaskContinuationTransaction(
	ctx context.Context,
	tx pgx.Tx,
	workspaceID, issueID, actorID, runtimeID pgtype.UUID,
	write func(*IssueWrite) error,
) error {
	if tx == nil || !workspaceID.Valid || !issueID.Valid || !actorID.Valid || !runtimeID.Valid || write == nil {
		return ErrInvalidIssueWriteCapability
	}
	qtx := db.New(tx)
	if _, err := qtx.LockWorkflowReleaseWorkspaceAdmission(ctx, workspaceID); err != nil {
		return err
	}
	actor, err := qtx.LockWorkflowReleaseAdmissionActor(ctx, db.LockWorkflowReleaseAdmissionActorParams{WorkspaceID: workspaceID, AgentID: actorID})
	if err != nil || actor.RuntimeID != runtimeID {
		return ErrInvalidIssueWriteCapability
	}
	if _, err := qtx.LockWorkflowReleaseIssueAdmission(ctx, db.LockWorkflowReleaseIssueAdmissionParams{WorkspaceID: workspaceID, IssueID: issueID}); err != nil {
		return err
	}
	if _, err := qtx.LockWorkflowReleaseAdmissionRuntime(ctx, db.LockWorkflowReleaseAdmissionRuntimeParams{WorkspaceID: workspaceID, RuntimeID: runtimeID}); err != nil {
		return err
	}
	capability := &IssueWrite{state: &issueWriteState{queries: qtx, workspaceID: workspaceID, issueID: issueID, active: true}}
	invalidate := func() {
		capability.state.mu.Lock()
		capability.state.active = false
		capability.state.mu.Unlock()
	}
	defer invalidate()
	if err := write(capability); err != nil {
		return err
	}
	invalidate()
	return nil
}

// InReleaseAdmissionTransaction is the only admission path for commands that
// freeze release operator/verifier authority. It preserves the global
// workspace -> agents -> issue lock order used by archival and task creation.
func InReleaseAdmissionTransaction(ctx context.Context, starter TxStarter, queries *db.Queries, workspaceID, issueID pgtype.UUID, actorIDs []pgtype.UUID, write func(*IssueWrite) error) error {
	if len(actorIDs) != 2 || !actorIDs[0].Valid || !actorIDs[1].Valid || actorIDs[0] == actorIDs[1] {
		return ErrInvalidIssueWriteCapability
	}
	actors := append([]pgtype.UUID(nil), actorIDs...)
	sort.Slice(actors, func(i, j int) bool { return bytes.Compare(actors[i].Bytes[:], actors[j].Bytes[:]) < 0 })
	tx, err := starter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := queries.WithTx(tx)
	if _, err := qtx.LockWorkflowReleaseWorkspaceAdmission(ctx, workspaceID); err != nil {
		return err
	}
	runtimeIDs := make([]pgtype.UUID, 0, len(actors))
	for _, actorID := range actors {
		actor, err := qtx.LockWorkflowReleaseAdmissionActor(ctx, db.LockWorkflowReleaseAdmissionActorParams{WorkspaceID: workspaceID, AgentID: actorID})
		if err != nil {
			return err
		}
		runtimeIDs = append(runtimeIDs, actor.RuntimeID)
	}
	if _, err := qtx.LockWorkflowReleaseIssueAdmission(ctx, db.LockWorkflowReleaseIssueAdmissionParams{WorkspaceID: workspaceID, IssueID: issueID}); err != nil {
		return err
	}
	sort.Slice(runtimeIDs, func(i, j int) bool { return bytes.Compare(runtimeIDs[i].Bytes[:], runtimeIDs[j].Bytes[:]) < 0 })
	for index, runtimeID := range runtimeIDs {
		if index > 0 && runtimeID == runtimeIDs[index-1] {
			continue
		}
		if _, err := qtx.LockWorkflowReleaseAdmissionRuntime(ctx, db.LockWorkflowReleaseAdmissionRuntimeParams{WorkspaceID: workspaceID, RuntimeID: runtimeID}); err != nil {
			return err
		}
	}
	capability := &IssueWrite{state: &issueWriteState{queries: qtx, workspaceID: workspaceID, issueID: issueID, active: true, releaseActors: map[pgtype.UUID]struct{}{actors[0]: {}, actors[1]: {}}}}
	invalidate := func() { capability.state.mu.Lock(); capability.state.active = false; capability.state.mu.Unlock() }
	defer invalidate()
	if err := write(capability); err != nil {
		return err
	}
	invalidate()
	return tx.Commit(ctx)
}
