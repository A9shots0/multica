package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestProjectExecutionRouteSettingsProtection(t *testing.T) {
	ctx := context.Background()
	ws := dbfx.Workspace(t, "Execution route settings", "routes-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	fx := testutil.New(testPool, ws, testUserID)
	project := fx.Project(t, "Host-bound")
	host := uuid.NewString()
	fx.Runtime(t, "Approved machine", testutil.Cols{"daemon_id": host})
	routes := map[string][]string{project: {host}}
	call := func(body any, want int) {
		t.Helper()
		r := withURLParam(newRequest("PATCH", "/api/workspaces/"+ws, body), "id", ws)
		testutil.Call(t, testHandler.UpdateWorkspace, r).Want(want)
	}
	call(map[string]any{"project_execution_routes": routes}, http.StatusOK)
	for _, actor := range []string{"task_token", "cloud_pat"} {
		r := withURLParam(newRequest("PATCH", "/api/workspaces/"+ws, map[string]any{"project_execution_routes": map[string]any{}}), "id", ws)
		r.Header.Set("X-Actor-Source", actor)
		testutil.Call(t, testHandler.UpdateWorkspace, r).Want(http.StatusForbidden)
	}
	call(map[string]any{"settings": map[string]any{"unrelated": "kept"}}, http.StatusOK)
	call(map[string]any{"settings": map[string]any{"unrelated": "round-trip", "project_execution_routes": routes}}, http.StatusOK)
	call(map[string]any{"settings": map[string]any{"project_execution_routes": map[string]any{}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_routes": map[string][]string{uuid.NewString(): {host}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_routes": map[string][]string{project: {uuid.NewString()}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_routes": routes, "name": "mixed"}, http.StatusBadRequest)
	fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, ws, testUserID)
	call(map[string]any{"project_execution_routes": map[string]any{}}, http.StatusForbidden)
	var raw []byte
	fx.QueryRow(t, `SELECT settings FROM workspace WHERE id=$1`, ws).Scan(&raw)
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	expected, _ := json.Marshal(routes)
	var actual map[string][]string
	if err := json.Unmarshal(settings["project_execution_routes"], &actual); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(actual)
	if string(got) != string(expected) {
		t.Fatalf("settings erased route: %s", raw)
	}
	// The generic SQL writer must preserve the current policy too, not just
	// the HTTP handler's older snapshot, including concurrent settings saves.
	if _, err := testHandler.Queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{ID: parseUUID(ws), Settings: []byte(`{"unrelated":"db-writer"}`)}); err != nil {
		t.Fatal(err)
	}
	fx.QueryRow(t, `SELECT settings->'project_execution_routes' FROM workspace WHERE id=$1`, ws).Scan(&raw)
	if err := json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	got, _ = json.Marshal(actual)
	if string(got) != string(expected) {
		t.Fatal("generic SQL writer erased route")
	}
}

func TestProjectExecutionRouteClaimWithholdsPayload(t *testing.T) {
	ctx := context.Background()
	ws := dbfx.Workspace(t, "Execution route claim", "route-claim-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	fx := testutil.New(testPool, ws, testUserID)
	project := fx.Project(t, "Host-bound")
	runtimeID := fx.Runtime(t, "Wrong machine", testutil.Cols{"daemon_id": uuid.NewString()})
	agent := fx.Agent(t, "Wrong worker", runtimeID)
	issue := fx.Issue(t, "No payload on wrong machine", testutil.Cols{"project_id": project})
	taskID := fx.Task(t, agent, testutil.Cols{"runtime_id": runtimeID, "issue_id": issue, "status": "dispatched", "dispatched_at": testutil.Raw("now()")})
	routes, _ := json.Marshal(map[string][]string{project: {uuid.NewString()}})
	if _, err := testHandler.Queries.UpdateWorkspaceProjectExecutionRoutes(ctx, db.UpdateWorkspaceProjectExecutionRoutesParams{ID: parseUUID(ws), Routes: routes}); err != nil {
		t.Fatal(err)
	}
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := testHandler.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{ID: parseUUID(runtimeID), WorkspaceID: parseUUID(ws)})
	if err != nil {
		t.Fatal(err)
	}
	resp, _, _, _, failure := testHandler.buildClaimedTaskResponse(newRequest("POST", "/claim", nil), &task, runtime, runtimeID, ws)
	if failure == nil || failure.status != http.StatusConflict || resp.Agent != nil || resp.AuthToken != "" || resp.IssueID != "" {
		t.Fatalf("wrong-machine claim exposed payload: %+v %+v", resp, failure)
	}
	stored, err := testHandler.Queries.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failed" || stored.StartedAt.Valid {
		t.Fatalf("wrong-machine claim not settled: %+v", stored)
	}
}

func TestProjectExecutionCapabilitySettingsProtection(t *testing.T) {
	ctx := context.Background()
	ws := dbfx.Workspace(t, "Execution capability settings", "capabilities-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	fx := testutil.New(testPool, ws, testUserID)
	project := fx.Project(t, "Capability-bound")
	runtimeID := fx.Runtime(t, "Capability worker")
	agentID := fx.Agent(t, "Approved capability worker", runtimeID)
	skillID := fx.Insert(t, "skill", testutil.Cols{
		"workspace_id": ws, "name": "Required skill " + uuid.NewString(),
		"description": "", "content": "", "created_by": testUserID,
	})
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": skillID}, "agent_id=$1 AND skill_id=$2", agentID, skillID)
	rules := map[string]service.ProjectExecutionCapabilityRule{
		project: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{agentID}},
	}
	call := func(body any, want int) {
		t.Helper()
		r := withURLParam(newRequest("PATCH", "/api/workspaces/"+ws, body), "id", ws)
		testutil.Call(t, testHandler.UpdateWorkspace, r).Want(want)
	}
	call(map[string]any{"project_execution_capabilities": rules}, http.StatusOK)
	for _, actor := range []string{"task_token", "cloud_pat"} {
		r := withURLParam(newRequest("PATCH", "/api/workspaces/"+ws, map[string]any{"project_execution_capabilities": rules}), "id", ws)
		r.Header.Set("X-Actor-Source", actor)
		testutil.Call(t, testHandler.UpdateWorkspace, r).Want(http.StatusForbidden)
	}
	call(map[string]any{"settings": map[string]any{"unrelated": "kept"}}, http.StatusOK)
	call(map[string]any{"settings": map[string]any{"project_execution_capabilities": rules, "unrelated": "round-trip"}}, http.StatusOK)
	call(map[string]any{"settings": map[string]any{"project_execution_capabilities": map[string]any{}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": []string{}, "eligible_agent_ids": []string{agentID}}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_capabilities": map[string]any{uuid.NewString(): map[string]any{"required_skill_ids": []string{skillID}, "eligible_agent_ids": []string{agentID}}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": []string{uuid.NewString()}, "eligible_agent_ids": []string{agentID}}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": []string{skillID, skillID}, "eligible_agent_ids": []string{agentID}}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": []string{skillID}, "eligible_agent_ids": []string{agentID, agentID}}}}, http.StatusBadRequest)
	tooManySkills := make([]string, 17)
	tooManyAgents := make([]string, 65)
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": tooManySkills, "eligible_agent_ids": []string{agentID}}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": []string{skillID}, "eligible_agent_ids": tooManyAgents}}}, http.StatusBadRequest)
	unskilledID := fx.Agent(t, "Unskilled capability worker", runtimeID)
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": []string{skillID}, "eligible_agent_ids": []string{unskilledID}}}}, http.StatusBadRequest)
	call(map[string]any{"project_execution_capabilities": rules, "name": "mixed"}, http.StatusBadRequest)

	// Empty eligible_agent_ids is a valid, explicit deny-all rule.
	call(map[string]any{"project_execution_capabilities": map[string]any{project: map[string]any{"required_skill_ids": []string{skillID}, "eligible_agent_ids": []string{}}}}, http.StatusOK)
	call(map[string]any{"project_execution_capabilities": rules}, http.StatusOK)
	if _, err := testHandler.Queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{ID: parseUUID(ws), Settings: []byte(`{"unrelated":"db-writer"}`)}); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	fx.QueryRow(t, `SELECT settings->'project_execution_capabilities' FROM workspace WHERE id=$1`, ws).Scan(&raw)
	var actual map[string]service.ProjectExecutionCapabilityRule
	if err := json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual) != 1 || len(actual[project].EligibleAgentIDs) != 1 || actual[project].EligibleAgentIDs[0] != agentID {
		t.Fatalf("generic SQL writer erased capability policy: %s", raw)
	}

	fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, ws, testUserID)
	call(map[string]any{"project_execution_capabilities": map[string]any{}}, http.StatusForbidden)
}

func TestProjectExecutionCapabilityClaimWithholdsPayload(t *testing.T) {
	ctx := context.Background()
	ws := dbfx.Workspace(t, "Execution capability claim", "capability-claim-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	fx := testutil.New(testPool, ws, testUserID)
	project := fx.Project(t, "Capability-bound")
	runtimeID := fx.Runtime(t, "Capability machine")
	agentID := fx.Agent(t, "Capability worker", runtimeID)
	issueID := fx.Issue(t, "No payload without capability", testutil.Cols{"project_id": project})
	taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "status": "dispatched", "dispatched_at": testutil.Raw("now()")})
	skillID := fx.Insert(t, "skill", testutil.Cols{
		"workspace_id": ws, "name": "Missing required skill " + uuid.NewString(),
		"description": "", "content": "", "created_by": testUserID,
	})
	rules, _ := json.Marshal(map[string]service.ProjectExecutionCapabilityRule{
		project: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{agentID}},
	})
	if _, err := testHandler.Queries.UpdateWorkspaceProjectExecutionCapabilities(ctx, db.UpdateWorkspaceProjectExecutionCapabilitiesParams{ID: parseUUID(ws), Capabilities: rules}); err != nil {
		t.Fatal(err)
	}
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := testHandler.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{ID: parseUUID(runtimeID), WorkspaceID: parseUUID(ws)})
	if err != nil {
		t.Fatal(err)
	}
	resp, _, _, _, failure := testHandler.buildClaimedTaskResponse(newRequest("POST", "/claim", nil), &task, runtime, runtimeID, ws)
	if failure == nil || failure.status != http.StatusConflict || failure.outcome != "error_project_execution_capability" || resp.Agent != nil || resp.AuthToken != "" || resp.IssueID != "" {
		t.Fatalf("capability-denied claim exposed payload: %+v %+v", resp, failure)
	}
}
