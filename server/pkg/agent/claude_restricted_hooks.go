package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/pkg/taskpolicy"
)

const (
	restrictedClaudeHookBodyLimit = 64 << 10
	restrictedClaudeHookTimeout   = 5 * time.Second
	restrictedClaudeMaxToolCalls  = 32
)

type restrictedClaudeHookSettings struct {
	Hooks map[string][]restrictedClaudeHookMatcher `json:"hooks"`
}

type restrictedClaudeHookMatcher struct {
	Matcher string                      `json:"matcher"`
	Hooks   []restrictedClaudeHookEntry `json:"hooks"`
}

type restrictedClaudeHookEntry struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Timeout   int    `json:"timeout"`
	OnFailure string `json:"onFailure,omitempty"`
}

type restrictedClaudeHookEvent struct {
	EventName    string          `json:"hook_event_name"`
	ToolName     string          `json:"tool_name"`
	ToolInput    map[string]any  `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
	Error        string          `json:"error"`
	ToolUseID    string          `json:"tool_use_id"`
}

type restrictedClaudeHookCall struct {
	tool      string
	input     map[string]any
	startedAt time.Time
	done      chan struct{}
	duration  time.Duration
	completed bool
}

type restrictedClaudeHooks struct {
	policy *taskpolicy.ExecutionPolicy
	path   string

	mu             sync.Mutex
	calls          map[string]*restrictedClaudeHookCall
	authorizedDocs map[string]string
	callCount      int
	failure        string
	server         *http.Server
	listener       net.Listener
}

func startRestrictedClaudeHooks(policy *taskpolicy.ExecutionPolicy) (*restrictedClaudeHooks, string, error) {
	if err := taskpolicy.Validate(policy); err != nil {
		return nil, "", err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("listen for restricted Claude policy hooks: %w", err)
	}
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		_ = listener.Close()
		return nil, "", fmt.Errorf("create restricted Claude hook token: %w", err)
	}
	hooks := &restrictedClaudeHooks{
		policy:         policy,
		path:           "/" + hex.EncodeToString(token),
		calls:          make(map[string]*restrictedClaudeHookCall),
		authorizedDocs: make(map[string]string),
		listener:       listener,
	}
	hooks.server = &http.Server{
		Handler:           http.HandlerFunc(hooks.serveHTTP),
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		_ = hooks.server.Serve(listener)
	}()
	return hooks, "http://" + listener.Addr().String() + hooks.path, nil
}

func (hooks *restrictedClaudeHooks) Close() {
	if hooks == nil || hooks.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = hooks.server.Shutdown(ctx)
}

func (hooks *restrictedClaudeHooks) settingsJSON(url string) (json.RawMessage, error) {
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		return nil, errors.New("restricted Claude hook URL must use loopback")
	}
	hook := restrictedClaudeHookEntry{
		Type:      "http",
		URL:       url,
		Timeout:   2,
		OnFailure: "block",
	}
	settings, err := json.Marshal(restrictedClaudeHookSettings{
		Hooks: map[string][]restrictedClaudeHookMatcher{
			"PreToolUse": {{
				Matcher: "*",
				Hooks:   []restrictedClaudeHookEntry{hook},
			}},
			"PostToolUse": {{
				Matcher: "^mcp__",
				Hooks:   []restrictedClaudeHookEntry{hook},
			}},
			"PostToolUseFailure": {{
				Matcher: "^mcp__",
				Hooks:   []restrictedClaudeHookEntry{hook},
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal restricted Claude policy hooks: %w", err)
	}
	return settings, nil
}

func (hooks *restrictedClaudeHooks) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != hooks.path {
		http.NotFound(w, r)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, restrictedClaudeHookBodyLimit+1))
	if err != nil || len(raw) > restrictedClaudeHookBodyLimit {
		hooks.setFailure("restricted Claude policy hook received invalid input")
		hooks.writeDecision(w, "deny")
		return
	}
	var event restrictedClaudeHookEvent
	if json.Unmarshal(raw, &event) != nil {
		hooks.setFailure("restricted Claude policy hook received unreadable input")
		hooks.writeDecision(w, "deny")
		return
	}

	switch event.EventName {
	case "PreToolUse":
		hooks.handlePreToolUse(w, event)
	case "PostToolUse", "PostToolUseFailure":
		hooks.handlePostToolUse(w, event)
	default:
		hooks.setFailure("restricted Claude policy hook received an unknown event")
		hooks.writeDecision(w, "deny")
	}
}

func (hooks *restrictedClaudeHooks) handlePreToolUse(w http.ResponseWriter, event restrictedClaudeHookEvent) {
	tool, allowed := restrictedClaudeToolName(hooks.policy, event.ToolName)
	if !allowed || event.ToolUseID == "" || !hooks.allowsToolInput(tool, event.ToolInput) {
		hooks.setFailure("restricted Claude attempted an out-of-scope tool call")
		hooks.writeDecision(w, "deny")
		return
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	if hooks.failure != "" {
		hooks.writeDecision(w, "deny")
		return
	}
	if hooks.callCount >= restrictedClaudeMaxToolCalls {
		hooks.failure = "restricted Claude exceeded the task tool-call limit"
		hooks.writeDecision(w, "deny")
		return
	}
	if _, exists := hooks.calls[event.ToolUseID]; exists {
		hooks.failure = "restricted Claude policy hook received a duplicate tool call identifier"
		hooks.writeDecision(w, "deny")
		return
	}
	hooks.callCount++
	hooks.calls[event.ToolUseID] = &restrictedClaudeHookCall{
		tool:      tool,
		input:     event.ToolInput,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}
	hooks.writeDecision(w, "allow")
}

func (hooks *restrictedClaudeHooks) handlePostToolUse(w http.ResponseWriter, event restrictedClaudeHookEvent) {
	hooks.mu.Lock()
	call, exists := hooks.calls[event.ToolUseID]
	if event.ToolUseID == "" || !exists || call.completed || call.tool != toolNameFromMCPID(hooks.policy, event.ToolName) {
		hooks.failure = "restricted Claude policy hook could not correlate a completed MCP call"
		hooks.mu.Unlock()
		hooks.writeDecision(w, "")
		return
	}
	if event.EventName == "PostToolUseFailure" {
		if !restrictedClaudeExpectedResolveFailure(event, call) {
			hooks.failure = "restricted Claude Brain MCP call failed"
		}
		call.duration = time.Since(call.startedAt)
		call.completed = true
		close(call.done)
		hooks.mu.Unlock()
		hooks.writeDecision(w, "")
		return
	}

	call.duration = time.Since(call.startedAt)
	call.completed = true
	close(call.done)
	hooks.mu.Unlock()
	if call.tool == "brain_context" {
		if !hooks.captureContextDocuments(event.ToolResponse) {
			hooks.setFailure("restricted Claude could not verify the scoped Brain context response")
		}
	}
	hooks.writeDecision(w, "")
}

func restrictedClaudeExpectedResolveFailure(event restrictedClaudeHookEvent, call *restrictedClaudeHookCall) bool {
	if call.tool != "brain_resolve" || len(call.input) != 1 ||
		call.input["identity"] != "not-an-enrolled-project" {
		return false
	}
	if event.Error == "brain request rejected (HTTP 404)" {
		return true
	}
	blocks, ok := restrictedClaudeHookResponseBlocks(event.ToolResponse)
	return ok && restrictedClaudeExpectedUnknownProjectRefusal(blocks, call.input)
}

func (hooks *restrictedClaudeHooks) allowsToolInput(tool string, input map[string]any) bool {
	if !taskpolicy.AllowsToolInput(hooks.policy, tool, input) {
		return false
	}
	if tool != "brain_read" {
		return true
	}
	id, idOK := input["id"].(string)
	revision, revisionOK := input["revision"].(string)
	if !idOK || !revisionOK {
		return false
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	return hooks.authorizedDocs[id] == revision
}

func (hooks *restrictedClaudeHooks) captureContextDocuments(raw json.RawMessage) bool {
	blocks, ok := restrictedClaudeHookResponseBlocks(raw)
	if !ok {
		return false
	}
	receipt := codexMCPBrainSafeReceipt("brain_context", map[string]any{
		"result": map[string]any{"isError": false, "content": blocks},
	})
	if receipt == "" {
		return false
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(receipt), &parsed) != nil ||
		parsed["connectionState"] != "online" ||
		parsed["projectId"] != hooks.policy.ProjectID ||
		!restrictedBrainReceiptIsVerifiable("brain_context", parsed) {
		return false
	}
	documents, ok := parsed["documents"].([]any)
	if !ok {
		return false
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	for _, item := range documents {
		document, ok := item.(map[string]any)
		if !ok {
			return false
		}
		id, idOK := document["id"].(string)
		revision, revisionOK := document["revision"].(string)
		if !idOK || !revisionOK {
			return false
		}
		hooks.authorizedDocs[id] = revision
	}
	return true
}

func restrictedClaudeHookResponseBlocks(raw json.RawMessage) ([]any, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var response struct {
		Content []any `json:"content"`
		IsError *bool `json:"isError"`
	}
	if json.Unmarshal(raw, &response) == nil && response.Content != nil &&
		(response.IsError == nil || !*response.IsError) {
		return response.Content, true
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return []any{map[string]any{"type": "text", "text": text}}, true
	}
	return nil, false
}

func toolNameFromMCPID(policy *taskpolicy.ExecutionPolicy, id string) string {
	name, ok := restrictedClaudeToolName(policy, id)
	if !ok {
		return ""
	}
	return name
}

func (hooks *restrictedClaudeHooks) WaitDuration(ctx context.Context, toolUseID, tool string) (time.Duration, bool) {
	hooks.mu.Lock()
	call, exists := hooks.calls[toolUseID]
	if !exists || call.tool != tool {
		hooks.mu.Unlock()
		return 0, false
	}
	done := call.done
	hooks.mu.Unlock()

	timer := time.NewTimer(restrictedClaudeHookTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	case <-ctx.Done():
		return 0, false
	}

	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	if hooks.failure != "" || hooks.calls[toolUseID] != call || call.duration == 0 {
		return 0, false
	}
	delete(hooks.calls, toolUseID)
	return call.duration, true
}

func (hooks *restrictedClaudeHooks) Failure() string {
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	return hooks.failure
}

func (hooks *restrictedClaudeHooks) HasPendingCalls() bool {
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	return len(hooks.calls) > 0
}

func (hooks *restrictedClaudeHooks) setFailure(message string) {
	hooks.mu.Lock()
	hooks.failure = message
	hooks.mu.Unlock()
}

func (hooks *restrictedClaudeHooks) writeDecision(w http.ResponseWriter, decision string) {
	w.Header().Set("Content-Type", "application/json")
	if decision == "" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return
	}
	encoded, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       decision,
			"permissionDecisionReason": "Task execution policy only permits the exact approved Brain MCP read scope.",
		},
	})
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
