package ownernotification

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/multica-ai/multica/server/internal/workflowprojection"
)

type EventType string

const (
	EventOwnerDecision        EventType = "owner_decision"
	EventImportantIncident    EventType = "important_incident"
	EventProductionRegression EventType = "production_regression"
	EventRollbackStarted      EventType = "rollback_started"
	EventVerifiedCompletion   EventType = "verified_completion"
)

type Intent struct {
	EventType      EventType
	EventKey       string
	SourceType     string
	SourceID       string
	SourceRevision int64
	Message        string
	ActionURL      string
	Payload        map[string]string
}

type PolicyInput struct {
	Summary       workflowprojection.OwnerIssueSummary
	IssueID       string
	IssueTitle    string
	WorkspaceSlug string
	AppBaseURL    string
}

// Evaluate admits only owner-consequential workflow truth. It deliberately has
// no fallback from issue status, agent runs, comments, or mutable prose.
func Evaluate(input PolicyInput) []Intent {
	if input.Summary.SchemaVersion != workflowprojection.OwnerIssueSummarySchemaVersion ||
		input.Summary.Stage == workflowprojection.OwnerStageStatusUnavailable ||
		strings.TrimSpace(input.IssueID) == "" || strings.TrimSpace(input.WorkspaceSlug) == "" ||
		strings.TrimSpace(input.AppBaseURL) == "" {
		return nil
	}

	base := strings.TrimRight(strings.TrimSpace(input.AppBaseURL), "/")
	issueURL := fmt.Sprintf("%s/%s/inbox?issue=%s", base, url.PathEscape(input.WorkspaceSlug), url.QueryEscape(input.IssueID))
	title := boundedTitle(input.IssueTitle)
	seen := map[string]bool{}
	intents := make([]Intent, 0, 4)
	appendRefs := func(eventType EventType, signalCode, message string, refs []workflowprojection.OwnerIssueReference, decision bool) {
		for _, ref := range refs {
			if strings.TrimSpace(ref.Type) == "" || strings.TrimSpace(ref.ID) == "" || ref.Revision < 1 {
				continue
			}
			// Revisions prove the source is current, but the semantic source ID is
			// the notification identity. A correction to the same decision,
			// incident, or outcome must not send the owner a second SMS.
			eventKey := fmt.Sprintf("%s:%s:%s", eventType, input.IssueID, ref.ID)
			if seen[eventKey] {
				continue
			}
			seen[eventKey] = true
			actionURL := issueURL
			if decision {
				actionURL += "&decision=" + url.QueryEscape(ref.ID)
			}
			intents = append(intents, Intent{
				EventType: eventType, EventKey: eventKey, SourceType: ref.Type,
				SourceID: ref.ID, SourceRevision: ref.Revision,
				Message: fmt.Sprintf(message, title, actionURL), ActionURL: actionURL,
				Payload: map[string]string{"signal_code": signalCode, "issue_id": input.IssueID},
			})
		}
	}

	var currentIncidentRefs []workflowprojection.OwnerIssueReference
	var technicalStopRefs []workflowprojection.OwnerIssueReference
	var hasSpecificIncident bool
	for _, signal := range input.Summary.Signals {
		switch signal.Code {
		case "owner_attention_required":
			appendRefs(EventOwnerDecision, signal.Code, "Multica needs you: %s. Open the current secure decision: %s", signal.References, true)
		case "current_regression":
			hasSpecificIncident = true
			appendRefs(EventProductionRegression, signal.Code, "Multica alert: production regression on %s. Recovery is active. %s", signal.References, false)
		case "rollback_in_progress":
			hasSpecificIncident = true
			appendRefs(EventRollbackStarted, signal.Code, "Multica alert: rollback started for %s. %s", signal.References, false)
		case "current_incident":
			currentIncidentRefs = append(currentIncidentRefs, signal.References...)
		case "technical_stop":
			technicalStopRefs = append(technicalStopRefs, signal.References...)
		case "outcome_complete":
			appendRefs(EventVerifiedCompletion, signal.Code, "Multica complete: %s. The accepted business outcome is independently verified. %s", signal.References, false)
		}
	}
	if !hasSpecificIncident {
		// A technical stop commonly references the same transition as a current
		// incident. Prefer the incident identity when present so one transition
		// cannot generate two generic alerts.
		genericIncidentRefs := currentIncidentRefs
		if len(genericIncidentRefs) == 0 {
			genericIncidentRefs = technicalStopRefs
		}
		appendRefs(EventImportantIncident, "current_incident", "Multica incident: %s. Open the current consequence and recovery status: %s", genericIncidentRefs, false)
	}
	return intents
}

func boundedTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return "an issue"
	}
	const max = 120
	if len(title) <= max {
		return title
	}
	return strings.TrimSpace(title[:max-1]) + "…"
}
