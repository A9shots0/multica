package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func chargedPartialFailureFixture(t *testing.T) (RealisticObservation, RealisticControllerResult) {
	t.Helper()
	jsonl := "{\"type\":\"step_finish\",\"part\":{\"cost\":0.0019,\"tokens\":{\"input\":12,\"output\":4,\"reasoning\":2,\"cache\":{\"read\":3,\"write\":1}}}}"
	raw, err := json.Marshal(map[string]any{"jsonl": jsonl, "export": map[string]any{"partial": true}, "transport_error": "connection reset"})
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	controller := RealisticControllerResult{
		CaseID: "R01", RunFamilyID: "failure-family", TerminalState: "invalid", FailureClass: "provider_transport_failure", ProviderCalls: 1,
		InputTokens: 16, OutputTokens: 4, ReasoningTokens: 2, CostMicros: 1900,
		ProviderReceipts: [][]byte{raw}, ProviderReceiptHashes: []string{hex.EncodeToString(hash[:])}, AmbiguousProviderCalls: 1,
		ProviderAccountingStatus: "partial", ProviderTokenAccountingStatus: "reported", ProviderCostAccountingStatus: "reported",
	}
	observation := RealisticObservation{ClaimID: "failure-family", ConfigurationID: "candidate-1", CaseID: "R01", Outcome: "global_halt", FailureClass: "provider_transport_failure", ProviderCalls: 1, TotalTokens: 22, CostMicros: 1900}
	return observation, controller
}

func TestSealRealisticPaidFailurePreservesChargedPartialReceipt(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	observation, controller := chargedPartialFailureFixture(t)
	sealed, path, err := SealRealisticPaidFailure(root, observation, controller)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyRealisticPaidFailureSeal(path, sealed); err != nil {
		t.Fatal(err)
	}
	reused, reusedPath, err := SealRealisticPaidFailure(root, observation, controller)
	if err != nil || reusedPath != path || reused != sealed {
		t.Fatalf("exact write-once failure seal was not reusable after interruption: path=%s err=%v", reusedPath, err)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "failure-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var evidence RealisticFailureEvidence
	if json.Unmarshal(raw, &evidence) != nil || evidence.KnownInputTokens != 16 || evidence.KnownOutputTokens != 4 || evidence.KnownReasoningTokens != 2 || evidence.KnownCostMicros != 1900 || evidence.ReceiptsWithKnownTokens != 1 || evidence.ReceiptsWithKnownCost != 1 || evidence.ProviderAccountingStatus != "partial" || evidence.MissingProviderReceipts != 0 {
		t.Fatalf("charged partial accounting was not sealed truthfully: %+v", evidence)
	}
}

func TestVerifyRealisticPaidFailureRejectsPartialReceiptTamper(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	observation, controller := chargedPartialFailureFixture(t)
	sealed, path, err := SealRealisticPaidFailure(root, observation, controller)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(filepath.Dir(path), "provider-receipts.json")
	if err := os.WriteFile(receiptPath, []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRealisticPaidFailureSeal(path, sealed); err == nil {
		t.Fatal("tampered partial transport receipt was accepted")
	}
}

func TestSealRealisticPaidFailurePropagatesEvidenceWriteFailure(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	root := filepath.Join(parent, "not-a-directory")
	if err := os.WriteFile(root, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	observation, controller := chargedPartialFailureFixture(t)
	if _, _, err := SealRealisticPaidFailure(root, observation, controller); err == nil {
		t.Fatal("failure evidence write error was ignored")
	}
}

func TestRecordRealisticObservationRejectsUnsealedPaidProtocolFailure(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	state := NewRealisticCampaignState()
	candidateID := matrix.Candidates[0].ConfigurationID
	state, _ = ClaimRealisticAssignment(state, matrix, candidateID, "R01", "claim")
	observation := RealisticObservation{ClaimID: "claim", ConfigurationID: candidateID, CaseID: "R01", Outcome: "candidate_fail", FailureClass: "response_or_tool_protocol_failure", ProviderCalls: 1}
	if _, err := RecordRealisticObservation(state, matrix, observation); err == nil {
		t.Fatal("unsealed paid protocol failure was recorded")
	}
}

func TestSealRealisticPaidProtocolFailureCanBeRecordedOnlyAfterSeal(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	matrix, _ := CurrentRealisticRunMatrix()
	candidateID := matrix.Candidates[0].ConfigurationID
	controller := RealisticControllerResult{CaseID: "R01", RunFamilyID: "protocol-family", TerminalState: "failed", FailureClass: "response_or_tool_protocol_failure"}
	applySealableProviderFixture(&controller)
	observation := RealisticObservation{ClaimID: "protocol-family", ConfigurationID: candidateID, CaseID: "R01", Outcome: "candidate_fail", FailureClass: controller.FailureClass, ProviderCalls: controller.ProviderCalls, TotalTokens: controller.InputTokens + controller.OutputTokens + controller.ReasoningTokens, CostMicros: controller.CostMicros}
	sealed, path, err := SealRealisticPaidFailure(root, observation, controller)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyRealisticPaidFailureSeal(path, sealed); err != nil {
		t.Fatal(err)
	}
	state := NewRealisticCampaignState()
	state, err = ClaimRealisticAssignment(state, matrix, candidateID, "R01", observation.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	state, err = RecordRealisticObservation(state, matrix, sealed)
	if err != nil || len(state.Observations) != 1 || state.Observations[0].FailureEvidenceSHA256 == "" {
		t.Fatalf("sealed paid protocol failure was not recorded: state=%+v err=%v", state, err)
	}
}
