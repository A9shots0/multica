package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// EnqueueOwnerIntakeInTransaction reuses normal issue/squad admission without
// publishing or waking a daemon before the owner receipt commits. A fresh
// service avoids copying TaskService's mutexes and transient runtime state.
func (s *TaskService) EnqueueOwnerIntakeInTransaction(ctx context.Context, tx pgx.Tx, issue db.Issue, commentID pgtype.UUID) (db.AgentTaskQueue, error) {
	q := s.Queries.WithTx(tx)
	worker := &TaskService{
		Queries: q, TxStarter: tx, Bus: events.New(),
		PrepareWorkflowGuardedAuthority: s.PrepareWorkflowGuardedAuthority,
		FeatureFlags:                    s.FeatureFlags, Entitlements: s.Entitlements, Composio: s.Composio,
	}
	if issue.AssigneeType.String == "squad" {
		if _, err := EnsureWorkflowController(ctx, q, nil, issue); err != nil {
			return db.AgentTaskQueue{}, err
		}
		squad, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			return db.AgentTaskQueue{}, err
		}
		return worker.EnqueueTaskForSquadLeader(ctx, issue, squad.LeaderID, squad.ID, commentID)
	}
	if issue.AssigneeType.String != "agent" {
		return db.AgentTaskQueue{}, errors.New("issue has no executable assignee")
	}
	return worker.EnqueueTaskForIssue(ctx, issue, commentID)
}

func (s *TaskService) PublishOwnerIntakeTask(ctx context.Context, task db.AgentTaskQueue) {
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
	s.NotifyTaskEnqueued(ctx, task)
}
