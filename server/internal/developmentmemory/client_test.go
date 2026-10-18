package developmentmemory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrepareTaskRegistersThenReturnsBoundedContext(t *testing.T) {
	var contextReads, workstreamWrites int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/development/context":
			contextReads++
			if got := r.Header.Get("Authorization"); got != "Bearer reader-secret" {
				t.Fatalf("reader authorization = %q", got)
			}
			if got := r.Header.Get("X-SMI-Reader"); got != "multica" {
				t.Fatalf("reader identity = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			if contextReads == 1 {
				_, _ = w.Write([]byte(`{"schema_version":1,"workstreams":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"schema_version":1,"workstreams":[{"id":"multica-issue-issue-1","repositories":[{"canonical_identity":"a9shots0/smcrm"}],"current_memory":[{"kind":"decision","statement":"CRM owns the operator UI."}]}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/development/workstreams":
			workstreamWrites++
			assertWriter(t, r)
			var payload struct {
				Repositories []struct {
					Identity      string  `json:"canonical_identity"`
					DefaultBranch *string `json:"default_branch"`
				} `json:"repositories"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Repositories) != 1 || payload.Repositories[0].Identity != "a9shots0/smcrm" {
				t.Fatalf("repositories = %#v", payload.Repositories)
			}
			if payload.Repositories[0].DefaultBranch != nil {
				t.Fatalf("unknown default branch was guessed as %q", *payload.Repositories[0].DefaultBranch)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"created"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL: server.URL, WriterToken: "writer-secret", ReaderToken: "reader-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := client.PrepareContext(context.Background(), Task{
		IssueID: "issue-1", IssueIdentifier: "MUL-1",
		Title: "Keep frontend and backend aligned", Goal: "Preserve accepted decisions across both repositories.",
		ProjectID: "project-1", ProjectName: "Viral Queue", WorkspaceID: "workspace-1",
		Repositories: []Repository{{URL: "https://github.com/A9shots0/SMCrm.git"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if contextReads != 2 || workstreamWrites != 1 {
		t.Fatalf("reads=%d workstreams=%d", contextReads, workstreamWrites)
	}
	if !strings.Contains(string(packet), "CRM owns the operator UI") {
		t.Fatalf("packet = %s", packet)
	}
	if strings.Contains(string(packet), "secret") {
		t.Fatalf("packet contains a credential: %s", packet)
	}
}

func TestDeliverWritesApprovedMemoryInReplaySafeOrder(t *testing.T) {
	paths := []string{}
	terminalStatus := ""
	memoryScope := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertWriter(t, r)
		paths = append(paths, r.URL.Path)
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["producer"] != "multica" {
			t.Fatalf("producer = %#v", payload["producer"])
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			terminalStatus, _ = payload["status"].(string)
		}
		if r.URL.Path == "/v1/development/memory-items" {
			memoryScope, _ = payload["scope"].(string)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, WriterToken: "writer-secret", ReaderToken: "reader-secret"})
	if err != nil {
		t.Fatal(err)
	}
	repo := "a9shots0/smcrm"
	err = client.Deliver(context.Background(), Delivery{
		ProposalID: "proposal-1", TaskID: "01a00000-0000-7000-8000-000000000001",
		WorkstreamID: "multica-issue-issue-1", AgentName: "Codex",
		TaskStartedAt:   time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		TaskCompletedAt: time.Date(2026, 10, 1, 12, 5, 0, 0, time.UTC),
		TaskStatus:      "complete",
		Repositories:    []BoundRepository{{CanonicalIdentity: repo}},
		Payload: ProposalDraft{
			Summary: "Implemented the accepted contract.", NextAction: "Run owner acceptance.",
			RemainingUncertainty: []string{"Owner acceptance remains open."},
			MemoryItems:          []MemoryItemDraft{{RepositoryIdentity: &repo, Scope: "project", Kind: "decision", Statement: "CRM owns the UI.", EvidenceRefs: []string{"commit:abc"}}},
			RepositoryEvidence:   []RepositoryEvidence{{CanonicalIdentity: repo, EvidenceRefs: []string{"commit:abc"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/v1/development/task-sessions",
		"/v1/development/memory-items",
		"/v1/development/checkpoints",
		"/v1/development/task-sessions/multica-task-01a00000-0000-7000-8000-000000000001/complete",
	}
	if strings.Join(paths, "|") != strings.Join(want, "|") {
		t.Fatalf("paths = %#v", paths)
	}
	if terminalStatus != "complete" {
		t.Fatalf("terminal status = %q", terminalStatus)
	}
	if memoryScope != "project" {
		t.Fatalf("memory scope = %q", memoryScope)
	}
}

func TestDeliverPreservesFailedTerminalStatus(t *testing.T) {
	terminalStatus := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			terminalStatus, _ = payload["status"].(string)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, WriterToken: "writer", ReaderToken: "reader"})
	if err != nil {
		t.Fatal(err)
	}
	err = client.Deliver(context.Background(), Delivery{
		ProposalID: "proposal-2", TaskID: "01a00000-0000-7000-8000-000000000002",
		WorkstreamID: "multica-issue-issue-2", AgentName: "Claude", TaskStatus: "failed",
		TaskStartedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), TaskCompletedAt: time.Date(2026, 10, 1, 12, 1, 0, 0, time.UTC),
		Repositories: []BoundRepository{{CanonicalIdentity: "a9shots0/viralcontentpuller"}},
		Payload: ProposalDraft{Summary: "The frontend direction failed acceptance.", NextAction: "Return to the accepted system map.",
			RemainingUncertainty: []string{}, MemoryItems: []MemoryItemDraft{}, RepositoryEvidence: []RepositoryEvidence{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if terminalStatus != "failed" {
		t.Fatalf("terminal status = %q", terminalStatus)
	}
}

func TestPrepareTaskRefusesRepositoryDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version":1,"workstreams":[{"id":"multica-issue-issue-1","repositories":[{"canonical_identity":"a9shots0/smcrm"}]}]}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, WriterToken: "writer", ReaderToken: "reader"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.PrepareContext(context.Background(), Task{
		IssueID: "issue-1", Repositories: []Repository{{URL: "https://github.com/A9shots0/viralcontentpuller.git"}},
	})
	if err == nil || !strings.Contains(err.Error(), "missing repository a9shots0/viralcontentpuller") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewRejectsCredentialedOrPublicPlaintextURL(t *testing.T) {
	for _, raw := range []string{
		"http://10.0.0.4:8080",
		"https://user:password@example.com",
		"ftp://localhost:8080",
	} {
		if _, err := New(Config{BaseURL: raw, WriterToken: "writer", ReaderToken: "reader"}); err == nil {
			t.Fatalf("New(%q) succeeded", raw)
		}
	}
}

func TestNewAllowsLocalDockerHostPlaintextURL(t *testing.T) {
	if _, err := New(Config{
		BaseURL: "http://host.docker.internal:8090", WriterToken: "writer", ReaderToken: "reader",
	}); err != nil {
		t.Fatalf("local Docker host URL rejected: %v", err)
	}
}

func TestFailClosedPreparerAlwaysRefusesDispatch(t *testing.T) {
	_, err := FailClosed(nil).PrepareContext(context.Background(), Task{IssueID: "issue-1"})
	if err == nil {
		t.Fatal("fail-closed preparer allowed dispatch")
	}
}

func TestBoundedTextKeepsContractLimitVisible(t *testing.T) {
	got := boundedText(strings.Repeat("é", 6), 5)
	if got != "éééé…" {
		t.Fatalf("bounded text = %q", got)
	}
}

func TestNormalizeRepositoriesConvertsSCPRemoteToCanonicalSSHURL(t *testing.T) {
	repositories, err := normalizeRepositories([]Repository{{URL: "git@github.com:A9shots0/SMCrm.git"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].CanonicalIdentity != "a9shots0/smcrm" {
		t.Fatalf("repositories = %#v", repositories)
	}
	if repositories[0].RemoteURL != "ssh://git@github.com/A9shots0/SMCrm.git" {
		t.Fatalf("remote URL = %q", repositories[0].RemoteURL)
	}
}

func assertWriter(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer writer-secret" {
		t.Fatalf("writer authorization = %q", got)
	}
}
