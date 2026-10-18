package ownernotification

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/workflowprojection"
)

func ref(kind, id string, revision int64) workflowprojection.OwnerIssueReference {
	return workflowprojection.OwnerIssueReference{Type: kind, ID: id, Revision: revision}
}

func summary(stage workflowprojection.OwnerIssueStage, signals ...workflowprojection.OwnerIssueSignal) workflowprojection.OwnerIssueSummary {
	return workflowprojection.OwnerIssueSummary{SchemaVersion: 1, CurrentPictureRevision: 9, Stage: stage, Signals: signals}
}

func input(s workflowprojection.OwnerIssueSummary) PolicyInput {
	return PolicyInput{Summary: s, IssueID: "issue-1", IssueTitle: "Repair employee gallery", WorkspaceSlug: "silver-mediums-pilot", AppBaseURL: "https://pilot.example.test/"}
}

func TestEvaluateSuppressesRoutineWorkflowNoise(t *testing.T) {
	stages := []workflowprojection.OwnerIssueStage{
		workflowprojection.OwnerStageActive, workflowprojection.OwnerStageInReview,
		workflowprojection.OwnerStageReleasing, workflowprojection.OwnerStageVerifying,
	}
	for _, stage := range stages {
		if got := Evaluate(input(summary(stage))); len(got) != 0 {
			t.Fatalf("stage %s admitted %d SMS intents", stage, len(got))
		}
	}
}

func TestEvaluateCreatesOneIntentPerCurrentDecision(t *testing.T) {
	s := summary(workflowprojection.OwnerStageNeedsOwner, workflowprojection.OwnerIssueSignal{
		Code: "owner_attention_required", References: []workflowprojection.OwnerIssueReference{ref("decision", "decision-a", 1), ref("decision", "decision-b", 3)},
	})
	got := Evaluate(input(s))
	if len(got) != 2 || got[0].EventKey == got[1].EventKey {
		t.Fatalf("intents = %#v", got)
	}
	if got[0].EventType != EventOwnerDecision || got[0].ActionURL != "https://pilot.example.test/silver-mediums-pilot/inbox?issue=issue-1&decision=decision-a" {
		t.Fatalf("first intent = %#v", got[0])
	}
}

func TestEvaluatePrefersRegressionOverGenericIncident(t *testing.T) {
	s := summary(workflowprojection.OwnerStageRecovering,
		workflowprojection.OwnerIssueSignal{Code: "current_incident", References: []workflowprojection.OwnerIssueReference{ref("incident", "incident-a", 2)}},
		workflowprojection.OwnerIssueSignal{Code: "current_regression", References: []workflowprojection.OwnerIssueReference{ref("release", "release-a", 4)}},
	)
	got := Evaluate(input(s))
	if len(got) != 1 || got[0].EventType != EventProductionRegression {
		t.Fatalf("intents = %#v", got)
	}
}

func TestEvaluateCompletionUsesOutcomeIdentity(t *testing.T) {
	s := summary(workflowprojection.OwnerStageComplete,
		workflowprojection.OwnerIssueSignal{Code: "outcome_complete", References: []workflowprojection.OwnerIssueReference{ref("outcome", "outcome-a", 7)}},
		workflowprojection.OwnerIssueSignal{Code: "verified_live", References: []workflowprojection.OwnerIssueReference{ref("release", "release-a", 8)}},
	)
	got := Evaluate(input(s))
	if len(got) != 1 || got[0].EventType != EventVerifiedCompletion || got[0].SourceID != "outcome-a" {
		t.Fatalf("intents = %#v", got)
	}
}

func TestEvaluateRevisionChangeDoesNotChangeSemanticEventKey(t *testing.T) {
	first := Evaluate(input(summary(workflowprojection.OwnerStageComplete,
		workflowprojection.OwnerIssueSignal{Code: "outcome_complete", References: []workflowprojection.OwnerIssueReference{ref("outcome", "outcome-a", 7)}},
	)))
	updated := Evaluate(input(summary(workflowprojection.OwnerStageComplete,
		workflowprojection.OwnerIssueSignal{Code: "outcome_complete", References: []workflowprojection.OwnerIssueReference{ref("outcome", "outcome-a", 8)}},
	)))
	if len(first) != 1 || len(updated) != 1 || first[0].EventKey != updated[0].EventKey {
		t.Fatalf("event keys changed across source revision: %#v %#v", first, updated)
	}
	if first[0].SourceRevision == updated[0].SourceRevision {
		t.Fatalf("source revision did not preserve current evidence: %#v %#v", first, updated)
	}
}

func TestEvaluateCurrentIncidentSuppressesDuplicateTechnicalStopAlert(t *testing.T) {
	s := summary(workflowprojection.OwnerStageBlocked,
		workflowprojection.OwnerIssueSignal{Code: "current_incident", References: []workflowprojection.OwnerIssueReference{ref("incident", "incident-a", 2)}},
		workflowprojection.OwnerIssueSignal{Code: "technical_stop", References: []workflowprojection.OwnerIssueReference{ref("stop", "stop-a", 1)}},
	)
	got := Evaluate(input(s))
	if len(got) != 1 || got[0].EventType != EventImportantIncident || got[0].SourceID != "incident-a" {
		t.Fatalf("intents = %#v", got)
	}
}

func TestEvaluateFailsClosedOnUnavailableProjection(t *testing.T) {
	s := summary(workflowprojection.OwnerStageStatusUnavailable, workflowprojection.OwnerIssueSignal{Code: "outcome_complete", References: []workflowprojection.OwnerIssueReference{ref("outcome", "outcome-a", 1)}})
	if got := Evaluate(input(s)); len(got) != 0 {
		t.Fatalf("intents = %#v", got)
	}
}
