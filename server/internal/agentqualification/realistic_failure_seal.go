package agentqualification

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
)

const RealisticFailureEvidenceSchemaVersion = "silver-mediums.realistic-failure-evidence.v1"

type RealisticFailureEvidence struct {
	SchemaVersion             string   `json:"schema_version"`
	ClaimID                   string   `json:"claim_id"`
	ConfigurationID           string   `json:"configuration_id"`
	CaseID                    string   `json:"case_id"`
	RunFamilyID               string   `json:"run_family_id"`
	TerminalState             string   `json:"terminal_state"`
	FailureClass              string   `json:"failure_class"`
	ProviderCalls             int      `json:"provider_calls"`
	AvailableProviderReceipts int      `json:"available_provider_receipts"`
	MissingProviderReceipts   int      `json:"missing_provider_receipts"`
	ProviderReceiptHashes     []string `json:"provider_receipt_hashes"`
	KnownInputTokens          int64    `json:"known_input_tokens"`
	KnownOutputTokens         int64    `json:"known_output_tokens"`
	KnownReasoningTokens      int64    `json:"known_reasoning_tokens"`
	KnownCostMicros           int64    `json:"known_cost_micros"`
	ReceiptsWithKnownTokens   int      `json:"receipts_with_known_tokens"`
	ReceiptsWithKnownCost     int      `json:"receipts_with_known_cost"`
	ProviderAccountingStatus  string   `json:"provider_accounting_status"`
	TokenAccountingStatus     string   `json:"token_accounting_status"`
	CostAccountingStatus      string   `json:"cost_accounting_status"`
	AmbiguousProviderCalls    int      `json:"ambiguous_provider_calls"`
	ControllerResultSHA256    string   `json:"controller_result_sha256"`
	RawProviderReceiptsSHA256 string   `json:"raw_provider_receipts_sha256"`
}

// SealRealisticFailure atomically writes a write-once directory containing the
// controller, every available raw provider receipt, recomputed known
// accounting, and the campaign observation that binds them. A zero-call seal
// is valid only for an approved terminal harness-side failure.
func SealRealisticFailure(evidenceRoot string, observation RealisticObservation, controller RealisticControllerResult) (RealisticObservation, string, error) {
	zeroCallSealedFailure := observation.ProviderCalls == 0 && (observation.Outcome == "invalid" && observation.FailureClass == "test_invalidity" || observation.Outcome == "global_halt" && contains([]string{"test_invalidity", "budget_breach", "credential_failure"}, observation.FailureClass))
	if !filepath.IsAbs(evidenceRoot) || observation.ProviderCalls < 0 || observation.ClaimID == "" || observation.ConfigurationID == "" || observation.CaseID == "" || controller.RunFamilyID == "" || controller.CaseID != observation.CaseID || controller.FailureClass != observation.FailureClass || controller.ProviderCalls != observation.ProviderCalls || observation.ProviderCalls == 0 && !zeroCallSealedFailure {
		return observation, "", fmt.Errorf("failure identity is incomplete")
	}
	if controller.TerminalState != "failed" && controller.TerminalState != "invalid" {
		return observation, "", fmt.Errorf("paid failure controller is not terminal")
	}
	if len(controller.ProviderReceipts) > controller.ProviderCalls || len(controller.ProviderReceiptHashes) != len(controller.ProviderReceipts) {
		return observation, "", fmt.Errorf("failure receipt cardinality is invalid")
	}
	if zeroCallSealedFailure && (len(controller.ProviderReceipts) != 0 || len(controller.ProviderReceiptHashes) != 0 || controller.InputTokens != 0 || controller.OutputTokens != 0 || controller.ReasoningTokens != 0 || controller.CostMicros != 0 || controller.AmbiguousProviderCalls != 0 || controller.PendingProviderCall != nil || controller.PendingPaidTurn != nil || observation.TotalTokens != 0 || observation.CostMicros != 0 || controller.ProviderAccountingStatus != "exact" || controller.ProviderTokenAccountingStatus != "exact" || controller.ProviderCostAccountingStatus != "exact") {
		return observation, "", fmt.Errorf("zero-call sealed failure accounting is not exact and empty")
	}
	sealedReceipts := make([]sealedRealisticProviderReceipt, 0, len(controller.ProviderReceipts))
	known := realisticProviderAccounting{}
	knownTokenReceipts, knownCostReceipts := 0, 0
	for index, raw := range controller.ProviderReceipts {
		hash := sha256.Sum256(raw)
		digest := hex.EncodeToString(hash[:])
		if controller.ProviderReceiptHashes[index] != digest {
			return observation, "", fmt.Errorf("paid failure provider receipt hash mismatch")
		}
		sealedReceipts = append(sealedReceipts, sealedRealisticProviderReceipt{SHA256: digest, Raw: append([]byte(nil), raw...)})
		usage, tokensKnown, costKnown := recomputeKnownFailureUsage(raw)
		known.InputTokens += usage.InputTokens
		known.OutputTokens += usage.OutputTokens
		known.ReasoningTokens += usage.ReasoningTokens
		known.CostMicros += usage.CostMicros
		if tokensKnown {
			knownTokenReceipts++
		}
		if costKnown {
			knownCostReceipts++
		}
	}
	if controller.InputTokens < known.InputTokens || controller.OutputTokens < known.OutputTokens || controller.ReasoningTokens < known.ReasoningTokens || controller.CostMicros < known.CostMicros {
		return observation, "", fmt.Errorf("paid failure mutable accounting is below recomputed known usage")
	}
	if !validFailureAccountingStatus(controller.ProviderAccountingStatus, "exact", "partial", "ambiguous") || !validFailureAccountingStatus(controller.ProviderTokenAccountingStatus, "exact", "reported", "unknown") || !validFailureAccountingStatus(controller.ProviderCostAccountingStatus, "exact", "reported", "unknown") {
		return observation, "", fmt.Errorf("paid failure accounting status is not explicit")
	}
	if controller.ProviderAccountingStatus == "exact" && (len(controller.ProviderReceipts) != controller.ProviderCalls || knownTokenReceipts != len(controller.ProviderReceipts) || knownCostReceipts != len(controller.ProviderReceipts) || controller.InputTokens != known.InputTokens || controller.OutputTokens != known.OutputTokens || controller.ReasoningTokens != known.ReasoningTokens || controller.CostMicros != known.CostMicros) {
		return observation, "", fmt.Errorf("paid failure exact accounting is not fully recomputable")
	}
	controllerRaw, err := json.MarshalIndent(controller, "", "  ")
	if err != nil {
		return observation, "", err
	}
	receiptsRaw, err := json.MarshalIndent(sealedReceipts, "", "  ")
	if err != nil {
		return observation, "", err
	}
	controllerHash, receiptsHash := sha256.Sum256(controllerRaw), sha256.Sum256(receiptsRaw)
	evidence := RealisticFailureEvidence{
		SchemaVersion: RealisticFailureEvidenceSchemaVersion, ClaimID: observation.ClaimID, ConfigurationID: observation.ConfigurationID, CaseID: observation.CaseID, RunFamilyID: controller.RunFamilyID, TerminalState: controller.TerminalState, FailureClass: controller.FailureClass,
		ProviderCalls: controller.ProviderCalls, AvailableProviderReceipts: len(controller.ProviderReceipts), MissingProviderReceipts: controller.ProviderCalls - len(controller.ProviderReceipts), ProviderReceiptHashes: append([]string(nil), controller.ProviderReceiptHashes...),
		KnownInputTokens: known.InputTokens, KnownOutputTokens: known.OutputTokens, KnownReasoningTokens: known.ReasoningTokens, KnownCostMicros: known.CostMicros, ReceiptsWithKnownTokens: knownTokenReceipts, ReceiptsWithKnownCost: knownCostReceipts,
		ProviderAccountingStatus: controller.ProviderAccountingStatus, TokenAccountingStatus: controller.ProviderTokenAccountingStatus, CostAccountingStatus: controller.ProviderCostAccountingStatus, AmbiguousProviderCalls: controller.AmbiguousProviderCalls,
		ControllerResultSHA256: hex.EncodeToString(controllerHash[:]), RawProviderReceiptsSHA256: hex.EncodeToString(receiptsHash[:]),
	}
	evidenceRaw, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return observation, "", err
	}
	evidenceHash := sha256.Sum256(evidenceRaw)
	accountingRaw, _ := json.Marshal(struct {
		ProviderCalls, Available, Missing, KnownTokenReceipts, KnownCostReceipts int
		InputTokens, OutputTokens, ReasoningTokens, CostMicros                   int64
		AccountingStatus, TokenStatus, CostStatus                                string
	}{controller.ProviderCalls, len(controller.ProviderReceipts), controller.ProviderCalls - len(controller.ProviderReceipts), knownTokenReceipts, knownCostReceipts, known.InputTokens, known.OutputTokens, known.ReasoningTokens, known.CostMicros, controller.ProviderAccountingStatus, controller.ProviderTokenAccountingStatus, controller.ProviderCostAccountingStatus})
	accountingHash := sha256.Sum256(accountingRaw)
	observation.FailureEvidenceSHA256 = hex.EncodeToString(evidenceHash[:])
	observation.ControllerResultSHA256 = evidence.ControllerResultSHA256
	observation.RawProviderReceiptsSHA256 = evidence.RawProviderReceiptsSHA256
	observation.ProviderAccountingSHA256 = hex.EncodeToString(accountingHash[:])
	if observation.TotalTokens != controller.InputTokens+controller.OutputTokens+controller.ReasoningTokens || observation.CostMicros != controller.CostMicros {
		return observation, "", fmt.Errorf("paid failure observation accounting does not match controller")
	}
	observationRaw, err := json.MarshalIndent(observation, "", "  ")
	if err != nil {
		return observation, "", err
	}
	finalDir := filepath.Join(evidenceRoot, "failure-seal")
	if _, err := os.Stat(finalDir); err == nil {
		observationPath := filepath.Join(finalDir, "observation.json")
		if err := VerifyRealisticPaidFailureSeal(observationPath, observation); err != nil {
			return observation, "", fmt.Errorf("persisted paid failure evidence disagrees with recomputed evidence: %w", err)
		}
		return observation, observationPath, nil
	} else if !os.IsNotExist(err) {
		return observation, "", err
	}
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		return observation, "", err
	}
	tempDir, err := os.MkdirTemp(evidenceRoot, ".failure-seal-tmp-")
	if err != nil {
		return observation, "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(tempDir)
		}
	}()
	for name, raw := range map[string][]byte{"controller-result.json": controllerRaw, "provider-receipts.json": receiptsRaw, "failure-evidence.json": evidenceRaw, "observation.json": observationRaw} {
		if err := os.WriteFile(filepath.Join(tempDir, name), raw, 0o600); err != nil {
			return observation, "", err
		}
	}
	if err := os.Rename(tempDir, finalDir); err != nil {
		return observation, "", err
	}
	committed = true
	return observation, filepath.Join(finalDir, "observation.json"), nil
}

// SealRealisticPaidFailure retains the narrower API used by paid-failure
// callers while the generalized sealer also handles approved zero-call halts.
func SealRealisticPaidFailure(evidenceRoot string, observation RealisticObservation, controller RealisticControllerResult) (RealisticObservation, string, error) {
	if observation.ProviderCalls < 1 {
		return observation, "", fmt.Errorf("paid failure identity is incomplete")
	}
	return SealRealisticFailure(evidenceRoot, observation, controller)
}

func VerifyRealisticPaidFailureSeal(observationPath string, observation RealisticObservation) error {
	if !filepath.IsAbs(observationPath) {
		return fmt.Errorf("paid failure seal path must be absolute")
	}
	dir := filepath.Dir(observationPath)
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, name)) }
	observationRaw, err := read("observation.json")
	if err != nil {
		return err
	}
	var sealedObservation RealisticObservation
	if json.Unmarshal(observationRaw, &sealedObservation) != nil || sealedObservation != observation {
		return fmt.Errorf("paid failure observation artifact drifted")
	}
	controllerRaw, err := read("controller-result.json")
	if err != nil {
		return err
	}
	receiptsRaw, err := read("provider-receipts.json")
	if err != nil {
		return err
	}
	evidenceRaw, err := read("failure-evidence.json")
	if err != nil {
		return err
	}
	controllerHash, receiptsHash, evidenceHash := sha256.Sum256(controllerRaw), sha256.Sum256(receiptsRaw), sha256.Sum256(evidenceRaw)
	if observation.ControllerResultSHA256 != hex.EncodeToString(controllerHash[:]) || observation.RawProviderReceiptsSHA256 != hex.EncodeToString(receiptsHash[:]) || observation.FailureEvidenceSHA256 != hex.EncodeToString(evidenceHash[:]) {
		return fmt.Errorf("paid failure seal hash mismatch")
	}
	var evidence RealisticFailureEvidence
	var receipts []sealedRealisticProviderReceipt
	if json.Unmarshal(evidenceRaw, &evidence) != nil || json.Unmarshal(receiptsRaw, &receipts) != nil || evidence.ControllerResultSHA256 != observation.ControllerResultSHA256 || evidence.RawProviderReceiptsSHA256 != observation.RawProviderReceiptsSHA256 || evidence.ClaimID != observation.ClaimID || evidence.ConfigurationID != observation.ConfigurationID || evidence.CaseID != observation.CaseID || len(receipts) != evidence.AvailableProviderReceipts || len(receipts) != len(evidence.ProviderReceiptHashes) {
		return fmt.Errorf("paid failure seal identity drifted")
	}
	for index, receipt := range receipts {
		hash := sha256.Sum256(receipt.Raw)
		if receipt.SHA256 != hex.EncodeToString(hash[:]) || receipt.SHA256 != evidence.ProviderReceiptHashes[index] {
			return fmt.Errorf("paid failure raw receipt %d drifted", index+1)
		}
	}
	return nil
}

func validFailureAccountingStatus(value string, allowed ...string) bool {
	return contains(allowed, value)
}

func recomputeKnownFailureUsage(raw []byte) (realisticProviderAccounting, bool, bool) {
	accounting := realisticProviderAccounting{}
	var envelope realisticRawProviderEnvelope
	if json.Unmarshal(raw, &envelope) != nil || envelope.JSONL == "" {
		return accounting, false, false
	}
	if len(envelope.Export) != 0 {
		if result, _, err := ParseOpenCodeRunEnvelope([]byte(envelope.JSONL), envelope.Export); err == nil {
			if result.Usage.InputTokens != nil && result.Usage.OutputTokens != nil && result.Usage.ReasoningTokens != nil {
				accounting.InputTokens, accounting.OutputTokens, accounting.ReasoningTokens = *result.Usage.InputTokens, *result.Usage.OutputTokens, *result.Usage.ReasoningTokens
			}
			if result.Usage.ProviderReportedCostMicros != nil {
				accounting.CostMicros = *result.Usage.ProviderReportedCostMicros
			}
			return accounting, result.Usage.InputTokens != nil && result.Usage.OutputTokens != nil && result.Usage.ReasoningTokens != nil, result.Usage.ProviderReportedCostMicros != nil
		}
	}
	tokensKnown, costKnown := false, false
	scanner := bufio.NewScanner(bytes.NewBufferString(envelope.JSONL))
	for scanner.Scan() {
		var event struct {
			Type string `json:"type"`
			Part struct {
				Cost   json.Number `json:"cost"`
				Tokens struct {
					Input     int64 `json:"input"`
					Output    int64 `json:"output"`
					Reasoning int64 `json:"reasoning"`
					Cache     struct {
						Read  int64 `json:"read"`
						Write int64 `json:"write"`
					} `json:"cache"`
				} `json:"tokens"`
			} `json:"part"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Type != "step_finish" || event.Part.Tokens.Input < 0 || event.Part.Tokens.Output < 0 || event.Part.Tokens.Reasoning < 0 || event.Part.Tokens.Cache.Read < 0 || event.Part.Tokens.Cache.Write < 0 {
			continue
		}
		accounting.InputTokens += event.Part.Tokens.Input + event.Part.Tokens.Cache.Read + event.Part.Tokens.Cache.Write
		accounting.OutputTokens += event.Part.Tokens.Output
		accounting.ReasoningTokens += event.Part.Tokens.Reasoning
		tokensKnown = true
		if micros, ok := failureCostMicros(event.Part.Cost.String()); ok {
			accounting.CostMicros += micros
			costKnown = true
		}
	}
	return accounting, tokensKnown, costKnown
}

func failureCostMicros(value string) (int64, bool) {
	amount := new(big.Rat)
	if _, ok := amount.SetString(value); !ok || amount.Sign() < 0 {
		return 0, false
	}
	amount.Mul(amount, big.NewRat(1_000_000, 1))
	quotient := new(big.Int).Quo(amount.Num(), amount.Denom())
	if new(big.Int).Mod(amount.Num(), amount.Denom()).Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, false
	}
	return quotient.Int64(), true
}
