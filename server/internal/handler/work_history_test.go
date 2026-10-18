package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/brain"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const historyWorkspaceID = "11111111-1111-4111-8111-111111111111"
const historyProjectID = "22222222-2222-4222-8222-222222222222"

func historyHandlerWithBrain(t *testing.T, upstream http.HandlerFunc) (*Handler, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(upstream)
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte("brain-test-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := brain.Config{URL: server.URL, TokenFile: tokenPath, Workspaces: map[string]brain.Workspace{
		historyWorkspaceID: {Projects: map[string]string{historyProjectID: "linkhub"}, DefaultProject: "company"},
	}}
	raw, _ := json.Marshal(config)
	configPath := filepath.Join(dir, "brain.json")
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := brain.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{SharedBrain: &brain.Service{Client: client}}, server
}

func scopedHistoryRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	raw, _ := json.Marshal(body)
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	request = request.WithContext(middleware.SetMemberContext(request.Context(), historyWorkspaceID, db.Member{}))
	return request
}

func TestWorkHistoryHandlerMapsProjectBeforeBrainImport(t *testing.T) {
	text := "LinkHub 403 [REDACTED:EMAIL]"
	hash := sha256.Sum256([]byte(text))
	handler, server := historyHandlerWithBrain(t, func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Fragments []struct {
				Project string `json:"project"`
			} `json:"fragments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Fragments) != 1 || request.Fragments[0].Project != "linkhub" {
			t.Fatalf("brain project was not server-mapped: %+v", request)
		}
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","recorded":{"submitted":1,"inserted":1,"unchanged":0,"recordClass":"conversation_recollection"}}`))
	})
	defer server.Close()
	body := map[string]any{"project_id": historyProjectID, "fragments": []map[string]any{{
		"provider": "codex", "session_id": "session", "project": historyProjectID,
		"occurred_at": "2026-10-15T13:00:00Z", "role": "user", "text": text,
		"text_sha256": hex.EncodeToString(hash[:]), "source_reference": "work-history://codex/session/session/record/1?offset=0&part=0",
		"source_ordinal": 1, "source_offset": 0, "evidence_class": "conversation_recollection",
		"redaction_version": 1, "importer_version": 1,
	}}}
	response := httptest.NewRecorder()
	handler.ImportWorkHistory(response, scopedHistoryRequest(t, http.MethodPost, "/api/work-history/fragments", body))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestWorkHistoryHandlerSearchesOnlyWorkspaceProjects(t *testing.T) {
	handler, server := historyHandlerWithBrain(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RawQuery != "" {
			t.Fatalf("private query leaked into URL or wrong method: %s %q", r.Method, r.URL.RawQuery)
		}
		var request struct {
			Query    string   `json:"query"`
			Projects []string `json:"projects"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Projects) != 2 || request.Projects[0] != "company" || request.Projects[1] != "linkhub" || request.Query != "LinkHub 403" {
			t.Fatalf("workspace history search = %+v", request)
		}
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","history":{"groups":[]}}`))
	})
	defer server.Close()
	response := httptest.NewRecorder()
	handler.SearchWorkHistory(response, scopedHistoryRequest(t, http.MethodPost, "/api/work-history/search", map[string]any{"query": "LinkHub 403"}))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
