package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// squadWithWorkflowTemplate creates a squad whose durable template binds a
// distinct implementer and reviewer. A squad route without this controller
// definition fails closed.
func squadWithWorkflowTemplate(t *testing.T, name, leaderID, implementerID, reviewerID string) string {
	t.Helper()
	ctx := context.Background()
	var squadID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO squad (workspace_id, name, description, leader_id, creator_id)
		VALUES ($1, $2, '', $3, $4)
		RETURNING id
	`, testWorkspaceID, name, leaderID, testUserID).Scan(&squadID); err != nil {
		t.Fatalf("create squad %q: %v", name, err)
	}
	for _, member := range []struct {
		id, role string
	}{{leaderID, "leader"}, {implementerID, "implementer"}, {reviewerID, "reviewer"}} {
		if _, err := testPool.Exec(ctx, `
			INSERT INTO squad_member (squad_id, member_type, member_id, role)
			VALUES ($1, 'agent', $2, $3)
		`, squadID, member.id, member.role); err != nil {
			t.Fatalf("add %s to squad %q: %v", member.role, name, err)
		}
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO squad_workflow_template (
			squad_id, workspace_id, implementer_id, reviewer_id,
			requires_owner, requires_deployment, max_corrections
		) VALUES ($1, $2, $3, $4, false, false, 1)
	`, squadID, testWorkspaceID, implementerID, reviewerID); err != nil {
		t.Fatalf("create workflow template for squad %q: %v", name, err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM squad_workflow_template WHERE squad_id = $1`, squadID)
		testPool.Exec(context.Background(), `DELETE FROM squad_member WHERE squad_id = $1`, squadID)
		testPool.Exec(context.Background(), `DELETE FROM squad WHERE id = $1`, squadID)
	})
	return squadID
}

// configureTestSquadController explicitly supplies the controller prerequisite
// for legacy trigger fixtures. Missing-template admission tests never call it.
func configureTestSquadController(t *testing.T, squadID, leaderID, implementerID string) {
	t.Helper()
	if implementerID == "" {
		implementerID = createHandlerTestAgent(t, "Controller implementer "+squadID, nil)
	}
	for _, member := range []struct{ id, role string }{{leaderID, "leader"}, {implementerID, "implementer"}} {
		dbfx.Exec(t, `INSERT INTO squad_member (squad_id,member_type,member_id,role)
    VALUES ($1,'agent',$2,$3) ON CONFLICT (squad_id,member_type,member_id) DO NOTHING`, squadID, member.id, member.role)
	}
	dbfx.InsertNoID(t, "squad_workflow_template", testutil.Cols{
		"squad_id": squadID, "workspace_id": testWorkspaceID, "implementer_id": implementerID, "reviewer_id": leaderID,
		"requires_owner": false, "requires_deployment": false, "max_corrections": 1,
	}, "squad_id = $1", squadID)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM squad_member WHERE squad_id=$1 AND member_type='agent' AND member_id IN ($2,$3)`, squadID, leaderID, implementerID)
	})
}

func newE2EPlannedSquad(t *testing.T) (ownerH, squadID, leaderID, workerID, reviewerID string) {
	t.Helper()
	workerID, ownerH, _ = privateAgentTestFixture(t)
	prefix := strings.SplitN(ownerH, "-", 2)[0]
	leaderID = createPrivateAgentOwnedBy(t, fmt.Sprintf("silv48-e2e-leader-%s", prefix), ownerH)
	reviewerID = createPrivateAgentOwnedBy(t, fmt.Sprintf("silv48-e2e-reviewer-%s", prefix), ownerH)
	squadID = squadWithWorkflowTemplate(t, fmt.Sprintf("silv48-e2e-squad-%s", prefix), leaderID, workerID, reviewerID)
	return
}

func newLeaderTask(t *testing.T, issueID, leaderID string) string {
	t.Helper()
	var leaderTaskID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id FROM agent_task_queue
		WHERE issue_id = $1 AND agent_id = $2 AND is_leader_task
		ORDER BY created_at DESC LIMIT 1
	`, issueID, leaderID).Scan(&leaderTaskID); err != nil {
		t.Fatalf("load squad-leader task for issue %s: %v", issueID, err)
	}
	return leaderTaskID
}

func cleanupPlannedIssue(t *testing.T, issueID string) {
	t.Helper()
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM comment WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue_workflow_event WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue_workflow_plan WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})
}

func createPlannedSquadIssue(t *testing.T, ownerID, squadID, title string) IssueResponse {
	t.Helper()
	req := newRequestAs(ownerID, "POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": title, "assignee_type": "squad", "assignee_id": squadID,
	})
	var created IssueResponse
	testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated).JSON(&created)
	cleanupPlannedIssue(t, created.ID)
	return created
}

// The member-created squad issue installs a durable plan, enqueues the leader
// with the member's attribution, and admits that exact leader task when it
// delegates to the plan-bound implementer.
func TestWorkflowPlanGate_E2E_CreateAssignSquad_PrivateWorkerMentionTriggered(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	ownerH, squadID, leaderID, workerID, _ := newE2EPlannedSquad(t)
	created := createPlannedSquadIssue(t, ownerH, squadID, "SILV-48 E2E member-created squad root")

	var planState, leaderTaskID, leaderOriginator string
	if err := testPool.QueryRow(ctx,
		`SELECT current_state FROM issue_workflow_plan WHERE issue_id = $1`, created.ID,
	).Scan(&planState); err != nil {
		t.Fatalf("load workflow plan: %v", err)
	}
	if planState != "working" {
		t.Fatalf("plan state = %q, want working", planState)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT id, COALESCE(originator_user_id::text, '')
		FROM agent_task_queue
		WHERE issue_id = $1 AND agent_id = $2 AND is_leader_task
		ORDER BY created_at DESC LIMIT 1
	`, created.ID, leaderID).Scan(&leaderTaskID, &leaderOriginator); err != nil {
		t.Fatalf("load squad-leader task: %v", err)
	}
	if leaderOriginator != ownerH {
		t.Fatalf("leader task originator = %q, want %q", leaderOriginator, ownerH)
	}

	req := newRequest("POST", "/api/issues/"+created.ID+"/comments", map[string]any{
		"content": "handing the plan-bound part to [@Worker](mention://agent/" + workerID + ")",
	})
	req.Header.Set("X-Agent-ID", leaderID)
	req.Header.Set("X-Task-ID", leaderTaskID)
	req = withURLParam(req, "id", created.ID)
	var response struct {
		TriggerOutcomes []CommentTriggerOutcome `json:"trigger_outcomes"`
	}
	testutil.Call(t, testHandler.CreateComment, req).Want(http.StatusCreated).JSON(&response)

	var queuedForHuman int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM agent_task_queue
		WHERE issue_id = $1 AND agent_id = $2 AND status = 'queued' AND originator_user_id = $3
	`, created.ID, workerID, ownerH).Scan(&queuedForHuman); err != nil {
		t.Fatalf("count worker tasks: %v", err)
	}
	if queuedForHuman != 1 {
		t.Fatalf("plan-bound worker queued tasks attributed to member = %d, want 1", queuedForHuman)
	}
}

// A workflow plan does not override the Silver workspace's member-only issue
// creation policy. The negative explicitly enables that policy.
func TestWorkflowPlanGate_E2E_LeaderCannotCreatePlanBoundChild(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	restoreMemberOnlyIssueCreation(t)
	ownerH, squadID, leaderID, workerID, _ := newE2EPlannedSquad(t)
	parent := createPlannedSquadIssue(t, ownerH, squadID, "SILV-48 policy-bound child parent")
	leaderTaskID := newLeaderTask(t, parent.ID, leaderID)
	uniqueTitle := "SILV-48 policy-refused plan child " + leaderTaskID

	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": uniqueTitle, "parent_issue_id": parent.ID,
		"assignee_type": "agent", "assignee_id": workerID,
	})
	req.Header.Set("X-Agent-ID", leaderID)
	req.Header.Set("X-Task-ID", leaderTaskID)
	var response map[string]any
	testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusForbidden).JSON(&response)

	var rows int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue WHERE title = $1`, uniqueTitle).Scan(&rows); err != nil {
		t.Fatalf("count refused child rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("policy-refused leader create persisted %d child rows; want 0", rows)
	}
}

func assertRejectedAgentCreatePersistedNoIssue(t *testing.T, agentID, taskID, title string, body map[string]any) {
	t.Helper()
	body["title"] = title
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, body)
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Task-ID", taskID)
	var response map[string]any
	testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusForbidden).JSON(&response)
	var rows int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue WHERE title = $1`, title).Scan(&rows); err != nil {
		t.Fatalf("count rejected issue rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("rejected create persisted %d issue rows; want 0", rows)
	}
}

// The negative explicitly enables member-only creation; no global agent ban is
// inferred when the workspace policy is disabled.
func TestAgentCreate_RootIssue_AdmissionForbidden_ZeroRows(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	restoreMemberOnlyIssueCreation(t)
	agentID := createHandlerTestAgent(t, "silv48-policy-root-create", nil)
	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, originator_user_id, accountable_user_id)
		VALUES ($1, (SELECT runtime_id FROM agent WHERE id = $1), 'running', 0, $2, $2)
		RETURNING id
	`, agentID, testUserID).Scan(&taskID); err != nil {
		t.Fatalf("seed acting task: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID) })
	assertRejectedAgentCreatePersistedNoIssue(t, agentID, taskID, "SILV-48 policy-refused root "+taskID, map[string]any{})
}

// The policy remains the creation boundary for a non-leader task even when
// its parent has a valid workflow plan. Plan authority controls dispatch only.
func TestWorkflowPlanGate_NonLeaderChildCreateForbidden_ZeroRows(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	restoreMemberOnlyIssueCreation(t)
	ownerH, squadID, _, workerID, _ := newE2EPlannedSquad(t)
	parent := createPlannedSquadIssue(t, ownerH, squadID, "SILV-48 non-leader policy parent")
	zAgent := createHandlerTestAgent(t, "silv48-policy-non-leader", nil)
	var zTaskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, issue_id, originator_user_id, accountable_user_id)
		VALUES ($1, (SELECT runtime_id FROM agent WHERE id = $1), 'running', 0, $2, $3, $3)
		RETURNING id
	`, zAgent, parent.ID, ownerH).Scan(&zTaskID); err != nil {
		t.Fatalf("seed non-leader task: %v", err)
	}
	assertRejectedAgentCreatePersistedNoIssue(t, zAgent, zTaskID, "SILV-48 policy-refused non-leader child "+zTaskID, map[string]any{
		"parent_issue_id": parent.ID, "assignee_type": "agent", "assignee_id": workerID,
	})
}

// The policy also blocks a leader task from minting a child for an agent that
// the plan does not bind; defining a plan never widens creation authority.
func TestWorkflowPlanGate_LeaderChildOutsidePlanForbidden_ZeroRows(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	restoreMemberOnlyIssueCreation(t)
	ownerH, squadID, leaderID, _, _ := newE2EPlannedSquad(t)
	outsiderID := createPrivateAgentOwnedBy(t, "silv48-policy-outsider", ownerH)
	parent := createPlannedSquadIssue(t, ownerH, squadID, "SILV-48 outside-plan policy parent")
	leaderTaskID := newLeaderTask(t, parent.ID, leaderID)
	assertRejectedAgentCreatePersistedNoIssue(t, leaderID, leaderTaskID, "SILV-48 policy-refused outside-plan child "+leaderTaskID, map[string]any{
		"parent_issue_id": parent.ID, "assignee_type": "agent", "assignee_id": outsiderID,
	})
}

// This second named root-path regression preserves the original workflow
// acceptance selector while proving the corrected workspace-scoped policy.
func TestWorkflowPlanGate_AgentRootCreateForbidden_ZeroRows(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	restoreMemberOnlyIssueCreation(t)
	agentID := createHandlerTestAgent(t, "silv48-policy-root-create-workflow", nil)
	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, originator_user_id, accountable_user_id)
		VALUES ($1, (SELECT runtime_id FROM agent WHERE id = $1), 'running', 0, $2, $2)
		RETURNING id
	`, agentID, testUserID).Scan(&taskID); err != nil {
		t.Fatalf("seed acting task: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID) })
	assertRejectedAgentCreatePersistedNoIssue(t, agentID, taskID, "SILV-48 policy-refused workflow root "+taskID, map[string]any{})
}

func findTriggerOutcome(outcomes []CommentTriggerOutcome, targetID string) *CommentTriggerOutcome {
	for i := range outcomes {
		if outcomes[i].TargetID == targetID {
			return &outcomes[i]
		}
	}
	return nil
}

// A non-leader task on a planned squad issue cannot wake a plan worker.
func TestWorkflowPlanDispatch_WorkerMentionByNonLeader_BlockedNoTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	ownerH, squadID, _, workerID, _ := newE2EPlannedSquad(t)
	parent := createPlannedSquadIssue(t, ownerH, squadID, "SILV-48 blocked non-leader mention")
	zAgent := createHandlerTestAgent(t, "silv48-non-leader-mention", nil)
	var zTaskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, issue_id, originator_user_id, accountable_user_id)
		VALUES ($1, (SELECT runtime_id FROM agent WHERE id = $1), 'running', 0, $2, $3, $3)
		RETURNING id
	`, zAgent, parent.ID, ownerH).Scan(&zTaskID); err != nil {
		t.Fatalf("seed non-leader task: %v", err)
	}

	req := newRequest("POST", "/api/issues/"+parent.ID+"/comments", map[string]any{
		"content": "wake [@Worker](mention://agent/" + workerID + ")",
	})
	req.Header.Set("X-Agent-ID", zAgent)
	req.Header.Set("X-Task-ID", zTaskID)
	req = withURLParam(req, "id", parent.ID)
	var response struct {
		TriggerOutcomes []CommentTriggerOutcome `json:"trigger_outcomes"`
	}
	testutil.Call(t, testHandler.CreateComment, req).Want(http.StatusCreated).JSON(&response)
	got := findTriggerOutcome(response.TriggerOutcomes, workerID)
	if got == nil || got.Status != DispatchBlocked || got.ReasonCode != ReasonWorkflowPlanBlocked {
		t.Fatalf("worker outcome = %+v, want blocked/%s", got, ReasonWorkflowPlanBlocked)
	}
	var rows int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, parent.ID, workerID).Scan(&rows); err != nil {
		t.Fatalf("count worker tasks: %v", err)
	}
	if rows != 0 {
		t.Fatalf("blocked non-leader mention created %d worker tasks; want 0", rows)
	}
}

// Even the active leader cannot route to an agent outside the durable plan.
func TestWorkflowPlanDispatch_LeaderMentionOutsidePlan_BlockedNoTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	ownerH, squadID, leaderID, _, _ := newE2EPlannedSquad(t)
	outsiderID := createPrivateAgentOwnedBy(t, "silv48-plan-outsider", ownerH)
	parent := createPlannedSquadIssue(t, ownerH, squadID, "SILV-48 outside-plan mention")
	leaderTaskID := newLeaderTask(t, parent.ID, leaderID)

	req := newRequest("POST", "/api/issues/"+parent.ID+"/comments", map[string]any{
		"content": "wake [@Outsider](mention://agent/" + outsiderID + ")",
	})
	req.Header.Set("X-Agent-ID", leaderID)
	req.Header.Set("X-Task-ID", leaderTaskID)
	req = withURLParam(req, "id", parent.ID)
	var response struct {
		TriggerOutcomes []CommentTriggerOutcome `json:"trigger_outcomes"`
	}
	testutil.Call(t, testHandler.CreateComment, req).Want(http.StatusCreated).JSON(&response)
	got := findTriggerOutcome(response.TriggerOutcomes, outsiderID)
	if got == nil || got.Status != DispatchBlocked || got.ReasonCode != ReasonWorkflowPlanBlocked {
		t.Fatalf("outsider outcome = %+v, want blocked/%s", got, ReasonWorkflowPlanBlocked)
	}
	var rows int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, parent.ID, outsiderID).Scan(&rows); err != nil {
		t.Fatalf("count outsider tasks: %v", err)
	}
	if rows != 0 {
		t.Fatalf("outside-plan mention created %d tasks; want 0", rows)
	}
}

// A leader-stamped task on squad-owned work still fails closed when no durable
// workflow plan exists.
func TestWorkflowPlanDispatch_SquadWithoutPlanLeaderMentionBlockedNoTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	leaderID := createHandlerTestAgent(t, "silv48-no-plan-leader", nil)
	workerID := createHandlerTestAgent(t, "silv48-no-plan-worker", nil)
	var squadID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO squad (workspace_id, name, description, leader_id, creator_id)
		VALUES ($1, 'SILV-48 no-plan dispatch squad', '', $2, $3)
		RETURNING id
	`, testWorkspaceID, leaderID, testUserID).Scan(&squadID); err != nil {
		t.Fatalf("create no-plan squad: %v", err)
	}
	for _, member := range []struct{ id, role string }{{leaderID, "leader"}, {workerID, "implementer"}} {
		if _, err := testPool.Exec(ctx, `INSERT INTO squad_member (squad_id, member_type, member_id, role) VALUES ($1, 'agent', $2, $3)`, squadID, member.id, member.role); err != nil {
			t.Fatalf("add %s: %v", member.role, err)
		}
	}

	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "SILV-48 squad parent without workflow plan", "assignee_type": "squad", "assignee_id": squadID,
	})
	var parent IssueResponse
	testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated).JSON(&parent)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM squad_member WHERE squad_id = $1`, squadID)
		testPool.Exec(context.Background(), `DELETE FROM squad WHERE id = $1`, squadID)
	})
	cleanupPlannedIssue(t, parent.ID)
	var planRows int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_plan WHERE issue_id = $1`, parent.ID).Scan(&planRows); err != nil {
		t.Fatalf("count plans: %v", err)
	}
	if planRows != 0 {
		t.Fatalf("fixture unexpectedly installed %d plans", planRows)
	}
	var leaderTaskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, issue_id, squad_id, is_leader_task, originator_user_id, accountable_user_id)
		VALUES ($1, (SELECT runtime_id FROM agent WHERE id = $1), 'running', 0, $2, $3, true, $4, $4)
		RETURNING id
	`, leaderID, parent.ID, squadID, testUserID).Scan(&leaderTaskID); err != nil {
		t.Fatalf("seed leader task: %v", err)
	}

	req = newRequest("POST", "/api/issues/"+parent.ID+"/comments", map[string]any{
		"content": "dispatch [@Worker](mention://agent/" + workerID + ")",
	})
	req.Header.Set("X-Agent-ID", leaderID)
	req.Header.Set("X-Task-ID", leaderTaskID)
	req = withURLParam(req, "id", parent.ID)
	var response struct {
		TriggerOutcomes []CommentTriggerOutcome `json:"trigger_outcomes"`
	}
	testutil.Call(t, testHandler.CreateComment, req).Want(http.StatusCreated).JSON(&response)
	got := findTriggerOutcome(response.TriggerOutcomes, workerID)
	if got == nil || got.Status != DispatchBlocked || got.ReasonCode != ReasonWorkflowPlanBlocked {
		t.Fatalf("worker outcome = %+v, want blocked/%s", got, ReasonWorkflowPlanBlocked)
	}
}

// Member delegation on an issue without a workflow plan retains the legacy
// pass-through behavior because no task-scoped agent is claiming plan authority.
func TestWorkflowPlanDispatch_UnplannedIssueMemberMentionUnchanged(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	target := createHandlerTestAgent(t, "silv48-unplanned-mention-target", nil)
	var created IssueResponse
	testutil.Call(t, testHandler.CreateIssue, newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "SILV-48 unplanned member mention",
	})).Want(http.StatusCreated).JSON(&created)
	cleanupPlannedIssue(t, created.ID)

	req := newRequest("POST", "/api/issues/"+created.ID+"/comments", map[string]any{
		"content": "please look [@Target](mention://agent/" + target + ")",
	})
	req = withURLParam(req, "id", created.ID)
	var response struct {
		TriggerOutcomes []CommentTriggerOutcome `json:"trigger_outcomes"`
	}
	testutil.Call(t, testHandler.CreateComment, req).Want(http.StatusCreated).JSON(&response)
	var rows int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, created.ID, target).Scan(&rows); err != nil {
		t.Fatalf("count target tasks: %v", err)
	}
	if rows != 1 {
		t.Fatalf("unplanned member mention created %d target tasks; want 1", rows)
	}
}
