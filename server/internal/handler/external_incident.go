package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

const externalIncidentBodyLimit = 256 << 10

var (
	externalIncidentSourcePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,99}$`)
	externalIncidentEventPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,239}$`)
	externalIncidentDigestPattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
	externalIncidentEvidencePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,39}:[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)
)

type externalIncidentSourceBindingRequest struct {
	ProjectID        string `json:"project_id"`
	CreatorAgentID   string `json:"creator_agent_id"`
	Enabled          bool   `json:"enabled"`
	RotateCredential bool   `json:"rotate_credential,omitempty"`
	ExpectedRevision *int64 `json:"expected_revision,omitempty"`
}

type externalIncidentSourceBindingResponse struct {
	WorkspaceID     string `json:"workspace_id"`
	SourceKey       string `json:"source_key"`
	CreatorAgentID  string `json:"creator_agent_id"`
	ProjectID       string `json:"project_id"`
	Enabled         bool   `json:"enabled"`
	BindingRevision int64  `json:"binding_revision"`
	CredentialHint  string `json:"credential_hint"`
	Credential      string `json:"credential,omitempty"`
}

func (h *Handler) PutExternalIncidentSourceBinding(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, workspaceIDFromURL(r, "id"), "workspace_id")
	if !ok {
		return
	}
	sourceKey := strings.TrimSpace(chi.URLParam(r, "sourceKey"))
	if !externalIncidentSourcePattern.MatchString(sourceKey) {
		writeError(w, http.StatusBadRequest, "invalid source key")
		return
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	var req externalIncidentSourceBindingRequest
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	projectID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.ProjectID), "project_id")
	if !ok {
		return
	}
	creatorAgentID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.CreatorAgentID), "creator_agent_id")
	if !ok {
		return
	}
	configured, err := h.ExternalIncidentService.ConfigureSource(r.Context(), workspaceID, sourceKey, service.ExternalIncidentSourceConfig{
		ProjectID: projectID, CreatorAgentID: creatorAgentID, Enabled: req.Enabled,
		RotateCredential: req.RotateCredential, ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			writeError(w, http.StatusBadRequest, "project not found in this workspace")
			return
		}
		if errors.Is(err, service.ErrExternalIncidentCreatorAgent) {
			writeError(w, http.StatusBadRequest, "creator agent not found in this workspace")
			return
		}
		if errors.Is(err, service.ErrExternalIncidentBindingRevision) {
			writeErrorCode(w, http.StatusConflict, "binding_revision_conflict", "source binding changed; reload before updating or rotating it")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save source binding")
		return
	}
	binding := configured.Binding
	writeJSON(w, http.StatusOK, externalIncidentSourceBindingResponse{
		WorkspaceID: util.UUIDToString(binding.WorkspaceID), SourceKey: binding.SourceKey,
		CreatorAgentID: util.UUIDToString(binding.CreatorAgentID), ProjectID: util.UUIDToString(binding.ProjectID),
		Enabled: binding.Enabled, BindingRevision: binding.Revision, CredentialHint: binding.CredentialHint,
		Credential: configured.Credential,
	})
}

func (h *Handler) PostExternalIncidentOccurrence(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, workspaceIDFromURL(r, "id"), "workspace_id")
	if !ok {
		return
	}
	sourceKey := strings.TrimSpace(chi.URLParam(r, "sourceKey"))
	if !externalIncidentSourcePattern.MatchString(sourceKey) {
		writeError(w, http.StatusBadRequest, "invalid source key")
		return
	}
	binding, err := h.ExternalIncidentService.AuthenticateSource(r.Context(), workspaceID, sourceKey, middleware.BearerToken(r))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrExternalIncidentSourceDisabled):
			writeError(w, http.StatusForbidden, "source binding is disabled")
		case errors.Is(err, service.ErrExternalIncidentCredential):
			writeError(w, http.StatusUnauthorized, "external incident credential required")
		default:
			writeError(w, http.StatusInternalServerError, "failed to authenticate incident source")
		}
		return
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, externalIncidentBodyLimit))
	decoder.DisallowUnknownFields()
	var req service.ExternalIncidentInput
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if message := validateExternalIncidentInput(&req); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}

	result, err := h.ExternalIncidentService.Ingest(r.Context(), binding, req)
	if err != nil {
		var rejected *service.ExternalIncidentRejectedError
		switch {
		case errors.Is(err, service.ErrExternalIncidentSourceNotFound):
			writeError(w, http.StatusNotFound, "source binding not found")
		case errors.Is(err, service.ErrExternalIncidentSourceDisabled):
			writeError(w, http.StatusForbidden, "source binding is disabled")
		case errors.Is(err, service.ErrExternalIncidentConflict):
			writeErrorCode(w, http.StatusConflict, "event_key_payload_changed", "event key was already used with different content")
		case errors.Is(err, service.ErrExternalIncidentProcessing):
			writeErrorCode(w, http.StatusConflict, "event_processing", "event is already processing")
		case errors.As(err, &rejected):
			writeErrorCode(w, http.StatusUnprocessableEntity, rejected.Code, "event was rejected by the incident lifecycle")
		default:
			writeError(w, http.StatusInternalServerError, "failed to process incident occurrence")
		}
		return
	}
	status := http.StatusCreated
	if result.ReceiptState == "pending_predecessor" {
		status = http.StatusAccepted
	} else if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func validateExternalIncidentInput(in *service.ExternalIncidentInput) string {
	in.EventKey = strings.TrimSpace(in.EventKey)
	in.FindingKey = strings.TrimSpace(in.FindingKey)
	in.IncidentKey = strings.TrimSpace(in.IncidentKey)
	in.PredecessorEventKey = strings.TrimSpace(in.PredecessorEventKey)
	in.Summary = strings.TrimSpace(in.Summary)
	in.NextStep = strings.TrimSpace(in.NextStep)
	if !externalIncidentEventPattern.MatchString(in.EventKey) {
		return "invalid event_key"
	}
	if !externalIncidentDigestPattern.MatchString(in.FindingKey) {
		return "invalid finding_key"
	}
	if !externalIncidentDigestPattern.MatchString(in.IncidentKey) {
		return "invalid incident_key"
	}
	if in.SourceSequence < 1 {
		return "source_sequence must be positive"
	}
	if in.PredecessorEventKey != "" && !externalIncidentEventPattern.MatchString(in.PredecessorEventKey) {
		return "invalid predecessor_event_key"
	}
	if in.PredecessorEventKey == in.EventKey {
		return "predecessor_event_key must differ from event_key"
	}
	if in.EpisodeGeneration < 1 {
		return "episode_generation must be positive"
	}
	switch in.Transition {
	case "new", "worsened", "needs_human", "recovering", "recovered", "suppressed", "retired":
	default:
		return "invalid transition"
	}
	switch in.Severity {
	case "low", "medium", "high", "critical":
	default:
		return "invalid severity"
	}
	if in.ActionRequired == nil {
		return "action_required is required"
	}
	if utf8.RuneCountInString(in.Summary) < 1 || utf8.RuneCountInString(in.Summary) > 1000 {
		return "summary must be between 1 and 1000 characters"
	}
	if utf8.RuneCountInString(in.NextStep) < 1 || utf8.RuneCountInString(in.NextStep) > 500 {
		return "next_step must be between 1 and 500 characters"
	}
	if in.ObservedAt.IsZero() {
		return "observed_at is required"
	}
	if len(in.EvidenceRefs) < 1 || len(in.EvidenceRefs) > 50 {
		return "evidence_refs must contain between 1 and 50 values"
	}
	seenEvidence := make(map[string]struct{}, len(in.EvidenceRefs))
	for i := range in.EvidenceRefs {
		in.EvidenceRefs[i] = strings.TrimSpace(in.EvidenceRefs[i])
		if utf8.RuneCountInString(in.EvidenceRefs[i]) > 240 || !externalIncidentEvidencePattern.MatchString(in.EvidenceRefs[i]) {
			return "invalid evidence_refs value"
		}
		if _, exists := seenEvidence[in.EvidenceRefs[i]]; exists {
			return "evidence_refs must be unique"
		}
		seenEvidence[in.EvidenceRefs[i]] = struct{}{}
	}
	in.ObservedAt = in.ObservedAt.UTC()
	return ""
}
