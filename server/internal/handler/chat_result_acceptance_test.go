package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatResultAcceptance(t *testing.T) {
	for _, change := range []string{"current", "changed_result", "new_run", "new_question", "workflow"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			agentID, sessionID, runtimeID, _ := setupDirectChatSession(t, ctx, "Result acceptance")
			issueID := dbfx.Issue(t, "Review the welcome wording")
			dbfx.Exec(t, `UPDATE issue SET status='in_review',assignee_type='agent',assignee_id=$2 WHERE id=$1`, issueID, agentID)
			resultID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "status": "completed", "accountable_user_id": testUserID, "result": `{"output":"Welcome to the team."}`})
			linkDeliveryIssue(t, sessionID, issueID)
			session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
			agent, _ := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
			target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID))
			if err != nil || target.ResultTaskID != resultID || target.ResultDigest == "" {
				t.Fatalf("result target missing: %+v %v", target, err)
			}
			sent, err := testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "I accept this finished wording.", nil, "member", parseUUID(testUserID), "", target)
			if err != nil {
				t.Fatal(err)
			}
			taskID := uuidToString(sent.Task.ID)
			markTaskRunning(t, ctx, taskID)
			switch change {
			case "changed_result":
				dbfx.Exec(t, `UPDATE agent_task_queue SET result='{"output":"Different result"}' WHERE id=$1`, resultID)
			case "new_run":
				dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "status": "queued", "accountable_user_id": testUserID})
			case "workflow":
				seedChatDecisionWorkflow(t, ctx, parseUUID(issueID))
			case "new_question":
				dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID, "issue_id": issueID, "type": "chat_intake", "severity": "action_required", "title": "One more choice", "body": "Which version?", "details": `{"question_scope":"ordinary_task"}`})
			}
			request := func(action string) *http.Request {
				r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": action}))
				r.Header.Set("X-Actor-Source", "task_token")
				r.Header.Set("X-Agent-ID", agentID)
				r.Header.Set("X-Task-ID", taskID)
				return r
			}
			for i := 0; i < 2; i++ {
				if change == "current" {
					testutil.Call(t, testHandler.SubmitChatWorkResponse, request("accept_result")).WantOneOf(200, 201)
				} else {
					testutil.Call(t, testHandler.SubmitChatWorkResponse, request("accept_result")).Want(409)
				}
			}
			issue, _ := testHandler.Queries.GetIssue(ctx, parseUUID(issueID))
			var count int
			dbfx.QueryRow(t, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, issueID).Scan(&count)
			if change == "current" {
				if issue.Status != "done" || count != 1 {
					t.Fatalf("acceptance missing: status=%s receipts=%d", issue.Status, count)
				}
				var words string
				dbfx.QueryRow(t, `SELECT content FROM issue_owner_directive WHERE issue_id=$1`, issueID).Scan(&words)
				if words != "I accept this finished wording." {
					t.Fatal("owner words changed")
				}
				dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID).Scan(&count)
				if count != 1 {
					t.Fatalf("acceptance dispatched another worker: %d", count)
				}
				testutil.Call(t, testHandler.SubmitChatWorkResponse, request("cancel")).Want(409)
			} else if issue.Status != "in_review" || count != 0 {
				t.Fatal("stale acceptance changed work")
			}
		})
	}
}
