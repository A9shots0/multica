package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowguard"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *TaskService) buildLegacyWorkflowTaskContext(ctx context.Context, issue db.Issue, target db.Agent, squadID pgtype.UUID, sourceTask db.AgentTaskQueue, eventID pgtype.UUID) ([]byte, error) {
	if err := ValidateLegacyWorkflowTargetConfiguration(target); err != nil {
		return nil, err
	}
	event, err := s.Queries.GetIssueWorkflowEvent(ctx, db.GetIssueWorkflowEventParams{
		ID: eventID, IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("load workflow event: %w", err)
	}
	plan, err := s.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("load workflow plan: %w", err)
	}
	runtime, err := s.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
		ID: target.RuntimeID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("load workflow runtime: %w", err)
	}
	_, refs, err := s.LoadAgentSkillBundles(ctx, target.ID, target.SystemKey.String, false)
	if err != nil {
		return nil, fmt.Errorf("snapshot workflow skills: %w", err)
	}
	requiredSkills := make([]workflowguard.RequiredSkill, 0, len(refs))
	for _, ref := range refs {
		requiredSkills = append(requiredSkills, workflowguard.RequiredSkill{
			ID: ref.ID, Source: ref.Source, SHA256: ref.Hash,
			SizeBytes: ref.SizeBytes, FileCount: ref.FileCount,
		})
	}
	acceptanceCriteria := json.RawMessage(issue.AcceptanceCriteria)
	if len(acceptanceCriteria) == 0 || !json.Valid(acceptanceCriteria) {
		acceptanceCriteria = json.RawMessage(`[]`)
	}
	payload := json.RawMessage(event.Payload)
	if len(payload) == 0 || !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	description := ""
	if issue.Description.Valid {
		description = issue.Description.String
	}
	return workflowguard.WrapLegacyAssignment(workflowguard.LegacyAssignmentV1{
		ContractVersion: workflowguard.LegacyAssignmentVersion,
		TaskKind:        legacyWorkflowTaskKind(plan, target.ID, event.EventType),
		IssueID:         util.UUIDToString(issue.ID), IssueTitle: issue.Title, IssueDescription: description,
		AcceptanceCriteria: acceptanceCriteria,
		SourceTaskID:       util.UUIDToString(sourceTask.ID), WorkflowEventID: util.UUIDToString(event.ID),
		WorkflowEventType: event.EventType, WorkflowEventReason: event.Reason, WorkflowEventPayload: payload,
		TargetAgentID: util.UUIDToString(target.ID), SquadID: util.UUIDToString(squadID),
		RuntimeID: util.UUIDToString(target.RuntimeID), RuntimeProvider: runtime.Provider,
		Model: target.Model.String, ReasoningEffort: target.ThinkingLevel.String,
		ContextPolicy:   workflowguard.ContextPolicy{},
		ExecutionPolicy: workflowguard.LegacyExecutionPolicyForAcceptance(acceptanceCriteria),
		RequiredSkills:  requiredSkills,
	})
}

// Legacy assignments seal an explicit provider configuration. Ordinary tasks
// may use runtime defaults, but those defaults cannot satisfy this contract.
// Check before accepting a handoff as well as when reconstructing its task.
func ValidateLegacyWorkflowTargetConfiguration(target db.Agent) error {
	if !target.Model.Valid || strings.TrimSpace(target.Model.String) == "" {
		return errors.New("legacy workflow successor requires an explicit model; use ordinary delegation for a runtime-default agent")
	}
	if !target.ThinkingLevel.Valid || strings.TrimSpace(target.ThinkingLevel.String) == "" {
		return errors.New("legacy workflow successor requires an explicit reasoning effort; use ordinary delegation for a runtime-default agent")
	}
	return nil
}

// ValidateLegacyWorkflowAssignmentSnapshot reconstructs every non-skill field
// from authoritative rows. The task context is mutable JSONB, so identifier
// checks alone are insufficient: any changed prompt-bearing field must make
// claim admission fail closed.
func (s *TaskService) ValidateLegacyWorkflowAssignmentSnapshot(ctx context.Context, task db.AgentTaskQueue, assignment workflowguard.LegacyAssignmentV1) error {
	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return fmt.Errorf("load workflow assignment issue: %w", err)
	}
	event, err := s.Queries.GetIssueWorkflowEvent(ctx, db.GetIssueWorkflowEventParams{
		ID: task.TriggerEvidenceRefID, IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("load workflow assignment event: %w", err)
	}
	plan, err := s.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("load workflow assignment plan: %w", err)
	}
	target, err := s.Queries.GetAgent(ctx, task.AgentID)
	if err != nil || target.RuntimeID != task.RuntimeID {
		return errors.New("workflow assignment target runtime changed")
	}
	runtime, err := s.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
		ID: task.RuntimeID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("load workflow assignment runtime: %w", err)
	}
	acceptanceCriteria := json.RawMessage(issue.AcceptanceCriteria)
	if len(acceptanceCriteria) == 0 || !json.Valid(acceptanceCriteria) {
		acceptanceCriteria = json.RawMessage(`[]`)
	}
	payload := json.RawMessage(event.Payload)
	if len(payload) == 0 || !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	description := ""
	if issue.Description.Valid {
		description = issue.Description.String
	}
	expected := workflowguard.LegacyAssignmentV1{
		ContractVersion: workflowguard.LegacyAssignmentVersion,
		TaskKind:        legacyWorkflowTaskKind(plan, task.AgentID, event.EventType),
		IssueID:         util.UUIDToString(issue.ID), IssueTitle: issue.Title, IssueDescription: description,
		AcceptanceCriteria: acceptanceCriteria,
		SourceTaskID:       util.UUIDToString(task.DelegatedFromTaskID), WorkflowEventID: util.UUIDToString(event.ID),
		WorkflowEventType: event.EventType, WorkflowEventReason: event.Reason, WorkflowEventPayload: payload,
		TargetAgentID: util.UUIDToString(task.AgentID), SquadID: util.UUIDToString(task.SquadID),
		RuntimeID: util.UUIDToString(task.RuntimeID), RuntimeProvider: runtime.Provider,
		Model: target.Model.String, ReasoningEffort: target.ThinkingLevel.String,
		ContextPolicy:   workflowguard.ContextPolicy{},
		ExecutionPolicy: workflowguard.LegacyExecutionPolicyForAcceptance(acceptanceCriteria),
	}
	if !workflowguard.MatchesAuthoritativeSnapshot(assignment, expected) {
		return errors.New("workflow assignment no longer matches authoritative issue, event, plan, or task state")
	}
	return nil
}

func legacyWorkflowTaskKind(plan db.IssueWorkflowPlan, targetID pgtype.UUID, eventType string) string {
	if eventType == "owner_decision_recorded" {
		return "owner_resume"
	}
	switch targetID {
	case plan.ReviewerID:
		return "review"
	case plan.ReleaseAgentID:
		return "release"
	case plan.VerifierID:
		return "verification"
	case plan.ImplementerID:
		if plan.CurrentState == "correction" {
			return "correction"
		}
		return "implementation"
	default:
		return "owner_resume"
	}
}

func (s *TaskService) isLegacyWorkflowControlledTask(ctx context.Context, taskID pgtype.UUID) bool {
	// A repeated terminal callback cannot enqueue a retry or recovery and does
	// not need workflow ownership resolution. Besides avoiding unnecessary plan
	// locking, this preserves the existing idempotent finalization contract for
	// callers backed by minimal task-only stores.
	if task, err := s.Queries.GetAgentTask(ctx, taskID); err == nil {
		switch task.Status {
		case "completed", "failed", "cancelled":
			return false
		}
	}
	if _, err := s.Queries.GetIssueWorkflowPlanByTask(ctx, taskID); err == nil {
		return true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// A plan lookup failure must not unlock generic retry/recovery fan-out.
		slog.Warn("workflow plan ownership lookup failed; suppressing generic recovery", "task_id", util.UUIDToString(taskID), "error", err)
		task, taskErr := s.Queries.GetAgentTask(ctx, taskID)
		return taskErr == nil && task.SquadID.Valid && !task.WorkflowActionID.Valid
	}
	return false
}

// isLegacyWorkflowControlledTerminalTask is the sweeper-safe ownership check.
// HandleFailedTasks receives rows after SQL has already made them terminal, so
// the idempotent live-callback shortcut above intentionally cannot be used.
func (s *TaskService) isLegacyWorkflowControlledTerminalTask(ctx context.Context, task db.AgentTaskQueue) bool {
	return isLegacyWorkflowControlledTerminalTaskWithQueries(ctx, s.Queries, task)
}

func isLegacyWorkflowControlledTerminalTaskWithQueries(ctx context.Context, queries *db.Queries, task db.AgentTaskQueue) bool {
	if task.WorkflowActionID.Valid || !task.IssueID.Valid || !task.SquadID.Valid {
		return false
	}
	if _, err := queries.GetIssueWorkflowPlanByTask(ctx, task.ID); err == nil {
		return true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// A transient ownership lookup error must suppress generic fan-out. It is
		// safer to leave a failed squad task stopped than to spend another run.
		slog.Warn("terminal workflow ownership lookup failed; suppressing generic recovery", "task_id", util.UUIDToString(task.ID), "error", err)
		return true
	}
	return false
}

func (s *TaskService) stopLegacyWorkflowAfterFailure(ctx context.Context, task db.AgentTaskQueue, reason string) {
	s.stopLegacyWorkflow(ctx, task, reason, false, nil)
}

// stopLegacyWorkflowAfterMissingDispatch raises the same owner-visible
// technical stop for a task that did advance the workflow, but whose accepted
// hand-off never produced the next task and could not be repaired.
func (s *TaskService) stopLegacyWorkflowAfterMissingDispatch(ctx context.Context, task db.AgentTaskQueue, reason string) {
	s.stopLegacyWorkflow(ctx, task, reason, true, nil)
}

// stopLegacyWorkflowAfterProviderCeiling turns a final, refused provider start
// into durable owner-visible truth. A concurrently admitted task still carries
// the workflow, so its losing sibling must not block the plan underneath it.
func (s *TaskService) stopLegacyWorkflowAfterProviderCeiling(ctx context.Context, task db.AgentTaskQueue, expectedPlanRevision int64) {
	if !task.IssueID.Valid || !task.SquadID.Valid {
		return
	}
	active, err := s.Queries.ListActiveTasksByIssue(ctx, task.IssueID)
	if err != nil {
		slog.Warn("workflow carrier lookup failed after provider ceiling; attempting durable stop",
			"task_id", util.UUIDToString(task.ID), "error", err)
	} else {
		for _, candidate := range active {
			if candidate.SquadID == task.SquadID {
				return
			}
		}
	}
	s.stopLegacyWorkflow(ctx, task,
		"Multica reached its automatic recovery limit before another agent could start. The issue was stopped instead of being left falsely marked as working.",
		false, &expectedPlanRevision)
}

func (s *TaskService) stopLegacyWorkflow(ctx context.Context, task db.AgentTaskQueue, reason string, advancedWithoutDispatch bool, expectedPlanRevision *int64) {
	if s.TxStarter == nil {
		slog.Error("stop legacy workflow requires transaction support", "task_id", util.UUIDToString(task.ID))
		return
	}
	var issue db.Issue
	var picture db.IssueCurrentPicture
	var ownerItem *db.InboxItem
	storedProjection := false
	err := s.runInTxWithHandle(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		if !task.IssueID.Valid {
			return nil
		}
		loadedIssue, err := qtx.GetIssue(ctx, task.IssueID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		issue, err = qtx.LockIssueTaskAdmission(ctx, db.LockIssueTaskAdmissionParams{
			ID: loadedIssue.ID, WorkspaceID: loadedIssue.WorkspaceID,
		})
		if err != nil {
			return err
		}
		plan, err := qtx.GetIssueWorkflowPlanByTask(ctx, task.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		plan, err = qtx.GetIssueWorkflowPlanForUpdate(ctx, db.GetIssueWorkflowPlanForUpdateParams{
			IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID,
		})
		if err != nil {
			return err
		}
		// A final-slot carrier can advance or stop the plan between the denied
		// start's commit and this follow-up transaction. Never overwrite that
		// newer controller truth with a stale ceiling observation.
		if expectedPlanRevision != nil && plan.Revision != *expectedPlanRevision {
			return nil
		}
		advanced, err := qtx.HasAdvancingIssueWorkflowEventForTask(ctx, task.ID)
		if err != nil || (advanced && !advancedWithoutDispatch && len(plan.OwnerRuntimeRoute) == 0) {
			return err
		}
		if plan.CurrentState != string(workflowstate.Blocked) {
			updatedPlan, updateErr := qtx.UpdateIssueWorkflowPlanState(ctx, db.UpdateIssueWorkflowPlanStateParams{
				CurrentState: string(workflowstate.Blocked), SuspendedState: pgtype.Text{String: plan.CurrentState, Valid: true},
				SuspendedAgentID: task.AgentID, CorrectionCount: plan.CorrectionCount,
				IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID, ExpectedRevision: plan.Revision,
			})
			if updateErr != nil {
				return updateErr
			}
			plan = updatedPlan
		}
		if plan.IssueID != issue.ID || plan.WorkspaceID != issue.WorkspaceID {
			return errors.New("workflow plan issue scope mismatch")
		}
		hasActive, err := qtx.HasActiveTaskForIssue(ctx, issue.ID)
		if err != nil {
			return err
		}
		if hasActive {
			return nil
		}
		if issue.Status != "blocked" {
			if _, err := qtx.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: issue.ID, Status: "blocked", WorkspaceID: issue.WorkspaceID}); err != nil {
				return err
			}
		}
		if len(plan.OwnerRuntimeRoute) > 0 {
			if err := qtx.RetireStoppedOwnerRuntimeQuestions(ctx, db.RetireStoppedOwnerRuntimeQuestionsParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); err != nil {
				return err
			}
			picture, ownerItem, err = s.StoreOwnerRuntimeProjection(ctx, tx, plan, task, reason)
			storedProjection = err == nil
			return err
		}
		params, err := workflowprojection.LegacyTechnicalStopParams(issue.WorkspaceID, issue.ID, task.ID, reason, time.Now().UTC())
		if err != nil {
			return err
		}
		picture, err = workflowstore.UpsertLegacyCurrentPicture(ctx, tx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			// A normalized v2/v3 picture already owns projection truth.
			return nil
		}
		if err != nil {
			return err
		}
		if s.AdmitLegacyWorkflowProjection == nil {
			return errors.New("legacy workflow owner Inbox admission is unavailable")
		}
		ownerItem, err = s.AdmitLegacyWorkflowProjection(ctx, qtx, issue.WorkspaceID, issue.ID, picture.Revision)
		if err != nil {
			return err
		}
		storedProjection = true
		return nil
	})
	if err != nil {
		slog.Error("stop legacy workflow after terminal task failure", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}
	if !storedProjection {
		return
	}
	if s.OnLegacyWorkflowProjection != nil {
		s.OnLegacyWorkflowProjection(ctx, issue.WorkspaceID, issue.ID, picture.Revision, ownerItem)
	}
}
