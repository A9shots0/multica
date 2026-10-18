package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// markLatestTaskCompleted marks the most recent task for this issue as
// completed, mirroring a realistic "the agent finished the work" state
// before the owner clicks Deploy — CreateIssue auto-enqueues a task for an
// agent-assigned issue, and idx_one_pending_task_per_issue_agent_thread only
// guards queued/dispatched tasks, so a still-pending implementation task
// would otherwise make a fresh deploy enqueue fail with a conflict that has
// nothing to do with deploy readiness.
func markLatestTaskCompleted(t *testing.T, ctx context.Context, issueID string) {
	t.Helper()
	if _, err := testPool.Exec(ctx, `
		UPDATE agent_task_queue SET status = 'completed', completed_at = now()
		WHERE id = (SELECT id FROM agent_task_queue WHERE issue_id = $1 ORDER BY created_at DESC LIMIT 1)
	`, issueID); err != nil {
		t.Fatalf("mark latest task completed: %v", err)
	}
}

func testAgentIDByName(t *testing.T, ctx context.Context, name string) string {
	t.Helper()
	var agentID string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, name,
	).Scan(&agentID); err != nil {
		t.Fatalf("failed to find test agent %q: %v", name, err)
	}
	return agentID
}

// TestTriggerIssueDeploy_EnqueuesTaskWithHandoffNote verifies the endpoint
// dispatches a real agent task (the same enqueue path every assign/promote
// action already uses) carrying deploy-specific instructions, not a generic
// or empty handoff note.
func TestTriggerIssueDeploy_EnqueuesTaskWithHandoffNote(t *testing.T) {
	ctx := context.Background()
	agentID := testAgentIDByName(t, ctx, "Handler Test Agent")

	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Deploy handler test issue",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	w := testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated)
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}

	// CreateIssue auto-enqueues a task for this agent-assigned issue; complete
	// it first so the deploy trigger models "work is done, now ship it"
	// rather than colliding with a still-pending implementation task.
	markLatestTaskCompleted(t, ctx, created.ID)

	var tasksBefore int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, created.ID).Scan(&tasksBefore); err != nil {
		t.Fatalf("count tasks before deploy: %v", err)
	}

	deployReq := newRequest("POST", "/api/issues/"+created.ID+"/deploy?workspace_id="+testWorkspaceID, nil)
	deployReq = withURLParam(deployReq, "id", created.ID)
	testutil.Call(t, testHandler.TriggerIssueDeploy, deployReq).Want(http.StatusOK)

	var handoffNote string
	if err := testPool.QueryRow(ctx, `
		SELECT handoff_note FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, created.ID).Scan(&handoffNote); err != nil {
		t.Fatalf("load deploy task handoff note: %v", err)
	}

	if !strings.Contains(handoffNote, "owner approved deploying") {
		t.Fatalf("handoff note does not carry deploy instructions: %q", handoffNote)
	}
	if !strings.Contains(handoffNote, "STOP") {
		t.Fatalf("handoff note is missing the fail-closed instruction for an undocumented deploy process: %q", handoffNote)
	}

	var tasksAfter int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, created.ID).Scan(&tasksAfter); err != nil {
		t.Fatalf("count tasks after deploy: %v", err)
	}
	if tasksAfter != tasksBefore+1 {
		t.Fatalf("expected exactly one new task from the deploy trigger, before=%d after=%d", tasksBefore, tasksAfter)
	}
}

// TestTriggerIssueDeploy_RejectsUnassignedIssue verifies an issue with no
// assignee cannot be deployed — there is no agent to dispatch the real work
// to, and the endpoint must fail loudly rather than silently no-op.
func TestTriggerIssueDeploy_RejectsUnassignedIssue(t *testing.T) {
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Unassigned deploy test issue",
	})
	w := testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated)
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}

	deployReq := newRequest("POST", "/api/issues/"+created.ID+"/deploy?workspace_id="+testWorkspaceID, nil)
	deployReq = withURLParam(deployReq, "id", created.ID)
	testutil.Call(t, testHandler.TriggerIssueDeploy, deployReq).Want(http.StatusBadRequest)
}

// TestTriggerIssueDeploy_RejectsWhileTaskActive verifies a clear 409 (not a
// generic 500) when the assigned agent already has a queued/dispatched task
// on this issue — a real, expected condition (double-click, agent still
// mid-run), not a wiring bug.
func TestTriggerIssueDeploy_RejectsWhileTaskActive(t *testing.T) {
	ctx := context.Background()
	agentID := testAgentIDByName(t, ctx, "Handler Test Agent")

	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Deploy conflict test issue",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	w := testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated)
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}

	// CreateIssue's own auto-enqueued task is left queued/dispatched on
	// purpose here, unlike the happy-path test above.
	deployReq := newRequest("POST", "/api/issues/"+created.ID+"/deploy?workspace_id="+testWorkspaceID, nil)
	deployReq = withURLParam(deployReq, "id", created.ID)
	testutil.Call(t, testHandler.TriggerIssueDeploy, deployReq).Want(http.StatusConflict)
}

// TestGetIssueDeployTaskStatus_ReturnsRealStatus verifies the Deployments
// page's polling endpoint reports the dispatched deploy task's actual
// status, not a static placeholder — and rejects a task id that belongs to
// a different issue (deploy-tasks/{taskId} is scoped through the issue).
func TestGetIssueDeployTaskStatus_ReturnsRealStatus(t *testing.T) {
	ctx := context.Background()
	agentID := testAgentIDByName(t, ctx, "Handler Test Agent")

	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Deploy status test issue",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	w := testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated)
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}
	markLatestTaskCompleted(t, ctx, created.ID)

	deployReq := newRequest("POST", "/api/issues/"+created.ID+"/deploy?workspace_id="+testWorkspaceID, nil)
	deployReq = withURLParam(deployReq, "id", created.ID)
	deployW := testutil.Call(t, testHandler.TriggerIssueDeploy, deployReq).Want(http.StatusOK)
	var deployed struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(deployW.Body).Decode(&deployed); err != nil {
		t.Fatalf("decode deploy response: %v", err)
	}

	statusReq := newRequest("GET", "/api/issues/"+created.ID+"/deploy-tasks/"+deployed.TaskID+"?workspace_id="+testWorkspaceID, nil)
	statusReq = withURLParams(statusReq, "id", created.ID, "taskId", deployed.TaskID)
	statusW := testutil.Call(t, testHandler.GetIssueDeployTaskStatus, statusReq).Want(http.StatusOK)

	var status deployTaskStatusResponse
	if err := json.NewDecoder(statusW.Body).Decode(&status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status.TaskID != deployed.TaskID {
		t.Fatalf("status task_id = %q, want %q", status.TaskID, deployed.TaskID)
	}
	if status.Status != "queued" {
		t.Fatalf("freshly enqueued deploy task status = %q, want %q", status.Status, "queued")
	}

	// A second, unrelated issue must not be able to read the first issue's
	// deploy task through its own {id} — the status route is issue-scoped.
	otherReq := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Unrelated issue for deploy status scoping",
	})
	otherW := testutil.Call(t, testHandler.CreateIssue, otherReq).Want(http.StatusCreated)
	var other IssueResponse
	if err := json.NewDecoder(otherW.Body).Decode(&other); err != nil {
		t.Fatalf("decode other issue: %v", err)
	}
	crossReq := newRequest("GET", "/api/issues/"+other.ID+"/deploy-tasks/"+deployed.TaskID+"?workspace_id="+testWorkspaceID, nil)
	crossReq = withURLParams(crossReq, "id", other.ID, "taskId", deployed.TaskID)
	testutil.Call(t, testHandler.GetIssueDeployTaskStatus, crossReq).Want(http.StatusNotFound)
}

// TestTriggerIssueDeploy_BatchesMultipleIssuesIntoOneTask verifies that
// deploying with additional_issue_ids dispatches exactly ONE agent task
// (not one per issue), the handoff note references every issue in the
// batch, and the response reports the full batch_issue_ids list so the
// client knows which issues to mark deployed once the one task succeeds.
func TestTriggerIssueDeploy_BatchesMultipleIssuesIntoOneTask(t *testing.T) {
	ctx := context.Background()
	agentID := testAgentIDByName(t, ctx, "Handler Test Agent")

	var projectID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project (workspace_id, title) VALUES ($1, $2) RETURNING id
	`, testWorkspaceID, "Batch deploy test project").Scan(&projectID); err != nil {
		t.Fatalf("insert test project: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID) })

	leadReq := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Batch lead issue", "assignee_type": "agent", "assignee_id": agentID, "project_id": projectID,
	})
	leadW := testutil.Call(t, testHandler.CreateIssue, leadReq).Want(http.StatusCreated)
	var lead IssueResponse
	json.NewDecoder(leadW.Body).Decode(&lead)
	markLatestTaskCompleted(t, ctx, lead.ID)

	otherReq := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Batch other issue", "assignee_type": "agent", "assignee_id": agentID, "project_id": projectID,
	})
	otherW := testutil.Call(t, testHandler.CreateIssue, otherReq).Want(http.StatusCreated)
	var other IssueResponse
	json.NewDecoder(otherW.Body).Decode(&other)
	markLatestTaskCompleted(t, ctx, other.ID)

	deployReq := newRequest("POST", "/api/issues/"+lead.ID+"/deploy?workspace_id="+testWorkspaceID, map[string]any{
		"additional_issue_ids": []string{other.ID},
	})
	deployReq = withURLParam(deployReq, "id", lead.ID)
	deployW := testutil.Call(t, testHandler.TriggerIssueDeploy, deployReq).Want(http.StatusOK)

	var resp struct {
		TaskID        string   `json:"task_id"`
		BatchIssueIDs []string `json:"batch_issue_ids"`
	}
	if err := json.NewDecoder(deployW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode deploy response: %v", err)
	}
	if len(resp.BatchIssueIDs) != 2 || resp.BatchIssueIDs[0] != lead.ID || resp.BatchIssueIDs[1] != other.ID {
		t.Fatalf("batch_issue_ids = %v, want [%s %s]", resp.BatchIssueIDs, lead.ID, other.ID)
	}

	// Exactly one real task for the lead issue, none for the other one — a
	// batch deploy must not silently dispatch N separate rebuilds.
	var leadTaskCount, otherTaskCount int
	testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND status IN ('queued','dispatched','running')`, lead.ID).Scan(&leadTaskCount)
	testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND status IN ('queued','dispatched','running')`, other.ID).Scan(&otherTaskCount)
	if leadTaskCount != 1 {
		t.Fatalf("lead issue active task count = %d, want 1", leadTaskCount)
	}
	if otherTaskCount != 0 {
		t.Fatalf("other issue active task count = %d, want 0 — batch must not dispatch a second task", otherTaskCount)
	}

	var handoffNote string
	testPool.QueryRow(ctx, `SELECT handoff_note FROM agent_task_queue WHERE id = $1`, resp.TaskID).Scan(&handoffNote)
	if !strings.Contains(handoffNote, "ONE deploy covering 2 issues") {
		t.Fatalf("batch handoff note doesn't describe the combined scope: %q", handoffNote)
	}
	if !strings.Contains(handoffNote, "Batch lead issue") || !strings.Contains(handoffNote, "Batch other issue") {
		t.Fatalf("batch handoff note doesn't name both issues: %q", handoffNote)
	}
}

// TestTriggerIssueDeploy_RejectsCrossProjectBatch verifies a batch cannot
// mix issues from different projects — a batched deploy is one project's
// redeploy, not a grab-bag.
func TestTriggerIssueDeploy_RejectsCrossProjectBatch(t *testing.T) {
	ctx := context.Background()
	agentID := testAgentIDByName(t, ctx, "Handler Test Agent")

	var projectAID, projectBID string
	testPool.QueryRow(ctx, `INSERT INTO project (workspace_id, title) VALUES ($1, $2) RETURNING id`, testWorkspaceID, "Cross-batch project A").Scan(&projectAID)
	testPool.QueryRow(ctx, `INSERT INTO project (workspace_id, title) VALUES ($1, $2) RETURNING id`, testWorkspaceID, "Cross-batch project B").Scan(&projectBID)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectAID)
		testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectBID)
	})

	leadReq := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Cross-batch lead", "assignee_type": "agent", "assignee_id": agentID, "project_id": projectAID,
	})
	leadW := testutil.Call(t, testHandler.CreateIssue, leadReq).Want(http.StatusCreated)
	var lead IssueResponse
	json.NewDecoder(leadW.Body).Decode(&lead)
	markLatestTaskCompleted(t, ctx, lead.ID)

	otherReq := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Cross-batch other", "assignee_type": "agent", "assignee_id": agentID, "project_id": projectBID,
	})
	otherW := testutil.Call(t, testHandler.CreateIssue, otherReq).Want(http.StatusCreated)
	var other IssueResponse
	json.NewDecoder(otherW.Body).Decode(&other)

	deployReq := newRequest("POST", "/api/issues/"+lead.ID+"/deploy?workspace_id="+testWorkspaceID, map[string]any{
		"additional_issue_ids": []string{other.ID},
	})
	deployReq = withURLParam(deployReq, "id", lead.ID)
	testutil.Call(t, testHandler.TriggerIssueDeploy, deployReq).Want(http.StatusBadRequest)
}

// TestMarkIssueDeployed_RecordsWithoutDispatchingATask verifies the manual
// "already deployed elsewhere" record shows up in deployment history as a
// completed deploy, without ever creating a queued/dispatched task for the
// assigned agent — nothing should actually run.
func TestMarkIssueDeployed_RecordsWithoutDispatchingATask(t *testing.T) {
	ctx := context.Background()
	agentID := testAgentIDByName(t, ctx, "Handler Test Agent")

	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Mark deployed test issue",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	w := testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated)
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}
	// CreateIssue auto-enqueues its own task for an agent-assigned issue;
	// complete it first so the only thing left to assert on is what
	// MarkIssueDeployed itself does.
	markLatestTaskCompleted(t, context.Background(), created.ID)

	markReq := newRequest("POST", "/api/issues/"+created.ID+"/mark-deployed?workspace_id="+testWorkspaceID, nil)
	markReq = withURLParam(markReq, "id", created.ID)
	testutil.Call(t, testHandler.MarkIssueDeployed, markReq).Want(http.StatusOK)

	var queuedOrDispatched int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND status IN ('queued', 'dispatched', 'running')
	`, created.ID).Scan(&queuedOrDispatched); err != nil {
		t.Fatalf("count active tasks: %v", err)
	}
	if queuedOrDispatched != 0 {
		t.Fatalf("mark-deployed must not leave any active task for the daemon to claim, found %d", queuedOrDispatched)
	}

	var handoffNote, status string
	if err := testPool.QueryRow(ctx, `
		SELECT handoff_note, status FROM agent_task_queue
		WHERE issue_id = $1 ORDER BY created_at DESC LIMIT 1
	`, created.ID).Scan(&handoffNote, &status); err != nil {
		t.Fatalf("load manual deploy record: %v", err)
	}
	if status != "completed" {
		t.Fatalf("manual deploy record status = %q, want %q", status, "completed")
	}
	if !strings.Contains(handoffNote, "Recorded manually") {
		t.Fatalf("manual deploy record note doesn't say it was manual: %q", handoffNote)
	}
}

// TestMarkIssueDeployed_RejectsUnassignedIssue mirrors the deploy trigger's
// own guard — there is no agent to attribute the record to.
func TestMarkIssueDeployed_RejectsUnassignedIssue(t *testing.T) {
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Unassigned mark-deployed test issue",
	})
	w := testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated)
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}

	markReq := newRequest("POST", "/api/issues/"+created.ID+"/mark-deployed?workspace_id="+testWorkspaceID, nil)
	markReq = withURLParam(markReq, "id", created.ID)
	testutil.Call(t, testHandler.MarkIssueDeployed, markReq).Want(http.StatusBadRequest)
}

// TestGetDeploymentHistory_ShowsCurrentlyLiveAndHistory verifies a completed
// deploy task shows up both in the flat history list and as that project's
// "currently live" entry — and that a project with no completed deploy
// simply has no currently-live entry, rather than a guessed one.
func TestGetDeploymentHistory_ShowsCurrentlyLiveAndHistory(t *testing.T) {
	ctx := context.Background()
	agentID := testAgentIDByName(t, ctx, "Handler Test Agent")

	var projectID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project (workspace_id, title) VALUES ($1, $2) RETURNING id
	`, testWorkspaceID, "Deployment history test project").Scan(&projectID); err != nil {
		t.Fatalf("insert test project: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID) })

	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Deployment history test issue",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"project_id":    projectID,
	})
	w := testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated)
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}
	markLatestTaskCompleted(t, ctx, created.ID)

	deployReq := newRequest("POST", "/api/issues/"+created.ID+"/deploy?workspace_id="+testWorkspaceID, nil)
	deployReq = withURLParam(deployReq, "id", created.ID)
	deployW := testutil.Call(t, testHandler.TriggerIssueDeploy, deployReq).Want(http.StatusOK)
	var deployed struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(deployW.Body).Decode(&deployed); err != nil {
		t.Fatalf("decode deploy response: %v", err)
	}
	// Simulate the dispatched agent actually finishing the deploy — nothing
	// runs it for real in this test.
	markLatestTaskCompleted(t, ctx, created.ID)

	historyReq := newRequest("GET", "/api/deployments/history?workspace_id="+testWorkspaceID, nil)
	historyW := testutil.Call(t, testHandler.GetDeploymentHistory, historyReq).Want(http.StatusOK)
	var resp deploymentHistoryResponse
	if err := json.NewDecoder(historyW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode deployment history: %v", err)
	}

	foundInHistory := false
	for _, entry := range resp.History {
		if entry.TaskID == deployed.TaskID {
			foundInHistory = true
			if entry.Status != "completed" {
				t.Errorf("history entry status = %q, want %q", entry.Status, "completed")
			}
			if entry.ProjectID != projectID {
				t.Errorf("history entry project_id = %q, want %q", entry.ProjectID, projectID)
			}
		}
	}
	if !foundInHistory {
		t.Fatalf("deploy task %s not found in history: %+v", deployed.TaskID, resp.History)
	}

	foundLive := false
	for _, live := range resp.CurrentlyLive {
		if live.ProjectID == projectID {
			foundLive = true
			if live.IssueID != created.ID {
				t.Errorf("currently-live issue_id = %q, want %q", live.IssueID, created.ID)
			}
		}
	}
	if !foundLive {
		t.Fatalf("project %s not found in currently_live: %+v", projectID, resp.CurrentlyLive)
	}

	// A brand-new project with no completed deploy must not appear at all.
	var otherProjectID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project (workspace_id, title) VALUES ($1, $2) RETURNING id
	`, testWorkspaceID, "Never-deployed test project").Scan(&otherProjectID); err != nil {
		t.Fatalf("insert other test project: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, otherProjectID) })
	for _, live := range resp.CurrentlyLive {
		if live.ProjectID == otherProjectID {
			t.Fatalf("never-deployed project incorrectly appeared as currently live: %+v", live)
		}
	}
}
