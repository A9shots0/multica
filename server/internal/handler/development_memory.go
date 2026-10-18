package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/developmentmemory"
)

type developmentMemoryApprovalRequest struct {
	Payload developmentmemory.ProposalDraft `json:"payload"`
	Note    string                          `json:"note"`
}

type developmentMemoryRejectionRequest struct {
	Note string `json:"note"`
}

func (h *Handler) ListDevelopmentMemoryAttention(w http.ResponseWriter, r *http.Request) {
	if h.DevelopmentMemoryStore == nil {
		writeJSON(w, http.StatusOK, []developmentmemory.Proposal{})
		return
	}
	proposals, err := h.DevelopmentMemoryStore.ListAttention(r.Context(), h.resolveWorkspaceID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "memory review is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, proposals)
}

func (h *Handler) ListIssueDevelopmentMemory(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if h.DevelopmentMemoryStore == nil {
		writeJSON(w, http.StatusOK, []developmentmemory.Proposal{})
		return
	}
	proposals, err := h.DevelopmentMemoryStore.ListIssue(r.Context(), uuidToString(issue.WorkspaceID), uuidToString(issue.ID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "development memory review is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, proposals)
}

func (h *Handler) ApproveIssueDevelopmentMemory(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if h.DevelopmentMemoryStore == nil {
		writeError(w, http.StatusServiceUnavailable, "development memory review is unavailable")
		return
	}
	var req developmentMemoryApprovalRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid development memory approval")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if len([]rune(req.Note)) > 4000 {
		writeError(w, http.StatusBadRequest, "review note must be at most 4000 characters")
		return
	}
	proposalUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposalId")
	if !ok {
		return
	}
	workspaceID, issueID, proposalID := uuidToString(issue.WorkspaceID), uuidToString(issue.ID), uuidToString(proposalUUID)
	binding, err := h.DevelopmentMemoryStore.BindingForProposal(r.Context(), workspaceID, issueID, proposalID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "development memory proposal not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "development memory proposal could not be verified")
		return
	}
	payload, err := developmentmemory.ValidateProposal(req.Payload, binding)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	proposal, err := h.DevelopmentMemoryStore.Approve(r.Context(), workspaceID, issueID, proposalID, requestUserID(r), payload, req.Note)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "development memory proposal was already reviewed")
			return
		}
		writeError(w, http.StatusInternalServerError, "development memory approval failed")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) RejectIssueDevelopmentMemory(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if h.DevelopmentMemoryStore == nil {
		writeError(w, http.StatusServiceUnavailable, "development memory review is unavailable")
		return
	}
	var req developmentMemoryRejectionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid development memory rejection")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if req.Note == "" || len([]rune(req.Note)) > 4000 {
		writeError(w, http.StatusBadRequest, "a rejection reason up to 4000 characters is required")
		return
	}
	proposalUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposalId")
	if !ok {
		return
	}
	proposal, err := h.DevelopmentMemoryStore.Reject(r.Context(), uuidToString(issue.WorkspaceID), uuidToString(issue.ID), uuidToString(proposalUUID), requestUserID(r), req.Note)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "development memory proposal was already reviewed")
			return
		}
		writeError(w, http.StatusInternalServerError, "development memory rejection failed")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) RetryIssueDevelopmentMemory(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if h.DevelopmentMemoryStore == nil {
		writeError(w, http.StatusServiceUnavailable, "development memory review is unavailable")
		return
	}
	proposalUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposalId")
	if !ok {
		return
	}
	proposal, err := h.DevelopmentMemoryStore.RetryDelivery(r.Context(), uuidToString(issue.WorkspaceID), uuidToString(issue.ID), uuidToString(proposalUUID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "development memory delivery is not retryable")
			return
		}
		writeError(w, http.StatusInternalServerError, "development memory retry failed")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}
