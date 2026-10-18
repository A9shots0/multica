package brain

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type Store interface {
	BindBrainTask(context.Context, db.BindBrainTaskParams) (db.BrainTaskLink, error)
	CaptureBrainEvents(context.Context) (int64, error)
	ListPendingBrainEvents(context.Context) ([]db.BrainEventOutbox, error)
	MarkBrainEventDelivered(context.Context, string) error
	MarkBrainEventFailure(context.Context, db.MarkBrainEventFailureParams) error
}

type Service struct {
	Client *Client
	Store  Store
}

type Reference struct {
	State     string          `json:"state"`
	Project   string          `json:"project,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	Lifecycle string          `json:"lifecycle"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// Claim is called only after Multica's workspace and workflow admission gates.
// Brain unavailability is visible to the agent and never grants new authority.
func (s *Service) Claim(ctx context.Context, workspace, project, targetRevision string, task db.AgentTaskQueue) json.RawMessage {
	ref := Reference{State: "unconfigured", Lifecycle: "not_recorded", Reason: "Shared brain is not configured for this workspace/project. Current source evidence is required."}
	if s != nil && s.Client != nil && s.Store != nil {
		if target, ok := s.Client.Project(workspace, project); ok {
			ref.Project = target
			ws, err := util.ParseUUID(workspace)
			if err != nil {
				ref.State = "unavailable"
				ref.Reason = "Invalid canonical workspace."
			} else {
				link, err := s.Store.BindBrainTask(ctx, db.BindBrainTaskParams{WorkspaceID: ws, TaskID: task.ID, Attempt: task.Attempt, ProjectID: target, TargetRevision: targetRevision})
				if err != nil || link.ProjectID != target || link.TargetRevision != targetRevision {
					ref.State = "unavailable"
					ref.Reason = "Durable project binding unavailable or changed; context was withheld."
				} else {
					ref.Lifecycle = "durable_source_bound"
					ref.State = "online"
					ref.Reason = ""
					ref.Data, err = s.Client.Context(ctx, target, "")
					if err != nil {
						ref.State = "unavailable"
						ref.Reason = err.Error()
					}
				}
			}
		}
	}
	raw, _ := json.Marshal(ref)
	return raw
}

func (c *Client) Allows(workspace, project string) bool {
	if c == nil {
		return false
	}
	mapping, ok := c.config.Workspaces[workspace]
	if !ok {
		return false
	}
	if mapping.DefaultProject == project {
		return true
	}
	for _, p := range mapping.Projects {
		if p == project {
			return true
		}
	}
	return false
}

// Reconcile recovers late commits and crash-after-send-before-receipt. The
// service deduplicates identical keys/bodies; conflicts stay blocked for review.
func (s *Service) Reconcile(ctx context.Context) error {
	if s == nil || s.Client == nil {
		return nil
	}
	if _, err := s.Store.CaptureBrainEvents(ctx); err != nil {
		return err
	}
	rows, err := s.Store.ListPendingBrainEvents(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		blocked := false
		if !s.Client.Allows(util.UUIDToString(row.WorkspaceID), row.ProjectID) {
			err = errors.New("brain project mapping was removed; delivery requires review")
			blocked = true
		} else {
			err = s.Client.Send(ctx, row.Payload)
		}
		if err == nil {
			if err = s.Store.MarkBrainEventDelivered(ctx, row.EventID); err != nil {
				return err
			}
			continue
		}
		var response *ResponseError
		if errors.As(err, &response) && response.Status >= 400 && response.Status < 500 && response.Status != 408 && response.Status != 429 {
			blocked = true
		}
		if err = s.Store.MarkBrainEventFailure(ctx, db.MarkBrainEventFailureParams{EventID: row.EventID, IsBlocked: blocked, LastError: pgtype.Text{String: err.Error(), Valid: true}}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Run(ctx context.Context) {
	if s == nil {
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		batch, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := s.Reconcile(batch)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("shared brain reconciliation pending", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
