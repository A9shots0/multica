package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ChatWork is shared by the owner UI, CLI and each fresh/resumed Mika turn.
// Existing workflow records remain authoritative; this creates no second engine.
type ChatWork struct {
	CatchUp        []ChatCatchUpUpdate          `json:"catch_up"`
	HasMoreCatchUp bool                         `json:"has_more_catch_up"`
	Updates        []ChatWorkUpdate             `json:"updates"`
	ObservedAt     time.Time                    `json:"observed_at"`
	Issues         []ChatWorkIssue              `json:"issues"`
	Activity       []db.ListChatWorkActivityRow `json:"activity"`
	HasOlder       bool                         `json:"has_older"`
	Requests       []ChatWorkOwnerRequest       `json:"requests"`
}

// ChatWorkOwnerRequest is the Inbox wire shape needed by both Chat and Mika.
// The sqlc row keeps Details as []byte; encoding that row directly turns the
// JSON document into base64 and makes the owner's authored options invisible
// to the client. Normalize at this service boundary just as the Inbox handler
// does at its own boundary.
type ChatWorkOwnerRequest struct {
	ID            string          `json:"id"`
	WorkspaceID   string          `json:"workspace_id"`
	RecipientType string          `json:"recipient_type"`
	RecipientID   string          `json:"recipient_id"`
	Type          string          `json:"type"`
	Severity      string          `json:"severity"`
	IssueID       *string         `json:"issue_id"`
	Title         string          `json:"title"`
	Body          *string         `json:"body"`
	Read          bool            `json:"read"`
	Archived      bool            `json:"archived"`
	CreatedAt     string          `json:"created_at"`
	ActorType     *string         `json:"actor_type"`
	ActorID       *string         `json:"actor_id"`
	Details       json.RawMessage `json:"details"`
	IssueStatus   *string         `json:"issue_status"`
	IssuePriority *string         `json:"issue_priority"`
}

func chatWorkOwnerRequest(row db.ListInboxItemsRow) ChatWorkOwnerRequest {
	return ChatWorkOwnerRequest{
		ID: util.UUIDToString(row.ID), WorkspaceID: util.UUIDToString(row.WorkspaceID),
		RecipientType: row.RecipientType, RecipientID: util.UUIDToString(row.RecipientID),
		Type: row.Type, Severity: row.Severity, IssueID: util.UUIDToPtr(row.IssueID),
		Title: row.Title, Body: util.TextToPtr(row.Body), Read: row.Read, Archived: row.Archived,
		CreatedAt: util.TimestampToString(row.CreatedAt), ActorType: util.TextToPtr(row.ActorType),
		ActorID: util.UUIDToPtr(row.ActorID), Details: json.RawMessage(row.Details),
		IssueStatus: util.TextToPtr(row.IssueStatus), IssuePriority: util.TextToPtr(row.IssuePriority),
	}
}

type ChatCatchUpUpdate struct {
	MessageID string `json:"message_id"`
	ChatWorkUpdate
}

type ChatWorkIssue struct {
	ID              string                               `json:"id"`
	Number          int32                                `json:"number"`
	Title           string                               `json:"title"`
	Revision        int64                                `json:"revision"`
	ProjectID       pgtype.UUID                          `json:"project_id"`
	State           string                               `json:"state"`
	TaskID          pgtype.UUID                          `json:"task_id"`
	TaskStatus      string                               `json:"task_status"`
	WaitReason      string                               `json:"wait_reason"`
	Source          string                               `json:"source"`
	ObservedAt      pgtype.Timestamptz                   `json:"observed_at"`
	Summary         workflowprojection.OwnerIssueSummary `json:"summary"`
	Target          ChatWorkTarget                       `json:"target"`
	Picture         json.RawMessage                      `json:"picture"`
	PendingRequests []ChatWorkRequest                    `json:"pending_requests"`
}

type ChatWorkRequest struct {
	ID       string         `json:"id"`
	Question string         `json:"question"`
	Target   ChatWorkTarget `json:"target"`
}

func LoadChatWork(ctx context.Context, q *db.Queries, workspaceID, sessionID pgtype.UUID, beforeTime pgtype.Timestamptz, beforeID pgtype.UUID) (ChatWork, error) {
	out := ChatWork{ObservedAt: time.Now().UTC(), Issues: []ChatWorkIssue{}, Activity: []db.ListChatWorkActivityRow{}}
	rows, err := q.ListChatWork(ctx, db.ListChatWorkParams{WorkspaceID: workspaceID, ChatSessionID: sessionID, FreshnessSeconds: RuntimeClaimFreshnessSeconds})
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		summary := workflowprojection.OwnerIssueSummaryFromDocument(row.Picture, row.PictureRevision, row.ObservedAt.Time)
		if row.TaskStatus == "completed" && row.Status != "done" && summary.Stage != workflowprojection.OwnerStageComplete && row.WaitReason == "" {
			row.WaitReason = "The worker finished. Its result is available; the issue has not been accepted as complete."
		}
		out.Issues = append(out.Issues, ChatWorkIssue{ID: util.UUIDToString(row.ID), Number: row.Number, Title: row.Title, Revision: row.Revision, ProjectID: row.ProjectID, State: ChatWorkState(row, summary), TaskID: row.TaskID, TaskStatus: row.TaskStatus, WaitReason: row.WaitReason, Source: row.Source, ObservedAt: row.ObservedAt, Summary: summary, Picture: row.Picture})
	}
	// Include the same owner's canonical pending requests. Ordinary worker
	// questions may predate Current Picture; they must still reach Chat/Mika.
	session, err := q.GetChatSession(ctx, sessionID)
	if err != nil {
		return out, err
	}
	items, err := q.ListInboxItems(ctx, db.ListInboxItemsParams{WorkspaceID: workspaceID, RecipientType: "member", RecipientID: session.CreatorID})
	if err != nil {
		return out, err
	}
	out.Requests = []ChatWorkOwnerRequest{}
	requestRows := []db.ListInboxItemsRow{}
	for _, item := range items {
		var details map[string]string
		_ = json.Unmarshal(item.Details, &details)
		if details["intake_state"] == "continued" {
			continue
		}
		if item.Type != "workflow_owner_decision" && !(item.Type == "chat_intake" && details["question_scope"] == "ordinary_task") {
			continue
		}
		for index := range out.Issues {
			if out.Issues[index].ID != util.UUIDToString(item.IssueID) {
				continue
			}
			out.Issues[index].State = "needs_you"
			requestRows = append(requestRows, item)
			out.Requests = append(out.Requests, chatWorkOwnerRequest(item))
		}
	}
	for index := range out.Issues {
		issue := &out.Issues[index]
		var id pgtype.UUID
		_ = id.Scan(issue.ID)
		target, err := CaptureChatWorkTarget(ctx, q, session, session.CreatorID, id)
		if err != nil {
			return out, err
		}
		// Fail closed if a concurrent mutation split the display and target read.
		if target.IssueRevision != issue.Revision {
			return out, ErrChatWorkTargetChanged
		}
		issue.Target = target
		issue.PendingRequests = []ChatWorkRequest{}
		for _, item := range requestRows {
			if item.IssueID != id {
				continue
			}
			requestID := util.UUIDToString(item.ID)
			selected, err := CaptureChatWorkTarget(ctx, q, session, session.CreatorID, id, requestID)
			if err != nil {
				return out, err
			}
			if selected.IssueRevision != issue.Revision {
				return out, ErrChatWorkTargetChanged
			}
			var details map[string]string
			_ = json.Unmarshal(item.Details, &details)
			question := details["question"]
			if question == "" {
				question = item.Title
			}
			issue.PendingRequests = append(issue.PendingRequests, ChatWorkRequest{ID: requestID, Question: question, Target: selected})
		}
	}
	if !beforeTime.Valid {
		beforeTime = pgtype.Timestamptz{Time: out.ObservedAt.Add(time.Second), Valid: true}
		beforeID = pgtype.UUID{Bytes: [16]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, Valid: true}
	}
	updates, err := q.ListRecentChatWorkUpdates(ctx, db.ListRecentChatWorkUpdatesParams{WorkspaceID: workspaceID, ChatSessionID: sessionID})
	if err != nil {
		return out, err
	}
	out.Updates = []ChatWorkUpdate{}
	unseen, err := q.ListUnseenChatWorkUpdates(ctx, db.ListUnseenChatWorkUpdatesParams{WorkspaceID: workspaceID, ChatSessionID: sessionID})
	if err != nil {
		return out, err
	}
	out.CatchUp = []ChatCatchUpUpdate{}
	out.HasMoreCatchUp = len(unseen) > 100
	if out.HasMoreCatchUp {
		unseen = unseen[:100]
	}
	for _, message := range unseen {
		var update ChatWorkUpdate
		if err := json.Unmarshal(message.WorkUpdate, &update); err != nil {
			return out, err
		}
		// Catch-up is a bounded index, not a second copy of every full report.
		update.Body = ChatWorkBodyForDisplay(update.Body, 600)
		out.CatchUp = append(out.CatchUp, ChatCatchUpUpdate{MessageID: util.UUIDToString(message.ID), ChatWorkUpdate: update})
	}
	for _, message := range updates {
		var update ChatWorkUpdate
		if err := json.Unmarshal(message.WorkUpdate, &update); err != nil {
			return out, err
		}
		update.Body = ChatWorkBodyForDisplay(update.Body, 4000)
		out.Updates = append(out.Updates, update)
	}
	out.Activity, err = q.ListChatWorkActivity(ctx, db.ListChatWorkActivityParams{WorkspaceID: workspaceID, ChatSessionID: sessionID, BeforeTime: beforeTime, BeforeID: beforeID})
	if len(out.Activity) > 50 {
		out.HasOlder = true
		out.Activity = out.Activity[:50]
	}
	return out, err
}

func ChatWorkState(row db.ListChatWorkRow, summary workflowprojection.OwnerIssueSummary) string {
	// An unanswered owner request wins over another parallel execution.
	switch summary.Stage {
	case workflowprojection.OwnerStageNeedsOwner:
		return "needs_you"
	case workflowprojection.OwnerStageBlocked:
		return "waiting"
	case workflowprojection.OwnerStageTechnicalStop:
		return "stopped"
	}
	// A stale runtime never leaves a convincing working spinner behind.
	if row.TaskStatus == "running" {
		if !row.RuntimeFresh {
			return "unavailable"
		}
		switch summary.Stage {
		case workflowprojection.OwnerStageInReview, workflowprojection.OwnerStageVerifying:
			return "checking"
		}
		return "working"
	}
	switch row.TaskStatus {
	case "queued", "dispatched":
		return "queued"
	case "waiting_local_directory":
		return "waiting"
	case "failed", "cancelled":
		return "stopped"
	}
	if summary.Stage == workflowprojection.OwnerStageComplete || (row.Status == "done" && row.PictureRevision == 0) {
		return "done"
	}
	if row.TaskStatus == "completed" {
		return "waiting"
	}
	// Neither a final assistant message nor a completed task proves acceptance.
	return "unavailable"
}
