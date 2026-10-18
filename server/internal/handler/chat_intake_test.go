package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestChatIntakeBindingAndOwnerContinuation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Fatal("isolated test database is required")
	}
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Intake assistant", nil)
	workerID := createHandlerTestAgent(t, "Intake worker", nil)
	sessionID := dbfx.ChatSession(t, agentID, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
	otherSession := dbfx.ChatSession(t, workerID, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
	issueID := dbfx.Issue(t, "Continue bounded intake", testutil.Cols{"status": "todo", "assignee_type": "agent", "assignee_id": workerID})
	otherIssue := dbfx.Issue(t, "Second issue in the same conversation", testutil.Cols{"status": "todo", "assignee_type": "agent", "assignee_id": workerID})
	taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "status": "running", "chat_session_id": sessionID, "started_at": testutil.Raw("now()")})
	bind := func(body any, agent string) *httptest.ResponseRecorder {
		r := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/api/chat/intake", body))
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Task-ID", taskID)
		r.Header.Set("X-Agent-ID", agent)
		w := httptest.NewRecorder()
		testHandler.CreateChatIntake(w, r)
		return w
	}
	boundRequest := "Verify this existing issue without changing scope.\n" + strings.Repeat("Preserve existing scope. ", 100) + "\nattachment-id: 10000000-0000-0000-0000-000000000001 sha256: " + strings.Repeat("a", 64)
	request := map[string]any{"issue_id": issueID, "request": boundRequest}
	if w := bind(map[string]any{"issue_id": issueID, "request": "spoof", "chat_session_id": otherSession}, agentID); w.Code != http.StatusBadRequest {
		t.Fatalf("session injection: %d %s", w.Code, w.Body)
	}
	if w := bind(request, workerID); w.Code != http.StatusForbidden {
		t.Fatalf("agent injection: %d %s", w.Code, w.Body)
	}
	if w := bind(map[string]any{"issue_id": uuid.NewString(), "request": "wrong workspace"}, agentID); w.Code != http.StatusNotFound {
		t.Fatalf("foreign issue: %d %s", w.Code, w.Body)
	}

	// Canonical authority admission: a refused grant rolls back the direction and comment as well as the task.
	previousAdmission := testHandler.TaskService.PrepareWorkflowGuardedAuthority
	t.Cleanup(func() { testHandler.TaskService.PrepareWorkflowGuardedAuthority = previousAdmission })
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = func(context.Context, *db.Queries, db.Issue, db.Agent, pgtype.UUID, string, []byte) ([]byte, bool, error) {
		return nil, false, errors.New("test authority refused")
	}
	if denied := bind(request, agentID); denied.Code != http.StatusConflict {
		t.Fatalf("authority denial: %d %s", denied.Code, denied.Body)
	}
	var deniedWrites int
	if err := testPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)+(SELECT count(*) FROM inbox_item WHERE issue_id=$1)`, issueID).Scan(&deniedWrites); err != nil {
		t.Fatal(err)
	}
	if deniedWrites != 0 {
		t.Fatalf("authority denial leaked %d writes", deniedWrites)
	}
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = previousAdmission

	w := bind(request, agentID)
	if w.Code != http.StatusCreated {
		t.Fatalf("bind: %d %s", w.Code, w.Body)
	}
	var intake map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &intake); err != nil {
		t.Fatal(err)
	}
	intakeID := intake["intake_id"].(string)
	if intake["inbox_item_id"] != intakeID || intake["chat_session_id"] != sessionID || intake["state"] != "continued" || intake["task_id"] == "" {
		t.Fatalf("not an admitted and queued receipt: %v", intake)
	}
	if w := bind(request, agentID); w.Code != http.StatusOK {
		t.Fatalf("replay: %d %s", w.Code, w.Body)
	}
	if w := bind(map[string]any{"issue_id": otherIssue, "request": request["request"]}, agentID); w.Code != http.StatusCreated {
		t.Fatalf("second issue: %d %s", w.Code, w.Body)
	}
	var links, tasks int
	_ = testPool.QueryRow(ctx, `SELECT count(*) FROM chat_issue_link WHERE chat_session_id=$1 AND issue_id=$2 AND source='binding_receipt'`, sessionID, issueID).Scan(&links)
	_ = testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID).Scan(&tasks)
	if links != 1 || tasks != 1 {
		t.Fatalf("bind must queue worker directly: links=%d tasks=%d", links, tasks)
	}

	assertIntakeClaimContent(t, intake["task_id"].(string), boundRequest)

	// Even a terminal task cannot be duplicated by replaying the accepted intake.
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, intake["task_id"]); err != nil {
		t.Fatal(err)
	}
	if replay := bind(request, agentID); replay.Code != http.StatusOK {
		t.Fatalf("terminal replay: %d %s", replay.Code, replay.Body)
	}
	_ = testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID).Scan(&tasks)
	var directives, comments int
	_ = testPool.QueryRow(ctx, `SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1`, issueID).Scan(&directives)
	_ = testPool.QueryRow(ctx, `SELECT count(*) FROM comment WHERE issue_id=$1`, issueID).Scan(&comments)
	if tasks != 1 || directives != 1 || comments != 1 {
		t.Fatalf("duplicate continuation: tasks=%d directives=%d comments=%d", tasks, directives, comments)
	}
}

func assertIntakeClaimContent(t *testing.T, taskID, request string, notes ...string) {
	t.Helper()
	ctx := context.Background()
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := testHandler.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{ID: task.RuntimeID, WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	runtimeID := uuidToString(task.RuntimeID)
	r := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "intake-context-regression")
	response, _, _, _, failure := testHandler.buildClaimedTaskResponse(r, &task, runtime, runtimeID, testWorkspaceID)
	if failure != nil {
		t.Fatalf("build queued worker context: %+v", failure)
	}
	if !strings.Contains(response.TriggerCommentContent, request) {
		t.Fatalf("queued worker lost complete bound request: %q", response.TriggerCommentContent)
	}
	for _, note := range notes {
		if !strings.Contains(response.TriggerCommentContent, note) {
			t.Fatalf("queued worker lost owner note: %q", response.TriggerCommentContent)
		}
	}
}
