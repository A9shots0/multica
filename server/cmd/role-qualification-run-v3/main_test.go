package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRoleQualificationRunV3CommandsAreProviderFree(t *testing.T) {
	for _, args := range [][]string{nil, {"validate"}, {"status", t.TempDir()}, {"replay", t.TempDir()}} {
		if err := run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, bad := range [][]string{{"execute"}, {"run"}, {"provider"}} {
		if err := run(bad); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("provider-like command accepted: %v", bad)
		}
	}
}
func TestRoleQualificationRunV3PlanReportsLocked(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := run([]string{"validate"})
	_ = w.Close()
	os.Stdout = old
	raw, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"provider_calls_allowed": false`) || !strings.Contains(text, `"cells": 36`) {
		t.Fatalf("validation output=%s", text)
	}
}
