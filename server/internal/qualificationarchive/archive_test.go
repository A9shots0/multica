package qualificationarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"testing/fstest"
)

func TestReadRequiresExactScopedBindingAndBytes(t *testing.T) {
	raw := []byte("immutable historical fixture\n")
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	fixture := func() fstest.MapFS {
		return fstest.MapFS{
			"data/manifest.json": {Data: []byte(fmt.Sprintf(`{"experiment":{"source.go":%q}}`, digest))},
			"data/" + digest:     {Data: append([]byte(nil), raw...)},
		}
	}
	if got, err := read(fixture(), "experiment", "source.go", digest); err != nil || string(got) != string(raw) {
		t.Fatalf("exact fixture: %q, %v", got, err)
	}
	for _, name := range []string{"../source.go", "/source.go", "folder/../source.go", `folder\source.go`, "source.go/"} {
		if _, err := read(fixture(), "experiment", name, digest); err == nil {
			t.Fatalf("invalid path admitted: %q", name)
		}
	}
	for _, scope := range []string{"", "foreign-experiment"} {
		if _, err := read(fixture(), scope, "source.go", digest); err == nil {
			t.Fatalf("foreign scope admitted: %q", scope)
		}
	}
	for _, mutation := range []string{"missing-manifest", "malformed-manifest", "missing-blob", "corrupt-blob", "changed-binding"} {
		t.Run(mutation, func(t *testing.T) {
			fs := fixture()
			switch mutation {
			case "missing-manifest":
				delete(fs, "data/manifest.json")
			case "malformed-manifest":
				fs["data/manifest.json"].Data = []byte("{")
			case "missing-blob":
				delete(fs, "data/"+digest)
			case "corrupt-blob":
				fs["data/"+digest].Data = []byte("modified")
			case "changed-binding":
				fs["data/manifest.json"].Data = []byte(`{"experiment":{"source.go":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
			}
			if _, err := read(fs, "experiment", "source.go", digest); err == nil {
				t.Fatal("damaged archive admitted")
			}
		})
	}
	if _, err := read(fixture(), "experiment", "source.go", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err == nil {
		t.Fatal("caller-rebound historical digest admitted")
	}
}
