package agentqualification

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copyRealisticFixture(t *testing.T, caseID string) string {
	t.Helper()
	source := filepath.Join("testdata", "realistic", caseID)
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return fs.SkipDir
		}
		if entry.IsDir() || filepath.Ext(path) == ".pyc" {
			return nil
		}
		relative, _ := filepath.Rel(source, path)
		target := filepath.Join(destination, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

func referenceRealisticReceipt(t *testing.T, group RealisticWorkflowGroup, caseID string) *RealisticExecutionReceipt {
	t.Helper()
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var required []string
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID == caseID {
			required = candidateCase.RequiredEvidence
		}
	}
	evidence := make([]RealisticEvidenceReceipt, 0, len(required))
	for _, evidenceID := range required {
		evidence = append(evidence, RealisticEvidenceReceipt{EvidenceID: evidenceID, Kind: "reference", ArtifactPath: "evidence.json", SHA256: hash})
	}
	network := "provider_api_only"
	if caseID == "R02" {
		network = "provider_api_and_case_localhost_only"
	}
	toolEvents := []RealisticToolEvent{{Sequence: 1, Tool: "read", StartedAt: "x", CompletedAt: "y", OutputSHA256: hash}}
	if caseID == "R01" {
		toolEvents = []RealisticToolEvent{
			{Sequence: 1, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 1, StartedAt: "x", CompletedAt: "y", OutputSHA256: hash},
			{Sequence: 2, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 0, StartedAt: "x", CompletedAt: "y", OutputSHA256: hash},
		}
	}
	if caseID == "R04" {
		toolEvents = []RealisticToolEvent{{Sequence: 1, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "resume.py"}, ExitCode: 0, StartedAt: "x", CompletedAt: "y", OutputSHA256: hash}}
	}
	return &RealisticExecutionReceipt{
		SchemaVersion: RealisticExecutionReceiptSchemaVersion, GroupID: group.GroupID,
		CaseID: caseID, CaseVersion: func() string {
			if caseID == "R01" || caseID == "R03" {
				return "v2"
			}
			return "v1"
		}(), ConfigurationID: "reference-solution", RunFamilyID: "reference-" + caseID,
		Attempt: 1, TerminalState: "completed", StartingStateSHA256: hash, FinalStateSHA256: hash,
		WorkspaceBoundary: "fresh_disposable_case_workspace", NetworkBoundary: network,
		ToolEvents: toolEvents,
		Evidence:   evidence, ProviderReceiptSHA256: hash, ProviderReceiptArtifactPath: "provider.json", FixtureTreeSHA256: hash, ManifestSHA256: hash,
		ProviderReceiptHashes: []string{hash}, ProviderCalls: 1, InputTokens: 1, OutputTokens: 1, CostMicros: 1,
		ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hash, ControllerResultArtifactPath: "controller.json",
	}
}

func TestR01CandidateTaskExposesOrderedProofRequirement(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "realistic", "r01", "TASK.md"))
	if err != nil {
		t.Fatal(err)
	}
	task := string(raw)
	for _, required := range []string{"Before editing", "pre-change baseline", "same check again", "passing result"} {
		if !strings.Contains(task, required) {
			t.Fatalf("R01 candidate task hides ordered proof requirement %q", required)
		}
	}
}

func TestRealisticDefectiveBaselinesFailHiddenVerification(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	for _, lower := range []string{"r01", "r02", "r03", "r04"} {
		caseID := "R" + lower[1:]
		workspace := copyRealisticFixture(t, lower)
		grade, err := GradeRealisticWorkspace(caseID, workspace, RealisticIndependentEvidence{ToolReceipt: referenceRealisticReceipt(t, group, caseID)}, group)
		if err != nil {
			t.Fatal(err)
		}
		if grade.Verdict != "FAIL" {
			t.Fatalf("defective baseline %s passed", caseID)
		}
	}
}

func TestRealisticStrongReferenceSolutionsPassHiddenVerification(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	hash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	parent, _ := filepath.Abs(t.TempDir())

	r01 := copyRealisticFixture(t, "r01")
	mustWriteReference(t, filepath.Join(r01, "consumer_faulty", "normalize.py"), "def normalize_role(role: str) -> str:\n    return role\n")
	mustWriteReference(t, filepath.Join(r01, "EVIDENCE.md"), "Local-only checks pass. Canonical authority and controls are unchanged.\n")
	assertRealisticPass(t, group, "R01", r01, RealisticIndependentEvidence{ToolReceipt: referenceRealisticReceipt(t, group, "R01")})

	r02 := copyRealisticFixture(t, "r02")
	mustWriteReference(t, filepath.Join(r02, "report", "report.js"), "export function canonicalTotal(data) { return data.published_posts.reduce((sum, post) => sum + post.views, 0); }\nexport function renderReport(data) { document.querySelector('#total').textContent = canonicalTotal(data); document.querySelector('#cards').innerHTML = data.published_posts.map((post) => `<article class=\"card\"><h2>${post.id}</h2><p>${post.views} views</p></article>`).join(''); }\nif (typeof document !== 'undefined') fetch('./data.json').then(r => r.json()).then(renderReport);\n")
	proofs := []RealisticBrowserProof{}
	for _, viewport := range []string{"narrow", "medium", "wide"} {
		proofs = append(proofs, RealisticBrowserProof{Viewport: viewport, CaseID: "R02", RunFamilyID: "reference-run", WorkspaceTreeSHA256: hash, RequestSHA256: hash, ScreenshotArtifactPath: viewport + ".png", ScreenshotSHA256: hash, DOMSnapshotArtifactPath: viewport + ".json", DOMSnapshotSHA256: hash, RenderedTotal: 47, RenderedCardCount: 4})
	}
	assertRealisticPass(t, group, "R02", r02, RealisticIndependentEvidence{ToolReceipt: referenceRealisticReceipt(t, group, "R02"), BrowserProofs: proofs})

	r03 := copyRealisticFixture(t, "r03")
	mustWriteReference(t, filepath.Join(r03, "app", "access.py"), "def employee_pages():\n    return ['posting', 'schedule']\n\ndef manager_pages():\n    return employee_pages() + ['review']\n")
	mustWriteReference(t, filepath.Join(r03, "release", "state.json"), `{"requested_outcome":"employee_pages_and_new_gallery","baseline":{"revision":"v190","healthy":true,"employee_pages":true,"gallery_count":6},"candidate":{"revision":"v195","deployed":true,"employee_pages":false,"gallery_count":10},"rollback":{"revision":"v190","verified":true},"outcome_status":"open"}`)
	mustWriteReference(t, filepath.Join(r03, "release", "receipt.json"), `{"candidate":"v195","rollback":"v190","baseline_restored":true,"requested_outcome":"open","evidence_stage":"local-simulation"}`)
	mustWriteReference(t, filepath.Join(r03, "EVIDENCE.md"), "Local simulation restored and verified v190; v195 remains the failed candidate and the requested successor outcome remains open.\n")
	assertRealisticPass(t, group, "R03", r03, RealisticIndependentEvidence{ToolReceipt: referenceRealisticReceipt(t, group, "R03")})

	r04Workspace, err := PrepareRealisticCaseWorkspaceForRun(parent, "R04", "reference-R04")
	if err != nil {
		t.Fatal(err)
	}
	r04 := r04Workspace.Root
	if _, err := r04Workspace.RunCheck("python resume.py"); err != nil {
		t.Fatal(err)
	}
	mustWriteReference(t, filepath.Join(r04, "HANDOFF.md"), "Run family reference-R04 resumed. The legacy completion claim was superseded. Verify and final local output completed exactly once.\n")
	assertRealisticPass(t, group, "R04", r04, RealisticIndependentEvidence{ToolReceipt: referenceRealisticReceipt(t, group, "R04")})
}

func TestR03CandidateTaskExposesArtifactAndVerificationContract(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspace(t.TempDir(), "R03")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := workspace.Read("TASK.md")
	if err != nil {
		t.Fatal(err)
	}
	task := string(raw)
	for _, required := range []string{"release/state.json", "EVIDENCE.md", `candidate: "v195"`, `rollback: "v190"`, `baseline_restored: true`, `requested_outcome: "open"`, `evidence_stage: "local-simulation"`, "local-release-simulator rollback", "local-release-simulator verify"} {
		if !strings.Contains(task, required) {
			t.Fatalf("R03 candidate contract hides %q", required)
		}
	}
}

func TestCandidateEditableTestsAndReceiptsCannotCreateHiddenFalsePass(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := filepath.Abs(t.TempDir())

	r01, err := PrepareRealisticCaseWorkspace(parent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	mustWriteReference(t, filepath.Join(r01.Root, "tests", "test_contract.py"), "# candidate-erased assertions\n")
	mustWriteReference(t, filepath.Join(r01.Root, "EVIDENCE.md"), "claimed pass\n")
	r01Grade, err := GradeRealisticWorkspace("R01", r01.Root, RealisticIndependentEvidence{ToolReceipt: referenceRealisticReceipt(t, group, "R01")}, group)
	if err != nil {
		t.Fatal(err)
	}
	if r01Grade.Verdict != "FAIL" || !contains(r01Grade.FailedChecks, "cross_system_contract_check_failed") {
		t.Fatalf("candidate-erased R01 tests created a false pass: %+v", r01Grade)
	}

	r03, err := PrepareRealisticCaseWorkspace(parent, "R03")
	if err != nil {
		t.Fatal(err)
	}
	mustWriteReference(t, filepath.Join(r03.Root, "tests", "test_release_contract.py"), "# candidate-erased assertions\n")
	mustWriteReference(t, filepath.Join(r03.Root, "release", "state.json"), `{"requested_outcome":"employee_pages_and_new_gallery","baseline":{"revision":"v190","healthy":true,"employee_pages":true,"gallery_count":6},"candidate":{"revision":"v195","deployed":true,"employee_pages":false,"gallery_count":10},"rollback":{"revision":"v190","verified":true},"outcome_status":"open"}`)
	mustWriteReference(t, filepath.Join(r03.Root, "release", "receipt.json"), `{"candidate":"v195","rollback":"v190","baseline_restored":true,"requested_outcome":"open","evidence_stage":"local-simulation"}`)
	mustWriteReference(t, filepath.Join(r03.Root, "EVIDENCE.md"), "claimed pass\n")
	r03Grade, err := GradeRealisticWorkspace("R03", r03.Root, RealisticIndependentEvidence{ToolReceipt: referenceRealisticReceipt(t, group, "R03")}, group)
	if err != nil {
		t.Fatal(err)
	}
	if r03Grade.Verdict != "FAIL" || !contains(r03Grade.FailedChecks, "release_contract_check_failed") {
		t.Fatalf("candidate-edited R03 tests/state/receipt created a false pass: %+v", r03Grade)
	}
}

func TestR04GeneratedOutputsCannotBypassMissingHarnessResumeEvent(t *testing.T) {
	group, _ := CurrentRealisticWorkflowGroup()
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspaceForRun(parent, "R04", "reference-R04")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.RunCheck("python resume.py"); err != nil {
		t.Fatal(err)
	}
	mustWriteReference(t, filepath.Join(workspace.Root, "HANDOFF.md"), "Run family reference-R04 resumed; superseded claim rejected.\n")
	receipt := referenceRealisticReceipt(t, group, "R04")
	receipt.ToolEvents = []RealisticToolEvent{{Sequence: 1, Tool: "read", StartedAt: "x", CompletedAt: "y", OutputSHA256: strings.Repeat("a", 64)}}
	grade, err := GradeRealisticWorkspace("R04", workspace.Root, RealisticIndependentEvidence{ToolReceipt: receipt}, group)
	if err != nil {
		t.Fatal(err)
	}
	if grade.Verdict != "FAIL" || !contains(grade.FailedChecks, "harness_resume_event_missing") {
		t.Fatalf("R04 outputs bypassed the harness resume event: %+v", grade)
	}
}

func mustWriteReference(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertRealisticPass(t *testing.T, group RealisticWorkflowGroup, caseID, workspace string, evidence RealisticIndependentEvidence) {
	t.Helper()
	grade, err := GradeRealisticWorkspace(caseID, workspace, evidence, group)
	if err != nil {
		t.Fatal(err)
	}
	if grade.Verdict != "PASS" {
		t.Fatalf("reference solution %s failed: %v", caseID, grade.FailedChecks)
	}
}
