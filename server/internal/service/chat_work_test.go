package service

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestChatWorkNeverConfusesResponseWithOutcome(t *testing.T) {
	if got := ChatWorkState(db.ListChatWorkRow{TaskStatus: "completed", Status: "done"}, workflowprojection.OwnerIssueSummary{}); got != "done" {
		t.Fatalf("explicitly accepted ordinary work must be done: %s", got)
	}
	if got := ChatWorkState(db.ListChatWorkRow{TaskStatus: "completed", Status: "in_review"}, workflowprojection.OwnerIssueSummary{}); got != "waiting" {
		t.Fatalf("a finished worker awaiting review must not be done, working or unavailable: %s", got)
	}
	for _, tc := range []struct {
		name, task, want string
		stage            workflowprojection.OwnerIssueStage
		fresh            bool
	}{
		{"running fresh", "running", "working", workflowprojection.OwnerStageActive, true},
		{"lost runtime", "running", "unavailable", workflowprojection.OwnerStageActive, false},
		{"queued is not running", "queued", "queued", workflowprojection.OwnerStageActive, true},
		{"decision wins", "running", "needs_you", workflowprojection.OwnerStageNeedsOwner, true},
		{"completed response is not done", "completed", "waiting", workflowprojection.OwnerStageStatusUnavailable, true},
		{"accepted outcome", "completed", "done", workflowprojection.OwnerStageComplete, true},
		{"dependency wins", "queued", "waiting", workflowprojection.OwnerStageBlocked, true},
		{"failed run", "failed", "stopped", workflowprojection.OwnerStageActive, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ChatWorkState(db.ListChatWorkRow{TaskStatus: tc.task, RuntimeFresh: tc.fresh}, workflowprojection.OwnerIssueSummary{Stage: tc.stage})
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
