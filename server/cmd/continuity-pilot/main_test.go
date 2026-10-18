package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeStrictRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, input := range []string{`{"study_id":"x","surprise":true}`, `{} {}`} {
		var target struct {
			StudyID string `json:"study_id"`
		}
		if err := decodeStrict(strings.NewReader(input), &target); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestRunRejectsExecutionMode(t *testing.T) {
	if err := run("execute", "-", &bytes.Buffer{}); err == nil {
		t.Fatal("execution mode accepted")
	}
}
