package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type realisticEvidenceIndex struct {
	EvidenceID            string                       `json:"evidence_id"`
	CaseID                string                       `json:"case_id"`
	RunFamilyID           string                       `json:"run_family_id"`
	FinalStateSHA256      string                       `json:"final_state_sha256"`
	ToolEvents            []RealisticToolEvent         `json:"tool_events"`
	BrowserProofs         []RealisticBrowserProof      `json:"browser_proofs,omitempty"`
	AttemptHistory        []RealisticAttemptCheckpoint `json:"attempt_history,omitempty"`
	CandidateEvidenceFile string                       `json:"candidate_evidence_file,omitempty"`
	ArtifactBindings      []realisticArtifactBinding   `json:"artifact_bindings,omitempty"`
	ToolEventSequences    []int                        `json:"tool_event_sequences,omitempty"`
	BrowserViewports      []string                     `json:"browser_viewports,omitempty"`
	AttemptNumbers        []int                        `json:"attempt_numbers,omitempty"`
	ChangedPaths          []string                     `json:"changed_paths,omitempty"`
}

type realisticProviderAccounting struct {
	ProviderCalls   int
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CostMicros      int64
	ReceiptHashes   []string
}

type realisticRawProviderEnvelope struct {
	JSONL          string          `json:"jsonl"`
	Export         json.RawMessage `json:"export"`
	TransportError string          `json:"transport_error"`
}

type sealedRealisticProviderReceipt struct {
	SHA256 string `json:"sha256"`
	Raw    []byte `json:"raw"`
}

// SealRealisticExecutionReceipt writes only harness-derived, hash-bound
// evidence. It does not turn the candidate's completion claim into a pass; the
// hidden verifier remains the independent semantic decision.
func SealRealisticExecutionReceipt(evidenceRoot string, workspace RealisticCaseWorkspace, candidate RealisticCandidate, result RealisticControllerResult) (*RealisticExecutionReceipt, string, error) {
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
	allowedCandidate, err := IsFrozenRealisticCandidate(candidate)
	if err != nil {
		return nil, "", err
	}
	if !allowedCandidate {
		return nil, "", fmt.Errorf("candidate is not in the realistic matrix")
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

func recomputeRealisticProviderAccounting(rawReceipts [][]byte) (realisticProviderAccounting, error) {
	accounting := realisticProviderAccounting{ProviderCalls: len(rawReceipts)}
	for index, raw := range rawReceipts {
		if !json.Valid(raw) {
			return accounting, fmt.Errorf("provider receipt %d is not valid JSON", index+1)
		}
		hash := sha256.Sum256(raw)
		accounting.ReceiptHashes = append(accounting.ReceiptHashes, hex.EncodeToString(hash[:]))
		var envelope realisticRawProviderEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil || envelope.JSONL == "" || len(envelope.Export) == 0 {
			return accounting, fmt.Errorf("provider receipt %d envelope is incomplete", index+1)
		}
		if envelope.TransportError != "" {
			return accounting, fmt.Errorf("completed execution contains ambiguous provider receipt %d", index+1)
		}
		result, _, err := ParseOpenCodeRunEnvelope([]byte(envelope.JSONL), envelope.Export)
		if err != nil {
			return accounting, fmt.Errorf("recompute provider receipt %d: %w", index+1, err)
		}
		if result.Usage.InputTokens == nil || result.Usage.OutputTokens == nil || result.Usage.ReasoningTokens == nil || result.Usage.ProviderReportedCostMicros == nil {
			return accounting, fmt.Errorf("provider receipt %d usage is incomplete", index+1)
		}
		accounting.InputTokens += *result.Usage.InputTokens
		accounting.OutputTokens += *result.Usage.OutputTokens
		accounting.ReasoningTokens += *result.Usage.ReasoningTokens
		accounting.CostMicros += *result.Usage.ProviderReportedCostMicros
	}
	return accounting, nil
}

func realisticReceiptAccountingMatches(receipt RealisticExecutionReceipt, accounting realisticProviderAccounting) bool {
	return receipt.ProviderCalls == accounting.ProviderCalls && receipt.InputTokens == accounting.InputTokens && receipt.OutputTokens == accounting.OutputTokens && receipt.ReasoningTokens == accounting.ReasoningTokens && receipt.CostMicros == accounting.CostMicros && equalStrings(receipt.ProviderReceiptHashes, accounting.ReceiptHashes)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func writeNewEvidenceFile(path string, raw []byte) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("write-once evidence already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// writeOrVerifyEvidenceFile preserves write-once semantics while allowing a
// crash-restarted finalizer to reuse an identical artifact written before the
// final receipt was committed. Any byte drift remains an integrity failure.
func writeOrVerifyEvidenceFile(path string, raw []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		if !bytes.Equal(existing, raw) {
			return fmt.Errorf("write-once evidence disagrees with recomputed content: %s", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return writeNewEvidenceFile(path, raw)
}
