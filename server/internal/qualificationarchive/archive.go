// Package qualificationarchive preserves original experiment inputs. Its data
// proves historical custody only; it never grants current execution authority.
package qualificationarchive

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed data/*
var sources embed.FS

func Read(scope, name, expected string) ([]byte, error) {
	return read(sources, scope, name, expected)
}

func read(source fs.FS, scope, name, expected string) ([]byte, error) {
	if !fs.ValidPath(name) || path.Clean(name) != name || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("invalid historical source path %q", name)
	}
	var manifest map[string]map[string]string
	raw, err := fs.ReadFile(source, "data/manifest.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	digest := manifest[scope][name]
	if len(digest) != 64 || digest != expected {
		return nil, fmt.Errorf("historical binding missing or changed: %s/%s", scope, name)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return nil, fmt.Errorf("invalid historical digest: %w", err)
	}
	raw, err = fs.ReadFile(source, "data/"+digest)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, fmt.Errorf("historical bytes changed: %s/%s", scope, name)
	}
	return raw, nil
}
