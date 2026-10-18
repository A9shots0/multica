package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	currentPictureContractVersionV2 = 2
	currentPictureContractVersionV3 = 3
)

// issueCurrentPictureCompactResponse is the bounded projection embedded in
// issue list rows. State values deliberately remain strings so newer servers
// can add states without breaking installed clients.
type issueCurrentPictureCompactResponse struct {
	ContractVersion        int32                                `json:"contract_version"`
	OwnerSummary           workflowprojection.OwnerIssueSummary `json:"owner_summary"`
	ReleaseStatus          *workflowprojection.ReleaseStatus    `json:"release_status,omitempty"`
	Revision               int64                                `json:"revision"`
	ScopeRevision          *int64                               `json:"scope_revision"`
	AuthorityRevision      *int64                               `json:"authority_revision"`
	OutcomeState           string                               `json:"outcome_state"`
	OutcomeOpen            bool                                 `json:"outcome_open"`
	ExecutionState         string                               `json:"execution_state"`
	ReleaseState           string                               `json:"release_state"`
	HealthState            string                               `json:"health_state"`
	AttentionState         string                               `json:"attention_state"`
	CurrentMilestone       string                               `json:"current_milestone"`
	LatestMeaningfulChange json.RawMessage                      `json:"latest_meaningful_change"`
	NextAutomaticAction    json.RawMessage                      `json:"next_automatic_action"`
	ActiveActor            json.RawMessage                      `json:"active_actor"`
	WaitReason             json.RawMessage                      `json:"wait_reason"`
	Candidate              json.RawMessage                      `json:"candidate"`
	Target                 json.RawMessage                      `json:"target"`
	EvidenceSummary        json.RawMessage                      `json:"evidence_summary"`
	UnresolvedDecisions    json.RawMessage                      `json:"unresolved_decisions"`
	UnresolvedIncidents    json.RawMessage                      `json:"unresolved_incidents"`
	UnresolvedDependencies json.RawMessage                      `json:"unresolved_dependencies"`
	TerminalReason         string                               `json:"terminal_reason"`
	NextActor              json.RawMessage                      `json:"next_actor"`
	GeneratedAt            string                               `json:"generated_at"`
}

type issueCurrentPictureResponse struct {
	issueCurrentPictureCompactResponse
	WorkspaceID         string          `json:"workspace_id"`
	IssueID             string          `json:"issue_id"`
	AuthoritySummary    json.RawMessage `json:"authority_summary"`
	BudgetSummary       json.RawMessage `json:"budget_summary"`
	SupersessionSummary json.RawMessage `json:"supersession_summary"`
	SourceRevisions     json.RawMessage `json:"source_revisions"`
	Document            json.RawMessage `json:"document"`
	UpdatedAt           string          `json:"updated_at"`
}

func decodeCurrentPictureObject(value []byte) (map[string]any, bool) {
	var decoded map[string]any
	if json.Unmarshal(value, &decoded) != nil || decoded == nil {
		return nil, false
	}
	return decoded, true
}

func decodeCurrentPictureArray(value []byte) ([]map[string]any, bool) {
	var decoded []map[string]any
	if json.Unmarshal(value, &decoded) != nil || decoded == nil {
		return nil, false
	}
	return decoded, true
}

func currentPictureState(object map[string]any, fallback string) (string, bool) {
	return currentPictureStringField(object, "state", fallback)
}

func currentPictureStringField(object map[string]any, field, fallback string) (string, bool) {
	value, exists := object[field]
	if !exists {
		return fallback, true
	}
	state, ok := value.(string)
	return state, ok && strings.TrimSpace(state) != ""
}

func currentPictureRevision(object map[string]any) (*int64, bool) {
	value, exists := object["revision"]
	if !exists {
		return nil, true
	}
	number, ok := value.(float64)
	if !ok || number < 1 || number != float64(int64(number)) {
		return nil, false
	}
	revision := int64(number)
	return &revision, true
}

type currentPictureSemantic struct {
	LatestMeaningfulChange map[string]any
	NextAutomaticAction    map[string]any
	ActiveActor            map[string]any
	WaitReason             map[string]any
	Candidate              map[string]any
	Target                 map[string]any
	EvidenceSummary        map[string]any
	UnresolvedDecisions    []map[string]any
	UnresolvedIncidents    []map[string]any
	UnresolvedDependencies []map[string]any
	NextActor              map[string]any
}

func currentPictureSemanticFields(row db.IssueCurrentPicture) (currentPictureSemantic, bool) {
	var semantic currentPictureSemantic
	var ok bool
	objects := []struct {
		raw  []byte
		dest *map[string]any
	}{
		{row.LatestMeaningfulChange, &semantic.LatestMeaningfulChange},
		{row.NextAutomaticAction, &semantic.NextAutomaticAction},
		{row.ActiveActor, &semantic.ActiveActor}, {row.WaitReason, &semantic.WaitReason},
		{row.Candidate, &semantic.Candidate}, {row.Target, &semantic.Target},
		{row.EvidenceSummary, &semantic.EvidenceSummary}, {row.NextActor, &semantic.NextActor},
	}
	for _, object := range objects {
		*object.dest, ok = decodeCurrentPictureObject(object.raw)
		if !ok {
			return currentPictureSemantic{}, false
		}
	}
	arrays := []struct {
		raw  []byte
		dest *[]map[string]any
	}{
		{row.UnresolvedDecisions, &semantic.UnresolvedDecisions},
		{row.UnresolvedIncidents, &semantic.UnresolvedIncidents},
		{row.UnresolvedDependencies, &semantic.UnresolvedDependencies},
	}
	for _, array := range arrays {
		*array.dest, ok = decodeCurrentPictureArray(array.raw)
		if !ok {
			return currentPictureSemantic{}, false
		}
	}
	return semantic, true
}

type currentPictureDocumentV2 struct {
	ContractVersion           int32                             `json:"contract_version"`
	OwnerSummarySchemaVersion int32                             `json:"owner_summary_schema_version"`
	ReleaseStatus             *workflowprojection.ReleaseStatus `json:"release_status"`
	OriginalBusinessRequest   map[string]any                    `json:"original_business_request"`
	Outcome                   map[string]any                    `json:"outcome"`
	HistoricalCompletion      map[string]any                    `json:"historical_completion"`
	Scope                     map[string]any                    `json:"scope"`
	Authority                 map[string]any                    `json:"authority"`
	Execution                 map[string]any                    `json:"execution"`
	ActiveBranches            []map[string]any                  `json:"active_branches"`
	PrimaryBranch             map[string]any                    `json:"primary_branch"`
	Release                   map[string]any                    `json:"release"`
	Health                    string                            `json:"health"`
	Attention                 map[string]any                    `json:"attention"`
	CurrentMilestone          string                            `json:"current_milestone"`
	LatestMeaningfulChange    map[string]any                    `json:"latest_meaningful_change"`
	NextAutomaticAction       map[string]any                    `json:"next_automatic_action"`
	ActiveActor               map[string]any                    `json:"active_actor"`
	WaitReason                map[string]any                    `json:"wait_reason"`
	Candidate                 map[string]any                    `json:"candidate"`
	Target                    map[string]any                    `json:"target"`
	EvidenceSummary           map[string]any                    `json:"evidence_summary"`
	AuthoritySummary          map[string]any                    `json:"authority_summary"`
	UnresolvedDecisions       []map[string]any                  `json:"unresolved_decisions"`
	UnresolvedIncidents       []map[string]any                  `json:"unresolved_incidents"`
	UnresolvedDependencies    []map[string]any                  `json:"unresolved_dependencies"`
	Budgets                   []map[string]any                  `json:"budgets"`
	Supersession              map[string]any                    `json:"supersession"`
	TerminalReason            string                            `json:"terminal_reason"`
	NextActor                 map[string]any                    `json:"next_actor"`
}

func validCurrentPictureDetailJSON(row db.IssueCurrentPicture, semantic currentPictureSemantic) bool {
	authoritySummary, authorityOK := decodeCurrentPictureObject(row.AuthoritySummary)
	budgetSummary, budgetOK := decodeCurrentPictureArray(row.BudgetSummary)
	supersession, supersessionOK := decodeCurrentPictureObject(row.SupersessionSummary)
	var sources []struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
	}
	if json.Unmarshal(row.SourceRevisions, &sources) != nil || sources == nil {
		return false
	}
	for _, source := range sources {
		if strings.TrimSpace(source.Type) == "" || strings.TrimSpace(source.ID) == "" || source.Revision < 1 {
			return false
		}
	}
	var document currentPictureDocumentV2
	if !authorityOK || !budgetOK || !supersessionOK || json.Unmarshal(row.Document, &document) != nil ||
		(document.ContractVersion != currentPictureContractVersionV2 && document.ContractVersion != currentPictureContractVersionV3) ||
		document.ContractVersion != row.ContractVersion || document.OriginalBusinessRequest == nil ||
		document.Outcome == nil || document.HistoricalCompletion == nil || document.Scope == nil ||
		document.Authority == nil || document.Execution == nil || document.ActiveBranches == nil ||
		document.PrimaryBranch == nil || document.Release == nil || document.Attention == nil ||
		document.LatestMeaningfulChange == nil || document.NextAutomaticAction == nil ||
		document.ActiveActor == nil || document.WaitReason == nil || document.Candidate == nil ||
		document.Target == nil || document.EvidenceSummary == nil || document.AuthoritySummary == nil ||
		document.UnresolvedDecisions == nil || document.UnresolvedIncidents == nil ||
		document.UnresolvedDependencies == nil || document.Budgets == nil || document.Supersession == nil ||
		document.NextActor == nil || strings.TrimSpace(document.Health) == "" ||
		strings.TrimSpace(document.CurrentMilestone) == "" {
		return false
	}
	if (document.ContractVersion == currentPictureContractVersionV3) != (document.ReleaseStatus != nil) {
		return false
	}
	if document.ContractVersion == currentPictureContractVersionV3 && !workflowprojection.ValidReleaseStatusDocument(row.Document) {
		return false
	}
	outcomeState, outcomeOK := currentPictureState(document.Outcome, "open")
	executionState, executionOK := currentPictureState(document.Execution, "received")
	releaseState, releaseOK := currentPictureState(document.Release, "pending")
	attentionState, attentionOK := currentPictureStringField(document.Attention, "resolution_state", "resolved")
	scopeRevision, scopeOK := currentPictureRevision(document.Scope)
	authorityRevision, authorityRevisionOK := currentPictureRevision(document.Authority)
	if !outcomeOK || !executionOK || !releaseOK || !attentionOK || !scopeOK || !authorityRevisionOK ||
		outcomeState != row.OutcomeState || (outcomeState == "open") != row.OutcomeOpen ||
		executionState != row.ExecutionState || releaseState != row.ReleaseState ||
		document.Health != row.HealthState || attentionState != row.AttentionState ||
		document.CurrentMilestone != row.CurrentMilestone || document.TerminalReason != row.TerminalReason ||
		!reflect.DeepEqual(scopeRevision, int8ToPtr(row.ScopeRevision)) ||
		!reflect.DeepEqual(authorityRevision, int8ToPtr(row.AuthorityRevision)) {
		return false
	}
	return reflect.DeepEqual(document.LatestMeaningfulChange, semantic.LatestMeaningfulChange) &&
		reflect.DeepEqual(document.NextAutomaticAction, semantic.NextAutomaticAction) &&
		reflect.DeepEqual(document.ActiveActor, semantic.ActiveActor) &&
		reflect.DeepEqual(document.WaitReason, semantic.WaitReason) &&
		reflect.DeepEqual(document.Candidate, semantic.Candidate) &&
		reflect.DeepEqual(document.Target, semantic.Target) &&
		reflect.DeepEqual(document.EvidenceSummary, semantic.EvidenceSummary) &&
		reflect.DeepEqual(document.AuthoritySummary, authoritySummary) &&
		reflect.DeepEqual(document.UnresolvedDecisions, semantic.UnresolvedDecisions) &&
		reflect.DeepEqual(document.UnresolvedIncidents, semantic.UnresolvedIncidents) &&
		reflect.DeepEqual(document.UnresolvedDependencies, semantic.UnresolvedDependencies) &&
		reflect.DeepEqual(document.Budgets, budgetSummary) &&
		reflect.DeepEqual(document.Supersession, supersession) &&
		reflect.DeepEqual(document.NextActor, semantic.NextActor)
}

func currentPictureCompactFromBatch(row db.ListIssueCurrentPicturesByIssueIDsRow) (*issueCurrentPictureCompactResponse, bool) {
	return currentPictureCompact(db.IssueCurrentPicture{
		WorkspaceID: row.WorkspaceID, IssueID: row.IssueID,
		ContractVersion: row.ContractVersion, Revision: row.Revision,
		ScopeRevision: row.ScopeRevision, AuthorityRevision: row.AuthorityRevision,
		OutcomeState: row.OutcomeState, OutcomeOpen: row.OutcomeOpen,
		ExecutionState: row.ExecutionState, ReleaseState: row.ReleaseState,
		HealthState: row.HealthState, AttentionState: row.AttentionState,
		CurrentMilestone:       row.CurrentMilestone,
		LatestMeaningfulChange: row.LatestMeaningfulChange,
		NextAutomaticAction:    row.NextAutomaticAction, ActiveActor: row.ActiveActor,
		WaitReason: row.WaitReason, Candidate: row.Candidate, Target: row.Target,
		EvidenceSummary: row.EvidenceSummary, UnresolvedDecisions: row.UnresolvedDecisions,
		UnresolvedIncidents: row.UnresolvedIncidents, UnresolvedDependencies: row.UnresolvedDependencies,
		TerminalReason: row.TerminalReason, NextActor: row.NextActor, Document: row.Document, GeneratedAt: row.GeneratedAt,
	})
}

func currentPictureCompact(row db.IssueCurrentPicture) (*issueCurrentPictureCompactResponse, bool) {
	_, semanticOK := currentPictureSemanticFields(row)
	if (row.ContractVersion != currentPictureContractVersionV2 && row.ContractVersion != currentPictureContractVersionV3) || row.Revision < 1 ||
		!row.GeneratedAt.Valid || !semanticOK || strings.TrimSpace(row.OutcomeState) == "" ||
		strings.TrimSpace(row.ExecutionState) == "" || strings.TrimSpace(row.ReleaseState) == "" ||
		strings.TrimSpace(row.HealthState) == "" || strings.TrimSpace(row.AttentionState) == "" ||
		strings.TrimSpace(row.CurrentMilestone) == "" {
		return nil, false
	}
	var envelope struct {
		ReleaseStatus *workflowprojection.ReleaseStatus `json:"release_status"`
	}
	if json.Unmarshal(row.Document, &envelope) != nil ||
		(row.ContractVersion == currentPictureContractVersionV3) != (envelope.ReleaseStatus != nil) {
		return nil, false
	}
	if row.ContractVersion == currentPictureContractVersionV3 && !workflowprojection.ValidReleaseStatusDocument(row.Document) {
		return nil, false
	}
	return &issueCurrentPictureCompactResponse{
		ContractVersion: row.ContractVersion, Revision: row.Revision,
		OwnerSummary:  workflowprojection.OwnerIssueSummaryFromDocument(row.Document, row.Revision, row.GeneratedAt.Time),
		ReleaseStatus: envelope.ReleaseStatus,
		ScopeRevision: int8ToPtr(row.ScopeRevision), AuthorityRevision: int8ToPtr(row.AuthorityRevision),
		OutcomeState: row.OutcomeState, OutcomeOpen: row.OutcomeOpen,
		ExecutionState: row.ExecutionState, ReleaseState: row.ReleaseState,
		HealthState: row.HealthState, AttentionState: row.AttentionState,
		CurrentMilestone:       row.CurrentMilestone,
		LatestMeaningfulChange: json.RawMessage(row.LatestMeaningfulChange),
		NextAutomaticAction:    json.RawMessage(row.NextAutomaticAction),
		ActiveActor:            json.RawMessage(row.ActiveActor), WaitReason: json.RawMessage(row.WaitReason),
		Candidate: json.RawMessage(row.Candidate), Target: json.RawMessage(row.Target),
		EvidenceSummary:        json.RawMessage(row.EvidenceSummary),
		UnresolvedDecisions:    json.RawMessage(row.UnresolvedDecisions),
		UnresolvedIncidents:    json.RawMessage(row.UnresolvedIncidents),
		UnresolvedDependencies: json.RawMessage(row.UnresolvedDependencies),
		TerminalReason:         row.TerminalReason, NextActor: json.RawMessage(row.NextActor),
		GeneratedAt: timestampToString(row.GeneratedAt),
	}, true
}

// currentPicturesByIssue performs exactly one workspace-scoped read for a
// response page. A query failure or malformed optional row drops enrichment;
// it never turns an otherwise usable issue list into an error.
func (h *Handler) currentPicturesByIssue(ctx context.Context, workspaceID pgtype.UUID, issueIDs []pgtype.UUID) map[string]*issueCurrentPictureCompactResponse {
	result := make(map[string]*issueCurrentPictureCompactResponse)
	poisoned := make(map[string]bool)
	if len(issueIDs) == 0 {
		return result
	}
	rows, err := h.Queries.ListIssueCurrentPicturesByIssueIDs(ctx, db.ListIssueCurrentPicturesByIssueIDsParams{
		WorkspaceID: workspaceID,
		IssueIds:    issueIDs,
	})
	if err != nil {
		slog.Warn("load compact Current Pictures failed", "error", err)
		return result
	}
	for _, row := range rows {
		key := uuidToString(row.IssueID)
		if poisoned[key] {
			continue
		}
		if _, duplicate := result[key]; duplicate {
			delete(result, key)
			poisoned[key] = true
			slog.Error("refusing duplicate compact Current Picture", "issue_id", key)
			continue
		}
		compact, ok := currentPictureCompactFromBatch(row)
		if !ok {
			slog.Warn("dropping malformed compact Current Picture", "issue_id", uuidToString(row.IssueID))
			continue
		}
		result[key] = compact
	}
	return result
}

// GetIssueCurrentPicture returns the versioned, detailed projection only after
// resolving the issue through the normal membership-aware loader. Missing and
// cross-workspace projections are intentionally indistinguishable.
func (h *Handler) GetIssueCurrentPicture(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	row, err := h.Queries.GetIssueCurrentPicture(r.Context(), db.GetIssueCurrentPictureParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	if err != nil {
		slog.Warn("load Current Picture failed", append(logger.RequestAttrs(r), "issue_id", uuidToString(issue.ID), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to load issue current picture")
		return
	}
	if row.WorkspaceID != issue.WorkspaceID || row.IssueID != issue.ID {
		slog.Error("refusing cross-identity Current Picture", append(logger.RequestAttrs(r), "issue_id", uuidToString(issue.ID))...)
		writeError(w, http.StatusInternalServerError, "issue current picture is unavailable")
		return
	}
	semantic, semanticOK := currentPictureSemanticFields(row)
	compact, valid := currentPictureCompact(row)
	if !valid || !semanticOK || !validCurrentPictureDetailJSON(row, semantic) {
		slog.Error("refusing corrupt Current Picture", append(logger.RequestAttrs(r), "issue_id", uuidToString(issue.ID), "contract_version", row.ContractVersion)...)
		writeError(w, http.StatusInternalServerError, "issue current picture is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, issueCurrentPictureResponse{
		issueCurrentPictureCompactResponse: *compact,
		WorkspaceID:                        uuidToString(row.WorkspaceID), IssueID: uuidToString(row.IssueID),
		AuthoritySummary:    json.RawMessage(row.AuthoritySummary),
		BudgetSummary:       json.RawMessage(row.BudgetSummary),
		SupersessionSummary: json.RawMessage(row.SupersessionSummary),
		SourceRevisions:     json.RawMessage(row.SourceRevisions),
		Document:            json.RawMessage(row.Document), UpdatedAt: timestampToString(row.UpdatedAt),
	})
}
