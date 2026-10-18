package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatIntakeContinuationRequiresInvocationPermission(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Fatal("isolated test database is required")
	}
	ctx := context.Background()
	for _, assignment := range []string{"agent", "squad"} {
		for _, mode := range []string{"private", "public_to"} {
			t.Run(assignment+"/"+mode, func(t *testing.T) {
				worker, _, _ := privateAgentTestFixture(t)
				if _, err := testPool.Exec(ctx, `UPDATE agent SET permission_mode=$2 WHERE id=$1`, worker, mode); err != nil {
					t.Fatal(err)
				}
				// An admin can see this agent, but has no implicit invocation grant.
				if _, err := testPool.Exec(ctx, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_, _ = testPool.Exec(ctx, `UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID)
				})
				assignee := worker
				if assignment == "squad" {
					assignee = dbfx.Squad(t, "Restricted intake squad", worker)
				}
				issue := dbfx.Issue(t, "Readable restricted intake", testutil.Cols{"status": "todo", "assignee_type": assignment, "assignee_id": assignee})
				assistant := createHandlerTestAgent(t, "Owner intake assistant", nil)
				session := dbfx.ChatSession(t, assistant, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
				task := dbfx.Task(t, assistant, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "status": "running", "chat_session_id": session, "started_at": testutil.Raw("now()")})
				bindCall := func() *httptest.ResponseRecorder {
					request := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", map[string]any{"issue_id": issue, "request": "Continue this existing issue."}))
					request.Header.Set("X-Actor-Source", "task_token")
					request.Header.Set("X-Task-ID", task)
					request.Header.Set("X-Agent-ID", assistant)
					bound := httptest.NewRecorder()
					testHandler.CreateChatIntake(bound, request)
					return bound
				}
				denied := bindCall()
				if denied.Code != http.StatusForbidden {
					t.Fatalf("restricted %s invocation: %d %s", assignment, denied.Code, denied.Body)
				}
				var writes int
				if err := testPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)+(SELECT count(*) FROM inbox_item WHERE issue_id=$1)`, issue).Scan(&writes); err != nil {
					t.Fatal(err)
				}
				if writes != 0 {
					t.Fatalf("denied invocation mutated request: writes=%d", writes)
				}
				if assignment == "agent" && mode == "public_to" {
					if _, err := testPool.Exec(ctx, `INSERT INTO agent_invocation_target(agent_id,target_type,target_id) VALUES($1,'member',$2)`, worker, testUserID); err != nil {
						t.Fatal(err)
					}
					allowed := bindCall()
					if allowed.Code != http.StatusCreated {
						t.Fatalf("explicitly allowlisted owner continuation: %d %s", allowed.Code, allowed.Body)
					}
				}
			})
		}
	}
}
