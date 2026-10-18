package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

func TestLoadSourceBytesStaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "context.md"), []byte("truth"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadSourceBytes(root, []systemgraph.StewardshipSource{{ID: "context", Path: "projects/context.md"}})
	if err != nil || string(got["projects/context.md"]) != "truth" {
		t.Fatalf("load = %q, %v", got["projects/context.md"], err)
	}
	for _, path := range []string{"../escape", "C:/escape"} {
		if _, err := loadSourceBytes(root, []systemgraph.StewardshipSource{{ID: "bad", Path: path}}); err == nil {
			t.Fatalf("unsafe path %q accepted", path)
		}
	}
}

func TestLoadManifestRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{"version":"silver-mediums.system-stewardship.v1","graph_sha256":"x","role_catalog_sha256":"y","sources":[],"surprise":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(path); err == nil {
		t.Fatal("unknown manifest field accepted")
	}
}
