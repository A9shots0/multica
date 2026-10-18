package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/plugincontract"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
	"github.com/multica-ai/multica/server/pkg/redact"
)

const (
	maxContextRequestBytes = 64 * 1024
	maxCompactContextBytes = 64 * 1024
)

// CreatePluginContextRequest resolves a deliberately small, read-only context
// packet for outside work already linked to an issue. Workspace and Brain
// project routing come only from the authenticated issue; neither can be
// selected by request prose.
func (h *Handler) CreatePluginContextRequest(w http.ResponseWriter, r *http.Request) {
	caller, _, ok := h.pluginCaller(w, r, plugincontract.ScopeIssuesRead)
	if !ok {
		return
	}
	issue, ok := h.pluginIssueForUser(w, r, caller, chi.URLParam(r, "issue_ref"))
	if !ok {
		return
	}

	var request publicapiv1.ContextRequestV02
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxContextRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		publicapiv1.WriteProblem(w, r, http.StatusBadRequest, "invalid_request", "invalid Context Request v0.2 body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		publicapiv1.WriteProblem(w, r, http.StatusBadRequest, "invalid_request", "request body must contain one JSON object")
		return
	}
	if err := request.Validate(); err != nil {
		publicapiv1.WriteProblem(w, r, http.StatusBadRequest, "invalid_context_request", err.Error())
		return
	}

	now := time.Now().UTC()
	respond := func(status, system, evidence string, reference *string, payload map[string]any, limitations []string, observedAt string, expiresAt *string) {
		response := publicapiv1.ContextResponseV02{
			RequestID: request.RequestID, WorkID: request.WorkID, Status: status,
			Source:        publicapiv1.ContextResponseSource{System: system, Reference: reference},
			EvidenceLevel: evidence, ObservedAt: observedAt, ExpiresAt: expiresAt,
			Payload: payload, Limitations: limitations,
		}
		if err := response.Validate(); err != nil {
			publicapiv1.WriteProblem(w, r, http.StatusInternalServerError, "invalid_context_response", "context broker produced an invalid response")
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
	observedNow := now.Format(time.RFC3339Nano)
	baseLimitations := []string{"Shared context is reference data, not permission, runtime truth, or task acceptance."}

	linked, err := h.Queries.HasAdmittedExternalWorkForIssue(r.Context(), db.HasAdmittedExternalWorkForIssueParams{
		WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, WorkID: request.WorkID,
	})
	if err != nil {
		respond("error", "multica", "observed", nil, map[string]any{}, []string{"The linked-work check could not be completed."}, observedNow, nil)
		return
	}
	if !linked || request.AuthorizationBoundary != "read_only" {
		respond("unauthorized", "multica", "authoritative", nil, map[string]any{}, []string{"This request is not read-only linked work on the selected issue."}, observedNow, nil)
		return
	}
	if request.MaximumPayload != "compact" {
		respond("too_large", "brain", "observed", nil, map[string]any{}, []string{"The first broker slice serves compact packets only; request an exact immutable reference before asking for a document or artifact."}, observedNow, nil)
		return
	}
	if request.RequestType != "source" && request.RequestType != "policy" {
		respond("unknown", "multica", "observed", nil, map[string]any{}, []string{"No deterministic resolver is installed for this request type yet."}, observedNow, nil)
		return
	}
	if request.ResponseContract.EvidenceLevel == "authoritative" {
		respond("unknown", "brain", "observed", nil, map[string]any{}, []string{"A Brain search packet is observed reference data. Request an exact immutable document revision for authoritative evidence."}, observedNow, nil)
		return
	}
	if len(request.Need) > 1000 {
		respond("too_large", "brain", "observed", nil, map[string]any{}, []string{"The context query exceeds the 1,000-byte broker limit; ask for one narrower fact."}, observedNow, nil)
		return
	}
	if h.SharedBrain == nil || h.SharedBrain.Client == nil {
		respond("error", "brain", "observed", nil, map[string]any{}, []string{"Shared Brain is not configured."}, observedNow, nil)
		return
	}
	project, allowed := h.SharedBrain.Client.Project(uuidToString(issue.WorkspaceID), uuidToString(issue.ProjectID))
	if !allowed {
		respond("unauthorized", "brain", "authoritative", nil, map[string]any{}, []string{"The selected issue has no authorized Brain project mapping."}, observedNow, nil)
		return
	}

	raw, err := h.SharedBrain.Client.Context(r.Context(), project, redact.Text(request.Need))
	if err != nil {
		respond("error", "brain", "observed", nil, map[string]any{}, []string{"Shared Brain could not serve the bounded request."}, observedNow, nil)
		return
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil {
		respond("error", "brain", "observed", nil, map[string]any{}, []string{"Shared Brain returned an unreadable packet."}, observedNow, nil)
		return
	}
	observedAt := observedNow
	if servedRaw, exists := envelope["servedAt"]; exists {
		var served string
		if json.Unmarshal(servedRaw, &served) == nil {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, served); parseErr == nil {
				observedAt = parsed.UTC().Format(time.RFC3339Nano)
			}
		}
	}

	payload := make(map[string]any, len(request.ResponseContract.RequiredFields))
	missing := make([]string, 0)
	allowedEnvelopeFields := map[string]bool{"project": true, "references": true}
	for _, field := range request.ResponseContract.RequiredFields {
		if field == "served_at" || field == "observed_at" {
			payload[field] = observedAt
			continue
		}
		if !allowedEnvelopeFields[field] {
			missing = append(missing, field)
			continue
		}
		value, exists := envelope[field]
		if !exists {
			missing = append(missing, field)
			continue
		}
		var decoded any
		if json.Unmarshal(value, &decoded) != nil {
			missing = append(missing, field)
			continue
		}
		payload[field] = decoded
	}
	encoded, _ := json.Marshal(payload)
	if len(encoded) > maxCompactContextBytes {
		respond("too_large", "brain", "observed", nil, map[string]any{}, []string{"The requested fields exceed the compact response limit; narrow the fields or request one immutable reference."}, observedAt, nil)
		return
	}
	if len(missing) != 0 {
		respond("unknown", "brain", "observed", nil, payload, append(baseLimitations, "Missing requested fields: "+strings.Join(missing, ", ")+"."), observedAt, nil)
		return
	}

	digest := sha256.Sum256(raw)
	reference := "brain://" + project + "/context/sha256/" + hex.EncodeToString(digest[:])
	var expiresAt *string
	if strings.EqualFold(strings.TrimSpace(request.CachePolicy), "never") || strings.EqualFold(strings.TrimSpace(request.FreshnessRequired), "current") {
		value := observedAt
		expiresAt = &value
	} else {
		value := "when the Brain project changes"
		expiresAt = &value
	}
	respond("found", "brain", "observed", &reference, payload, baseLimitations, observedAt, expiresAt)
}
