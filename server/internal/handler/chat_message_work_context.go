package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// This is a display label, never a delegated permission or a new target.
type ChatMessageWorkContext struct {
	IssueID   string `json:"issue_id"`
	Title     string `json:"title"`
	RequestID string `json:"request_id,omitempty"`
}

// Read the already persisted owner-input target in one bounded query per page.
// The session gate has run before this helper. Do not infer context from the
// current composer selection, an adjacent message, or an assistant's prose.
func (h *Handler) chatMessageWorkContexts(ctx context.Context, session db.ChatSession, ids []pgtype.UUID) (map[string]*ChatMessageWorkContext, error) {
	result := map[string]*ChatMessageWorkContext{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := h.DB.Query(ctx, `SELECT m.id, t.context->'chat_work_target'->>'issue_id',
 COALESCE(NULLIF(i.title,''),'Issue unavailable'), COALESCE(t.context->'chat_work_target'->>'request_id','')
 FROM chat_message m
 JOIN agent_task_queue t ON t.id=m.task_id AND t.chat_session_id=m.chat_session_id
 JOIN chat_session c ON c.id=m.chat_session_id
 LEFT JOIN issue i ON i.id::text=t.context->'chat_work_target'->>'issue_id' AND i.workspace_id=c.workspace_id
 WHERE m.chat_session_id=$1 AND c.workspace_id=$2 AND m.id=ANY($3::uuid[]) AND m.role='user'
 AND t.context->'chat_work_target'->>'issue_id' IS NOT NULL
 AND COALESCE(t.context->'chat_work_target'->>'correction_only','false') <> 'true'`, session.ID, session.WorkspaceID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id pgtype.UUID
		var item ChatMessageWorkContext
		if err := rows.Scan(&id, &item.IssueID, &item.Title, &item.RequestID); err != nil {
			return nil, err
		}
		result[uuidToString(id)] = &item
	}
	return result, rows.Err()
}
