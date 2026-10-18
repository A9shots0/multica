package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// reviewMark is normalized to the reviewed image. That makes the feedback
// stable when the screenshot is viewed on a phone, laptop, or large monitor.
type reviewMark struct {
	ID   int     `json:"id"`
	Shot string  `json:"shot"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	Note string  `json:"note"`
}

type createOwnerReviewFeedbackRequest struct {
	InboxItemID                  string       `json:"inbox_item_id"`
	SourceAttachmentID           string       `json:"source_attachment_id"`
	MarkedScreenshotAttachmentID string       `json:"marked_screenshot_attachment_id"`
	Marks                        []reviewMark `json:"marks"`
	Note                         string       `json:"note"`
}

type ownerReviewFeedbackResponse struct {
	ID                           string          `json:"id"`
	IssueID                      string          `json:"issue_id"`
	InboxItemID                  *string         `json:"inbox_item_id,omitempty"`
	SourceAttachmentID           string          `json:"source_attachment_id"`
	SourceFilename               string          `json:"source_filename"`
	SourceContentType            string          `json:"source_content_type"`
	MarkedScreenshotAttachmentID string          `json:"marked_screenshot_attachment_id"`
	Marks                        json.RawMessage `json:"marks"`
	Note                         string          `json:"note"`
	SchemaVersion                int32           `json:"schema_version"`
	// The marked screenshot upload is an issue attachment and therefore moves
	// the issue revision before this receipt is created. Return the revision
	// observed by this endpoint so the immediately-following owner directive
	// can be checked against the post-upload issue, not the stale pre-markup
	// revision the page originally loaded.
	IssueRevision int64  `json:"issue_revision"`
	CreatedAt     string `json:"created_at"`
}

func ownerReviewFeedbackToResponse(row db.OwnerReviewFeedback, issueRevision int64) ownerReviewFeedbackResponse {
	return ownerReviewFeedbackResponse{
		ID:                           uuidToString(row.ID),
		IssueID:                      uuidToString(row.IssueID),
		InboxItemID:                  uuidToPtr(row.InboxItemID),
		SourceAttachmentID:           uuidToString(row.SourceAttachmentID),
		SourceFilename:               row.SourceFilename,
		SourceContentType:            row.SourceContentType,
		MarkedScreenshotAttachmentID: uuidToString(row.MarkedScreenshotAttachmentID),
		Marks:                        json.RawMessage(row.Marks),
		Note:                         row.Note,
		SchemaVersion:                row.SchemaVersion,
		IssueRevision:                issueRevision,
		CreatedAt:                    timestampToString(row.CreatedAt),
	}
}

func validReviewMarks(marks []reviewMark, sourceID string) bool {
	if len(marks) == 0 || len(marks) > 100 {
		return false
	}
	seen := make(map[int]struct{}, len(marks))
	for _, mark := range marks {
		if mark.ID <= 0 || mark.Shot != sourceID || strings.TrimSpace(mark.Note) == "" || len([]rune(mark.Note)) > 2000 {
			return false
		}
		if _, duplicate := seen[mark.ID]; duplicate {
			return false
		}
		seen[mark.ID] = struct{}{}
		if mark.X < 0 || mark.Y < 0 || mark.W <= 0 || mark.H <= 0 || mark.X+mark.W > 1.000001 || mark.Y+mark.H > 1.000001 {
			return false
		}
	}
	return true
}

// CreateOwnerReviewFeedback records the exact source, structured marks, and a
// rendered screenshot. It intentionally does not answer the owner decision:
// the browser must receive this durable receipt before it may advance work.
func (h *Handler) CreateOwnerReviewFeedback(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req createOwnerReviewFeedbackRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid review feedback")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if len([]rune(req.Note)) > 4000 {
		writeError(w, http.StatusBadRequest, "review note must be at most 4000 characters")
		return
	}
	sourceID, ok := parseUUIDOrBadRequest(w, req.SourceAttachmentID, "source_attachment_id")
	if !ok {
		return
	}
	markedID, ok := parseUUIDOrBadRequest(w, req.MarkedScreenshotAttachmentID, "marked_screenshot_attachment_id")
	if !ok {
		return
	}
	if req.SourceAttachmentID == req.MarkedScreenshotAttachmentID {
		writeError(w, http.StatusBadRequest, "marked screenshot must be a new attachment")
		return
	}
	var inboxID pgtype.UUID
	if strings.TrimSpace(req.InboxItemID) != "" {
		inboxID, ok = parseUUIDOrBadRequest(w, req.InboxItemID, "inbox_item_id")
		if !ok {
			return
		}
		item, itemErr := h.Queries.GetInboxItemInWorkspace(r.Context(), db.GetInboxItemInWorkspaceParams{
			ID: inboxID, WorkspaceID: issue.WorkspaceID,
		})
		if itemErr != nil || item.RecipientType != "member" || uuidToString(item.RecipientID) != requestUserID(r) ||
			!item.IssueID.Valid || item.IssueID != issue.ID {
			writeError(w, http.StatusBadRequest, "inbox item does not belong to this issue review")
			return
		}
	}
	if !validReviewMarks(req.Marks, req.SourceAttachmentID) {
		writeError(w, http.StatusBadRequest, "review marks must identify this screenshot, include notes, and stay within its bounds")
		return
	}
	source, err := h.Queries.GetAttachment(r.Context(), db.GetAttachmentParams{ID: sourceID, WorkspaceID: issue.WorkspaceID})
	if err != nil || !source.IssueID.Valid || uuidToString(source.IssueID) != uuidToString(issue.ID) || !strings.HasPrefix(strings.ToLower(source.ContentType), "image/") {
		writeError(w, http.StatusBadRequest, "source screenshot does not belong to this issue")
		return
	}
	marked, err := h.Queries.GetAttachment(r.Context(), db.GetAttachmentParams{ID: markedID, WorkspaceID: issue.WorkspaceID})
	if err != nil || !marked.IssueID.Valid || uuidToString(marked.IssueID) != uuidToString(issue.ID) || !strings.HasPrefix(strings.ToLower(marked.ContentType), "image/") {
		writeError(w, http.StatusBadRequest, "marked screenshot does not belong to this issue")
		return
	}
	marksJSON, err := json.Marshal(req.Marks)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid review marks")
		return
	}
	row, err := h.Queries.CreateOwnerReviewFeedback(r.Context(), db.CreateOwnerReviewFeedbackParams{
		ID:                           dbid.NewV7(),
		WorkspaceID:                  issue.WorkspaceID,
		UserID:                       parseUUID(requestUserID(r)),
		IssueID:                      issue.ID,
		InboxItemID:                  inboxID,
		SourceAttachmentID:           source.ID,
		SourceFilename:               source.Filename,
		SourceContentType:            source.ContentType,
		MarkedScreenshotAttachmentID: marked.ID,
		Marks:                        marksJSON,
		Note:                         req.Note,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "review feedback could not be saved")
		return
	}
	writeJSON(w, http.StatusCreated, ownerReviewFeedbackToResponse(row, issue.Revision))
}

func (h *Handler) ListOwnerReviewFeedback(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListOwnerReviewFeedbackByIssue(r.Context(), db.ListOwnerReviewFeedbackByIssueParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "review feedback is unavailable")
		return
	}
	result := make([]ownerReviewFeedbackResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, ownerReviewFeedbackToResponse(row, issue.Revision))
	}
	writeJSON(w, http.StatusOK, result)
}
