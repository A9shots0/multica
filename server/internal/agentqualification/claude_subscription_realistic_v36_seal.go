package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SealClaudeSubscriptionRealisticV36Execution writes the same independently
// recomputable realistic receipt as the OpenCode lane, while replaying native
// Claude Code subscription envelopes instead of pretending they are OpenCode.
func SealClaudeSubscriptionRealisticV36Execution(evidenceRoot string, workspace RealisticCaseWorkspace, result RealisticControllerResult) (*RealisticExecutionReceipt, string, error) {
	d, err := ClaudeSubscriptionRealisticV36()
	if err != nil {
		return nil, "", err
	}
	if !filepath.IsAbs(evidenceRoot) || result.TerminalState != "completed" || result.CaseID != workspace.CaseID || result.RunFamilyID == "" || result.Attempt < 1 || result.ProviderCalls < 1 || len(result.ProviderReceipts) != result.ProviderCalls || len(result.ProviderReceiptHashes) != result.ProviderCalls {
		return nil, "", fmt.Errorf("completed v36 realistic execution identity is required")
	}
	var input, output, cost int64
	for i, raw := range result.ProviderReceipts {
		receipt, parseErr := ParseClaudeRealisticV36ProviderReceipt(raw)
		if parseErr != nil {
			return nil, "", fmt.Errorf("provider receipt %d: %w", i+1, parseErr)
		}
		h := sha256.Sum256(raw)
		if hex.EncodeToString(h[:]) != result.ProviderReceiptHashes[i] {
			return nil, "", fmt.Errorf("provider receipt hash drift")
		}
		var request RealisticProviderRequest
		if err := json.Unmarshal(receipt.RequestJSON, &request); err != nil {
			return nil, "", fmt.Errorf("provider request %d: %w", i+1, err)
		}
		wantAttempt := 1
		if workspace.CaseID == "R04" {
			wantAttempt = 2
		}
		wantCallID := fmt.Sprintf("%s:attempt-%d:turn-%d", result.RunFamilyID, wantAttempt, i+1)
		if receipt.ProviderCallID != wantCallID || request.ProviderTurn != i+1 || request.Attempt != wantAttempt || request.CaseID != workspace.CaseID || request.CaseVersion != workspace.CaseVersion || request.RunFamilyID != result.RunFamilyID {
			return nil, "", fmt.Errorf("provider receipt %d request identity/order drift", i+1)
		}
		var ok bool
		if input, ok = checkedAddClaudeV36(input, receipt.InputTokens); !ok {
			return nil, "", fmt.Errorf("v36 input-token accounting overflow")
		}
		if output, ok = checkedAddClaudeV36(output, receipt.OutputTokens); !ok {
			return nil, "", fmt.Errorf("v36 output-token accounting overflow")
		}
		if cost, ok = checkedAddClaudeV36(cost, receipt.CostMicros); !ok {
			return nil, "", fmt.Errorf("v36 cost accounting overflow")
		}
	}
	if result.InputTokens != input || result.OutputTokens != output || result.ReasoningTokens != 0 || result.CostMicros != cost || result.ProviderAccountingStatus != "exact" || result.ProviderTokenAccountingStatus != "exact" || result.ProviderCostAccountingStatus != "exact" || result.PendingProviderCall != nil || result.PendingPaidTurn != nil || result.AmbiguousProviderCalls != 0 {
		return nil, "", fmt.Errorf("v36 provider accounting is not exact and closed")
	}
	if err := validateClaudeV36AttemptHistory(workspace, result); err != nil {
		return nil, "", err
	}
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return nil, "", err
	}
	if _, err := CurrentRealisticFixtureManifest(); err != nil {
		return nil, "", err
	}
	finalHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		return nil, "", err
	}
	providerMessages := make([]sealedRealisticProviderReceipt, 0, len(result.ProviderReceipts))
	for i, raw := range result.ProviderReceipts {
		providerMessages = append(providerMessages, sealedRealisticProviderReceipt{SHA256: result.ProviderReceiptHashes[i], Raw: append([]byte(nil), raw...)})
	}
	providerRaw, _ := json.MarshalIndent(providerMessages, "", "  ")
	providerPath := "provider-receipts.json"
	if err := writeOrVerifyEvidenceFile(filepath.Join(evidenceRoot, providerPath), providerRaw); err != nil {
		return nil, "", err
	}
	providerHash := sha256.Sum256(providerRaw)
	controllerRaw, _ := json.MarshalIndent(result, "", "  ")
	controllerPath := "controller-result.json"
	if err := writeOrVerifyEvidenceFile(filepath.Join(evidenceRoot, controllerPath), controllerRaw); err != nil {
		return nil, "", err
	}
	controllerHash := sha256.Sum256(controllerRaw)
	receipt := &RealisticExecutionReceipt{
		SchemaVersion: RealisticExecutionReceiptSchemaVersion, GroupID: group.GroupID, CaseID: workspace.CaseID, CaseVersion: workspace.CaseVersion,
		ConfigurationID: d.Configuration.ID, RunFamilyID: result.RunFamilyID, Attempt: result.Attempt, TerminalState: result.TerminalState,
		StartingStateSHA256: workspace.StartingTreeSHA256, FinalStateSHA256: finalHash, WorkspaceBoundary: "fresh_disposable_case_workspace", NetworkBoundary: "provider_api_only",
		ToolEvents: append([]RealisticToolEvent(nil), result.ToolEvents...), AttemptHistory: append([]RealisticAttemptCheckpoint(nil), result.AttemptHistory...), OwnerInterventions: 0, Retries: 0,
		ExternalSideEffects: []string{}, ProhibitedEvents: []string{}, ProviderReceiptSHA256: hex.EncodeToString(providerHash[:]), ProviderReceiptArtifactPath: providerPath,
		ProviderReceiptHashes: append([]string(nil), result.ProviderReceiptHashes...), ProviderCalls: result.ProviderCalls, InputTokens: input, OutputTokens: output, ReasoningTokens: 0, CostMicros: cost,
		ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hex.EncodeToString(controllerHash[:]), ControllerResultArtifactPath: controllerPath,
		FixtureTreeSHA256: workspace.Fixture.TreeSHA256, ManifestSHA256: workspace.ManifestSHA256,
	}
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID != workspace.CaseID {
			continue
		}
		for _, evidenceID := range candidateCase.RequiredEvidence {
			index, supported, deriveErr := deriveRealisticEvidenceIndex(workspace.CaseID, evidenceID, result.RunFamilyID, finalHash, workspace.InitialCopy, workspace.Root, receipt.ToolEvents, nil, receipt.AttemptHistory)
			if deriveErr != nil {
				return nil, "", fmt.Errorf("derive %s: %w", evidenceID, deriveErr)
			}
			if !supported {
				continue
			}
			raw, _ := json.MarshalIndent(index, "", "  ")
			path := filepath.ToSlash(filepath.Join("proofs", evidenceID+".json"))
			if err := writeOrVerifyEvidenceFile(filepath.Join(evidenceRoot, filepath.FromSlash(path)), raw); err != nil {
				return nil, "", err
			}
			h := sha256.Sum256(raw)
			receipt.Evidence = append(receipt.Evidence, RealisticEvidenceReceipt{EvidenceID: evidenceID, Kind: "harness_bound_evidence_index", ArtifactPath: path, SHA256: hex.EncodeToString(h[:])})
		}
	}
	// The common verifier's provider-artifact parser is OpenCode-specific. All
	// non-provider bindings are recomputed here; native Claude receipts were
	// already replayed above byte-for-byte and are intentionally not translated.
	if reasons := ValidateRealisticExecutionReceipt(*receipt, group); len(reasons) != 0 {
		return nil, "", fmt.Errorf("v36 receipt invalid: %v", reasons)
	}
	receiptRaw, _ := json.MarshalIndent(receipt, "", "  ")
	receiptPath := filepath.Join(evidenceRoot, "execution-receipt.json")
	if err := writeOrVerifyEvidenceFile(receiptPath, receiptRaw); err != nil {
		return nil, "", err
	}
	return receipt, receiptPath, nil
}

func checkedAddClaudeV36(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || a > int64(^uint64(0)>>1)-b {
		return 0, false
	}
	return a + b, true
}

func validateClaudeV36AttemptHistory(workspace RealisticCaseWorkspace, result RealisticControllerResult) error {
	if workspace.CaseID == "R03" {
		if result.Attempt != 1 || len(result.AttemptHistory) != 1 || result.AttemptHistory[0].Attempt != 1 || result.AttemptHistory[0].TerminalState != "completed" || result.AttemptHistory[0].ProviderCalls != result.ProviderCalls {
			return fmt.Errorf("v36 R03 attempt history is incomplete")
		}
		return nil
	}
	if workspace.CaseID != "R04" || result.Attempt != 2 || len(result.AttemptHistory) != 2 {
		return fmt.Errorf("v36 R04 exact-once attempts are incomplete")
	}
	first, second := result.AttemptHistory[0], result.AttemptHistory[1]
	if first.Attempt != 1 || first.TerminalState != "interrupted" || first.ProviderCalls != 0 || first.TotalTokens != 0 || first.CostMicros != 0 || first.WorkspaceTreeSHA256 != workspace.StartingTreeSHA256 || second.Attempt != 2 || second.TerminalState != "completed" || second.ProviderCalls != result.ProviderCalls || second.CostMicros != result.CostMicros {
		return fmt.Errorf("v36 R04 exact-once attempts are incomplete")
	}
	total, ok := checkedAddClaudeV36(result.InputTokens, result.OutputTokens)
	if !ok {
		return fmt.Errorf("v36 R04 attempt accounting overflow")
	}
	total, ok = checkedAddClaudeV36(total, result.ReasoningTokens)
	if !ok || second.TotalTokens != total {
		return fmt.Errorf("v36 R04 attempt accounting mismatch")
	}
	finalHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil || second.WorkspaceTreeSHA256 != finalHash {
		return fmt.Errorf("v36 R04 final attempt state mismatch")
	}
	return nil
}

func GradeClaudeSubscriptionRealisticV36(workspace RealisticCaseWorkspace, evidenceRoot string, receipt *RealisticExecutionReceipt, finalSummary string) (RealisticWorkspaceGrade, error) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return RealisticWorkspaceGrade{}, err
	}
	grade, err := GradeRealisticWorkspace(workspace.CaseID, workspace.Root, RealisticIndependentEvidence{ToolReceipt: receipt}, group)
	if err != nil {
		return grade, err
	}
	if receipt == nil {
		return grade, nil
	}
	if reasons := evaluateRealisticDeclarations(workspace.CaseID, *receipt, finalSummary); len(reasons) != 0 {
		grade.Verdict = "FAIL"
		grade.FailedChecks = unique(append(grade.FailedChecks, reasons...))
	}
	return grade, nil
}
