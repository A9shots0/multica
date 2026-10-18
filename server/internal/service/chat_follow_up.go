package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func IsChatFollowUp(task db.AgentTaskQueue) bool {
	var context map[string]json.RawMessage
	return task.ChatSessionID.Valid && json.Unmarshal(task.Context, &context) == nil && len(context["chat_follow_up"]) > 0
}

// Follow-up never borrows human input, approves work or resumes its provider session.
func (s *TaskService) EnqueueChatFollowUps(ctx context.Context) (int, error) {
	sessions, err := s.Queries.ListChatFollowUpSessions(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, session := range sessions {
		task, err := s.enqueueChatFollowUp(ctx, session)
		if err != nil {
			return count, err
		}
		if task.ID.Valid {
			s.FinalizeChatTaskEnqueue(ctx, task)
			count++
		}
	}
	return count, nil
}
func (s *TaskService) enqueueChatFollowUp(ctx context.Context, session db.ChatSession) (db.AgentTaskQueue, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	current, err := q.LockChatSessionForEnqueue(ctx, session.ID)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	if current.Status != "active" {
		return db.AgentTaskQueue{}, nil
	}
	rows, err := q.ListPendingChatFollowUps(ctx, session.ID)
	if err != nil || len(rows) == 0 {
		return db.AgentTaskQueue{}, err
	}
	a, err := q.GetAgentForClaimUpdate(ctx, current.AgentID)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	if a.ArchivedAt.Valid || !a.RuntimeID.Valid {
		return db.AgentTaskQueue{}, nil
	}
	ids := []pgtype.UUID{}
	evidence := []json.RawMessage{}
	for _, row := range rows {
		ids = append(ids, row.ID)
		evidence = append(evidence, row.WorkUpdate)
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	task, err := q.CreateChatTask(ctx, db.CreateChatTaskParams{
		ID: dbid.NewV7(), AgentID: current.AgentID, RuntimeID: a.RuntimeID, Priority: 1, ChatSessionID: current.ID,
		InitiatorUserID: current.CreatorID, OriginatorUserID: current.CreatorID, AccountableUserID: current.CreatorID,
		ForceFreshSession: pgtype.Bool{Bool: true, Valid: true}, OriginatorSource: pgtype.Text{String: "chat_follow_up", Valid: true},
		TriggerEvidenceKind: pgtype.Text{String: "chat_work_update", Valid: true}, TriggerEvidenceRefID: rows[0].ID,
	})
	if err != nil {
		return task, fmt.Errorf("create chat follow-up: %w", err)
	}
	task, err = q.SetChatFollowUpContext(ctx, db.SetChatFollowUpContextParams{TaskID: task.ID, Evidence: raw})
	if err != nil {
		return task, err
	}
	if err = q.SealChatFollowUps(ctx, db.SealChatFollowUpsParams{TaskID: task.ID, MessageIds: ids, ChatSessionID: current.ID}); err != nil {
		return task, err
	}
	if err = tx.Commit(ctx); err != nil {
		return db.AgentTaskQueue{}, err
	}
	return task, nil
}

func updateHumanChatSession(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, p db.UpdateChatSessionSessionParams) error {
	if IsChatFollowUp(task) {
		return nil
	}
	return q.UpdateChatSessionSession(ctx, p)
}
