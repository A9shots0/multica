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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	guardedShadowControllerID = "00000000-0000-0000-0000-000000001401"
	guardedShadowDaemonID     = "item14-provider-free-shadow-daemon"
	guardedShadowHost         = "alienware_windows"
	guardedShadowModel        = "fake-provider-model-only-v1"
	guardedShadowEffort       = "fake-provider-effort-v1"
	guardedShadowPrompt       = "This is an execution-boundary probe. No repository, tools, evidence source, production system, or outbound action is available or authorized. Return exactly the twelve-field JSON object required by the bound Data Sync and Integrity v2 instructions. Set `named_boundary` to `item14_execution_boundary`, `route_settled` to `false`, and `stop_reason` to `insufficient supplied evidence`. Use empty strings and arrays for all other fields, except `comparison`, which must use zero numerator, zero denominator, and definition `no evidence supplied`. Do not request or attempt any action."
)

// TestGuardedAuthorityFakeProviderShadow crosses the real controller store,
// claim handler, daemon HTTP client, task start transaction, executable
// staging, one-use fake credential capsule, and Claude tool-free adapter. The
// executable is a local fixture and has no network/provider implementation.
func TestGuardedAuthorityFakeProviderShadow(t *testing.T) {
	pool := guardedShadowDatabase(t)
	ctx := context.Background()
	marker := filepath.Join(t.TempDir(), "fake-provider-launches")
	fakeCredential := filepath.Join(t.TempDir(), "fake-credential.json")
	if err := os.WriteFile(fakeCredential, []byte("fake-credential-only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact, err := agentqualification.ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		t.Fatal(err)
	}
	renderedPrompt, err := agentqualification.RenderGuardedModelOnlyPromptV1(artifact, guardedShadowPrompt)
	if err != nil {
		t.Fatal(err)
	}
	fakeProvider := buildGuardedShadowFakeProvider(t, marker, agentqualification.GuardedPromptSHA256V1(renderedPrompt))
	facts, err := guardedHostExecutionFacts("claude", fakeProvider)
	if err != nil {
		t.Fatal(err)
	}

	workspaceID, _, runtimeID, agentID, issueID := seedGuardedShadowBase(t, pool)
	hub := realtime.NewHub()
	go hub.Run()
	h := handler.New(db.New(pool), pool, hub, events.New(), service.NewEmailService(), nil, nil, analytics.NoopClient{}, handler.Config{
		WorkflowControllerID: util.MustParseUUID(guardedShadowControllerID),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeCtx := chi.NewRouteContext()
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")
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
			http.Error(w, "unexpected provider-free shadow route", http.StatusNotFound)
		}
	}))
	defer server.Close()

	d := New(Config{
		ServerBaseURL: server.URL, DaemonID: guardedShadowDaemonID,
		HostAttestation: guardedShadowHost,
		Agents:          map[string]AgentEntry{"claude": {Path: fakeProvider}},
		WorkspacesRoot:  t.TempDir(), AgentTimeout: 10 * time.Second,
	}, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	// The installed multica binary normally owns the isolated preparation
	// helper mode. A Go test binary does not, so keep environment preparation
	// in-process while still crossing the production daemon preparation logic;
	// the provider process itself uses the exact isolated child boundary.
	d.executionEnvironmentCommand = nil
	d.client.SetToken("provider-free-test-token")

	positiveTaskID, positiveIssue := seedGuardedShadowTaskAuthority(t, ctx, pool, h.PermissionReceiptStore, facts, agentID, runtimeID, issueID, fakeCredential, "positive", time.Now().Add(10*time.Minute))
	// Exact controller retries converge on the same immutable authority rather
	// than minting a second launch.
	if _, err := h.PermissionReceiptStore.IssueGuardedExecutionAuthority(ctx, guardedShadowControllerID, positiveIssue, time.Now().UTC()); err != nil {
		t.Fatalf("exact authority retry: %v", err)
	}
	claimed, err := d.client.ClaimTask(ctx, runtimeID)
	if err != nil || claimed == nil || claimed.ID != positiveTaskID {
		t.Fatalf("real claim = %+v, error = %v", claimed, err)
	}
	if claimed.GuardedExecution == nil || claimed.Agent == nil || claimed.Agent.Model != guardedShadowModel || claimed.Agent.ThinkingLevel != guardedShadowEffort {
		t.Fatalf("claim did not carry exact approved package: %+v", claimed)
	}
	claimedDigest, err := agentqualification.GuardedExecutionPackageSHA256V1(claimed.GuardedExecution)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a server restart that lost controller configuration after the
	// claim. Guarded start must remain denied, while ordinary legacy tasks are
	// unaffected by the always-installed gate.
	withoutController := handler.New(db.New(pool), pool, hub, events.New(), service.NewEmailService(), nil, nil, analytics.NoopClient{}, handler.Config{})
	configLossBody, err := json.Marshal(map[string]string{"guarded_execution_package_sha256": claimedDigest})
	if err != nil {
		t.Fatal(err)
	}
	configLossRequest := httptest.NewRequest(http.MethodPost, "/api/daemon/tasks/"+positiveTaskID+"/start", strings.NewReader(string(configLossBody)))
	configLossRoute := chi.NewRouteContext()
	configLossRoute.URLParams.Add("taskId", positiveTaskID)
	configLossContext := middleware.WithDaemonContext(configLossRequest.Context(), workspaceID, guardedShadowDaemonID)
	configLossRequest = configLossRequest.WithContext(context.WithValue(configLossContext, chi.RouteCtxKey, configLossRoute))
	configLossResponse := httptest.NewRecorder()
	withoutController.StartTask(configLossResponse, configLossRequest)
	if configLossResponse.Code == http.StatusOK {
		t.Fatal("guarded start succeeded after controller configuration loss")
	}
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 0)

	altered := *claimed
	alteredPackage := *claimed.GuardedExecution
	alteredEnvelope := alteredPackage.Envelope
	alteredEnvelope.Model = "altered-after-claim"
	alteredPackage.Envelope = alteredEnvelope
	altered.GuardedExecution = &alteredPackage
	if _, err := d.runTask(ctx, altered, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "guarded execution denied") {
		t.Fatalf("altered authority run error = %v", err)
	}
	assertGuardedShadowLaunches(t, marker, 0)

	alteredPrompt := *claimed
	alteredPromptPackage := *claimed.GuardedExecution
	alteredPromptScope := alteredPromptPackage.CapabilityScope
	alteredPromptShadow := *alteredPromptScope.ModelOnlyShadow
	alteredPromptShadow.Prompt += " altered"
	alteredPromptScope.ModelOnlyShadow = &alteredPromptShadow
	alteredPromptPackage.CapabilityScope = alteredPromptScope
	alteredPrompt.GuardedExecution = &alteredPromptPackage
	if _, err := d.runTask(ctx, alteredPrompt, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "guarded execution denied") {
		t.Fatalf("altered prompt run error = %v", err)
	}
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 0)
	assertGuardedShadowLaunches(t, marker, 0)

	alteredCredential := *claimed
	alteredCredentialPackage := *claimed.GuardedExecution
	alteredCredentialScope := alteredCredentialPackage.CapabilityScope
	alteredCredentialShadow := *alteredCredentialScope.ModelOnlyShadow
	alteredCredentialShadow.Credential.SourcePath = filepath.Join(t.TempDir(), "coherent-but-unapproved.credentials.json")
	alteredCredentialScope.ModelOnlyShadow = &alteredCredentialShadow
	alteredCredentialPackage.CapabilityScope = alteredCredentialScope
	alteredCredential.GuardedExecution = &alteredCredentialPackage
	if _, err := d.runTask(ctx, alteredCredential, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "start task failed") {
		t.Fatalf("altered credential capsule run error = %v", err)
	}
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 0)
	assertGuardedShadowLaunches(t, marker, 0)

	alteredIdentity := *claimed
	alteredIdentity.ID = "00000000-0000-0000-0000-000000001499"
	if _, err := d.runTask(ctx, alteredIdentity, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "start task failed") {
		t.Fatalf("altered task identity run error = %v", err)
	}
	assertGuardedShadowLaunches(t, marker, 0)

	wrongExecutableMarker := filepath.Join(t.TempDir(), "wrong-executable-launches")
	wrongExecutable := buildGuardedShadowFakeProvider(t, wrongExecutableMarker, agentqualification.GuardedPromptSHA256V1(renderedPrompt)+"-different")
	dWrongExecutable := New(Config{
		ServerBaseURL: server.URL, DaemonID: guardedShadowDaemonID,
		HostAttestation: guardedShadowHost,
		Agents:          map[string]AgentEntry{"claude": {Path: wrongExecutable}},
		WorkspacesRoot:  t.TempDir(), AgentTimeout: 10 * time.Second,
	}, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	dWrongExecutable.executionEnvironmentCommand = nil
	dWrongExecutable.client.SetToken("provider-free-test-token")
	if _, err := dWrongExecutable.runTask(ctx, *claimed, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "guarded execution denied") {
		t.Fatalf("altered executable run error = %v", err)
	}
	assertGuardedShadowLaunches(t, wrongExecutableMarker, 0)
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 0)

	coherentAltered := *claimed
	coherentAlteredPackage := *claimed.GuardedExecution
	coherentAlteredReceipt := coherentAlteredPackage.RuntimeBinding
	coherentAlteredReceipt.EvidenceReceiptIDs = append([]string(nil), coherentAlteredReceipt.EvidenceReceiptIDs...)
	coherentAlteredReceipt.EvidenceReceiptIDs = append(coherentAlteredReceipt.EvidenceReceiptIDs, "self-consistent-but-unapproved-evidence")
	coherentAlteredPackage.RuntimeBinding = coherentAlteredReceipt
	coherentAltered.GuardedExecution = &coherentAlteredPackage
	if _, err := d.runTask(ctx, coherentAltered, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "start task failed") {
		t.Fatalf("coherent altered authority run error = %v", err)
	}
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 0)
	assertGuardedShadowLaunches(t, marker, 0)
	if err := d.client.StartTask(ctx, positiveTaskID, strings.Repeat("a", 64)); err == nil {
		t.Fatal("controller accepted a package digest not present in its immutable authority")
	}
	var status string
	var startedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, started_at FROM agent_task_queue WHERE id = $1`, positiveTaskID).Scan(&status, &startedAt); err != nil {
		t.Fatal(err)
	}
	if status != "dispatched" || startedAt != nil {
		t.Fatalf("wrong package digest persisted start status=%q started_at=%v", status, startedAt)
	}
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 0)
	assertGuardedShadowLaunches(t, marker, 0)

	result, err := d.runTask(ctx, *claimed, "claude", 0, slog.Default())
	if err != nil {
		t.Fatalf("provider-free guarded run: %v", err)
	}
	if result.Status != "completed" || result.Comment != "provider-free-ok" || result.ProviderTurns != 1 || len(result.ProviderModels) != 1 || result.ProviderModels[0] != guardedShadowModel {
		t.Fatalf("provider-free result = %+v", result)
	}
	assertGuardedShadowLaunches(t, marker, 1)
	assertGuardedShadowCredentialCopiesAbsent(t, marker)
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 1)
	if err := d.client.CompleteTask(ctx, positiveTaskID, result.Comment, result.BranchName, result.SessionID, result.WorkDir, result.SessionRolloutMissing, result.RetiredSessionID, result.DurableWorkDir); err != nil {
		t.Fatalf("complete provider-free shadow task: %v", err)
	}

	if _, err := d.runTask(ctx, *claimed, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "start task failed") {
		t.Fatalf("replayed launch error = %v", err)
	}
	assertGuardedShadowLaunches(t, marker, 1)
	assertGuardedShadowConsumption(t, pool, positiveTaskID, positiveIssue.CapabilityScope.Receipt.ID, 1)

	revokedTaskID, revokedIssue := seedGuardedShadowTaskAuthority(t, ctx, pool, h.PermissionReceiptStore, facts, agentID, runtimeID, issueID, fakeCredential, "revoked", time.Now().Add(10*time.Minute))
	revokedClaim, err := d.client.ClaimTask(ctx, runtimeID)
	if err != nil || revokedClaim == nil || revokedClaim.ID != revokedTaskID {
		t.Fatalf("revoked-path claim = %+v, error = %v", revokedClaim, err)
	}
	if err := h.PermissionReceiptStore.RevokeQualifiedRuntimeBinding(ctx, guardedShadowControllerID, workspaceID, revokedIssue.Receipt.ID, "provider-free runtime qualification revocation proof", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.runTask(ctx, *revokedClaim, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "start task failed") {
		t.Fatalf("revoked launch error = %v", err)
	}
	assertGuardedShadowLaunches(t, marker, 1)
	assertGuardedShadowConsumption(t, pool, revokedTaskID, revokedIssue.CapabilityScope.Receipt.ID, 0)
	if _, err := h.TaskService.CancelTask(ctx, util.MustParseUUID(revokedTaskID)); err != nil {
		t.Fatalf("settle revoked fixture task: %v", err)
	}

	expiredTaskID, expiredIssue := seedGuardedShadowTaskAuthority(t, ctx, pool, h.PermissionReceiptStore, facts, agentID, runtimeID, issueID, fakeCredential, "expired", time.Now().Add(1200*time.Millisecond))
	expiredClaim, err := d.client.ClaimTask(ctx, runtimeID)
	if err != nil || expiredClaim == nil || expiredClaim.ID != expiredTaskID {
		t.Fatalf("expired-path claim = %+v, error = %v", expiredClaim, err)
	}
	time.Sleep(1300 * time.Millisecond)
	if _, err := d.runTask(ctx, *expiredClaim, "claude", 0, slog.Default()); err == nil || !strings.Contains(err.Error(), "guarded execution denied") {
		t.Fatalf("expired launch error = %v", err)
	}
	assertGuardedShadowConsumption(t, pool, expiredTaskID, expiredIssue.CapabilityScope.Receipt.ID, 0)
	assertGuardedShadowLaunches(t, marker, 1)
}

func guardedShadowDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	databaseURL := os.Getenv("MULTICA_ITEM14_DISPOSABLE_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set explicit loopback-only MULTICA_ITEM14_DISPOSABLE_DATABASE_URL for the disposable shadow schema")
	}
	parsedDatabaseURL, err := url.Parse(databaseURL)
	if err != nil || parsedDatabaseURL.Scheme != "postgres" || parsedDatabaseURL.Port() == "" {
		t.Fatalf("Item 14 disposable database URL is invalid")
	}
	switch parsedDatabaseURL.Hostname() {
	case "127.0.0.1", "localhost", "::1":
	default:
		t.Fatalf("Item 14 disposable database must use an explicit loopback host")
	}
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Skipf("provider-free shadow database unavailable: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("provider-free shadow database unavailable: %v", err)
	}
	schema := fmt.Sprintf("guarded_shadow_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+identifier+" CASCADE")
		admin.Close()
	})
	for version := 544; version <= 557; version++ {
		matches, err := filepath.Glob(filepath.Join("..", "..", "migrations", fmt.Sprintf("%d_*.up.sql", version)))
		if err != nil || len(matches) != 1 {
			t.Fatalf("resolve guarded migration %d: matches=%v err=%v", version, matches, err)
		}
		body, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply guarded migration %d: %v", version, err)
		}
	}
	return pool
}

func seedGuardedShadowBase(t *testing.T, pool *pgxpool.Pool) (workspaceID, userID, runtimeID, agentID, issueID string) {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	if err := pool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Item 14 fake provider', $1) RETURNING id`, fmt.Sprintf("item14-shadow-%d@example.invalid", suffix)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('Item 14 fake provider', $1, 'provider-free only', 'I14') RETURNING id`, fmt.Sprintf("item14-shadow-%d", suffix)).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, workspaceID, userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at, visibility, owner_id)
		VALUES ($1, $2, 'Item 14 fake runtime', 'cloud', 'claude', 'online', 'provider-free fixture', '{}'::jsonb, now(), 'private', $3)
		RETURNING id`, workspaceID, guardedShadowDaemonID, userID).Scan(&runtimeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, description, runtime_mode, runtime_config, runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id, instructions, model, thinking_level)
		VALUES ($1, 'Item 14 fake executor', 'provider-free fixture only', 'cloud', '{}'::jsonb, $2, 'private', 'private', 8, $3, 'untrusted mutable instruction', $4, $5)
		RETURNING id`, workspaceID, runtimeID, userID, guardedShadowModel, guardedShadowEffort).Scan(&agentID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_id, creator_type, number, position)
		VALUES ($1, 'Item 14 provider-free shadow', 'in_progress', 'none', $2, 'member', 1, 0)
		RETURNING id`, workspaceID, userID).Scan(&issueID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM task_token WHERE task_id IN (SELECT id FROM agent_task_queue WHERE agent_id = $1)`, agentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent_task_message WHERE task_id IN (SELECT id FROM agent_task_queue WHERE agent_id = $1)`, agentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM issue WHERE id = $1`, issueID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspace WHERE id = $1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user" WHERE id = $1`, userID)
	})
	return workspaceID, userID, runtimeID, agentID, issueID
}

func seedGuardedShadowTaskAuthority(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	store *agentqualification.PermissionReceiptStore,
	facts agentqualification.GuardedHostExecutionFactsV1,
	agentID, runtimeID, issueID, credentialSourcePath, suffix string,
	expiresAt time.Time,
) (string, agentqualification.GuardedExecutionIssueV1) {
	return seedGuardedShadowTaskAuthorityWithOptions(t, ctx, pool, store, facts, agentID, runtimeID, issueID, guardedShadowAuthorityOptions{
		CredentialSourcePath: credentialSourcePath,
		Suffix:               suffix,
		ExpiresAt:            expiresAt,
		ConfigurationID:      "item14.provider-free.fixture.v1",
		Model:                guardedShadowModel,
		Effort:               guardedShadowEffort,
		Prompt:               guardedShadowPrompt,
	})
}

type guardedShadowAuthorityOptions struct {
	CredentialSourcePath string
	Suffix               string
	ExpiresAt            time.Time
	ConfigurationID      string
	Model                string
	Effort               string
	Prompt               string
}

func seedGuardedShadowTaskAuthorityWithOptions(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	store *agentqualification.PermissionReceiptStore,
	facts agentqualification.GuardedHostExecutionFactsV1,
	agentID, runtimeID, issueID string,
	options guardedShadowAuthorityOptions,
) (string, agentqualification.GuardedExecutionIssueV1) {
	t.Helper()
	var workspaceID string
	if err := pool.QueryRow(ctx, `SELECT workspace_id FROM issue WHERE id = $1`, issueID).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, context)
		VALUES ($1, $2, $3, 'queued', 0, '{}'::jsonb)
		RETURNING id`, agentID, runtimeID, issueID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
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
	receiptID := "item14-shadow-runtime-" + options.Suffix
	grantID := "item14-shadow-grant-" + options.Suffix
	configurationID := options.ConfigurationID
	receipt := agentqualification.QualifiedRuntimeBindingReceiptV2{
		Version: agentqualification.QualifiedRuntimeBindingVersionV2, ID: receiptID,
		WorkspaceID: workspaceID, IssueID: issueID, ExecutorAgentID: agentID,
		RuntimeID: runtimeID, ConfigurationID: configurationID, RoleID: artifact.RoleID,
		RuntimeProvider: "claude", Model: options.Model, ReasoningEffort: options.Effort,
		ChildExecution: agentqualification.GuardedChildExecutionBoundaryV1{
			Version: agentqualification.GuardedChildExecutionBoundaryVersionV1,
			HostOS:  facts.HostOS, Adapter: facts.Adapter,
			EnvironmentPolicy:          facts.EnvironmentPolicy,
			EnvironmentAllowlistSHA256: facts.EnvironmentAllowlistSHA256,
			ToolPolicy:                 facts.ToolPolicy, HostAttestationCustody: facts.HostAttestationCustody,
			ExecutableSHA256: facts.ExecutableSHA256,
		},
		ArtifactBindingSHA256: hex.EncodeToString(artifactSum[:]),
		QualificationStatus:   agentqualification.StatusQualifiedBounded,
		EvidenceReceiptIDs:    []string{"item14-shadow-evidence-" + options.Suffix},
		Source:                "controller_record", Immutable: true,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: options.ExpiresAt.UTC(),
	}
	envelope := agentqualification.GuardedDispatchEnvelopeV1{
		Version:     agentqualification.GuardedDispatchEnvelopeVersionV2,
		WorkspaceID: workspaceID, IssueID: issueID, ConfigurationID: configurationID,
		RoleID: artifact.RoleID, ExecutorAgentID: agentID, RuntimeID: runtimeID,
		RuntimeProvider: "claude", Model: options.Model, ReasoningEffort: options.Effort,
		ArtifactBindingID: artifact.BindingID, ArtifactBinding: artifact,
		RuntimeBindingReceiptID: receiptID,
	}
	route := systemgraph.RoutePlan{
		Mode: "contribute", AllowedHosts: []string{guardedShadowHost},
		ScopedNodeIDs: []string{"crm_product"}, CoverageGate: "expand",
		RiskClass: "bounded_read", RequestedAction: "read_boundary_state",
	}
	capability := systemgraph.CapabilityReceipt{
		ID: grantID, ExecutorAgentID: agentID, IssueID: issueID,
		AuthorityRevision: 2, RouteVersion: "item14-provider-free-route-v1",
		RoleID: artifact.RoleID, ConfigurationID: configurationID,
		Action: "read_boundary_state", TargetNodeIDs: []string{"crm_product"},
		HostAttestation: guardedShadowHost, AllowedTools: []string{"provider_free_result_fixture"},
		CredentialPurposes: []string{agentqualification.GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1},
		IssuedAt:           now.Add(-time.Minute), ExpiresAt: options.ExpiresAt.UTC(),
		CoverageReceiptID:           "item14-assurance-" + options.Suffix,
		OwnerAuthorizationReceiptID: "item14-owner-" + options.Suffix,
		CandidateID:                 "item14-candidate-" + options.Suffix, AuthorizedScope: "item14-execution-boundary-only",
	}
	assurance := &systemgraph.ArchitectureAssuranceReceipt{
		ID: capability.CoverageReceiptID, IssueID: issueID,
		RouteVersion: capability.RouteVersion, CoverageGate: route.CoverageGate,
		IssuerAgentID: "item14-fake-architecture-agent", IssuerRoleID: "architecture_assurance",
		IssuerConfigurationID: "item14.fake.architecture.v1", Source: "controller_record",
		Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: options.ExpiresAt.UTC(),
	}
	owner := &systemgraph.OwnerAuthorizationReceipt{
		ID: capability.OwnerAuthorizationReceiptID, IssueID: issueID,
		AuthorityRevision: capability.AuthorityRevision, CandidateID: capability.CandidateID,
		Action: capability.Action, TargetNodeIDs: capability.TargetNodeIDs,
		AuthorizedScope: capability.AuthorizedScope, IssuerRoleID: "owner",
		Source: "controller_record", Immutable: true, RecordedAt: now.Add(-time.Minute),
	}
	shadow, err := agentqualification.NewGuardedModelOnlyShadowV1(options.Prompt, options.CredentialSourcePath, artifact)
	if err != nil {
		t.Fatal(err)
	}
	scope := agentqualification.TaskCapabilityScopeV1{
		Version:     agentqualification.TaskCapabilityScopeVersionV2,
		WorkspaceID: workspaceID, IssueID: issueID, ExecutorAgentID: agentID,
		RuntimeID: runtimeID, RoleID: artifact.RoleID, ConfigurationID: configurationID,
		RuntimeBindingReceiptID: receiptID, RouteVersion: capability.RouteVersion,
		HostAttestation: guardedShadowHost, Action: capability.Action,
		Route: route, Receipt: capability, Assurance: assurance, OwnerAuthorization: owner,
		ModelOnlyShadow: &shadow,
	}
	issue := agentqualification.GuardedExecutionIssueV1{
		TaskID: taskID, Receipt: receipt, Envelope: envelope, CapabilityScope: scope,
	}
	if _, err := store.IssueGuardedExecutionAuthority(ctx, guardedShadowControllerID, issue, now); err != nil {
		t.Fatalf("issue guarded authority %s: %v", options.Suffix, err)
	}
	contextRaw, err := json.Marshal(map[string]any{agentqualification.GuardedDispatchTaskContextKey: envelope})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_task_queue SET context = $1::jsonb WHERE id = $2`, contextRaw, taskID); err != nil {
		t.Fatal(err)
	}
	return taskID, issue
}

func buildGuardedShadowFakeProvider(t *testing.T, marker, expectedPromptSHA256 string) string {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	binaryPath := filepath.Join(dir, "fake-claude")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	source := fmt.Sprintf(`package main
import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
)
func main() {
    if os.Getenv("CLAUDE_CODE_MAX_RETRIES") != "0" { os.Exit(25) }
    raw, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
    if err != nil { os.Exit(20) }
    var input struct { Message struct { Content []struct { Text string `+"`json:\"text\"`"+` } `+"`json:\"content\"`"+` } `+"`json:\"message\"`"+` }
    if err := json.Unmarshal(raw, &input); err != nil || len(input.Message.Content) != 1 { os.Exit(22) }
    sum := sha256.Sum256([]byte(input.Message.Content[0].Text))
    if hex.EncodeToString(sum[:]) != %q { os.Exit(23) }
    configDir := os.Getenv("CLAUDE_CONFIG_DIR")
    credentialPath := configDir + string(os.PathSeparator) + ".credentials.json"
    credential, err := os.ReadFile(credentialPath)
    if err != nil || string(credential) != "fake-credential-only\n" { os.Exit(24) }
    f, err := os.OpenFile(%q, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil { os.Exit(21) }
    _, _ = fmt.Fprintln(f, credentialPath)
    _ = f.Close()
    fmt.Println("{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"session_id\":\"provider-free\",\"result\":\"provider-free-ok\",\"num_turns\":1,\"modelUsage\":{\"fake-provider-model-only-v1\":{\"inputTokens\":1,\"outputTokens\":1,\"cacheReadInputTokens\":0,\"cacheCreationInputTokens\":0}}}")
}
`, expectedPromptSHA256, marker)
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "go-cache")
	tempDir := filepath.Join(dir, "go-tmp")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	cmd.Env = []string{
		"CGO_ENABLED=0", "GO111MODULE=off", "GOTOOLCHAIN=local",
		"GOCACHE=" + cacheDir, "GOTMPDIR=" + tempDir,
		"PATH=" + os.Getenv("PATH"), "TEMP=" + tempDir, "TMP=" + tempDir,
	}
	if runtime.GOOS == "windows" {
		cmd.Env = append(cmd.Env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile local fake provider: %v\n%s", err, output)
	}
	return binaryPath
}

func assertGuardedShadowCredentialCopiesAbsent(t *testing.T, marker string) {
	t.Helper()
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range strings.Fields(string(raw)) {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary fake credential still exists at %q (stat error %v)", path, err)
		}
	}
}

func assertGuardedShadowLaunches(t *testing.T, marker string, want int) {
	t.Helper()
	f, err := os.Open(marker)
	if os.IsNotExist(err) && want == 0 {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("fake provider launches = %d, want %d", count, want)
	}
}

func assertGuardedShadowConsumption(t *testing.T, pool *pgxpool.Pool, taskID, grantID string, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM guarded_execution_launch_consumption
		WHERE task_id = $1 AND capability_grant_id = $2`, taskID, grantID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("launch consumption count = %d, want %d", count, want)
	}
}
