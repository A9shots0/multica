package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

func validOutcomeReceipt(t *testing.T) publicapiv1.OutcomeReceiptV02 {
	t.Helper()
	return publicapiv1.OutcomeReceiptV02{
		WorkID:          "external-work-1",
		RunID:           "run-1",
		ReceiptRevision: 1,
		Provider:        "codex",
		Objective:       "Verify the importer",
		Scope:           "One explicit test receipt",
		RequestedMode:   "review",
		Authority: publicapiv1.OutcomeReceiptAuthority{
			Reference:      "owner-request-1",
			AllowedEffects: []string{},
		},
		State:                  "working",
		CurrentMilestone:       "Importer exercised",
		ChangeSinceLastReceipt: "First receipt",
		Evidence: []publicapiv1.OutcomeReceiptEvidence{
			{Type: "test", Reference: "cmd_issue_receipt_test"},
		},
		Dependencies: []string{},
		NextAction:   "Record the result",
		OwnerDecision: publicapiv1.OutcomeReceiptOwnerDecision{
			Required:      false,
			Options:       []string{},
			InvalidatedBy: []string{},
		},
		Risk: "low",
		ResourceUsage: publicapiv1.OutcomeReceiptResourceUsage{
			Source: "unavailable",
		},
		Effect: publicapiv1.OutcomeReceiptEffect{
			UserOutcome:       "unchanged",
			ProductionChanged: false,
		},
		Freshness: publicapiv1.OutcomeReceiptFreshness{
			ObservedAt: "2026-10-14T12:00:00Z",
		},
		Acceptance: publicapiv1.OutcomeReceiptAcceptance{
			Stage:             "local",
			Status:            "partial",
			CriteriaRemaining: []string{"Protected CI"},
		},
	}
}

func writeReceiptFixture(t *testing.T, dir string, receipt publicapiv1.OutcomeReceiptV02) string {
	t.Helper()
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func executeReceiptImport(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "multica", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("server-url", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(newIssueReceiptCommand())
	root.SetArgs(append([]string{"receipt", "import"}, args...))
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(context.Background())
	return stdout.String(), err
}

func prepareReceiptTest(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(cli.TaskConfigRootEnv, filepath.Join(dir, "state"))
	t.Setenv(receiptTokenEnv, "mpi_test-secret-value")
	path := writeReceiptFixture(t, dir, validOutcomeReceipt(t))
	return dir, filepath.Base(path)
}

func TestReceiptImportDryRunIsOffline(t *testing.T) {
	_, path := prepareReceiptTest(t)
	t.Setenv(receiptTokenEnv, "")
	t.Setenv("MULTICA_SERVER_URL", "")

	out, err := executeReceiptImport(t, "SILV-42", "--file", path, "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out, "No data was sent") || !strings.Contains(out, "SHA-256:") {
		t.Fatalf("unexpected dry-run output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv(cli.TaskConfigRootEnv), "receipt-submissions")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run wrote retry state: %v", err)
	}
}

func TestReceiptImportRejectsMalformedAndOversizedFilesOffline(t *testing.T) {
	dir, path := prepareReceiptTest(t)

	if err := os.WriteFile(filepath.Join(dir, path), []byte(`{"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := executeReceiptImport(t, "SILV-42", "--file", path, "--dry-run")
	if err == nil || cli.ExitCodeFor(err) != cli.ExitValidation {
		t.Fatalf("malformed file error = %v, exit = %d", err, cli.ExitCodeFor(err))
	}

	if err := os.WriteFile(filepath.Join(dir, path), bytes.Repeat([]byte("x"), maxOutcomeReceiptFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = executeReceiptImport(t, "SILV-42", "--file", path, "--dry-run")
	if err == nil || cli.ExitCodeFor(err) != cli.ExitValidation || !strings.Contains(cli.FormatError(err, false), "1 MiB") {
		t.Fatalf("oversized file error = %v, formatted = %q", err, cli.FormatError(err, false))
	}
}

func TestReceiptImportSubmitsOnceAndUsesLocalAcceptedRecord(t *testing.T) {
	_, path := prepareReceiptTest(t)
	var gets atomic.Int32
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mpi_test-secret-value" {
			t.Errorf("unexpected authorization header")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/issues/SILV-42":
			gets.Add(1)
			w.Header().Set("ETag", `W/"7"`)
			json.NewEncoder(w).Encode(publicapiv1.Issue{ID: "issue-uuid", Identifier: "SILV-42", Revision: 7})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/issues/issue-uuid/external-receipts":
			posts.Add(1)
			if r.Header.Get("If-Match") != `W/"7"` || !strings.HasPrefix(r.Header.Get(publicapiv1.HeaderIdempotencyKey), "outcome-receipt-v02-") {
				t.Errorf("missing concurrency or idempotency headers")
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(publicapiv1.ExternalReceiptAdmissionResponse{ReceiptID: "receipt-1", EvidenceID: "evidence-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	out, err := executeReceiptImport(t, "SILV-42", "--file", path, "--output", "json")
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if !strings.Contains(out, `"status": "accepted"`) {
		t.Fatalf("unexpected first output: %s", out)
	}
	out, err = executeReceiptImport(t, "SILV-42", "--file", path, "--output", "json")
	if err != nil {
		t.Fatalf("repeat import: %v", err)
	}
	if !strings.Contains(out, `"status": "already_accepted"`) || gets.Load() != 1 || posts.Load() != 1 {
		t.Fatalf("repeat did not stay local: output=%s gets=%d posts=%d", out, gets.Load(), posts.Load())
	}

	stateRoot := os.Getenv(cli.TaskConfigRootEnv)
	entries, err := os.ReadDir(filepath.Join(stateRoot, "receipt-submissions"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("retry state entries = %d, err=%v", len(entries), err)
	}
	state, err := os.ReadFile(filepath.Join(stateRoot, "receipt-submissions", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(state, []byte("test-secret-value")) {
		t.Fatal("credential leaked into durable retry state")
	}
}

func TestReceiptImportRetriesPreparedSubmissionWithoutRefetching(t *testing.T) {
	_, path := prepareReceiptTest(t)
	serverCalls := atomic.Int32{}
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("interrupted retry unexpectedly refetched the issue: %s", r.Method)
		}
		gotKey = r.Header.Get(publicapiv1.HeaderIdempotencyKey)
		json.NewEncoder(w).Encode(publicapiv1.ExternalReceiptAdmissionResponse{ReceiptID: "receipt-replayed", Replay: true})
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	_, digest, err := readOutcomeReceiptFile(path)
	if err != nil {
		t.Fatal(err)
	}
	serverIdentity, _ := safeReceiptServerIdentity(srv.URL)
	recordPath, err := receiptSubmissionRecordPath("", serverIdentity, "SILV-42", digest)
	if err != nil {
		t.Fatal(err)
	}
	record := &receiptSubmissionRecord{
		Version: receiptSubmissionStateV1, Server: serverIdentity, IssueRef: "SILV-42", IssueID: "issue-uuid",
		ReceiptSHA256: digest, IdempotencyKey: receiptIdempotencyKey("issue-uuid", digest), IfMatch: `W/"7"`,
		Status: "prepared", PreparedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := saveReceiptSubmissionRecord(recordPath, record); err != nil {
		t.Fatal(err)
	}

	if _, err := executeReceiptImport(t, "SILV-42", "--file", path); err != nil {
		t.Fatalf("retry import: %v", err)
	}
	if serverCalls.Load() != 1 || gotKey != record.IdempotencyKey {
		t.Fatalf("calls=%d key=%q want=%q", serverCalls.Load(), gotKey, record.IdempotencyKey)
	}
}

func TestReceiptImportPreservesPreparedRecordOnStaleRevision(t *testing.T) {
	_, path := prepareReceiptTest(t)
	var gets atomic.Int32
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets.Add(1)
			w.Header().Set("ETag", `W/"7"`)
			json.NewEncoder(w).Encode(publicapiv1.Issue{ID: "issue-uuid", Revision: 7})
			return
		}
		posts.Add(1)
		w.WriteHeader(http.StatusPreconditionFailed)
		io.WriteString(w, `{"error":"issue_revision_conflict"}`)
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	for attempt := 0; attempt < 2; attempt++ {
		_, err := executeReceiptImport(t, "SILV-42", "--file", path)
		if err == nil {
			t.Fatal("expected stale revision failure")
		}
	}
	if gets.Load() != 1 || posts.Load() != 2 {
		t.Fatalf("stale retry should reuse the prepared request: gets=%d posts=%d", gets.Load(), posts.Load())
	}
}

func TestReceiptImportUnavailableAndAuthErrorsDoNotLeakCredential(t *testing.T) {
	_, path := prepareReceiptTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"bad token"}`)
	}))
	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	_, err := executeReceiptImport(t, "SILV-42", "--file", path)
	if err == nil || cli.ExitCodeFor(err) != cli.ExitAuth {
		t.Fatalf("auth error=%v exit=%d", err, cli.ExitCodeFor(err))
	}
	if strings.Contains(cli.FormatError(err, true), "test-secret-value") {
		t.Fatal("credential leaked into debug error output")
	}
	srv.Close()

	_, err = executeReceiptImport(t, "OTHER-1", "--file", path)
	if err == nil || cli.ExitCodeFor(err) != cli.ExitNetwork {
		t.Fatalf("unavailable error=%v exit=%d", err, cli.ExitCodeFor(err))
	}
	var netErr *cli.NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("expected network error, got %T", err)
	}
}
