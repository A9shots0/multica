package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ownerNotificationReceiptResponse struct {
	ID            string  `json:"id"`
	EventType     string  `json:"event_type"`
	DeliveryMode  string  `json:"delivery_mode"`
	State         string  `json:"state"`
	ProviderState *string `json:"provider_state"`
	AdmittedAt    string  `json:"admitted_at"`
	DeliveredAt   *string `json:"delivered_at"`
}

// ListIssueOwnerNotifications exposes bounded delivery truth for the secure
// owner control panel. It deliberately omits the destination, message body,
// provider identifier, and credentials.
func (h *Handler) ListIssueOwnerNotifications(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListOwnerNotificationsByIssue(r.Context(), db.ListOwnerNotificationsByIssueParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, []ownerNotificationReceiptResponse{})
		return
	}
	if err != nil {
		slog.Warn("list owner notification receipts failed", append(logger.RequestAttrs(r), "issue_id", uuidToString(issue.ID), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to load notification receipts")
		return
	}
	result := make([]ownerNotificationReceiptResponse, 0, len(rows))
	for _, row := range rows {
		item := ownerNotificationReceiptResponse{
			ID: uuidToString(row.ID), EventType: row.EventType, DeliveryMode: row.DeliveryMode,
			State: row.State, AdmittedAt: timestampToString(row.AdmittedAt),
		}
		if row.ProviderState.Valid {
			item.ProviderState = &row.ProviderState.String
		}
		if row.DeliveredAt.Valid {
			value := timestampToString(row.DeliveredAt)
			item.DeliveredAt = &value
		}
		result = append(result, item)
	}
	writeJSON(w, http.StatusOK, result)
}
