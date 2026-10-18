package daemon

import (
	"fmt"
	"strings"

	"github.com/multica-ai/multica/server/internal/service"
)

type automaticWorkMethod = service.AutomaticWorkMethod

const (
	outcomeOwnershipSkill = service.OutcomeOwnershipSkill
	evidenceFirstSkill    = service.EvidenceFirstSkill
	longWorkSkill         = service.LongWorkSkill
	researchREASkill      = service.ResearchREASkill
	learnAndHardenSkill   = service.LearnAndHardenSkill
	uiAcceptanceSkill     = service.UIAcceptanceSkill
)

// automaticallySelectedWorkMethods deterministically maps the task's stable
// shape and current request wording to the Silver Mediums methods the model
// should load. The skills remain provider-native bundles; this selection is a
// small explicit routing hint, not a second skill implementation.
func automaticallySelectedWorkMethods(task Task) []automaticWorkMethod {
	available := installedSkillNames(task.Agent)
	if len(available) == 0 {
		return nil
	}

	candidates := service.SelectAutomaticWorkMethods(workMethodSelectionInput(task))
	selected := make([]automaticWorkMethod, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := available[candidate.Name]; ok {
			selected = append(selected, candidate)
		}
	}
	return selected
}

func workMethodSelectionInput(task Task) service.WorkMethodSelectionInput {
	return service.WorkMethodSelectionInput{
		IssueID:               task.IssueID,
		ThreadName:            task.ThreadName,
		TriggerCommentContent: task.TriggerCommentContent,
		ChatMessage:           task.ChatMessage,
		ChatFollowUp:          task.ChatFollowUp,
		HandoffNote:           task.HandoffNote,
		AutopilotRunID:        task.AutopilotRunID,
		AutopilotTitle:        task.AutopilotTitle,
		AutopilotDescription:  task.AutopilotDescription,
		QuickCreatePrompt:     task.QuickCreatePrompt,
		IsLeaderTask:          task.IsLeaderTask,
		PriorSessionID:        task.PriorSessionID,
		ControllerBound:       len(task.WorkflowPacket) > 0 || task.GuardedExecution != nil,
	}
}

func installedSkillNames(agent *AgentData) map[string]struct{} {
	if agent == nil {
		return nil
	}
	names := make(map[string]struct{}, len(agent.Skills)+len(agent.SkillRefs))
	for _, skill := range agent.Skills {
		if name := strings.TrimSpace(skill.Name); name != "" {
			names[name] = struct{}{}
		}
	}
	for _, ref := range agent.SkillRefs {
		if name := strings.TrimSpace(ref.Name); name != "" {
			names[name] = struct{}{}
		}
	}
	return names
}

func buildAutomaticWorkMethodsBlock(task Task) string {
	methods := automaticallySelectedWorkMethods(task)
	if len(methods) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Automatically selected work methods\n\n")
	b.WriteString("Multica selected these methods from the task type and current request. Load each named skill before acting; the owner does not need to invoke it.\n\n")
	for _, method := range methods {
		fmt.Fprintf(&b, "- `%s` — %s\n", method.Name, method.Reason)
	}
	b.WriteString("\n")
	return b.String()
}
