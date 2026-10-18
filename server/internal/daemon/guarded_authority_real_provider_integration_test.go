//go:build agentintegration

package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	guardedRealShadowCredentialPath = `C:\Users\a9sho\.claude\.credentials.json`
	guardedRealShadowModel          = "claude-sonnet-4-6"
	guardedRealShadowEffort         = "medium"
	guardedRealShadowDaemonID       = "item14-model-boundary-shadow-once"
)

// TestGuardedAuthorityRealClaudeSubscriptionShadow is deliberately one test,
// one task, one process, and one provider turn. Do not add table cases,
// subtests, retry logic, or a second execution to this test.
func TestGuardedAuthorityRealClaudeSubscriptionShadow(t *testing.T) {
	prepareOnly := os.Getenv("MULTICA_ITEM14_REAL_SHADOW_PREPARE_ONLY") == "1"
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" && !prepareOnly {
		t.Skip("set MULTICA_RUN_REAL_AGENT_SMOKE=1 only with exact owner authority")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	claudeExecutable, err := agent.ResolveGuardedExecutablePath("claude", "claude")
	if err != nil {
		t.Fatalf("resolve native Claude executable without running it: %v", err)
	}
	facts, err := guardedHostExecutionFacts("claude", claudeExecutable)
	if err != nil {
		t.Fatal(err)
	}
	pool := guardedShadowDatabase(t)
	workspaceID, _, runtimeID, agentID, issueID := seedGuardedShadowBase(t, pool)
	if _, err := pool.Exec(ctx, `UPDATE agent_runtime SET daemon_id = $1, name = 'Item 14 real model boundary shadow once' WHERE id = $2`, guardedRealShadowDaemonID, runtimeID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent SET name = 'item14-model-boundary-shadow-once', description = 'non-promotable one-call execution boundary fixture', model = $1, thinking_level = $2 WHERE id = $3`, guardedRealShadowModel, guardedRealShadowEffort, agentID); err != nil {
		t.Fatal(err)
	}

	hub := realtime.NewHub()
	go hub.Run()
	h := handler.New(db.New(pool), pool, hub, events.New(), service.NewEmailService(), nil, nil, analytics.NoopClient{}, handler.Config{
		WorkflowControllerID: util.MustParseUUID(guardedShadowControllerID),
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeCtx := chi.NewRouteContext()
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		requestCtx := middleware.WithDaemonContext(r.Context(), workspaceID, guardedRealShadowDaemonID)
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
			http.Error(w, "unexpected real shadow route", http.StatusNotFound)
		}
	}))
	defer server.Close()

	d := New(Config{
		ServerBaseURL: server.URL, DaemonID: guardedRealShadowDaemonID,
		HostAttestation: guardedShadowHost,
		Agents:          map[string]AgentEntry{"claude": {Path: claudeExecutable}},
		WorkspacesRoot:  t.TempDir(), AgentTimeout: 90 * time.Second,
	}, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	d.executionEnvironmentCommand = nil
	d.client.SetToken("item14-real-shadow-task-local-token")

	taskID, issued := seedGuardedShadowTaskAuthorityWithOptions(t, ctx, pool, h.PermissionReceiptStore, facts, agentID, runtimeID, issueID, guardedShadowAuthorityOptions{
		CredentialSourcePath: guardedRealShadowCredentialPath,
		Suffix:               "real-claude-subscription-once",
		ExpiresAt:            time.Now().Add(10 * time.Minute),
		ConfigurationID:      "item14.claude-subscription.boundary-shadow.once.v1",
		Model:                guardedRealShadowModel,
		Effort:               guardedRealShadowEffort,
		Prompt:               guardedShadowPrompt,
	})
	claimed, err := d.client.ClaimTask(ctx, runtimeID)
	if err != nil || claimed == nil || claimed.ID != taskID {
		t.Fatalf("claim exact real shadow task = %+v, error = %v", claimed, err)
	}
	packageDigest, err := agentqualification.GuardedExecutionPackageSHA256V1(claimed.GuardedExecution)
	if err != nil {
		t.Fatal(err)
	}
	if prepareOnly {
		t.Logf("ITEM14_REAL_SHADOW_PREPARE_ONLY provider=claude model=%s effort=%s package_sha256=%s credential_opened=false provider_started=false", guardedRealShadowModel, guardedRealShadowEffort, packageDigest)
		return
	}

	// This is the sole authorized call site. There is intentionally no retry.
	result, err := d.runTask(ctx, *claimed, "claude", 0, slog.Default())
	if err != nil {
		t.Fatalf("single guarded Claude subscription inference: %v", err)
	}
	if result.Status != "completed" || result.SessionID != "" {
		t.Fatalf("real shadow terminal result = status %q session persisted %t", result.Status, result.SessionID != "")
	}
	if result.ProviderTurns != 1 || len(result.ProviderModels) != 1 || result.ProviderModels[0] != guardedRealShadowModel {
		t.Fatalf("provider terminal evidence = models %v turns %d, want [%s] and 1", result.ProviderModels, result.ProviderTurns, guardedRealShadowModel)
	}
	validateGuardedRealShadowOutput(t, result.Comment)
	if len(result.Usage) != 1 {
		t.Fatalf("provider usage entries = %d, want 1", len(result.Usage))
	}
	usage := result.Usage[0]
	if usage.Provider != "claude" || usage.Model != guardedRealShadowModel || usage.InputTokens <= 0 || usage.OutputTokens <= 0 {
		t.Fatalf("provider usage identity/counts did not match the bound call: provider=%q model=%q input=%d output=%d", usage.Provider, usage.Model, usage.InputTokens, usage.OutputTokens)
	}
	assertGuardedShadowConsumption(t, pool, taskID, issued.CapabilityScope.Receipt.ID, 1)
	if err := d.client.CompleteTask(ctx, taskID, result.Comment, result.BranchName, result.SessionID, result.WorkDir, result.SessionRolloutMissing, result.RetiredSessionID, result.DurableWorkDir); err != nil {
		t.Fatalf("complete real model-only shadow task: %v", err)
	}
	outputSum := sha256.Sum256([]byte(result.Comment))
	t.Logf("ITEM14_REAL_SHADOW_PASS provider=claude model=%s effort=%s calls=1 retries=0 tools=0 consumption=1 cleanup_verified=true", guardedRealShadowModel, guardedRealShadowEffort)
	t.Logf("ITEM14_REAL_SHADOW_RECEIPTS executable_sha256=%s package_sha256=%s rendered_prompt_sha256=%s output_sha256=%s", facts.ExecutableSHA256, packageDigest, issued.CapabilityScope.ModelOnlyShadow.RenderedPromptSHA256, hex.EncodeToString(outputSum[:]))
	t.Logf("ITEM14_REAL_SHADOW_USAGE input=%d output=%d cache_read=%d cache_write=%d cost_ticks=%d", usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheWriteTokens, usage.CostUSDTicks)
}

func validateGuardedRealShadowOutput(t *testing.T, output string) {
	t.Helper()
	var decoded struct {
		NamedBoundary   string `json:"named_boundary"`
		AuthoritySource string `json:"authority_source"`
		ExpectedState   string `json:"expected_state"`
		ObservedState   string `json:"observed_state"`
		Comparison      struct {
			Numerator   int    `json:"numerator"`
			Denominator int    `json:"denominator"`
			Definition  string `json:"definition"`
		} `json:"comparison"`
		FirstDivergence             string   `json:"first_divergence"`
		AffectedConsumers           []string `json:"affected_consumers"`
		RepairConstraints           []string `json:"repair_constraints"`
		RequiredReconciliationProof []string `json:"required_reconciliation_proof"`
		Evidence                    []any    `json:"evidence"`
		RouteSettled                bool     `json:"route_settled"`
		StopReason                  string   `json:"stop_reason"`
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatalf("provider output was not a strict JSON object: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("provider output contained trailing data: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &keys); err != nil || len(keys) != 12 {
		t.Fatalf("provider output field count = %d, error = %v", len(keys), err)
	}
	exactFields := map[string]string{
		"named_boundary":                `"item14_execution_boundary"`,
		"authority_source":              `""`,
		"expected_state":                `""`,
		"observed_state":                `""`,
		"first_divergence":              `""`,
		"affected_consumers":            `[]`,
		"repair_constraints":            `[]`,
		"required_reconciliation_proof": `[]`,
		"evidence":                      `[]`,
		"route_settled":                 `false`,
		"stop_reason":                   `"insufficient supplied evidence"`,
	}
	for key, want := range exactFields {
		var compact bytes.Buffer
		if err := json.Compact(&compact, keys[key]); err != nil || compact.String() != want {
			t.Fatalf("provider output field %q = %s, want %s (error %v)", key, compact.String(), want, err)
		}
	}
	var comparison map[string]json.RawMessage
	if err := json.Unmarshal(keys["comparison"], &comparison); err != nil || len(comparison) != 3 {
		t.Fatalf("provider comparison shape is not exact: fields=%d error=%v", len(comparison), err)
	}
	for key, want := range map[string]string{"numerator": "0", "denominator": "0", "definition": `"no evidence supplied"`} {
		var compact bytes.Buffer
		if err := json.Compact(&compact, comparison[key]); err != nil || compact.String() != want {
			t.Fatalf("provider comparison field %q = %s, want %s (error %v)", key, compact.String(), want, err)
		}
	}
	if decoded.NamedBoundary != "item14_execution_boundary" || decoded.RouteSettled || decoded.StopReason != "insufficient supplied evidence" ||
		decoded.AuthoritySource != "" || decoded.ExpectedState != "" || decoded.ObservedState != "" || decoded.FirstDivergence != "" ||
		decoded.Comparison.Numerator != 0 || decoded.Comparison.Denominator != 0 || decoded.Comparison.Definition != "no evidence supplied" ||
		len(decoded.AffectedConsumers) != 0 || len(decoded.RepairConstraints) != 0 || len(decoded.RequiredReconciliationProof) != 0 || len(decoded.Evidence) != 0 {
		t.Fatal("provider output did not match the fixed harmless response contract")
	}
}
