package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const requireProjectContextForAgentWorkSetting = "require_project_context_for_agent_work"
const requireMemberIssueCreationSetting = "require_member_issue_creation"
const ProjectExecutionRoutesSetting = "project_execution_routes"
const ProjectExecutionCapabilitiesSetting = "project_execution_capabilities"

var ErrProjectExecutionRouteDenied = errors.New("this project's work must run on an approved machine; choose a worker on that machine")
var ErrProjectExecutionCapabilityDenied = errors.New("this project's work requires an explicitly approved worker with all required skills")

// ProjectExecutionAuthoritySourceError identifies the task's source read
// separately from policy/runtime/capability reads so claims retain
// missing-versus-transient recovery.
type ProjectExecutionAuthoritySourceError struct {
	Err error
}

func (e *ProjectExecutionAuthoritySourceError) Error() string {
	return "load task project execution authority source: " + e.Err.Error()
}

func (e *ProjectExecutionAuthoritySourceError) Unwrap() error { return e.Err }

// ErrIssueProjectRequired means a workspace opted into project-scoped
// execution and an issue without a project attempted to start agent work.
var ErrIssueProjectRequired = errors.New("a project is required before agent work can start")

// ErrAgentIssueCreationDisabled means a workspace requires issue creation to
// originate from a member. It applies to root issues and sub-issues equally.
var ErrAgentIssueCreationDisabled = errors.New("workspace policy requires a member to create issues")

type workspaceExecutionSettings struct {
	RequireProjectContextForAgentWork bool                                      `json:"require_project_context_for_agent_work"`
	RequireMemberIssueCreation        bool                                      `json:"require_member_issue_creation"`
	ProjectExecutionRoutes            map[string][]string                       `json:"project_execution_routes"`
	ProjectExecutionCapabilities      map[string]ProjectExecutionCapabilityRule `json:"project_execution_capabilities"`
}

type ProjectExecutionCapabilityRule struct {
	RequiredSkillIDs []string `json:"required_skill_ids"`
	EligibleAgentIDs []string `json:"eligible_agent_ids"`
}

// validateProjectExecutionRoute uses stored workspace policy and runtime identity,
// never a worker's name, prompt or editable task context. Projects without a
// declared route retain existing behavior. An empty declared route denies all.
func validateProjectExecutionRoute(ctx context.Context, q *db.Queries, issue db.Issue, runtimeID pgtype.UUID) error {
	if !issue.ProjectID.Valid {
		return nil
	}
	settings, err := loadWorkspaceExecutionSettings(ctx, q, issue.WorkspaceID)
	if err != nil {
		return fmt.Errorf("load project execution route: %w", err)
	}
	return validateProjectExecutionRouteWithSettings(ctx, q, issue, runtimeID, settings)
}

func validateProjectExecutionRouteWithSettings(ctx context.Context, q *db.Queries, issue db.Issue, runtimeID pgtype.UUID, settings workspaceExecutionSettings) error {
	allowed, declared := settings.ProjectExecutionRoutes[util.UUIDToString(issue.ProjectID)]
	if !declared {
		return nil
	}
	if !runtimeID.Valid || len(allowed) == 0 {
		return ErrProjectExecutionRouteDenied
	}
	runtime, err := q.LockAgentRuntimeForProjectExecutionAuthority(ctx, db.LockAgentRuntimeForProjectExecutionAuthorityParams{ID: runtimeID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectExecutionRouteDenied
	}
	if err != nil {
		return fmt.Errorf("load project execution runtime: %w", err)
	}
	if !runtime.DaemonID.Valid || !slices.Contains(allowed, runtime.DaemonID.String) {
		return ErrProjectExecutionRouteDenied
	}
	return nil
}

// Capability policy applies to the already selected worker. It never selects
// a substitute. Both explicit listing and every currently enabled skill
// binding are required; revoking either fails closed at the next boundary.
func validateProjectExecutionCapabilityWithSettings(ctx context.Context, q *db.Queries, issue db.Issue, agentID pgtype.UUID, settings workspaceExecutionSettings) error {
	rule, declared := settings.ProjectExecutionCapabilities[util.UUIDToString(issue.ProjectID)]
	if !declared {
		return nil
	}
	agentIDString := util.UUIDToString(agentID)
	if !agentID.Valid || len(rule.RequiredSkillIDs) == 0 || len(rule.EligibleAgentIDs) == 0 || !slices.Contains(rule.EligibleAgentIDs, agentIDString) {
		return ErrProjectExecutionCapabilityDenied
	}
	agent, err := q.LockAgentForAutopilotAssignment(ctx, db.LockAgentForAutopilotAssignmentParams{ID: agentID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (agent.ArchivedAt.Valid || agent.Kind != "user")) {
		return ErrProjectExecutionCapabilityDenied
	}
	if err != nil {
		return fmt.Errorf("load project execution capability agent: %w", err)
	}
	requiredIDs := make([]pgtype.UUID, 0, len(rule.RequiredSkillIDs))
	for _, skillID := range rule.RequiredSkillIDs {
		parsed, err := util.ParseUUID(skillID)
		if err != nil {
			return fmt.Errorf("decode project execution capability skill: %w", err)
		}
		requiredIDs = append(requiredIDs, parsed)
	}
	enabled, err := q.LockAgentSkillsByIDsForProjectExecutionAuthority(ctx, db.LockAgentSkillsByIDsForProjectExecutionAuthorityParams{AgentID: agent.ID, SkillIds: requiredIDs})
	if err != nil {
		return fmt.Errorf("load project execution capability skills: %w", err)
	}
	if len(enabled) != len(requiredIDs) {
		return ErrProjectExecutionCapabilityDenied
	}
	return nil
}

// validateProjectExecutionAuthority preserves the route error when both
// independent policies fail by always evaluating machine authority first.
func validateProjectExecutionAuthority(ctx context.Context, q *db.Queries, issue db.Issue, agentID, runtimeID pgtype.UUID) error {
	if !issue.ProjectID.Valid {
		return nil
	}
	settings, err := loadWorkspaceExecutionSettings(ctx, q, issue.WorkspaceID)
	if err != nil {
		return fmt.Errorf("load project execution authority: %w", err)
	}
	if err := validateProjectExecutionRouteWithSettings(ctx, q, issue, runtimeID, settings); err != nil {
		return err
	}
	return validateProjectExecutionCapabilityWithSettings(ctx, q, issue, agentID, settings)
}

func loadWorkspaceExecutionSettings(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID) (workspaceExecutionSettings, error) {
	ws, err := q.LockWorkspaceProjectExecutionAuthority(ctx, workspaceID)
	if err != nil {
		return workspaceExecutionSettings{}, err
	}
	var settings workspaceExecutionSettings
	if len(ws.Settings) > 0 {
		if err := json.Unmarshal(ws.Settings, &settings); err != nil {
			return workspaceExecutionSettings{}, err
		}
	}
	return settings, nil
}

// validateIssueAssigneeProjectExecutionAuthority resolves the actual agent that an
// issue assignment would run (the assignee itself, or a squad's leader) and
// applies route and capability policy before any task is admitted. It does not
// choose a substitute worker.
func validateIssueAssigneeProjectExecutionAuthority(ctx context.Context, q *db.Queries, issue db.Issue) error {
	if !issue.AssigneeType.Valid || !issue.AssigneeID.Valid {
		return nil
	}
	targetID := issue.AssigneeID
	switch issue.AssigneeType.String {
	case "agent":
	case "squad":
		squad, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
			ID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("load project execution squad: %w", err)
		}
		targetID = squad.LeaderID
	default:
		return nil
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
		ID: targetID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("load project execution agent: %w", err)
	}
	return validateProjectExecutionAuthority(ctx, q, issue, agent.ID, agent.RuntimeID)
}

// ValidateTaskProjectExecutionAuthority is repeated before claim credentials and
// provider launch so rebinding or policy changes cannot rely on old admission.
func (s *TaskService) ValidateTaskProjectExecutionAuthority(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) error {
	if !task.IssueID.Valid {
		return nil
	}
	issue, err := q.LockIssueProjectExecutionAuthority(ctx, task.IssueID)
	if err != nil {
		return &ProjectExecutionAuthoritySourceError{Err: err}
	}
	return validateProjectExecutionAuthority(ctx, q, issue, task.AgentID, task.RuntimeID)
}

func requireMemberIssueCreation(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID) (bool, error) {
	workspace, err := q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("load workspace issue creation policy: %w", err)
	}
	if len(workspace.Settings) == 0 {
		return false, nil
	}
	var settings workspaceExecutionSettings
	if err := json.Unmarshal(workspace.Settings, &settings); err != nil {
		return false, fmt.Errorf("decode workspace issue creation policy: %w", err)
	}
	return settings.RequireMemberIssueCreation, nil
}

func validateIssueCreatorForCreation(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, creatorType string) error {
	required, err := requireMemberIssueCreation(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	return admitAgentIssueCreate(required, creatorType)
}

// isAgentIssueCreationPolicyViolation recognizes the database trigger that
// backs up service preflight checks. The trigger closes race windows and keeps
// direct/future issue writers from bypassing the workspace policy.
func isAgentIssueCreationPolicyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" && pgErr.ConstraintName == "issue_agent_creation_policy"
}

func requireProjectContextForAgentWork(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID) (bool, error) {
	workspace, err := q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("load workspace execution policy: %w", err)
	}
	if len(workspace.Settings) == 0 {
		return false, nil
	}
	var settings workspaceExecutionSettings
	if err := json.Unmarshal(workspace.Settings, &settings); err != nil {
		return false, fmt.Errorf("decode workspace execution policy: %w", err)
	}
	return settings.RequireProjectContextForAgentWork, nil
}

func validateIssueProjectForAgentWork(ctx context.Context, q *db.Queries, issue db.Issue) error {
	required, err := requireProjectContextForAgentWork(ctx, q, issue.WorkspaceID)
	if err != nil {
		return err
	}
	if required && !issue.ProjectID.Valid {
		return ErrIssueProjectRequired
	}
	return nil
}
