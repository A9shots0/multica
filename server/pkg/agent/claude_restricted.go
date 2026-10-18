package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/pkg/taskpolicy"
)

const (
	// --restricted first appeared in this supported version. --tools "" is
	// still required because restricted mode alone leaves file tools enabled.
	minRestrictedClaudeVersion = "2.1.248"
	restrictedClaudeAgentName  = "brain-reader"
)

var claudeVersionPattern = regexp.MustCompile(`(?:^|[^0-9])([0-9]+)\.([0-9]+)\.([0-9]+)(?:$|[ )])`)

type restrictedClaudeAgent struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools"`
}

func validateRestrictedClaudeConfig(cfg Config, opts ExecOptions) (json.RawMessage, error) {
	if err := taskpolicy.Validate(opts.ExecutionPolicy); err != nil {
		return nil, err
	}
	if opts.ExecutionPolicy == nil {
		return nil, errors.New("restricted Claude execution policy is required")
	}
	if !cfg.BuiltinRuntime {
		return nil, errors.New("restricted Claude execution requires the verified built-in Claude Code runtime")
	}
	if !claudeVersionAtLeast(cfg.CLIVersion, minRestrictedClaudeVersion) {
		return nil, fmt.Errorf("restricted Claude execution requires Claude Code %s or newer; detected %q", minRestrictedClaudeVersion, cfg.CLIVersion)
	}
	return taskpolicy.FilterMCPConfig(opts.McpConfig, opts.ExecutionPolicy)
}

func claudeVersionAtLeast(value, minimum string) bool {
	parse := func(version string) ([3]uint64, bool) {
		match := claudeVersionPattern.FindStringSubmatch(strings.TrimSpace(version))
		if len(match) != 4 {
			return [3]uint64{}, false
		}
		var parts [3]uint64
		for i := range parts {
			var n uint64
			if _, err := fmt.Sscanf(match[i+1], "%d", &n); err != nil {
				return [3]uint64{}, false
			}
			parts[i] = n
		}
		return parts, true
	}
	got, ok := parse(value)
	if !ok {
		return false
	}
	want, ok := parse(minimum)
	if !ok {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return got[i] > want[i]
		}
	}
	return true
}

func buildRestrictedClaudeArgs(opts ExecOptions) ([]string, error) {
	if err := taskpolicy.Validate(opts.ExecutionPolicy); err != nil {
		return nil, err
	}
	if opts.ExecutionPolicy == nil {
		return nil, errors.New("restricted Claude execution policy is required")
	}
	agents, err := json.Marshal(map[string]restrictedClaudeAgent{
		restrictedClaudeAgentName: {
			Description: "Retrieve only the explicitly authorized Brain MCP sources and return verified, minimal receipts.",
			Prompt:      "Use only the configured Brain MCP tools. Do not use local files, shell commands, or any other tools. Return no source bodies or private prose.",
			Tools:       append([]string(nil), opts.ExecutionPolicy.Tools...),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal restricted Claude agent: %w", err)
	}
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
		"--restricted",
		"--no-session-persistence",
		// The exact main-agent allowlist below re-enables only authorized MCP
		// tools; strict MCP config also excludes managed/user MCP additions.
		"--tools", "",
		"--agents", string(agents),
		"--agent", restrictedClaudeAgentName,
		// A conflicting managed MCP file causes CLI startup to fail closed.
		"--strict-mcp-config",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.ThinkingLevel != "" {
		args = append(args, "--effort", opts.ThinkingLevel)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	return args, nil
}

type restrictedClaudeCall struct {
	tool      string
	startedAt time.Time
	input     map[string]any
}

func restrictedClaudeToolName(policy *taskpolicy.ExecutionPolicy, id string) (string, bool) {
	if policy == nil || !strings.HasPrefix(id, "mcp__"+policy.MCPServer+"__") {
		return "", false
	}
	name := strings.TrimPrefix(id, "mcp__"+policy.MCPServer+"__")
	if !taskpolicy.IsBrainMCPReadOnlyTool(policy, id) {
		return "", false
	}
	return name, true
}

func (b *claudeBackend) handleRestrictedAssistant(msg claudeSDKMessage, ch chan<- Message, usage map[string]TokenUsage, policy *taskpolicy.ExecutionPolicy, calls map[string]restrictedClaudeCall) (assistantTurn, string) {
	var content claudeMessageContent
	if err := json.Unmarshal(msg.Message, &content); err != nil {
		return assistantTurn{}, "restricted Claude emitted an unreadable assistant event"
	}
	if content.Usage != nil && content.Model != "" {
		u := usage[content.Model]
		u.InputTokens += content.Usage.InputTokens
		u.OutputTokens += content.Usage.OutputTokens
		u.CacheReadTokens += content.Usage.CacheReadInputTokens
		u.CacheWriteTokens += content.Usage.CacheCreationInputTokens
		usage[content.Model] = u
	}
	turn := assistantTurn{understood: true}
	for _, block := range content.Content {
		if block.Type != "tool_use" {
			continue
		}
		turn.toolUses++
		// The CLI may retain this non-data end-turn control with MCP tools.
		// It cannot read/write data and is never emitted as a receipt.
		if block.Name == "EndConversation" {
			if block.ID != "" {
				if _, exists := calls[block.ID]; exists {
					return turn, "restricted Claude emitted a duplicate tool call identifier"
				}
				calls[block.ID] = restrictedClaudeCall{tool: "EndConversation", startedAt: time.Now()}
			}
			continue
		}
		tool, allowed := restrictedClaudeToolName(policy, block.Name)
		if !allowed || block.ID == "" {
			return turn, "restricted Claude attempted an unauthorized or uncorrelated tool call"
		}
		input := make(map[string]any)
		if len(block.Input) == 0 || json.Unmarshal(block.Input, &input) != nil || !restrictedClaudeToolInputAllowed(tool, input) {
			return turn, "restricted Claude attempted a tool call outside the authorized read scope"
		}
		if _, exists := calls[block.ID]; exists {
			return turn, "restricted Claude emitted a duplicate tool call identifier"
		}
		calls[block.ID] = restrictedClaudeCall{tool: tool, startedAt: time.Now(), input: input}
		trySend(ch, Message{Type: MessageToolUse, Tool: block.Name, CallID: block.ID})
	}
	return turn, ""
}

func (b *claudeBackend) handleRestrictedUser(msg claudeSDKMessage, ch chan<- Message, policy *taskpolicy.ExecutionPolicy, calls map[string]restrictedClaudeCall) (string, []json.RawMessage, bool) {
	var content claudeMessageContent
	if err := json.Unmarshal(msg.Message, &content); err != nil {
		return "restricted Claude emitted an unreadable tool result event", nil, true
	}
	var receipts []json.RawMessage
	unverifiable := false
	for _, block := range content.Content {
		if block.Type != "tool_result" {
			continue
		}
		call, ok := calls[block.ToolUseID]
		if !ok {
			return "restricted Claude emitted a tool result without an authorized call", receipts, true
		}
		delete(calls, block.ToolUseID)
		if call.tool == "EndConversation" {
			continue
		}
		duration := time.Since(call.startedAt)
		output, verified := restrictedClaudeReceiptWithInput(call.tool, block.ToolUseID, block.Content, block.IsError, duration, call.input)
		receipts = append(receipts, json.RawMessage(output))
		if !verified {
			unverifiable = true
		}
		toolID := "mcp__" + policy.MCPServer + "__" + call.tool
		trySend(ch, Message{
			Type:             MessageToolResult,
			Tool:             toolID,
			CallID:           block.ToolUseID,
			Output:           output,
			DurationMS:       duration.Milliseconds(),
			DurationMeasured: true,
		})
	}
	return "", receipts, unverifiable
}

func hasUnfinishedRestrictedMCPCall(calls map[string]restrictedClaudeCall) bool {
	for _, call := range calls {
		if call.tool != "EndConversation" {
			return true
		}
	}
	return false
}

func (b *claudeBackend) handleRestrictedControlRequest(msg claudeSDKMessage, stdin io.Writer, policy *taskpolicy.ExecutionPolicy) string {
	var req claudeControlRequestPayload
	if err := json.Unmarshal(msg.Request, &req); err != nil {
		return "restricted Claude emitted an unreadable control request"
	}
	_, allowed := restrictedClaudeToolName(policy, req.ToolName)
	behavior := map[string]any{"behavior": "deny", "message": "Task execution policy does not authorize this tool."}
	failure := "restricted Claude rejected an unauthorized control request"
	if allowed {
		if len(req.Input) == 0 || string(req.Input) == "null" {
			return "restricted Claude emitted an authorized tool request without input"
		}
		input := make(map[string]any)
		if err := json.Unmarshal(req.Input, &input); err != nil || !restrictedClaudeToolInputAllowed(
			strings.TrimPrefix(req.ToolName, "mcp__"+policy.MCPServer+"__"), input) {
			return "restricted Claude emitted malformed authorized tool input"
		}
		behavior = map[string]any{
			"behavior":     "allow",
			"updatedInput": input,
		}
		failure = ""
	}
	if msg.RequestID == "" {
		return "restricted Claude emitted a control request without an identifier"
	}
	response := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": msg.RequestID,
			"response":   behavior,
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		return "restricted Claude control response could not be encoded"
	}
	data = append(data, '\n')
	if _, err := stdin.Write(data); err != nil {
		return "restricted Claude control response could not be sent"
	}
	return failure
}

func restrictedClaudeToolInputAllowed(tool string, input map[string]any) bool {
	if input == nil {
		return false
	}
	if tool != "company_events" {
		return true
	}
	if len(input) != 3 {
		return false
	}
	afterID, afterOK := input["afterId"].(float64)
	throughID, throughOK := input["throughId"].(float64)
	limit, limitOK := input["limit"].(float64)
	return afterOK && afterID == 199900 &&
		throughOK && throughID == 199901 &&
		limitOK && limit == 1
}

func restrictedClaudeReceipt(tool, callID string, content json.RawMessage, isError bool, duration time.Duration) (string, bool) {
	return restrictedClaudeReceiptWithInput(tool, callID, content, isError, duration, nil)
}

func restrictedClaudeReceiptWithInput(tool, callID string, content json.RawMessage, isError bool, duration time.Duration, input map[string]any) (string, bool) {
	if tool == "company_events" && !restrictedClaudeToolInputAllowed(tool, input) {
		return restrictedClaudeReceiptUnavailable(tool, callID, isError, duration), false
	}
	blocks, ok := restrictedClaudeContentBlocks(content)
	if !ok {
		return restrictedClaudeReceiptUnavailable(tool, callID, isError, duration), false
	}
	codexItem := map[string]any{
		"result": map[string]any{
			"isError": isError,
			"content": blocks,
		},
	}
	receipt := codexMCPBrainSafeReceipt(tool, codexItem)
	if receipt == "" {
		return restrictedClaudeReceiptUnavailable(tool, callID, isError, duration), false
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(receipt), &values); err != nil {
		return restrictedClaudeReceiptUnavailable(tool, callID, isError, duration), false
	}
	if tool == "company_events" {
		values["eventSelector"] = map[string]any{
			"afterId": input["afterId"], "throughId": input["throughId"], "limit": input["limit"],
		}
	}
	if !restrictedBrainReceiptIsVerifiable(tool, values) {
		if tool != "company_events" {
			return restrictedClaudeReceiptUnavailable(tool, callID, isError, duration), false
		}
		values["callId"] = callID
		values["isError"] = isError
		values["receiptVerified"] = false
		values["durationMs"] = duration.Milliseconds()
		values["durationSource"] = "caller_monotonic"
		encoded, err := json.Marshal(values)
		if err != nil {
			return restrictedClaudeReceiptUnavailable(tool, callID, isError, duration), false
		}
		return string(encoded), false
	}
	values["callId"] = callID
	values["isError"] = isError
	values["receiptVerified"] = true
	values["durationMs"] = duration.Milliseconds()
	values["durationSource"] = "caller_monotonic"
	encoded, err := json.Marshal(values)
	if err != nil {
		return restrictedClaudeReceiptUnavailable(tool, callID, isError, duration), false
	}
	return string(encoded), true
}

func restrictedBrainReceiptIsVerifiable(tool string, receipt map[string]any) bool {
	hasNonEmptyString := func(key string) bool {
		value, ok := receipt[key].(string)
		return ok && value != ""
	}
	switch tool {
	case "brain_resolve":
		return hasNonEmptyString("projectId")
	case "brain_context":
		documentsTotal, totalOK := safeBrainReceiptInt(receipt["documentsTotal"])
		documents, documentsOK := receipt["documents"].([]any)
		incomplete, _ := receipt["documentReceiptsIncomplete"].(bool)
		if !hasNonEmptyString("projectId") || !totalOK || documentsTotal == 0 ||
			!documentsOK || len(documents) != int(documentsTotal) || incomplete {
			return false
		}
		for _, value := range documents {
			document, ok := value.(map[string]any)
			if !ok {
				return false
			}
			id, idOK := document["id"].(string)
			revision, revisionOK := safeBrainReceiptHash(document["revision"])
			if !idOK || id == "" || !revisionOK || revision == "" {
				return false
			}
		}
		return true
	case "brain_read":
		revision, revisionOK := safeBrainReceiptHash(receipt["revision"])
		currentRevision, currentOK := receipt["currentRevision"].(string)
		truncated, truncatedOK := receipt["truncated"].(bool)
		available, availableOK := receipt["available"].(bool)
		bodyVerified, bodyVerifiedOK := receipt["bodySha256Verified"].(bool)
		byteLengthVerified, byteLengthVerifiedOK := receipt["byteLengthVerified"].(bool)
		_, bytesOK := safeBrainReceiptInt(receipt["bytes"])
		return hasNonEmptyString("documentId") &&
			hasNonEmptyString("projectId") &&
			revisionOK && currentOK && currentRevision == revision &&
			truncatedOK && !truncated &&
			availableOK && available &&
			bodyVerifiedOK && bodyVerified &&
			byteLengthVerifiedOK && byteLengthVerified &&
			bytesOK
	case "brain_company_projects":
		projects, projectsOK := receipt["projects"].([]any)
		total, totalOK := safeBrainReceiptInt(receipt["projectsTotal"])
		incomplete, _ := receipt["projectReceiptsIncomplete"].(bool)
		if !projectsOK || !totalOK || total == 0 || incomplete || len(projects) != int(total) {
			return false
		}
		for _, value := range projects {
			project, ok := value.(map[string]any)
			if !ok || project["portfolioClass"] != "company" {
				return false
			}
			projectID, idOK := project["projectId"].(string)
			sourceHash, hashOK := safeBrainReceiptHash(project["classificationSourceSha256"])
			if !idOK || projectID == "" || !hashOK || sourceHash == "" {
				return false
			}
		}
		return true
	case "brain_projects":
		projects, projectsOK := receipt["projects"].([]any)
		total, totalOK := safeBrainReceiptInt(receipt["projectsTotal"])
		incomplete, _ := receipt["projectReceiptsIncomplete"].(bool)
		if !projectsOK || !totalOK || total != int64(len(projects)) || incomplete {
			return false
		}
		for _, value := range projects {
			project, ok := value.(map[string]any)
			projectID, idOK := project["projectId"].(string)
			if !ok || !idOK || projectID == "" {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func restrictedClaudeContentBlocks(raw json.RawMessage) ([]any, bool) {
	var blocks []any
	if json.Unmarshal(raw, &blocks) == nil {
		return blocks, true
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return []any{map[string]any{"type": "text", "text": text}}, true
	}
	return nil, false
}

func restrictedClaudeReceiptUnavailable(tool, callID string, isError bool, duration time.Duration) string {
	encoded, _ := json.Marshal(map[string]any{
		"tool":            tool,
		"callId":          callID,
		"isError":         isError,
		"receiptVerified": false,
		"receipt":         "unavailable",
		"durationMs":      duration.Milliseconds(),
		"durationSource":  "caller_monotonic",
	})
	return string(encoded)
}
