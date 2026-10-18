package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

func main() {
	manifestPath := flag.String("manifest", "", "path to a stewardship manifest")
	sourceRoot := flag.String("source-root", "", "root containing the manifest's logical source paths")
	flag.Parse()
	if *manifestPath == "" || *sourceRoot == "" {
		fatalf("-manifest and -source-root are required")
	}
	manifest, err := loadManifest(*manifestPath)
	if err != nil {
		fatalf("load manifest: %v", err)
	}
	sources, err := loadSourceBytes(*sourceRoot, manifest.Sources)
	if err != nil {
		fatalf("load sources: %v", err)
	}
	if err := manifest.ValidateCurrent(time.Now().UTC(), sources); err != nil {
		fatalf("%v", err)
	}
	result := struct {
		Valid             bool   `json:"valid"`
		GraphSHA256       string `json:"graph_sha256"`
		RoleCatalogSHA256 string `json:"role_catalog_sha256"`
		SourceCount       int    `json:"source_count"`
	}{true, manifest.GraphSHA256, manifest.RoleCatalogSHA256, len(manifest.Sources)}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatalf("encode result: %v", err)
	}
}

func loadManifest(path string) (systemgraph.StewardshipManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return systemgraph.StewardshipManifest{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest systemgraph.StewardshipManifest
	if err := decoder.Decode(&manifest); err != nil {
		return systemgraph.StewardshipManifest{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return systemgraph.StewardshipManifest{}, fmt.Errorf("unexpected trailing JSON value")
		}
		return systemgraph.StewardshipManifest{}, err
	}
	return manifest, nil
}

func loadSourceBytes(root string, sources []systemgraph.StewardshipSource) (map[string][]byte, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(sources))
	for _, source := range sources {
		clean := filepath.Clean(filepath.FromSlash(source.Path))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("source %q has unsafe path %q", source.ID, source.Path)
		}
		path := filepath.Join(root, clean)
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("source %q escapes source root", source.ID)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", source.ID, err)
		}
		result[source.Path] = contents
	}
	return result, nil
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
