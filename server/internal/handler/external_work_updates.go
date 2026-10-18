package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

const (
	externalWorkUpdateLimit     = 20
	externalWorkUpdateListLimit = 8
	externalWorkUpdateTextRunes = 2000
	externalWorkUpdateSchema    = 1
)

type externalWorkUpdateEvidence struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
}

type externalWorkUpdateResourceUsage struct {
	Source               string   `json:"source"`
	Attempts             *int64   `json:"attempts"`
	WallClockSeconds     *int64   `json:"wall_clock_seconds"`
	ActiveComputeSeconds *int64   `json:"active_compute_seconds"`
	InputTokens          *int64   `json:"input_tokens"`
	OutputTokens         *int64   `json:"output_tokens"`
	CostUSD              *float64 `json:"cost_usd"`
	CapacityWindowRef    *string  `json:"capacity_window_ref"`
}

type externalWorkUpdate struct {
	ID                 string                          `json:"id"`
	Provider           string                          `json:"provider"`
	ReportedState      string                          `json:"reported_state"`
	ObservedAt         string                          `json:"observed_at"`
	RecordedAt         string                          `json:"recorded_at"`
	ReceiptRevision    int64                           `json:"receipt_revision"`
	CurrentMilestone   string                          `json:"current_milestone"`
	Summary            string                          `json:"summary"`
	Evidence           []externalWorkUpdateEvidence    `json:"evidence"`
	EvidenceTotal      int                             `json:"evidence_total"`
	Blockers           []string                        `json:"blockers"`
	BlockerTotal       int                             `json:"blocker_total"`
	NextAction         string                          `json:"next_action"`
	AcceptanceStage    string                          `json:"acceptance_stage"`
	AcceptanceStatus   string                          `json:"acceptance_status"`
	ResourceUsage      externalWorkUpdateResourceUsage `json:"resource_usage"`
	ProviderReportOnly bool                            `json:"provider_report_only"`
}

type externalWorkUpdatesResponse struct {
	SchemaVersion int                  `json:"schema_version"`
	IssueID       string               `json:"issue_id"`
	Items         []externalWorkUpdate `json:"items"`
}

func externalWorkDisplayText(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= externalWorkUpdateTextRunes {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:externalWorkUpdateTextRunes])) + "…"
}

func externalWorkUpdateFromRow(row db.IssueExternalWorkReceipt) (externalWorkUpdate, bool) {
	if !row.ID.Valid || !row.AdmittedAt.Valid || row.ContractVersion != publicapiv1.OutcomeReceiptContractVersion ||
		!row.EvidenceID.Valid || !row.ExecutionID.Valid {
		return externalWorkUpdate{}, false
	}
	var receipt publicapiv1.OutcomeReceiptV02
	if err := json.Unmarshal(row.RawReceipt, &receipt); err != nil || receipt.Validate() != nil || receipt.Provider != row.Provider ||
		receipt.WorkID != row.WorkID || receipt.RunID != row.RunID || receipt.ReceiptRevision != row.ReceiptRevision {
		return externalWorkUpdate{}, false
	}
	evidenceLimit := min(len(receipt.Evidence), externalWorkUpdateListLimit)
	evidence := make([]externalWorkUpdateEvidence, 0, evidenceLimit)
	for _, item := range receipt.Evidence[:evidenceLimit] {
		evidence = append(evidence, externalWorkUpdateEvidence{Type: item.Type, Reference: externalWorkDisplayText(item.Reference)})
	}
	blockerLimit := min(len(receipt.Dependencies), externalWorkUpdateListLimit)
	blockers := make([]string, 0, blockerLimit)
	for _, item := range receipt.Dependencies[:blockerLimit] {
		blockers = append(blockers, externalWorkDisplayText(item))
	}
	var capacityWindowRef *string
	if receipt.ResourceUsage.CapacityWindowRef != nil {
		value := externalWorkDisplayText(*receipt.ResourceUsage.CapacityWindowRef)
		capacityWindowRef = &value
	}
	return externalWorkUpdate{
		ID: uuidToString(row.ID), Provider: receipt.Provider, ReportedState: receipt.State,
		ObservedAt: receipt.Freshness.ObservedAt, RecordedAt: row.AdmittedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		ReceiptRevision: receipt.ReceiptRevision, CurrentMilestone: externalWorkDisplayText(receipt.CurrentMilestone),
		Summary: externalWorkDisplayText(receipt.ChangeSinceLastReceipt), Evidence: evidence, EvidenceTotal: len(receipt.Evidence),
		Blockers: blockers, BlockerTotal: len(receipt.Dependencies), NextAction: externalWorkDisplayText(receipt.NextAction),
		AcceptanceStage: receipt.Acceptance.Stage, AcceptanceStatus: receipt.Acceptance.Status,
		ResourceUsage: externalWorkUpdateResourceUsage{
			Source: receipt.ResourceUsage.Source, Attempts: receipt.ResourceUsage.Attempts,
			WallClockSeconds: receipt.ResourceUsage.WallClockSeconds, ActiveComputeSeconds: receipt.ResourceUsage.ActiveComputeSeconds,
			InputTokens: receipt.ResourceUsage.InputTokens, OutputTokens: receipt.ResourceUsage.OutputTokens,
			CostUSD: receipt.ResourceUsage.CostUSD, CapacityWindowRef: capacityWindowRef,
		},
		ProviderReportOnly: true,
	}, true
}

// GetIssueExternalWorkUpdates exposes a bounded, owner-readable view of
// admitted provider reports. It never turns a provider report into native
// acceptance or returns credentials, authority envelopes, or raw payloads.
func (h *Handler) GetIssueExternalWorkUpdates(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListExternalWorkReceiptsForIssue(r.Context(), db.ListExternalWorkReceiptsForIssueParams{
		WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, PageSize: externalWorkUpdateLimit,
	})
	if err != nil {
		slog.Warn("load external work updates failed", append(logger.RequestAttrs(r), "issue_id", uuidToString(issue.ID), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to load outside work updates")
		return
	}
	items := make([]externalWorkUpdate, 0, len(rows))
	for _, row := range rows {
		item, valid := externalWorkUpdateFromRow(row)
		if !valid {
			slog.Warn("skipping malformed external work update", append(logger.RequestAttrs(r), "issue_id", uuidToString(issue.ID), "receipt_id", uuidToString(row.ID))...)
			continue
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, externalWorkUpdatesResponse{
		SchemaVersion: externalWorkUpdateSchema, IssueID: uuidToString(issue.ID), Items: items,
	})
}
