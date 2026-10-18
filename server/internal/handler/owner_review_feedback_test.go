package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestValidReviewMarks(t *testing.T) {
	valid := []reviewMark{{ID: 1, Shot: "source", X: .1, Y: .2, W: .3, H: .4, Note: "Remove this"}}
	if !validReviewMarks(valid, "source") {
		t.Fatal("expected normalized, noted mark to be valid")
	}
	for name, marks := range map[string][]reviewMark{
		"missing note":  {{ID: 1, Shot: "source", W: .2, H: .2}},
		"wrong source":  {{ID: 1, Shot: "other", W: .2, H: .2, Note: "No"}},
		"outside image": {{ID: 1, Shot: "source", X: .9, W: .2, H: .2, Note: "No"}},
		"duplicate ids": {
			{ID: 1, Shot: "source", W: .2, H: .2, Note: "One"},
			{ID: 1, Shot: "source", X: .3, W: .2, H: .2, Note: "Two"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if validReviewMarks(marks, "source") {
				t.Fatal("expected marks to be rejected")
			}
		})
	}
}

func TestCreateOwnerReviewFeedbackPersistsExactEvidence(t *testing.T) {
	issueID := dbfx.Issue(t, "Owner visual review")
	inboxID := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID,
		"issue_id": issueID, "type": "workflow_owner_decision", "severity": "action_required",
		"title": "Review the design", "body": "Is this ready?",
	})
	sourceID := dbfx.Insert(t, "attachment", testutil.Cols{
		"workspace_id": testWorkspaceID, "issue_id": issueID,
		"uploader_type": "member", "uploader_id": testUserID,
		"filename": "desktop.png", "url": "memory://desktop.png",
		"content_type": "image/png", "size_bytes": 100,
	})
	markedID := dbfx.Insert(t, "attachment", testutil.Cols{
		"workspace_id": testWorkspaceID, "issue_id": issueID,
		"uploader_type": "member", "uploader_id": testUserID,
		"filename": "desktop-marked.png", "url": "memory://desktop-marked.png",
		"content_type": "image/png", "size_bytes": 120,
	})

	body := createOwnerReviewFeedbackRequest{
		InboxItemID:        inboxID,
		SourceAttachmentID: sourceID, MarkedScreenshotAttachmentID: markedID,
		Marks: []reviewMark{{ID: 7, Shot: sourceID, X: .1, Y: .2, W: .3, H: .4, Note: "This section has no purpose"}},
		Note:  "The hierarchy is not usable.",
	}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/review-feedback", body), "id", issueID)
	w := httptest.NewRecorder()
	testHandler.CreateOwnerReviewFeedback(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var response ownerReviewFeedbackResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testPool.Exec(t.Context(), `DELETE FROM owner_review_feedback WHERE id = $1`, response.ID) })
	if response.SourceAttachmentID != sourceID || response.MarkedScreenshotAttachmentID != markedID || response.SourceFilename != "desktop.png" {
		t.Fatalf("evidence identity was not preserved: %+v", response)
	}
	if response.IssueRevision < 1 {
		t.Fatalf("post-upload issue revision was not returned: %+v", response)
	}
	var marks []reviewMark
	if err := json.Unmarshal(response.Marks, &marks); err != nil || len(marks) != 1 || marks[0].Note != body.Marks[0].Note {
		t.Fatalf("structured marks were not preserved: %s (%v)", response.Marks, err)
	}
}

func TestCreateOwnerReviewFeedbackRejectsUnrelatedInboxItem(t *testing.T) {
	issueID := dbfx.Issue(t, "Reviewed issue")
	otherIssueID := dbfx.Issue(t, "Different issue")
	inboxID := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID,
		"issue_id": otherIssueID, "type": "workflow_owner_decision", "severity": "action_required",
		"title": "Different decision", "body": "Choose",
	})
	sourceID := dbfx.Insert(t, "attachment", testutil.Cols{
		"workspace_id": testWorkspaceID, "issue_id": issueID, "uploader_type": "member", "uploader_id": testUserID,
		"filename": "source.png", "url": "memory://source.png", "content_type": "image/png", "size_bytes": 100,
	})
	markedID := dbfx.Insert(t, "attachment", testutil.Cols{
		"workspace_id": testWorkspaceID, "issue_id": issueID, "uploader_type": "member", "uploader_id": testUserID,
		"filename": "marked.png", "url": "memory://marked.png", "content_type": "image/png", "size_bytes": 120,
	})
	body := createOwnerReviewFeedbackRequest{
		InboxItemID: inboxID, SourceAttachmentID: sourceID, MarkedScreenshotAttachmentID: markedID,
		Marks: []reviewMark{{ID: 1, Shot: sourceID, X: .1, Y: .1, W: .2, H: .2, Note: "Wrong section"}},
	}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/review-feedback", body), "id", issueID)
	w := httptest.NewRecorder()
	testHandler.CreateOwnerReviewFeedback(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}
