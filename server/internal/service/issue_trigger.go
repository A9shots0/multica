package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// RunEnqueueSource identifies which kind of issue write would start an agent
// run. It is surfaced in preview responses so the UI can explain each trigger.
type RunEnqueueSource string

const (
	// RunSourceAssign covers issue creation and assignee changes — the issue
	// is being handed to an agent/squad. Parks silently on backlog.
	RunSourceAssign RunEnqueueSource = "assign"
	// RunSourceStatus covers promoting an already-assigned issue out of
	// backlog into an active status.
	RunSourceStatus RunEnqueueSource = "status"
)

// IssueTriggerProbe carries the request-scoped checks WillEnqueueRun cannot
// resolve from issue state alone.
//
// CanAccessAgent is the private-agent gate. The write paths enforce it at the
// HTTP boundary (validateAssigneePair on assign, canEnqueueSquadLeader inside
// the squad enqueue helper) and therefore pass an allow-all probe so the gate
// is never duplicated or sunk into the service layer. Preview passes the real
// gate so it never leaks a private agent's readiness to a member who cannot
// see it. A nil func is treated as allow-all.
//
// IsSelfLoop reports whether promoting this issue out of backlog would be the
// calling agent re-triggering its own running task. Only the status source
// consults it. A nil func means "not a self-loop".
//
// SuppressActiveSelfAssignment remains as an actor-aware diagnostic hook for a
// trusted agent claiming ownership for itself. WillEnqueueRun independently
// enforces the stronger product invariant for every actor: an existing run on
// the issue suppresses a bookkeeping-triggered second run.
type IssueTriggerProbe struct {
	CanAccessAgent               func(agent db.Agent) bool
	IsSelfLoop                   func() bool
	SuppressActiveSelfAssignment func(agentID pgtype.UUID) bool
}

// IssueTriggerInput describes one prospective issue write in its post-write
// shape. AssigneeChanged / StatusChanged mark which fields the write touches;
// IsCreate marks a brand-new issue (no prior task to cancel, no self-loop).
type IssueTriggerInput struct {
	Issue           db.Issue
	PrevStatus      string
	IsCreate        bool
	AssigneeChanged bool
	StatusChanged   bool
}

// IssueRunTrigger is the resolved decision shared by preview and the write
// paths. AgentID is the agent that will actually run — the assignee for an
// agent issue, the squad leader for a squad issue.
type IssueRunTrigger struct {
	IssueID      pgtype.UUID
	AgentID      pgtype.UUID
	AssigneeType string
	Source       RunEnqueueSource
}

// IssueRunDecision is the complete deterministic admission result. Err is set
// when the write otherwise would start work but its explicit worker cannot run
// on the project's approved machine, or when that policy cannot be read
// safely. Callers that mutate issue ownership use it to reject before the write
// instead of silently dropping the enqueue.
type IssueRunDecision struct {
	Trigger     IssueRunTrigger
	WillEnqueue bool
	Err         error
}

func allowAllAgents(db.Agent) bool { return true }

// WillEnqueueRun is the single predicate answering "will this issue write
// start an agent run, and for whom". It is the one source of truth shared by
// the issue update / batch-update write paths and the preview endpoint,
// replacing the per-site copies that drifted (squad omitted, self-loop
// omitted, four entry points inconsistent — see MUL-3375).
//
// It is intentionally a distinct predicate from the comment trigger
// (assignee fallback comment routing): issue writes park in backlog and blocked
// categories while comments have their own explicit-routing rules.
//
// The decision must equal the real enqueue conditions so preview never claims
// a net-new run that the write path then drops. For every non-create write, an
// existing queued or active run anywhere on the issue suppresses the optional
// enqueue. This keeps assignment/status bookkeeping from silently creating a
// second concurrent execution, even when ownership changes to another agent.
func (s *IssueService) WillEnqueueRun(ctx context.Context, in IssueTriggerInput, probe IssueTriggerProbe) (IssueRunTrigger, bool) {
	decision := s.EvaluateIssueRun(ctx, in, probe)
	return decision.Trigger, decision.WillEnqueue
}

// EvaluateIssueRun is WillEnqueueRun with the route-denial reason preserved for
// admission and preview callers. WillEnqueueRun remains the compatibility
// wrapper for post-write dispatch sites that only need the yes/no decision.
func (s *IssueService) EvaluateIssueRun(ctx context.Context, in IssueTriggerInput, probe IssueTriggerProbe) IssueRunDecision {
	issue := in.Issue
	if !issue.AssigneeType.Valid || !issue.AssigneeID.Valid {
		return IssueRunDecision{}
	}
	// Match the task-boundary safety policy in the shared preview/write
	// predicate. A project-less issue remains editable, but it cannot be
	// advertised as starting work when the workspace requires project context.
	if err := validateIssueProjectForAgentWork(ctx, s.Queries, issue); err != nil {
		return IssueRunDecision{}
	}
	canAccess := probe.CanAccessAgent
	if canAccess == nil {
		canAccess = allowAllAgents
	}

	// The status source requires leaving a parking category, not merely changing
	// the status key. Custom statuses inherit either backlog or blocked behavior,
	// so moving between keys in one of those categories cannot start work; moving
	// from either category into an active category can resume it. (MUL-6463)
	//
	// Both sides of the transition are normalized to the canonical status they
	// inherit, so a custom status in the `backlog` category parks exactly like
	// Backlog and a custom status in the `todo` category starts a run exactly
	// like Todo. Built-in keys resolve to themselves without a query, leaving
	// this decision bit-identical for workspaces with no custom statuses —
	// which is the whole set of them until an admin defines one. (MUL-6243)
	currentStatus := issuestatus.Effective(ctx, s.Queries, issue.WorkspaceID, issue.Status)
	prevStatus := issuestatus.Effective(ctx, s.Queries, issue.WorkspaceID, in.PrevStatus)

	var source RunEnqueueSource
	switch {
	case in.IsCreate || in.AssigneeChanged:
		// Backlog and blocked/Needs You are parking states: assigning there
		// records ownership without starting work before the wait is resolved.
		if currentStatus == "backlog" || currentStatus == "blocked" {
			return IssueRunDecision{}
		}
		source = RunSourceAssign
	case in.StatusChanged && (prevStatus == "backlog" || prevStatus == "blocked") &&
		currentStatus != "backlog" &&
		currentStatus != "blocked" &&
		currentStatus != "done" && currentStatus != "cancelled":
		if probe.IsSelfLoop != nil && probe.IsSelfLoop() {
			return IssueRunDecision{}
		}
		source = RunSourceStatus
	default:
		return IssueRunDecision{}
	}

	switch issue.AssigneeType.String {
	case "agent":
		agent, err := s.Queries.GetAgent(ctx, issue.AssigneeID)
		if err != nil || !agent.RuntimeID.Valid || agent.ArchivedAt.Valid {
			return IssueRunDecision{}
		}
		if !canAccess(agent) {
			return IssueRunDecision{}
		}
		if err := validateProjectExecutionAuthority(ctx, s.Queries, issue, agent.ID, agent.RuntimeID); err != nil {
			return IssueRunDecision{Err: err}
		}
		if !in.IsCreate && s.hasActiveRun(ctx, issue.ID) {
			return IssueRunDecision{}
		}
		if source == RunSourceAssign && !in.IsCreate && probe.SuppressActiveSelfAssignment != nil &&
			probe.SuppressActiveSelfAssignment(issue.AssigneeID) {
			return IssueRunDecision{}
		}
		return IssueRunDecision{Trigger: IssueRunTrigger{
			IssueID:      issue.ID,
			AgentID:      issue.AssigneeID,
			AssigneeType: "agent",
			Source:       source,
		}, WillEnqueue: true}

	case "squad":
		// Pair-scoped self-assignment suppression intentionally applies only to
		// direct agent ownership. Assigning a squad changes execution context
		// (leader briefing, roles, and member routing), so even when the acting
		// agent is that squad's leader it is an intentional group handoff rather
		// than a redundant direct self-claim. The status path below still uses
		// the leader's pending-task guard.
		squad, err := s.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
			ID:          issue.AssigneeID,
			WorkspaceID: issue.WorkspaceID,
		})
		if err != nil {
			return IssueRunDecision{}
		}
		leader, err := s.Queries.GetAgent(ctx, squad.LeaderID)
		if err != nil {
			return IssueRunDecision{}
		}
		if !canAccess(leader) {
			return IssueRunDecision{}
		}
		if err := validateProjectExecutionAuthority(ctx, s.Queries, issue, leader.ID, leader.RuntimeID); err != nil {
			return IssueRunDecision{Err: err}
		}
		verdict, err := AgentReadiness(ctx, s.runtimeLookup(s.Queries), leader)
		if err != nil || !verdict.Ready() {
			return IssueRunDecision{}
		}
		if !in.IsCreate && s.hasActiveRun(ctx, issue.ID) {
			return IssueRunDecision{}
		}
		return IssueRunDecision{Trigger: IssueRunTrigger{
			IssueID:      issue.ID,
			AgentID:      squad.LeaderID,
			AssigneeType: "squad",
			Source:       source,
		}, WillEnqueue: true}
	}
	return IssueRunDecision{}
}

// hasActiveRun covers queued, dispatched, running and runtime-waiting work on
// the whole issue. Bookkeeping writes must not start a second run beside any
// of those states, even when the prospective target agent differs.
// Errors fail closed because skipping an optional enqueue is safer than
// duplicating execution whose existence the server could not verify.
func (s *IssueService) hasActiveRun(ctx context.Context, issueID pgtype.UUID) bool {
	active, err := s.Queries.HasActiveTaskForIssue(ctx, issueID)
	return err != nil || active
}
