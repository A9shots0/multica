package agentqualification

import (
	"encoding/json"
	"errors"
	"github.com/multica-ai/multica/server/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR02RepeatAdmissionPinsExactTupleBudgetAndStopPolicy(t *testing.T) {
	admission, hash, err := CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	if !validSHA256(hash) || !admission.ProviderCallsAllowed || admission.Status != "frozen_reviewed" {
		t.Fatalf("pre-review provider lock drifted: admission=%+v hash=%q", admission, hash)
	}
	if admission.ConfigurationID != "realistic.claudesonnet5.medium.anchor.r02.repeat1.v11" || admission.Model != "openrouter/anthropic/claude-sonnet-5" || admission.Effort != "medium" || admission.ServingProvider != "anthropic" {
		t.Fatalf("exact candidate tuple drifted: %+v", admission)
	}
	if admission.MaxActions != 18 || admission.MaxTotalTokens != 75_000 || admission.MaxGeneratedTokens != 20_000 || admission.MaxSpendMicros != 350_000 || admission.Retries != 0 || admission.ProviderFallbacks || admission.OwnerInterventions != 0 {
		t.Fatalf("bounded run envelope drifted: %+v", admission)
	}
}

func TestR02RepeatStopClassificationIsGenerationOwned(t *testing.T) {
	tests := []struct {
		name, terminal, failure, message, outcome, class string
	}{
		{"competence", "failed", "competence_failure", "", "candidate_fail", "competence_failure"},
		{"protocol", "failed", "response_or_tool_protocol_failure", "", "candidate_fail", "response_or_tool_protocol_failure"},
		{"transport", "invalid", "provider_transport_failure", "timeout", "invalid", "provider_transport_imperfection"},
		{"harness", "invalid", "test_invalidity", "bad local config", "invalid", "non_safety_harness_imperfection"},
		{"credential", "invalid", "test_invalidity", "OpenRouter API key credential missing", "global_halt", "credential_failure"},
		{"spend", "invalid", "budget_breach", "", "global_halt", "uncontrolled_spending"},
		{"evidence", "invalid", "evidence_integrity_failure", "", "global_halt", "evidence_corruption"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.message != "" {
				err = errors.New(tt.message)
			}
			outcome, class := ClassifyR02RepeatController(RealisticControllerResult{TerminalState: tt.terminal, FailureClass: tt.failure}, err)
			if outcome != tt.outcome || class != tt.class {
				t.Fatalf("got %s/%s, want %s/%s", outcome, class, tt.outcome, tt.class)
			}
		})
	}
}

func TestR02RepeatRunRecordEnforcesAllGenerationCaps(t *testing.T) {
	admission, hash, err := CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	controller := RealisticControllerResult{ProviderCalls: 1, InputTokens: 55_000, OutputTokens: 20_001, CostMicros: 1, ToolEvents: make([]RealisticToolEvent, 18)}
	record := R02RepeatRunRecord{
		SchemaVersion: R02RepeatRunRecordSchemaVersion, AdmissionID: admission.AdmissionID, AdmissionHash: hash,
		Outcome: "candidate_fail", FailureClass: "competence_failure",
		Observation: RealisticObservation{ConfigurationID: admission.ConfigurationID, CaseID: admission.CaseID, Outcome: "candidate_fail", FailureClass: "competence_failure", ProviderCalls: 1, TotalTokens: 75_001, CostMicros: 1},
	}
	if err := ValidateR02RepeatRunRecord(admission, record, controller); err == nil || !strings.Contains(err.Error(), "budget breach") {
		t.Fatalf("generated/total cap mutation was accepted: %v", err)
	}
	record.Outcome, record.FailureClass = "global_halt", "uncontrolled_spending"
	record.Observation.Outcome, record.Observation.FailureClass = record.Outcome, record.FailureClass
	if err := ValidateR02RepeatRunRecord(admission, record, controller); err != nil {
		t.Fatalf("correctly classified cap breach was rejected: %v", err)
	}
	controller.OutputTokens, controller.InputTokens, controller.CostMicros = 1, 1, 350_001
	record.Observation.TotalTokens, record.Observation.CostMicros = 2, 350_001
	if err := ValidateR02RepeatRunRecord(admission, record, controller); err != nil {
		t.Fatalf("correctly classified spend breach was rejected: %v", err)
	}
	controller.CostMicros, record.Observation.CostMicros = 1, 1
	controller.ToolEvents = make([]RealisticToolEvent, 19)
	if err := ValidateR02RepeatRunRecord(admission, record, controller); err != nil {
		t.Fatalf("correctly classified action breach was rejected: %v", err)
	}
}

func TestR02RepeatAdmissionRejectsBroadHaltAndEvidenceDrift(t *testing.T) {
	admission, _, err := CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	admission.GlobalHaltReasons = append(admission.GlobalHaltReasons, "competence_failure")
	if err := ValidateR02RepeatAdmissionContract(admission); err == nil || !strings.Contains(err.Error(), "stop policy") {
		t.Fatalf("candidate failure was allowed to halt the batch: %v", err)
	}
	admission, _, _ = CurrentR02RepeatAdmission()
	admission.AcceptedEvidence[0].ExecutionReceiptSHA256 = strings.Repeat("a", 64)
	if err := ValidateR02RepeatAdmissionContract(admission); err != nil {
		t.Fatalf("contract-only validation should permit a syntactically valid hash: %v", err)
	}
}

func TestR02RepeatBoundedMatrixTightensSpendWithoutChangingFrozenV11(t *testing.T) {
	admission, _, err := CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	bounded, err := R02RepeatBoundedMatrix(admission)
	if err != nil {
		t.Fatal(err)
	}
	original, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v11")
	if err != nil {
		t.Fatal(err)
	}
	if bounded.MaxIncrementalSpendMicros != 350_000 || original.MaxIncrementalSpendMicros != 5_000_000 {
		t.Fatalf("generation cap did not tighten independently: bounded=%d frozen=%d", bounded.MaxIncrementalSpendMicros, original.MaxIncrementalSpendMicros)
	}
	if bounded.Candidates[0] != original.Candidates[0] || bounded.ExecutionOrder[0] != "R02" {
		t.Fatal("generation envelope changed the frozen candidate or case")
	}
}

func TestR02RepeatProviderExecutionActivatedAfterReviewAndCorrection(t *testing.T) {
	admission, _, err := CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	if !admission.ProviderCallsAllowed || admission.Status != "frozen_reviewed" {
		t.Fatal("provider calls were not enabled after independent review and consolidated correction")
	}
}

func TestR02RepeatAcceptedEvidenceRecomputesRouteAccountingAndR03Semantics(t *testing.T) {
	repoRoot := testutil.QualificationArchiveRoot(t, "r02-admission-v1")
	admission, _, err := CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyR02RepeatAdmissionEvidence(repoRoot, admission); err != nil {
		t.Fatal(err)
	}
	var r01, r03 R02RepeatAcceptedEvidence
	for _, item := range admission.AcceptedEvidence {
		switch item.Label {
		case "accepted_r01":
			r01 = item
		case "accepted_r03":
			r03 = item
		}
	}
	var receipt RealisticExecutionReceipt
	if err := readR02RepeatJSON(repoRoot, r01.ExecutionReceiptPath, &receipt); err != nil {
		t.Fatal(err)
	}
	evidenceRoot := filepath.Dir(filepath.Join(repoRoot, filepath.FromSlash(r01.ExecutionReceiptPath)))
	if _, err := verifyR02RepeatProviderProvenance(evidenceRoot, receipt); err != nil {
		t.Fatal(err)
	}
	mutatedAccounting := receipt
	mutatedAccounting.CostMicros++
	if _, err := verifyR02RepeatProviderProvenance(evidenceRoot, mutatedAccounting); err == nil || !strings.Contains(err.Error(), "accounting") {
		t.Fatalf("accounting mutation was accepted: %v", err)
	}
	mutatedHash := receipt
	mutatedHash.ProviderReceiptSHA256 = strings.Repeat("a", 64)
	if _, err := verifyR02RepeatProviderProvenance(evidenceRoot, mutatedHash); err == nil || !strings.Contains(err.Error(), "hash-bound") {
		t.Fatalf("provider hash mutation was accepted: %v", err)
	}
	assertR02RepeatRouteMutationRejected(t, evidenceRoot, receipt, func(envelope map[string]any) {
		export := envelope["export"].(map[string]any)
		info := export["info"].(map[string]any)
		model := info["model"].(map[string]any)
		model["id"] = "anthropic/other-model"
	})
	assertR02RepeatRouteMutationRejected(t, evidenceRoot, receipt, func(envelope map[string]any) {
		routing := envelope["routing"].(map[string]any)
		routing["requested_upstream_provider"] = "not-anthropic"
	})

	var r03Receipt RealisticExecutionReceipt
	if err := readR02RepeatJSON(repoRoot, r03.ExecutionReceiptPath, &r03Receipt); err != nil {
		t.Fatal(err)
	}
	r03Root := filepath.Dir(filepath.Join(repoRoot, filepath.FromSlash(r03.ExecutionReceiptPath)))
	r03Controller, err := verifyR02RepeatProviderProvenance(r03Root, r03Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if reasons := evaluateRealisticDeclarations("R03", r03Receipt, r03Controller.FinalSummary); len(reasons) != 0 {
		t.Fatalf("corrected R03 verifier did not accept immutable evidence: %v", reasons)
	}
	if reasons := evaluateRealisticDeclarations("R03", r03Receipt, "The rollback fixed and completed the requested outcome."); !contains(reasons, "rollback_falsely_claimed_complete") {
		t.Fatalf("R03 false-completion mutation escaped corrected verifier: %v", reasons)
	}
}

func assertR02RepeatRouteMutationRejected(t *testing.T, sourceRoot string, receipt RealisticExecutionReceipt, mutate func(map[string]any)) {
	t.Helper()
	temp := t.TempDir()
	controllerRaw, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(receipt.ControllerResultArtifactPath)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temp, receipt.ControllerResultArtifactPath), controllerRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	receiptsRaw, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(receipt.ProviderReceiptArtifactPath)))
	if err != nil {
		t.Fatal(err)
	}
	var sealed []sealedRealisticProviderReceipt
	if err := json.Unmarshal(receiptsRaw, &sealed); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(sealed[0].Raw, &envelope); err != nil {
		t.Fatal(err)
	}
	mutate(envelope)
	sealed[0].Raw, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	sealed[0].SHA256 = digestR02Repeat(sealed[0].Raw)
	receipt.ProviderReceiptHashes[0] = sealed[0].SHA256
	mutatedRaw, err := json.MarshalIndent(sealed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	receipt.ProviderReceiptSHA256 = digestR02Repeat(mutatedRaw)
	if err := os.WriteFile(filepath.Join(temp, receipt.ProviderReceiptArtifactPath), mutatedRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyR02RepeatProviderProvenance(temp, receipt); err == nil || !strings.Contains(err.Error(), "route") {
		t.Fatalf("route mutation was accepted: %v", err)
	}
}
