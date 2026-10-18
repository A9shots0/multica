package developmentmemory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeout   = 5 * time.Second
	maxResponseBytes = 128 << 10
)

// Repository is the bounded repository metadata Multica already resolved for
// a claimed task. Ref may be empty when Multica does not know the default
// branch; Intelligence preserves that as unknown instead of guessing "main".
type Repository struct {
	URL string
	Ref string
}

// Task is the server-owned issue context used to resolve one durable
// workstream in Intelligence. Credentials never cross this boundary into the
// daemon payload. Task-session receipts are intentionally a later, durable
// lifecycle integration rather than a claim-time write that could remain
// falsely active if dispatch fails after this call.
type Task struct {
	IssueID         string
	IssueIdentifier string
	Title           string
	Goal            string
	ProjectID       string
	ProjectName     string
	WorkspaceID     string
	WorkspaceSlug   string
	Repositories    []Repository
}

// Preparer is the claim-time boundary used by the Multica handler. A prepared
// packet is safe to send to the coding daemon; implementation credentials are
// retained only by the server-side client.
type Preparer interface {
	PrepareContext(context.Context, Task) (json.RawMessage, error)
}

type unavailablePreparer struct{ cause error }

// FailClosed keeps a partially configured deployment from silently launching
// repository-backed tasks without the memory the operator intended to require.
func FailClosed(cause error) Preparer {
	if cause == nil {
		cause = errors.New("development memory is unavailable")
	}
	return unavailablePreparer{cause: cause}
}

func (u unavailablePreparer) PrepareContext(context.Context, Task) (json.RawMessage, error) {
	return nil, u.cause
}

type Config struct {
	BaseURL     string
	WriterToken string
	ReaderToken string
	HTTPClient  *http.Client
}

type Client struct {
	baseURL     string
	writerToken string
	readerToken string
	http        *http.Client
}

type repositoryRegistration struct {
	CanonicalIdentity string  `json:"canonical_identity"`
	RemoteURL         string  `json:"remote_url"`
	DefaultBranch     *string `json:"default_branch"`
	Role              string  `json:"role"`
}

type contextPacket struct {
	SchemaVersion int                 `json:"schema_version"`
	Workstreams   []contextWorkstream `json:"workstreams"`
}

type contextWorkstream struct {
	ID           string              `json:"id"`
	Repositories []contextRepository `json:"repositories"`
}

type contextRepository struct {
	CanonicalIdentity string `json:"canonical_identity"`
}

func New(cfg Config) (*Client, error) {
	base, err := validateBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	writer := strings.TrimSpace(cfg.WriterToken)
	reader := strings.TrimSpace(cfg.ReaderToken)
	if writer == "" || reader == "" {
		return nil, errors.New("development memory requires both writer and reader tokens")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Client{
		baseURL:     base,
		writerToken: writer,
		readerToken: reader,
		http:        httpClient,
	}, nil
}

func validateBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", errors.New("development memory base URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", errors.New("development memory base URL is invalid")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("development memory base URL must not contain credentials, query, or fragment")
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		host := strings.TrimSpace(parsed.Hostname())
		ip := net.ParseIP(host)
		if !strings.EqualFold(host, "localhost") && !strings.EqualFold(host, "host.docker.internal") && (ip == nil || !ip.IsLoopback()) {
			return "", errors.New("unencrypted development memory URL must be loopback or the local Docker host")
		}
	default:
		return "", errors.New("development memory base URL must use https or loopback http")
	}
	return raw, nil
}

func (c *Client) PrepareContext(ctx context.Context, task Task) (json.RawMessage, error) {
	if strings.TrimSpace(task.IssueID) == "" {
		return nil, errors.New("development memory issue identity is required")
	}
	repositories, err := normalizeRepositories(task.Repositories)
	if err != nil {
		return nil, err
	}
	if len(repositories) == 0 {
		return nil, nil
	}
	workstreamID := "multica-issue-" + strings.TrimSpace(task.IssueID)
	raw, packet, err := c.readRawContext(ctx, workstreamID)
	if err != nil {
		return nil, fmt.Errorf("read development memory context: %w", err)
	}
	if len(packet.Workstreams) == 0 {
		if err := c.registerWorkstream(ctx, task, workstreamID, repositories); err != nil {
			return nil, fmt.Errorf("register development memory workstream: %w", err)
		}
		raw, packet, err = c.readRawContext(ctx, workstreamID)
		if err != nil {
			return nil, fmt.Errorf("read registered development memory context: %w", err)
		}
	}
	if err := validateBoundRepositories(workstreamID, packet, repositories); err != nil {
		return nil, err
	}
	return raw, nil
}

// Deliver writes only a human-approved proposal. Every Intelligence endpoint
// is idempotent, so the worker may replay this whole ordered sequence after a
// timeout without duplicating durable memory.
func (c *Client) Deliver(ctx context.Context, delivery Delivery) error {
	finalStatus := normalizeTaskStatus(delivery.TaskStatus)
	if finalStatus == "" {
		return errors.New("approved development memory delivery has an invalid task status")
	}
	taskSessionID := "multica-task-" + strings.TrimSpace(delivery.TaskID)
	repositories := make([]map[string]any, 0, len(delivery.Repositories))
	for _, repo := range delivery.Repositories {
		repositories = append(repositories, map[string]any{
			"canonical_identity": repo.CanonicalIdentity,
			"base_revision":      repo.BaseRevision,
			"branch":             repo.Branch,
		})
	}
	if err := c.write(ctx, "/v1/development/task-sessions", map[string]any{
		"producer": "multica", "workstream_id": delivery.WorkstreamID,
		"session": map[string]any{
			"id": taskSessionID, "agent": boundedText(delivery.AgentName, 80),
			"external_task_id": delivery.TaskID, "status": "running",
			"started_at": delivery.TaskStartedAt.UTC().Format(time.RFC3339Nano),
		},
		"repositories": repositories,
	}); err != nil {
		return fmt.Errorf("register approved task session: %w", err)
	}
	for index, item := range delivery.Payload.MemoryItems {
		if err := c.write(ctx, "/v1/development/memory-items", map[string]any{
			"producer": "multica", "workstream_id": delivery.WorkstreamID,
			"task_session_id": taskSessionID, "repository_identity": item.RepositoryIdentity,
			"scope": item.Scope, "kind": item.Kind, "statement": item.Statement, "status": "active",
			"supersedes_id": nil, "evidence_refs": item.EvidenceRefs,
			"idempotency_key": fmt.Sprintf("multica-proposal-%s-item-%d", delivery.ProposalID, index),
		}); err != nil {
			return fmt.Errorf("write approved memory item %d: %w", index, err)
		}
	}
	if err := c.write(ctx, "/v1/development/checkpoints", map[string]any{
		"producer": "multica", "workstream_id": delivery.WorkstreamID,
		"task_session_id":       taskSessionID,
		"idempotency_key":       "multica-proposal-" + delivery.ProposalID + "-checkpoint",
		"summary":               delivery.Payload.Summary,
		"repository_evidence":   delivery.Payload.RepositoryEvidence,
		"remaining_uncertainty": delivery.Payload.RemainingUncertainty,
		"next_action":           delivery.Payload.NextAction,
	}); err != nil {
		return fmt.Errorf("write approved checkpoint: %w", err)
	}
	if err := c.write(ctx, "/v1/development/task-sessions/"+url.PathEscape(taskSessionID)+"/complete", map[string]any{
		"producer": "multica", "status": finalStatus, "completed_at": delivery.TaskCompletedAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("complete approved task session: %w", err)
	}
	return nil
}

func normalizeRepositories(input []Repository) ([]repositoryRegistration, error) {
	result := make([]repositoryRegistration, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		remote := strings.TrimSpace(item.URL)
		if remote == "" {
			continue
		}
		identity, err := canonicalIdentity(remote)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		remote = canonicalRemoteURL(remote)
		var branch *string
		if ref := strings.TrimPrefix(strings.TrimSpace(item.Ref), "refs/heads/"); ref != "" {
			branch = &ref
		}
		result = append(result, repositoryRegistration{
			CanonicalIdentity: identity,
			RemoteURL:         remote,
			DefaultBranch:     branch,
			Role:              "task_repository",
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CanonicalIdentity < result[j].CanonicalIdentity
	})
	return result, nil
}

func canonicalRemoteURL(remote string) string {
	if !strings.HasPrefix(remote, "git@") {
		return remote
	}
	parts := strings.SplitN(strings.TrimPrefix(remote, "git@"), ":", 2)
	return "ssh://git@" + parts[0] + "/" + strings.TrimLeft(parts[1], "/")
}

func canonicalIdentity(remote string) (string, error) {
	var host, path string
	if strings.HasPrefix(remote, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(remote, "git@"), ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(remote, "?#") {
			return "", errors.New("development memory repository URL is not canonical")
		}
		host, path = parts[0], parts[1]
	} else {
		parsed, err := url.Parse(remote)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "ssh") {
			return "", errors.New("development memory repository URL must be canonical https or ssh Git URL")
		}
		if parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", errors.New("development memory repository URL contains unsupported data")
		}
		if parsed.User != nil {
			_, hasPassword := parsed.User.Password()
			if hasPassword || parsed.Scheme != "ssh" || parsed.User.Username() != "git" {
				return "", errors.New("development memory repository URL contains unsupported user information")
			}
		}
		host, path = parsed.Hostname(), parsed.Path
	}
	path = strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	if path == "" || strings.Contains(path, "..") {
		return "", errors.New("development memory repository URL has no stable identity")
	}
	identity := strings.ToLower(path)
	if !strings.EqualFold(host, "github.com") {
		identity = strings.ToLower(host) + "/" + identity
	}
	return identity, nil
}

func (c *Client) registerWorkstream(ctx context.Context, task Task, workstreamID string, repos []repositoryRegistration) error {
	projectID := firstNonEmpty(task.ProjectID, task.WorkspaceID)
	projectName := boundedText(firstNonEmpty(task.ProjectName, task.WorkspaceSlug, "Multica workspace"), 160)
	title := boundedText(firstNonEmpty(task.Title, task.IssueIdentifier, task.IssueID), 240)
	goal := boundedText(firstNonEmpty(task.Goal, title), 4000)
	payload := map[string]any{
		"producer": "multica",
		"project":  map[string]string{"id": projectID, "name": projectName},
		"workstream": map[string]string{
			"id": workstreamID, "slug": workstreamID, "title": title,
			"goal": goal, "status": "active",
		},
		"repositories": repos,
	}
	return c.write(ctx, "/v1/development/workstreams", payload)
}

func validateBoundRepositories(workstreamID string, packet contextPacket, repos []repositoryRegistration) error {
	if len(packet.Workstreams) != 1 || packet.Workstreams[0].ID != workstreamID {
		return errors.New("development memory returned an unexpected workstream")
	}
	bound := make(map[string]struct{}, len(packet.Workstreams[0].Repositories))
	for _, repo := range packet.Workstreams[0].Repositories {
		bound[strings.ToLower(strings.TrimSpace(repo.CanonicalIdentity))] = struct{}{}
	}
	for _, repo := range repos {
		if _, ok := bound[repo.CanonicalIdentity]; !ok {
			return fmt.Errorf("development memory workstream is missing repository %s", repo.CanonicalIdentity)
		}
	}
	return nil
}

func (c *Client) readRawContext(ctx context.Context, workstreamID string) (json.RawMessage, contextPacket, error) {
	endpoint := c.baseURL + "/v1/development/context?workstream_id=" + url.QueryEscape(workstreamID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, contextPacket{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.readerToken)
	req.Header.Set("X-SMI-Reader", "multica")
	raw, err := c.do(req, http.StatusOK)
	if err != nil {
		return nil, contextPacket{}, err
	}
	var packet contextPacket
	if err := json.Unmarshal(raw, &packet); err != nil || packet.SchemaVersion != 1 {
		return nil, contextPacket{}, errors.New("development memory returned an invalid context packet")
	}
	return json.RawMessage(raw), packet, nil
}

func (c *Client) write(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.writerToken)
	req.Header.Set("Content-Type", "application/json")
	_, err = c.do(req, http.StatusOK, http.StatusCreated)
	return err
}

func (c *Client) do(req *http.Request, accepted ...int) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxResponseBytes {
		return nil, errors.New("development memory response exceeded the size limit")
	}
	for _, status := range accepted {
		if resp.StatusCode == status {
			return raw, nil
		}
	}
	return nil, fmt.Errorf("development memory returned HTTP %d", resp.StatusCode)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func boundedText(value string, maximum int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maximum {
		return string(runes)
	}
	if maximum < 2 {
		return string(runes[:maximum])
	}
	return string(runes[:maximum-1]) + "…"
}
