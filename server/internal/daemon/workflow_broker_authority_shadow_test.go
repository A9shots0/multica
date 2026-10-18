package daemon

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	workflowBrokerFixtureModel   = "claude-sonnet-4-6"
	workflowBrokerFixtureEffort  = "medium"
	workflowBrokerLeaderName     = "Item 14 Replay Intake"
	workflowBrokerExecutorName   = "Item 14 Replay Executor"
	workflowBrokerReviewerName   = "Item 14 Replay Reviewer"
	workflowBrokerLeaderResult   = "ITEM14_ROUTE_APPROVED"
	workflowBrokerExecutorResult = "ITEM14_EXECUTION_APPROVED"
	workflowBrokerReviewerResult = "ITEM14_REVIEW_PASS"
)

type workflowBrokerShadowFixture struct {
	WorkspaceID string
	UserID      string
	RuntimeID   string
	IssueID     string
	SquadID     string
	SkillID     string
	LeaderID    string
	ExecutorID  string
	ReviewerID  string
}

// TestWorkflowBrokerAuthorityFakeProviderFullPath is the provider-free dress
// rehearsal for the local replay. It crosses the production enqueue hook,
// durable qualified-runtime-binding.v2 template, per-task grant, HTTP claim,
// task-token authentication, daemon staging/start consumption, exact broker,
// exact fake provider child, workflow event controller, and terminal callbacks.
// The fake provider is invoked exactly once for each of the three bounded role
// tasks and has no provider or credential implementation.
func TestWorkflowBrokerAuthorityFakeProviderFullPath(t *testing.T) {
	pool := guardedShadowDatabase(t)
	ctx := context.Background()
	reportDir := filepath.Join(t.TempDir(), "replay-report")
	launchMarker := filepath.Join(t.TempDir(), "fake-provider-launches")
	brokerPath := buildWorkflowBrokerCommand(t, "./cmd/item14-integration-pilot-broker", "-X=main.authorizedReportDirectory="+reportDir)
	providerPath := buildWorkflowBrokerCommand(t, "./cmd/item14-integration-pilot-broker/fake-provider", "-X=main.launchMarker="+launchMarker)
	facts, err := guardedWorkflowBrokerHostExecutionFacts(brokerPath, providerPath)
	if err != nil {
		t.Fatal(err)
	}

	fx := seedWorkflowBrokerShadowFixture(t, pool)
	hub := realtime.NewHub()
	go hub.Run()
	h := handler.New(db.New(pool), pool, hub, events.New(), service.NewEmailService(), nil, nil, analytics.NoopClient{}, handler.Config{
		WorkflowControllerID: util.MustParseUUID(guardedShadowControllerID),
	})
	for _, role := range []struct {
		agentID string
		name    string
		kinds   []string
	}{
		{fx.LeaderID, workflowBrokerLeaderName, []string{"leader"}},
		{fx.ExecutorID, workflowBrokerExecutorName, []string{"implementation", "correction"}},
		{fx.ReviewerID, workflowBrokerReviewerName, []string{"review"}},
	} {
		storeWorkflowBrokerRuntimeReceipt(t, h.PermissionReceiptStore, facts, fx, role.agentID, role.name, role.kinds, brokerPath, providerPath)
	}

	server := newWorkflowBrokerShadowServer(t, h, fx.WorkspaceID)
	defer server.Close()
	d := New(Config{
		ServerBaseURL: server.URL, DaemonID: guardedShadowDaemonID,
		HostAttestation: guardedShadowHost,
		Agents:          map[string]AgentEntry{"claude": {Path: brokerPath}},
		WorkspacesRoot:  t.TempDir(), AgentTimeout: 2 * time.Minute,
	}, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	d.executionEnvironmentCommand = nil
	d.client.SetToken("provider-free-daemon-token")

	issue, err := h.Queries.GetIssue(ctx, util.MustParseUUID(fx.IssueID))
	if err != nil {
		t.Fatal(err)
	}
	leaderTask, err := h.TaskService.EnqueueTaskForSquadLeaderByActor(
		ctx, issue, util.MustParseUUID(fx.LeaderID), util.MustParseUUID(fx.SquadID), util.MustParseUUID(fx.UserID),
	)
	if err != nil {
		t.Fatalf("enqueue guarded leader: %v", err)
	}

	completed := []string{
		runWorkflowBrokerShadowTask(t, ctx, d, fx, util.UUIDToString(leaderTask.ID), workflowBrokerLeaderName, "leader", workflowBrokerLeaderResult),
		runWorkflowBrokerShadowTask(t, ctx, d, fx, "", workflowBrokerExecutorName, "implementation", workflowBrokerExecutorResult),
		runWorkflowBrokerShadowTask(t, ctx, d, fx, "", workflowBrokerReviewerName, "review", workflowBrokerReviewerResult),
	}

	assertWorkflowBrokerFakeLaunches(t, launchMarker, []string{
		workflowBrokerLeaderResult, workflowBrokerExecutorResult, workflowBrokerReviewerResult,
	})
	for _, taskID := range completed {
		assertGuardedShadowConsumption(t, pool, taskID, "item14-workflow-broker-task-v1."+taskID, 1)
	}
	var total, terminal, grants, consumptions int
	if err := pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE status = 'completed')
		FROM agent_task_queue WHERE issue_id = $1`, fx.IssueID).Scan(&total, &terminal); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM guarded_task_capability_grant WHERE issue_id = $1`, fx.IssueID).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM guarded_execution_launch_consumption WHERE issue_id = $1`, fx.IssueID).Scan(&consumptions); err != nil {
		t.Fatal(err)
	}
	if total != 3 || terminal != 3 || grants != 3 || consumptions != 3 {
		t.Fatalf("workflow rows total/completed/grants/consumptions = %d/%d/%d/%d, want 3/3/3/3", total, terminal, grants, consumptions)
	}
	var currentState string
	if err := pool.QueryRow(ctx, `SELECT current_state FROM issue_workflow_plan WHERE issue_id = $1`, fx.IssueID).Scan(&currentState); err != nil {
		t.Fatal(err)
	}
	if currentState != "done" {
		t.Fatalf("workflow state = %q, want done", currentState)
	}
	for _, name := range []string{"inventory.json", "WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md", "EVIDENCE.md"} {
		info, err := os.Stat(filepath.Join(reportDir, name))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			t.Fatalf("frozen fake replay artifact %s is unavailable: %v", name, err)
		}
	}
}

func runWorkflowBrokerShadowTask(t *testing.T, ctx context.Context, d *Daemon, fx workflowBrokerShadowFixture, expectedTaskID, expectedAgentName, expectedKind, expectedResult string) string {
	t.Helper()
	claimed, err := d.client.ClaimTask(ctx, fx.RuntimeID)
	if err != nil || claimed == nil {
		t.Fatalf("claim %s task = %+v, error = %v", expectedKind, claimed, err)
	}
	if expectedTaskID != "" && claimed.ID != expectedTaskID {
		t.Fatalf("claimed %s task %s, want %s", expectedKind, claimed.ID, expectedTaskID)
	}
	if claimed.Agent == nil || claimed.Agent.Name != expectedAgentName || claimed.Agent.Model != workflowBrokerFixtureModel || claimed.Agent.ThinkingLevel != workflowBrokerFixtureEffort {
		t.Fatalf("%s claim selected a different agent package: %+v", expectedKind, claimed.Agent)
	}
	if len(claimed.WorkflowPacket) != 0 || claimed.GuardedExecution == nil || claimed.GuardedExecution.CapabilityScope.WorkflowBroker == nil {
		t.Fatalf("%s claim crossed the boundary without one V3 broker package: %+v", expectedKind, claimed)
	}
	bound := claimed.GuardedExecution.CapabilityScope.WorkflowBroker
	if bound.AgentName != expectedAgentName || bound.TaskKind != expectedKind || bound.ProviderCallLimit != 1 || bound.RetryLimit != 0 || bound.MaxTurns != 1 {
		t.Fatalf("%s broker authority drifted: %+v", expectedKind, bound)
	}
	result, err := d.runTask(ctx, *claimed, "claude", 0, slog.Default())
	if err != nil {
		t.Fatalf("run %s through guarded broker: %v", expectedKind, err)
	}
	if result.Status != "completed" || strings.TrimSpace(result.Comment) != expectedResult || result.ProviderTurns != 1 ||
		!sameStrings(result.ProviderModels, []string{"claude-haiku-4-5-20251001", workflowBrokerFixtureModel}) {
		t.Fatalf("%s result = %+v", expectedKind, result)
	}
	if err := d.client.CompleteTask(ctx, claimed.ID, result.Comment, result.BranchName, result.SessionID, result.WorkDir, result.SessionRolloutMissing, result.RetiredSessionID, result.DurableWorkDir); err != nil {
		t.Fatalf("complete %s task: %v", expectedKind, err)
	}
	return claimed.ID
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func newWorkflowBrokerShadowServer(t *testing.T, h *handler.Handler, workspaceID string) *httptest.Server {
	t.Helper()
	brokerAPI := middleware.Auth(h.Queries, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeCtx := chi.NewRouteContext()
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "issues" {
			http.Error(w, "unexpected guarded broker API route", http.StatusNotFound)
			return
		}
		routeCtx.URLParams.Add("id", parts[2])
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
		switch {
		case r.Method == http.MethodGet && parts[3] == "workflow-plan":
			h.GetIssueWorkflowPlan(w, r)
		case r.Method == http.MethodPost && parts[3] == "comments":
			h.CreateComment(w, r)
		case r.Method == http.MethodPost && parts[3] == "workflow-events":
			h.PostIssueWorkflowEvent(w, r)
		default:
			http.Error(w, "unexpected guarded broker API method", http.StatusNotFound)
		}
	}))
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeCtx := chi.NewRouteContext()
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		requestCtx := middleware.WithDaemonContext(r.Context(), workspaceID, guardedShadowDaemonID)
		switch {
		case len(parts) == 6 && parts[0] == "api" && parts[1] == "daemon" && parts[2] == "runtimes" && parts[4] == "tasks" && parts[5] == "claim":
			routeCtx.URLParams.Add("runtimeId", parts[3])
			h.ClaimTaskByRuntime(w, r.WithContext(context.WithValue(requestCtx, chi.RouteCtxKey, routeCtx)))
		case len(parts) == 7 && parts[0] == "api" && parts[1] == "daemon" && parts[2] == "runtimes" && parts[4] == "tasks" && parts[6] == "prepare-lease":
			routeCtx.URLParams.Add("runtimeId", parts[3])
			routeCtx.URLParams.Add("taskId", parts[5])
			h.ExtendTaskPrepareLease(w, r.WithContext(context.WithValue(requestCtx, chi.RouteCtxKey, routeCtx)))
		case len(parts) == 5 && parts[0] == "api" && parts[1] == "daemon" && parts[2] == "tasks" && parts[4] == "start":
			routeCtx.URLParams.Add("taskId", parts[3])
			h.StartTask(w, r.WithContext(context.WithValue(requestCtx, chi.RouteCtxKey, routeCtx)))
		case len(parts) == 5 && parts[0] == "api" && parts[1] == "daemon" && parts[2] == "tasks" && parts[4] == "progress":
			routeCtx.URLParams.Add("taskId", parts[3])
			h.ReportTaskProgress(w, r.WithContext(context.WithValue(requestCtx, chi.RouteCtxKey, routeCtx)))
		case len(parts) == 5 && parts[0] == "api" && parts[1] == "daemon" && parts[2] == "tasks" && parts[4] == "messages":
			routeCtx.URLParams.Add("taskId", parts[3])
			h.ReportTaskMessages(w, r.WithContext(context.WithValue(requestCtx, chi.RouteCtxKey, routeCtx)))
		case len(parts) == 5 && parts[0] == "api" && parts[1] == "daemon" && parts[2] == "tasks" && parts[4] == "complete":
			routeCtx.URLParams.Add("taskId", parts[3])
			h.CompleteTask(w, r.WithContext(context.WithValue(requestCtx, chi.RouteCtxKey, routeCtx)))
		default:
			brokerAPI.ServeHTTP(w, r)
		}
	}))
}

func buildWorkflowBrokerCommand(t *testing.T, packagePath, linkerBinding string) string {
	t.Helper()
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "item14-command")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	cacheDir := filepath.Join(dir, "go-cache")
	tempDir := filepath.Join(dir, "go-tmp")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	args := []string{"build", "-trimpath", "-o", binaryPath}
	if linkerBinding != "" {
		args = append(args, "-ldflags", linkerBinding)
	}
	args = append(args, packagePath)
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = []string{
		"CGO_ENABLED=0", "GO111MODULE=on", "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off",
		"GOCACHE=" + cacheDir, "GOTMPDIR=" + tempDir, "PATH=" + os.Getenv("PATH"), "TEMP=" + tempDir, "TMP=" + tempDir,
	}
	if runtime.GOOS == "windows" {
		cmd.Env = append(cmd.Env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile %s: %v\n%s", packagePath, err, output)
	}
	return binaryPath
}

func seedWorkflowBrokerShadowFixture(t *testing.T, pool *pgxpool.Pool) workflowBrokerShadowFixture {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	fx := workflowBrokerShadowFixture{}
	if err := pool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Item 14 fake workflow owner', $1) RETURNING id`, fmt.Sprintf("item14-workflow-%d@example.invalid", suffix)).Scan(&fx.UserID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('Item 14 fake workflow', $1, 'provider-free full path only', 'I14') RETURNING id`, fmt.Sprintf("item14-workflow-%d", suffix)).Scan(&fx.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, fx.WorkspaceID, fx.UserID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at, visibility, owner_id)
		VALUES ($1, $2, 'Item 14 fake workflow runtime', 'cloud', 'claude', 'online', 'provider-free fixture', '{}'::jsonb, now(), 'private', $3)
		RETURNING id`, fx.WorkspaceID, guardedShadowDaemonID, fx.UserID).Scan(&fx.RuntimeID); err != nil {
		t.Fatal(err)
	}
	insertAgent := func(name string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO agent (workspace_id, name, description, runtime_mode, runtime_config, runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id, instructions, model, thinking_level)
			VALUES ($1, $2, 'provider-free workflow fixture only', 'cloud', '{}'::jsonb, $3, 'private', 'private', 8, $4, 'untrusted mutable instruction', $5, $6)
			RETURNING id`, fx.WorkspaceID, name, fx.RuntimeID, fx.UserID, workflowBrokerFixtureModel, workflowBrokerFixtureEffort).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	fx.LeaderID = insertAgent(workflowBrokerLeaderName)
	fx.ExecutorID = insertAgent(workflowBrokerExecutorName)
	fx.ReviewerID = insertAgent(workflowBrokerReviewerName)
	if err := pool.QueryRow(ctx, `INSERT INTO skill (workspace_id, name, description, content, config, created_by) VALUES ($1, $2, 'provider-free fixture', 'bounded local workflow rules', '{}'::jsonb, $3) RETURNING id`, fx.WorkspaceID, fmt.Sprintf("item14-workflow-skill-%d", suffix), fx.UserID).Scan(&fx.SkillID); err != nil {
		t.Fatal(err)
	}
	for _, agentID := range []string{fx.LeaderID, fx.ExecutorID, fx.ReviewerID} {
		if _, err := pool.Exec(ctx, `INSERT INTO agent_skill (agent_id, skill_id) VALUES ($1, $2)`, agentID, fx.SkillID); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, `INSERT INTO squad (workspace_id, name, description, leader_id, creator_id) VALUES ($1, $2, 'provider-free workflow fixture', $3, $4) RETURNING id`, fx.WorkspaceID, fmt.Sprintf("Item 14 Fake Replay %d", suffix), fx.LeaderID, fx.UserID).Scan(&fx.SquadID); err != nil {
		t.Fatal(err)
	}
	for _, member := range []struct{ id, role string }{{fx.LeaderID, "leader"}, {fx.ExecutorID, "implementer"}, {fx.ReviewerID, "reviewer"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO squad_member (squad_id, member_type, member_id, role) VALUES ($1, 'agent', $2, $3)`, fx.SquadID, member.id, member.role); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_id, creator_type, number, position, assignee_type, assignee_id)
		VALUES ($1, 'Item 14 provider-free guarded workflow', 'in_progress', 'none', $2, 'member', 1, 0, 'squad', $3)
		RETURNING id`, fx.WorkspaceID, fx.UserID, fx.SquadID).Scan(&fx.IssueID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO issue_workflow_plan (issue_id, workspace_id, squad_id, implementer_id, reviewer_id, requires_owner, requires_deployment, current_state)
		VALUES ($1, $2, $3, $4, $5, false, false, 'working')`, fx.IssueID, fx.WorkspaceID, fx.SquadID, fx.ExecutorID, fx.ReviewerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM task_token WHERE workspace_id = $1`, fx.WorkspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent_task_message WHERE task_id IN (SELECT id FROM agent_task_queue WHERE issue_id = $1)`, fx.IssueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM issue_workflow_event WHERE issue_id = $1`, fx.IssueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM issue_workflow_plan WHERE issue_id = $1`, fx.IssueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM inbox_item WHERE issue_id = $1`, fx.IssueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM comment WHERE issue_id = $1`, fx.IssueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent_task_queue WHERE issue_id = $1`, fx.IssueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM issue WHERE id = $1`, fx.IssueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM squad_member WHERE squad_id = $1`, fx.SquadID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM squad WHERE id = $1`, fx.SquadID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent_skill WHERE skill_id = $1`, fx.SkillID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM skill_file WHERE skill_id = $1`, fx.SkillID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM skill WHERE id = $1`, fx.SkillID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent WHERE id = ANY($1::uuid[])`, []string{fx.LeaderID, fx.ExecutorID, fx.ReviewerID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent_runtime WHERE id = $1`, fx.RuntimeID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, fx.WorkspaceID, fx.UserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspace WHERE id = $1`, fx.WorkspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user" WHERE id = $1`, fx.UserID)
	})
	return fx
}

func storeWorkflowBrokerRuntimeReceipt(t *testing.T, store *agentqualification.PermissionReceiptStore, facts agentqualification.GuardedHostExecutionFactsV1, fx workflowBrokerShadowFixture, agentID, agentName string, allowedKinds []string, brokerPath, providerPath string) {
	t.Helper()
	artifact, err := agentqualification.ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		t.Fatal(err)
	}
	artifactRaw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	artifactSum := sha256.Sum256(artifactRaw)
	now := time.Now().UTC()
	receipt := agentqualification.QualifiedRuntimeBindingReceiptV2{
		Version:     agentqualification.QualifiedRuntimeBindingVersionV2,
		ID:          agentqualification.WorkflowBrokerRuntimeReceiptIDV1(fx.IssueID, agentID),
		WorkspaceID: fx.WorkspaceID, IssueID: fx.IssueID, ExecutorAgentID: agentID, RuntimeID: fx.RuntimeID,
		ConfigurationID: "item14.workflow-broker.fake-replay.v1." + agentID, RoleID: artifact.RoleID,
		RuntimeProvider: "claude", Model: workflowBrokerFixtureModel, ReasoningEffort: workflowBrokerFixtureEffort,
		ChildExecution: agentqualification.GuardedChildExecutionBoundaryV1{
			Version: agentqualification.GuardedChildExecutionBoundaryVersionV1, HostOS: facts.HostOS, Adapter: facts.Adapter,
			EnvironmentPolicy: facts.EnvironmentPolicy, EnvironmentAllowlistSHA256: facts.EnvironmentAllowlistSHA256,
			ToolPolicy: facts.ToolPolicy, HostAttestationCustody: facts.HostAttestationCustody,
			ExecutableSHA256: facts.ExecutableSHA256, ProviderExecutableSHA256: facts.ProviderExecutableSHA256,
		},
		WorkflowBroker: &agentqualification.GuardedWorkflowBrokerTemplateV1{
			Version: agentqualification.GuardedWorkflowBrokerTemplateVersionV1, AgentName: agentName,
			BrokerExecutablePath: brokerPath, ProviderExecutablePath: providerPath,
			ProviderExecutableSHA256: facts.ProviderExecutableSHA256,
			ProviderProfileKind:      agentqualification.GuardedWorkflowBrokerProfileFixtureV1,
			HostAttestation:          guardedShadowHost, AllowedTaskKinds: allowedKinds,
		},
		ArtifactBindingSHA256: hex.EncodeToString(artifactSum[:]), QualificationStatus: agentqualification.StatusQualifiedBounded,
		EvidenceReceiptIDs: []string{"item14-workflow-broker-fake-full-path"}, Source: "controller_record", Immutable: true,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(10 * time.Minute),
	}
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), guardedShadowControllerID, receipt, now); err != nil {
		t.Fatalf("store %s workflow broker receipt: %v", agentName, err)
	}
}

func assertWorkflowBrokerFakeLaunches(t *testing.T, marker string, want []string) {
	t.Helper()
	f, err := os.Open(marker)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var got []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !sameStrings(got, want) {
		t.Fatalf("fake provider launches = %v, want %v", got, want)
	}
}
