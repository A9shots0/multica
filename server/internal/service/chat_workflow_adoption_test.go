package service

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkflowDriverDoesNotAdoptOrdinaryChat(t *testing.T) {
	for _, binding := range []string{"opening", "intake"} {
		t.Run(binding, func(t *testing.T) {
			fx, agent, runtime := newArchiveCancelFixture(t)
			q := db.New(fx.Pool)
			reviewer := fx.Agent(t, "Reviewer", runtime)
			squad := fx.Squad(t, "Generic controller", agent)
			fx.InsertNoID(t, "squad_workflow_template", testutil.Cols{"squad_id": squad, "workspace_id": fx.WorkspaceID, "implementer_id": agent, "reviewer_id": reviewer}, "squad_id=$1", squad)
			issueID := fx.Issue(t, "Ordinary conversation", testutil.Cols{"assignee_type": "agent", "assignee_id": agent})
			if binding == "opening" {
				chat := fx.ChatSession(t, agent)
				task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "chat_session_id": chat, "status": "completed"})
				fx.Exec(t, `UPDATE issue SET origin_type='agent_create',origin_id=$2 WHERE id=$1`, issueID, task)
			} else {
				fx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": fx.WorkspaceID, "recipient_type": "member", "recipient_id": fx.UserID, "type": "chat_intake", "severity": "info", "issue_id": issueID, "title": "Dispatch receipt", "details": `{"intake_state":"continued"}`})
			}
			driver := &WorkflowDriver{Pool: fx.Pool, Queries: q}
			for pass := 0; pass < 2; pass++ {
				action, err := driver.driveIssue(context.Background(), StalledWorkIssue{IssueID: issueID})
				if err != nil || action.Action != "skipped" {
					t.Fatalf("ordinary execution adopted: %+v %v", action, err)
				}
			}
			if fx.Count(t, `SELECT (SELECT count(*) FROM issue_workflow_plan WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)+(SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type<>'chat_intake')`, issueID) != 0 {
				t.Fatal("legacy driver created a plan, duplicate worker or owner routing question")
			}
			issue, err := q.GetIssue(context.Background(), util.MustParseUUID(issueID))
			if err != nil {
				t.Fatal(err)
			}
			if _, proposed, err := ProposeIssueWorkflowPlan(context.Background(), q, issue); err != nil || proposed {
				t.Fatalf("ordinary execution proposed for legacy controller: %v %v", proposed, err)
			}
		})
	}
}
