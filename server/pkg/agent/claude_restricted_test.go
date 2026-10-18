package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/taskpolicy"
)

func brainReadOnlyPolicy(t *testing.T) *taskpolicy.ExecutionPolicy {
	t.Helper()
	policy, err := taskpolicy.NewBrainMCPReadOnly(taskpolicy.Request{
		Mode:      taskpolicy.BrainMCPReadOnlyMode,
		MCPServer: "brain",
	})
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func TestClaudeVersionAtLeast(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"2.1.247", false},
		{"2.1.248", true},
		{"Claude Code 2.1.273", true},
		{"2.1.248-beta", false},
		{"2.1.273.4", false},
		{"unknown", false},
		{"", false},
	} {
		if got := claudeVersionAtLeast(tc.value, minRestrictedClaudeVersion); got != tc.want {
			t.Errorf("claudeVersionAtLeast(%q) = %t, want %t", tc.value, got, tc.want)
		}
	}
}

func TestBuildRestrictedClaudeArgsExposeOnlyExactMCPTools(t *testing.T) {
	policy := brainReadOnlyPolicy(t)
	args, err := buildRestrictedClaudeArgs(ExecOptions{
		ExecutionPolicy:    policy,
		Model:              "claude-sonnet",
		CustomArgs:         []string{"--tools", "Bash", "--agent", "other"},
		ExtraArgs:          []string{"--dangerous"},
		ClaudeSettingsPath: "/tmp/untrusted-settings.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\x00")
	for _, required := range []string{"--restricted", "--no-session-persistence", "--tools", "", "--agents", "--agent", restrictedClaudeAgentName, "--strict-mcp-config"} {
		if !strings.Contains(joined, required) {
			t.Errorf("restricted args omit %q: %#v", required, args)
		}
	}
	toolsIndex := slicesIndex(args, "--tools")
	if toolsIndex < 0 || toolsIndex+1 >= len(args) || args[toolsIndex+1] != "" {
		t.Fatalf("--tools must be followed by an empty argument to disable built-ins: %#v", args)
	}
	for _, forbidden := range []string{"Bash", "Read", "Write", "Edit", "Patch", "--dangerous", "/tmp/untrusted-settings.json", "bypassPermissions"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("restricted args contain forbidden value %q: %#v", forbidden, args)
		}
	}
	index := slicesIndex(args, "--agents")
	if index < 0 || index+1 >= len(args) {
		t.Fatalf("missing --agents JSON: %#v", args)
	}
	var agents map[string]restrictedClaudeAgent
	if err := json.Unmarshal([]byte(args[index+1]), &agents); err != nil {
		t.Fatalf("decode dynamic agent: %v", err)
	}
	agent, ok := agents[restrictedClaudeAgentName]
	if !ok {
		t.Fatalf("dynamic main agent missing: %#v", agents)
	}
	if !reflect.DeepEqual(agent.Tools, policy.Tools) {
		t.Fatalf("agent tools = %#v, want exact policy list %#v", agent.Tools, policy.Tools)
	}
}

func TestValidateRestrictedClaudeConfigFailsClosed(t *testing.T) {
	policy := brainReadOnlyPolicy(t)
	config := json.RawMessage(`{"mcpServers":{"brain":{"type":"http","url":"https://brain.invalid/mcp"},"other":{"type":"http","url":"https://other.invalid/mcp"}}}`)
	for _, tc := range []struct {
		name string
		cfg  Config
		opts ExecOptions
	}{
		{name: "unsupported version", cfg: Config{BuiltinRuntime: true, CLIVersion: "2.1.247"}, opts: ExecOptions{ExecutionPolicy: policy, McpConfig: config}},
		{name: "unknown version", cfg: Config{BuiltinRuntime: true}, opts: ExecOptions{ExecutionPolicy: policy, McpConfig: config}},
		{name: "custom runtime", cfg: Config{CLIVersion: "2.1.273"}, opts: ExecOptions{ExecutionPolicy: policy, McpConfig: config}},
		{name: "missing server", cfg: Config{BuiltinRuntime: true, CLIVersion: "2.1.273"}, opts: ExecOptions{ExecutionPolicy: policy, McpConfig: json.RawMessage(`{"mcpServers":{"other":{"type":"http","url":"https://other.invalid/mcp"}}}`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validateRestrictedClaudeConfig(tc.cfg, tc.opts); err == nil {
				t.Fatal("restricted config unexpectedly accepted")
			}
		})
	}
}

func TestRestrictedClaudeExecuteRejectsUnsupportedVersionBeforeStartingCLI(t *testing.T) {
	b := &claudeBackend{cfg: Config{
		ExecutablePath: "/nonexistent/claude",
		BuiltinRuntime: true,
		CLIVersion:     "2.1.247",
		Logger:         slog.Default(),
	}}
	_, err := b.Execute(t.Context(), "private task prompt", ExecOptions{
		ExecutionPolicy: brainReadOnlyPolicy(t),
		McpConfig:       json.RawMessage(`{"mcpServers":{"brain":{"type":"http","url":"https://brain.invalid/mcp"}}}`),
	})
	if err == nil || !strings.Contains(err.Error(), minRestrictedClaudeVersion) {
		t.Fatalf("unsupported CLI was not rejected before launch: %v", err)
	}
}

func TestValidateRestrictedClaudeConfigFiltersServersWithoutMutatingInput(t *testing.T) {
	policy := brainReadOnlyPolicy(t)
	config := json.RawMessage(`{"mcpServers":{"brain":{"type":"http","url":"https://brain.invalid/mcp"},"other":{"type":"http","url":"https://other.invalid/mcp"}}}`)
	before := append(json.RawMessage(nil), config...)
	filtered, err := validateRestrictedClaudeConfig(Config{BuiltinRuntime: true, CLIVersion: "2.1.273"}, ExecOptions{
		ExecutionPolicy: policy,
		McpConfig:       config,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(config, before) {
		t.Fatalf("shared MCP config changed: %s", config)
	}
	var got struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(filtered, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.MCPServers) != 1 || len(got.MCPServers["brain"]) == 0 {
		t.Fatalf("filtered config = %s", filtered)
	}
}

func TestRestrictedClaudeAssistantDeniesLocalAndUnknownTools(t *testing.T) {
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	policy := brainReadOnlyPolicy(t)
	for _, tool := range []string{"Bash", "Read", "Write", "Edit", "mcp__other__brain_read", "mcp__brain__unknown"} {
		ch := make(chan Message, 2)
		msg := claudeSDKMessage{Message: mustMarshal(t, claudeMessageContent{
			Content: []claudeContentBlock{{Type: "tool_use", ID: "call-denied", Name: tool}},
		})}
		_, failure := b.handleRestrictedAssistant(msg, ch, map[string]TokenUsage{}, policy, map[string]restrictedClaudeCall{})
		if failure == "" {
			t.Errorf("tool %q was not rejected", tool)
		}
		if len(ch) != 0 {
			t.Errorf("denied tool %q emitted a persisted message", tool)
		}
	}
}

func TestRestrictedClaudeEndConversationIsHarmlessAndNeverAReceipt(t *testing.T) {
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	policy := brainReadOnlyPolicy(t)
	calls := map[string]restrictedClaudeCall{}
	assistant := claudeSDKMessage{Message: mustMarshal(t, claudeMessageContent{
		Content: []claudeContentBlock{{Type: "tool_use", ID: "end-1", Name: "EndConversation"}},
	})}
	turn, failure := b.handleRestrictedAssistant(assistant, make(chan Message, 1), map[string]TokenUsage{}, policy, calls)
	if failure != "" || turn.toolUses != 1 || hasUnfinishedRestrictedMCPCall(calls) {
		t.Fatalf("EndConversation was not treated as harmless: turn=%+v failure=%q calls=%+v", turn, failure, calls)
	}

	user := claudeSDKMessage{Message: mustMarshal(t, claudeMessageContent{
		Content: []claudeContentBlock{{Type: "tool_result", ToolUseID: "end-1", Content: json.RawMessage(`"private control result"`)}},
	})}
	output := make(chan Message, 1)
	failure, receipts, unverifiable := b.handleRestrictedUser(user, output, policy, calls)
	if failure != "" || unverifiable || len(receipts) != 0 || len(output) != 0 || len(calls) != 0 {
		t.Fatalf("EndConversation result affected task evidence: failure=%q unverifiable=%t receipts=%s messages=%d calls=%+v", failure, unverifiable, receipts, len(output), calls)
	}
}

func TestRestrictedClaudeAssistantDoesNotExposeToolInput(t *testing.T) {
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	policy := brainReadOnlyPolicy(t)
	ch := make(chan Message, 1)
	calls := map[string]restrictedClaudeCall{}
	msg := claudeSDKMessage{Message: mustMarshal(t, claudeMessageContent{
		Content: []claudeContentBlock{{
			Type: "tool_use", ID: "call-1", Name: "mcp__brain__brain_read",
			Input: mustMarshal(t, map[string]any{"documentId": "private-doc-id", "query": "private query"}),
		}},
	})}
	turn, failure := b.handleRestrictedAssistant(msg, ch, map[string]TokenUsage{}, policy, calls)
	if failure != "" || turn.toolUses != 1 {
		t.Fatalf("restricted assistant result: turn=%+v failure=%q", turn, failure)
	}
	got := <-ch
	if got.Tool != "mcp__brain__brain_read" || got.Input != nil {
		t.Fatalf("unsafe tool-use message: %+v", got)
	}
	if strings.Contains(fmt.Sprint(got), "private") {
		t.Fatalf("private tool input escaped: %+v", got)
	}
}

func TestRestrictedClaudeEventToolUsesExactFrozenRange(t *testing.T) {
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	policy := brainReadOnlyPolicy(t)
	for _, tc := range []struct {
		name  string
		input map[string]any
		want  bool
	}{
		{name: "exact selector", input: map[string]any{"afterId": 199900, "throughId": 199901, "limit": 1}, want: true},
		{name: "wider range", input: map[string]any{"afterId": 199000, "throughId": 199901, "limit": 1}},
		{name: "larger limit", input: map[string]any{"afterId": 199900, "throughId": 199901, "limit": 10}},
		{name: "extra query", input: map[string]any{"afterId": 199900, "throughId": 199901, "limit": 1, "query": "private"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan Message, 1)
			calls := map[string]restrictedClaudeCall{}
			msg := claudeSDKMessage{Message: mustMarshal(t, claudeMessageContent{
				Content: []claudeContentBlock{{
					Type: "tool_use", ID: "event-call", Name: "mcp__brain__company_events",
					Input: mustMarshal(t, tc.input),
				}},
			})}
			_, failure := b.handleRestrictedAssistant(msg, ch, map[string]TokenUsage{}, policy, calls)
			if (failure == "") != tc.want {
				t.Fatalf("failure=%q, want allowed=%t", failure, tc.want)
			}
			if got := len(ch); (got == 1) != tc.want {
				t.Fatalf("emitted %d tool-use events, want allowed=%t", got, tc.want)
			}
		})
	}
}

func TestRestrictedClaudeReceiptOmitsDocumentBodyAndMeasuresDuration(t *testing.T) {
	body := "private document body"
	revision := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	payload := fmt.Sprintf(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"2026-04-01T12:00:00Z","dataPolicy":"authorized","document":{"id":"doc-1","project":"project-1","revision":"%s","body":%q,"bytes":%d,"truncated":false,"currentRevision":"%s","available":true}}`, revision, body, len(body), revision)
	content := json.RawMessage(fmt.Sprintf(`[{"type":"text","text":%q}]`, payload))
	got, verified := restrictedClaudeReceipt("brain_read", "call-1", content, false, 1500*time.Millisecond)
	if !verified {
		t.Fatalf("Brain read receipt not verified: %s", got)
	}
	for _, want := range []string{`"documentId":"doc-1"`, `"projectId":"project-1"`, `"bodySha256Verified":true`, `"byteLengthVerified":true`, `"durationMs":1500`, `"durationSource":"caller_monotonic"`} {
		if !strings.Contains(got, want) {
			t.Errorf("receipt omits %s: %s", want, got)
		}
	}
	if strings.Contains(got, body) || strings.Contains(got, "bodyBytes") {
		t.Fatalf("receipt retained source body: %s", got)
	}
}

func TestRestrictedClaudeReceiptFailsClosedForUnknownCompanyEventSchema(t *testing.T) {
	content := json.RawMessage(`[{"type":"text","text":"{\"event_id\":\"199901\",\"citation\":\"citation\"}"}]`)
	got, verified := restrictedClaudeReceipt("company_events", "call-1", content, false, time.Second)
	if verified || strings.Contains(got, "199901") || strings.Contains(got, "citation") {
		t.Fatalf("unknown company event response was treated as verified: verified=%t receipt=%s", verified, got)
	}
}

func TestRestrictedClaudeCompanyEventsReceiptKeepsKnownEnvelopeUnverified(t *testing.T) {
	content := json.RawMessage(`[{"type":"text","text":"{\"schemaVersion\":\"silvermediums.brain.v2\",\"servedAt\":\"2026-10-18T01:00:00Z\",\"dataPolicy\":\"authorized\",\"items\":[{}],\"next_after_id\":199901,\"snapshot_through_id\":199901}"}]`)
	input := map[string]any{"afterId": float64(199900), "throughId": float64(199901), "limit": float64(1)}
	got, verified := restrictedClaudeReceiptWithInput("company_events", "call-1", content, false, 1500*time.Millisecond, input)
	for _, want := range []string{`"eventSelector":{"afterId":199900,"limit":1,"throughId":199901}`, `"eventItemsTotal":1`, `"nextAfterId":199901`, `"snapshotThroughId":199901`, `"eventRowReceipt":"unavailable"`, `"receiptVerified":false`, `"durationMs":1500`, `"durationSource":"caller_monotonic"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("unverified event receipt omitted %s: %s", want, got)
		}
	}
	if verified || strings.Contains(got, `"citation"`) || strings.Contains(got, `"eventId"`) {
		t.Fatalf("unknown event row contract was treated as verified: verified=%t receipt=%s", verified, got)
	}
}

func TestRestrictedClaudeCompanyEventsResultIsPersistedAsUnverifiedSafeSummary(t *testing.T) {
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	policy := brainReadOnlyPolicy(t)
	input := map[string]any{"afterId": float64(199900), "throughId": float64(199901), "limit": float64(1)}
	calls := map[string]restrictedClaudeCall{}
	assistant := claudeSDKMessage{Message: mustMarshal(t, claudeMessageContent{
		Content: []claudeContentBlock{{
			Type: "tool_use", ID: "event-call", Name: "mcp__brain__company_events",
			Input: mustMarshal(t, input),
		}},
	})}
	if _, failure := b.handleRestrictedAssistant(assistant, make(chan Message, 1), map[string]TokenUsage{}, policy, calls); failure != "" {
		t.Fatalf("exact event selector was rejected: %s", failure)
	}
	payload := `{"schemaVersion":"silvermediums.brain.v2","servedAt":"2026-10-18T01:00:00Z","dataPolicy":"authorized","items":[{"id":"199901","citation":"SENSITIVE_CITATION","body":"SENSITIVE_EVENT_BODY"}],"next_after_id":199901,"snapshot_through_id":199901}`
	content := json.RawMessage(fmt.Sprintf(`[{"type":"text","text":%q}]`, payload))
	user := claudeSDKMessage{Message: mustMarshal(t, claudeMessageContent{
		Content: []claudeContentBlock{{Type: "tool_result", ToolUseID: "event-call", Content: content}},
	})}
	output := make(chan Message, 1)
	failure, receipts, unverifiable := b.handleRestrictedUser(user, output, policy, calls)
	if failure != "" || !unverifiable || len(receipts) != 1 {
		t.Fatalf("company event result state: failure=%q unverifiable=%t receipts=%s", failure, unverifiable, receipts)
	}
	for _, secret := range []string{"SENSITIVE_CITATION", "SENSITIVE_EVENT_BODY", `"eventId"`, `"citation"`} {
		if strings.Contains(string(receipts[0]), secret) {
			t.Fatalf("unverified event summary retained %q: %s", secret, receipts[0])
		}
	}
	for _, safeField := range []string{`"eventSelector"`, `"eventRowReceipt":"unavailable"`, `"receiptVerified":false`, `"durationSource":"caller_monotonic"`} {
		if !strings.Contains(string(receipts[0]), safeField) {
			t.Fatalf("unverified event summary omitted %s: %s", safeField, receipts[0])
		}
	}
	resultMessage := <-output
	if !resultMessage.DurationMeasured || strings.Contains(resultMessage.Output, "SENSITIVE") {
		t.Fatalf("event tool result was not safely summarized/timed: %+v", resultMessage)
	}
}

func TestRestrictedBrainReceiptRequiresCompleteEvidence(t *testing.T) {
	validRead := map[string]any{
		"documentId":         "doc-1",
		"projectId":          "project-1",
		"revision":           strings.Repeat("a", 64),
		"currentRevision":    strings.Repeat("a", 64),
		"bytes":              float64(10),
		"truncated":          false,
		"available":          true,
		"bodySha256Verified": true,
		"byteLengthVerified": true,
	}
	if !restrictedBrainReceiptIsVerifiable("brain_read", validRead) {
		t.Fatal("complete immutable read receipt was rejected")
	}
	for name, mutate := range map[string]func(map[string]any){
		"truncated":        func(receipt map[string]any) { receipt["truncated"] = true },
		"unavailable":      func(receipt map[string]any) { receipt["available"] = false },
		"body hash failed": func(receipt map[string]any) { receipt["bodySha256Verified"] = false },
		"body byte count failed": func(receipt map[string]any) {
			receipt["byteLengthVerified"] = false
		},
		"revision changed": func(receipt map[string]any) { receipt["currentRevision"] = strings.Repeat("b", 64) },
		"missing project":  func(receipt map[string]any) { receipt["projectId"] = "" },
	} {
		t.Run(name, func(t *testing.T) {
			receipt := make(map[string]any, len(validRead))
			for key, value := range validRead {
				receipt[key] = value
			}
			mutate(receipt)
			if restrictedBrainReceiptIsVerifiable("brain_read", receipt) {
				t.Fatal("incomplete or invalid evidence was accepted")
			}
		})
	}
}

func TestRestrictedBrainCompanySelectorRequiresClassifiedCompleteProjects(t *testing.T) {
	valid := map[string]any{
		"projects": []any{map[string]any{
			"projectId":                  "multica",
			"portfolioClass":             "company",
			"classificationSourceSha256": strings.Repeat("a", 64),
		}},
		"projectsTotal":             float64(1),
		"projectReceiptsIncomplete": false,
	}
	if !restrictedBrainReceiptIsVerifiable("brain_company_projects", valid) {
		t.Fatal("complete company selector receipt was rejected")
	}
	for name, mutate := range map[string]func(map[string]any){
		"wrong portfolio class": func(receipt map[string]any) {
			receipt["projects"].([]any)[0].(map[string]any)["portfolioClass"] = "personal"
		},
		"missing source pin": func(receipt map[string]any) {
			delete(receipt["projects"].([]any)[0].(map[string]any), "classificationSourceSha256")
		},
		"incomplete list": func(receipt map[string]any) {
			receipt["projectReceiptsIncomplete"] = true
		},
		"count mismatch": func(receipt map[string]any) {
			receipt["projectsTotal"] = float64(2)
		},
	} {
		t.Run(name, func(t *testing.T) {
			receipt := map[string]any{
				"projects": []any{map[string]any{
					"projectId":                  "multica",
					"portfolioClass":             "company",
					"classificationSourceSha256": strings.Repeat("a", 64),
				}},
				"projectsTotal":             float64(1),
				"projectReceiptsIncomplete": false,
			}
			mutate(receipt)
			if restrictedBrainReceiptIsVerifiable("brain_company_projects", receipt) {
				t.Fatal("incomplete or non-company selector evidence was accepted")
			}
		})
	}
}

func TestRestrictedClaudeControlRequestDeniesUnauthorizedTools(t *testing.T) {
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	policy := brainReadOnlyPolicy(t)
	var output bytes.Buffer
	msg := claudeSDKMessage{
		RequestID: "req-1",
		Request: mustMarshal(t, claudeControlRequestPayload{
			Subtype: "tool_use", ToolName: "Bash",
			Input: mustMarshal(t, map[string]any{"command": "cat /etc/passwd"}),
		}),
	}
	if failure := b.handleRestrictedControlRequest(msg, &output, policy); failure == "" {
		t.Fatal("unauthorized control request was accepted")
	}
	if strings.Contains(output.String(), "cat /etc/passwd") {
		t.Fatalf("control response exposed tool input: %s", output.String())
	}
	var response struct {
		Response struct {
			Response struct {
				Behavior string `json:"behavior"`
			} `json:"response"`
		} `json:"response"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatal(err)
	}
	if response.Response.Response.Behavior != "deny" {
		t.Fatalf("behavior = %q, want deny", response.Response.Response.Behavior)
	}
}

func TestRestrictedClaudeControlRequestUsesExactEventSelector(t *testing.T) {
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	policy := brainReadOnlyPolicy(t)
	for _, tc := range []struct {
		name  string
		input map[string]any
		want  bool
	}{
		{name: "exact selector", input: map[string]any{"afterId": 199900, "throughId": 199901, "limit": 1}, want: true},
		{name: "wider selector", input: map[string]any{"afterId": 1, "throughId": 199901, "limit": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			msg := claudeSDKMessage{
				RequestID: "req-event",
				Request: mustMarshal(t, claudeControlRequestPayload{
					Subtype: "tool_use", ToolName: "mcp__brain__company_events",
					Input: mustMarshal(t, tc.input),
				}),
			}
			failure := b.handleRestrictedControlRequest(msg, &output, policy)
			if (failure == "") != tc.want {
				t.Fatalf("failure=%q, want allowed=%t", failure, tc.want)
			}
			if tc.want {
				if !strings.Contains(output.String(), `"behavior":"allow"`) {
					t.Fatalf("control response = %s, want allow", output.String())
				}
			} else if strings.Contains(output.String(), `"behavior":"allow"`) {
				t.Fatalf("out-of-scope event selector was allowed: %s", output.String())
			}
		})
	}
}

func TestClaudeRestrictedExecutePersistsOnlySafeReceipt(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	body := "SENSITIVE_DOCUMENT_BODY"
	revision := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	payload, err := json.Marshal(map[string]any{
		"schemaVersion": "silvermediums.brain.v2",
		"servedAt":      "2026-04-01T12:00:00Z",
		"dataPolicy":    "authorized",
		"document": map[string]any{
			"id": "doc-1", "project": "project-1", "revision": revision,
			"body": body, "bytes": len(body), "truncated": false,
			"currentRevision": revision, "available": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assistant, err := json.Marshal(map[string]any{
		"role": "assistant",
		"content": []any{
			map[string]any{"type": "text", "text": "SENSITIVE_ASSISTANT_TEXT"},
			map[string]any{"type": "thinking", "thinking": "SENSITIVE_THINKING"},
			map[string]any{"type": "tool_use", "id": "call-1", "name": "mcp__brain__brain_read", "input": map[string]any{"documentId": "SENSITIVE_INPUT"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	user, err := json.Marshal(map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "call-1", "content": []any{map[string]any{"type": "text", "text": string(payload)}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	events := []any{
		map[string]any{"type": "system", "subtype": "init", "session_id": "SENSITIVE_SESSION"},
		map[string]any{"type": "assistant", "message": json.RawMessage(assistant)},
		map[string]any{"type": "user", "message": json.RawMessage(user)},
		map[string]any{"type": "result", "subtype": "success", "is_error": false, "session_id": "SENSITIVE_SESSION", "result": "SENSITIVE_FINAL_OUTPUT"},
	}
	var fixture strings.Builder
	for _, event := range events {
		line, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		fixture.Write(line)
		fixture.WriteByte('\n')
	}

	fakePath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nIFS= read -r _\nprintf '%s\\n' 'SENSITIVE_STDERR' >&2\ncat <<'MULTICA_EVENTS'\n" + fixture.String() + "MULTICA_EVENTS\n"
	writeTestExecutable(t, fakePath, []byte(script))
	var logs bytes.Buffer
	backend, err := New("claude", Config{
		ExecutablePath: fakePath,
		CLIVersion:     "2.1.273",
		BuiltinRuntime: true,
		Env:            map[string]string{"IS_SANDBOX": "1"},
		Logger:         slog.New(slog.NewTextHandler(&logs, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(t.Context(), "SENSITIVE_PROMPT", ExecOptions{
		ExecutionPolicy: brainReadOnlyPolicy(t),
		McpConfig:       json.RawMessage(`{"mcpServers":{"brain":{"type":"http","url":"https://brain.invalid/mcp"},"other":{"type":"http","url":"https://other.invalid/mcp"}}}`),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var messages []Message
	for message := range session.Messages {
		messages = append(messages, message)
	}
	result := <-session.Result
	if result.Status != "completed" {
		t.Fatalf("restricted task status=%q error=%q", result.Status, result.Error)
	}
	if result.SessionID != "" {
		t.Fatalf("restricted task retained session id %q", result.SessionID)
	}
	for _, secret := range []string{"SENSITIVE_DOCUMENT_BODY", "SENSITIVE_ASSISTANT_TEXT", "SENSITIVE_THINKING", "SENSITIVE_INPUT", "SENSITIVE_FINAL_OUTPUT", "SENSITIVE_SESSION", "SENSITIVE_STDERR", "SENSITIVE_PROMPT"} {
		if strings.Contains(result.Output, secret) || strings.Contains(result.Error, secret) || strings.Contains(logs.String(), secret) {
			t.Fatalf("restricted task persisted %s", secret)
		}
	}
	if !strings.Contains(result.Output, `"bodySha256Verified":true`) || !strings.Contains(result.Output, `"durationSource":"caller_monotonic"`) {
		t.Fatalf("safe result omitted verification or timing receipt: %s", result.Output)
	}
	for _, message := range messages {
		if message.SessionID != "" {
			t.Fatalf("restricted message retained session id: %+v", message)
		}
		switch message.Type {
		case MessageText, MessageThinking, MessageLog:
			t.Fatalf("restricted task exposed private provider content: %+v", message)
		case MessageToolUse:
			if message.Input != nil || message.Tool != "mcp__brain__brain_read" {
				t.Fatalf("unsafe restricted tool-use event: %+v", message)
			}
		case MessageToolResult:
			if !message.DurationMeasured || message.DurationMS < 0 {
				t.Fatalf("restricted tool result omitted caller timing: %+v", message)
			}
		}
	}
}

func slicesIndex(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}
