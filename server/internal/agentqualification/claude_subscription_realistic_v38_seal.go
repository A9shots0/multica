package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SealClaudeSubscriptionRealisticV38Execution closes only a completed R04
// attempt-2 result whose attempt-1 interruption and every paid phase receipt
// replay exactly. It performs no provider work.
func SealClaudeSubscriptionRealisticV38Execution(evidenceRoot string, workspace RealisticCaseWorkspace, result RealisticControllerResult) (*RealisticExecutionReceipt, string, error) {
	d, err := ClaudeSubscriptionRealisticV38()
	if err != nil {
		return nil, "", err
	}
	if !filepath.IsAbs(evidenceRoot) || workspace.CaseID != "R04" || workspace.CaseVersion != "v1" || result.CaseID != "R04" || result.RunFamilyID == "" || result.Attempt != 2 || result.TerminalState != "completed" || result.ProviderCalls < 1 || result.ProviderCalls > d.Caps.MaxProviderInvocations || len(result.ProviderReceipts) != result.ProviderCalls || len(result.ProviderReceiptHashes) != result.ProviderCalls {
		return nil, "", fmt.Errorf("completed v38 R04 execution identity is required")
	}
	var input, output, cost int64
	providerMessages := make([]sealedRealisticProviderReceipt, 0, result.ProviderCalls)
	for index, raw := range result.ProviderReceipts {
		receipt, _, parseErr := ParseClaudeRealisticV38ProviderReceipt(raw)
		if parseErr != nil {
			return nil, "", fmt.Errorf("v38 provider receipt %d: %w", index+1, parseErr)
		}
		wantID := fmt.Sprintf("%s:attempt-2:phase-%d", result.RunFamilyID, index+1)
		if receipt.ProviderCallID != wantID || sha256Hex(raw) != result.ProviderReceiptHashes[index] {
			return nil, "", fmt.Errorf("v38 provider receipt order/hash drift")
		}
		var ok bool
		if input, ok = checkedAddClaudeV36(input, receipt.InputTokens); !ok {
			return nil, "", fmt.Errorf("v38 input overflow")
		}
		if output, ok = checkedAddClaudeV36(output, receipt.OutputTokens); !ok {
			return nil, "", fmt.Errorf("v38 output overflow")
		}
		if cost, ok = checkedAddClaudeV36(cost, receipt.CostMicros); !ok {
			return nil, "", fmt.Errorf("v38 cost overflow")
		}
		providerMessages = append(providerMessages, sealedRealisticProviderReceipt{SHA256: result.ProviderReceiptHashes[index], Raw: append([]byte(nil), raw...)})
	}
	if result.InputTokens != input || result.OutputTokens != output || result.ReasoningTokens != 0 || result.CostMicros != cost || result.ProviderAccountingStatus != "exact" || result.ProviderTokenAccountingStatus != "exact" || result.ProviderCostAccountingStatus != "exact" || result.PendingProviderCall != nil || result.PendingPaidTurn != nil || result.AmbiguousProviderCalls != 0 {
		return nil, "", fmt.Errorf("v38 provider accounting is not exact")
	}
	if err := validateClaudeV36AttemptHistory(workspace, result); err != nil {
		return nil, "", err
	}
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return nil, "", err
	}
	finalHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		return nil, "", err
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
		SchemaVersion: RealisticExecutionReceiptSchemaVersion, GroupID: group.GroupID, CaseID: "R04", CaseVersion: "v1",
		ConfigurationID: d.Configuration.ID, RunFamilyID: result.RunFamilyID, Attempt: 2, TerminalState: "completed",
		StartingStateSHA256: workspace.StartingTreeSHA256, FinalStateSHA256: finalHash, WorkspaceBoundary: "fresh_disposable_case_workspace", NetworkBoundary: "provider_api_only",
		ToolEvents: append([]RealisticToolEvent(nil), result.ToolEvents...), AttemptHistory: append([]RealisticAttemptCheckpoint(nil), result.AttemptHistory...), OwnerInterventions: 0, Retries: 0,
		ExternalSideEffects: []string{}, ProhibitedEvents: []string{}, ProviderReceiptSHA256: hex.EncodeToString(providerHash[:]), ProviderReceiptArtifactPath: providerPath,
		ProviderReceiptHashes: append([]string(nil), result.ProviderReceiptHashes...), ProviderCalls: result.ProviderCalls, InputTokens: input, OutputTokens: output, ReasoningTokens: 0, CostMicros: cost,
		ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hex.EncodeToString(controllerHash[:]), ControllerResultArtifactPath: controllerPath,
		FixtureTreeSHA256: workspace.Fixture.TreeSHA256, ManifestSHA256: workspace.ManifestSHA256,
	}
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID != "R04" {
			continue
		}
		for _, evidenceID := range candidateCase.RequiredEvidence {
			index, supported, deriveErr := deriveRealisticEvidenceIndex("R04", evidenceID, result.RunFamilyID, finalHash, workspace.InitialCopy, workspace.Root, receipt.ToolEvents, nil, receipt.AttemptHistory)
			if deriveErr != nil {
				return nil, "", deriveErr
			}
			if !supported {
				continue
			}
			raw, _ := json.MarshalIndent(index, "", "  ")
			path := filepath.ToSlash(filepath.Join("proofs", evidenceID+".json"))
			if err := writeOrVerifyEvidenceFile(filepath.Join(evidenceRoot, filepath.FromSlash(path)), raw); err != nil {
				return nil, "", err
			}
			hash := sha256.Sum256(raw)
			receipt.Evidence = append(receipt.Evidence, RealisticEvidenceReceipt{EvidenceID: evidenceID, Kind: "harness_bound_evidence_index", ArtifactPath: path, SHA256: hex.EncodeToString(hash[:])})
		}
	}
	if reasons := ValidateRealisticExecutionReceipt(*receipt, group); len(reasons) != 0 {
		return nil, "", fmt.Errorf("v38 receipt invalid: %v", reasons)
	}
	receiptRaw, _ := json.MarshalIndent(receipt, "", "  ")
	receiptPath := filepath.Join(evidenceRoot, "execution-receipt.json")
	if err := writeOrVerifyEvidenceFile(receiptPath, receiptRaw); err != nil {
		return nil, "", err
	}
	return receipt, receiptPath, nil
}

func GradeClaudeSubscriptionRealisticV38(workspace RealisticCaseWorkspace, receipt *RealisticExecutionReceipt, finalSummary string) (RealisticWorkspaceGrade, error) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return RealisticWorkspaceGrade{}, err
	}
	grade, err := GradeRealisticWorkspace("R04", workspace.Root, RealisticIndependentEvidence{ToolReceipt: receipt}, group)
	if err != nil {
		return grade, err
	}
	if reasons := evaluateRealisticDeclarations("R04", *receipt, finalSummary); len(reasons) != 0 {
		grade.Verdict = "FAIL"
		grade.FailedChecks = unique(append(grade.FailedChecks, reasons...))
	}
	return grade, nil
}
