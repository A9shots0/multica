package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CaptureOpeningChatTasks records provenance, not a late issue selection. An
// ordinary message sent before its opening issue exists may later correct only
// an issue created by one of these exact owner turns. Manual links and later
// unrelated turns never gain authority through this path.
func CaptureOpeningChatTasks(ctx context.Context, q *db.Queries, session db.ChatSession, task db.AgentTaskQueue) error {
	links, err := q.ListIssuesForChat(ctx, db.ListIssuesForChatParams{WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID})
	if err != nil || len(links) != 0 {
		return err
	}
	pending, err := q.ListPendingChatTasksForSession(ctx, session.ID)
	if err != nil {
		return err
	}
	ids := []string{util.UUIDToString(task.ID)}
	for _, prior := range pending {
		if prior.ID != task.ID && prior.MessageID.Valid {
			ids = append(ids, util.UUIDToString(prior.ID))
		}
	}
	// Refuse an unusually large unresolved opening instead of widening its scope.
	if len(ids) > 32 {
		return nil
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return q.SetChatOpeningTasks(ctx, db.SetChatOpeningTasksParams{TaskID: task.ID, ChatSessionID: session.ID, OpeningTasks: raw})
}

// ResolveOpeningChatCorrection freezes once, as soon as the exact opening
// issue exists. Called at claim and again at submission because the preceding
// interrupted turn may finish creating its issue after the next turn claims.
// Ordinary revision/access/request guards still run at submission. This path
// never permits approval, acceptance, cancellation or a question answer.
func ResolveOpeningChatCorrection(ctx context.Context, q *db.Queries, session db.ChatSession, task db.AgentTaskQueue) (db.AgentTaskQueue, error) {
	current, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		return task, err
	}
	if current.ChatSessionID != session.ID || !current.ChatInputTaskID.Valid {
		return current, nil
	}
	var data struct {
		Target    ChatWorkTarget  `json:"chat_work_target"`
		Opening   []string        `json:"chat_opening_tasks"`
		Responses json.RawMessage `json:"chat_work_responses"`
	}
	if json.Unmarshal(current.Context, &data) != nil || data.Target.IssueID != "" || len(data.Responses) > 0 || len(data.Opening) == 0 || len(data.Opening) > 32 {
		return current, nil
	}
	links, err := q.ListIssuesForChat(ctx, db.ListIssuesForChatParams{WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID})
	if err != nil {
		return current, err
	}
	if len(links) != 1 || (links[0].Source != "origin" && links[0].Source != "binding_receipt") {
		return current, nil
	}
	issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: session.WorkspaceID, ID: links[0].ID})
	if err != nil {
		return current, err
	}
	if issue.OriginType.String != "agent_create" || !issue.OriginID.Valid || !issue.CreatedAt.Valid || !current.CreatedAt.Valid || issue.CreatedAt.Time.Before(current.CreatedAt.Time) {
		return current, nil
	}
	allowed := false
	for _, id := range data.Opening {
		if id == util.UUIDToString(issue.OriginID) {
			allowed = true
			break
		}
	}
	if !allowed {
		return current, nil
	}
	origin, err := q.GetAgentTask(ctx, issue.OriginID)
	if err != nil {
		return current, err
	}
	if origin.ChatSessionID != session.ID || origin.AgentID != session.AgentID || !origin.ChatInputTaskID.Valid || origin.AccountableUserID != session.CreatorID {
		return current, nil
	}
	target, err := CaptureConversationCorrection(ctx, q, session, session.CreatorID)
	if err != nil {
		return current, err
	}
	if target.IssueID == "" {
		return current, nil
	}
	raw, err := json.Marshal(target)
	if err != nil {
		return current, err
	}
	opening, _ := json.Marshal(data.Opening)
	frozen, err := q.FreezeOpeningChatWorkTarget(ctx, db.FreezeOpeningChatWorkTargetParams{TaskID: current.ID, ChatSessionID: session.ID, Target: raw, OpeningTasks: opening})
	if errors.Is(err, pgx.ErrNoRows) {
		return q.GetAgentTask(ctx, current.ID)
	}
	return frozen, err
}
