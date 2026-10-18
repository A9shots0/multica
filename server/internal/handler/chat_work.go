package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Acknowledgement is a personal reading action, not permission for Mika to
// answer a question, accept an issue, or suppress unseen updates on our behalf.
func (h *Handler) AcknowledgeChatWork(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Actor-Source") == "task_token" || r.Header.Get("X-Agent-ID") != "" {
		writeError(w, 403, "only the conversation owner can acknowledge updates")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	session, ok := h.gatePublicChatSessionForUser(w, r, userID, ctxWorkspaceID(r.Context()), chi.URLParam(r, "sessionId"))
	if !ok {
		return
	}
	var input struct {
		MessageIDs []string `json:"message_ids"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || len(input.MessageIDs) == 0 || len(input.MessageIDs) > 100 {
		writeError(w, 400, "provide between 1 and 100 update identities")
		return
	}
	ids := make([]pgtype.UUID, 0, len(input.MessageIDs))
	for _, value := range input.MessageIDs {
		id, valid := parseUUIDOrBadRequest(w, value, "message id")
		if !valid {
			return
		}
		ids = append(ids, id)
	}
	err := h.Queries.AcknowledgeChatWorkUpdates(r.Context(), db.AcknowledgeChatWorkUpdatesParams{
		WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID, OwnerID: parseUUID(userID), MessageIds: ids,
	})
	if err != nil {
		writeError(w, 500, "could not acknowledge updates")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetChatWork(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	session, ok := h.gatePublicChatSessionForUser(w, r, userID, ctxWorkspaceID(r.Context()), chi.URLParam(r, "sessionId"))
	if !ok {
		return
	}
	h.writeChatWork(w, r, session)
}

// Task callers can only inspect their current conversation, never choose a
// different owner's session. Match the same access gate used by the UI.
func (h *Handler) GetCurrentChatWork(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Actor-Source") != "task_token" {
		writeError(w, 403, "requires a Chat task")
		return
	}
	task, ok := h.taskFromRequestHeader(r)
	if !ok || task.Status != "running" || !task.ChatSessionID.Valid || uuidToString(task.AgentID) != r.Header.Get("X-Agent-ID") {
		writeError(w, 403, "requires the running Chat task")
		return
	}
	ownerRequest, session, ok := h.chatOwnerRequest(w, r, task)
	if !ok {
		return
	}
	r = ownerRequest
	h.writeChatWork(w, r, session)
}

func (h *Handler) writeChatWork(w http.ResponseWriter, r *http.Request, session db.ChatSession) {
	var before pgtype.Timestamptz
	var id pgtype.UUID
	if value := r.URL.Query().Get("before_time"); value != "" {
		t, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			writeError(w, 400, "invalid activity cursor")
			return
		}
		var ok bool
		id, ok = parseUUIDOrBadRequest(w, r.URL.Query().Get("before_id"), "activity cursor")
		if !ok {
			return
		}
		before = pgtype.Timestamptz{Time: t, Valid: true}
	}
	work, err := service.LoadChatWork(r.Context(), h.Queries, session.WorkspaceID, session.ID, before, id)
	if err != nil {
		writeError(w, 500, "conversation work is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, work)
}

// GetChatWorkIndex supplies only aggregate flags for the owner's conversation
// library. Full evidence still requires the individual session access gate.
func (h *Handler) GetChatWorkIndex(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUserID(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListChatWorkIndex(r.Context(), db.ListChatWorkIndexParams{WorkspaceID: parseUUID(ctxWorkspaceID(r.Context())), OwnerID: parseUUID(user), FreshnessSeconds: service.RuntimeClaimFreshnessSeconds})
	if err != nil {
		writeError(w, 500, "conversation states unavailable")
		return
	}
	type flags struct {
		Working  bool `json:"working"`
		NeedsYou bool `json:"needs_you"`
	}
	result := map[string]flags{}
	for _, row := range rows {
		summary := workflowprojection.OwnerIssueSummaryFromDocument(row.Picture, row.PictureRevision, row.ObservedAt.Time)
		state := service.ChatWorkState(db.ListChatWorkRow{TaskStatus: row.TaskStatus, RuntimeFresh: row.RuntimeFresh}, summary)
		id := uuidToString(row.SessionID)
		item := result[id]
		item.Working = item.Working || state == "working" || state == "checking"
		item.NeedsYou = item.NeedsYou || row.NeedsOwner || state == "needs_you"
		result[id] = item
	}
	writeJSON(w, 200, result)
}
