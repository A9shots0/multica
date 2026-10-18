package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func applySealableProviderFixture(result *RealisticControllerResult) {
	raw, _ := json.Marshal(map[string]any{"jsonl": openCodeFixtureJSONL, "export": json.RawMessage(openCodeFixtureExport)})
	hash := sha256.Sum256(raw)
	result.ProviderCalls = 1
	result.ProviderReceipts = [][]byte{raw}
	result.ProviderReceiptHashes = []string{hex.EncodeToString(hash[:])}
	result.InputTokens = 200
	result.OutputTokens = 25
	result.ReasoningTokens = 5
	result.CostMicros = 12345
	result.ProviderAccountingStatus = "exact"
	result.ProviderTokenAccountingStatus = "exact"
	result.ProviderCostAccountingStatus = "exact"
}

func TestSealRealisticExecutionReceiptRecomputesWorkspaceAndEvidence(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("consumer_faulty/normalize.py", []byte("def normalize_role(role: str) -> str:\n    return role\n")); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("EVIDENCE.md", []byte("Current local truth: affected and control checks passed; authority unchanged.\n")); err != nil {
		t.Fatal(err)
	}
	finalHash, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	result := RealisticControllerResult{
		CaseID: "R01", RunFamilyID: "seal-family", Attempt: 1, TerminalState: "completed", ProviderCalls: 1,
		ProviderReceipts: [][]byte{[]byte(`{"provider":"fixture"}`)}, ProviderReceiptHashes: []string{hash}, InputTokens: 10, OutputTokens: 5, CostMicros: 1,
		ToolEvents: []RealisticToolEvent{
			{Sequence: 1, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 1, StartedAt: now, CompletedAt: now, OutputSHA256: hash},
			{Sequence: 2, Tool: "edit_within_workspace", ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash, TouchedPaths: []string{"consumer_faulty/normalize.py", "EVIDENCE.md"}},
			{Sequence: 3, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash},
		},
		AttemptHistory: []RealisticAttemptCheckpoint{{Attempt: 1, TerminalState: "completed", ProviderCalls: 1, TotalTokens: 15, CostMicros: 1, WorkspaceTreeSHA256: finalHash}},
	}
	applySealableProviderFixture(&result)
	matrix, _ := CurrentRealisticRunMatrix()
	evidenceRoot := filepath.Join(parent, "evidence")
	receipt, receiptPath, err := SealRealisticExecutionReceipt(evidenceRoot, workspace, matrix.Candidates[0], result)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.FinalStateSHA256 != finalHash || len(receipt.Evidence) == 0 {
		t.Fatalf("sealed receipt incomplete: %+v", receipt)
	}
	for _, evidence := range receipt.Evidence {
		if evidence.EvidenceID != "current_truth_receipt" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(evidenceRoot, filepath.FromSlash(evidence.ArtifactPath)))
		if err != nil {
			t.Fatal(err)
		}
		var index realisticEvidenceIndex
		if err := json.Unmarshal(raw, &index); err != nil {
			t.Fatal(err)
		}
		if len(index.ArtifactBindings) < 5 || len(index.ToolEventSequences) < 2 {
			t.Fatalf("current truth remained a presence-only receipt: %+v", index)
		}
	}
	if _, err := os.Stat(receiptPath); err != nil {
		t.Fatal(err)
	}
	reused, reusedPath, err := SealRealisticExecutionReceipt(evidenceRoot, workspace, matrix.Candidates[0], result)
	if err != nil || reusedPath != receiptPath || !reflect.DeepEqual(reused, receipt) {
		t.Fatalf("identical crash-restarted seal was not reused: path=%s err=%v", reusedPath, err)
	}
	group, _ := CurrentRealisticWorkflowGroup()
	manifest, _ := CurrentRealisticFixtureManifest()
	providerPath := filepath.Join(evidenceRoot, filepath.FromSlash(receipt.ProviderReceiptArtifactPath))
	providerRaw, err := os.ReadFile(providerPath)
	if err != nil {
		t.Fatal(err)
	}
	var providerEntries []sealedRealisticProviderReceipt
	if err := json.Unmarshal(providerRaw, &providerEntries); err != nil {
		t.Fatal(err)
	}
	providerEntries[0].SHA256 = strings.Repeat("f", 64)
	providerRaw, _ = json.MarshalIndent(providerEntries, "", "  ")
	if err := os.WriteFile(providerPath, providerRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	providerArtifactHash := sha256.Sum256(providerRaw)
	receipt.ProviderReceiptSHA256 = hex.EncodeToString(providerArtifactHash[:])
	reasons := VerifyRealisticExecutionReceipt(*receipt, group, manifest, workspace.InitialCopy, workspace.Root, evidenceRoot)
	if !contains(reasons, "provider_receipt_entry_hash_mismatch") {
		t.Fatalf("mutable per-receipt hash was accepted: %v", reasons)
	}
	if err := os.WriteFile(filepath.Join(workspace.Root, "consumer_faulty", "normalize.py"), []byte("tampered after seal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	grade, err := GradeVerifiedRealisticWorkspace("R01", workspace.Root, workspace.InitialCopy, evidenceRoot, RealisticIndependentEvidence{ToolReceipt: receipt}, group, manifest)
	if err != nil || grade.Verdict != "FAIL" {
		t.Fatalf("post-seal workspace tamper was not rejected: grade=%+v err=%v", grade, err)
	}
	if _, _, err := SealRealisticExecutionReceipt(evidenceRoot, workspace, matrix.Candidates[0], result); err == nil {
		t.Fatal("write-once receipt was overwritten")
	}
}

func TestSealPreservesMissingFailingTestBeforeForBlindGrade(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("consumer_faulty/normalize.py", []byte("def normalize_role(role: str) -> str:\n    return role\n")); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("EVIDENCE.md", []byte("Current local truth.\n")); err != nil {
		t.Fatal(err)
	}
	finalHash, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := strings.Repeat("a", 64)
	result := RealisticControllerResult{
		CaseID: "R01", RunFamilyID: "missing-failing-test", Attempt: 1, TerminalState: "completed", ProviderCalls: 1,
		ProviderReceipts: [][]byte{[]byte(`{"provider":"fixture"}`)}, ProviderReceiptHashes: []string{hash},
		ToolEvents: []RealisticToolEvent{
			{Sequence: 1, Tool: "edit_within_workspace", ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash, TouchedPaths: []string{"consumer_faulty/normalize.py", "EVIDENCE.md"}},
			{Sequence: 2, Tool: "run_allowlisted_local_checks", Arguments: []string{"pytest"}, ExitCode: 1, StartedAt: now, CompletedAt: now, OutputSHA256: hash},
			{Sequence: 3, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash},
		},
		AttemptHistory: []RealisticAttemptCheckpoint{{Attempt: 1, TerminalState: "completed", WorkspaceTreeSHA256: finalHash}},
	}
	applySealableProviderFixture(&result)
	matrix, _ := CurrentRealisticRunMatrix()
	evidenceRoot := filepath.Join(parent, "evidence")
	receipt, _, err := SealRealisticExecutionReceipt(evidenceRoot, workspace, matrix.Candidates[0], result)
	if err != nil {
		t.Fatalf("honest incomplete execution did not seal: %v", err)
	}
	group, _ := CurrentRealisticWorkflowGroup()
	manifest, _ := CurrentRealisticFixtureManifest()
	if reasons := VerifyRealisticExecutionReceipt(*receipt, group, manifest, workspace.InitialCopy, workspace.Root, evidenceRoot); len(reasons) != 0 {
		t.Fatalf("honest incomplete receipt failed integrity verification: %v", reasons)
	}
	if reasons := MissingRequiredRealisticEvidence(*receipt, group); !contains(reasons, "required_evidence_missing:failing_test_before") {
		t.Fatalf("missing failing-test evidence was not preserved semantically: %v", reasons)
	}
	grade, err := GradeVerifiedRealisticWorkspace("R01", workspace.Root, workspace.InitialCopy, evidenceRoot, RealisticIndependentEvidence{ToolReceipt: receipt}, group, manifest)
	if err != nil || grade.Verdict != "FAIL" || !contains(grade.FailedChecks, "required_evidence_missing:failing_test_before") {
		t.Fatalf("blind grade did not reject incomplete evidence: grade=%+v err=%v", grade, err)
	}
}

func TestVerifierRejectsFabricatedSimulatedRollbackReceiptEvidence(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R03")
	if err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		"app/access.py":        "def employee_pages():\n    return ['posting', 'schedule']\n\ndef manager_pages():\n    return employee_pages() + ['review']\n",
		"release/state.json":   `{"requested_outcome":"employee_pages_and_new_gallery","baseline":{"revision":"v190","healthy":true,"employee_pages":true,"gallery_count":6},"candidate":{"revision":"v195","deployed":true,"employee_pages":false,"gallery_count":10},"rollback":{"revision":"v190","verified":true},"outcome_status":"open"}`,
		"release/receipt.json": `{"candidate":"v195","rollback":"v190","baseline_restored":true,"requested_outcome":"open","evidence_stage":"local-simulation"}`,
		"EVIDENCE.md":          "Local simulated rollback and release-contract evidence.\n",
	} {
		if err := workspace.Edit(path, []byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	finalHash, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := strings.Repeat("a", 64)
	result := RealisticControllerResult{
		CaseID: "R03", RunFamilyID: "fabricated-rollback", Attempt: 1, TerminalState: "completed", ProviderCalls: 1,
		FinalSummary:     "Rollback complete. The issue is fixed and done.",
		ProviderReceipts: [][]byte{[]byte(`{"provider":"fixture"}`)}, ProviderReceiptHashes: []string{hash},
		ToolEvents: []RealisticToolEvent{
			{Sequence: 1, Tool: "edit_within_workspace", ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash, TouchedPaths: []string{"app/access.py", "release/state.json", "release/receipt.json", "EVIDENCE.md"}},
			{Sequence: 2, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash},
		},
		AttemptHistory: []RealisticAttemptCheckpoint{{Attempt: 1, TerminalState: "completed", WorkspaceTreeSHA256: finalHash}},
	}
	applySealableProviderFixture(&result)
	matrix, _ := CurrentRealisticRunMatrix()
	evidenceRoot := filepath.Join(parent, "evidence")
	receipt, _, err := SealRealisticExecutionReceipt(evidenceRoot, workspace, matrix.Candidates[0], result)
	if err != nil {
		t.Fatal(err)
	}
	group, _ := CurrentRealisticWorkflowGroup()
	manifest, _ := CurrentRealisticFixtureManifest()
	falseSummaryGrade, err := GradeVerifiedRealisticWorkspace("R03", workspace.Root, workspace.InitialCopy, evidenceRoot, RealisticIndependentEvidence{ToolReceipt: receipt}, group, manifest)
	if err != nil || falseSummaryGrade.Verdict != "FAIL" || !contains(falseSummaryGrade.FailedChecks, "rollback_falsely_claimed_complete") {
		t.Fatalf("false completion summary passed verified grading: grade=%+v err=%v", falseSummaryGrade, err)
	}
	for i := range receipt.Evidence {
		if receipt.Evidence[i].EvidenceID != "simulated_rollback_receipt" {
			continue
		}
		path := filepath.Join(evidenceRoot, filepath.FromSlash(receipt.Evidence[i].ArtifactPath))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var index realisticEvidenceIndex
		if err := json.Unmarshal(raw, &index); err != nil {
			t.Fatal(err)
		}
		index.ArtifactBindings[0].SHA256 = strings.Repeat("b", 64)
		raw, _ = json.MarshalIndent(index, "", "  ")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		artifactHash := sha256.Sum256(raw)
		receipt.Evidence[i].SHA256 = hex.EncodeToString(artifactHash[:])
	}
	reasons := VerifyRealisticExecutionReceipt(*receipt, group, manifest, workspace.InitialCopy, workspace.Root, evidenceRoot)
	if !contains(reasons, "evidence_index_mismatch:simulated_rollback_receipt") {
		t.Fatalf("fabricated rollback evidence was accepted: %v", reasons)
	}
}

func TestSealPreservesMissingNoRepeatedCompletedStepForBlindGrade(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R04")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.RunCheck("python resume.py"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("HANDOFF.md", []byte("Run family resume-fixture-17 resumed. Superseded claim rejected.\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Root, "state", "step_counts.json"), []byte(`{"collect":2,"normalize":1,"verify":1,"publish_local_output":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	finalHash, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := strings.Repeat("a", 64)
	result := RealisticControllerResult{
		CaseID: "R04", RunFamilyID: "resume-fixture-17", Attempt: 2, TerminalState: "completed", ProviderCalls: 1,
		ProviderReceipts: [][]byte{[]byte(`{"provider":"fixture"}`)}, ProviderReceiptHashes: []string{hash},
		ToolEvents: []RealisticToolEvent{
			{Sequence: 1, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "resume.py"}, ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash, TouchedPaths: []string{"state/resume-action.json", "state/step_counts.json", "FINAL-OUTPUT.json"}},
			{Sequence: 2, Tool: "edit_within_workspace", ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hash, TouchedPaths: []string{"HANDOFF.md"}},
		},
		AttemptHistory: []RealisticAttemptCheckpoint{{Attempt: 1, TerminalState: "interrupted", WorkspaceTreeSHA256: workspace.StartingTreeSHA256}, {Attempt: 2, TerminalState: "completed", WorkspaceTreeSHA256: finalHash}},
	}
	applySealableProviderFixture(&result)
	matrix, _ := CurrentRealisticRunMatrix()
	evidenceRoot := filepath.Join(parent, "evidence")
	receipt, _, err := SealRealisticExecutionReceipt(evidenceRoot, workspace, matrix.Candidates[0], result)
	if err != nil {
		t.Fatalf("honest repeated-step execution did not seal: %v", err)
	}
	group, _ := CurrentRealisticWorkflowGroup()
	manifest, _ := CurrentRealisticFixtureManifest()
	if reasons := VerifyRealisticExecutionReceipt(*receipt, group, manifest, workspace.InitialCopy, workspace.Root, evidenceRoot); len(reasons) != 0 {
		t.Fatalf("honest repeated-step receipt failed integrity verification: %v", reasons)
	}
	if reasons := MissingRequiredRealisticEvidence(*receipt, group); !contains(reasons, "required_evidence_missing:no_repeated_completed_step") {
		t.Fatalf("missing exact-once evidence was not preserved semantically: %v", reasons)
	}
	grade, err := GradeVerifiedRealisticWorkspace("R04", workspace.Root, workspace.InitialCopy, evidenceRoot, RealisticIndependentEvidence{ToolReceipt: receipt}, group, manifest)
	if err != nil || grade.Verdict != "FAIL" || !contains(grade.FailedChecks, "required_evidence_missing:no_repeated_completed_step") {
		t.Fatalf("blind grade did not reject repeated completed step: grade=%+v err=%v", grade, err)
	}
}

func TestSealRejectsMutableProviderTotalsAndReceiptHash(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	for _, mutation := range []string{"totals", "hash"} {
		t.Run(mutation, func(t *testing.T) {
			parent, _ := filepath.Abs(t.TempDir())
			workspace, err := PrepareRealisticCaseWorkspace(parent, "R01")
			if err != nil {
				t.Fatal(err)
			}
			result := RealisticControllerResult{CaseID: "R01", RunFamilyID: "accounting-" + mutation, Attempt: 1, TerminalState: "completed"}
			applySealableProviderFixture(&result)
			if mutation == "totals" {
				result.CostMicros++
			} else {
				result.ProviderReceiptHashes[0] = strings.Repeat("f", 64)
			}
			_, _, err = SealRealisticExecutionReceipt(filepath.Join(parent, "evidence"), workspace, matrix.Candidates[0], result)
			if err == nil || !strings.Contains(err.Error(), "mutable accounting totals") {
				t.Fatalf("mutable provider %s was accepted: %v", mutation, err)
			}
		})
	}
}
