// Package laneevidence is the one definition of what an implementation-lane run
// recorded for an issue, and of the deterministic checks that decide whether
// that record still stands. The issue bridge writes the record after a lane run
// passes; the bounded review broker re-reads everything from disk and verifies
// it before a tool-free reviewer is asked to classify the result.
package laneevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Record is written by the bridge when a lane run passed for an issue.
type Record struct {
	IssueID        string    `json:"issue_id"`
	TaskID         string    `json:"task_id"`
	RunID          string    `json:"run_id"`
	EvidenceDir    string    `json:"evidence_dir"`
	Repository     string    `json:"repository"`
	PatchSHA256    string    `json:"patch_sha256"`
	OutcomeSHA256  string    `json:"outcome_sha256"`
	RecordedAt     time.Time `json:"recorded_at"`
	ElapsedSeconds float64   `json:"elapsed_seconds"`
}

// RecordPath is where the bridge keeps the record for one issue.
func RecordPath(stateDir, issueID string) string {
	return filepath.Join(stateDir, issueID+".implementation.json")
}

// Digest is the hex SHA-256 of raw bytes.
func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

const maxEvidenceBytes = 8 << 20

// ReadBounded reads a regular, non-link file of bounded size.
func ReadBounded(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}
	if info.Size() > maxEvidenceBytes {
		return nil, fmt.Errorf("%s exceeds the evidence size limit", filepath.Base(path))
	}
	return os.ReadFile(path)
}

// LoadRecord reads the record for an issue and checks it names that issue and
// keeps its evidence inside the allowed evidence root.
func LoadRecord(stateDir, issueID, evidenceRoot string) (Record, error) {
	raw, err := ReadBounded(RecordPath(stateDir, issueID))
	if err != nil {
		return Record{}, errors.New("no lane run was recorded for this issue")
	}
	var record Record
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("the lane record is not valid: %w", err)
	}
	if record.IssueID != issueID {
		return Record{}, errors.New("the lane record names another issue")
	}
	if evidenceRoot != "" {
		root, rootErr := filepath.Abs(evidenceRoot)
		dir, dirErr := filepath.Abs(record.EvidenceDir)
		if rootErr != nil || dirErr != nil {
			return Record{}, errors.New("the lane evidence location cannot be resolved")
		}
		rel, relErr := filepath.Rel(root, dir)
		if relErr != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return Record{}, errors.New("the lane evidence is outside the lane evidence root")
		}
	}
	return record, nil
}

// Verify re-reads the lane evidence and returns every reason it does not stand.
// An empty result means the patch and outcome are unchanged since the bridge
// recorded them and every lane gate is recorded as passed.
func Verify(record Record) []string {
	var problems []string
	patchRaw, err := ReadBounded(filepath.Join(record.EvidenceDir, "final.patch"))
	if err != nil || Digest(patchRaw) != record.PatchSHA256 {
		problems = append(problems, "final.patch is missing or changed since publication")
	}
	outcomeRaw, err := ReadBounded(filepath.Join(record.EvidenceDir, "outcome.json"))
	if err != nil || Digest(outcomeRaw) != record.OutcomeSHA256 {
		return append(problems, "outcome.json is missing or changed since publication")
	}
	var outcome map[string]any
	if json.Unmarshal(outcomeRaw, &outcome) != nil {
		return append(problems, "outcome.json is not valid JSON")
	}
	for _, gate := range []string{"succeeded", "verification_passed", "reviewer_passed", "test_fails_before_change",
		"authority_revoked", "worktree_removed"} {
		if outcome[gate] != true {
			problems = append(problems, "gate not passed: "+gate)
		}
	}
	if value, ok := outcome["regression_passed"]; ok && value != true {
		problems = append(problems, "gate not passed: regression_passed")
	}
	if value, ok := outcome["dependency_install_unchanged"]; ok && value != true {
		problems = append(problems, "gate not passed: dependency_install_unchanged")
	}
	if freeze, ok := outcome["lane_freeze"].(map[string]any); !ok || freeze["intact"] != true {
		problems = append(problems, "the lane was not frozen and intact for this run")
	}
	return problems
}

// ReviewerLine names who reviewed the lane run, from its outcome record.
func ReviewerLine(outcomeRaw []byte) string {
	var outcome struct {
		Reviewer          map[string]any   `json:"reviewer"`
		ReviewerFallbacks []map[string]any `json:"reviewer_fallbacks"`
	}
	_ = json.Unmarshal(outcomeRaw, &outcome)
	line := fmt.Sprintf("reviewed by %v %v at %v effort", outcome.Reviewer["provider"], outcome.Reviewer["model"], outcome.Reviewer["effort"])
	if len(outcome.ReviewerFallbacks) > 0 {
		line += fmt.Sprintf(" (Codex fallback used: %v)", outcome.ReviewerFallbacks[0]["reason"])
	}
	return line
}
