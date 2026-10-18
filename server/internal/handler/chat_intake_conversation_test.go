package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestChatIntakeCannotSubstituteAnUnlinkedLegacyIssue(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Fatal("isolated test database required")
	}
	for _, original := range []string{
		"Can you find our CRM UI playbook? i need a mockup done for our clients page. Im not a big fan of it and we just redid the clients detailed page with refrences in the UI playbook.",
		"I need a new dashboard made for phone farm. I had an agent write me a prompt to use.",
		"Prepare this month's supplier report.",
	} {
		t.Run(original, func(t *testing.T) {
			ctx := context.Background()
			assistant := createHandlerTestAgent(t, "Fresh conversation assistant", nil)
			worker := createHandlerTestAgent(t, "Fresh conversation worker", nil)
			session := dbfx.ChatSession(t, assistant, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
			legacy := dbfx.Issue(t, "Similar older work found by search", testutil.Cols{"status": "todo", "assignee_type": "agent", "assignee_id": worker})
			task := dbfx.Task(t, assistant, testutil.Cols{
				"runtime_id": handlerTestRuntimeID(t), "chat_session_id": session,
				"status": "running", "started_at": testutil.Raw("now()"),
				"initiator_user_id": testUserID, "originator_user_id": testUserID,
				"accountable_user_id": testUserID,
			})
			dbfx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=id WHERE id=$1`, task)
			dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "user", "content": original, "task_id": task})
			request := func() *http.Request {
				// The assistant's rewritten request cannot select old work for the owner.
				r := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": legacy, "request": "Resume the similar legacy issue found by search."}))
				r.Header.Set("X-Actor-Source", "task_token")
				r.Header.Set("X-Agent-ID", assistant)
				r.Header.Set("X-Task-ID", task)
				return r
			}
			var denial map[string]any
			testutil.Call(t, testHandler.CreateChatIntake, request()).Want(http.StatusConflict).JSON(&denial)
			if !strings.Contains(denial["error"].(string), "not linked to this conversation") {
				t.Fatalf("missing actionable target refusal: %v", denial)
			}
			var writes int
			if err := testPool.QueryRow(ctx, `SELECT
				(SELECT count(*) FROM comment WHERE issue_id=$1)+
				(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+
				(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)+
				(SELECT count(*) FROM inbox_item WHERE issue_id=$1)+
				(SELECT count(*) FROM chat_issue_link WHERE issue_id=$1)`, legacy).Scan(&writes); err != nil || writes != 0 {
				t.Fatalf("unselected legacy issue mutated: writes=%d err=%v", writes, err)
			}

			// A real owner link selects existing work without clearing its history.
			link := withURLParam(withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/"+session+"/issues", map[string]any{"issue_id": legacy})), "sessionId", session)
			for _, source := range []string{"task_token", "cloud_pat"} {
				machineLink := link.Clone(link.Context())
				machineLink.Body = newRequest(http.MethodPost, "/", map[string]any{"issue_id": legacy}).Body
				machineLink.Header.Set("X-Actor-Source", source)
				machineLink.Header.Set("X-Agent-ID", assistant)
				machineLink.Header.Set("X-Task-ID", task)
				testutil.Call(t, testHandler.LinkChatIssue, machineLink).Want(http.StatusForbidden)
			}
			testutil.Call(t, testHandler.LinkChatIssue, link).Want(http.StatusOK)
			var receipt map[string]any
			testutil.Call(t, testHandler.CreateChatIntake, request()).Want(http.StatusCreated).JSON(&receipt)
			if receipt["task_state"] != "queued" || receipt["issue_id"] != legacy {
				t.Fatalf("selected work did not continue: %v", receipt)
			}
			linked, err := testHandler.Queries.ListIssuesForChat(ctx, db.ListIssuesForChatParams{WorkspaceID: parseUUID(testWorkspaceID), ChatSessionID: parseUUID(session)})
			if err != nil || len(linked) != 1 {
				t.Fatalf("selected work link lost: %v %v", linked, err)
			}
		})
	}
}
