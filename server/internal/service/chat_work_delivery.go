package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// ChatWorkUpdate is evidence from a worker, not a reply authored by Mika. The
// request id is a reference only: answering still captures a fresh canonical
// target and uses the existing revision/ownership checks.
type ChatWorkUpdate struct {
	Kind       string          `json:"kind"`
	IssueID    string          `json:"issue_id"`
	Title      string          `json:"title"`
	SourceID   string          `json:"source_id"`
	RequestID  string          `json:"request_id,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
	Body       string          `json:"body"`
	Review     *ChatWorkReview `json:"review,omitempty"`
}

// ChatWorkReview is the immutable, versioned owner-review payload stored on a
// delivered Chat message. It turns server-authored attachment prose into data
// the UI can render without scraping it again. Attachments remain the durable
// file authority; this is the exact presentation snapshot for this result.
type ChatWorkReview struct {
	SchemaVersion int                `json:"schema_version"`
	SourceTaskID  string             `json:"source_task_id"`
	Artifacts     []ChatWorkArtifact `json:"artifacts"`
}

type ChatWorkArtifact struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Original bool   `json:"original"`
}

func reviewFromArtifacts(sourceTaskID string, raw []byte) *ChatWorkReview {
	var artifacts []ChatWorkArtifact
	if len(raw) == 0 || json.Unmarshal(raw, &artifacts) != nil || len(artifacts) == 0 {
		return nil
	}
	return &ChatWorkReview{SchemaVersion: 1, SourceTaskID: sourceTaskID, Artifacts: artifacts}
}

// DeliverChatWorkUpdates can be retried after a crash and run by multiple
// servers. The DB's deterministic message identity is the delivery receipt.
// No model invocation, task completion, inbox mutation or outbound channel send.
func DeliverChatWorkUpdates(ctx context.Context, q *db.Queries, bus *events.Bus) (int, error) {
	rows, err := q.ListUndeliveredChatWorkUpdates(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		update := ChatWorkUpdate{Kind: row.Kind, IssueID: util.UUIDToString(row.IssueID), Title: row.Title,
			SourceID: util.UUIDToString(row.SourceID), OccurredAt: row.OccurredAt.Time,
			Body: ChatWorkBodyForDisplay(row.Body, 4000)}
		if row.Kind == "result" && update.Body == longerChatWorkUpdateNotice {
			// SourceID for result events is the exact task, never a latest-task
			// lookup that could borrow another revision's answer.
			if source, loadErr := q.GetAgentTask(ctx, row.SourceID); loadErr == nil && source.Status == "completed" {
				var result struct {
					Output string `json:"output"`
				}
				if json.Unmarshal(source.Result, &result) == nil {
					if summary := chatWorkResultSummary(result.Output, 4000); summary != "" {
						update.Body = summary
					}
				}
			}
		}
		if row.RequestID.Valid {
			update.RequestID = util.UUIDToString(row.RequestID)
		}
		// A blocked run can still leave useful evidence. Keep its artifacts
		// accessible without implying the requested work succeeded.
		update.Review = reviewFromArtifacts(update.SourceID, row.Artifacts)
		raw, err := json.Marshal(update)
		if err != nil {
			return count, err
		}
		_, err = q.DeliverChatWorkUpdate(ctx, db.DeliverChatWorkUpdateParams{
			ID: row.MessageID, OwnerID: row.CreatorID, WorkspaceID: row.WorkspaceID, ChatSessionID: row.ChatSessionID, IssueID: row.IssueID,
			Content: fmt.Sprintf("Work update — %s\n%s", update.Title, update.Body), WorkUpdate: raw,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return count, err
		}
		count++
		if bus != nil {
			id := util.UUIDToString(row.ChatSessionID)
			// Only an invalidation, never private content in a workspace broadcast.
			bus.Publish(events.Event{Type: protocol.EventChatWorkUpdate, WorkspaceID: util.UUIDToString(row.WorkspaceID),
				ActorType: "system", ActorID: util.UUIDToString(row.CreatorID), ChatSessionID: id, Payload: map[string]string{"chat_session_id": id}})
		}
	}
	return count, nil
}

func RunChatWorkDelivery(ctx context.Context, q *db.Queries, bus *events.Bus, followUps ...*TaskService) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		batchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		_, err := DeliverChatWorkUpdates(batchCtx, q, bus)
		if err == nil && len(followUps) > 0 {
			_, err = followUps[0].EnqueueChatFollowUps(batchCtx)
		}
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("chat work delivery will retry", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
