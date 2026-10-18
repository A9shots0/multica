package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) chatMessageResponseFormats(ctx context.Context, session db.ChatSession, ids []pgtype.UUID) (map[string]string, error) {
	result := map[string]string{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := h.DB.Query(ctx, `SELECT m.id, t.context
 FROM chat_message m
 JOIN agent_task_queue t ON t.id=m.task_id AND t.chat_session_id=m.chat_session_id
 JOIN chat_session c ON c.id=m.chat_session_id
 WHERE m.chat_session_id=$1 AND c.workspace_id=$2 AND m.id=ANY($3::uuid[]) AND m.role='user'
 AND t.context ? 'chat_response_format'`, session.ID, session.WorkspaceID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id pgtype.UUID
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		result[uuidToString(id)] = service.ChatResponseFormatFromContext(raw)
	}
	return result, rows.Err()
}
