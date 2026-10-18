package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// EnsureIssueWorkflowPlan installs a controller for an ordinary, agent-routed
// issue.
//
// Until now a plan was only ever created by EnsureSquadWorkflowPlan, which
// returns immediately unless the issue is assigned to a SQUAD with a template.
// Ordinary work is assigned to a single agent, so it got no plan — and with no
// plan there is no stage, no reviewer, and nothing for the controller or the
// driver to advance. In the Silver Mediums pilot that left 19 of 21 open
// issues permanently inert, which is the root cause of the frozen board.
//
// The rule here is deliberately narrow, because picking the wrong people is
// worse than picking none:
//
//   - The IMPLEMENTER is the agent the issue is already assigned to. That
//     assignment is the domain routing decision someone already made — the
//     Phone Farm specialist on Phone Farm work, Product & UI on UI work — and
//     re-deriving it from scratch would throw that away.
//   - The REVIEWER comes from the squad template, so "who is allowed to
//     verify" stays a configured property of the squad rather than something
//     this function invents.
//   - An issue assigned to the squad's own reviewer is a review request, not
//     an implementation request, so the template's implementer builds it and
//     the assignee keeps the reviewer seat. This also satisfies the table's
//     implementer <> reviewer constraint without silently reassigning work.
//
// Plans are created in `received`, the state whose only legal transition is to
// queue the implementer. The plan therefore asserts that work SHOULD start,
// never that it already did.
func EnsureIssueWorkflowPlan(ctx context.Context, q *db.Queries, issue db.Issue) (db.IssueWorkflowPlan, bool, error) {
	if plan, err := q.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); err == nil {
		return plan, false, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.IssueWorkflowPlan{}, false, fmt.Errorf("read workflow plan: %w", err)
	}
	proposal, ok, err := ProposeIssueWorkflowPlan(ctx, q, issue)
	if err != nil || !ok {
		return db.IssueWorkflowPlan{}, false, err
	}
	squad, template, implementer, reviewer := proposal.Squad, proposal.Template, proposal.ImplementerID, proposal.ReviewerID

	plan, err := q.UpsertIssueWorkflowPlan(ctx, db.UpsertIssueWorkflowPlanParams{
		IssueID:            issue.ID,
		WorkspaceID:        issue.WorkspaceID,
		SquadID:            squad.ID,
		ImplementerID:      implementer,
		ReviewerID:         reviewer,
		ReleaseAgentID:     template.ReleaseAgentID,
		VerifierID:         template.VerifierID,
		RequiresOwner:      template.RequiresOwner,
		RequiresDeployment: template.RequiresDeployment,
		CurrentState:       string(workflowstate.Received),
	})
	if err != nil {
		return db.IssueWorkflowPlan{}, false, fmt.Errorf("install workflow plan: %w", err)
	}
	return plan, true, nil
}

// WorkflowPlanProposal is the controller that WOULD be installed for an issue.
// Separating the decision from the write lets the same rules be previewed
// against real board state without waking an agent.
type WorkflowPlanProposal struct {
	Squad         db.Squad
	Template      db.SquadWorkflowTemplate
	ImplementerID pgtype.UUID
	ReviewerID    pgtype.UUID
}

// ProposeIssueWorkflowPlan applies the routing rules described on
// EnsureIssueWorkflowPlan and returns the result without persisting it.
// ok is false when no controller can honestly be chosen.
func ProposeIssueWorkflowPlan(ctx context.Context, q *db.Queries, issue db.Issue) (WorkflowPlanProposal, bool, error) {
	ordinaryChat, err := q.IsOrdinaryChatIssue(ctx, db.IsOrdinaryChatIssueParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil || ordinaryChat.Bool {
		return WorkflowPlanProposal{}, false, err
	}
	// A squad-assigned issue already has an owner for this decision, and that
	// path installs the template verbatim. Never race it.
	if issue.AssigneeType.Valid && issue.AssigneeType.String == "squad" {
		return WorkflowPlanProposal{}, false, nil
	}
	// A workspace's first generic template cannot decide which specialist or
	// host owns project work. Preserve explicit routing; leave unassigned
	// project issues for the coordinator instead of silently claiming Builder.
	if issue.ProjectID.Valid && (!issue.AssigneeType.Valid || issue.AssigneeType.String != "agent" || !issue.AssigneeID.Valid) {
		return WorkflowPlanProposal{}, false, nil
	}

	squad, template, err := resolveWorkflowTemplate(ctx, q, issue)
	if err != nil {
		return WorkflowPlanProposal{}, false, err
	}
	if !squad.ID.Valid {
		// No squad in this workspace declares a controller. Installing a plan
		// with invented roles would make the board claim a review that nobody
		// is configured to perform.
		return WorkflowPlanProposal{}, false, nil
	}

	implementer := template.ImplementerID
	if issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" && issue.AssigneeID.Valid {
		implementer = issue.AssigneeID
	}
	reviewer := template.ReviewerID
	if implementer == reviewer {
		// The issue is assigned to the reviewer: treat it as a review request
		// and let the template's implementer produce the candidate.
		implementer = template.ImplementerID
	}
	if !implementer.Valid || !reviewer.Valid || implementer == reviewer {
		return WorkflowPlanProposal{}, false, nil
	}
	return WorkflowPlanProposal{Squad: squad, Template: template, ImplementerID: implementer, ReviewerID: reviewer}, true, nil
}

// resolveWorkflowTemplate picks the squad whose controller should own an
// ordinary issue.
//
// Preference order is: the squad the assignee actually belongs to, then any
// squad that declares a template. Membership first matters because the roster
// is where domain knowledge lives — an agent that sits in Cross-boundary
// change should be reviewed under Cross-boundary change's rules, not under
// whichever template the database happens to return first.
func resolveWorkflowTemplate(ctx context.Context, q *db.Queries, issue db.Issue) (db.Squad, db.SquadWorkflowTemplate, error) {
	squads, err := q.ListSquads(ctx, issue.WorkspaceID)
	if err != nil {
		return db.Squad{}, db.SquadWorkflowTemplate{}, fmt.Errorf("list squads: %w", err)
	}

	var fallbackSquad db.Squad
	var fallbackTemplate db.SquadWorkflowTemplate
	for _, squad := range squads {
		if squad.ArchivedAt.Valid {
			continue
		}
		template, err := q.GetSquadWorkflowTemplate(ctx, db.GetSquadWorkflowTemplateParams{
			SquadID: squad.ID, WorkspaceID: issue.WorkspaceID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return db.Squad{}, db.SquadWorkflowTemplate{}, fmt.Errorf("read squad template: %w", err)
		}
		if issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" && issue.AssigneeID.Valid {
			member, memberErr := squadHasAgent(ctx, q, squad.ID, issue.AssigneeID)
			if memberErr != nil {
				return db.Squad{}, db.SquadWorkflowTemplate{}, memberErr
			}
			if member {
				return squad, template, nil
			}
		}
		if !fallbackSquad.ID.Valid {
			fallbackSquad, fallbackTemplate = squad, template
		}
	}
	return fallbackSquad, fallbackTemplate, nil
}

func squadHasAgent(ctx context.Context, q *db.Queries, squadID, agentID pgtype.UUID) (bool, error) {
	members, err := q.ListSquadMembers(ctx, squadID)
	if err != nil {
		return false, fmt.Errorf("list squad members: %w", err)
	}
	for _, member := range members {
		if member.MemberType == "agent" && member.MemberID == agentID {
			return true, nil
		}
	}
	return false, nil
}
