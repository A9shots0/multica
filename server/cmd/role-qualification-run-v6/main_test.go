package main

import (
	"context"
	"strings"
	"testing"
)

func TestV6CommandNeverImplicitlyExecutesOrPreflights(t *testing.T) {
	for _, args := range [][]string{nil, {"execute"}, {"execute-batch"}, {"execute-batch", "all", "x"}, {"preflight"}} {
		err := run(context.Background(), args)
		if err == nil {
			t.Fatalf("accepted %v", args)
		}
		if args == nil && !strings.Contains(err.Error(), "use plan") {
			t.Fatal(err)
		}
	}
}
func TestV6ProviderFreeCommands(t *testing.T) {
	if err := run(context.Background(), []string{"validate"}); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"status", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
}
