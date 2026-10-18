package service

import (
	"regexp"
	"strings"

	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

const (
	OutcomeOwnershipSkill = "multica-outcome-ownership"
	EvidenceFirstSkill    = "multica-evidence-first-debugging"
	LongWorkSkill         = "multica-long-work-supervision"
	ResearchREASkill      = "multica-research-rea"
	LearnAndHardenSkill   = "multica-learn-and-harden"
	UIAcceptanceSkill     = "multica-ui-acceptance-review"
	maxWorkMethodText     = 8 << 10
)

// AutomaticWorkMethod is one deterministic method selected from the stable
// task shape and current request text. It is shared by claim-time skill
// filtering and daemon prompt rendering so old and new workers cannot drift.
type AutomaticWorkMethod struct {
	Name   string
	Reason string
}

// WorkMethodSelectionInput contains only bounded, claim-visible facts. Project
// descriptions, ambient workspace instructions, Brain data and prior results
// are intentionally absent: they are not the current request.
type WorkMethodSelectionInput struct {
	IssueID               string
	ThreadName            string
	TriggerCommentContent string
	ChatMessage           string
	ChatFollowUp          string
	HandoffNote           string
	AutopilotRunID        string
	AutopilotTitle        string
	AutopilotDescription  string
	QuickCreatePrompt     string
	IsLeaderTask          bool
	PriorSessionID        string
	ControllerBound       bool
}

var (
	workActionPattern        = regexp.MustCompile(`(?i)(^|[^a-z0-9])(audit|build|change|create|deploy|design|fix|implement|improve|investigate|modernize|monitor|plan|redo|replace|research|review|rewrite|ship|streamline|supervise|update)([^a-z0-9]|$)`)
	workEvidencePattern      = regexp.MustCompile(`(?i)(^|[^a-z0-9])(audit|bug|broken|crash|debug|diagnose|error|incident|incorrect|outage|regression|repair|restore)([^a-z0-9]|$)`)
	workFailurePattern       = regexp.MustCompile(`(?i)(^|[^a-z0-9])(fail|failure)([^a-z0-9]|$)`)
	workStatusAlertPattern   = regexp.MustCompile(`(?i)milestones?.{0,32}failures?.{0,32}decisions?.{0,32}completion`)
	workLongPattern          = regexp.MustCompile(`(?i)(^|[^a-z0-9])(migration|modernization|modernize|monitor|orchestrate|overhaul|program|roadmap|supervise|supervision)([^a-z0-9]|$)`)
	workResearchPattern      = regexp.MustCompile(`(?i)(^|[^a-z0-9])(audit|benchmark|compare|explore|investigate|rea|research|reverse-engineer|reverse engineering)([^a-z0-9]|$)`)
	workHardenPattern        = regexp.MustCompile(`(?i)(^|[^a-z0-9])(guardrail|postmortem|regression|repeated|retrospective|retro|skill)([^a-z0-9]|$)`)
	workUISurfacePattern     = regexp.MustCompile(`(?i)(^|[^a-z0-9])(dashboard|frontend|interface|layout|page|screen|ui|ux|visual)([^a-z0-9]|$)`)
	workUISubstantialPattern = regexp.MustCompile(`(?i)(^|[^a-z0-9])(acceptance|again|build|correct|correction|create|design|implement|implementation|overhaul|preview|rebuild|redesign|repeated|review)([^a-z0-9]|$)`)
)

var workLongPhrases = []string{
	"24 hour", "24-hour", "long running", "long-running", "multi stage",
	"multi-stage", "over night", "overnight", "keep working", "continue working",
}

var workResearchPhrases = []string{
	"find examples", "find projects", "reference implementation", "reverse engineer",
}

var workHardenPhrases = []string{
	"agents.md", "context bloat", "keeps happening", "make this automatic",
	"stop agents from", "turn this into a skill", "workflow keeps",
}

// SelectAutomaticWorkMethods is the single routing contract used by both the
// server and daemon. Quick-create, automatic follow-ups and controller-bound
// work remain isolated from ambient methods.
func SelectAutomaticWorkMethods(input WorkMethodSelectionInput) []AutomaticWorkMethod {
	if input.ControllerBound || input.QuickCreatePrompt != "" || input.ChatFollowUp != "" {
		return nil
	}

	text := workMethodRoutingText(input)
	selected := make([]AutomaticWorkMethod, 0, 6)
	add := func(name, reason string) {
		selected = append(selected, AutomaticWorkMethod{Name: name, Reason: reason})
	}

	if input.IssueID != "" || input.AutopilotRunID != "" || workActionPattern.MatchString(text) {
		add(OutcomeOwnershipSkill, "carry the request to a usable, evidenced result")
	}
	if workEvidencePattern.MatchString(text) || (workFailurePattern.MatchString(text) && !workStatusAlertPattern.MatchString(text)) {
		add(EvidenceFirstSkill, "establish cause and recovery from real evidence")
	}
	if input.IsLeaderTask || (input.PriorSessionID != "" && input.IssueID != "") || workLongPattern.MatchString(text) || containsWorkMethodPhrase(text, workLongPhrases) {
		add(LongWorkSkill, "track milestones, stalls and usage across sustained work")
	}
	if workResearchPattern.MatchString(text) || containsWorkMethodPhrase(text, workResearchPhrases) {
		add(ResearchREASkill, "turn external evidence into a decision or experiment")
	}
	if workHardenPattern.MatchString(text) || containsWorkMethodPhrase(text, workHardenPhrases) {
		add(LearnAndHardenSkill, "convert a demonstrated pattern into the smallest durable guardrail")
	}
	if workUISurfacePattern.MatchString(text) && workUISubstantialPattern.MatchString(text) {
		add(UIAcceptanceSkill, "verify the exact interface against real user tasks before calling it ready")
	}
	return selected
}

func workMethodRoutingText(input WorkMethodSelectionInput) string {
	fields := []string{
		input.ThreadName,
		input.TriggerCommentContent,
		input.ChatMessage,
		input.HandoffNote,
		input.AutopilotTitle,
		input.AutopilotDescription,
		input.QuickCreatePrompt,
	}
	var b strings.Builder
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if len(field) > maxWorkMethodText {
			field = field[:maxWorkMethodText]
		}
		b.WriteString(field)
		b.WriteByte('\n')
	}
	return strings.ToLower(b.String())
}

func containsWorkMethodPhrase(text string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func automaticWorkMethodNames(input WorkMethodSelectionInput) map[string]struct{} {
	methods := SelectAutomaticWorkMethods(input)
	names := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		names[method.Name] = struct{}{}
	}
	return names
}

func isAutomaticWorkMethod(name string) bool {
	switch name {
	case OutcomeOwnershipSkill, EvidenceFirstSkill, LongWorkSkill, ResearchREASkill, LearnAndHardenSkill, UIAcceptanceSkill:
		return true
	default:
		return false
	}
}

// FilterAutomaticWorkMethodSkills preserves workspace skills and unrelated
// platform built-ins while dropping every automatic method not selected for
// this task. This makes deterministic routing work even on older daemons.
func FilterAutomaticWorkMethodSkills(skills []AgentSkillData, input WorkMethodSelectionInput) []AgentSkillData {
	selected := automaticWorkMethodNames(input)
	result := make([]AgentSkillData, 0, len(skills))
	for _, skill := range skills {
		builtin := skill.Source == skillbundle.SourceBuiltin || (skill.Source == "" && skill.ID == "")
		if builtin && isAutomaticWorkMethod(skill.Name) {
			if _, ok := selected[skill.Name]; !ok {
				continue
			}
		}
		result = append(result, skill)
	}
	return result
}

// FilterAutomaticWorkMethodRefs is the lightweight claim equivalent of
// FilterAutomaticWorkMethodSkills.
func FilterAutomaticWorkMethodRefs(refs []AgentSkillRefData, input WorkMethodSelectionInput) []AgentSkillRefData {
	selected := automaticWorkMethodNames(input)
	result := make([]AgentSkillRefData, 0, len(refs))
	for _, ref := range refs {
		if ref.Source == skillbundle.SourceBuiltin && isAutomaticWorkMethod(ref.Name) {
			if _, ok := selected[ref.Name]; !ok {
				continue
			}
		}
		result = append(result, ref)
	}
	return result
}
