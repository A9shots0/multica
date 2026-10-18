package taskpolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

const (
	BrainMCPReadOnlyMode     = "brain_mcp_read_only"
	BrainMCPReadOnlyProvider = "claude"
)

var brainMCPServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

var brainMCPToolNames = [...]string{
	"brain_resolve",
	"brain_context",
	"brain_read",
	"brain_company_projects",
	"brain_projects",
	"company_events",
}

// Request is the minimal owner-authorized opt-in accepted on a one-task rerun.
// The tool set and provider are fixed by this server-side policy.
type Request struct {
	Mode      string `json:"mode"`
	MCPServer string `json:"mcp_server"`
}

// ExecutionPolicy is persisted with a single task and echoed in its claim.
type ExecutionPolicy struct {
	Mode      string   `json:"mode"`
	Provider  string   `json:"provider"`
	MCPServer string   `json:"mcp_server"`
	Tools     []string `json:"tools"`
}

func NewBrainMCPReadOnly(request Request) (*ExecutionPolicy, error) {
	if request.Mode != BrainMCPReadOnlyMode {
		return nil, fmt.Errorf("unsupported task execution policy mode %q", request.Mode)
	}
	if !brainMCPServerNamePattern.MatchString(request.MCPServer) {
		return nil, errors.New("task execution policy requires a valid MCP server name")
	}
	return &ExecutionPolicy{
		Mode:      BrainMCPReadOnlyMode,
		Provider:  BrainMCPReadOnlyProvider,
		MCPServer: request.MCPServer,
		Tools:     BrainMCPToolIDs(request.MCPServer),
	}, nil
}

func Validate(policy *ExecutionPolicy) error {
	if policy == nil {
		return nil
	}
	if policy.Mode != BrainMCPReadOnlyMode {
		return fmt.Errorf("unsupported task execution policy mode %q", policy.Mode)
	}
	if policy.Provider != BrainMCPReadOnlyProvider {
		return fmt.Errorf("task execution policy requires provider %q", BrainMCPReadOnlyProvider)
	}
	if !brainMCPServerNamePattern.MatchString(policy.MCPServer) {
		return errors.New("task execution policy requires a valid MCP server name")
	}
	expected := BrainMCPToolIDs(policy.MCPServer)
	if len(policy.Tools) != len(expected) {
		return errors.New("task execution policy must contain the exact Brain MCP tool allowlist")
	}
	got := make(map[string]struct{}, len(policy.Tools))
	for _, tool := range policy.Tools {
		if _, exists := got[tool]; exists {
			return errors.New("task execution policy contains a duplicate MCP tool")
		}
		got[tool] = struct{}{}
	}
	for _, tool := range expected {
		if _, ok := got[tool]; !ok {
			return errors.New("task execution policy must contain the exact Brain MCP tool allowlist")
		}
	}
	return nil
}

func BrainMCPToolIDs(serverName string) []string {
	tools := make([]string, len(brainMCPToolNames))
	for index, name := range brainMCPToolNames {
		tools[index] = "mcp__" + serverName + "__" + name
	}
	return tools
}

func IsBrainMCPReadOnlyTool(policy *ExecutionPolicy, tool string) bool {
	if policy == nil || Validate(policy) != nil {
		return false
	}
	for _, allowed := range policy.Tools {
		if tool == allowed {
			return true
		}
	}
	return false
}

// FromTaskContext reads the opt-in policy from agent_task_queue.context. Other
// context fields, such as head_sha, remain opaque to this package.
func FromTaskContext(raw []byte) (*ExecutionPolicy, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var context map[string]json.RawMessage
	if err := json.Unmarshal(raw, &context); err != nil {
		return nil, fmt.Errorf("parse task context: %w", err)
	}
	policyJSON, ok := context["execution_policy"]
	if !ok || bytes.Equal(bytes.TrimSpace(policyJSON), []byte("null")) {
		return nil, nil
	}
	var policy ExecutionPolicy
	if err := json.Unmarshal(policyJSON, &policy); err != nil {
		return nil, fmt.Errorf("parse task execution policy: %w", err)
	}
	if err := Validate(&policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// FilterMCPConfig returns a fresh Claude MCP config containing only the
// policy's already-configured server. It never adds, edits, or mutates a
// shared agent/runtime configuration.
func FilterMCPConfig(raw json.RawMessage, policy *ExecutionPolicy) (json.RawMessage, error) {
	if err := Validate(policy); err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, errors.New("task execution policy is required")
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, errors.New("task execution policy requires a managed MCP config")
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse effective MCP config: %w", err)
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(config["mcpServers"], &servers); err != nil || servers == nil {
		return nil, errors.New("task execution policy requires a managed mcpServers map")
	}
	server, ok := servers[policy.MCPServer]
	if !ok || len(server) == 0 || string(server) == "null" {
		return nil, fmt.Errorf("authorized MCP server %q is not present in the effective config", policy.MCPServer)
	}
	filtered, err := json.Marshal(map[string]any{
		"mcpServers": map[string]json.RawMessage{policy.MCPServer: server},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal task MCP config: %w", err)
	}
	return filtered, nil
}
