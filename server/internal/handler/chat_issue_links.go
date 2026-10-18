package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type chatIssueLinkResponse struct {
	IssueID   string `json:"issue_id"`
	Number    int32  `json:"number"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

type issueChatLinkResponse struct {
	ChatSessionID string `json:"chat_session_id"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Source        string `json:"source"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type linkChatIssueRequest struct {
	IssueID string `json:"issue_id"`
}

func (h *Handler) ListIssuesForChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	sessionID := chi.URLParam(r, "sessionId")
	if _, ok := h.gatePublicChatSessionForUser(w, r, userID, ctxWorkspaceID(r.Context()), sessionID); !ok {
		return
	}
	sessionUUID, ok := parseUUIDOrBadRequest(w, sessionID, "session id")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, ctxWorkspaceID(r.Context()), "workspace id")
	if !ok {
		return
	}
	rows, err := h.Queries.ListIssuesForChat(r.Context(), db.ListIssuesForChatParams{WorkspaceID: workspaceUUID, ChatSessionID: sessionUUID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list linked issues")
		return
	}
	result := make([]chatIssueLinkResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, chatIssueLinkResponse{IssueID: uuidToString(row.ID), Number: row.Number, Title: row.Title, Status: row.Status, Source: row.Source, CreatedAt: timestampToString(row.CreatedAt)})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) LinkChatIssue(w http.ResponseWriter, r *http.Request) {
	// Manual linking selects existing work on the owner's behalf. Task-created
	// issues use the separate origin link in their creation transaction.
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "select existing work through the owner's Chat session; machine credentials cannot create an owner selection")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")
	if _, ok := h.gatePublicChatSessionForUser(w, r, userID, workspaceID, sessionID); !ok {
		return
	}
	var req linkChatIssueRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.IssueID) == "" {
		writeError(w, http.StatusBadRequest, "issue_id is required")
		return
	}
	sessionUUID, ok := parseUUIDOrBadRequest(w, sessionID, "session id")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	issueUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.IssueID), "issue id")
	if !ok {
		return
	}
	if _, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: issueUUID, WorkspaceID: workspaceUUID}); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load issue")
		return
	}
	link, err := h.Queries.LinkChatIssue(r.Context(), db.LinkChatIssueParams{WorkspaceID: workspaceUUID, ChatSessionID: sessionUUID, IssueID: issueUUID, Source: "manual"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link issue")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issue_id": uuidToString(link.IssueID), "chat_session_id": uuidToString(link.ChatSessionID), "source": link.Source, "created_at": timestampToString(link.CreatedAt)})
}

func (h *Handler) UnlinkChatIssue(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")
	if _, ok := h.gatePublicChatSessionForUser(w, r, userID, workspaceID, sessionID); !ok {
		return
	}
	sessionUUID, ok := parseUUIDOrBadRequest(w, sessionID, "session id")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	issueUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "issueId"), "issue id")
	if !ok {
		return
	}
	if err := h.Queries.UnlinkChatIssue(r.Context(), db.UnlinkChatIssueParams{WorkspaceID: workspaceUUID, ChatSessionID: sessionUUID, IssueID: issueUUID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink issue")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListChatsForIssue(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListChatsForIssue(r.Context(), db.ListChatsForIssueParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list linked chats")
		return
	}
	result := make([]issueChatLinkResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, issueChatLinkResponse{ChatSessionID: uuidToString(row.ID), Title: row.Title, Status: row.Status, Source: row.Source, CreatedAt: timestampToString(row.CreatedAt), UpdatedAt: timestampToString(row.UpdatedAt)})
	}
	writeJSON(w, http.StatusOK, result)
}
