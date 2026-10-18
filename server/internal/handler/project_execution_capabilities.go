package handler

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	maxProjectExecutionCapabilityRules = 64
	maxRequiredSkillsPerProject        = 16
	maxEligibleAgentsPerProject        = 64
)

// Capabilities are execution authority, not descriptive metadata. Every ID is
// canonical, distinct, and workspace-scoped before the whole policy is stored.
func (h *Handler) validateProjectExecutionCapabilities(ctx context.Context, workspaceID pgtype.UUID, rules map[string]service.ProjectExecutionCapabilityRule) error {
	if len(rules) > maxProjectExecutionCapabilityRules {
		return fmt.Errorf("at most %d project execution capability rules are allowed", maxProjectExecutionCapabilityRules)
	}
	for projectID, rule := range rules {
		if !canonicalUUID(projectID) {
			return fmt.Errorf("project execution capability requires a canonical project ID")
		}
		if _, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: parseUUID(projectID), WorkspaceID: workspaceID}); err != nil {
			return fmt.Errorf("execution capability project does not belong to this workspace")
		}
		if len(rule.RequiredSkillIDs) == 0 {
			return fmt.Errorf("at least one required skill is required per project execution capability")
		}
		if len(rule.RequiredSkillIDs) > maxRequiredSkillsPerProject {
			return fmt.Errorf("at most %d required skills are allowed per project", maxRequiredSkillsPerProject)
		}
		if len(rule.EligibleAgentIDs) > maxEligibleAgentsPerProject {
			return fmt.Errorf("at most %d eligible agents are allowed per project", maxEligibleAgentsPerProject)
		}
		seenSkills := make(map[string]struct{}, len(rule.RequiredSkillIDs))
		for _, skillID := range rule.RequiredSkillIDs {
			if !canonicalUUID(skillID) {
				return fmt.Errorf("execution capability requires distinct canonical workspace skill IDs")
			}
			if _, duplicate := seenSkills[skillID]; duplicate {
				return fmt.Errorf("execution capability requires distinct canonical workspace skill IDs")
			}
			if _, err := h.Queries.GetSkillInWorkspace(ctx, db.GetSkillInWorkspaceParams{ID: parseUUID(skillID), WorkspaceID: workspaceID}); err != nil {
				return fmt.Errorf("execution capability skill does not belong to this workspace")
			}
			seenSkills[skillID] = struct{}{}
		}
		seenAgents := make(map[string]struct{}, len(rule.EligibleAgentIDs))
		for _, agentID := range rule.EligibleAgentIDs {
			if !canonicalUUID(agentID) {
				return fmt.Errorf("execution capability requires distinct canonical workspace agent IDs")
			}
			if _, duplicate := seenAgents[agentID]; duplicate {
				return fmt.Errorf("execution capability requires distinct canonical workspace agent IDs")
			}
			agent, err := h.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: parseUUID(agentID), WorkspaceID: workspaceID})
			if err != nil || agent.ArchivedAt.Valid || agent.Kind != "user" {
				return fmt.Errorf("execution capability agent does not belong to this workspace")
			}
			requiredIDs := make([]pgtype.UUID, 0, len(rule.RequiredSkillIDs))
			for _, skillID := range rule.RequiredSkillIDs {
				requiredIDs = append(requiredIDs, parseUUID(skillID))
			}
			enabled, err := h.Queries.ListAgentSkillsByIDs(ctx, db.ListAgentSkillsByIDsParams{AgentID: agent.ID, SkillIds: requiredIDs})
			if err != nil || len(enabled) != len(requiredIDs) {
				return fmt.Errorf("every eligible agent must currently have all required skills enabled")
			}
			seenAgents[agentID] = struct{}{}
		}
	}
	return nil
}

func canonicalUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}
