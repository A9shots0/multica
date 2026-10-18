package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/workflowguard"
)

// boundedWorkflowTaskContext prevents the provider's native instruction file
// from becoming a second, mutable assignment channel. The server has already
// authenticated the workflow packet and exact skill snapshot before claim;
// only that exact skill surface belongs beside the packet at execution time.
func boundedWorkflowTaskContext(task Task, ordinary execenv.TaskContextForEnv) execenv.TaskContextForEnv {
	if len(task.WorkflowPacket) == 0 {
		return ordinary
	}
	return execenv.TaskContextForEnv{
		AgentID:     ordinary.AgentID,
		AgentName:   ordinary.AgentName,
		AgentSkills: ordinary.AgentSkills,
	}
}

func validateBoundedWorkflowClaimExposure(task Task, provider string, assignment workflowguard.LegacyAssignmentV1) error {
	if task.Agent == nil || task.Agent.ID != task.AgentID ||
		assignment.TargetAgentID != task.AgentID || assignment.IssueID != task.IssueID ||
		assignment.SquadID != task.SquadID || assignment.RuntimeID != task.RuntimeID ||
		assignment.RuntimeProvider != provider || assignment.Model != task.Agent.Model ||
		assignment.ReasoningEffort != task.Agent.ThinkingLevel {
		return fmt.Errorf("workflow assignment identity or exact runtime selector mismatch")
	}
	actual := make([]workflowguard.RequiredSkill, 0, len(task.Agent.SkillRefs))
	for _, ref := range task.Agent.SkillRefs {
		actual = append(actual, workflowguard.RequiredSkill{
			ID: ref.ID, Source: ref.Source, SHA256: ref.Hash,
			SizeBytes: ref.SizeBytes, FileCount: ref.FileCount,
		})
	}
	if !workflowguard.MatchRequiredSkills(assignment.RequiredSkills, actual) {
		return fmt.Errorf("workflow assignment exact skill references mismatch")
	}
	if len(task.Agent.Skills) != 0 || strings.TrimSpace(task.Agent.Instructions) != "" ||
		len(task.Agent.CustomEnv) != 0 || len(task.Agent.CustomArgs) != 0 ||
		len(bytes.TrimSpace(task.Agent.McpConfig)) != 0 || len(bytes.TrimSpace(task.Agent.RuntimeConfig)) != 0 ||
		task.Agent.ServiceTier != "" || len(task.Agent.DisabledRuntimeSkills) != 0 ||
		len(task.ConnectedApps) != 0 || len(task.RemoteMCPConnections) != 0 || len(task.PluginHookTools) != 0 ||
		len(task.Repos) != 0 || task.ProjectID != "" || task.ProjectTitle != "" || task.ProjectDescription != "" ||
		len(task.ProjectResources) != 0 || task.WorkspaceContext != "" || len(bytes.TrimSpace(task.DevelopmentContext)) != 0 || len(bytes.TrimSpace(task.BrainContext)) != 0 || len(task.IssueStatuses) != 0 ||
		task.IssueStatusesOmitted != 0 || task.WorkspaceSlug != "" || task.IssueIdentifier != "" || task.ThreadName != "" ||
		task.PriorSessionID != "" || task.PriorWorkDir != "" || task.PriorSessionResumeUnavailable ||
		task.TriggerCommentID != "" || len(task.CoalescedCommentIDs) != 0 || len(task.CoalescedComments) != 0 ||
		task.TriggerThreadID != "" || task.TriggerCommentContent != "" || task.TriggerAuthorType != "" ||
		task.TriggerAuthorName != "" || task.NewCommentCount != 0 || task.NewCommentsSince != "" || task.NewCommentsDeltaKnown ||
		task.ChatSessionID != "" || task.ChatMessage != "" || len(task.ChatMessageAttachments) != 0 ||
		task.AutopilotRunID != "" || task.AutopilotID != "" || task.AutopilotTitle != "" || task.AutopilotDescription != "" ||
		len(bytes.TrimSpace(task.AutopilotTriggerPayload)) != 0 || task.QuickCreatePrompt != "" ||
		len(task.QuickCreateAttachmentIDs) != 0 || len(bytes.TrimSpace(task.QuickCreateSourceContext)) != 0 ||
		task.HandoffNote != "" || task.SquadName != "" || task.ParentIssueID != "" || task.ParentIssueIdentifier != "" ||
		task.RequestingUserName != "" || task.RequestingUserProfileDescription != "" || task.InitiatorType != "" ||
		task.InitiatorID != "" || task.InitiatorName != "" || task.InitiatorEmail != "" {
		return fmt.Errorf("workflow claim contains ambient context or capability exposure")
	}
	return nil
}

func validateBoundedWorkflowResolvedSkills(task Task, assignment workflowguard.LegacyAssignmentV1) error {
	if task.Agent == nil || len(task.Agent.Skills) != len(assignment.RequiredSkills) {
		return fmt.Errorf("workflow exact skill payload count mismatch")
	}
	actual := make([]workflowguard.RequiredSkill, 0, len(task.Agent.Skills))
	for _, skill := range task.Agent.Skills {
		actual = append(actual, workflowguard.RequiredSkill{
			ID: skill.ID, Source: skill.Source, SHA256: skill.Hash,
			SizeBytes: skill.SizeBytes, FileCount: len(skill.Files),
		})
	}
	if !workflowguard.MatchRequiredSkills(assignment.RequiredSkills, actual) {
		return fmt.Errorf("workflow resolved skill payload mismatch")
	}
	return nil
}

func appendBoundedWorkflowSkillPayload(prompt string, skills []SkillData) (string, error) {
	payload, err := json.Marshal(skills)
	if err != nil {
		return "", fmt.Errorf("encode exact workflow skill payload: %w", err)
	}
	return prompt + "\n\nExact skill bundle payload (JSON; no other skill source is authorized):\n" + string(payload), nil
}
