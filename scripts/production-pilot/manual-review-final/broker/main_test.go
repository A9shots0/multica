package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExactTransportWithFakeChild(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "fake-provider.exe")
	marker := filepath.Join(t.TempDir(), "launches.txt")
	source := `C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\server\cmd\item14-integration-pilot-broker\fake-provider\main.go`
	cmd := exec.Command("go", "build", "-ldflags", "-X=main.launchMarker="+filepath.ToSlash(marker), "-o", fake, source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %s %v", output, err)
	}
	t.Setenv("MULTICA_TOKEN", "fixture-must-not-leak")
	t.Setenv("ANTHROPIC_API_KEY", "fixture-must-not-leak")
	var output bytes.Buffer
	r, err := runProvider(context.Background(), fake, "task_local_fixture", "", "Return exactly ITEM14_EXECUTION_APPROVED.", &output)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil || strings.Count(string(raw), "ITEM14_EXECUTION_APPROVED") != 1 {
		t.Fatal("fake launch count was not exactly one")
	}
	if r.ResultText != "ITEM14_EXECUTION_APPROVED" || len(r.Models) != 2 {
		t.Fatal("primary/result/composite evidence did not bind")
	}
}

func TestProviderCannotBroadenOutputContract(t *testing.T) {
	cases := []string{
		`{"summary":"This is a valid-length summary","handoff_markdown":"short"}`,
		`{"summary":"This is a valid-length summary","verdict":"pass","findings":["A material error"]}`,
		`{"summary":"This is a valid-length summary","verdict":"fail","findings":[]}`,
		`{"summary":"This is a valid-length summary","verdict":"pass","command":"rm"}`,
		`{"summary":"Send mention://agent/someone a request","verdict":"pass"}`,
		`{"summary":"This is a valid-length summary","verdict":"pass"}{}`,
	}
	for _, raw := range cases {
		if _, err := parseResult(raw, "review"); err == nil {
			t.Fatalf("accepted unsafe result %s", raw)
		}
	}
	if _, err := parseResult(`{"summary":"The captured facts and handoff agree.","verdict":"pass","findings":[]}`, "review"); err != nil {
		t.Fatal(err)
	}
}
func TestRoleAndIssueDestinationsAreFixed(t *testing.T) {
	for _, pair := range [][2]string{{plannerName, "implementation"}, {writerName, "leader"}, {reviewerName, "correction"}, {"Mika", "leader"}} {
		if _, err := roleSlot(pair[0], pair[1]); err == nil {
			t.Fatal("role/task expansion accepted")
		}
	}
	for _, raw := range []string{"https://127.0.0.1:8080", "http://localhost:8080", "http://127.0.0.1:18083", "http://example.com:8080", "http://127.0.0.1:8080/api"} {
		t.Setenv("MULTICA_SERVER_URL", raw)
		if _, err := requireLoopbackServer(); err == nil {
			t.Fatal("wrong server accepted")
		}
	}
	t.Setenv("MULTICA_ISSUE_ID", "other")
	if _, err := brokerIssueID(); err == nil {
		t.Fatal("wrong issue accepted")
	}
}
func TestProviderSlotCannotReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slot.json")
	if err := writeNew(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := writeNew(path, []byte("second")); err == nil {
		t.Fatal("consumed slot overwritten")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "first" {
		t.Fatal("original receipt lost")
	}
}
func TestModelAndToolFramesFailClosed(t *testing.T) {
	for _, model := range []string{"claude-sonnet-4-5", "wrong-model"} {
		if _, _, err := inspectProviderEvent(map[string]any{"type": "system", "subtype": "init", "model": model}, map[string]struct{}{}); err == nil {
			t.Fatal("wrong model accepted")
		}
	}
	event := map[string]any{"type": "assistant", "message": map[string]any{"model": exactClaudeModel, "content": []any{map[string]any{"type": "tool_use", "name": "Bash"}}}}
	if _, _, err := inspectProviderEvent(event, map[string]struct{}{}); err == nil {
		t.Fatal("tool use accepted")
	}
	if _, err := parseResult("# CRM engineering handoff\n"+strings.Repeat("evidence ", 250), "implementation"); err != nil {
		t.Fatal(err)
	}
}
func TestPhoneFarmAndRemoteFilesystemPathsDenied(t *testing.T) {
	for _, path := range []string{`C:\Dev\SMC`, `C:\Dev\SMC\nested`, `\\host\share`, `relative`} {
		if err := permittedWorktreePath(path); err == nil {
			t.Fatal("unauthorized path accepted")
		}
	}
	if err := permittedWorktreePath(`C:\Dev\CRM`); err != nil {
		t.Fatal(err)
	}
}

func TestWriterReceivesPersistedPlanAndHash(t *testing.T) {
	dir := t.TempDir()
	if _, err := preparePromptFromDirectory(writerName, "implementation", factsDocument{}, dir); err == nil {
		t.Fatal("writer ran without a planner handoff")
	}
	plan := []byte(`{"summary":"Plan the current evidence-based CRM handoff.","plan":["Verify snapshot scope","Explain cached divergence","Prioritize independent checks"]}`)
	if err := os.WriteFile(filepath.Join(dir, "PLAN.json"), plan, 0600); err != nil {
		t.Fatal(err)
	}
	prompt, err := preparePromptFromDirectory(writerName, "implementation", factsDocument{}, dir)
	if err != nil || !strings.Contains(prompt, string(plan)) || !strings.Contains(prompt, hashBytes(plan)) {
		t.Fatal("persisted plan and digest did not reach the writer", err)
	}
}

func TestProviderWorkspacesAreFreshEmptyAndRemoved(t *testing.T) {
	a, cleanA, err := freshProviderWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanA()
	if err := os.WriteFile(filepath.Join(a, "prior-marker"), []byte("private fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	b, cleanB, err := freshProviderWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanB()
	entries, err := os.ReadDir(b)
	if err != nil || a == b || len(entries) != 0 {
		t.Fatal("provider workspace reused or not empty", err)
	}
	cleanA()
	if _, err := os.Stat(filepath.Dir(a)); !os.IsNotExist(err) {
		t.Fatal("first provider temporary root retained")
	}
	if _, err := os.Stat(b); err != nil {
		t.Fatal("cleanup affected another invocation")
	}
	cleanB()
	if _, err := os.Stat(filepath.Dir(b)); !os.IsNotExist(err) {
		t.Fatal("second provider temporary root retained")
	}
}

func TestReasoningIsDiscardedWhileToolsRemainDenied(t *testing.T) {
	if text, primary, err := inspectProviderEvent(map[string]any{"type": "system", "subtype": "thinking_tokens"}, map[string]struct{}{}); err != nil || primary || text != "" {
		t.Fatal("observed progress-only event rejected", err)
	}
	if _, _, err := inspectProviderEvent(map[string]any{"type": "system", "subtype": "unknown_operation"}, map[string]struct{}{}); err == nil {
		t.Fatal("unknown system event accepted")
	}
	event := map[string]any{"type": "assistant", "message": map[string]any{"model": exactClaudeModel, "content": []any{map[string]any{"type": "thinking", "thinking": "private reasoning fixture"}, map[string]any{"type": "text", "text": "final useful answer"}}}}
	text, primary, err := inspectProviderEvent(event, map[string]struct{}{})
	if err != nil || !primary || text != "final useful answer" {
		t.Fatal("valid text was not retained", err)
	}
	message := event["message"].(map[string]any)
	if len(message["content"].([]any)) != 1 {
		t.Fatal("reasoning was forwarded")
	}
	if _, err := roleSlot(writerName, "correction"); err == nil {
		t.Fatal("recovery expanded its three-call allowance")
	}
	errEvent := map[string]any{"type": "assistant", "is_api_error_message": true, "message": map[string]any{"model": "<synthetic>", "content": []any{map[string]any{"type": "text", "text": "API Error: 401 private fixture token"}}}}
	_, _, err = inspectProviderEvent(errEvent, map[string]struct{}{})
	if err == nil || err.Error() != "provider API failure: authentication" {
		t.Fatal("safe diagnostic classification failed", err)
	}
}

func TestFinalAnswerFormatBoundary(t *testing.T) {
	valid := "```json\n" + `{"summary":"The handoff agrees with the captured evidence.","verdict":"pass","findings":[]}` + "\n```"
	if _, err := parseResult(valid, "review"); err != nil {
		t.Fatal("harmless outer JSON fence rejected", err)
	}
	for _, text := range []string{valid + "\nextra", "prefix\n" + valid, "```json\n{}{}\n```"} {
		if _, err := parseResult(text, "review"); err == nil {
			t.Fatal("ambiguous review accepted")
		}
	}
	if _, err := roleSlot(plannerName, "leader"); err == nil {
		t.Fatal("unused planner role could consume a provider call")
	}
	if _, err := parseResult("# Short\nToo short.", "implementation"); err == nil {
		t.Fatal("empty handoff accepted")
	}
}

func TestManualReviewFinalCannotLaunchWriterOrPlanner(t *testing.T) {
	for _, pair := range [][2]string{{writerName, "implementation"}, {writerName, "correction"}, {plannerName, "leader"}, {reviewerName, "review"}} {
		if _, err := roleSlot(pair[0], pair[1]); err == nil {
			t.Fatal("manual review boundary expanded")
		}
	}
	if slot, err := roleSlot(reviewerName, "implementation"); err != nil || slot != "03-review" {
		t.Fatal("exact manual review denied")
	}
}
