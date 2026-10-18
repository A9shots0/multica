package agentqualification

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealisticHarnessRejectsPlausibleCompletionClaimsOnEveryDefectiveBaseline(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	group, _ := CurrentRealisticWorkflowGroup()
	manifest, _ := CurrentRealisticFixtureManifest()
	for _, caseID := range []string{"R01", "R02", "R03", "R04"} {
		t.Run(caseID, func(t *testing.T) {
			parent, _ := filepath.Abs(t.TempDir())
			workspace, err := PrepareRealisticCaseWorkspace(parent, caseID)
			if err != nil {
				t.Fatal(err)
			}
			provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "Everything is fixed and verified."}}}
			result, err := runRealisticCandidateFrozen(context.Background(), workspace, "adversarial-finish-"+caseID, 1, provider, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.TerminalState == "interrupted" {
				result, err = resumeRealisticCandidateFrozen(context.Background(), workspace, result, provider, nil)
				if err != nil {
					t.Fatal(err)
				}
			}
			applySealableProviderFixture(&result)
			evidenceRoot := filepath.Join(parent, "evidence")
			receipt, _, err := SealRealisticExecutionReceipt(evidenceRoot, workspace, matrix.Candidates[0], result)
			if err != nil {
				if !strings.Contains(err.Error(), "required runtime evidence is missing") {
					t.Fatal(err)
				}
				return
			}
			grade, err := GradeVerifiedRealisticWorkspace(caseID, workspace.Root, workspace.InitialCopy, evidenceRoot, RealisticIndependentEvidence{ToolReceipt: receipt}, group, manifest)
			if err != nil {
				t.Fatal(err)
			}
			if grade.Verdict != "FAIL" {
				t.Fatalf("unsupported completion claim passed %s", caseID)
			}
		})
	}
}

func TestRealisticControllerRecordsForbiddenActionAsCandidateFailure(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R03")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "deploy", Path: "production"}}}
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "forbidden-action", 1, provider, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.TerminalState != "failed" || result.FailureClass != "response_or_tool_protocol_failure" || result.ProviderCalls != 1 || len(result.ProviderReceipts) != 1 {
		t.Fatalf("forbidden action classification drifted: %+v", result)
	}
}
