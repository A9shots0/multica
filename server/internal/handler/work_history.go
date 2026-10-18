package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/workhistory"
)

const (
	maxWorkHistoryImportBytes = 768 * 1024
	maxWorkHistorySearchBytes = 8 * 1024
)

type workHistoryImportRequest struct {
	ProjectID string                 `json:"project_id"`
	Fragments []workhistory.Fragment `json:"fragments"`
}

type workHistorySearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

func decodeSingleJSON(w http.ResponseWriter, r *http.Request, maximum int64, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximum))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

// ImportWorkHistory accepts only already-redacted fragments from an owner or
// administrator. The Multica project UUID is mapped server-side to the Brain
// project so a caller cannot widen the destination with request prose.
func (h *Handler) ImportWorkHistory(w http.ResponseWriter, r *http.Request) {
	if h.SharedBrain == nil || h.SharedBrain.Client == nil {
		writeError(w, http.StatusServiceUnavailable, "shared brain is not configured")
		return
	}
	var request workHistoryImportRequest
	if err := decodeSingleJSON(w, r, maxWorkHistoryImportBytes, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid work history import")
		return
	}
	workspaceID := middleware.WorkspaceIDFromContext(r.Context())
	if _, err := uuid.Parse(request.ProjectID); err != nil || len(request.Fragments) == 0 || len(request.Fragments) > 100 {
		writeError(w, http.StatusBadRequest, "invalid work history import scope")
		return
	}
	target, ok := h.SharedBrain.Client.Project(workspaceID, request.ProjectID)
	if !ok {
		writeError(w, http.StatusNotFound, "project has no shared brain mapping")
		return
	}
	for index := range request.Fragments {
		if request.Fragments[index].Project != request.ProjectID {
			writeError(w, http.StatusBadRequest, "work history fragment project mismatch")
			return
		}
		request.Fragments[index].Project = target
	}
	payload, err := json.Marshal(map[string]any{"fragments": request.Fragments})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "work history import could not be prepared")
		return
	}
	raw, err := h.SharedBrain.Client.IngestHistory(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadGateway, "shared brain could not store work history")
		return
	}
	var response map[string]any
	if json.Unmarshal(raw, &response) != nil {
		writeError(w, http.StatusBadGateway, "shared brain returned an unreadable history receipt")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// SearchWorkHistory searches only Brain projects enrolled for the selected
// Multica workspace. Results remain explicitly unaccepted recollection.
func (h *Handler) SearchWorkHistory(w http.ResponseWriter, r *http.Request) {
	if h.SharedBrain == nil || h.SharedBrain.Client == nil {
		writeError(w, http.StatusServiceUnavailable, "shared brain is not configured")
		return
	}
	var request workHistorySearchRequest
	if err := decodeSingleJSON(w, r, maxWorkHistorySearchBytes, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid work history search")
		return
	}
	request.Query = strings.TrimSpace(request.Query)
	if request.Query == "" || len(request.Query) > 2000 {
		writeError(w, http.StatusBadRequest, "work history query must be 1 to 2000 characters")
		return
	}
	if request.Limit == 0 {
		request.Limit = 20
	}
	if request.Limit < 1 || request.Limit > 50 {
		writeError(w, http.StatusBadRequest, "work history limit must be 1 to 50")
		return
	}
	workspaceID := middleware.WorkspaceIDFromContext(r.Context())
	projects := h.SharedBrain.Client.Projects(workspaceID)
	if len(projects) == 0 {
		writeError(w, http.StatusNotFound, "workspace has no shared brain projects")
		return
	}
	raw, err := h.SharedBrain.Client.SearchHistory(r.Context(), projects, request.Query, request.Limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, "shared brain could not search work history")
		return
	}
	var response map[string]any
	if json.Unmarshal(raw, &response) != nil {
		writeError(w, http.StatusBadGateway, "shared brain returned unreadable work history")
		return
	}
	writeJSON(w, http.StatusOK, response)
}
