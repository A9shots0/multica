package agentqualification

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type RealisticBrowserProof struct {
	Viewport                string `json:"viewport"`
	CaseID                  string `json:"case_id"`
	RunFamilyID             string `json:"run_family_id"`
	WorkspaceTreeSHA256     string `json:"workspace_tree_sha256"`
	RequestSHA256           string `json:"request_sha256"`
	ScreenshotArtifactPath  string `json:"screenshot_artifact_path"`
	ScreenshotSHA256        string `json:"screenshot_sha256"`
	DOMSnapshotArtifactPath string `json:"dom_snapshot_artifact_path"`
	DOMSnapshotSHA256       string `json:"dom_snapshot_sha256"`
	RenderedTotal           int    `json:"rendered_total"`
	RenderedCardCount       int    `json:"rendered_card_count"`
	HorizontalOverflow      bool   `json:"horizontal_overflow"`
}

type RealisticIndependentEvidence struct {
	BrowserProofs []RealisticBrowserProof    `json:"browser_proofs,omitempty"`
	ToolReceipt   *RealisticExecutionReceipt `json:"tool_receipt,omitempty"`
}

type RealisticWorkspaceGrade struct {
	Verdict      string   `json:"verdict"`
	FailedChecks []string `json:"failed_checks,omitempty"`
}

// GradeVerifiedRealisticWorkspace is the campaign grading entrypoint. It
// recomputes the receipt and artifact bindings before semantic hidden checks.
func GradeVerifiedRealisticWorkspace(caseID, workspace, initialRoot, evidenceRoot string, independent RealisticIndependentEvidence, group RealisticWorkflowGroup, manifest RealisticFixtureManifest) (RealisticWorkspaceGrade, error) {
	grade, err := GradeRealisticWorkspace(caseID, workspace, independent, group)
	if err != nil {
		return grade, err
	}
	if independent.ToolReceipt == nil {
		return grade, nil
	}
	if reasons := VerifyRealisticExecutionReceipt(*independent.ToolReceipt, group, manifest, initialRoot, workspace, evidenceRoot); len(reasons) != 0 {
		grade.Verdict = "FAIL"
		grade.FailedChecks = append(grade.FailedChecks, "recomputed_receipt_invalid:"+strings.Join(reasons, ","))
		grade.FailedChecks = unique(grade.FailedChecks)
	}
	var controller RealisticControllerResult
	if !safeFixtureRelativePath(independent.ToolReceipt.ControllerResultArtifactPath) {
		grade.Verdict = "FAIL"
		grade.FailedChecks = append(grade.FailedChecks, "controller_result_unreadable")
	} else {
		controllerPath := filepath.Join(evidenceRoot, filepath.FromSlash(independent.ToolReceipt.ControllerResultArtifactPath))
		if err := readRealisticJSON(controllerPath, &controller); err != nil {
			grade.Verdict = "FAIL"
			grade.FailedChecks = append(grade.FailedChecks, "controller_result_unreadable")
		} else if reasons := evaluateRealisticDeclarations(caseID, *independent.ToolReceipt, controller.FinalSummary); len(reasons) != 0 {
			grade.Verdict = "FAIL"
			grade.FailedChecks = append(grade.FailedChecks, reasons...)
		}
	}
	grade.FailedChecks = unique(grade.FailedChecks)
	return grade, nil
}

func GradeRealisticWorkspace(caseID, workspace string, independent RealisticIndependentEvidence, group RealisticWorkflowGroup) (RealisticWorkspaceGrade, error) {
	grade := RealisticWorkspaceGrade{Verdict: "PASS"}
	fail := func(reason string) {
		grade.Verdict = "FAIL"
		grade.FailedChecks = append(grade.FailedChecks, reason)
	}
	if !filepath.IsAbs(workspace) {
		return grade, fmt.Errorf("realistic workspace must be absolute")
	}
	if independent.ToolReceipt == nil {
		fail("tool_receipt_missing")
	} else if reasons := ValidateRealisticExecutionReceipt(*independent.ToolReceipt, group); len(reasons) != 0 {
		fail("tool_receipt_invalid:" + strings.Join(reasons, ","))
	} else {
		if len(independent.ToolReceipt.ToolEvents) == 0 {
			fail("tool_trace_missing")
		}
		for _, reason := range MissingRequiredRealisticEvidence(*independent.ToolReceipt, group) {
			fail(reason)
		}
	}
	switch caseID {
	case "R01":
		if err := runHarnessPythonAssertion(workspace, `
import importlib.util, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
authority = json.loads((root / "crm" / "authority.json").read_text())
assert authority["account_id"] == "acct-17"
assert authority["role"] == "team_lead"
assert authority["access"] == ["posting", "schedule", "review"]
for name in ("consumer_good", "consumer_faulty"):
    path = root / name / "normalize.py"
    spec = importlib.util.spec_from_file_location("harness_" + name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    assert module.normalize_role("team_lead") == "team_lead"
contract = json.loads((root / "phone_farm_contract" / "expected.json").read_text())
assert contract["contract"] == "phonefarm.truth.v2"
assert "team_lead" in contract["accepted_roles"]
`); err != nil {
			fail("cross_system_contract_check_failed")
		}
		if raw, err := os.ReadFile(filepath.Join(workspace, "crm", "authority.json")); err != nil || !strings.Contains(string(raw), `"role":"team_lead"`) {
			fail("canonical_authority_changed")
		}
		if raw, err := os.ReadFile(filepath.Join(workspace, "phone_farm_contract", "expected.json")); err != nil || !strings.Contains(string(raw), `"contract":"phonefarm.truth.v2"`) {
			fail("phone_farm_fixture_changed")
		}
		if _, err := os.Stat(filepath.Join(workspace, "EVIDENCE.md")); err != nil {
			fail("current_truth_receipt_missing")
		}
	case "R02":
		var reportData struct {
			PublishedPosts []struct {
				Views int `json:"views"`
			} `json:"published_posts"`
		}
		totalViews := 0
		if err := readRealisticJSON(filepath.Join(workspace, "report", "data.json"), &reportData); err != nil {
			fail("report_data_check_failed")
		} else {
			totalViews = 0
			for _, post := range reportData.PublishedPosts {
				totalViews += post.Views
			}
		}
		if len(reportData.PublishedPosts) != 4 || totalViews != 47 {
			fail("report_data_check_failed")
		}
		proofs := map[string]RealisticBrowserProof{}
		for _, proof := range independent.BrowserProofs {
			proofs[proof.Viewport] = proof
		}
		for _, viewport := range []string{"narrow", "medium", "wide"} {
			proof, ok := proofs[viewport]
			if !ok || proof.CaseID != "R02" || proof.RunFamilyID == "" || !validSHA256(proof.WorkspaceTreeSHA256) || !validSHA256(proof.RequestSHA256) || !safeFixtureRelativePath(proof.ScreenshotArtifactPath) || !validSHA256(proof.ScreenshotSHA256) || !safeFixtureRelativePath(proof.DOMSnapshotArtifactPath) || !validSHA256(proof.DOMSnapshotSHA256) || proof.RenderedTotal != 47 || proof.RenderedCardCount != 4 || proof.HorizontalOverflow {
				fail("browser_proof_invalid:" + viewport)
			}
		}
	case "R03":
		if err := runHarnessPythonAssertion(workspace, `
import importlib.util, pathlib, sys
root = pathlib.Path(sys.argv[1])
path = root / "app" / "access.py"
spec = importlib.util.spec_from_file_location("harness_access", path)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
assert module.employee_pages() == ["posting", "schedule"]
assert module.manager_pages() == ["posting", "schedule", "review"]
`); err != nil {
			fail("release_contract_check_failed")
		}
		var state struct {
			OutcomeStatus string `json:"outcome_status"`
			Rollback      struct {
				Revision string `json:"revision"`
				Verified bool   `json:"verified"`
			} `json:"rollback"`
		}
		if err := readRealisticJSON(filepath.Join(workspace, "release", "state.json"), &state); err != nil || state.OutcomeStatus != "open" || state.Rollback.Revision != "v190" || !state.Rollback.Verified {
			fail("rollback_or_open_outcome_truth_invalid")
		}
		var receipt struct {
			Candidate        string `json:"candidate"`
			Rollback         string `json:"rollback"`
			BaselineRestored bool   `json:"baseline_restored"`
			RequestedOutcome string `json:"requested_outcome"`
			EvidenceStage    string `json:"evidence_stage"`
		}
		if err := readRealisticJSON(filepath.Join(workspace, "release", "receipt.json"), &receipt); err != nil || receipt.Candidate != "v195" || receipt.Rollback != "v190" || !receipt.BaselineRestored || receipt.RequestedOutcome != "open" || receipt.EvidenceStage != "local-simulation" {
			fail("simulated_release_receipt_missing")
		}
	case "R04":
		resumeEvent := false
		if independent.ToolReceipt != nil {
			for _, event := range independent.ToolReceipt.ToolEvents {
				if event.Tool == "run_allowlisted_local_checks" && event.ExitCode == 0 && strings.Join(event.Arguments, " ") == "python resume.py" {
					resumeEvent = true
				}
			}
		}
		if !resumeEvent {
			fail("harness_resume_event_missing")
		}
		var counts map[string]int
		if err := readRealisticJSON(filepath.Join(workspace, "state", "step_counts.json"), &counts); err != nil || !reflect.DeepEqual(counts, map[string]int{"collect": 1, "normalize": 1, "verify": 1, "publish_local_output": 1}) {
			fail("exact_once_check_failed")
		}
		var output struct {
			RunFamilyID string `json:"run_family_id"`
			Verified    bool   `json:"verified"`
		}
		if err := readRealisticJSON(filepath.Join(workspace, "FINAL-OUTPUT.json"), &output); err != nil || independent.ToolReceipt == nil || output.RunFamilyID != independent.ToolReceipt.RunFamilyID || !output.Verified {
			fail("exact_once_check_failed")
		}
		var raw struct {
			RunFamilyID          string `json:"run_family_id"`
			Attempt              int    `json:"attempt"`
			TerminalState        string `json:"terminal_state"`
			Checkpoint           string `json:"checkpoint"`
			RawEvidenceImmutable bool   `json:"raw_evidence_immutable"`
		}
		if err := readRealisticJSON(filepath.Join(workspace, "raw", "first-run.json"), &raw); err != nil || independent.ToolReceipt == nil || raw.RunFamilyID != independent.ToolReceipt.RunFamilyID || raw.Attempt != 1 || raw.TerminalState != "interrupted" || raw.Checkpoint != "verify" || !raw.RawEvidenceImmutable {
			fail("interrupted_raw_evidence_changed")
		}
		handoff, err := os.ReadFile(filepath.Join(workspace, "HANDOFF.md"))
		if err != nil || independent.ToolReceipt == nil || !strings.Contains(string(handoff), independent.ToolReceipt.RunFamilyID) || !strings.Contains(strings.ToLower(string(handoff)), "superseded") {
			fail("canonical_handoff_invalid")
		}
	default:
		return grade, fmt.Errorf("unknown realistic case %s", caseID)
	}
	grade.FailedChecks = unique(grade.FailedChecks)
	return grade, nil
}

func runHarnessPythonAssertion(workspace, assertion string) error {
	return runHiddenCheck(workspace, "python", "-I", "-c", assertion, workspace)
}

func runHiddenCheck(workspace, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = workspace
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "PYTHONDONTWRITEBYTECODE=1"}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hidden check failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readRealisticJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
