package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func SealR04EffortV2ExecutionReceiptForFrozenExecutor(frozenExecutor, candidate RealisticCandidate, evidenceRoot string, workspace RealisticCaseWorkspace, result RealisticControllerResult) (*RealisticExecutionReceipt, string, error) {
	if candidate != frozenExecutor {
		return nil, "", fmt.Errorf("R04 effort v1 executor configuration is not frozen")
	}
	if !filepath.IsAbs(evidenceRoot) || result.TerminalState != "completed" || result.CaseID != workspace.CaseID || result.RunFamilyID == "" || result.Attempt < 1 {
		return nil, "", fmt.Errorf("completed realistic execution identity is required")
	}
	if result.ProviderCalls < 1 || len(result.ProviderReceipts) != result.ProviderCalls || len(result.ProviderReceiptHashes) != result.ProviderCalls {
		return nil, "", fmt.Errorf("provider receipt accounting is incomplete")
	}
	accounting, err := recomputeRealisticProviderAccounting(result.ProviderReceipts)
	if err != nil {
		return nil, "", err
	}
	if result.ProviderCalls != accounting.ProviderCalls || result.InputTokens != accounting.InputTokens || result.OutputTokens != accounting.OutputTokens || result.ReasoningTokens != accounting.ReasoningTokens || result.CostMicros != accounting.CostMicros || !equalStrings(result.ProviderReceiptHashes, accounting.ReceiptHashes) {
		return nil, "", fmt.Errorf("provider receipt hashes or mutable accounting totals do not match raw receipts")
	}
	if result.ProviderAccountingStatus != "exact" || result.ProviderTokenAccountingStatus != "exact" || result.ProviderCostAccountingStatus != "exact" || result.AmbiguousProviderCalls != 0 || result.PendingProviderCall != nil || result.PendingPaidTurn != nil {
		return nil, "", fmt.Errorf("completed provider accounting is not exact and closed")
	}
	if workspace.CaseID == "R04" {
		if result.Attempt != 2 || len(result.AttemptHistory) != 2 || result.AttemptHistory[0].Attempt != 1 || result.AttemptHistory[0].TerminalState != "interrupted" || result.AttemptHistory[1].Attempt != 2 || result.AttemptHistory[1].TerminalState != "completed" {
			return nil, "", fmt.Errorf("R04 does not contain one interrupted and one completed attempt")
		}
	}
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return nil, "", err
	}
	manifest, err := CurrentRealisticFixtureManifest()
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
	providerMessages := make([]sealedRealisticProviderReceipt, 0, len(result.ProviderReceipts))
	for index, raw := range result.ProviderReceipts {
		if !json.Valid(raw) {
			return nil, "", fmt.Errorf("provider receipt %d is not valid JSON", index+1)
		}
		providerMessages = append(providerMessages, sealedRealisticProviderReceipt{SHA256: accounting.ReceiptHashes[index], Raw: append([]byte(nil), raw...)})
	}
	providerRaw, err := json.MarshalIndent(providerMessages, "", "  ")
	if err != nil {
		return nil, "", err
	}
	providerPath := "provider-receipts.json"
	if err := writeOrVerifyEvidenceFile(filepath.Join(evidenceRoot, providerPath), providerRaw); err != nil {
		return nil, "", err
	}
	providerHash := sha256.Sum256(providerRaw)
	controllerRaw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, "", err
	}
	controllerPath := "controller-result.json"
	if err := writeOrVerifyEvidenceFile(filepath.Join(evidenceRoot, controllerPath), controllerRaw); err != nil {
		return nil, "", err
	}
	controllerHash := sha256.Sum256(controllerRaw)

	receipt := &RealisticExecutionReceipt{
		SchemaVersion: RealisticExecutionReceiptSchemaVersion, GroupID: group.GroupID, CaseID: workspace.CaseID, CaseVersion: workspace.CaseVersion,
		ConfigurationID: candidate.ConfigurationID, RunFamilyID: result.RunFamilyID, Attempt: result.Attempt, TerminalState: result.TerminalState,
		StartingStateSHA256: workspace.StartingTreeSHA256, FinalStateSHA256: finalHash, WorkspaceBoundary: "fresh_disposable_case_workspace",
		NetworkBoundary: "provider_api_only", ToolEvents: append([]RealisticToolEvent(nil), result.ToolEvents...), BrowserProofs: append([]RealisticBrowserProof(nil), result.BrowserProofs...), AttemptHistory: append([]RealisticAttemptCheckpoint(nil), result.AttemptHistory...), OwnerInterventions: 0, Retries: 0,
		ExternalSideEffects: []string{}, ProhibitedEvents: []string{}, ProviderReceiptSHA256: hex.EncodeToString(providerHash[:]), ProviderReceiptArtifactPath: providerPath,
		ProviderReceiptHashes: append([]string(nil), accounting.ReceiptHashes...), ProviderCalls: accounting.ProviderCalls, InputTokens: accounting.InputTokens, OutputTokens: accounting.OutputTokens, ReasoningTokens: accounting.ReasoningTokens, CostMicros: accounting.CostMicros,
		ProviderAccountingStatus: result.ProviderAccountingStatus, ProviderTokenAccountingStatus: result.ProviderTokenAccountingStatus, ProviderCostAccountingStatus: result.ProviderCostAccountingStatus,
		ControllerResultSHA256: hex.EncodeToString(controllerHash[:]), ControllerResultArtifactPath: controllerPath,
		FixtureTreeSHA256: workspace.Fixture.TreeSHA256, ManifestSHA256: workspace.ManifestSHA256,
	}
	if workspace.CaseID == "R02" {
		receipt.NetworkBoundary = "provider_api_and_case_localhost_only"
	}
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID != workspace.CaseID {
			continue
		}
		for _, evidenceID := range candidateCase.RequiredEvidence {
			index, supported, deriveErr := deriveRealisticEvidenceIndex(workspace.CaseID, evidenceID, result.RunFamilyID, finalHash, workspace.InitialCopy, workspace.Root, receipt.ToolEvents, receipt.BrowserProofs, receipt.AttemptHistory)
			if deriveErr != nil {
				return nil, "", fmt.Errorf("derive required evidence %s: %w", evidenceID, deriveErr)
			}
			if !supported {
				// Preserve the actual evidence produced. Completeness is a model
				// competence question for the blind grader, not receipt corruption.
				continue
			}
			raw, marshalErr := json.MarshalIndent(index, "", "  ")
			if marshalErr != nil {
				return nil, "", marshalErr
			}
			path := filepath.ToSlash(filepath.Join("proofs", evidenceID+".json"))
			if err := writeOrVerifyEvidenceFile(filepath.Join(evidenceRoot, filepath.FromSlash(path)), raw); err != nil {
				return nil, "", err
			}
			hash := sha256.Sum256(raw)
			receipt.Evidence = append(receipt.Evidence, RealisticEvidenceReceipt{EvidenceID: evidenceID, Kind: "harness_bound_evidence_index", ArtifactPath: path, SHA256: hex.EncodeToString(hash[:])})
		}
	}
	if reasons := VerifyRealisticExecutionReceipt(*receipt, group, manifest, workspace.InitialCopy, workspace.Root, evidenceRoot); len(reasons) != 0 {
		return nil, "", fmt.Errorf("sealed realistic receipt failed recomputation: %v", reasons)
	}
	receiptRaw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", err
	}
	receiptPath := filepath.Join(evidenceRoot, "execution-receipt.json")
	if err := writeOrVerifyEvidenceFile(receiptPath, receiptRaw); err != nil {
		return nil, "", err
	}
	return receipt, receiptPath, nil
}
