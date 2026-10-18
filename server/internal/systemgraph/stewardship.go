package systemgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const StewardshipManifestVersion = "silver-mediums.system-stewardship.v1"
const SilverMediumsKnowledgeSourceSet = "silver-mediums-knowledge-authority.v1"

// StewardshipSource binds a maintained document to exact bytes and a review
// deadline. Paths are stable logical identifiers; callers decide where the
// bytes are loaded from and must not place credentials in this manifest.
type StewardshipSource struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	TruthOwner  string `json:"truth_owner"`
	ObservedAt  string `json:"observed_at"`
	ReviewAfter string `json:"review_after"`
}

// StewardshipManifest makes the graph, role catalog, and maintained knowledge
// sources one replayable evidence set. It records evidence, not secret values.
type StewardshipManifest struct {
	Version           string              `json:"version"`
	SourceSet         string              `json:"source_set,omitempty"`
	GraphSHA256       string              `json:"graph_sha256"`
	RoleCatalogSHA256 string              `json:"role_catalog_sha256"`
	Sources           []StewardshipSource `json:"sources"`
}

type StewardshipFinding struct {
	SourceID string `json:"source_id"`
	Path     string `json:"path,omitempty"`
	State    string `json:"state"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// GraphArtifactSHA256 identifies the exact embedded graph bytes used by the
// controller. It intentionally complements RolesArtifactSHA256.
func GraphArtifactSHA256() string {
	return stewardshipSHA256(defaultGraphJSON)
}

func NewStewardshipManifest(sources []StewardshipSource) StewardshipManifest {
	copySources := append([]StewardshipSource(nil), sources...)
	sort.Slice(copySources, func(i, j int) bool { return copySources[i].ID < copySources[j].ID })
	return StewardshipManifest{
		Version:           StewardshipManifestVersion,
		GraphSHA256:       GraphArtifactSHA256(),
		RoleCatalogSHA256: RolesArtifactSHA256(),
		Sources:           copySources,
	}
}

// Findings compares the frozen manifest with the embedded graph/catalog and
// caller-supplied maintained document bytes. Missing, stale, malformed, and
// changed evidence remains visible instead of being treated as current truth.
func (m StewardshipManifest) Findings(now time.Time, sourceBytes map[string][]byte) []StewardshipFinding {
	var findings []StewardshipFinding
	if m.Version != StewardshipManifestVersion {
		findings = append(findings, StewardshipFinding{SourceID: "manifest", State: "version_mismatch", Expected: StewardshipManifestVersion, Actual: m.Version})
	}
	if m.GraphSHA256 != GraphArtifactSHA256() {
		findings = append(findings, StewardshipFinding{SourceID: "system_graph", State: "hash_mismatch", Expected: m.GraphSHA256, Actual: GraphArtifactSHA256()})
	}
	if m.RoleCatalogSHA256 != RolesArtifactSHA256() {
		findings = append(findings, StewardshipFinding{SourceID: "role_catalog", State: "hash_mismatch", Expected: m.RoleCatalogSHA256, Actual: RolesArtifactSHA256()})
	}
	seen := make(map[string]struct{}, len(m.Sources))
	manifestPaths := make(map[string]struct{}, len(m.Sources))
	for _, source := range m.Sources {
		if source.ID == "" || source.Path == "" || source.TruthOwner == "" || !validStewardshipSHA256(source.SHA256) {
			findings = append(findings, StewardshipFinding{SourceID: source.ID, Path: source.Path, State: "invalid_source_receipt"})
			continue
		}
		if _, exists := seen[source.ID]; exists {
			findings = append(findings, StewardshipFinding{SourceID: source.ID, Path: source.Path, State: "duplicate_source"})
			continue
		}
		seen[source.ID] = struct{}{}
		manifestPaths[source.Path] = struct{}{}
		actualBytes, ok := sourceBytes[source.Path]
		if !ok {
			findings = append(findings, StewardshipFinding{SourceID: source.ID, Path: source.Path, State: "missing"})
			continue
		}
		actualHash := stewardshipSHA256(actualBytes)
		if actualHash != source.SHA256 {
			findings = append(findings, StewardshipFinding{SourceID: source.ID, Path: source.Path, State: "hash_mismatch", Expected: source.SHA256, Actual: actualHash})
		}
		observedAt, observedErr := time.Parse(time.RFC3339, source.ObservedAt)
		reviewAfter, reviewErr := time.Parse(time.RFC3339, source.ReviewAfter)
		if observedErr != nil || reviewErr != nil || observedAt.After(now) || !reviewAfter.After(observedAt) {
			findings = append(findings, StewardshipFinding{SourceID: source.ID, Path: source.Path, State: "invalid_freshness"})
		} else if !now.Before(reviewAfter) {
			findings = append(findings, StewardshipFinding{SourceID: source.ID, Path: source.Path, State: "stale"})
		}
	}
	if m.SourceSet == SilverMediumsKnowledgeSourceSet {
		for _, required := range silverMediumsKnowledgeRequiredPaths(sourceBytes) {
			if _, ok := manifestPaths[required]; !ok {
				findings = append(findings, StewardshipFinding{SourceID: "source_set", Path: required, State: "unbound_authoritative_source"})
			}
		}
	} else if m.SourceSet != "" {
		findings = append(findings, StewardshipFinding{SourceID: "source_set", State: "unknown_source_set", Actual: m.SourceSet})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].SourceID == findings[j].SourceID {
			return findings[i].State < findings[j].State
		}
		return findings[i].SourceID < findings[j].SourceID
	})
	return findings
}

// silverMediumsKnowledgeRequiredPaths defines the complete maintained authority
// surface used by this pilot. Project files are derived from the bound registry
// rather than from a hard-coded project list, so a newly registered project
// cannot silently escape stewardship.
func silverMediumsKnowledgeRequiredPaths(sourceBytes map[string][]byte) []string {
	required := []string{"AGENTS.md", "START_HERE.md", "GOVERNANCE.md", "CURRENT_STATE.md", "CHANGELOG.md", "projects/registry.json"}
	var registry struct {
		Projects []struct {
			Context   string `json:"context"`
			Changelog string `json:"changelog"`
		} `json:"projects"`
	}
	if raw, ok := sourceBytes["projects/registry.json"]; ok && json.Unmarshal(raw, &registry) == nil {
		for _, project := range registry.Projects {
			required = append(required, project.Context, project.Changelog)
		}
	}
	sort.Strings(required)
	return required
}

func (m StewardshipManifest) ValidateCurrent(now time.Time, sourceBytes map[string][]byte) error {
	if findings := m.Findings(now, sourceBytes); len(findings) != 0 {
		return fmt.Errorf("system stewardship evidence is not current: %+v", findings)
	}
	return nil
}

func stewardshipSHA256(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func validStewardshipSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
