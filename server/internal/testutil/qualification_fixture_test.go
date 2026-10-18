package testutil

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestAllHistoricalFixtureScopesVerifyEveryOriginalByte(t *testing.T) {
	raw, err := fs.ReadFile(qualificationFixtureFiles, "qualification_data/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]map[string]string
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for scope, bindings := range manifest {
		files, err := qualificationFixtureSnapshot(qualificationFixtureFiles, scope)
		if err != nil {
			t.Fatalf("%s: %v", scope, err)
		}
		if len(files) != len(bindings) {
			t.Fatalf("incomplete %s", scope)
		}
	}
}

func TestHistoricalFixtureRejectsCorruptionMissingForeignAndUnsafePaths(t *testing.T) {
	raw := []byte("inert historical fixture")
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	fixture := func() fstest.MapFS {
		return fstest.MapFS{
			"qualification_data/manifest.json":     {Data: []byte(fmt.Sprintf(`{"experiment":{"source.go":%q}}`, digest))},
			"qualification_data/" + digest + ".gz": {Data: append([]byte(nil), compressed.Bytes()...)},
		}
	}
	if _, err := qualificationFixtureSnapshot(fixture(), "experiment"); err != nil {
		t.Fatal(err)
	}
	if _, err := qualificationFixtureSnapshot(fixture(), "foreign"); err == nil {
		t.Fatal("foreign scope admitted")
	}
	for _, mutation := range []string{"missing", "corrupt", "modified-bytes", "unsafe-path", "changed-binding"} {
		t.Run(mutation, func(t *testing.T) {
			files := fixture()
			switch mutation {
			case "missing":
				delete(files, "qualification_data/"+digest+".gz")
			case "corrupt":
				files["qualification_data/"+digest+".gz"].Data = []byte("invalid gzip")
			case "modified-bytes":
				var other bytes.Buffer
				w := gzip.NewWriter(&other)
				if _, err := w.Write([]byte("different bytes")); err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				files["qualification_data/"+digest+".gz"].Data = other.Bytes()
			case "unsafe-path":
				files["qualification_data/manifest.json"].Data = []byte(fmt.Sprintf(`{"experiment":{"../source.go":%q}}`, digest))
			case "changed-binding":
				files["qualification_data/manifest.json"].Data = []byte(`{"experiment":{"source.go":"bad-digest"}}`)
			}
			if _, err := qualificationFixtureSnapshot(files, "experiment"); err == nil {
				t.Fatal("damaged fixture admitted")
			}
		})
	}
}
