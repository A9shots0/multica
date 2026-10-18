package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProviderChildEnvironmentExcludesBrokerCapabilityAndAPIKeys(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "mat_parent_secret")
	t.Setenv("ANTHROPIC_API_KEY", "provider-api-secret")
	t.Setenv("OPENROUTER_API_KEY", "router-secret")
	env, err := providerChildEnvironment(t.TempDir(), filepath.Join(t.TempDir(), "provider.exe"), "task_local_fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"MULTICA_TOKEN", "MULTICA_SERVER_URL", "MULTICA_TASK_ID", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY"} {
		if _, present := env[forbidden]; present {
			t.Fatalf("parent-only key %s entered provider child environment", forbidden)
		}
	}
	if env["CLAUDE_CODE_MAX_RETRIES"] != "0" || env["CLAUDE_CODE_SAFE_MODE"] != "1" || env["CLAUDE_CONFIG_DIR"] == "" {
		t.Fatalf("provider child policy is incomplete: %#v", env)
	}
}

func TestRunProviderUsesOneExactToolFreeFakeInvocation(t *testing.T) {
	fake := buildFakeProvider(t)
	if err := validateProviderExecutable(fake, fileSHA256(t, fake)); err != nil {
		t.Fatal(err)
	}
	if err := validateProviderExecutable(fake, strings.Repeat("0", 64)); err == nil {
		t.Fatal("altered provider digest was accepted")
	}
	var streamed bytes.Buffer
	receipt, err := runProvider(context.Background(), fake, "task_local_fixture", "", "Return exactly "+leaderApproval+".", &streamed)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ResultText != leaderApproval || len(receipt.ResultLine) == 0 {
		t.Fatalf("fake provider receipt = %#v", receipt)
	}
	if strings.Contains(streamed.String(), `"type":"result"`) {
		t.Fatal("terminal provider result was exposed before broker action")
	}
	if strings.Join(receipt.Models, ",") != acceptedBackgroundModel+","+exactClaudeModel {
		t.Fatalf("observed models = %v", receipt.Models)
	}
}

func TestRunProviderRejectsUsageOnlyExactModelClaim(t *testing.T) {
	fake := buildFakeProviderWithLinker(t, "-X=main.transcriptMode=usage_only")
	var streamed bytes.Buffer
	if _, err := runProvider(context.Background(), fake, "task_local_fixture", "", "Return exactly "+leaderApproval+".", &streamed); err == nil || !strings.Contains(err.Error(), "not bound to one exact primary assistant response") {
		t.Fatalf("usage-only exact-model transcript error = %v", err)
	}
}

func TestFakeBrokerActionsCreateExactArtifactsAndAdvanceWorkflow(t *testing.T) {
	repo := initTestRepository(t)
	reportParent := t.TempDir()
	report := filepath.Join(reportParent, "report")
	restore := overridePilotPaths(t, []rootSpec{{Index: 1, Label: "fixture", Path: repo}}, report)
	defer restore()

	var mu sync.Mutex
	var commentBodies, eventBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mat_fixture" || r.Header.Get("X-Task-ID") != "task-1" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/issues/issue-1/task-runs":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "task-1", "issue_id": "issue-1", "agent_id": "agent-1", "status": "running"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/issues/issue-1/workflow-plan":
			_ = json.NewEncoder(w).Encode(map[string]any{"implementer_id": "implementer-1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/issues/issue-1/comments":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			commentBodies = append(commentBodies, body)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/issues/issue-1/workflow-events":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			eventBodies = append(eventBodies, body)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("MULTICA_SERVER_URL", server.URL)
	t.Setenv("MULTICA_TOKEN", "mat_fixture")
	t.Setenv("MULTICA_AGENT_ID", "agent-1")
	t.Setenv("MULTICA_TASK_ID", "task-1")
	t.Setenv("MULTICA_ISSUE_ID", "issue-1")

	if err := routeToImplementer(context.Background()); err != nil {
		t.Fatal(err)
	}
	inventory, err := collectInventory(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := performRoleAction(context.Background(), executorAgentName, "", executorApproval, preparedRole{Inventory: &inventory}); err != nil {
		t.Fatal(err)
	}
	// The single allowed correction replaces the same three files in place.
	// This specifically exercises Windows replacement semantics before a real
	// replay can reach that branch.
	corrected, err := collectInventory(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeArtifacts(report, corrected, true); err != nil {
		t.Fatalf("replace exact artifacts during correction: %v", err)
	}
	verification, err := verifyArtifacts(report)
	if err != nil || !verification.Pass {
		t.Fatalf("artifact verification = %#v, %v", verification, err)
	}
	if err := performRoleAction(context.Background(), reviewerAgentName, "review", reviewerPass, preparedRole{Verification: &verification}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(commentBodies) != 1 || !strings.Contains(commentBodies[0]["content"].(string), "mention://agent/implementer-1") {
		t.Fatalf("leader routing bodies = %#v", commentBodies)
	}
	if len(eventBodies) != 2 || eventBodies[0]["event"] != "implementation_ready" || eventBodies[1]["event"] != "review_passed" {
		t.Fatalf("workflow event bodies = %#v", eventBodies)
	}
	evidence, ok := eventBodies[0]["artifact_evidence"].(map[string]any)
	if !ok || evidence["contract"] != "repository_worktree_inventory.v1" {
		t.Fatalf("artifact evidence was not attached: %#v", eventBodies[0])
	}
}

func TestBrokerFailsClosedForRedirectToolAttemptAndAlteredArtifacts(t *testing.T) {
	for _, raw := range []string{
		`{"type":"control_request"}`,
		`{"type":"assistant","message":{"model":"claude-sonnet-4-6","content":[{"type":"tool_use"}]}}`,
	} {
		var event map[string]any
		if json.Unmarshal([]byte(raw), &event) != nil {
			t.Fatal("bad fixture")
		}
		if _, _, err := inspectProviderEvent(event, map[string]struct{}{}); err == nil {
			t.Fatalf("unsafe provider event accepted: %s", raw)
		}
	}
	for _, raw := range []string{
		`{"type":"assistant","message":{"model":"claude-haiku-4-5-20251001","content":[{"type":"text","text":"ITEM14_ROUTE_APPROVED"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"ITEM14_ROUTE_APPROVED"}]}}`,
		`{"type":"assistant","message":{"model":"claude-sonnet-4-6","content":[{"type":"thinking","thinking":"hidden"}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result"}]}}`,
		`{"type":"server_tool_result"}`,
	} {
		var event map[string]any
		if json.Unmarshal([]byte(raw), &event) != nil {
			t.Fatal("bad adversarial fixture")
		}
		if _, _, err := inspectProviderEvent(event, map[string]struct{}{}); err == nil {
			t.Fatalf("ambiguous provider event accepted: %s", raw)
		}
	}
	t.Setenv("MULTICA_SERVER_URL", "https://example.com")
	if _, err := requireLoopbackServer(); err == nil {
		t.Fatal("non-loopback server was accepted")
	}

	repo := initTestRepository(t)
	report := filepath.Join(t.TempDir(), "report")
	restore := overridePilotPaths(t, []rootSpec{{Index: 1, Label: "fixture", Path: repo}}, report)
	defer restore()
	inventory, err := collectInventory(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeArtifacts(report, inventory, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(report, "inventory.json"), []byte(`{"altered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	verification, err := verifyArtifacts(report)
	if err == nil && verification.Pass {
		t.Fatal("altered artifact passed independent verification")
	}
}

func buildFakeProvider(t *testing.T) string {
	return buildFakeProviderWithLinker(t, "")
}

func buildFakeProviderWithLinker(t *testing.T, linkerFlags string) string {
	t.Helper()
	ext := ".exe"
	if runtime.GOOS != "windows" {
		ext = ""
	}
	path := filepath.Join(t.TempDir(), "fake-provider"+ext)
	args := []string{"build", "-buildvcs=false", "-o", path}
	if linkerFlags != "" {
		args = append(args, "-ldflags", linkerFlags)
	}
	args = append(args, "./fake-provider")
	cmd := exec.Command("go", args...)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake provider: %v: %s", err, output)
	}
	return path
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func initTestRepository(t *testing.T) string {
	t.Helper()
	fixtureGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	commands := [][]string{
		{"init", root},
		{"-C", root, "config", "user.email", "item14@example.invalid"},
		{"-C", root, "config", "user.name", "Item 14 Fixture"},
	}
	for _, args := range commands {
		if output, err := exec.Command(fixtureGit, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", root, "add", "README.md"}, {"-C", root, "commit", "-m", "fixture"}} {
		if output, err := exec.Command(fixtureGit, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	return root
}

func overridePilotPaths(t *testing.T, roots []rootSpec, report string) func() {
	t.Helper()
	fixtureGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	oldGit := inventoryGit
	inventoryGit = func(root string, args ...string) (string, error) {
		return runGitWithExecutable(fixtureGit, root, args...)
	}
	oldRoots := allowedRoots
	oldReport := authorizedReportDirectory
	allowedRoots = roots
	authorizedReportDirectory = report
	return func() {
		inventoryGit = oldGit
		allowedRoots = oldRoots
		authorizedReportDirectory = oldReport
	}
}
