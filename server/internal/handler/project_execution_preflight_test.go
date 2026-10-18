package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func projectRoutePreflightFixture(t *testing.T) (projectID, agentID, agentDaemon string) {
	t.Helper()
	ctx := context.Background()
	projectID = dbfx.Project(t, "Project route preflight "+uuid.NewString())
	wrongDaemon := uuid.NewString()
	approvedDaemon := uuid.NewString()
	runtimeID := dbfx.Runtime(t, "Wrong project machine", testutil.Cols{
		"daemon_id": wrongDaemon,
		"status":    "online",
	})
	agentID = dbfx.Agent(t, "Wrong project worker", runtimeID)

	var previous []byte
	if err := testPool.QueryRow(ctx, `SELECT settings FROM workspace WHERE id=$1`, testWorkspaceID).Scan(&previous); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE workspace SET settings=$2 WHERE id=$1`, testWorkspaceID, previous)
	})
	routes, _ := json.Marshal(map[string][]string{projectID: {approvedDaemon}})
	if _, err := testHandler.Queries.UpdateWorkspaceProjectExecutionRoutes(ctx, db.UpdateWorkspaceProjectExecutionRoutesParams{
		ID: util.MustParseUUID(testWorkspaceID), Routes: routes,
	}); err != nil {
		t.Fatal(err)
	}
	return projectID, agentID, wrongDaemon
}

func requireProjectRouteDenied(t *testing.T, response *testutil.Response) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	response.Want(http.StatusConflict).JSON(&body)
	if body.Code != "project_execution_route_denied" {
		t.Fatalf("error code = %q, want project_execution_route_denied", body.Code)
	}
}

func requireProjectCapabilityDenied(t *testing.T, response *testutil.Response) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	response.Want(http.StatusConflict).JSON(&body)
	if body.Code != "project_execution_capability_denied" {
		t.Fatalf("error code = %q, want project_execution_capability_denied", body.Code)
	}
}

func TestProjectExecutionRoutePreflightRejectsPreviewAndCreate(t *testing.T) {
	projectID, agentID, agentDaemon := projectRoutePreflightFixture(t)

	preview := testutil.Call(t, testHandler.PreviewIssueTrigger, newRequest(http.MethodPost,
		"/api/issues/preview-trigger?workspace_id="+testWorkspaceID, map[string]any{
			"is_create": true, "status": "todo", "project_id": projectID,
			"assignee_type": "agent", "assignee_id": agentID,
		}))
	requireProjectRouteDenied(t, preview)

	title := "Rejected route create " + uuid.NewString()
	create := testutil.Call(t, testHandler.CreateIssue, newRequest(http.MethodPost,
		"/api/issues?workspace_id="+testWorkspaceID, map[string]any{
			"title": title, "status": "todo", "project_id": projectID,
			"assignee_type": "agent", "assignee_id": agentID,
		}))
	requireProjectRouteDenied(t, create)
	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title); got != 0 {
		t.Fatalf("denied create persisted %d issues", got)
	}

	// A squad resolves through its leader's runtime and is denied by the same
	// deterministic machine route.
	squadID := dbfx.Squad(t, "Wrong project squad", agentID)
	squadPreview := testutil.Call(t, testHandler.PreviewIssueTrigger, newRequest(http.MethodPost,
		"/api/issues/preview-trigger?workspace_id="+testWorkspaceID, map[string]any{
			"is_create": true, "status": "todo", "project_id": projectID,
			"assignee_type": "squad", "assignee_id": squadID,
		}))
	requireProjectRouteDenied(t, squadPreview)

	// Positive control: changing the declared route to the agent's actual daemon
	// restores both preview and create without changing the worker.
	routes, _ := json.Marshal(map[string][]string{projectID: {agentDaemon}})
	if _, err := testHandler.Queries.UpdateWorkspaceProjectExecutionRoutes(context.Background(), db.UpdateWorkspaceProjectExecutionRoutesParams{
		ID: util.MustParseUUID(testWorkspaceID), Routes: routes,
	}); err != nil {
		t.Fatal(err)
	}
	allowedPreview := testutil.Call(t, testHandler.PreviewIssueTrigger, newRequest(http.MethodPost,
		"/api/issues/preview-trigger?workspace_id="+testWorkspaceID, map[string]any{
			"is_create": true, "status": "todo", "project_id": projectID,
			"assignee_type": "agent", "assignee_id": agentID,
		})).Want(http.StatusOK)
	var previewBody IssueTriggerPreviewResponse
	allowedPreview.JSON(&previewBody)
	if previewBody.TotalCount != 1 {
		t.Fatalf("allowed route preview = %+v, want one run", previewBody)
	}
	allowedTitle := "Allowed route create " + uuid.NewString()
	var created IssueResponse
	testutil.Call(t, testHandler.CreateIssue, newRequest(http.MethodPost,
		"/api/issues?workspace_id="+testWorkspaceID, map[string]any{
			"title": allowedTitle, "status": "todo", "project_id": projectID,
			"assignee_type": "agent", "assignee_id": agentID,
		})).Want(http.StatusCreated).JSON(&created)
	t.Cleanup(func() {
		testHandler.DeleteIssue(httptest.NewRecorder(), withURLParam(newRequest(http.MethodDelete, "/api/issues/"+created.ID, nil), "id", created.ID))
	})
	if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2`, created.ID, agentID); got != 1 {
		t.Fatalf("allowed create enqueued %d tasks, want 1", got)
	}
}

func TestProjectExecutionRoutePreflightRejectsSingleAssignment(t *testing.T) {
	projectID, agentID, _ := projectRoutePreflightFixture(t)
	assertUnchanged := func(issueID string) {
		t.Helper()
		issue, err := testHandler.Queries.GetIssue(context.Background(), util.MustParseUUID(issueID))
		if err != nil {
			t.Fatal(err)
		}
		if issue.AssigneeID.Valid || issue.AssigneeType.Valid {
			t.Fatalf("denied assignment changed issue assignee: %+v", issue)
		}
		if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID); got != 0 {
			t.Fatalf("denied assignment enqueued %d tasks", got)
		}
	}

	singleID := dbfx.Issue(t, "Single route preflight", testutil.Cols{"project_id": projectID, "status": "todo"})
	single := testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest(http.MethodPut,
		"/api/issues/"+singleID, map[string]any{
			"assignee_type": "agent", "assignee_id": agentID,
		}), "id", singleID))
	requireProjectRouteDenied(t, single)
	assertUnchanged(singleID)

	// suppress_run is the deliberate parking path. It may record ownership on
	// an active-status issue without claiming that work can start yet.
	parkedID := dbfx.Issue(t, "Suppressed route assignment", testutil.Cols{"project_id": projectID, "status": "todo"})
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest(http.MethodPut,
		"/api/issues/"+parkedID, map[string]any{
			"assignee_type": "agent", "assignee_id": agentID, "suppress_run": true,
		}), "id", parkedID)).Want(http.StatusOK)
	parked, err := testHandler.Queries.GetIssue(context.Background(), util.MustParseUUID(parkedID))
	if err != nil {
		t.Fatal(err)
	}
	if !parked.AssigneeID.Valid || util.UUIDToString(parked.AssigneeID) != agentID {
		t.Fatalf("suppressed assignment did not preserve ownership: %+v", parked)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, parkedID); got != 0 {
		t.Fatalf("suppressed assignment enqueued %d tasks", got)
	}
}

func TestProjectExecutionRoutePreflightRejectsBeforeActiveRunSuppression(t *testing.T) {
	projectID, agentID, _ := projectRoutePreflightFixture(t)
	issueID := dbfx.Issue(t, "Active run route preflight", testutil.Cols{"project_id": projectID, "status": "todo"})
	agent, err := testHandler.Queries.GetAgent(context.Background(), util.MustParseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, context)
		VALUES ($1, $2, $3, 'running', 0, '{}')`, agentID, agent.RuntimeID, issueID)

	response := testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest(http.MethodPut,
		"/api/issues/"+issueID, map[string]any{
			"assignee_type": "agent", "assignee_id": agentID,
		}), "id", issueID))
	requireProjectRouteDenied(t, response)

	issue, err := testHandler.Queries.GetIssue(context.Background(), util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	if issue.AssigneeID.Valid || issue.AssigneeType.Valid {
		t.Fatalf("active run hid denied assignment: %+v", issue)
	}
}

func TestProjectExecutionRoutePreflightRejectsBatchBeforeAnyMutation(t *testing.T) {
	ctx := context.Background()
	allowedProjectID := dbfx.Project(t, "Allowed batch route "+uuid.NewString())
	deniedProjectID := dbfx.Project(t, "Denied batch route "+uuid.NewString())
	agentDaemon := uuid.NewString()
	runtimeID := dbfx.Runtime(t, "Batch route worker", testutil.Cols{
		"daemon_id": agentDaemon,
		"status":    "online",
	})
	agentID := dbfx.Agent(t, "Batch route worker", runtimeID)
	firstID := dbfx.Issue(t, "First batch route issue", testutil.Cols{"project_id": allowedProjectID, "status": "todo"})
	secondID := dbfx.Issue(t, "Second batch route issue", testutil.Cols{"project_id": deniedProjectID, "status": "todo"})

	var previous []byte
	if err := testPool.QueryRow(ctx, `SELECT settings FROM workspace WHERE id=$1`, testWorkspaceID).Scan(&previous); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE workspace SET settings=$2 WHERE id=$1`, testWorkspaceID, previous)
	})
	routes, _ := json.Marshal(map[string][]string{
		allowedProjectID: {agentDaemon},
		deniedProjectID:  {uuid.NewString()},
	})
	if _, err := testHandler.Queries.UpdateWorkspaceProjectExecutionRoutes(ctx, db.UpdateWorkspaceProjectExecutionRoutesParams{
		ID: util.MustParseUUID(testWorkspaceID), Routes: routes,
	}); err != nil {
		t.Fatal(err)
	}

	response := testutil.Call(t, testHandler.BatchUpdateIssues, newRequest(http.MethodPost,
		"/api/issues/batch-update?workspace_id="+testWorkspaceID, map[string]any{
			"issue_ids": []string{firstID, secondID},
			"updates": map[string]any{
				"assignee_type": "agent", "assignee_id": agentID,
			},
		}))
	requireProjectRouteDenied(t, response)

	for _, issueID := range []string{firstID, secondID} {
		issue, err := testHandler.Queries.GetIssue(ctx, util.MustParseUUID(issueID))
		if err != nil {
			t.Fatal(err)
		}
		if issue.AssigneeID.Valid || issue.AssigneeType.Valid {
			t.Fatalf("denied batch partially mutated issue %s: %+v", issueID, issue)
		}
		if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID); got != 0 {
			t.Fatalf("denied batch enqueued %d tasks for issue %s", got, issueID)
		}
	}
}

func TestProjectExecutionCapabilityPreflightRejectsBatchBeforeAnyMutation(t *testing.T) {
	ctx := context.Background()
	allowedProjectID := dbfx.Project(t, "Allowed batch capability "+uuid.NewString())
	deniedProjectID := dbfx.Project(t, "Denied batch capability "+uuid.NewString())
	runtimeID := dbfx.Runtime(t, "Batch capability worker")
	agentID := dbfx.Agent(t, "Batch capability worker", runtimeID)
	skillID := dbfx.Insert(t, "skill", testutil.Cols{
		"workspace_id": testWorkspaceID, "name": "Batch required skill " + uuid.NewString(),
		"description": "", "content": "", "created_by": testUserID,
	})
	dbfx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": skillID}, "agent_id=$1 AND skill_id=$2", agentID, skillID)
	firstID := dbfx.Issue(t, "First batch capability issue", testutil.Cols{"project_id": allowedProjectID, "status": "todo"})
	secondID := dbfx.Issue(t, "Second batch capability issue", testutil.Cols{"project_id": deniedProjectID, "status": "todo"})

	var previous []byte
	if err := testPool.QueryRow(ctx, `SELECT settings FROM workspace WHERE id=$1`, testWorkspaceID).Scan(&previous); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE workspace SET settings=$2 WHERE id=$1`, testWorkspaceID, previous)
	})
	rules, _ := json.Marshal(map[string]service.ProjectExecutionCapabilityRule{
		allowedProjectID: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{agentID}},
		deniedProjectID:  {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{}},
	})
	if _, err := testHandler.Queries.UpdateWorkspaceProjectExecutionCapabilities(ctx, db.UpdateWorkspaceProjectExecutionCapabilitiesParams{ID: util.MustParseUUID(testWorkspaceID), Capabilities: rules}); err != nil {
		t.Fatal(err)
	}

	response := testutil.Call(t, testHandler.BatchUpdateIssues, newRequest(http.MethodPost,
		"/api/issues/batch-update?workspace_id="+testWorkspaceID, map[string]any{
			"issue_ids": []string{firstID, secondID},
			"updates":   map[string]any{"assignee_type": "agent", "assignee_id": agentID},
		}))
	requireProjectCapabilityDenied(t, response)

	for _, issueID := range []string{firstID, secondID} {
		issue, err := testHandler.Queries.GetIssue(ctx, util.MustParseUUID(issueID))
		if err != nil {
			t.Fatal(err)
		}
		if issue.AssigneeID.Valid || issue.AssigneeType.Valid {
			t.Fatalf("denied capability batch partially mutated issue %s: %+v", issueID, issue)
		}
		if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID); got != 0 {
			t.Fatalf("denied capability batch enqueued %d tasks for issue %s", got, issueID)
		}
	}
}

func TestProjectExecutionCapabilityPreflightDirectAndSquadLeader(t *testing.T) {
	ctx := context.Background()
	projectID := dbfx.Project(t, "Capability preflight "+uuid.NewString())
	runtimeID := dbfx.Runtime(t, "Capability preflight worker")
	agentID := dbfx.Agent(t, "Capability preflight worker", runtimeID)
	skillID := dbfx.Insert(t, "skill", testutil.Cols{
		"workspace_id": testWorkspaceID, "name": "Preflight required skill " + uuid.NewString(),
		"description": "", "content": "", "created_by": testUserID,
	})
	dbfx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": skillID}, "agent_id=$1 AND skill_id=$2", agentID, skillID)
	squadID := dbfx.Squad(t, "Capability preflight squad", agentID)
	var previous []byte
	if err := testPool.QueryRow(ctx, `SELECT settings FROM workspace WHERE id=$1`, testWorkspaceID).Scan(&previous); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE workspace SET settings=$2 WHERE id=$1`, testWorkspaceID, previous)
	})
	setRules := func(eligible []string) {
		t.Helper()
		rules, _ := json.Marshal(map[string]service.ProjectExecutionCapabilityRule{
			projectID: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: eligible},
		})
		if _, err := testHandler.Queries.UpdateWorkspaceProjectExecutionCapabilities(ctx, db.UpdateWorkspaceProjectExecutionCapabilitiesParams{ID: util.MustParseUUID(testWorkspaceID), Capabilities: rules}); err != nil {
			t.Fatal(err)
		}
	}
	setRules([]string{})
	for _, assignment := range []struct {
		name, assigneeType, assigneeID string
	}{
		{name: "direct", assigneeType: "agent", assigneeID: agentID},
		{name: "squad", assigneeType: "squad", assigneeID: squadID},
	} {
		preview := testutil.Call(t, testHandler.PreviewIssueTrigger, newRequest(http.MethodPost,
			"/api/issues/preview-trigger?workspace_id="+testWorkspaceID, map[string]any{
				"is_create": true, "status": "todo", "project_id": projectID,
				"assignee_type": assignment.assigneeType, "assignee_id": assignment.assigneeID,
			}))
		requireProjectCapabilityDenied(t, preview)
		title := "Denied capability " + assignment.name + " " + uuid.NewString()
		create := testutil.Call(t, testHandler.CreateIssue, newRequest(http.MethodPost,
			"/api/issues?workspace_id="+testWorkspaceID, map[string]any{
				"title": title, "status": "todo", "project_id": projectID,
				"assignee_type": assignment.assigneeType, "assignee_id": assignment.assigneeID,
			}))
		requireProjectCapabilityDenied(t, create)
		if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title); got != 0 {
			t.Fatalf("denied %s create persisted %d issues", assignment.name, got)
		}
	}

	setRules([]string{agentID})
	for _, assignment := range []struct {
		name, assigneeType, assigneeID string
	}{
		{name: "direct", assigneeType: "agent", assigneeID: agentID},
		{name: "squad", assigneeType: "squad", assigneeID: squadID},
	} {
		var body IssueTriggerPreviewResponse
		testutil.Call(t, testHandler.PreviewIssueTrigger, newRequest(http.MethodPost,
			"/api/issues/preview-trigger?workspace_id="+testWorkspaceID, map[string]any{
				"is_create": true, "status": "todo", "project_id": projectID,
				"assignee_type": assignment.assigneeType, "assignee_id": assignment.assigneeID,
			})).Want(http.StatusOK).JSON(&body)
		if body.TotalCount != 1 || len(body.Triggers) != 1 || body.Triggers[0].AgentID != agentID {
			t.Fatalf("allowed %s preview did not resolve to explicit leader: %+v", assignment.name, body)
		}
	}
}
