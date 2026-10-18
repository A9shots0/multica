package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

const companySignalBodyLimit = 128 << 10

var (
	companyObjectKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]{0,159}$`)
	companySourcePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,99}$`)
	companyEventPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,239}$`)
	companyDigestPattern    = regexp.MustCompile(`^[a-f0-9]{64}$`)
	companyReferencePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,39}:[A-Za-z0-9][A-Za-z0-9._:/-]{0,199}$`)
)

type companyServiceRequest struct {
	DisplayName    string         `json:"display_name"`
	AreaKey        string         `json:"area_key"`
	Environment    string         `json:"environment"`
	SourceSystem   string         `json:"source_system"`
	SourceNativeID string         `json:"source_native_id"`
	ProjectID      string         `json:"project_id,omitempty"`
	Authority      string         `json:"authority"`
	FreshnessState string         `json:"freshness_state"`
	PrivacyClass   string         `json:"privacy_class"`
	ObservedAt     string         `json:"observed_at"`
	ExpiresAt      string         `json:"expires_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	SystemNodeIDs  []string       `json:"system_node_ids,omitempty"`
	JourneyIDs     []string       `json:"journey_ids,omitempty"`
}

type companyWorkTopologyRequest struct {
	GraphVersion  string   `json:"graph_version"`
	SystemNodeIDs []string `json:"system_node_ids"`
	JourneyIDs    []string `json:"journey_ids"`
	Authority     string   `json:"authority"`
	EvidenceRefs  []string `json:"evidence_refs"`
	ObservedAt    string   `json:"observed_at"`
	ExpiresAt     string   `json:"expires_at,omitempty"`
}

func (h *Handler) PutCompanyService(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, workspaceIDFromURL(r, "id"), "workspace_id")
	if !ok {
		return
	}
	serviceKey := strings.TrimSpace(chi.URLParam(r, "serviceKey"))
	if !companyObjectKeyPattern.MatchString(serviceKey) {
		writeError(w, http.StatusBadRequest, "invalid service key")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	var req companyServiceRequest
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	in, message := parseCompanyServiceRequest(req)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	row, err := h.CompanyOperationsService.UpsertService(r.Context(), workspaceID, serviceKey, in)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			writeError(w, http.StatusBadRequest, "project not found in this workspace")
			return
		}
		if errors.Is(err, service.ErrCompanyServiceTopology) {
			writeError(w, http.StatusBadRequest, "invalid system topology mapping")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save company service")
		return
	}
	writeJSON(w, http.StatusOK, service.ProjectCompanyService(row))
}

func (h *Handler) PutCompanyWorkTopology(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, workspaceIDFromURL(r, "id"), "workspace_id")
	if !ok {
		return
	}
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "issueId"), "issue_id")
	if !ok {
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	var req companyWorkTopologyRequest
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	observedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ObservedAt))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid observed_at")
		return
	}
	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		value, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid expires_at")
			return
		}
		expiresAt = &value
	}
	row, err := h.CompanyOperationsService.UpsertWorkTopology(r.Context(), workspaceID, issueID, service.CompanyWorkTopologyInput{
		GraphVersion: req.GraphVersion, SystemNodeIDs: req.SystemNodeIDs, JourneyIDs: req.JourneyIDs,
		Authority: req.Authority, EvidenceRefs: req.EvidenceRefs, ObservedAt: observedAt, ExpiresAt: expiresAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCompanyWorkIssueNotFound):
			writeError(w, http.StatusNotFound, "issue not found in this workspace")
		case errors.Is(err, service.ErrCompanyWorkTopology):
			writeError(w, http.StatusBadRequest, "invalid work topology mapping")
		default:
			writeError(w, http.StatusInternalServerError, "failed to save work topology mapping")
		}
		return
	}
	writeJSON(w, http.StatusOK, service.ProjectCompanyWorkTopology(row, time.Now().UTC()))
}

func (h *Handler) PostCompanySignal(w http.ResponseWriter, r *http.Request) {
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
			writeError(w, http.StatusUnauthorized, "company signal credential required")
		default:
			writeError(w, http.StatusInternalServerError, "failed to authenticate company signal source")
		}
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, companySignalBodyLimit))
	decoder.DisallowUnknownFields()
	var req service.CompanySignalInput
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if message := validateCompanySignalInput(&req); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	result, err := h.CompanyOperationsService.IngestSignal(r.Context(), binding, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCompanyServiceNotFound):
			writeErrorCode(w, http.StatusUnprocessableEntity, "service_not_cataloged", "service must be cataloged before accepting signals")
		case errors.Is(err, service.ErrCompanySignalServiceBinding):
			writeErrorCode(w, http.StatusUnprocessableEntity, "service_binding_mismatch", "signal does not match the cataloged service binding")
		case errors.Is(err, service.ErrCompanySignalPredecessor):
			writeErrorCode(w, http.StatusUnprocessableEntity, "predecessor_mismatch", "signal does not follow the current source stream")
		case errors.Is(err, service.ErrCompanySignalConflict):
			writeErrorCode(w, http.StatusConflict, "event_id_payload_changed", "event id was already used with different content")
		default:
			slog.Warn("process company signal failed", "workspace_id", util.UUIDToString(workspaceID), "source_key", sourceKey, "event_id", req.EventID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to process company signal")
		}
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (h *Handler) GetCompanyOperatingPicture(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, workspaceIDFromURL(r, "id"), "workspace_id")
	if !ok {
		return
	}
	picture, err := h.CompanyOperationsService.OperatingPicture(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load company operating picture")
		return
	}
	writeJSON(w, http.StatusOK, picture)
}

func parseCompanyServiceRequest(req companyServiceRequest) (service.CompanyServiceInput, string) {
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.AreaKey = strings.TrimSpace(req.AreaKey)
	req.Environment = strings.TrimSpace(req.Environment)
	req.SourceSystem = strings.TrimSpace(req.SourceSystem)
	req.SourceNativeID = strings.TrimSpace(req.SourceNativeID)
	if utf8.RuneCountInString(req.DisplayName) < 1 || utf8.RuneCountInString(req.DisplayName) > 200 {
		return service.CompanyServiceInput{}, "display_name must be between 1 and 200 characters"
	}
	if !companyObjectKeyPattern.MatchString(req.AreaKey) || !companyObjectKeyPattern.MatchString(req.Environment) {
		return service.CompanyServiceInput{}, "invalid area_key or environment"
	}
	if !companySourcePattern.MatchString(req.SourceSystem) || req.SourceNativeID == "" || utf8.RuneCountInString(req.SourceNativeID) > 240 {
		return service.CompanyServiceInput{}, "invalid source identity"
	}
	switch req.Authority {
	case "source", "owner", "cited":
	default:
		return service.CompanyServiceInput{}, "invalid authority"
	}
	switch req.FreshnessState {
	case "current", "stale", "unknown":
	default:
		return service.CompanyServiceInput{}, "invalid freshness_state"
	}
	if !validCompanyPrivacyClass(req.PrivacyClass) {
		return service.CompanyServiceInput{}, "invalid privacy_class"
	}
	observedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ObservedAt))
	if err != nil {
		return service.CompanyServiceInput{}, "invalid observed_at"
	}
	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		value, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
		if err != nil {
			return service.CompanyServiceInput{}, "invalid expires_at"
		}
		expiresAt = &value
	}
	projectID := pgtype.UUID{}
	if strings.TrimSpace(req.ProjectID) != "" {
		parsed, err := util.ParseUUID(strings.TrimSpace(req.ProjectID))
		if err != nil {
			return service.CompanyServiceInput{}, "invalid project_id"
		}
		projectID = parsed
	}
	return service.CompanyServiceInput{
		DisplayName: req.DisplayName, AreaKey: req.AreaKey, Environment: req.Environment,
		SourceSystem: req.SourceSystem, SourceNativeID: req.SourceNativeID, ProjectID: projectID,
		Authority: req.Authority, FreshnessState: req.FreshnessState, PrivacyClass: req.PrivacyClass,
		ObservedAt: observedAt, ExpiresAt: expiresAt, Metadata: req.Metadata,
		SystemNodeIDs: req.SystemNodeIDs, JourneyIDs: req.JourneyIDs,
	}, ""
}

func validateCompanySignalInput(in *service.CompanySignalInput) string {
	in.EventID = strings.TrimSpace(in.EventID)
	in.SourceSystem = strings.TrimSpace(in.SourceSystem)
	in.SourceInstance = strings.TrimSpace(in.SourceInstance)
	in.SourceNativeID = strings.TrimSpace(in.SourceNativeID)
	in.SourceRevision = strings.TrimSpace(in.SourceRevision)
	in.Summary = strings.TrimSpace(in.Summary)
	in.NextStep = strings.TrimSpace(in.NextStep)
	in.ServiceKey = strings.TrimSpace(in.ServiceKey)
	in.Environment = strings.TrimSpace(in.Environment)
	in.PredecessorEventID = strings.TrimSpace(in.PredecessorEventID)
	if in.ContractVersion != 1 {
		return "unsupported contract_version"
	}
	if !companyEventPattern.MatchString(in.EventID) || !companySourcePattern.MatchString(in.SourceSystem) {
		return "invalid event or source identity"
	}
	if !companyObjectKeyPattern.MatchString(in.SourceInstance) || in.SourceNativeID == "" || utf8.RuneCountInString(in.SourceNativeID) > 240 || in.SourceRevision == "" || utf8.RuneCountInString(in.SourceRevision) > 160 {
		return "invalid source stream identity"
	}
	switch in.EventKind {
	case "availability", "application_log", "support_report", "deployment", "verification":
	default:
		return "invalid event_kind"
	}
	switch in.SourceLane {
	case "availability", "application_logs", "support_impact", "deployment", "verification":
	default:
		return "invalid source_lane"
	}
	switch in.LifecycleState {
	case "new", "worsened", "needs_human", "recovering", "recovered", "suppressed", "retired":
	default:
		return "invalid lifecycle_state"
	}
	switch in.Severity {
	case "low", "medium", "high", "critical":
	default:
		return "invalid severity"
	}
	if utf8.RuneCountInString(in.Summary) < 1 || utf8.RuneCountInString(in.Summary) > 1000 || utf8.RuneCountInString(in.NextStep) < 1 || utf8.RuneCountInString(in.NextStep) > 500 {
		return "invalid summary or next_step"
	}
	if !companyObjectKeyPattern.MatchString(in.ServiceKey) || !companyObjectKeyPattern.MatchString(in.Environment) {
		return "invalid service_ref or environment_ref"
	}
	if !companyDigestPattern.MatchString(in.RawPayloadHash) || !companyDigestPattern.MatchString(in.CorrelationKey) {
		return "invalid payload or correlation hash"
	}
	if in.IncidentGeneration < 1 || in.ObservedAt.IsZero() {
		return "incident_generation and observed_at are required"
	}
	if !validCompanyPrivacyClass(in.PrivacyClass) {
		return "invalid privacy_class"
	}
	if len(in.EvidenceRefs) < 1 || len(in.EvidenceRefs) > 50 || len(in.AffectedRefs) > 100 {
		return "invalid evidence_refs or affected_refs count"
	}
	for _, values := range [][]string{in.EvidenceRefs, in.AffectedRefs} {
		seen := map[string]struct{}{}
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
			if !companyReferencePattern.MatchString(values[i]) {
				return "invalid evidence or affected reference"
			}
			if _, exists := seen[values[i]]; exists {
				return "evidence and affected references must be unique"
			}
			seen[values[i]] = struct{}{}
		}
	}
	if in.PredecessorEventID != "" && (!companyEventPattern.MatchString(in.PredecessorEventID) || in.PredecessorEventID == in.EventID) {
		return "invalid predecessor_event_id"
	}
	in.ObservedAt = in.ObservedAt.UTC()
	return ""
}

func validCompanyPrivacyClass(value string) bool {
	return value == "internal" || value == "confidential" || value == "restricted"
}
