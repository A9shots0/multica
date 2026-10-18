// Package brain connects authorized Multica runs to shared reference evidence.
// Multica remains the authority for workspaces, targets and task acceptance.
package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxResponse = 2 * 1024 * 1024

type Workspace struct {
	Projects       map[string]string `json:"projects"`
	DefaultProject string            `json:"defaultProject,omitempty"`
}

// Config is administrator-owned. Project IDs come from canonical Multica rows,
// never model text, a repository basename or a caller-supplied brain project.
type Config struct {
	URL        string               `json:"url"`
	TokenFile  string               `json:"tokenFile"`
	Workspaces map[string]Workspace `json:"workspaces"`
}

type Client struct {
	config Config
	token  string
	http   *http.Client
}

// Load deliberately returns generic errors: protected config and HTTP errors
// can contain credentials or query text and must not be logged.
func Load(path string) (*Client, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("brain configuration unavailable")
	}
	var config Config
	if len(raw) > 64*1024 || json.Unmarshal(raw, &config) != nil {
		return nil, errors.New("invalid brain configuration")
	}
	u, err := url.Parse(config.URL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("invalid brain endpoint")
	}
	for ws, mapping := range config.Workspaces {
		if _, err := uuid.Parse(ws); err != nil {
			return nil, errors.New("invalid brain workspace mapping")
		}
		for project, target := range mapping.Projects {
			if _, err := uuid.Parse(project); err != nil || strings.TrimSpace(target) == "" {
				return nil, errors.New("invalid brain project mapping")
			}
		}
	}
	token, err := os.ReadFile(config.TokenFile)
	if err != nil || len(token) > 4096 || strings.TrimSpace(string(token)) == "" {
		return nil, errors.New("brain credential unavailable")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // Do not send private bearer credentials to ambient proxies.
	return &Client{config: config, token: strings.TrimSpace(string(token)), http: &http.Client{Timeout: 4 * time.Second, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) Project(workspace, project string) (string, bool) {
	if c == nil {
		return "", false
	}
	mapping, ok := c.config.Workspaces[workspace]
	if !ok {
		return "", false
	}
	if project == "" {
		return mapping.DefaultProject, mapping.DefaultProject != ""
	}
	target, ok := mapping.Projects[project]
	return target, ok && target != ""
}

// Projects returns the exact Brain projects enrolled for one Multica
// workspace. It never widens to other projects allowed by the service token.
func (c *Client) Projects(workspace string) []string {
	if c == nil {
		return nil
	}
	mapping, ok := c.config.Workspaces[workspace]
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	if mapping.DefaultProject != "" {
		seen[mapping.DefaultProject] = true
	}
	for _, project := range mapping.Projects {
		if project != "" {
			seen[project] = true
		}
	}
	projects := make([]string, 0, len(seen))
	for project := range seen {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	return projects
}

func (c *Client) request(ctx context.Context, method, path string, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.config.URL, "/")+"/v2/knowledge/"+path, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("invalid brain request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("brain unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ResponseError{Status: resp.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(raw) > maxResponse {
		return nil, errors.New("brain response unreadable or oversized")
	}
	var envelope struct {
		Schema   string `json:"schemaVersion"`
		ServedAt string `json:"servedAt"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Schema != "silvermediums.brain.v2" || envelope.ServedAt == "" {
		return nil, errors.New("unsupported brain response")
	}
	return raw, nil
}

type ResponseError struct{ Status int }

func (e *ResponseError) Error() string {
	return fmt.Sprintf("brain request rejected (HTTP %d)", e.Status)
}

// Context contains reference excerpts and immutable read references. It is not
// permission, a verified answer or a substitute for reading the full revision.
func (c *Client) Context(ctx context.Context, project, query string) (json.RawMessage, error) {
	raw, err := c.request(ctx, http.MethodGet, "context?"+url.Values{"project": {project}, "q": {query}}.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if len(raw) > 64*1024 || json.Unmarshal(raw, &response) != nil || response.Project.ID != project {
		return nil, errors.New("brain project mismatch")
	}
	return json.RawMessage(raw), nil
}

func (c *Client) Read(ctx context.Context, project, id, revision string) (json.RawMessage, error) {
	if _, err := uuid.Parse(id); err != nil || len(revision) != 64 {
		return nil, errors.New("immutable document id and revision required")
	}
	raw, err := c.request(ctx, http.MethodGet, "documents/"+url.PathEscape(id)+"?"+url.Values{"revision": {revision}}.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Document struct {
			Project   string `json:"project"`
			ID        string `json:"id"`
			Revision  string `json:"revision"`
			Truncated bool   `json:"truncated"`
		} `json:"document"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Document.Project != project || response.Document.ID != id || response.Document.Revision != revision || response.Document.Truncated {
		return nil, errors.New("document is unavailable in this task project or revision")
	}
	return raw, nil
}

func (c *Client) Send(ctx context.Context, payload []byte) error {
	raw, err := c.request(ctx, http.MethodPost, "events", payload)
	if err != nil {
		return err
	}
	var response struct {
		Recorded struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Acceptance string `json:"acceptance"`
		} `json:"recorded"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Recorded.Status != "task_observation" || response.Recorded.Acceptance != "not_established" {
		return errors.New("brain durable event receipt unavailable")
	}
	id, err := uuid.Parse(response.Recorded.ID)
	if err != nil || id == uuid.Nil {
		return errors.New("brain durable event receipt unavailable")
	}
	return nil
}

func (c *Client) IngestHistory(ctx context.Context, payload []byte) (json.RawMessage, error) {
	raw, err := c.request(ctx, http.MethodPost, "history/fragments", payload)
	if err != nil {
		return nil, err
	}
	var response struct {
		Recorded struct {
			Submitted   int    `json:"submitted"`
			Inserted    int    `json:"inserted"`
			Unchanged   int    `json:"unchanged"`
			RecordClass string `json:"recordClass"`
		} `json:"recorded"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Recorded.RecordClass != "conversation_recollection" ||
		response.Recorded.Submitted < 1 || response.Recorded.Inserted < 0 || response.Recorded.Unchanged < 0 ||
		response.Recorded.Inserted+response.Recorded.Unchanged != response.Recorded.Submitted {
		return nil, errors.New("brain history receipt unavailable")
	}
	return json.RawMessage(raw), nil
}

func (c *Client) SearchHistory(ctx context.Context, projects []string, query string, limit int) (json.RawMessage, error) {
	if len(projects) == 0 || len(projects) > 100 || strings.TrimSpace(query) == "" || len(query) > 2000 || limit < 1 || limit > 50 {
		return nil, errors.New("invalid brain history search")
	}
	for _, project := range projects {
		if strings.TrimSpace(project) == "" {
			return nil, errors.New("invalid brain history search")
		}
	}
	payload, err := json.Marshal(map[string]any{"query": query, "limit": limit, "projects": projects})
	if err != nil {
		return nil, errors.New("invalid brain history search")
	}
	raw, err := c.request(ctx, http.MethodPost, "history/search", payload)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(projects))
	for _, project := range projects {
		allowed[project] = true
	}
	var response struct {
		History struct {
			Groups []struct {
				Project string `json:"project"`
				Matches []struct {
					Project     string `json:"project"`
					RecordClass string `json:"recordClass"`
					Assured     bool   `json:"assured"`
					Accepted    bool   `json:"accepted"`
					Authority   string `json:"authority"`
				} `json:"matches"`
			} `json:"groups"`
		} `json:"history"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return nil, errors.New("brain history response unreadable")
	}
	for _, group := range response.History.Groups {
		if !allowed[group.Project] {
			return nil, errors.New("brain history project mismatch")
		}
		for _, match := range group.Matches {
			if match.Project != group.Project || match.RecordClass != "conversation_recollection" || match.Assured || match.Accepted || match.Authority != "none" {
				return nil, errors.New("brain history trust boundary mismatch")
			}
		}
	}
	return json.RawMessage(raw), nil
}
