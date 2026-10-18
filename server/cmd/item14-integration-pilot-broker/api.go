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

func routeToImplementer(ctx context.Context) error {
	issueID, err := brokerIssueID()
	if err != nil {
		return err
	}
	var plan struct {
		ImplementerID string `json:"implementer_id"`
	}
	if err := brokerJSON(ctx, http.MethodGet, "/api/issues/"+url.PathEscape(issueID)+"/workflow-plan", nil, &plan); err != nil {
		return fmt.Errorf("read frozen workflow plan: %w", err)
	}
	if strings.TrimSpace(plan.ImplementerID) == "" {
		return fmt.Errorf("workflow plan has no implementer")
	}
	content := fmt.Sprintf(
		"[@%s](mention://agent/%s) Execute the frozen Item 14 replay packet through the exact local broker. Do not expand scope.",
		executorAgentName, plan.ImplementerID,
	)
	return postBrokerComment(ctx, issueID, content)
}

func postBrokerComment(ctx context.Context, issueID, content string) error {
	body, err := brokerreply.CommentBody(ctx, func(ctx context.Context, path string, out any) error {
		return brokerJSON(ctx, http.MethodGet, path, nil, out)
	}, issueID, os.Getenv("MULTICA_TASK_ID"), os.Getenv("MULTICA_AGENT_ID"), content)
	if err != nil {
		return err
	}
	return brokerJSON(ctx, http.MethodPost, "/api/issues/"+url.PathEscape(issueID)+"/comments", body, nil)
}

func postWorkflowEvent(ctx context.Context, body map[string]any) error {
	issueID, err := brokerIssueID()
	if err != nil {
		return err
	}
	return brokerJSON(ctx, http.MethodPost, "/api/issues/"+url.PathEscape(issueID)+"/workflow-events", body, nil)
}

func brokerIssueID() (string, error) {
	issueID := strings.TrimSpace(os.Getenv("MULTICA_ISSUE_ID"))
	if issueID == "" {
		return "", fmt.Errorf("pilot broker issue identity is unavailable")
	}
	return issueID, nil
}

func brokerJSON(ctx context.Context, method, path string, body any, out any) error {
	server, err := requireLoopbackServer()
	if err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv("MULTICA_TOKEN"))
	if !strings.HasPrefix(token, "mat_") {
		return fmt.Errorf("pilot broker requires a task-scoped Multica capability")
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
