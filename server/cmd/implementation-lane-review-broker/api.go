package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/pkg/brokerreply"
)

const maxBrokerAPIResponseBytes = 1 << 20

func postWorkflowEvent(ctx context.Context, body map[string]any) error {
	return brokerJSON(ctx, http.MethodPost, "/api/issues/"+url.PathEscape(laneIssueID)+"/workflow-events", body, nil)
}

// allowedBrokerPath is the whole API surface of the review broker.
func allowedBrokerPath(path string) bool {
	switch path {
	case "/api/issues/" + laneIssueID + "/task-runs?active=true",
		"/api/issues/" + laneIssueID + "/workflow-events",
		"/api/issues/" + laneIssueID + "/comments":
		return true
	}
	return false
}

func brokerJSON(ctx context.Context, method, path string, body any, out any) error {
	server, err := requireLoopbackServer()
	if err != nil {
		return err
	}
	if !allowedBrokerPath(path) {
		return fmt.Errorf("out-of-scope broker API path")
	}
	token := strings.TrimSpace(os.Getenv("MULTICA_TOKEN"))
	if !strings.HasPrefix(token, "mat_") {
		return fmt.Errorf("the review broker requires a task-scoped Multica capability")
	}
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, server+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", strings.TrimSpace(os.Getenv("MULTICA_AGENT_ID")))
	req.Header.Set("X-Task-ID", strings.TrimSpace(os.Getenv("MULTICA_TASK_ID")))
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBrokerAPIResponseBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxBrokerAPIResponseBytes {
		return fmt.Errorf("loopback response exceeded broker limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("loopback %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode loopback response: %w", err)
		}
	}
	return nil
}

// postBoundComment publishes the review as a reply to the current trigger, with
// the three lane roles suppressed so the comment itself routes no work.
func postBoundComment(ctx context.Context, content string, roles []string) error {
	body, err := brokerreply.CommentBody(ctx, func(ctx context.Context, path string, out any) error {
		return brokerJSON(ctx, http.MethodGet, path, nil, out)
	}, laneIssueID, os.Getenv("MULTICA_TASK_ID"), os.Getenv("MULTICA_AGENT_ID"), content)
	if err != nil {
		return err
	}
	body["suppress_agent_ids"] = roles
	return brokerJSON(ctx, http.MethodPost, "/api/issues/"+laneIssueID+"/comments", body, nil)
}
