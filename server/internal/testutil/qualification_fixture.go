package testutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

// QualificationArchiveFS supplies exact inert historical bytes to an explicit
// pure validator. It never resolves provider executables or credentials.
func QualificationArchiveFS(t *testing.T, scope string) fs.FS {
	t.Helper()
	files, err := qualificationFixtureSnapshot(qualificationFixtureFiles, scope)
	if err != nil {
		t.Fatal(err)
	}
	result := fstest.MapFS{}
	for name, raw := range files {
		result[name] = &fstest.MapFile{Data: raw, Mode: 0600}
	}
	return result
}

// QualificationArchiveRoot materializes an explicit historical test fixture
// for validators that read relative files. Every file is non-executable.
func QualificationArchiveRoot(t *testing.T, scope string) string {
	t.Helper()
	files, err := qualificationFixtureSnapshot(qualificationFixtureFiles, scope)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for name, raw := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

//go:embed qualification_data/*
var qualificationFixtureFiles embed.FS

func qualificationFixtureSnapshot(source fs.FS, scope string) (map[string][]byte, error) {
	raw, err := fs.ReadFile(source, "qualification_data/manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest map[string]map[string]string
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	bindings, ok := manifest[scope]
	if !ok || len(bindings) == 0 {
		return nil, fmt.Errorf("historical test fixture scope missing: %s", scope)
	}
	result := make(map[string][]byte, len(bindings))
	for name, digest := range bindings {
		if !fs.ValidPath(name) || path.Clean(name) != name || strings.Contains(name, "\\") {
			return nil, fmt.Errorf("invalid historical test fixture path: %s", name)
		}
		if len(digest) != 64 {
			return nil, fmt.Errorf("invalid historical test fixture digest")
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return nil, err
		}
		compressed, err := fs.ReadFile(source, "qualification_data/"+digest+".gz")
		if err != nil {
			return nil, err
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, 32*1024*1024+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		sum := sha256.Sum256(data)
		if len(data) > 32*1024*1024 || hex.EncodeToString(sum[:]) != digest {
			return nil, fmt.Errorf("historical test fixture bytes changed: %s", name)
		}
		result[name] = data
	}
	return result, nil
}
