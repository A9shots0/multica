package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) chatLibraryOwner(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	if r.Header.Get("X-Actor-Source") == "task_token" {
		task, ok := h.taskFromRequestHeader(r)
		if !ok || task.Status != "running" || !task.ChatSessionID.Valid || !task.ChatInputTaskID.Valid || uuidToString(task.AgentID) != r.Header.Get("X-Agent-ID") {
			writeError(w, 403, "requires the running owner Chat turn")
			return r, false
		}
		ownerRequest, _, ok := h.chatOwnerRequest(w, r, task)
		return ownerRequest, ok
	}
	_, ok := requireUserID(w, r)
	return r, ok
}
func (h *Handler) GetChatLibrary(w http.ResponseWriter, r *http.Request) {
	r, ok := h.chatLibraryOwner(w, r)
	if !ok {
		return
	}
	row, err := h.Queries.GetChatLibrary(r.Context(), db.GetChatLibraryParams{WorkspaceID: parseUUID(ctxWorkspaceID(r.Context())), OwnerID: parseUUID(requestUserID(r))})
	out := service.EmptyChatLibrary()
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, out)
		return
	}
	if err != nil || json.Unmarshal(row.Preferences, &out) != nil {
		writeError(w, 500, "conversation organization is unavailable")
		return
	}
	out.Revision = row.Revision
	writeJSON(w, 200, out)
}
func (h *Handler) ChangeChatLibrary(w http.ResponseWriter, r *http.Request) {
	r, ok := h.chatLibraryOwner(w, r)
	if !ok {
		return
	}
	var input service.ChatLibraryChange
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 262144))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, "invalid library change")
		return
	}
	if input.Action == "move_chat" && input.SessionID == "" && r.Header.Get("X-Actor-Source") == "task_token" {
		task, _ := h.taskFromRequestHeader(r)
		input.SessionID = uuidToString(task.ChatSessionID)
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "could not save organization")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	ws, owner := parseUUID(ctxWorkspaceID(r.Context())), parseUUID(requestUserID(r))
	if err = q.EnsureChatLibrary(r.Context(), db.EnsureChatLibraryParams{WorkspaceID: ws, OwnerID: owner}); err != nil {
		writeError(w, 500, "could not save organization")
		return
	}
	row, err := q.LockChatLibrary(r.Context(), db.LockChatLibraryParams{WorkspaceID: ws, OwnerID: owner})
	if err != nil {
		writeError(w, 500, "could not read organization")
		return
	}
	current := service.EmptyChatLibrary()
	if json.Unmarshal(row.Preferences, &current) != nil {
		writeError(w, 500, "could not read organization")
		return
	}
	current.Revision = row.Revision
	sessions, err := q.ListAllChatSessionsByCreator(r.Context(), db.ListAllChatSessionsByCreatorParams{WorkspaceID: ws, CreatorID: owner})
	if err != nil {
		writeError(w, 500, "could not verify conversations")
		return
	}
	owned := map[string]bool{}
	for _, s := range sessions {
		owned[uuidToString(s.ID)] = true
	}
	owns := func(id string) bool { return owned[id] }
	next, err := service.ApplyChatLibraryChange(current, input, owns)
	if err == nil && input.Action == "create_group" && input.SessionID != "" {
		next, err = service.ApplyChatLibraryChange(next, service.ChatLibraryChange{Action: "move_chat", ID: input.ID, SessionID: input.SessionID}, owns)
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	raw, _ := json.Marshal(next)
	saved, err := q.SaveChatLibrary(r.Context(), db.SaveChatLibraryParams{WorkspaceID: ws, OwnerID: owner, Preferences: raw})
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, 500, "could not save organization")
		return
	}
	next.Revision = saved.Revision
	writeJSON(w, 200, next)
}
