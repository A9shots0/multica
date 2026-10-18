package workflowstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// InOperatorCompletionTransaction serializes completion requests for this issue
// before taking owner row locks. Exact replays read the first committed receipt.
// Workspace admission then precedes nonblocking issue and plan locks: legacy
// writers do not acquire the completion key, so their lock contention still
// yields without a cycle with existing plan-first writers.
func InOperatorCompletionTransaction(ctx context.Context, starter TxStarter, queries *db.Queries, workspaceID, issueID pgtype.UUID, operation func(*IssueWrite) error) error {
	tx, err := starter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := queries.WithTx(tx)
	if err := qtx.LockOperatorCompletionAdmission(ctx, db.LockOperatorCompletionAdmissionParams{WorkspaceID: workspaceID, IssueID: issueID}); err != nil {
		return err
	}
	if _, err := qtx.LockWorkflowReleaseWorkspaceAdmission(ctx, workspaceID); err != nil {
		return err
	}
	if _, err := qtx.LockIssueForOperatorCompletion(ctx, db.LockIssueForOperatorCompletionParams{ID: issueID, WorkspaceID: workspaceID}); err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "55P03" {
			return ErrOperatorCompletionBusy
		}
		return err
	}
	write := &IssueWrite{state: &issueWriteState{queries: qtx, workspaceID: workspaceID, issueID: issueID, active: true}}
	invalidate := func() { write.state.mu.Lock(); write.state.active = false; write.state.mu.Unlock() }
	defer invalidate()
	if err := operation(write); err != nil {
		return err
	}
	invalidate()
	return tx.Commit(ctx)
}

type OperatorCompletionSnapshot struct {
	Issue                db.Issue
	Plan                 db.IssueWorkflowPlan
	Review               db.AgentTaskQueue
	ReviewTrigger        *db.Comment
	Comment              db.Comment
	Picture              *db.IssueCurrentPicture
	Existing             *db.IssueWorkflowEvent
	ActiveTasks          bool
	Normalized           bool
	PendingOwnerDecision bool
}

var ErrOperatorCompletionBusy = errors.New("workflow is changing; refresh its current state and retry completion")

func (write *IssueWrite) ReadOperatorCompletion(ctx context.Context, workspaceID, issueID, reviewID, commentID pgtype.UUID) (OperatorCompletionSnapshot, error) {
	finish, err := write.beginOperation(ctx, workspaceID, issueID)
	if err != nil {
		return OperatorCompletionSnapshot{}, err
	}
	defer finish()
	q := write.state.queries
	var s OperatorCompletionSnapshot
	s.Issue, err = q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: workspaceID})
	if err != nil {
		return s, err
	}
	s.Plan, err = q.GetIssueWorkflowPlanForCompletion(ctx, db.GetIssueWorkflowPlanForCompletionParams{IssueID: issueID, WorkspaceID: workspaceID})
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "55P03" {
			return s, ErrOperatorCompletionBusy
		}
		return s, err
	}
	picture, err := q.GetIssueCurrentPicture(ctx, db.GetIssueCurrentPictureParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err == nil {
		s.Picture = &picture
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return s, err
	}
	existing, err := q.GetIssueWorkflowEventBySource(ctx, db.GetIssueWorkflowEventBySourceParams{SourceTaskID: reviewID, EventType: "operator_completed"})
	if err == nil {
		s.Existing = &existing
		return s, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return s, err
	}
	s.Review, err = q.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: reviewID, WorkspaceID: workspaceID})
	if err != nil {
		return s, err
	}
	if !s.Review.SquadID.Valid && s.Review.TriggerCommentID.Valid {
		trigger, err := q.GetCommentInWorkspace(ctx, db.GetCommentInWorkspaceParams{ID: s.Review.TriggerCommentID, WorkspaceID: workspaceID})
		if err != nil {
			return s, err
		}
		s.ReviewTrigger = &trigger
	}
	s.Comment, err = q.GetCommentInWorkspace(ctx, db.GetCommentInWorkspaceParams{ID: commentID, WorkspaceID: workspaceID})
	if err != nil {
		return s, err
	}
	active, err := q.ListActiveTasksByIssue(ctx, issueID)
	if err != nil {
		return s, err
	}
	s.ActiveTasks = len(active) != 0
	s.PendingOwnerDecision, err = q.HasUnansweredLegacyWorkflowDecision(ctx, db.HasUnansweredLegacyWorkflowDecisionParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return s, err
	}
	decisions, err := q.ListActiveIssueWorkflowDecisions(ctx, db.ListActiveIssueWorkflowDecisionsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return s, err
	}
	inputs, err := q.ListIssueWorkflowProjectionInputs(ctx, db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return s, err
	}
	heads, err := q.ListIssueWorkflowConcernHeads(ctx, db.ListIssueWorkflowConcernHeadsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return s, err
	}
	s.Normalized = len(inputs) != 0 || len(heads) != 0 || len(decisions) != 0
	return s, nil
}

// StoreOperatorCompletion keeps the explicit operator event, issue, plan,
// Current Picture, resolved incident Inbox rows, and completion Inbox admission
// in the same scoped transaction. It never changes historical tasks or events.
func (write *IssueWrite) StoreOperatorCompletion(ctx context.Context, event db.CreateIssueWorkflowEventParams, plan db.UpdateIssueWorkflowPlanStateParams, picture db.UpsertIssueCurrentPictureParams, admit func(context.Context, *db.Queries, pgtype.UUID, pgtype.UUID, int64) (*db.InboxItem, error)) (db.IssueCurrentPicture, *db.InboxItem, error) {
	finish, err := write.beginOperation(ctx, event.WorkspaceID, event.IssueID)
	if err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	defer finish()
	if plan.WorkspaceID != event.WorkspaceID || plan.IssueID != event.IssueID || picture.WorkspaceID != event.WorkspaceID || picture.IssueID != event.IssueID || admit == nil {
		return db.IssueCurrentPicture{}, nil, ErrInvalidIssueWriteCapability
	}
	q := write.state.queries
	if _, err = q.CreateIssueWorkflowEvent(ctx, event); err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	if _, err = q.UpdateIssueWorkflowPlanState(ctx, plan); err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	if _, err = q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: event.IssueID, WorkspaceID: event.WorkspaceID, Status: "done"}); err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	// The public scoped CAS validates the same capability and never exposes q.
	stored, err := write.UpsertCurrentPictureCAS(ctx, picture)
	if err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	if _, err = q.ArchiveInboxByIssueAndType(ctx, db.ArchiveInboxByIssueAndTypeParams{WorkspaceID: event.WorkspaceID, IssueID: event.IssueID, Type: "workflow_incident"}); err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	item, err := admit(ctx, q, event.WorkspaceID, event.IssueID, stored.Revision)
	if err != nil || item == nil {
		if err == nil {
			err = errors.New("operator completion requires durable owner Inbox admission")
		}
		return db.IssueCurrentPicture{}, nil, err
	}
	return stored, item, nil
}
