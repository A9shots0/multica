package loopguard

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDetectorReportsThirdIdenticalMatchedCycleWithoutPayloads(t *testing.T) {
	detector := NewDetector(" Codex ")
	secret := "sk-123456789012345678901234"
	var got *Observation
	for i := 0; i < 3; i++ {
		callID := "call-" + string(rune('a'+i))
		detector.ObserveToolUse(callID, " Exec_Command ", map[string]any{"token": secret, "command": "git status"})
		got = detector.ObserveToolResult(callID, "", "result "+secret, "")
		if i < 2 && got != nil {
			t.Fatalf("reported after %d cycles", i+1)
		}
	}
	if got == nil {
		t.Fatal("third identical cycle did not produce shadow observation")
	}
	if got.Provider != "codex" || got.Tool != "exec_command" || got.ResultStatus != "completed" || got.RepeatedCycles != 3 {
		t.Fatalf("observation = %#v", got)
	}
	raw, err := got.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "git status") || strings.Contains(string(raw), "call-") || strings.Contains(string(raw), "result ") {
		t.Fatalf("observation retained raw payload or transient identity: %s", raw)
	}
	if _, err := Parse(raw); err != nil {
		t.Fatalf("parse canonical observation: %v", err)
	}
}

func TestDetectorIgnoresUnmatchedAndResetsOnDifferentCompletedCycle(t *testing.T) {
	detector := NewDetector("claude")
	if got := detector.ObserveToolResult("missing", "read", "same", "completed"); got != nil {
		t.Fatal("unmatched result produced observation")
	}
	cycle := func(callID, command string) *Observation {
		detector.ObserveToolUse(callID, "bash", map[string]any{"command": command})
		return detector.ObserveToolResult(callID, "bash", "same", "completed")
	}
	cycle("1", "pwd")
	cycle("2", "pwd")
	cycle("different", "ls")
	cycle("3", "pwd")
	cycle("4", "pwd")
	if got := cycle("5", "pwd"); got == nil {
		t.Fatal("expected third consecutive identical completed cycle")
	}
}

func TestFingerprintIgnoresMapOrderAndRedactedSecretValue(t *testing.T) {
	a := NewDetector("codex")
	b := NewDetector("codex")
	for i := 0; i < 3; i++ {
		id := string(rune('a' + i))
		a.ObserveToolUse(id, "read", map[string]any{"b": 2, "a": "sk-aaaaaaaaaaaaaaaaaaaaaaaa"})
		b.ObserveToolUse(id, "read", map[string]any{"a": "sk-bbbbbbbbbbbbbbbbbbbbbbbb", "b": 2})
		gotA := a.ObserveToolResult(id, "", "Bearer abc.def", "completed")
		gotB := b.ObserveToolResult(id, "", "Bearer other.value", "completed")
		if i == 2 && (gotA == nil || gotB == nil || gotA.FingerprintSHA256 != gotB.FingerprintSHA256) {
			t.Fatalf("redacted equivalent cycles differ: %#v %#v", gotA, gotB)
		}
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	d := NewDetector("codex")
	var observation *Observation
	for i := 0; i < 3; i++ {
		id := string(rune('a' + i))
		d.ObserveToolUse(id, "read", map[string]any{})
		observation = d.ObserveToolResult(id, "", "ok", "completed")
	}
	raw, err := observation.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["raw_output"] = "must never be accepted"
	bad, _ := json.Marshal(object)
	if _, err := Parse(bad); err == nil {
		t.Fatal("unknown field accepted")
	}
}
