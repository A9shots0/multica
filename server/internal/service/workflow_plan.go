package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	workflowControllerInboxType      = "workflow_controller_unavailable"
	workflowControllerRecoveryAction = "retry_controller_recovery"
	workflowPlanReconcileLimit       = 100
	workflowPlanReconcileInterval    = 30 * time.Second
)

// EnsureSquadWorkflowPlan installs a configured squad's stable execution path
// before its leader receives the issue. Squads without a template retain the
// stock Multica behavior. An existing per-issue plan is never reset.
func EnsureSquadWorkflowPlan(ctx context.Context, q *db.Queries, issue db.Issue) (bool, error) {
	if !issue.AssigneeType.Valid || issue.AssigneeType.String != "squad" || !issue.AssigneeID.Valid {
		return false, nil
	}
	if plan, err := q.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); err == nil && len(plan.OwnerRuntimeRoute) > 0 {
		_, err = ParseOwnerRuntimeRoute(plan)
		return true, err
	}
	_, err := q.GetSquadWorkflowTemplate(ctx, db.GetSquadWorkflowTemplateParams{
		SquadID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	_, err = q.InstallIssueWorkflowPlanFromSquadTemplate(ctx, db.InstallIssueWorkflowPlanFromSquadTemplateParams{
		IssueID: issue.ID, SquadID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = q.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{
			IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
		})
	}
	if err != nil {
		return true, err
	}
	return true, nil
}

// EnsureWorkflowController is the single pre-dispatch guard for squad work.
// A squad route without a template is not ordinary work: it has no deterministic
// controller to own recovery, so fail closed and leave an owner-visible action.
func EnsureWorkflowController(ctx context.Context, q *db.Queries, bus *events.Bus, issue db.Issue) (bool, error) {
	configured, err := EnsureSquadWorkflowPlan(ctx, q, issue)
	if err != nil {
		_ = SurfaceWorkflowControllerFailure(ctx, q, bus, issue, err)
		return configured, fmt.Errorf("ensure workflow controller: %w", err)
	}
	if issue.AssigneeType.Valid && issue.AssigneeType.String == "squad" && !configured {
		cause := errors.New("no workflow controller template is configured for the assigned squad")
		if surfaceErr := SurfaceWorkflowControllerFailure(ctx, q, bus, issue, cause); surfaceErr != nil {
			return false, fmt.Errorf("surface missing workflow controller: %w", surfaceErr)
		}
		return false, cause
	}
	if configured {
		_ = RecoverWorkflowController(ctx, q, bus, issue)
	}
	return configured, nil
}

// SurfaceWorkflowControllerFailure moves the issue to the explicit blocked
// projection and admits one owner-visible Inbox action. The stable recovery
// key makes retries and concurrent reconcilers harmless.
func SurfaceWorkflowControllerFailure(ctx context.Context, q *db.Queries, bus *events.Bus, issue db.Issue, cause error) error {
	if !issue.ID.Valid || !issue.WorkspaceID.Valid {
		return errors.New("workflow controller failure requires an issue identity")
	}
	previousStatus := issue.Status
	if previousStatus == "blocked" {
		// A pre-existing blocked issue has no safe status to restore when the
		// controller is eventually repaired. Keep it blocked until the owner or
		// workflow explicitly advances it.
		previousStatus = ""
	}
	if issue.Status != "blocked" {
		if _, err := q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID, Status: "blocked"}); err != nil {
			return fmt.Errorf("block issue after workflow controller failure: %w", err)
		}
	}

	recipients, err := q.ListWorkspaceManagerUserIDs(ctx, issue.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list workflow controller recovery recipients: %w", err)
	}
	details, _ := json.Marshal(map[string]string{
		"owner_relevant":  "true",
		"owner_category":  "incident",
		"required_action": workflowControllerRecoveryAction,
		"recovery_key":    workflowControllerRecoveryKey(issue),
		"previous_status": previousStatus,
		"evidence":        cause.Error(),
	})
	for _, recipientID := range recipients {
		_, lookupErr := q.GetActiveWorkflowControllerInbox(ctx, db.GetActiveWorkflowControllerInboxParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
		if lookupErr == nil {
			continue
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return fmt.Errorf("check workflow controller recovery notice: %w", lookupErr)
		}
		item, createErr := q.CreateInboxItem(ctx, db.CreateInboxItemParams{
			ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID,
			RecipientType: "member", RecipientID: recipientID,
			Type: workflowControllerInboxType, Severity: "action_required", IssueID: issue.ID,
			Title:     issue.Title,
			Body:      pgtypeText("The deterministic workflow controller is unavailable. Retry recovery after the evidence below is resolved."),
			ActorType: pgtypeText("system"), ActorID: pgtype.UUID{}, Details: details,
		})
		if createErr != nil {
			return fmt.Errorf("create workflow controller recovery notice: %w", createErr)
		}
		if bus != nil {
			bus.Publish(events.Event{Type: protocol.EventInboxNew, WorkspaceID: util.UUIDToString(issue.WorkspaceID), Payload: map[string]any{"item": item}})
		}
		break
	}
	return nil
}

// RecoverWorkflowController clears a previous recovery notice only after a
// plan is present. It restores the exact pre-failure board status recorded in
// the notice and never overwrites a later human status change.
func RecoverWorkflowController(ctx context.Context, q *db.Queries, bus *events.Bus, issue db.Issue) error {
	item, err := q.GetActiveWorkflowControllerInbox(ctx, db.GetActiveWorkflowControllerInboxParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load workflow controller recovery notice: %w", err)
	}
	var details struct {
		PreviousStatus string `json:"previous_status"`
	}
	_ = json.Unmarshal(item.Details, &details)
	if issue.Status == "blocked" && details.PreviousStatus != "" {
		if _, err := q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID, Status: details.PreviousStatus}); err != nil {
			return fmt.Errorf("restore issue status after workflow controller recovery: %w", err)
		}
	}
	if _, err := q.ArchiveWorkflowControllerInbox(ctx, db.ArchiveWorkflowControllerInboxParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID}); err != nil {
		return fmt.Errorf("archive workflow controller recovery notice: %w", err)
	}
	if bus != nil {
		bus.Publish(events.Event{Type: protocol.EventInboxArchived, WorkspaceID: util.UUIDToString(issue.WorkspaceID), Payload: map[string]any{"issue_id": util.UUIDToString(issue.ID), "item_id": util.UUIDToString(item.ID)}})
	}
	return nil
}

func workflowControllerRecoveryKey(issue db.Issue) string {
	return util.UUIDToString(issue.WorkspaceID) + ":" + util.UUIDToString(issue.ID)
}

// The generated pgtype values below keep this package independent of handler
// response types while making the Inbox write explicit and nullable-safe.
func pgtypeText(value string) pgtype.Text { return pgtype.Text{String: value, Valid: true} }

// WorkflowPlanReconciler repairs plans that were missing before this guard was
// installed (or whose enqueue transaction was interrupted). A single process
// may run many passes; unique plan rows, pending-task deduplication, and the
// stable Inbox recovery key make replay safe across restarts and replicas.
type WorkflowPlanReconciler struct {
	Queries     *db.Queries
	TaskService *TaskService
	Bus         *events.Bus
	Interval    time.Duration
}

func (r *WorkflowPlanReconciler) Run(ctx context.Context) {
	interval := r.Interval
	if interval <= 0 {
		interval = workflowPlanReconcileInterval
	}
	for {
		if err := r.ReconcileOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("workflow plan reconciliation failed", "error", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (r *WorkflowPlanReconciler) ReconcileOnce(ctx context.Context) error {
	if r == nil || r.Queries == nil {
		return errors.New("workflow plan reconciler is not configured")
	}
	if r.TaskService != nil {
		if err := r.TaskService.recoverOwnerRuntimeCompletions(ctx, pgtype.UUID{}); err != nil {
			return err
		}
	}
	issues, err := r.Queries.ListIssuesMissingWorkflowPlan(ctx, int32(workflowPlanReconcileLimit))
	if err != nil {
		return err
	}
	var firstErr error
	for _, issue := range issues {
		configured, ensureErr := EnsureWorkflowController(ctx, r.Queries, r.Bus, issue)
		if ensureErr != nil {
			if firstErr == nil {
				firstErr = ensureErr
			}
			continue
		}
		if !configured || r.TaskService == nil {
			continue
		}
		if enqueueErr := r.resumeIssue(ctx, issue); enqueueErr != nil && !errors.Is(enqueueErr, ErrDuplicatePendingTask) {
			_ = SurfaceWorkflowControllerFailure(ctx, r.Queries, r.Bus, issue, enqueueErr)
			if firstErr == nil {
				firstErr = enqueueErr
			}
		}
	}
	return firstErr
}

func (r *WorkflowPlanReconciler) resumeIssue(ctx context.Context, issue db.Issue) error {
	squad, err := r.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return err
	}
	plan, err := r.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return err
	}
	if plan.CurrentState != "working" && plan.CurrentState != "queued" && plan.CurrentState != "correction" {
		return nil
	}
	hasPending, err := r.Queries.HasPendingTaskForIssueAndAgent(ctx, db.HasPendingTaskForIssueAndAgentParams{IssueID: issue.ID, AgentID: squad.LeaderID, HeadSha: headShaText(r.TaskService.ResolveIssueReviewSHA(ctx, issue.ID))})
	if err != nil || hasPending {
		return err
	}
	_, err = r.TaskService.EnqueueTaskForSquadLeader(ctx, issue, squad.LeaderID, squad.ID, pgtype.UUID{})
	return err
}
