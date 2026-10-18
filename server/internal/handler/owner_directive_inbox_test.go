package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestOwnerDirectiveClosesExactInboxRequestAtomically(t *testing.T) {
	issueID := dbfx.Issue(t, "Owner direction closes Inbox request")
	inboxID := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID,
		"issue_id": issueID, "type": "workflow_owner_decision", "severity": "action_required",
		"title": "Choose how work continues", "body": "Pick one option",
	})
	var revision int64
	if err := testPool.QueryRow(t.Context(), `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision); err != nil {
		t.Fatal(err)
	}

	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/owner-directives", ownerDirectiveRequest{
		InboxItemID: inboxID, ExpectedIssueRevision: revision, Directive: "correct", Content: "Use the reviewed prevention plan.",
	}), "id", issueID)
	w := httptest.NewRecorder()
	testHandler.PostIssueOwnerDirective(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var receipt map[string]any
	if err := json.NewDecoder(w.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["inbox_item_id"] != inboxID {
		t.Fatalf("receipt lost Inbox binding: %#v", receipt)
	}
	var read, archived bool
	if err := testPool.QueryRow(t.Context(), `SELECT read, archived FROM inbox_item WHERE id=$1`, inboxID).Scan(&read, &archived); err != nil {
		t.Fatal(err)
	}
	if !read || !archived {
		t.Fatalf("answered Inbox request remained actionable: read=%v archived=%v", read, archived)
	}
}

func TestOwnerDirectiveRejectsUnrelatedInboxRequestWithoutWriting(t *testing.T) {
	issueID := dbfx.Issue(t, "Directed issue")
	otherIssueID := dbfx.Issue(t, "Unrelated Inbox issue")
	inboxID := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID,
		"issue_id": otherIssueID, "type": "workflow_owner_decision", "severity": "action_required",
		"title": "Unrelated choice", "body": "Do not consume me",
	})
	var revision int64
	if err := testPool.QueryRow(t.Context(), `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision); err != nil {
		t.Fatal(err)
	}

	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/owner-directives", ownerDirectiveRequest{
		InboxItemID: inboxID, ExpectedIssueRevision: revision, Directive: "correct", Content: "This must not be recorded.",
	}), "id", issueID)
	w := httptest.NewRecorder()
	testHandler.PostIssueOwnerDirective(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var archived bool
	if err := testPool.QueryRow(t.Context(), `SELECT archived FROM inbox_item WHERE id=$1`, inboxID).Scan(&archived); err != nil {
		t.Fatal(err)
	}
	if archived {
		t.Fatal("unrelated Inbox request was consumed")
	}
	var comments int
	if err := testPool.QueryRow(t.Context(), `SELECT count(*) FROM comment WHERE issue_id=$1 AND content LIKE '%This must not be recorded.%'`, issueID).Scan(&comments); err != nil {
		t.Fatal(err)
	}
	if comments != 0 {
		t.Fatalf("failed atomic close left %d directive comments", comments)
	}
}
