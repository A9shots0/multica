package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/taskcontinuity"
)

func TestInstallAdapterCreatesAndUpdatesOneManagedBlock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".codex", "AGENTS.md")
	if err := installAdapter(path, "codex"); err != nil {
		t.Fatalf("installAdapter(create) error = %v", err)
	}
	if err := installAdapter(path, "codex"); err != nil {
		t.Fatalf("installAdapter(update) error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed adapter: %v", err)
	}
	if strings.Count(string(content), taskcontinuity.AdapterBegin) != 1 ||
		!taskcontinuity.InstalledContractMatches(string(content)) {
		t.Fatalf("installed content is not one current managed block:\n%s", content)
	}
}
