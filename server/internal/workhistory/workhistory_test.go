package workhistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanCodexKeepsOnlyVisibleMessagesAndRedacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout.jsonl")
	writeLines(t, path,
		map[string]any{"timestamp": "2026-10-15T13:00:00Z", "ordinal": 1, "type": "session_meta", "payload": map[string]any{"id": "codex-session-1", "base_instructions": "hidden"}},
		map[string]any{"timestamp": "2026-10-15T13:01:00Z", "ordinal": 2, "type": "response_item", "payload": map[string]any{"type": "message", "role": "developer", "content": []any{map[string]any{"type": "input_text", "text": "hidden instructions"}}}},
		map[string]any{"timestamp": "2026-10-15T13:02:00Z", "ordinal": 3, "type": "response_item", "payload": map[string]any{"type": "reasoning", "summary": "private reasoning"}},
		map[string]any{"timestamp": "2026-10-15T13:03:00Z", "ordinal": 4, "type": "response_item", "payload": map[string]any{"type": "function_call_output", "output": "Bearer tool-secret-123456789"}},
		map[string]any{"timestamp": "2026-10-15T13:04:00Z", "ordinal": 5, "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "LinkHub 403 for owner@example.com with Authorization: Bearer abcdefghijklmnop"}, map[string]any{"type": "image", "source": "private"}}}},
		map[string]any{"timestamp": "2026-10-15T13:05:00Z", "ordinal": 6, "type": "response_item", "payload": map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "I found the failed tracking-link request."}, map[string]any{"type": "tool_use", "input": "secret"}}}},
	)

	result, err := ScanFile(ScanOptions{Provider: ProviderCodex, Project: "linkhub", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fragments) != 2 {
		t.Fatalf("fragments = %d, want 2", len(result.Fragments))
	}
	joined := result.Fragments[0].Text + "\n" + result.Fragments[1].Text
	for _, forbidden := range []string{"hidden instructions", "private reasoning", "tool-secret", "owner@example.com", "abcdefghijklmnop"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("indexed text contains excluded value %q: %s", forbidden, joined)
		}
	}
	if !strings.Contains(joined, "LinkHub 403") || !strings.Contains(joined, "[REDACTED:EMAIL]") || !strings.Contains(joined, "[REDACTED:TOKEN]") {
		t.Fatalf("indexed text did not preserve useful text and redact secrets: %s", joined)
	}
	fragment := result.Fragments[0]
	if fragment.SessionID != "codex-session-1" || fragment.Project != "linkhub" || fragment.EvidenceClass != EvidenceClassConversationRecollection {
		t.Fatalf("unexpected provenance: %+v", fragment)
	}
	if !strings.HasPrefix(fragment.SourceReference, "work-history://codex/session/codex-session-1/") {
		t.Fatalf("source reference = %q", fragment.SourceReference)
	}
	if result.Redactions != 2 {
		t.Fatalf("redactions = %d, want 2", result.Redactions)
	}
}

func TestScanCodexExcludesApplicationContextMasqueradingAsUserText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout.jsonl")
	writeLines(t, path,
		map[string]any{"timestamp": "2026-10-15T13:00:00Z", "type": "session_meta", "payload": map[string]any{"id": "codex-session-context"}},
		map[string]any{"timestamp": "2026-10-15T13:01:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": `<codex_internal_context source="goal">hidden goal</codex_internal_context>`}}}},
		map[string]any{"timestamp": "2026-10-15T13:02:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": `<external_codex_apps_open_page>hidden page</external_codex_apps_open_page>`}}}},
		map[string]any{"timestamp": "2026-10-15T13:03:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": `<environment_context><cwd>/Users/private/work</cwd></environment_context>`}}}},
		map[string]any{"timestamp": "2026-10-15T13:04:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": `<in-app-browser-context source="ambient-ui-state">hidden URL</in-app-browser-context>\n\n## My request:\nFind the LinkHub 403 conversation.`}}}},
		map[string]any{"timestamp": "2026-10-15T13:05:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "# Files mentioned by the user:\n\n## secret.png: /Users/private/secret.png\n\n## My request:\nExplain this failure simply."}}}},
		map[string]any{"timestamp": "2026-10-15T13:06:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "# AGENTS.md instructions for /Users/private/work\n<INSTRUCTIONS>hidden rules</INSTRUCTIONS><environment_context><cwd>/Users/private/work</cwd></environment_context>"}}}},
		map[string]any{"timestamp": "2026-10-15T13:07:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "Can context-mode improve our company history?"}}}},
		map[string]any{"timestamp": "2026-10-15T13:08:00Z", "type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": `<system-reminder id="private">hidden reminder</system-reminder>`}}}},
	)

	result, err := ScanFile(ScanOptions{Provider: ProviderCodex, Project: "multica", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fragments) != 3 {
		t.Fatalf("fragments = %d, want 3: %+v", len(result.Fragments), result.Fragments)
	}
	joined := result.Fragments[0].Text + "\n" + result.Fragments[1].Text + "\n" + result.Fragments[2].Text
	for _, wanted := range []string{"Find the LinkHub 403 conversation.", "Explain this failure simply.", "Can context-mode improve our company history?"} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("visible request %q was lost: %s", wanted, joined)
		}
	}
	for _, forbidden := range []string{"hidden goal", "hidden page", "hidden URL", "hidden rules", "secret.png", "/Users/private/"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("application context %q entered history: %s", forbidden, joined)
		}
	}
}

func TestVisibleUserTextFailsClosedForMalformedInternalWrapper(t *testing.T) {
	got := visibleUserText("Keep this request.\n<codex_internal_context source=\"goal\">hidden")
	if got != "Keep this request." {
		t.Fatalf("visibleUserText = %q", got)
	}
}

func TestVisibleUserTextDoesNotMistakeSimilarLiteralTagForInternalContext(t *testing.T) {
	got := visibleUserText("Discuss <app-contextual> as ordinary text.")
	if got != "Discuss <app-contextual> as ordinary text." {
		t.Fatalf("visibleUserText = %q", got)
	}
}

func TestVisibleUserTextDropsMulticaGeneratedClaudeAssignment(t *testing.T) {
	generated := `You are running as a local coding agent for a Multica workspace.

Your assigned issue ID is: 01a0b33e-0a94-722e-83a1-9b768f2a38d8

You were handed this issue with a handoff note. Treat it as the assigner's scoping instruction.

## Session Continuity Notice

This run was meant to continue an earlier conversation.`
	if got := visibleUserText(generated); got != "" {
		t.Fatalf("generated Multica assignment was indexed: %q", got)
	}
	ownerText := "Explain how a local coding agent should work in Multica."
	if got := visibleUserText(ownerText); got != ownerText {
		t.Fatalf("ordinary owner text changed: %q", got)
	}
}

func TestScanClaudeKeepsTextButNotThinkingToolsOrImages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	writeLines(t, path,
		map[string]any{"type": "queue-operation", "timestamp": "2026-10-15T14:00:00Z", "sessionId": "claude-session-1"},
		map[string]any{"type": "user", "timestamp": "2026-10-15T14:01:00Z", "sessionId": "claude-session-1", "message": map[string]any{"content": "Why did the tracking link return forbidden? password=hunter2"}},
		map[string]any{"type": "assistant", "timestamp": "2026-10-15T14:02:00Z", "sessionId": "claude-session-1", "message": map[string]any{"content": []any{
			map[string]any{"type": "thinking", "thinking": "private chain"},
			map[string]any{"type": "text", "text": "The request was rejected before creation."},
			map[string]any{"type": "tool_use", "name": "shell", "input": map[string]any{"token": "raw-tool-token"}},
		}}},
		map[string]any{"type": "user", "timestamp": "2026-10-15T14:03:00Z", "sessionId": "claude-session-1", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_result", "content": "private tool output"},
			map[string]any{"type": "image", "source": map[string]any{"data": "private image"}},
			map[string]any{"type": "text", "text": "Try the LinkHub status endpoint next."},
		}}},
	)

	result, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "linkhub", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fragments) != 3 {
		t.Fatalf("fragments = %d, want 3", len(result.Fragments))
	}
	joined := result.Fragments[0].Text + "\n" + result.Fragments[1].Text + "\n" + result.Fragments[2].Text
	for _, forbidden := range []string{"hunter2", "private chain", "raw-tool-token", "private tool output", "private image"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("indexed text contains excluded value %q: %s", forbidden, joined)
		}
	}
	if !strings.Contains(joined, "[REDACTED CREDENTIAL]") {
		t.Fatalf("credential was not redacted: %s", joined)
	}
}

func TestScanIsIncrementalAndNoOpWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	writeLines(t, path,
		map[string]any{"type": "user", "timestamp": "2026-10-15T15:00:00Z", "sessionId": "claude-session-2", "message": map[string]any{"content": "first"}},
	)

	first, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Fragments) != 1 {
		t.Fatalf("first fragments = %d", len(first.Fragments))
	}
	appendLines(t, path,
		map[string]any{"type": "assistant", "timestamp": "2026-10-15T15:01:00Z", "sessionId": "claude-session-2", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "second"}}}},
	)
	second, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path, Checkpoint: &first.Checkpoint})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Fragments) != 1 || second.Fragments[0].Text != "second" {
		t.Fatalf("second scan = %+v", second.Fragments)
	}
	third, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path, Checkpoint: &second.Checkpoint})
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Fragments) != 0 || third.BytesRead != 0 || !third.Complete {
		t.Fatalf("unchanged scan should be a complete no-op: %+v", third)
	}
}

func TestScanRejectsChangedCheckpointBoundary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	writeLines(t, path,
		map[string]any{"type": "user", "timestamp": "2026-10-15T15:00:00Z", "sessionId": "claude-session-3", "message": map[string]any{"content": "first"}},
		map[string]any{"type": "assistant", "timestamp": "2026-10-15T15:01:00Z", "sessionId": "claude-session-3", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "second"}}}},
	)
	first, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-3] = 'X'
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path, Checkpoint: &first.Checkpoint})
	if !errorsIs(err, errSourceChanged) {
		t.Fatalf("error = %v, want source changed", err)
	}
}

func TestScanLeavesPartialLineForNextPass(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"user","timestamp":"2026-10-15T15:00:00Z","sessionId":"partial","message":{"content":"unfinished"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fragments) != 0 || result.Checkpoint.Offset != 0 || result.Complete {
		t.Fatalf("partial line was consumed: %+v", result)
	}
}

func TestScanRejectsSourceOutsideAllowedRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	path := filepath.Join(other, "session.jsonl")
	writeLines(t, path, map[string]any{"type": "user", "timestamp": "2026-10-15T15:00:00Z", "sessionId": "outside", "message": map[string]any{"content": "no"}})
	_, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: root, Path: path})
	if err == nil || !strings.Contains(err.Error(), "outside the allowed root") {
		t.Fatalf("error = %v", err)
	}
}

func TestScanChunksLongVisibleMessageForSemanticRetrieval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	text := strings.Repeat("ordinary background words ", 90) +
		"the remote campaign service refused creation because the account permission was missing " +
		strings.Repeat("more ordinary background words ", 90)
	writeLines(t, path,
		map[string]any{"type": "user", "timestamp": "2026-10-15T15:00:00Z", "sessionId": "long-session", "message": map[string]any{"content": text}},
	)
	result, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "linkhub", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fragments) < 3 {
		t.Fatalf("long message produced %d fragments, want at least 3", len(result.Fragments))
	}
	foundMeaningfulChunk := false
	references := map[string]bool{}
	for _, fragment := range result.Fragments {
		if len(fragment.Text) > semanticChunkBytes {
			t.Fatalf("chunk is %d bytes, want at most %d", len(fragment.Text), semanticChunkBytes)
		}
		if strings.Contains(fragment.Text, "campaign service refused creation") {
			foundMeaningfulChunk = true
		}
		if references[fragment.SourceReference] {
			t.Fatalf("duplicate source reference %q", fragment.SourceReference)
		}
		references[fragment.SourceReference] = true
	}
	if !foundMeaningfulChunk {
		t.Fatal("meaningful passage was lost while chunking")
	}
}

func TestParseSourceReferenceRejectsMalformedOrWidenedReferences(t *testing.T) {
	valid, err := ParseSourceReference("work-history://codex/session/session-1/record/42?offset=100&part=2")
	if err != nil {
		t.Fatal(err)
	}
	if valid.Provider != ProviderCodex || valid.SessionID != "session-1" || valid.Ordinal != 42 || valid.Offset != 100 || valid.Part != 2 {
		t.Fatalf("unexpected parsed location: %+v", valid)
	}
	for _, invalid := range []string{
		"https://codex/session/session-1/record/42?offset=100&part=2",
		"work-history://unknown/session/session-1/record/42?offset=100&part=2",
		"work-history://codex/session/session-1/record/0?offset=100&part=2",
		"work-history://codex/session/session-1/record/42?offset=-1&part=2",
		"work-history://codex/session/session-1/record/42?offset=100&part=2&path=/tmp/private",
	} {
		if _, err := ParseSourceReference(invalid); err == nil {
			t.Fatalf("accepted invalid reference %q", invalid)
		}
	}
}

func TestScanUsesSharedSecretRedactionAndEnforcesByteBound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	secret := "xox" + "b-123456789012-1234567890123-AbCdEfGhIjKl"
	writeLines(t, path,
		map[string]any{"type": "user", "timestamp": "2026-10-15T15:00:00Z", "sessionId": "bounded", "message": map[string]any{"content": "token " + secret}},
	)
	result, err := ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fragments) != 1 || strings.Contains(result.Fragments[0].Text, secret) || !strings.Contains(result.Fragments[0].Text, "[REDACTED SLACK TOKEN]") {
		t.Fatalf("shared secret redaction was not applied: %+v", result.Fragments)
	}
	_, err = ScanFile(ScanOptions{Provider: ProviderClaude, Project: "multica", AllowedRoot: dir, Path: path, MaxBytes: 32})
	if err == nil || !strings.Contains(err.Error(), "scan limit") {
		t.Fatalf("oversized first record error = %v", err)
	}
}

func writeLines(t *testing.T, path string, values ...map[string]any) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encodeLines(t, file, values...)
}

func appendLines(t *testing.T, path string, values ...map[string]any) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encodeLines(t, file, values...)
}

func encodeLines(t *testing.T, file *os.File, values ...map[string]any) {
	t.Helper()
	encoder := json.NewEncoder(file)
	for _, value := range values {
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
	}
}

func errorsIs(err, target error) bool {
	return err != nil && strings.Contains(err.Error(), target.Error())
}
