package main

import (
	"os"
	"testing"
)

func TestRoleQualificationRunProviderFreeCommands(t *testing.T) {
	for _, args := range [][]string{{"plan"}, {"validate"}, {"status", t.TempDir()}, {"replay", t.TempDir()}} {
		if err := run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}
func TestRoleQualificationRunHasNoExecuteCommand(t *testing.T) {
	if err := run([]string{"execute"}); err == nil {
		t.Fatal("execute command unexpectedly exists")
	}
}
func TestMain(m *testing.M) { os.Exit(m.Run()) }
