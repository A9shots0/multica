package laneevidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func passingOutcome() map[string]any {
	return map[string]any{
		"succeeded": true, "verification_passed": true, "reviewer_passed": true,
		"test_fails_before_change": true, "authority_revoked": true, "worktree_removed": true,
		"regression_passed": true, "lane_freeze": map[string]any{"intact": true},
		"reviewer": map[string]any{"provider": "codex", "model": "gpt-5.6-luna", "effort": "high"},
	}
}

func writeEvidence(t *testing.T, root string, outcome map[string]any) Record {
	t.Helper()
	dir := filepath.Join(root, "runs", "task-run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	patch := []byte("diff --git a/x b/x\n")
	outcomeRaw, _ := json.Marshal(outcome)
	if err := os.WriteFile(filepath.Join(dir, "final.patch"), patch, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outcome.json"), outcomeRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	return Record{IssueID: "11111111-1111-4111-8111-111111111111", TaskID: "task", RunID: "run", EvidenceDir: dir,
		Repository: "C:/Dev/multica", PatchSHA256: Digest(patch), OutcomeSHA256: Digest(outcomeRaw), RecordedAt: time.Now().UTC()}
}

func TestVerifyAcceptsUnchangedPassingEvidence(t *testing.T) {
	record := writeEvidence(t, t.TempDir(), passingOutcome())
	if problems := Verify(record); len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestVerifyRejectsChangedPatchAndFailedGates(t *testing.T) {
	outcome := passingOutcome()
	outcome["reviewer_passed"] = false
	delete(outcome, "lane_freeze")
	record := writeEvidence(t, t.TempDir(), outcome)
	if err := os.WriteFile(filepath.Join(record.EvidenceDir, "final.patch"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	problems := strings.Join(Verify(record), "; ")
	for _, want := range []string{"final.patch is missing or changed", "gate not passed: reviewer_passed", "not frozen and intact"} {
		if !strings.Contains(problems, want) {
			t.Fatalf("problems %q do not name %q", problems, want)
		}
	}
}

func TestVerifyRejectsAChangedOutcome(t *testing.T) {
	record := writeEvidence(t, t.TempDir(), passingOutcome())
	record.OutcomeSHA256 = strings.Repeat("0", 64)
	if problems := Verify(record); len(problems) == 0 {
		t.Fatal("a changed outcome digest was accepted")
	}
}

func TestLoadRecordKeepsEvidenceInsideTheLaneRoot(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	record := writeEvidence(t, root, passingOutcome())
	raw, _ := json.Marshal(record)
	if err := os.WriteFile(RecordPath(state, record.IssueID), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRecord(state, record.IssueID, filepath.Join(root, "runs")); err != nil {
		t.Fatalf("record inside the lane root was refused: %v", err)
	}
	if _, err := LoadRecord(state, record.IssueID, filepath.Join(root, "elsewhere")); err == nil {
		t.Fatal("record outside the lane root was accepted")
	}
	if _, err := LoadRecord(state, "22222222-2222-4222-8222-222222222222", ""); err == nil {
		t.Fatal("a missing record was accepted")
	}
}

func TestReviewerLineNamesTheLaneReviewer(t *testing.T) {
	raw, _ := json.Marshal(passingOutcome())
	if line := ReviewerLine(raw); line != "reviewed by codex gpt-5.6-luna at high effort" {
		t.Fatalf("line = %q", line)
	}
}
