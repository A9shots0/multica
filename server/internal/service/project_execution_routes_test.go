package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestProjectExecutionRouteAdmissionAndLaunch(t *testing.T) {
	ctx := context.Background()
	fx, agentID, runtimeID := newArchiveCancelFixture(t)
	q := db.New(fx.Pool)
	svc := NewTaskService(q, fx.Pool, nil, events.New())
	project := fx.Project(t, "Host-bound project")
	issueID := fx.Issue(t, "Ordinary project work", testutil.Cols{"project_id": project})
	issue, err := q.GetIssue(ctx, util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	agent, err := q.GetAgent(ctx, util.MustParseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	allowedHost := uuid.NewString()
	wrongHost := uuid.NewString()
	fx.Exec(t, `UPDATE agent_runtime SET daemon_id=$2 WHERE id=$1`, runtimeID, wrongHost)
	setRoutes := func(routes map[string][]string) {
		t.Helper()
		raw, _ := json.Marshal(routes)
		if _, err := q.UpdateWorkspaceProjectExecutionRoutes(ctx, db.UpdateWorkspaceProjectExecutionRoutesParams{ID: util.MustParseUUID(fx.WorkspaceID), Routes: raw}); err != nil {
			t.Fatal(err)
		}
	}
	setRoutes(map[string][]string{project: {allowedHost}})
	enqueue := func() (db.AgentTaskQueue, error) {
		return svc.createAgentTaskWithWorkflowAuthority(ctx, issue, agent, "ordinary", db.CreateAgentTaskParams{ID: util.MustParseUUID(uuid.NewString()), AgentID: agent.ID, RuntimeID: util.MustParseUUID(runtimeID), IssueID: issue.ID, TaskContext: []byte(`{"host":"approved","project_execution_routes":{}}`)})
	}
	if _, err := enqueue(); !errors.Is(err, ErrProjectExecutionRouteDenied) {
		t.Fatalf("wrong host enqueue: %v", err)
	}
	if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID); n != 0 {
		t.Fatalf("denied enqueue wrote %d tasks", n)
	}
	fx.Exec(t, `UPDATE agent_runtime SET daemon_id=$2 WHERE id=$1`, runtimeID, allowedHost)
	task, err := enqueue()
	if err != nil {
		t.Fatalf("approved host: %v", err)
	}
	t.Cleanup(func() { fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, task.ID) })
	fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1`, task.ID)
	task, err = q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The policy changes after enqueue. Finalization must withhold credentials,
	// and launch must settle the stale assignment without starting the provider.
	setRoutes(map[string][]string{project: {wrongHost}})
	token := db.CreateTaskTokenParams{ID: util.MustParseUUID(uuid.NewString()), TokenHash: uuid.NewString(), TaskID: task.ID, AgentID: agent.ID, WorkspaceID: issue.WorkspaceID, UserID: util.MustParseUUID(fx.UserID), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}
	if _, err := svc.FinalizeTaskClaim(ctx, task, token, nil, false); !errors.Is(err, ErrProjectExecutionRouteDenied) {
		t.Fatalf("stale claim: %v", err)
	}
	if n := fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, task.ID); n != 0 {
		t.Fatalf("route denial issued %d credentials", n)
	}
	if _, err := svc.StartTask(ctx, task.ID); !errors.Is(err, ErrProjectExecutionRouteDenied) {
		t.Fatalf("stale launch: %v", err)
	}
	stopped, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.StartedAt.Valid || stopped.Status != "failed" {
		t.Fatalf("denied provider launch left status %s started=%v", stopped.Status, stopped.StartedAt.Valid)
	}
	setRoutes(map[string][]string{project: {allowedHost}})
	approvedIssue := fx.Issue(t, "Approved launch", testutil.Cols{"project_id": project})
	approvedTask := fx.Task(t, agentID, testutil.Cols{"issue_id": approvedIssue, "runtime_id": runtimeID, "status": "dispatched", "dispatched_at": testutil.Raw("now()")})
	started, err := svc.StartTask(ctx, util.MustParseUUID(approvedTask))
	if err != nil || started == nil || started.Status != "running" {
		t.Fatalf("approved provider launch: %+v %v", started, err)
	}
	// Unconfigured projects and non-issue Chat retain ordinary behavior.
	setRoutes(map[string][]string{})
	if err := validateProjectExecutionRoute(ctx, q, issue, task.RuntimeID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ValidateTaskProjectExecutionAuthority(ctx, q, db.AgentTaskQueue{}); err != nil {
		t.Fatal(err)
	}
	setRoutes(map[string][]string{project: {}})
	if err := validateProjectExecutionRoute(ctx, q, issue, task.RuntimeID); !errors.Is(err, ErrProjectExecutionRouteDenied) {
		t.Fatalf("empty declared route: %v", err)
	}
}

func TestProjectExecutionCapabilityAdmissionAndRevocation(t *testing.T) {
	ctx := context.Background()
	fx, agentID, runtimeID := newArchiveCancelFixture(t)
	q := db.New(fx.Pool)
	svc := NewTaskService(q, fx.Pool, nil, events.New())
	project := fx.Project(t, "Capability-bound project")
	issueID := fx.Issue(t, "Capability-bound work", testutil.Cols{"project_id": project})
	issue, err := q.GetIssue(ctx, util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	agent, err := q.GetAgent(ctx, util.MustParseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	skillID := fx.Insert(t, "skill", testutil.Cols{
		"workspace_id": fx.WorkspaceID, "name": "Required capability " + uuid.NewString(),
		"description": "", "content": "", "created_by": fx.UserID,
	})
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": skillID}, "agent_id=$1 AND skill_id=$2", agentID, skillID)
	allowedHost := uuid.NewString()
	fx.Exec(t, `UPDATE agent_runtime SET daemon_id=$2 WHERE id=$1`, runtimeID, allowedHost)
	routes, _ := json.Marshal(map[string][]string{project: {allowedHost}})
	if _, err := q.UpdateWorkspaceProjectExecutionRoutes(ctx, db.UpdateWorkspaceProjectExecutionRoutesParams{ID: issue.WorkspaceID, Routes: routes}); err != nil {
		t.Fatal(err)
	}
	alternateAgentID := fx.Agent(t, "Equally skilled alternate", runtimeID)
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": alternateAgentID, "skill_id": skillID}, "agent_id=$1 AND skill_id=$2", alternateAgentID, skillID)
	setCapabilities := func(rules map[string]ProjectExecutionCapabilityRule) {
		t.Helper()
		raw, _ := json.Marshal(rules)
		if _, err := q.UpdateWorkspaceProjectExecutionCapabilities(ctx, db.UpdateWorkspaceProjectExecutionCapabilitiesParams{ID: util.MustParseUUID(fx.WorkspaceID), Capabilities: raw}); err != nil {
			t.Fatal(err)
		}
	}
	enqueue := func() (db.AgentTaskQueue, error) {
		return svc.createAgentTaskWithWorkflowAuthority(ctx, issue, agent, "ordinary", db.CreateAgentTaskParams{
			ID: util.MustParseUUID(uuid.NewString()), AgentID: agent.ID, RuntimeID: util.MustParseUUID(runtimeID), IssueID: issue.ID, TaskContext: []byte(`{}`),
		})
	}

	setCapabilities(map[string]ProjectExecutionCapabilityRule{project: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{uuid.NewString()}}})
	if _, err := enqueue(); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
		t.Fatalf("skilled but unlisted agent: %v", err)
	}
	if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID); n != 0 {
		t.Fatalf("denied capability enqueue wrote %d tasks", n)
	}

	setCapabilities(map[string]ProjectExecutionCapabilityRule{project: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{agentID}}})
	task, err := enqueue()
	if err != nil {
		t.Fatalf("listed skilled agent: %v", err)
	}
	t.Cleanup(func() { fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, task.ID) })
	if util.UUIDToString(task.AgentID) != agentID {
		t.Fatalf("capability gate substituted worker: got %s want %s", util.UUIDToString(task.AgentID), agentID)
	}
	fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1`, task.ID)
	task, err = q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, `UPDATE agent_skill SET enabled=false WHERE agent_id=$1 AND skill_id=$2`, agentID, skillID)
	token := db.CreateTaskTokenParams{ID: util.MustParseUUID(uuid.NewString()), TokenHash: uuid.NewString(), TaskID: task.ID, AgentID: agent.ID, WorkspaceID: issue.WorkspaceID, UserID: util.MustParseUUID(fx.UserID), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}
	if _, err := svc.FinalizeTaskClaim(ctx, task, token, nil, false); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
		t.Fatalf("revoked skill claim: %v", err)
	}
	if n := fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, task.ID); n != 0 {
		t.Fatalf("capability denial issued %d credentials", n)
	}
	if _, err := svc.StartTask(ctx, task.ID); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
		t.Fatalf("revoked skill launch: %v", err)
	}
	stopped, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.StartedAt.Valid || stopped.Status != "failed" || util.UUIDToString(stopped.AgentID) != agentID {
		t.Fatalf("denied launch changed worker or started: %+v", stopped)
	}
	if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2`, issueID, alternateAgentID); n != 0 {
		t.Fatalf("capability revocation auto-rerouted to alternate worker: %d tasks", n)
	}
	fx.Exec(t, `UPDATE agent_skill SET enabled=true WHERE agent_id=$1 AND skill_id=$2`, agentID, skillID)
	freshTask, err := enqueue()
	if err != nil {
		t.Fatalf("fresh explicit task after re-enable: %v", err)
	}
	t.Cleanup(func() { fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, freshTask.ID) })
	if util.UUIDToString(freshTask.AgentID) != agentID {
		t.Fatalf("fresh explicit task changed worker: got %s want %s", util.UUIDToString(freshTask.AgentID), agentID)
	}
}

func TestProjectExecutionAuthorityRoutePrecedesCapability(t *testing.T) {
	ctx := context.Background()
	fx, agentID, runtimeID := newArchiveCancelFixture(t)
	q := db.New(fx.Pool)
	project := fx.Project(t, "Combined authority")
	issueID := fx.Issue(t, "Both policies fail", testutil.Cols{"project_id": project})
	issue, err := q.GetIssue(ctx, util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	routes, _ := json.Marshal(map[string][]string{project: {uuid.NewString()}})
	if _, err := q.UpdateWorkspaceProjectExecutionRoutes(ctx, db.UpdateWorkspaceProjectExecutionRoutesParams{ID: issue.WorkspaceID, Routes: routes}); err != nil {
		t.Fatal(err)
	}
	capabilities, _ := json.Marshal(map[string]ProjectExecutionCapabilityRule{project: {RequiredSkillIDs: nil, EligibleAgentIDs: []string{agentID}}})
	if _, err := q.UpdateWorkspaceProjectExecutionCapabilities(ctx, db.UpdateWorkspaceProjectExecutionCapabilitiesParams{ID: issue.WorkspaceID, Capabilities: capabilities}); err != nil {
		t.Fatal(err)
	}
	if err := validateProjectExecutionAuthority(ctx, q, issue, util.MustParseUUID(agentID), util.MustParseUUID(runtimeID)); !errors.Is(err, ErrProjectExecutionRouteDenied) {
		t.Fatalf("combined denial = %v, want route denial", err)
	}
}

func TestProjectExecutionCapabilityDirectAndSquadLeaderSemantics(t *testing.T) {
	ctx := context.Background()
	fx, agentID, runtimeID := newArchiveCancelFixture(t)
	q := db.New(fx.Pool)
	project := fx.Project(t, "Direct and squad capability")
	skillID := fx.Insert(t, "skill", testutil.Cols{
		"workspace_id": fx.WorkspaceID, "name": "Shared qualification " + uuid.NewString(),
		"description": "", "content": "", "created_by": fx.UserID,
	})
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": skillID}, "agent_id=$1 AND skill_id=$2", agentID, skillID)
	squadID := fx.Squad(t, "Qualified squad", agentID)
	directIssueID := fx.Issue(t, "Direct qualified", testutil.Cols{"project_id": project, "assignee_type": "agent", "assignee_id": agentID})
	squadIssueID := fx.Issue(t, "Squad qualified", testutil.Cols{"project_id": project, "assignee_type": "squad", "assignee_id": squadID})
	directIssue, err := q.GetIssue(ctx, util.MustParseUUID(directIssueID))
	if err != nil {
		t.Fatal(err)
	}
	squadIssue, err := q.GetIssue(ctx, util.MustParseUUID(squadIssueID))
	if err != nil {
		t.Fatal(err)
	}
	setCapabilities := func(rules map[string]ProjectExecutionCapabilityRule) {
		t.Helper()
		raw, _ := json.Marshal(rules)
		if _, err := q.UpdateWorkspaceProjectExecutionCapabilities(ctx, db.UpdateWorkspaceProjectExecutionCapabilitiesParams{ID: util.MustParseUUID(fx.WorkspaceID), Capabilities: raw}); err != nil {
			t.Fatal(err)
		}
	}
	setCapabilities(map[string]ProjectExecutionCapabilityRule{project: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{agentID}}})
	for label, issue := range map[string]db.Issue{"direct": directIssue, "squad": squadIssue} {
		if err := validateIssueAssigneeProjectExecutionAuthority(ctx, q, issue); err != nil {
			t.Fatalf("%s qualified: %v", label, err)
		}
	}
	fx.Exec(t, `UPDATE agent_skill SET enabled=false WHERE agent_id=$1 AND skill_id=$2`, agentID, skillID)
	for label, issue := range map[string]db.Issue{"direct": directIssue, "squad": squadIssue} {
		if err := validateIssueAssigneeProjectExecutionAuthority(ctx, q, issue); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
			t.Fatalf("%s revoked skill: %v", label, err)
		}
	}
	fx.Exec(t, `UPDATE agent_skill SET enabled=true WHERE agent_id=$1 AND skill_id=$2`, agentID, skillID)
	fx.Exec(t, `UPDATE agent SET archived_at=now() WHERE id=$1`, agentID)
	if err := validateProjectExecutionAuthority(ctx, q, directIssue, util.MustParseUUID(agentID), util.MustParseUUID(runtimeID)); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
		t.Fatalf("archived listed agent: %v", err)
	}
	fx.Exec(t, `UPDATE agent SET archived_at=NULL WHERE id=$1`, agentID)
	setCapabilities(map[string]ProjectExecutionCapabilityRule{project: {RequiredSkillIDs: nil, EligibleAgentIDs: []string{agentID}}})
	if err := validateProjectExecutionAuthority(ctx, q, directIssue, util.MustParseUUID(agentID), util.MustParseUUID(runtimeID)); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
		t.Fatalf("declared rule without skills: %v", err)
	}
	setCapabilities(map[string]ProjectExecutionCapabilityRule{})
	if err := validateProjectExecutionAuthority(ctx, q, directIssue, util.MustParseUUID(agentID), util.MustParseUUID(runtimeID)); err != nil {
		t.Fatalf("undeclared rule compatibility: %v", err)
	}
}

type projectCapabilityRaceFixture struct {
	fx      *testutil.Fixture
	pool    *pgxpool.Pool
	service *TaskService
	task    db.AgentTaskQueue
	agentID string
	skillID string
	userID  string
	wsID    string
}

func newProjectCapabilityRaceFixture(t *testing.T) projectCapabilityRaceFixture {
	t.Helper()
	ctx := context.Background()
	fx, agentID, runtimeID := newArchiveCancelFixture(t)
	q := db.New(fx.Pool)
	projectID := fx.Project(t, "Capability authority race")
	issueID := fx.Issue(t, "Capability authority race", testutil.Cols{"project_id": projectID})
	skillID := fx.Insert(t, "skill", testutil.Cols{
		"workspace_id": fx.WorkspaceID, "name": "Race-required skill " + uuid.NewString(),
		"description": "", "content": "", "created_by": fx.UserID,
	})
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": skillID}, "agent_id=$1 AND skill_id=$2", agentID, skillID)
	rules, _ := json.Marshal(map[string]ProjectExecutionCapabilityRule{
		projectID: {RequiredSkillIDs: []string{skillID}, EligibleAgentIDs: []string{agentID}},
	})
	if _, err := q.UpdateWorkspaceProjectExecutionCapabilities(ctx, db.UpdateWorkspaceProjectExecutionCapabilitiesParams{ID: util.MustParseUUID(fx.WorkspaceID), Capabilities: rules}); err != nil {
		t.Fatal(err)
	}
	taskID := fx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "dispatched", "dispatched_at": testutil.Raw("now()"),
	})
	task, err := q.GetAgentTask(ctx, util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	return projectCapabilityRaceFixture{
		fx: fx, pool: fx.Pool, service: NewTaskService(q, fx.Pool, nil, events.New()), task: task,
		agentID: agentID, skillID: skillID, userID: fx.UserID, wsID: fx.WorkspaceID,
	}
}

func waitForAuthorityWriterBlocked(t *testing.T, pool *pgxpool.Pool, applicationName string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		if err := pool.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE application_name=$1 AND wait_event_type='Lock'
			)`, applicationName).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("authority writer %q did not block on the admission lock", applicationName)
}

func TestFinalizeTaskClaimSerializesConcurrentCapabilityRevocation(t *testing.T) {
	ctx := context.Background()
	fx := newProjectCapabilityRaceFixture(t)
	revoker, err := fx.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer revoker.Release()
	applicationName := "capability-finalize-race-" + uuid.NewString()
	if _, err := revoker.Exec(ctx, `SELECT set_config('application_name',$1,false)`, applicationName); err != nil {
		t.Fatal(err)
	}
	revoked := make(chan error, 1)
	abort := errors.New("abort after observing serialized revocation")
	fx.service.BeforeFinalizeClaim = func(context.Context, *db.Queries, db.AgentTaskQueue) error {
		go func() {
			_, updateErr := revoker.Exec(context.Background(), `UPDATE agent_skill SET enabled=false WHERE agent_id=$1 AND skill_id=$2`, fx.agentID, fx.skillID)
			revoked <- updateErr
		}()
		waitForAuthorityWriterBlocked(t, fx.pool, applicationName)
		return abort
	}
	token := db.CreateTaskTokenParams{
		ID: util.MustParseUUID(uuid.NewString()), TokenHash: uuid.NewString(), TaskID: fx.task.ID, AgentID: fx.task.AgentID,
		WorkspaceID: util.MustParseUUID(fx.wsID), UserID: util.MustParseUUID(fx.userID),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}
	if _, err := fx.service.FinalizeTaskClaim(ctx, fx.task, token, nil, false); !errors.Is(err, abort) {
		t.Fatalf("first finalize = %v, want controlled rollback", err)
	}
	if err := <-revoked; err != nil {
		t.Fatalf("revoke skill after finalize rollback: %v", err)
	}
	fx.service.BeforeFinalizeClaim = nil
	if _, err := fx.service.FinalizeTaskClaim(ctx, fx.task, token, nil, false); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
		t.Fatalf("finalize after serialized revoke = %v", err)
	}
	if n := fx.fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, fx.task.ID); n != 0 {
		t.Fatalf("serialized revocation leaked %d credentials", n)
	}
}

func TestStartTaskSerializesConcurrentCapabilityRevocation(t *testing.T) {
	ctx := context.Background()
	fx := newProjectCapabilityRaceFixture(t)
	revoker, err := fx.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer revoker.Release()
	applicationName := "capability-start-race-" + uuid.NewString()
	if _, err := revoker.Exec(ctx, `SELECT set_config('application_name',$1,false)`, applicationName); err != nil {
		t.Fatal(err)
	}
	revoked := make(chan error, 1)
	abort := errors.New("abort launch after observing serialized revocation")
	fx.service.BeforeStartTask = func(context.Context, *db.Queries, db.AgentTaskQueue, string) error {
		go func() {
			_, updateErr := revoker.Exec(context.Background(), `UPDATE agent_skill SET enabled=false WHERE agent_id=$1 AND skill_id=$2`, fx.agentID, fx.skillID)
			revoked <- updateErr
		}()
		waitForAuthorityWriterBlocked(t, fx.pool, applicationName)
		return abort
	}
	if _, err := fx.service.StartTask(ctx, fx.task.ID); !errors.Is(err, abort) {
		t.Fatalf("first start = %v, want controlled rollback", err)
	}
	if err := <-revoked; err != nil {
		t.Fatalf("revoke skill after start rollback: %v", err)
	}
	fx.service.BeforeStartTask = nil
	if _, err := fx.service.StartTask(ctx, fx.task.ID); !errors.Is(err, ErrProjectExecutionCapabilityDenied) {
		t.Fatalf("start after serialized revoke = %v", err)
	}
	stored, err := db.New(fx.pool).GetAgentTask(ctx, fx.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.StartedAt.Valid || stored.Status != "failed" {
		t.Fatalf("serialized revocation allowed provider start: %+v", stored)
	}
}
