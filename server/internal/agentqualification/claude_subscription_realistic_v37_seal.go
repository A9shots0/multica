package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func SealClaudeSubscriptionRealisticV37Execution(evidenceRoot string, workspace RealisticCaseWorkspace, result RealisticControllerResult) (*RealisticExecutionReceipt, string, error) {
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		return nil, "", err
	}
	if !filepath.IsAbs(evidenceRoot) || workspace.CaseID != "R03" || workspace.CaseVersion != "v2" || result.CaseID != "R03" || result.RunFamilyID == "" || result.Attempt != 1 || result.TerminalState != "completed" || result.ProviderCalls < 1 || result.ProviderCalls > d.Caps.MaxProviderInvocations || len(result.ProviderReceipts) != result.ProviderCalls || len(result.ProviderReceiptHashes) != result.ProviderCalls || len(result.AttemptHistory) != 1 {
		return nil, "", fmt.Errorf("completed v37 execution identity is required")
	}
	var input, output, cost int64
	providerMessages := make([]sealedRealisticProviderReceipt, 0, result.ProviderCalls)
	for index, raw := range result.ProviderReceipts {
		receipt, err := ParseClaudeRealisticV37ProviderReceipt(raw)
		if err != nil {
			return nil, "", err
		}
		wantID := fmt.Sprintf("%s:attempt-1:phase-%d", result.RunFamilyID, index+1)
		if receipt.ProviderCallID != wantID || sha256Hex(raw) != result.ProviderReceiptHashes[index] {
			return nil, "", fmt.Errorf("v37 provider receipt order/hash drift")
		}
		var ok bool
		if input, ok = checkedAddClaudeV36(input, receipt.InputTokens); !ok {
			return nil, "", fmt.Errorf("v37 input overflow")
		}
		if output, ok = checkedAddClaudeV36(output, receipt.OutputTokens); !ok {
			return nil, "", fmt.Errorf("v37 output overflow")
		}
		if cost, ok = checkedAddClaudeV36(cost, receipt.CostMicros); !ok {
			return nil, "", fmt.Errorf("v37 cost overflow")
		}
		providerMessages = append(providerMessages, sealedRealisticProviderReceipt{SHA256: result.ProviderReceiptHashes[index], Raw: append([]byte(nil), raw...)})
	}
	if result.InputTokens != input || result.OutputTokens != output || result.ReasoningTokens != 0 || result.CostMicros != cost || result.ProviderAccountingStatus != "exact" || result.ProviderTokenAccountingStatus != "exact" || result.ProviderCostAccountingStatus != "exact" || result.PendingProviderCall != nil || result.PendingPaidTurn != nil || result.AmbiguousProviderCalls != 0 {
		return nil, "", fmt.Errorf("v37 provider accounting is not exact")
	}
	finalHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return nil, "", err
	}
	total, ok := checkedAddClaudeV36(input, output)
	history := result.AttemptHistory[0]
	if !ok || history.Attempt != 1 || history.TerminalState != "completed" || history.ProviderCalls != result.ProviderCalls || history.TotalTokens != total || history.CostMicros != cost || history.WorkspaceTreeSHA256 != finalHash {
		return nil, "", fmt.Errorf("v37 attempt history invalid")
	}
	group, err := CurrentRealisticWorkflowGroup()
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
	receipt := &RealisticExecutionReceipt{SchemaVersion: RealisticExecutionReceiptSchemaVersion, GroupID: group.GroupID, CaseID: "R03", CaseVersion: "v2", ConfigurationID: d.Configuration.ID, RunFamilyID: result.RunFamilyID, Attempt: 1, TerminalState: "completed", StartingStateSHA256: workspace.StartingTreeSHA256, FinalStateSHA256: finalHash, WorkspaceBoundary: "fresh_disposable_case_workspace", NetworkBoundary: "provider_api_only", ToolEvents: append([]RealisticToolEvent(nil), result.ToolEvents...), AttemptHistory: append([]RealisticAttemptCheckpoint(nil), result.AttemptHistory...), OwnerInterventions: 0, Retries: 0, ExternalSideEffects: []string{}, ProhibitedEvents: []string{}, ProviderReceiptSHA256: hex.EncodeToString(providerHash[:]), ProviderReceiptArtifactPath: providerPath, ProviderReceiptHashes: append([]string(nil), result.ProviderReceiptHashes...), ProviderCalls: result.ProviderCalls, InputTokens: input, OutputTokens: output, ReasoningTokens: 0, CostMicros: cost, ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hex.EncodeToString(controllerHash[:]), ControllerResultArtifactPath: controllerPath, FixtureTreeSHA256: workspace.Fixture.TreeSHA256, ManifestSHA256: workspace.ManifestSHA256}
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID != "R03" {
			continue
		}
		for _, evidenceID := range candidateCase.RequiredEvidence {
			index, supported, deriveErr := deriveRealisticEvidenceIndex("R03", evidenceID, result.RunFamilyID, finalHash, workspace.InitialCopy, workspace.Root, receipt.ToolEvents, nil, receipt.AttemptHistory)
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
		return nil, "", fmt.Errorf("v37 receipt invalid: %v", reasons)
	}
	receiptRaw, _ := json.MarshalIndent(receipt, "", "  ")
	receiptPath := filepath.Join(evidenceRoot, "execution-receipt.json")
	if err := writeOrVerifyEvidenceFile(receiptPath, receiptRaw); err != nil {
		return nil, "", err
	}
	return receipt, receiptPath, nil
}

func GradeClaudeSubscriptionRealisticV37(workspace RealisticCaseWorkspace, receipt *RealisticExecutionReceipt, finalSummary string) (RealisticWorkspaceGrade, error) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return RealisticWorkspaceGrade{}, err
	}
	grade, err := GradeRealisticWorkspace("R03", workspace.Root, RealisticIndependentEvidence{ToolReceipt: receipt}, group)
	if err != nil {
		return grade, err
	}
	if reasons := evaluateRealisticDeclarations("R03", *receipt, finalSummary); len(reasons) != 0 {
		grade.Verdict = "FAIL"
		grade.FailedChecks = unique(append(grade.FailedChecks, reasons...))
	}
	return grade, nil
}
