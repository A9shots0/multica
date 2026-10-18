package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/brain"
	"github.com/multica-ai/multica/server/internal/externalreceipt"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

func TestContextRequestIsLinkedProjectScopedAndFieldBounded(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	issueID, issueRevision := seedExternalReceiptIssueWithoutScope(t, "Bounded outside context")
	projectID := dbfx.Project(t, "Bounded outside context project")
	dbfx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, projectID, issueID)
	issue, err := testHandler.Queries.GetIssueInWorkspace(context.Background(), db.GetIssueInWorkspaceParams{
		WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(issueID),
	})
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	brainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("project") != "hq-project" || r.URL.Query().Get("q") != "Exact company policy" {
			t.Fatalf("request escaped canonical routing: %s", r.URL.String())
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"schemaVersion": "silvermediums.brain.v2", "servedAt": "2026-10-15T12:00:00Z",
			"project":                   map[string]any{"id": "hq-project"},
			"references":                []any{map[string]any{"id": "doc-1", "revision": "rev-1"}},
			"unrequested_private_field": "must not leave the broker",
		})
	}))
	t.Cleanup(brainServer.Close)
	dir := t.TempDir()
	tokenPath, configPath := filepath.Join(dir, "token"), filepath.Join(dir, "config.json")
	if err := os.WriteFile(tokenPath, []byte("fixture-only"), 0600); err != nil {
		t.Fatal(err)
	}
	configRaw, _ := json.Marshal(brain.Config{
		URL: brainServer.URL, TokenFile: tokenPath,
		Workspaces: map[string]brain.Workspace{testWorkspaceID: {Projects: map[string]string{uuidToString(issue.ProjectID): "hq-project"}}},
	})
	if err := os.WriteFile(configPath, configRaw, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := brain.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	priorBrain := testHandler.SharedBrain
	testHandler.SharedBrain = &brain.Service{Client: client, Store: testHandler.Queries}
	t.Cleanup(func() { testHandler.SharedBrain = priorBrain })

	installationID := installPluginForAction(t, []string{"issues:read"})
	request := publicapiv1.ContextRequestV02{
		RequestID: "ctx-linked-1", WorkID: "phonefarm-modernization", RequestType: "policy",
		Need: "Exact company policy", Reason: "Avoid loading broad company history", Scope: "this issue",
		FreshnessRequired: "current", MaximumPayload: "compact",
		ResponseContract:      publicapiv1.ContextResponseContract{RequiredFields: []string{"references", "observed_at"}, EvidenceLevel: "observed"},
		AuthorizationBoundary: "read_only", CachePolicy: "never",
	}

	// A guessed work id cannot cause a Brain read, even for an issue the caller
	// is otherwise allowed to view.
	unlinked := httptest.NewRecorder()
	testHandler.CreatePluginContextRequest(unlinked, pluginActionRequest(http.MethodPost, "/v1/issues/"+issueID+"/context-requests", installationID, request, map[string]string{"issue_ref": issueID}))
	if unlinked.Code != http.StatusOK {
		t.Fatalf("unlinked status=%d body=%s", unlinked.Code, unlinked.Body.String())
	}
	var denied publicapiv1.ContextResponseV02
	if json.Unmarshal(unlinked.Body.Bytes(), &denied) != nil || denied.Status != "unauthorized" || calls != 0 {
		t.Fatalf("unlinked work reached Brain: response=%s calls=%d", unlinked.Body.String(), calls)
	}

	receipt := externalReceiptForCase(workforceBenchmarkCases[0])
	if _, err := testHandler.ExternalReceiptService.Admit(context.Background(), externalreceipt.Request{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), IssueRevision: issueRevision,
		IdempotencyKey: "context-broker-linked-work",
		Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
		Receipt:        receipt,
	}); err != nil {
		t.Fatalf("admit linked work: %v", err)
	}

	foundRecorder := httptest.NewRecorder()
	testHandler.CreatePluginContextRequest(foundRecorder, pluginActionRequest(http.MethodPost, "/v1/issues/"+issueID+"/context-requests", installationID, request, map[string]string{"issue_ref": issueID}))
	if foundRecorder.Code != http.StatusOK {
		t.Fatalf("found status=%d body=%s", foundRecorder.Code, foundRecorder.Body.String())
	}
	var found publicapiv1.ContextResponseV02
	if err := json.Unmarshal(foundRecorder.Body.Bytes(), &found); err != nil {
		t.Fatal(err)
	}
	if found.Status != "found" || found.Source.System != "brain" || found.Source.Reference == nil || calls != 1 {
		t.Fatalf("bounded request not resolved: response=%s calls=%d", foundRecorder.Body.String(), calls)
	}
	if _, ok := found.Payload["references"]; !ok || found.Payload["observed_at"] != "2026-10-15T12:00:00Z" {
		t.Fatalf("requested fields missing: %#v", found.Payload)
	}
	if _, leaked := found.Payload["unrequested_private_field"]; leaked {
		t.Fatalf("unrequested Brain field leaked: %#v", found.Payload)
	}
}
