package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const workspaceID = "11111111-1111-4111-8111-111111111111"
const projectID = "22222222-2222-4222-8222-222222222222"

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	dir := t.TempDir()
	token := filepath.Join(dir, "token")
	if err := os.WriteFile(token, []byte("test-credential"), 0600); err != nil {
		t.Fatal(err)
	}
	config := Config{URL: server.URL, TokenFile: token, Workspaces: map[string]Workspace{workspaceID: {Projects: map[string]string{projectID: "crm"}, DefaultProject: "company"}}}
	raw, _ := json.Marshal(config)
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestProjectIsolationAndFullEnvelope(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-credential" || r.URL.Query().Get("project") != "crm" {
			t.Error("incorrect identity or project")
		}
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"2026-09-24T00:00:00Z","project":{"id":"crm"},"references":[{"revision":"abc","assured":false}]}`))
	})
	if target, ok := c.Project(workspaceID, projectID); !ok || target != "crm" {
		t.Fatal("canonical mapping missing")
	}
	if _, ok := c.Project(workspaceID, "unregistered"); ok {
		t.Fatal("unknown project used workspace default")
	}
	if _, ok := c.Project("other", projectID); ok {
		t.Fatal("cross-workspace project exposed")
	}
	if got := c.Projects(workspaceID); strings.Join(got, ",") != "company,crm" {
		t.Fatalf("workspace projects = %v", got)
	}
	raw, err := c.Context(context.Background(), "crm", "")
	if err != nil || !strings.Contains(string(raw), `"assured":false`) {
		t.Fatalf("evidence metadata lost: %v", err)
	}
}

func TestHistoryTransportPreservesWorkspaceScopeAndTrustClass(t *testing.T) {
	var searchSeen bool
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-credential" {
			t.Fatal("brain credential missing")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/knowledge/history/fragments":
			if r.Method != http.MethodPost {
				t.Fatalf("history import method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","recorded":{"submitted":1,"inserted":1,"unchanged":0,"recordClass":"conversation_recollection"}}`))
		case "/v2/knowledge/history/search":
			searchSeen = true
			if r.Method != http.MethodPost || r.URL.RawQuery != "" {
				t.Fatalf("private history query leaked into URL or wrong method: %s %q", r.Method, r.URL.RawQuery)
			}
			var request struct {
				Query    string   `json:"query"`
				Limit    int      `json:"limit"`
				Projects []string `json:"projects"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if strings.Join(request.Projects, ",") != "company,crm" || request.Query != "LinkHub 403" || request.Limit != 20 {
				t.Fatalf("history search request = %+v", request)
			}
			_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","history":{"groups":[{"day":"2026-10-15","project":"crm","matches":[{"project":"crm","recordClass":"conversation_recollection","assured":false,"accepted":false,"authority":"none"}]}]}}`))
		default:
			t.Fatalf("unexpected route %s", r.URL.Path)
		}
	})
	if _, err := c.IngestHistory(context.Background(), []byte(`{"fragments":[{}]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SearchHistory(context.Background(), []string{"company", "crm"}, "LinkHub 403", 20); err != nil {
		t.Fatal(err)
	}
	if !searchSeen {
		t.Fatal("history search was not sent")
	}
}

func TestHistoryTransportRejectsTrustOrProjectWidening(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","history":{"groups":[{"day":"2026-10-15","project":"other","matches":[]}]}}`,
		`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","history":{"groups":[{"day":"2026-10-15","project":"crm","matches":[{"project":"crm","recordClass":"accepted_outcome","assured":true,"accepted":true,"authority":"task"}]}]}}`,
	} {
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
		if _, err := c.SearchHistory(context.Background(), []string{"crm"}, "LinkHub 403", 20); err == nil {
			t.Fatalf("unsafe history response accepted: %s", body)
		}
	}
}

func TestRejectMismatchRedirectAndOversize(t *testing.T) {
	for _, test := range []string{"mismatch", "redirect", "oversize", "unauthorized"} {
		t.Run(test, func(t *testing.T) {
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch test {
				case "mismatch":
					_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","project":{"id":"other"}}`))
				case "redirect":
					w.Header().Set("Location", "/elsewhere")
					w.WriteHeader(302)
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat("x", maxResponse+1)))
				case "unauthorized":
					w.WriteHeader(401)
					_, _ = w.Write([]byte("secret-proxy-error"))
				}
			})
			_, err := c.Context(context.Background(), "crm", "")
			if err == nil || strings.Contains(err.Error(), "secret-proxy-error") {
				t.Fatal("unsafe success or error")
			}
			if calls != 1 {
				t.Fatal("redirect was followed")
			}
		})
	}
}

func TestSendRequiresDurableEventReceipt(t *testing.T) {
	for _, recorded := range []string{`null`, `{}`, `{"id":"invalid","status":"task_observation","acceptance":"not_established"}`, `{"id":"00000000-0000-0000-0000-000000000000","status":"task_observation","acceptance":"not_established"}`, `{"id":"33333333-3333-4333-8333-333333333333","status":"queued","acceptance":"not_established"}`, `{"id":"33333333-3333-4333-8333-333333333333","status":"task_observation","acceptance":"accepted"}`} {
		t.Run(recorded, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","recorded":` + recorded + `}`))
			})
			if err := c.Send(context.Background(), []byte(`{"idempotencyKey":"stable"}`)); err == nil || err.Error() != "brain durable event receipt unavailable" {
				t.Fatal("invalid receipt treated as delivered or response leaked")
			}
		})
	}
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","recorded":{"id":"33333333-3333-4333-8333-333333333333","status":"task_observation","acceptance":"not_established"}}`))
	})
	for i := 0; i < 2; i++ {
		if err := c.Send(context.Background(), []byte(`{"idempotencyKey":"stable"}`)); err != nil {
			t.Fatal("valid initial/replay receipt rejected", err)
		}
	}
}
