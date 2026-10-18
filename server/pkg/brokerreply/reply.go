// Package brokerreply binds broker comments to the current native task trigger.
package brokerreply

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type GetJSON func(context.Context, string, any) error

// CommentBody reads fresh task context for every publication. A resumed or
// coalesced run must reply to its current trigger, never a cached thread root.
// The server remains responsible for the final task/parent authorization race.
func CommentBody(ctx context.Context, get GetJSON, issueID, taskID, agentID, content string) (map[string]any, error) {
	if strings.TrimSpace(issueID) == "" || strings.TrimSpace(taskID) == "" || strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("comment publication requires exact issue, task, and agent identity")
	}
	var tasks []struct {
		ID               string  `json:"id"`
		IssueID          string  `json:"issue_id"`
		AgentID          string  `json:"agent_id"`
		Status           string  `json:"status"`
		TriggerCommentID *string `json:"trigger_comment_id"`
	}
	if err := get(ctx, "/api/issues/"+url.PathEscape(issueID)+"/task-runs?active=true", &tasks); err != nil {
		return nil, fmt.Errorf("read current comment trigger: %w", err)
	}
	body := map[string]any{"content": content}
	matches := 0
	for _, task := range tasks {
		if task.ID != taskID {
			continue
		}
		matches++
		if task.IssueID != issueID || task.AgentID != agentID || task.Status != "running" {
			return nil, fmt.Errorf("comment publication task identity or running state changed")
		}
		if task.TriggerCommentID != nil && *task.TriggerCommentID != "" {
			parent := *task.TriggerCommentID
			parsed, err := uuid.Parse(parent)
			if err != nil || parsed == uuid.Nil || parsed.String() != parent {
				return nil, fmt.Errorf("current comment trigger is not a canonical UUID")
			}
			body["parent_id"] = parent
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("comment publication requires exactly one current task")
	}
	return body, nil
}
