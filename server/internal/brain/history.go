package brain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
)

// Events reads bounded history within the frozen task project. The optional
// source task filter narrows history; it never changes project authority.
func (c *Client) Events(ctx context.Context, project, taskID, cursor string, limit int) (json.RawMessage, error) {
	if cursor == "" {
		cursor = "0"
	}
	after, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || after < 0 || len(taskID) > 200 || limit < 1 || limit > 100 {
		return nil, errors.New("invalid bounded history query")
	}
	query := url.Values{"project": {project}, "cursor": {cursor}, "limit": {strconv.Itoa(limit)}}
	if taskID != "" {
		query.Set("taskId", taskID)
	}
	raw, err := c.request(ctx, http.MethodGet, "events?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Events *[]struct {
			Sequence int64  `json:"sequence"`
			Project  string `json:"project"`
			TaskID   string `json:"task_id"`
			Payload  struct {
				Project *string `json:"project"`
				TaskID  *string `json:"taskId"`
			} `json:"payload"`
		} `json:"events"`
		NextCursor string `json:"nextCursor"`
		HasMore    *bool  `json:"hasMore"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Events == nil || response.HasMore == nil || len(*response.Events) > limit {
		return nil, errors.New("invalid brain history envelope")
	}
	last := after
	for _, event := range *response.Events {
		if event.Project != project || (taskID != "" && event.TaskID != taskID) || event.Sequence <= last ||
			(event.Payload.Project != nil && *event.Payload.Project != project) || (event.Payload.TaskID != nil && *event.Payload.TaskID != event.TaskID) {
			return nil, errors.New("history is unavailable in this task project or cursor")
		}
		last = event.Sequence
	}
	next, err := strconv.ParseInt(response.NextCursor, 10, 64)
	if err != nil || next != last || (*response.HasMore && len(*response.Events) == 0) {
		return nil, errors.New("invalid brain history cursor")
	}
	return raw, nil
}

// Evidence requires the immutable revision, even when the upstream allows an
// omitted revision, and checks the returned record before exposing any content.
func (c *Client) Evidence(ctx context.Context, project, id, revision string) (json.RawMessage, error) {
	parsedID, idErr := uuid.Parse(id)
	_, hashErr := hex.DecodeString(revision)
	if idErr != nil || parsedID == uuid.Nil || len(revision) != 64 || hashErr != nil {
		return nil, errors.New("immutable evidence id and revision required")
	}
	raw, err := c.request(ctx, http.MethodGet, "evidence/"+url.PathEscape(id)+"?"+url.Values{"revision": {revision}}.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Evidence struct {
			ID          string          `json:"id"`
			Project     string          `json:"project"`
			Revision    string          `json:"revision"`
			Truncated   bool            `json:"truncated"`
			Observation json.RawMessage `json:"observation"`
		} `json:"evidence"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Evidence.ID != id || response.Evidence.Project != project || response.Evidence.Revision != revision || response.Evidence.Truncated || len(response.Evidence.Observation) == 0 || string(response.Evidence.Observation) == "null" {
		return nil, errors.New("evidence is unavailable in this task project or revision")
	}
	return raw, nil
}
