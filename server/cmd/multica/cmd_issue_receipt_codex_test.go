package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

func writeCodexRolloutFixture(t *testing.T, path string, entries ...map[string]any) {
	t.Helper()
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func codexSessionEntry(ordinal int64) map[string]any {
	return map[string]any{
		"timestamp": "2026-10-15T12:00:00Z", "ordinal": ordinal, "type": "session_meta",
		"payload": map[string]any{"id": "run-1", "session_id": "thread-1"},
	}
}

func codexTaskEntry(ordinal int64, tool string, arguments map[string]any) map[string]any {
	return map[string]any{
		"timestamp": "2026-10-15T12:00:00Z", "ordinal": ordinal, "type": "event_msg",
		"payload": map[string]any{
			"type": "item_completed",
			"item": map[string]any{"type": "McpToolCall", "tool": tool, "status": "completed", "arguments": arguments},
		},
	}
}

func executeCodexObserver(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "multica", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("server-url", "", "")
	root.PersistentFlags().String("profile", "", "")
	issue := &cobra.Command{Use: "issue"}
	issue.AddCommand(newIssueReceiptCommand())
	root.AddCommand(issue)
	root.SetArgs(append([]string{"issue", "receipt", "observe-codex"}, args...))
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(context.Background())
	return stdout.String(), err
}

func targetTaskArguments(status, focus string) map[string]any {
	return map[string]any{
		"id": "linked-task", "name": "Connect outside work", "status": status,
		"endGoal":      "Outside Codex work appears automatically in Multica.",
		"currentFocus": focus, "remaining": "Ship and verify it.",
		"nextProof": "A real receipt appears once.", "progressPct": 50,
		"ownerActionRequired": false,
	}
}

func TestScanCodexRolloutReadsOnlyStructuredExactTaskEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout.jsonl")
	writeCodexRolloutFixture(t, path,
		codexSessionEntry(1),
		map[string]any{
			"timestamp": "2026-10-15T12:00:01Z", "ordinal": 2, "type": "response_item",
			"payload": map[string]any{"type": "message", "role": "assistant", "content": "linked-task task_control.upsert_task"},
		},
		codexTaskEntry(3, "task_control.upsert_task", map[string]any{"id": "unrelated-task", "endGoal": "private"}),
		codexTaskEntry(4, "task_control.upsert_task", targetTaskArguments("active", "Build the adapter.")),
		codexTaskEntry(5, "task_control.record_milestone", map[string]any{
			"taskId": "linked-task", "kind": "milestone", "title": "Parser proven", "detail": "Exact-task parsing passed.",
		}),
	)

	scan, err := scanCodexRollout(path, "linked-task", nil)
	if err != nil {
		t.Fatal(err)
	}
	if scan.RunID != "thread-1" || len(scan.Events) != 2 {
		t.Fatalf("run=%q events=%d", scan.RunID, len(scan.Events))
	}
	if scan.Events[0].Ordinal != 4 || scan.Events[1].Title != "Parser proven" {
		t.Fatalf("unexpected events: %#v", scan.Events)
	}
	if strings.Contains(scan.Events[0].Detail, "private") {
		t.Fatal("unrelated task content crossed the exact-task boundary")
	}
}

func TestCodexReceiptTreatsCompletionAsUnacceptedObservation(t *testing.T) {
	event := codexObservedEvent{
		Ordinal: 7, ObservedAt: outcomeReceiptNow().UTC(), Kind: "upsert", Title: "Codex says complete",
		Detail: "Worker finished.", Snapshot: codexTaskSnapshot{
			Name: "Linked work", EndGoal: "Finish the linked work", Status: "complete", NextProof: "Verify it",
		},
	}
	receipt, err := codexEventReceipt("codex-task:linked-task", "thread-1", "linked-task", 2, event)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != "waiting_external" || receipt.Acceptance.Status != "partial" || receipt.Effect.ProductionChanged {
		t.Fatalf("completion was overclaimed: %#v", receipt)
	}
	if receipt.SupersedesReceiptRevision == nil || *receipt.SupersedesReceiptRevision != 1 {
		t.Fatalf("bad supersedes revision: %#v", receipt.SupersedesReceiptRevision)
	}
}

func TestObserveCodexDryRunIsOfflineAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(cli.TaskConfigRootEnv, filepath.Join(dir, "state"))
	t.Setenv(receiptTokenEnv, "")
	t.Setenv("MULTICA_SERVER_URL", "")
	path := filepath.Join(dir, "rollout.jsonl")
	writeCodexRolloutFixture(t, path,
		codexSessionEntry(1),
		codexTaskEntry(2, "task_control.upsert_task", targetTaskArguments("active", "Build the adapter.")),
	)

	out, err := executeCodexObserver(t, "SILV-98", "--rollout-file", "rollout.jsonl", "--task-id", "linked-task", "--dry-run", "--output", "json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out, `"status": "preview"`) || !strings.Contains(out, `"provider": "codex"`) {
		t.Fatalf("unexpected preview: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "state")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote state: %v", err)
	}
}

func TestObserveCodexSubmitsLatestThenOnlyNewMeaningfulUpdate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(cli.TaskConfigRootEnv, filepath.Join(dir, "state"))
	t.Setenv(receiptTokenEnv, "mpi_test-secret-value")
	path := filepath.Join(dir, "rollout.jsonl")
	writeCodexRolloutFixture(t, path,
		codexSessionEntry(1),
		codexTaskEntry(2, "task_control.upsert_task", targetTaskArguments("active", "Inspect the importer.")),
		codexTaskEntry(3, "task_control.record_milestone", map[string]any{
			"taskId": "linked-task", "kind": "milestone", "title": "Design fixed", "detail": "The exact link is defined.",
		}),
	)

	var gets atomic.Int32
	var posts atomic.Int32
	var receipts []publicapiv1.OutcomeReceiptV02
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mpi_test-secret-value" {
			t.Errorf("unexpected authorization header")
		}
		switch r.Method {
		case http.MethodGet:
			gets.Add(1)
			w.Header().Set("ETag", `W/"7"`)
			json.NewEncoder(w).Encode(publicapiv1.Issue{ID: "issue-uuid", Identifier: "SILV-98", Revision: 7})
		case http.MethodPost:
			posts.Add(1)
			var receipt publicapiv1.OutcomeReceiptV02
			if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
				t.Error(err)
			}
			receipts = append(receipts, receipt)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(publicapiv1.ExternalReceiptAdmissionResponse{ReceiptID: "receipt", EvidenceID: "evidence"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	args := []string{"SILV-98", "--rollout-file", "rollout.jsonl", "--task-id", "linked-task"}
	if _, err := executeCodexObserver(t, args...); err != nil {
		t.Fatalf("first observe: %v", err)
	}
	if len(receipts) != 1 || receipts[0].ReceiptRevision != 1 || receipts[0].CurrentMilestone != "Design fixed" {
		t.Fatalf("first receipts: %#v", receipts)
	}
	if _, err := executeCodexObserver(t, args...); err != nil {
		t.Fatalf("unchanged observe: %v", err)
	}
	if posts.Load() != 1 || gets.Load() != 1 {
		t.Fatalf("unchanged replay used network: gets=%d posts=%d", gets.Load(), posts.Load())
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := codexTaskEntry(4, "task_control.record_milestone", map[string]any{
		"taskId": "linked-task", "kind": "milestone", "title": "Design fixed", "detail": "The exact link is defined.",
	})
	if err := json.NewEncoder(f).Encode(duplicate); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := executeCodexObserver(t, args...); err != nil {
		t.Fatalf("duplicate semantic update: %v", err)
	}
	if posts.Load() != 1 || gets.Load() != 1 {
		t.Fatalf("duplicate semantic update used network: gets=%d posts=%d", gets.Load(), posts.Load())
	}

	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	entry := codexTaskEntry(5, "task_control.upsert_task", targetTaskArguments("active", "Protected tests passed."))
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := executeCodexObserver(t, args...); err != nil {
		t.Fatalf("second observe: %v", err)
	}
	if len(receipts) != 2 || receipts[1].ReceiptRevision != 2 || receipts[1].CurrentMilestone != "Protected tests passed." {
		t.Fatalf("second receipts: %#v", receipts)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "state", "receipt-observers"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("observer state entries=%d err=%v", len(entries), err)
	}
	checkpoint, err := os.ReadFile(filepath.Join(dir, "state", "receipt-observers", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(checkpoint, []byte("test-secret-value")) {
		t.Fatal("credential leaked into observer state")
	}
}
