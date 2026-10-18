package systemgraph

import (
	"strings"
	"testing"
	"time"
)

func TestStewardshipManifestAcceptsExactFreshEvidence(t *testing.T) {
	content := []byte("canonical hub evidence")
	manifest := NewStewardshipManifest([]StewardshipSource{{
		ID:          "hub-project-map",
		Path:        "projects/registry.json",
		SHA256:      stewardshipSHA256(content),
		TruthOwner:  "silver-mediums-knowledge",
		ObservedAt:  "2026-09-13T12:00:00Z",
		ReviewAfter: "2026-09-20T12:00:00Z",
	}})
	if err := manifest.ValidateCurrent(time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), map[string][]byte{"projects/registry.json": content}); err != nil {
		t.Fatalf("exact fresh evidence rejected: %v", err)
	}
}

func TestStewardshipManifestExposesDriftStalenessAndMissingSources(t *testing.T) {
	one := []byte("old")
	manifest := NewStewardshipManifest([]StewardshipSource{
		{ID: "changed", Path: "projects/changed.md", SHA256: stewardshipSHA256(one), TruthOwner: "domain", ObservedAt: "2026-09-01T00:00:00Z", ReviewAfter: "2026-09-10T00:00:00Z"},
		{ID: "missing", Path: "projects/missing.md", SHA256: stewardshipSHA256([]byte("missing")), TruthOwner: "domain", ObservedAt: "2026-09-01T00:00:00Z", ReviewAfter: "2026-10-01T00:00:00Z"},
	})
	findings := manifest.Findings(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), map[string][]byte{"projects/changed.md": []byte("new")})
	states := make(map[string]bool)
	for _, finding := range findings {
		states[finding.SourceID+":"+finding.State] = true
	}
	for _, want := range []string{"changed:hash_mismatch", "changed:stale", "missing:missing"} {
		if !states[want] {
			t.Fatalf("missing finding %s: %+v", want, findings)
		}
	}
}

func TestStewardshipManifestFailsClosedOnGraphOrRoleDrift(t *testing.T) {
	manifest := NewStewardshipManifest(nil)
	manifest.GraphSHA256 = strings.Repeat("0", 64)
	manifest.RoleCatalogSHA256 = strings.Repeat("1", 64)
	if err := manifest.ValidateCurrent(time.Now(), nil); err == nil {
		t.Fatal("graph and role drift were accepted")
	}
}

func TestStewardshipManifestRejectsDuplicateAndMalformedReceipts(t *testing.T) {
	content := []byte("same")
	manifest := NewStewardshipManifest([]StewardshipSource{
		{ID: "duplicate", Path: "a", SHA256: stewardshipSHA256(content), TruthOwner: "domain", ObservedAt: "bad", ReviewAfter: "bad"},
		{ID: "duplicate", Path: "b", SHA256: "not-a-hash", TruthOwner: "", ObservedAt: "2026-09-01T00:00:00Z", ReviewAfter: "2026-10-01T00:00:00Z"},
	})
	if err := manifest.ValidateCurrent(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), map[string][]byte{"a": content, "b": content}); err == nil {
		t.Fatal("malformed or duplicate receipts were accepted")
	}
}

func TestStewardshipManifestRejectsFutureObservation(t *testing.T) {
	content := []byte("future")
	manifest := NewStewardshipManifest([]StewardshipSource{{
		ID: "future", Path: "future.md", SHA256: stewardshipSHA256(content),
		TruthOwner: "domain", ObservedAt: "2026-09-14T00:00:00Z", ReviewAfter: "2026-09-20T00:00:00Z",
	}})
	if err := manifest.ValidateCurrent(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), map[string][]byte{"future.md": content}); err == nil {
		t.Fatal("future observation was accepted as current truth")
	}
}

func TestStewardshipKnowledgeSourceSetRequiresGovernanceAndRegistryProjects(t *testing.T) {
	registry := []byte(`{"projects":[{"context":"projects/crm/CONTEXT.md","changelog":"projects/crm/CHANGELOG.md"}]}`)
	manifest := NewStewardshipManifest([]StewardshipSource{
		{ID: "registry", Path: "projects/registry.json", SHA256: stewardshipSHA256(registry), TruthOwner: "hub", ObservedAt: "2026-09-13T00:00:00Z", ReviewAfter: "2026-09-20T00:00:00Z"},
	})
	manifest.SourceSet = SilverMediumsKnowledgeSourceSet
	findings := manifest.Findings(time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), map[string][]byte{"projects/registry.json": registry})
	want := map[string]bool{"AGENTS.md": false, "GOVERNANCE.md": false, "projects/crm/CONTEXT.md": false, "projects/crm/CHANGELOG.md": false}
	for _, finding := range findings {
		if finding.State == "unbound_authoritative_source" {
			if _, ok := want[finding.Path]; ok {
				want[finding.Path] = true
			}
		}
	}
	for path, found := range want {
		if !found {
			t.Fatalf("missing completeness finding for %s: %+v", path, findings)
		}
	}
}
