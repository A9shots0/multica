package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

const RealisticFixtureManifestSchemaVersion = "silver-mediums.realistic-fixture-manifest.v1"

type RealisticFixture struct {
	CaseID            string   `json:"case_id"`
	FixtureID         string   `json:"fixture_id"`
	TreeSHA256        string   `json:"tree_sha256"`
	EditablePaths     []string `json:"editable_paths"`
	AllowlistedChecks []string `json:"allowlisted_checks"`
	BrowserRequired   bool     `json:"browser_required"`
}

type RealisticFixtureManifest struct {
	SchemaVersion      string             `json:"schema_version"`
	GroupID            string             `json:"group_id"`
	Status             string             `json:"status"`
	Fixtures           []RealisticFixture `json:"fixtures"`
	CopyPolicy         string             `json:"copy_policy"`
	VerificationPolicy string             `json:"verification_policy"`
	HashPolicy         string             `json:"hash_policy"`
}

//go:embed realistic_fixture_manifest_v1.json
var realisticFixtureManifestV1JSON []byte

func CurrentRealisticFixtureManifest() (RealisticFixtureManifest, error) {
	var manifest RealisticFixtureManifest
	if err := json.Unmarshal(realisticFixtureManifestV1JSON, &manifest); err != nil {
		return manifest, fmt.Errorf("decode realistic fixture manifest: %w", err)
	}
	if err := ValidateRealisticFixtureManifest(manifest); err != nil {
		return RealisticFixtureManifest{}, err
	}
	return manifest, nil
}

func ValidateRealisticFixtureManifest(manifest RealisticFixtureManifest) error {
	if manifest.SchemaVersion != RealisticFixtureManifestSchemaVersion || manifest.GroupID != "realistic-workflow-foundation-v2" {
		return fmt.Errorf("realistic fixture manifest identity is incomplete")
	}
	if manifest.Status != "pending_independent_review" && manifest.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic fixture manifest status is invalid")
	}
	want := []string{"R01:synthetic-authority-contract-v2", "R02:synthetic-weekly-report-v1", "R03:synthetic-release-ledger-v2", "R04:synthetic-exact-once-resume-v1"}
	if len(manifest.Fixtures) != len(want) {
		return fmt.Errorf("realistic fixture manifest must contain four fixtures")
	}
	for index, fixture := range manifest.Fixtures {
		if fixture.CaseID+":"+fixture.FixtureID != want[index] || !validSHA256(fixture.TreeSHA256) || len(fixture.EditablePaths) == 0 || len(fixture.AllowlistedChecks) == 0 {
			return fmt.Errorf("realistic fixture %s is incomplete", fixture.CaseID)
		}
		for _, path := range fixture.EditablePaths {
			if !safeFixtureRelativePath(path) {
				return fmt.Errorf("realistic fixture editable path is unsafe: %s", path)
			}
		}
	}
	if !manifest.Fixtures[1].BrowserRequired || manifest.Fixtures[0].BrowserRequired || manifest.Fixtures[2].BrowserRequired || manifest.Fixtures[3].BrowserRequired {
		return fmt.Errorf("only R02 may require Browser proof")
	}
	if manifest.CopyPolicy != "copy_embedded_fixture_to_new_empty_temp_directory_excluding_generated_cache_files" || manifest.VerificationPolicy != "hidden_verifier_runs_from_clean_copy_and_recomputes_all_evidence" || manifest.HashPolicy != "sha256_of_sorted_relative_path_nul_file_sha256_newline_rows" {
		return fmt.Errorf("realistic fixture policy drifted")
	}
	return nil
}

func safeFixtureRelativePath(path string) bool {
	clean := strings.ReplaceAll(path, "\\", "/")
	return clean != "" && !strings.HasPrefix(clean, "/") && !strings.Contains(clean, "../") && !strings.Contains(clean, ":")
}

// RealisticFixtureTreeSHA256 computes the frozen manifest hash from any
// fixture filesystem. Generated caches are ignored and never copied.
func RealisticFixtureTreeSHA256(fsys fs.FS, root string) (string, error) {
	rows := []string{}
	err := fs.WalkDir(fsys, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".pyc") {
			return nil
		}
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		fileHash := sha256.Sum256(raw)
		relative := strings.TrimPrefix(strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), strings.TrimSuffix(strings.ReplaceAll(root, "\\", "/"), "/")), "/")
		rows = append(rows, relative+"\x00"+hex.EncodeToString(fileHash[:])+"\n")
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rows)
	treeHash := sha256.Sum256([]byte(strings.Join(rows, "")))
	return hex.EncodeToString(treeHash[:]), nil
}
