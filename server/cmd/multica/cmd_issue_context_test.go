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
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

func validContextRequest() publicapiv1.ContextRequestV02 {
	return publicapiv1.ContextRequestV02{
		RequestID: "ctx-1", WorkID: "work-1", RequestType: "source", Need: "one exact source fact",
		Reason: "avoid broad preload", Scope: "the linked issue", FreshnessRequired: "current", MaximumPayload: "compact",
		ResponseContract:      publicapiv1.ContextResponseContract{RequiredFields: []string{"references"}, EvidenceLevel: "observed"},
		AuthorizationBoundary: "read_only", CachePolicy: "never",
	}
}

func executeContextRequest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "multica", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("server-url", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(newIssueContextCommand())
	root.SetArgs(append([]string{"context", "request"}, args...))
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(context.Background())
	return stdout.String(), err
}

func writeContextRequestFixture(t *testing.T) string {
	t.Helper()
	raw, _ := json.Marshal(validContextRequest())
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestContextRequestDryRunNeedsNoCredentialOrNetwork(t *testing.T) {
	path := writeContextRequestFixture(t)
	t.Chdir(filepath.Dir(path))
	t.Setenv(contextTokenEnv, "")
	t.Setenv(receiptTokenEnv, "")
	t.Setenv("MULTICA_SERVER_URL", "")
	out, err := executeContextRequest(t, "SILV-42", "--file", filepath.Base(path), "--dry-run")
	if err != nil || !strings.Contains(out, `"status": "valid"`) || !strings.Contains(out, `"sent": false`) {
		t.Fatalf("dry-run output=%q err=%v", out, err)
	}
}

func TestContextRequestSendsExactPacketAndPrintsBoundedResponse(t *testing.T) {
	path := writeContextRequestFixture(t)
	t.Chdir(filepath.Dir(path))
	t.Setenv(cli.TaskConfigRootEnv, filepath.Join(filepath.Dir(path), "state"))
	t.Setenv(contextTokenEnv, "mpi_context-fixture")
	t.Setenv(receiptTokenEnv, "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/issues/SILV-42/context-requests" || r.Header.Get("Authorization") != "Bearer mpi_context-fixture" {
			t.Fatalf("unexpected request: %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		var got publicapiv1.ContextRequestV02
		if json.NewDecoder(r.Body).Decode(&got) != nil || got.RequestID != "ctx-1" || got.WorkID != "work-1" {
			t.Fatalf("wrong context request: %#v", got)
		}
		reference := "brain://company/context/sha256/abc"
		json.NewEncoder(w).Encode(publicapiv1.ContextResponseV02{
			RequestID: got.RequestID, WorkID: got.WorkID, Status: "found",
			Source:        publicapiv1.ContextResponseSource{System: "brain", Reference: &reference},
			EvidenceLevel: "observed", ObservedAt: "2026-10-15T12:00:00Z",
			Payload: map[string]any{"references": []any{map[string]any{"id": "doc-1"}}}, Limitations: []string{},
		})
	}))
	defer server.Close()
	t.Setenv("MULTICA_SERVER_URL", server.URL)
	out, err := executeContextRequest(t, "SILV-42", "--file", filepath.Base(path))
	if err != nil || !strings.Contains(out, `"status": "found"`) || !strings.Contains(out, `"doc-1"`) {
		t.Fatalf("context output=%q err=%v", out, err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv(cli.TaskConfigRootEnv), "context-requests")); err == nil {
		t.Fatal("read-only context command unexpectedly wrote durable state")
	}
}
