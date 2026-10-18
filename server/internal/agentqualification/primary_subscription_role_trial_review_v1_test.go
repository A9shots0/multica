package agentqualification

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPrimarySubscriptionRoleTrialIndependentReviewAndTamperDenial(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".qualification-evidence", "item14-primary-subscription-role-trial-v1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "trial-ledger.json")); os.IsNotExist(err) {
		t.Skip("sealed local trial evidence is not present")
	}
	report, err := ReviewPrimarySubscriptionRoleTrialEvidenceV1(root)
	if err != nil || report.Verdict != "PASS" || report.PassObservations != 5 || report.InvalidObservations != 2 || report.FailedObservations != 1 {
		t.Fatalf("sealed evidence review failed: report=%+v err=%v", report, err)
	}
	copyRoot := filepath.Join(t.TempDir(), "tampered")
	if err := copyReviewTree(root, copyRoot); err != nil {
		t.Fatal(err)
	}
	var ledger primarySubscriptionReviewLedger
	raw, err := os.ReadFile(filepath.Join(copyRoot, "trial-ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &ledger); err != nil {
		t.Fatal(err)
	}
	responsePath := filepath.Join(copyRoot, filepath.FromSlash(ledger.Sessions[0].RawResponsePath))
	file, err := os.OpenFile(responsePath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write([]byte("\n"))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("tamper fixture write failed")
	}
	tampered, err := ReviewPrimarySubscriptionRoleTrialEvidenceV1(copyRoot)
	if err != nil || tampered.Verdict != "REVISE" {
		t.Fatalf("tampered evidence passed: report=%+v err=%v", tampered, err)
	}
}

func TestPrimarySubscriptionReliabilityTrialV2IndependentReviewAndTamperDenial(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".qualification-evidence", "item14-primary-subscription-reliability-trial-v2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "trial-ledger.json")); os.IsNotExist(err) {
		t.Skip("sealed local reliability v2 evidence is not present")
	}
	report, err := ReviewPrimarySubscriptionReliabilityTrialEvidenceV2(root)
	if err != nil || report.Verdict != "PASS" || report.PassObservations != 6 || report.InvalidObservations != 0 || report.FailedObservations != 6 || report.ConfigurationSelection != "PROVISIONAL_LOCAL_R01_INLINE_SHARED_EXECUTOR_EXACT_SKILL_ONLY" {
		t.Fatalf("sealed reliability v2 evidence review failed: report=%+v err=%v", report, err)
	}
	copyRoot := filepath.Join(t.TempDir(), "tampered-v2")
	if err := copyReviewTree(root, copyRoot); err != nil {
		t.Fatal(err)
	}
	var ledger primarySubscriptionReviewLedger
	raw, err := os.ReadFile(filepath.Join(copyRoot, "trial-ledger.json"))
	if err != nil || json.Unmarshal(raw, &ledger) != nil {
		t.Fatal("read copied v2 ledger")
	}
	responsePath := filepath.Join(copyRoot, filepath.FromSlash(ledger.Sessions[0].RawResponsePath))
	file, err := os.OpenFile(responsePath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write([]byte("\n"))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("tamper fixture write failed")
	}
	if _, err := ReviewPrimarySubscriptionReliabilityTrialEvidenceV2(copyRoot); err == nil {
		t.Fatal("tampered v2 evidence passed independent review")
	}
}

func copyReviewTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
