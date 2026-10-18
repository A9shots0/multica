package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/previewpublication"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type previewPublicationStore interface {
	Register(context.Context, pgtype.UUID, string, previewpublication.ProducerClaim) (previewpublication.Publication, bool, error)
	Load(context.Context, pgtype.UUID, pgtype.UUID) (previewpublication.Publication, error)
}

func (h *Handler) previewPublicationStore() previewPublicationStore {
	if h.PreviewPublications != nil {
		return h.PreviewPublications
	}
	return previewpublication.Store{DB: h.TxStarter, Now: time.Now}
}

func (h *Handler) PublishTaskPreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task_id")
	if !ok {
		return
	}
	daemonID := middleware.DaemonIDFromContext(r.Context())
	if daemonID == "" {
		writeError(w, http.StatusForbidden, "daemon token required")
		return
	}
	var request struct {
		PreviewURL  string                        `json:"preview_url"`
		IdentityURL string                        `json:"identity_url"`
		DataPolicy  previewpublication.DataPolicy `json:"data_policy"`
		ExpiresAt   string                        `json:"expires_at"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid preview publication")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid preview publication")
		return
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
	if err != nil || !strings.HasSuffix(request.ExpiresAt, "Z") {
		writeError(w, http.StatusBadRequest, "invalid preview publication")
		return
	}
	claim := previewpublication.ProducerClaim{
		PreviewURL: request.PreviewURL, IdentityURL: request.IdentityURL,
		DataPolicy: request.DataPolicy, ExpiresAt: expiresAt,
	}
	publication, replay, err := h.previewPublicationStore().Register(r.Context(), taskID, daemonID, claim)
	switch {
	case errors.Is(err, previewpublication.ErrInvalidPublication):
		writeError(w, http.StatusBadRequest, "invalid preview publication")
	case errors.Is(err, previewpublication.ErrAuthorityUnavailable):
		writeError(w, http.StatusConflict, "preview publication authority unavailable")
	case errors.Is(err, previewpublication.ErrPublicationConflict):
		writeError(w, http.StatusConflict, "preview publication is immutable")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "failed to publish preview")
	default:
		status := http.StatusCreated
		if replay {
			status = http.StatusOK
		}
		writeJSON(w, status, publication)
	}
}

func (h *Handler) GetIssuePreviewPublication(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "issueId"), "issue_id")
	if !ok {
		return
	}
	workspaceID, err := util.ParseUUID(r.Header.Get("X-Workspace-ID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "preview publication not found")
		return
	}
	publication, err := h.previewPublicationStore().Load(r.Context(), workspaceID, issueID)
	switch {
	case errors.Is(err, previewpublication.ErrPublicationExpired):
		writeError(w, http.StatusGone, "preview publication expired")
	case errors.Is(err, previewpublication.ErrPublicationUnavailable):
		writeError(w, http.StatusNotFound, "preview publication not found")
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, "preview publication unavailable")
	default:
		writeJSON(w, http.StatusOK, publication)
	}
}

func (h *Handler) RelayIssuePreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "issueId"), "issue_id")
	if !ok {
		return
	}
	candidateID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "candidateId"), "candidate_id")
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(chi.URLParam(r, "candidateRevision"), 10, 64)
	if err != nil || revision != 1 {
		writeError(w, http.StatusBadRequest, "invalid candidate revision")
		return
	}
	workspaceID, err := util.ParseUUID(r.Header.Get("X-Workspace-ID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "preview publication not found")
		return
	}
	requestPath := "/" + chi.URLParam(r, "*")
	if len(requestPath) > 2<<10 || strings.ContainsRune(requestPath, '\x00') ||
		strings.Contains(requestPath, `\`) {
		writeError(w, http.StatusBadRequest, "invalid preview path")
		return
	}
	for _, segment := range strings.Split(requestPath, "/") {
		if segment == "." || segment == ".." {
			writeError(w, http.StatusBadRequest, "invalid preview path")
			return
		}
	}
	publication, err := h.previewPublicationStore().Load(r.Context(), workspaceID, issueID)
	switch {
	case errors.Is(err, previewpublication.ErrPublicationExpired):
		writeError(w, http.StatusGone, "preview publication expired")
		return
	case errors.Is(err, previewpublication.ErrPublicationUnavailable):
		writeError(w, http.StatusNotFound, "preview publication not found")
		return
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, "preview publication unavailable")
		return
	}
	if publication.Binding.CandidateID != util.UUIDToString(candidateID) ||
		publication.Binding.CandidateRevision != revision ||
		publication.Binding.IssueID != util.UUIDToString(issueID) ||
		publication.Binding.WorkspaceID != util.UUIDToString(workspaceID) {
		writeError(w, http.StatusNotFound, "preview publication not found")
		return
	}
	if h.DaemonHub == nil {
		writeError(w, http.StatusServiceUnavailable, "preview relay unavailable")
		return
	}
	rpcCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	response, err := h.DaemonHub.CallRuntime(
		rpcCtx,
		publication.Binding.WorkspaceID,
		publication.Binding.Worktree.DaemonID,
		publication.Binding.Worktree.RuntimeID,
		"preview.fetch",
		protocol.PreviewFetchRequest{
			WorkspaceID: publication.Binding.WorkspaceID, IssueID: publication.Binding.IssueID,
			AgentID: publication.Binding.AgentID, AgentRunID: publication.Binding.AgentRunID,
			TaskID:      publication.Binding.AgentRunID,
			CandidateID: publication.Binding.CandidateID, CandidateRevision: publication.Binding.CandidateRevision,
			SourceRevision: publication.Binding.SourceRevision,
			DaemonID:       publication.Binding.Worktree.DaemonID,
			RuntimeID:      publication.Binding.Worktree.RuntimeID,
			WorktreePath:   publication.Binding.Worktree.Path,
			Branch:         publication.Binding.Worktree.Branch,
			Method:         r.Method, Path: requestPath,
		},
	)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "preview relay unavailable")
		return
	}
	var fetch protocol.PreviewFetchResponse
	if len(response.Body) == 0 || json.Unmarshal(response.Body, &fetch) != nil {
		writeError(w, http.StatusServiceUnavailable, "preview relay unavailable")
		return
	}
	switch fetch.Status {
	case http.StatusOK, http.StatusNotFound, http.StatusGone, http.StatusMethodNotAllowed,
		http.StatusRequestEntityTooLarge, http.StatusServiceUnavailable:
	default:
		writeError(w, http.StatusServiceUnavailable, "preview relay unavailable")
		return
	}
	if len(fetch.Body) > 8<<20 {
		writeError(w, http.StatusRequestEntityTooLarge, "preview response too large")
		return
	}
	for _, name := range []string{
		"Content-Type", "Content-Length", "Content-Security-Policy",
		"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy",
		"Permissions-Policy", "Cross-Origin-Resource-Policy", "Allow",
	} {
		if value := fetch.Headers[name]; value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(fetch.Status)
	if r.Method == http.MethodGet {
		_, _ = w.Write(fetch.Body)
	}
}
