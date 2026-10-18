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

func executeReceiptStreamObserver(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "multica", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("server-url", "", "")
	root.PersistentFlags().String("profile", "", "")
	issue := &cobra.Command{Use: "issue"}
	issue.AddCommand(newIssueReceiptCommand())
	root.AddCommand(issue)
	root.SetArgs(append([]string{"issue", "receipt", "observe-stream"}, args...))
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(context.Background())
	return stdout.String(), err
}

func appendReceiptStream(t *testing.T, path string, receipt publicapiv1.OutcomeReceiptV02, newline bool) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	if newline {
		data = append(data, '\n')
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func streamReceipt(t *testing.T, provider, workID string, revision int64) publicapiv1.OutcomeReceiptV02 {
	t.Helper()
	receipt := validOutcomeReceipt(t)
	receipt.Provider = provider
	receipt.WorkID = workID
	receipt.ReceiptRevision = revision
	receipt.CurrentMilestone = "Milestone " + string(rune('0'+revision))
	if revision > 1 {
		previous := revision - 1
		receipt.SupersedesReceiptRevision = &previous
	}
	return receipt
}

func TestObserveReceiptStreamDryRunIsOfflineAndExact(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(cli.TaskConfigRootEnv, filepath.Join(dir, "state"))
	t.Setenv(receiptTokenEnv, "")
	t.Setenv("MULTICA_SERVER_URL", "")
	path := filepath.Join(dir, "claude-receipts.jsonl")
	appendReceiptStream(t, path, streamReceipt(t, "claude", "claude-task:42", 1), true)

	out, err := executeReceiptStreamObserver(t, "SILV-98", "--stream-file", filepath.Base(path), "--provider", "claude", "--work-id", "claude-task:42", "--dry-run", "--output", "json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out, `"status": "preview"`) || !strings.Contains(out, `"provider": "claude"`) || !strings.Contains(out, `"processed": 1`) {
		t.Fatalf("unexpected preview: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "state")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote state: %v", err)
	}

	_, err = executeReceiptStreamObserver(t, "SILV-98", "--stream-file", filepath.Base(path), "--provider", "chatgpt", "--work-id", "claude-task:42", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), "exact provider or work-ID") {
		t.Fatalf("provider mismatch error = %v", err)
	}
}

func TestObserveReceiptStreamCheckpointsOnlyCompleteCommittedLines(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(cli.TaskConfigRootEnv, filepath.Join(dir, "state"))
	t.Setenv(receiptTokenEnv, "mpi_test-secret-value")
	path := filepath.Join(dir, "outside-receipts.jsonl")
	appendReceiptStream(t, path, streamReceipt(t, "copilot", "copilot-task:7", 1), true)

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

	args := []string{"SILV-98", "--stream-file", filepath.Base(path), "--provider", "copilot", "--work-id", "copilot-task:7"}
	if _, err := executeReceiptStreamObserver(t, args...); err != nil {
		t.Fatalf("first observe: %v", err)
	}
	if len(receipts) != 1 || receipts[0].ReceiptRevision != 1 {
		t.Fatalf("first receipts: %#v", receipts)
	}
	if _, err := executeReceiptStreamObserver(t, args...); err != nil {
		t.Fatalf("unchanged observe: %v", err)
	}
	if posts.Load() != 1 || gets.Load() != 1 {
		t.Fatalf("unchanged observer used network: gets=%d posts=%d", gets.Load(), posts.Load())
	}

	second := streamReceipt(t, "copilot", "copilot-task:7", 2)
	appendReceiptStream(t, path, second, false)
	if _, err := executeReceiptStreamObserver(t, args...); err != nil {
		t.Fatalf("partial-line observe: %v", err)
	}
	if posts.Load() != 1 {
		t.Fatalf("partial line was submitted: posts=%d", posts.Load())
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{'\n'}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if _, err := executeReceiptStreamObserver(t, args...); err != nil {
		t.Fatalf("committed second observe: %v", err)
	}
	if len(receipts) != 2 || receipts[1].ReceiptRevision != 2 {
		t.Fatalf("all receipts: %#v", receipts)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "state", "receipt-streams"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("stream state entries=%d err=%v", len(entries), err)
	}
	checkpoint, err := os.ReadFile(filepath.Join(dir, "state", "receipt-streams", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(checkpoint, []byte("test-secret-value")) {
		t.Fatal("credential leaked into receipt stream checkpoint")
	}
}

func TestScanReceiptStreamRejectsRevisionGapsAndCommittedGarbage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stream.jsonl")
	appendReceiptStream(t, path, streamReceipt(t, "chatgpt", "chatgpt-task:1", 2), true)
	if _, err := scanReceiptStream(path, 0, "chatgpt", "chatgpt-task:1", 0); err == nil || !strings.Contains(err.Error(), "expected 1") {
		t.Fatalf("revision gap error = %v", err)
	}
	if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := scanReceiptStream(path, 0, "chatgpt", "chatgpt-task:1", 0); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("garbage error = %v", err)
	}
}
