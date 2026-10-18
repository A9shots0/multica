package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var ErrChatMessageNotEditable = errors.New("only an ordinary message in your active Chat can be edited")

// Restore the service contract already exercised by the existing edit tests.
// Message replacement and fresh enqueue commit together; discarded task rows
// remain as execution evidence but cannot resume into the edited conversation.
func (s *TaskService) EditChatMessageAndRegenerate(ctx context.Context, session db.ChatSession, agent db.Agent, message db.ChatMessage, owner pgtype.UUID, content, uploaderType string, uploaderID pgtype.UUID) (*DirectChatSendResult, error) {
	if message.Role != "user" || message.MessageKind != "message" || message.ChatSessionID != session.ID || session.CreatorID != owner || uploaderType != "member" || uploaderID != owner || strings.TrimSpace(content) == "" {
		return nil, ErrChatMessageNotEditable
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if _, err := q.LockChatSessionForRuntimeBind(ctx, session.ID); err != nil {
		return nil, err
	}
	current, err := q.GetChatSession(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	if current.Status != "active" || current.CreatorID != owner || current.AgentID != agent.ID {
		return nil, ErrChatMessageNotEditable
	}
	if _, err := q.GetAgentForClaimUpdate(ctx, agent.ID); err != nil {
		return nil, err
	}
	message, err = q.GetChatMessage(ctx, message.ID)
	if err != nil {
		return nil, ErrChatMessageNotEditable
	}
	var selected struct {
		Target         ChatWorkTarget `json:"chat_work_target"`
		ResponseFormat string         `json:"chat_response_format"`
	}
	if message.TaskID.Valid {
		input, err := q.GetAgentTask(ctx, message.TaskID)
		if err != nil {
			return nil, err
		}
		if len(input.Context) > 0 {
			if err := json.Unmarshal(input.Context, &selected); err != nil {
				return nil, err
			}
		}
	}
	options := DirectChatSendOptions{WorkTarget: selected.Target}
	if selected.Target.CorrectionOnly {
		// An edited ordinary message gets a fresh conversational snapshot;
		// it cannot promote an implicit correction into an explicit decision.
		options.WorkTarget = ChatWorkTarget{}
	}
	if message.ReplyToMessageID.Valid {
		var replyContext ChatReplyContext
		if len(message.ReplyContext) == 0 || json.Unmarshal(message.ReplyContext, &replyContext) != nil {
			return nil, ErrChatReplyTargetChanged
		}
		options.ReplyTarget = ChatReplyTarget{
			EventID:    util.UUIDToString(message.ReplyToMessageID),
			BlockID:    replyContext.BlockID,
			ArtifactID: replyContext.ArtifactID,
		}
	}
	rows, err := tx.Query(ctx, `SELECT id FROM agent_task_queue WHERE chat_session_id=$1 AND id IN (SELECT task_id FROM chat_message WHERE chat_session_id=$1 AND (created_at,id)>=($2,$3)) ORDER BY id FOR UPDATE`, session.ID, message.CreatedAt, message.ID)
	if err != nil {
		return nil, err
	}
	var ids []pgtype.UUID
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var cancelled []db.AgentTaskQueue
	for _, id := range ids {
		task, err := q.GetAgentTask(ctx, id)
		if err != nil {
			return nil, err
		}
		active := task.Status == "running" || task.Status == "queued" || task.Status == "dispatched" || task.Status == "waiting_local_directory"
		if active {
			task.Status = "cancelled"
			cancelled = append(cancelled, task)
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_task_queue SET status=$2, completed_at=CASE WHEN $3 THEN now() ELSE completed_at END, chat_session_id=NULL WHERE id=$1`, id, task.Status, active); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE attachment SET chat_message_id=NULL WHERE chat_message_id IN (SELECT id FROM chat_message WHERE chat_session_id=$1 AND (created_at,id)>=($2,$3))`, session.ID, message.CreatedAt, message.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_message WHERE chat_session_id=$1 AND (created_at,id)>=($2,$3)`, session.ID, message.CreatedAt, message.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE chat_session SET session_id=NULL,runtime_id=NULL WHERE id=$1`, session.ID); err != nil {
		return nil, err
	}
	local := NewTaskService(q, tx, nil, events.New())
	local.Composio = s.Composio
	local.FeatureFlags = s.FeatureFlags
	local.Entitlements = s.Entitlements
	result, err := local.SendDirectChatMessageWithOptions(ctx, current, agent, owner, content, nil, uploaderType, uploaderID, selected.ResponseFormat, options)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for _, task := range cancelled {
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)
	}
	s.notifyTasksFinished(cancelled)
	s.PublishOwnerIntakeTask(ctx, result.Task)
	return result, nil
}
