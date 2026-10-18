package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// taskStatusActive reports whether a task is still in a state that can carry
// authority (claimed or claimable). Terminal states (completed / cancelled /
// failed / escalated) hold no authority to route further work.
func taskStatusActive(status string) bool {
	switch status {
	case "queued", "dispatched", "running", "waiting_local_directory", "deferred":
		return true
	default:
		return false
	}
}

// planStateActive reports whether a workflow plan is in a state whose current
// leader task may decompose work. Suspended, completed, or otherwise-stopped
// plans must not admit new children or dispatches.
func planStateActive(currentState string) bool {
	switch currentState {
	case "working", "queued", "correction":
		return true
	default:
		return false
	}
}

// planBoundAgent reports whether agentID is one of the pipeline roles the
// workflow plan names for the issue. Verifier and release-agent are only
// required for issues that declare those stages, so an unset optional role
// does not bind.
func planBoundAgent(plan db.IssueWorkflowPlan, agentID pgtype.UUID) bool {
	if !agentID.Valid {
		return false
	}
	if plan.ImplementerID.Valid && plan.ImplementerID == agentID {
		return true
	}
	if plan.ReviewerID.Valid && plan.ReviewerID == agentID {
		return true
	}
	if plan.ReleaseAgentID.Valid && plan.ReleaseAgentID == agentID {
		return true
	}
	if plan.VerifierID.Valid && plan.VerifierID == agentID {
		return true
	}
	return false
}

// currentLeaderTaskAdmits checks that sourceTaskID is the ACTIVE current
// squad-leader task for issue under plan: it exists, carries the leader flag,
// belongs to the issue the plan governs, runs as that squad's canonical leader,
// and is still in an active task state. This is what keeps delegation tied to
// the exact run the deterministic controller released — a stale, foreign,
// worker, or finished task grants nothing.
func currentLeaderTaskAdmits(ctx context.Context, q *db.Queries, issue db.Issue, plan db.IssueWorkflowPlan, sourceTaskID pgtype.UUID) error {
	if !sourceTaskID.Valid {
		return ErrAgentDelegationRequiresLeaderTask
	}
	if !issue.AssigneeType.Valid || issue.AssigneeType.String != "squad" ||
		!issue.AssigneeID.Valid || !plan.SquadID.Valid || issue.AssigneeID != plan.SquadID {
		return ErrAgentDelegationRequiresLeaderTask
	}
	task, err := q.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{
		ID: sourceTaskID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAgentDelegationRequiresLeaderTask
		}
		return fmt.Errorf("load acting agent task: %w", err)
	}
	squad, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
		ID: plan.SquadID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAgentDelegationRequiresLeaderTask
		}
		return fmt.Errorf("load workflow squad: %w", err)
	}
	if task.AgentID != squad.LeaderID || !task.IsLeaderTask || !taskActiveForIssue(task, issue, plan) {
		return ErrAgentDelegationRequiresLeaderTask
	}
	return nil
}

func taskActiveForIssue(task db.AgentTaskQueue, issue db.Issue, plan db.IssueWorkflowPlan) bool {
	if !task.IssueID.Valid || task.IssueID != issue.ID {
		return false
	}
	if !plan.SquadID.Valid || !task.SquadID.Valid || task.SquadID != plan.SquadID {
		return false
	}
	return taskStatusActive(task.Status)
}

// admitAgentIssueCreate applies the workspace's issue-creation policy. It is
// deliberately policy-aware rather than a global agent ban: workspaces with
// the setting disabled keep all existing agent-origin and private-squad flows,
// while an enabled workspace requires every root and child issue to come from
// a member. The database trigger remains the race-proof enforcement layer.
func admitAgentIssueCreate(requireMember bool, creatorType string) error {
	if requireMember && creatorType == "agent" {
		return ErrAgentIssueCreationDisabled
	}
	return nil
}

// AdmitSquadWorkerDelegation is the dispatch-side workflow-plan gate.
// A comment trigger that hands work to a named agent on a squad-owned issue is
// admitted only when a durable workflow plan exists, the delegating comment was
// authored by the issue's active current squad-leader task, and the target
// agent is a plan-bound pipeline role. Ordinary non-squad issues keep their
// legacy routing unchanged.
func AdmitSquadWorkerDelegation(ctx context.Context, q *db.Queries, issue db.Issue, sourceTaskID pgtype.UUID, targetAgentID pgtype.UUID) error {
	// Member and system comments carry their own authority. The admission gate
	// is specifically for task-scoped agent delegation.
	if !sourceTaskID.Valid {
		return nil
	}
	// Only squad-owned work requires a squad workflow plan. Ordinary issues keep
	// their existing agent-to-agent mention behavior.
	if !issue.AssigneeType.Valid || issue.AssigneeType.String != "squad" || !issue.AssigneeID.Valid {
		return nil
	}
	plan, err := q.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrParentWorkflowPlanRequired
		}
		return fmt.Errorf("load issue workflow plan: %w", err)
	}
	if !planStateActive(plan.CurrentState) {
		return ErrParentWorkflowPlanNotActive
	}
	if err := currentLeaderTaskAdmits(ctx, q, issue, plan, sourceTaskID); err != nil {
		return err
	}
	if !planBoundAgent(plan, targetAgentID) {
		return ErrWorkerNotBoundToWorkflowPlan
	}
	return nil
}
