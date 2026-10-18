package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealisticRunnerCopiesFreshFixtureAndEnforcesTools(t *testing.T) {
	parent, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Root == workspace.InitialCopy || workspace.StartingTreeSHA256 != workspace.Fixture.TreeSHA256 {
		t.Fatal("workspace was not independently copied and pinned")
	}
	if _, err := workspace.Read("crm/authority.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Read("../outside"); err == nil {
		t.Fatal("runner allowed read outside workspace")
	}
	if err := workspace.Edit("crm/authority.json", []byte("changed")); err == nil {
		t.Fatal("runner allowed edit outside case allowlist")
	}
	if _, err := workspace.RunCheck("curl https://example.com"); err == nil {
		t.Fatal("runner allowed arbitrary command")
	}
	if err := workspace.Edit("consumer_faulty/normalize.py", []byte("def normalize_role(role: str) -> str:\n    return role\n")); err != nil {
		t.Fatal(err)
	}
	if output, err := workspace.RunCheck("python -m unittest discover -s tests -v"); err != nil {
		t.Fatalf("allowlisted check failed: %v: %s", err, output)
	}
	initial, _ := os.ReadFile(filepath.Join(workspace.InitialCopy, "consumer_faulty", "normalize.py"))
	if string(initial) == "def normalize_role(role: str) -> str:\n    return role\n" {
		t.Fatal("candidate edit mutated immutable initial copy")
	}
}

func TestR04RunBoundWorkspaceUsesOneHarnessIdentity(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	const runFamilyID = "candidate-r04-0123456789abcdef"
	workspace, err := PrepareRealisticCaseWorkspaceForRun(parent, "R04", runFamilyID)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{workspace.Root, workspace.InitialCopy} {
		for _, relative := range []string{"state/checkpoint.json", "raw/first-run.json", "tests/test_resume.py"} {
			raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !strings.Contains(string(raw), runFamilyID) || strings.Contains(string(raw), "resume-fixture-17") {
				t.Fatalf("R04 identity was not harness-bound in %s", relative)
			}
		}
	}
	initialHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.InitialCopy), ".")
	if err != nil || initialHash != workspace.StartingTreeSHA256 {
		t.Fatalf("run-bound initial tree was not sealed: hash=%s err=%v", initialHash, err)
	}
}

func TestR04CandidateCannotDirectlyEditHarnessGeneratedOutputs(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspaceForRun(parent, "R04", "locked-output-family")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"state/step_counts.json", "state/resume-action.json", "FINAL-OUTPUT.json"} {
		if err := workspace.Edit(path, []byte(`{"forged":true}`)); err == nil {
			t.Fatalf("candidate directly edited harness-generated output %s", path)
		}
	}
}

func TestR04ResumeActionRecoversCrashWithoutRepeatingCompletedSteps(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspaceForRun(parent, "R04", "crash-safe-family")
	if err != nil {
		t.Fatal(err)
	}
	const actionID = "crash-safe-family:attempt:2:check:python resume.py"
	if _, err := runRealisticResume(workspace.Root, actionID, true); err == nil {
		t.Fatal("injected crash did not interrupt resume action")
	}
	if _, err := runRealisticResume(workspace.Root, actionID, false); err != nil {
		t.Fatalf("prepared resume action did not recover: %v", err)
	}
	if _, err := runRealisticResume(workspace.Root, actionID, false); err != nil {
		t.Fatalf("completed resume action was not idempotent: %v", err)
	}
	if err := verifyRealisticResumeOutput(workspace.Root, "crash-safe-family", map[string]int{"collect": 1, "normalize": 1, "verify": 1, "publish_local_output": 1}); err != nil {
		t.Fatal(err)
	}
}

func TestRealisticReceiptRejectsDeclaredEvidenceWithoutRuntimeIndexes(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("consumer_faulty/normalize.py", []byte("def normalize_role(role: str) -> str:\n    return role\n")); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("EVIDENCE.md", []byte("local reference evidence\n")); err != nil {
		t.Fatal(err)
	}
	finalHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		t.Fatal(err)
	}
	evidenceRoot := t.TempDir()
	evidenceRaw := []byte("sealed verifier evidence")
	providerRaw := []byte("sealed provider receipt")
	if err := os.WriteFile(filepath.Join(evidenceRoot, "evidence.json"), evidenceRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceRoot, "provider.json"), providerRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	evidenceHash := sha256.Sum256(evidenceRaw)
	providerHash := sha256.Sum256(providerRaw)
	group, _ := CurrentRealisticWorkflowGroup()
	manifest, _ := CurrentRealisticFixtureManifest()
	receipt := referenceRealisticReceipt(t, group, "R01")
	receipt.StartingStateSHA256 = workspace.StartingTreeSHA256
	receipt.FinalStateSHA256 = finalHash
	receipt.FixtureTreeSHA256 = workspace.Fixture.TreeSHA256
	receipt.ManifestSHA256 = workspace.ManifestSHA256
	receipt.ProviderReceiptSHA256 = hex.EncodeToString(providerHash[:])
	receipt.ProviderReceiptArtifactPath = "provider.json"
	receipt.ToolEvents[0].TouchedPaths = []string{"consumer_faulty/normalize.py", "EVIDENCE.md"}
	for index := range receipt.Evidence {
		receipt.Evidence[index].ArtifactPath = "evidence.json"
		receipt.Evidence[index].SHA256 = hex.EncodeToString(evidenceHash[:])
	}
	if reasons := VerifyRealisticExecutionReceipt(*receipt, group, manifest, workspace.InitialCopy, workspace.Root, evidenceRoot); !contains(reasons, "evidence_index_invalid:authority_owner_preserved") {
		t.Fatalf("declared evidence without a case-specific runtime index was accepted: %v", reasons)
	}
	receipt.FinalStateSHA256 = workspace.StartingTreeSHA256
	if reasons := VerifyRealisticExecutionReceipt(*receipt, group, manifest, workspace.InitialCopy, workspace.Root, evidenceRoot); !contains(reasons, "final_tree_recompute_failed") {
		t.Fatalf("tampered final hash was accepted: %v", reasons)
	}
}

func TestRealisticRunnerExecutesStaticPreviewAndReleaseSimulatorAdapters(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	r02, err := PrepareRealisticCaseWorkspace(parent, "R02")
	if err != nil {
		t.Fatal(err)
	}
	if output, err := r02.RunCheck("local-static-preview report"); err != nil || !strings.Contains(string(output), "loopback_preview_ready") {
		t.Fatalf("static preview adapter failed: %v: %s", err, output)
	}

	r03, err := PrepareRealisticCaseWorkspace(parent, "R03")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r03.RunCheck("local-release-simulator rollback"); err == nil {
		t.Fatal("release simulator accepted an unverified rollback")
	}
	state := `{"requested_outcome":"employee_pages_and_new_gallery","baseline":{"revision":"v190","healthy":true,"employee_pages":true,"gallery_count":6},"candidate":{"revision":"v195","deployed":true,"employee_pages":false,"gallery_count":10},"rollback":{"revision":"v190","verified":true},"outcome_status":"open"}`
	if err := r03.Edit("release/state.json", []byte(state)); err != nil {
		t.Fatal(err)
	}
	if _, err := r03.RunCheck("local-release-simulator rollback"); err != nil {
		t.Fatalf("rollback adapter rejected valid local state: %v", err)
	}
	if _, err := r03.RunCheck("local-release-simulator verify"); err == nil {
		t.Fatal("release verifier accepted missing receipt")
	}
	receipt := `{"candidate":"v195","rollback":"v190","baseline_restored":true,"requested_outcome":"open","evidence_stage":"local-simulation"}`
	if err := r03.Edit("release/receipt.json", []byte(receipt)); err != nil {
		t.Fatal(err)
	}
	if _, err := r03.RunCheck("local-release-simulator verify"); err != nil {
		t.Fatalf("release verifier rejected valid simulated receipt: %v", err)
	}
}
