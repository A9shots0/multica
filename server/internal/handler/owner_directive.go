package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ownerDirectiveRequest struct {
	OwnerRuntimeRoute     *service.OwnerRuntimeRouteRequest `json:"owner_runtime_route,omitempty"`
	IntakeID              string                            `json:"intake_id,omitempty"`
	InboxItemID           string                            `json:"inbox_item_id,omitempty"`
	ExpectedIssueRevision int64                             `json:"expected_issue_revision"`
	Directive             string                            `json:"directive"`
	Content               string                            `json:"content"`
	ResultTaskID          string                            `json:"result_task_id,omitempty"`
	ResultDigest          string                            `json:"result_digest,omitempty"`
}

// Detailed corrections include the owner's exact prose and screenshot links.
// Keep Chat and the canonical directive boundary on the same bounded limit.
const maxOwnerDirectiveContentBytes = 64 * 1024

var validOwnerDirectives = map[string]bool{
	"correct": true, "request_changes": true, "partial_acceptance": true,
	"change_scope": true, "cancel": true, "accept_result": true,
}

// PostIssueOwnerDirective is the authoritative Inbox correction boundary.
// The typed directive and its issue comment commit together against an exact
// issue revision. Existing work is then stopped before the correction comment
// can start a replacement run, so an old scope cannot continue concurrently.
func (h *Handler) PostIssueOwnerDirective(w http.ResponseWriter, r *http.Request) {
	if isMachineCredentialActor(r) && !hasChatOwnerDelegation(r) {
		writeError(w, http.StatusForbidden, "owner directives require a human actor")
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ownerDirectiveRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Directive, req.Content = strings.TrimSpace(req.Directive), strings.TrimSpace(sanitizeNullBytes(req.Content))
	if !validOwnerDirectives[req.Directive] || req.ExpectedIssueRevision < 1 || req.Content == "" || len(req.Content) > maxOwnerDirectiveContentBytes {
		writeError(w, http.StatusBadRequest, "directive, expected_issue_revision, and content are required")
		return
	}
	if req.Directive == "accept_result" && (!hasChatOwnerDelegation(r) || req.ResultTaskID == "" || req.ResultDigest == "" || req.IntakeID != "") {
		writeError(w, http.StatusForbidden, "acceptance requires the owner's selected result in Chat")
		return
	}
	if req.IntakeID != "" {
		h.continueChatIntake(w, r, issue, req)
		return
	}
	if req.OwnerRuntimeRoute != nil {
		writeError(w, http.StatusBadRequest, "workflow routes require a pending Chat intake")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start owner directive")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	// Reuse the original Chat files, retaining their message/session pointers.
	// Only this immutable owner input batch can supply evidence. Lock files
	// before the issue, matching the existing attachment mutation lock order.
	var attachmentIDs []pgtype.UUID
	if hasChatOwnerDelegation(r) {
		grant := r.Context().Value(chatOwnerDelegationKey{}).(chatOwnerDelegation)
		attachments, readErr := qtx.ListAttachmentsByChatMessageIDs(r.Context(), db.ListAttachmentsByChatMessageIDsParams{Column1: grant.MessageIDs, WorkspaceID: issue.WorkspaceID})
		if readErr != nil {
			writeError(w, 500, "owner evidence unavailable")
			return
		}
		for _, attachment := range attachments {
			if attachment.ChatSessionID != grant.SessionID || attachment.UploaderType != "member" || uuidToString(attachment.UploaderID) != userID || attachment.SourceContextID.Valid {
				writeError(w, 409, "owner evidence scope changed")
				return
			}
			if attachment.IssueID.Valid {
				if attachment.IssueID != issue.ID {
					writeError(w, 409, "owner evidence belongs to another issue")
					return
				}
				continue // Already delivered; never move a prior comment's files.
			}
			attachmentIDs = append(attachmentIDs, attachment.ID)
		}
		if len(attachmentIDs) > 0 {
			lockedFiles, lockErr := qtx.LockAttachmentsForIssueLink(r.Context(), db.LockAttachmentsForIssueLinkParams{WorkspaceID: issue.WorkspaceID, AttachmentIds: attachmentIDs})
			if lockErr != nil || len(lockedFiles) != len(attachmentIDs) {
				writeError(w, 409, "owner evidence changed; retry the correction")
				return
			}
		}
	}
	locked, err := qtx.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err == nil && hasChatOwnerDelegation(r) {
		prior, readErr := qtx.ListIssueOwnerDirectives(r.Context(), db.ListIssueOwnerDirectivesParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
		if readErr != nil {
			writeError(w, 500, "directive receipt unavailable")
			return
		}
		for _, existing := range prior {
			if uuidToString(existing.ID) != chatDelegatedDirectiveID(r) {
				continue
			}
			if existing.DirectiveType != req.Directive || existing.Content != req.Content || existing.ExpectedIssueRevision != req.ExpectedIssueRevision || uuidToString(existing.ActorID) != userID {
				writeError(w, 409, "this Chat turn already submitted a different direction")
				return
			}
			if existing.DirectiveType == "request_changes" && h.TaskService != nil {
				if _, launchErr := h.TaskService.LaunchAIFailureInvestigation(r.Context(), issue, "owner_rejection", existing.ID, pgtype.UUID{}, pgtype.UUID{}, parseUUID(userID), existing.Content); launchErr != nil {
					slog.Warn("replay owner rejection investigation failed", "issue_id", uuidToString(issue.ID), "directive_id", uuidToString(existing.ID), "error", launchErr)
				}
			}
			// The durable receipt proves recording only. Never repeat dispatch
			// after a response loss; refresh work to inspect execution separately.
			writeJSON(w, 200, map[string]any{"id": uuidToString(existing.ID), "issue_id": uuidToString(issue.ID), "revision": existing.Revision, "directive": existing.DirectiveType, "state": existing.State, "comment_id": uuidToString(existing.CommentID), "replayed": true})
			return
		}
	}
	if err != nil || locked.Revision != req.ExpectedIssueRevision {
		writeError(w, http.StatusConflict, "issue changed; reload before directing work")
		return
	}
	directiveID := dbid.NewV7()
	if delegatedID := chatDelegatedDirectiveID(r); delegatedID != "" {
		directiveID = parseUUID(delegatedID)
	}
	commentText := fmt.Sprintf("Owner directive [%s] (%s):\n\n%s", uuidToString(directiveID), strings.ReplaceAll(req.Directive, "_", " "), req.Content)
	if req.Directive == "accept_result" {
		result, err := qtx.GetChatAcceptableResult(r.Context(), db.GetChatAcceptableResultParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, OwnerID: parseUUID(userID)})
		if err != nil || uuidToString(result.ID) != req.ResultTaskID || service.ChatResultDigest(result.Result) != req.ResultDigest {
			writeError(w, http.StatusConflict, "the reviewed result changed or requires a workflow decision; refresh the issue in Chat")
			return
		}
		commentText += fmt.Sprintf("\n\nAccepted result: %s (SHA-256 %s)", req.ResultTaskID, req.ResultDigest)
	}
	created, err := qtx.CreateComment(r.Context(), db.CreateCommentParams{
		ID: dbid.NewV7(), IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
		AuthorType: "member", AuthorID: parseUUID(userID), Content: commentText, Type: "comment",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record owner directive")
		return
	}
	var closedInbox *db.InboxItem
	if len(attachmentIDs) > 0 {
		linked, linkErr := qtx.LinkAttachmentsToIssue(r.Context(), db.LinkAttachmentsToIssueParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, AttachmentIds: attachmentIDs, BumpRevision: false})
		if linkErr != nil || linked.LinkedCount != int64(len(attachmentIDs)) {
			writeError(w, 409, "owner evidence changed; retry the correction")
			return
		}
		if err := qtx.LinkAttachmentsToComment(r.Context(), db.LinkAttachmentsToCommentParams{CommentID: created.ID, IssueID: issue.ID, Column3: attachmentIDs}); err != nil {
			writeError(w, 500, "failed to attach owner evidence")
			return
		}
	}
	if req.InboxItemID != "" {
		inboxItemID, valid := parseUUIDOrBadRequest(w, req.InboxItemID, "inbox item id")
		if !valid {
			return
		}
		closed, closeErr := qtx.CloseInboxItemForOwnerDirective(r.Context(), db.CloseInboxItemForOwnerDirectiveParams{
			InboxItemID: inboxItemID,
			WorkspaceID: issue.WorkspaceID,
			OwnerID:     parseUUID(userID),
			IssueID:     issue.ID,
		})
		if closeErr != nil {
			writeError(w, http.StatusConflict, "this Inbox request is no longer available; refresh before directing work")
			return
		}
		closedInbox = &closed
	}
	state := "recorded"
	if req.Directive == "accept_result" {
		state = "applied"
		updated, err := qtx.UpdateIssueStatus(r.Context(), db.UpdateIssueStatusParams{ID: issue.ID, Status: "done", WorkspaceID: issue.WorkspaceID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to accept result")
			return
		}
		issue = updated
	}
	if req.Directive == "cancel" {
		state = "cancelled"
		if _, err := qtx.UpdateIssueStatus(r.Context(), db.UpdateIssueStatusParams{ID: issue.ID, Status: "cancelled", WorkspaceID: issue.WorkspaceID}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to cancel issue")
			return
		}
	}
	directive, err := qtx.CreateIssueOwnerDirective(r.Context(), db.CreateIssueOwnerDirectiveParams{
		ID: directiveID, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID,
		DirectiveType: req.Directive, ExpectedIssueRevision: req.ExpectedIssueRevision,
		Content: req.Content, State: state, ActorID: parseUUID(userID), CommentID: created.ID,
	})
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit owner directive")
		return
	}

	// A correction invalidates the in-flight interpretation of the old scope.
	// Stop it first; only then may the newly persisted comment start replacement work.
	if h.TaskService != nil && req.Directive != "accept_result" {
		if err := h.TaskService.CancelTasksForIssue(r.Context(), issue.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "directive recorded but active work could not be stopped")
			return
		}
	}
	comment := created.Comment()
	issueRevision := created.IssueRevision
	if req.Directive == "accept_result" {
		issueRevision = issue.Revision
	}
	groupedAtt := h.groupAttachments(r, []pgtype.UUID{comment.ID})
	commentResp := commentToResponse(comment, nil, groupedAtt[uuidToString(comment.ID)])
	commentResp.IssueRevision = created.IssueRevision
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "member", userID, map[string]any{
		"comment": commentResp, "issue_title": issue.Title, "issue_status": issue.Status,
		"issue_revision": issueRevision,
	})
	investigationState := "not_required"
	if req.Directive == "request_changes" && h.TaskService != nil {
		investigationState = "queued"
		if _, launchErr := h.TaskService.LaunchAIFailureInvestigation(r.Context(), issue, "owner_rejection", directive.ID, pgtype.UUID{}, pgtype.UUID{}, parseUUID(userID), req.Content); launchErr != nil {
			investigationState = "recording_failed"
			slog.Warn("launch owner rejection investigation failed", "issue_id", uuidToString(issue.ID), "directive_id", uuidToString(directive.ID), "error", launchErr)
		}
	}
	var outcomes []CommentTriggerOutcome
	if req.Directive != "cancel" && req.Directive != "accept_result" && req.Directive != "request_changes" {
		outcomes = h.triggerTasksForComment(r.Context(), issue, comment, nil, "member", userID, userID, nil)
	}
	if req.Directive == "accept_result" {
		h.publish(protocol.EventIssueUpdated, uuidToString(issue.WorkspaceID), "member", userID, map[string]any{"issue": issueToResponse(issue, h.getIssuePrefix(r.Context(), issue.WorkspaceID)), "status_changed": true})
	}
	if closedInbox != nil {
		h.publish(protocol.EventInboxArchived, uuidToString(issue.WorkspaceID), "member", userID, map[string]any{
			"item_id":      uuidToString(closedInbox.ID),
			"issue_id":     uuidToPtr(closedInbox.IssueID),
			"recipient_id": uuidToString(closedInbox.RecipientID),
		})
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": uuidToString(directive.ID), "issue_id": uuidToString(issue.ID),
		"revision": directive.Revision, "directive": directive.DirectiveType,
		"state": directive.State, "comment_id": uuidToString(created.ID),
		"issue_revision": issueRevision, "trigger_outcomes": outcomes,
		"failure_investigation": investigationState,
		"inbox_item_id":         req.InboxItemID,
	})
}
