package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/workhistory"
	"github.com/spf13/cobra"
)

func historyTestRoot(t *testing.T, serverURL, workspaceID, token string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "multica"}
	root.PersistentFlags().String("server-url", serverURL, "")
	root.PersistentFlags().String("workspace-id", workspaceID, "")
	root.PersistentFlags().String("profile", "", "")
	root.SetOut(&strings.Builder{})
	root.SetErr(&strings.Builder{})
	root.AddCommand(newHistoryCommand())
	t.Setenv("MULTICA_TOKEN", token)
	t.Setenv("MULTICA_SERVER_URL", serverURL)
	t.Setenv("MULTICA_WORKSPACE_ID", workspaceID)
	return root
}

func writeHistoryFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "session.jsonl")
	lines := []map[string]any{
		{"timestamp": "2026-10-15T13:00:00Z", "ordinal": 1, "type": "session_meta", "payload": map[string]any{"id": "history-session"}},
		{"timestamp": "2026-10-15T13:01:00Z", "ordinal": 2, "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "LinkHub 403 owner@example.com"}}}},
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, line := range lines {
		if err := encoder.Encode(line); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHistoryImportRedactsPostsAndCheckpoints(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv(cli.TaskConfigRootEnv, configRoot)
	sessionRoot := t.TempDir()
	session := writeHistoryFixture(t, sessionRoot)
	const workspaceID = "11111111-1111-4111-8111-111111111111"
	const projectID = "22222222-2222-4222-8222-222222222222"
	var calls atomic.Int32
	var sourceReference string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/work-history/fragments" || r.Header.Get("X-Workspace-ID") != workspaceID || r.Header.Get("Authorization") != "Bearer owner-token" {
			t.Fatalf("unexpected request %s workspace=%q", r.URL.Path, r.Header.Get("X-Workspace-ID"))
		}
		var request struct {
			ProjectID string `json:"project_id"`
			Fragments []struct {
				Text            string `json:"text"`
				EvidenceClass   string `json:"evidence_class"`
				SourceReference string `json:"source_reference"`
			} `json:"fragments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.ProjectID != projectID || len(request.Fragments) != 1 || strings.Contains(request.Fragments[0].Text, "owner@example.com") || !strings.Contains(request.Fragments[0].Text, "[REDACTED:EMAIL]") || request.Fragments[0].EvidenceClass != "conversation_recollection" {
			t.Fatalf("unsafe import: %+v", request)
		}
		sourceReference = request.Fragments[0].SourceReference
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"2026-10-15T13:02:00Z","recorded":{"submitted":1,"inserted":1,"unchanged":0,"recordClass":"conversation_recollection"}}`))
	}))
	defer server.Close()

	root := historyTestRoot(t, server.URL, workspaceID, "owner-token")
	root.SetArgs([]string{"history", "import", "--provider", "codex", "--root", sessionRoot, "--file", session, "--project-id", projectID})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}

	root = historyTestRoot(t, server.URL, workspaceID, "owner-token")
	root.SetArgs([]string{"history", "import", "--provider", "codex", "--root", sessionRoot, "--file", session, "--project-id", projectID})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("unchanged source made another network call: %d", calls.Load())
	}
	location, err := workhistory.ParseSourceReference(sourceReference)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveHistorySource("", location)
	if err != nil {
		t.Fatal(err)
	}
	canonicalSession, err := filepath.EvalSymlinks(session)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Path != canonicalSession || resolved.SessionID != "history-session" || resolved.SourceOffset <= 0 {
		t.Fatalf("unexpected resolved source: %+v", resolved)
	}
}

func TestHistoryDryRunDoesNotSaveStateOrUseNetwork(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv(cli.TaskConfigRootEnv, configRoot)
	sessionRoot := t.TempDir()
	session := writeHistoryFixture(t, sessionRoot)
	root := historyTestRoot(t, "", "11111111-1111-4111-8111-111111111111", "")
	root.SetArgs([]string{"history", "import", "--provider", "codex", "--root", sessionRoot, "--file", session,
		"--project-id", "22222222-2222-4222-8222-222222222222", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configRoot, "work-history")); !os.IsNotExist(err) {
		t.Fatalf("dry run created state: %v", err)
	}
}

func TestHistoryImportBatchesStayBelowBrainPayloadLimit(t *testing.T) {
	const projectID = "22222222-2222-4222-8222-222222222222"
	fragments := make([]workhistory.Fragment, 12)
	for index := range fragments {
		fragments[index] = workhistory.Fragment{Text: strings.Repeat(string(rune('a'+index)), 60*1024)}
	}
	batches, err := historyImportBatches(projectID, fragments)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) < 2 {
		t.Fatalf("batches = %d, want multiple bounded requests", len(batches))
	}
	var submitted int
	for _, batch := range batches {
		raw, err := json.Marshal(map[string]any{"project_id": projectID, "fragments": batch})
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) > maxHistoryImportRequestBytes {
			t.Fatalf("request size = %d, exceeds %d", len(raw), maxHistoryImportRequestBytes)
		}
		submitted += len(batch)
	}
	if submitted != len(fragments) {
		t.Fatalf("submitted = %d, want %d", submitted, len(fragments))
	}
}
