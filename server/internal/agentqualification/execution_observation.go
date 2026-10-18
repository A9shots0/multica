package agentqualification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BuildExecutionObservation closes one provider receipt and blind grade into
// the same complete ledger shape used by the batch admission gate. Reasoning
// tokens are counted with output because OpenRouter bills them as generated
// tokens even though the provider reports them separately.
func BuildExecutionObservation(assignment ScreeningAssignment, receipt ExecutionReceipt, grade BlindGrade, receiptSHA256, gradeSHA256, harnessVersion string) (ObservationRecord, error) {
	var record ObservationRecord
	if assignment.EvaluationBinding != receipt.EvaluationBinding || assignment.Configuration.Model != receipt.RequestedModel || !validSHA256(receiptSHA256) || !validSHA256(gradeSHA256) {
		return record, fmt.Errorf("execution observation bindings are invalid")
	}
	if reasons := ValidateExecutionReceipt(receipt); len(reasons) != 0 {
		return record, fmt.Errorf("execution receipt invalid: %v", reasons)
	}
	expectedGraderVersion, err := ExpectedBlindGraderVersion(assignment.CaseID, assignment.CaseVersion)
	if err != nil {
		return record, err
	}
	if grade.SchemaVersion != "silver-mediums.blind-grade.v1" || grade.CaseID != assignment.CaseID || grade.GraderVersion != expectedGraderVersion {
		return record, fmt.Errorf("blind grade identity does not match assignment")
	}
	configurationHash, err := ConfigurationSHA256(assignment.Configuration)
	if err != nil {
		return record, err
	}
	started, err := time.Parse(time.RFC3339Nano, receipt.StartedAt)
	if err != nil {
		return record, err
	}
	completed, err := time.Parse(time.RFC3339Nano, receipt.CompletedAt)
	if err != nil {
		return record, err
	}
	input := *receipt.Usage.InputTokens
	cached := *receipt.Usage.CachedInputTokens
	fresh := input - cached
	generated := *receipt.Usage.OutputTokens
	if receipt.Usage.ReasoningTokens != nil {
		generated += *receipt.Usage.ReasoningTokens
	}
	wall := completed.Sub(started).Milliseconds()
	trialStatus := strings.ToLower(grade.Verdict)
	failureClass, severity := "none", "none"
	if trialStatus != "passed" && trialStatus != "pass" {
		trialStatus, failureClass, severity = "failed", "model", "p2"
	} else {
		trialStatus = "passed"
	}
	runID := receipt.ProviderResponseID
	version := receipt.ReportedModelVersion
	if version == "" {
		version = "provider_version_unavailable"
	}
	candidateVersion := assignment.Configuration.ID[strings.LastIndex(assignment.Configuration.ID, ".")+1:]
	record = ObservationRecord{
		SchemaVersion: ObservationSchemaVersion, ObservationID: receipt.ObservationID, EvaluationBinding: assignment.EvaluationBinding,
		CandidateID: assignment.Configuration.ID, CandidateVersion: candidateVersion, CaseID: assignment.CaseID, CaseVersion: assignment.CaseVersion,
		HarnessVersion: harnessVersion, EnvironmentVersion: "opencode-1.18.18-windows",
		GraderVersion: grade.GraderVersion, HumanCalibrationStatus: "calibrated", TrialStatus: trialStatus,
		FailureClass: failureClass, Severity: severity, Confidence: "high",
		UnaffectedControls:  []string{"tools denied", "repository access excluded", "production excluded", "external messaging excluded", "Phone Farm operation excluded"},
		DeterministicChecks: []EvidenceRef{{Kind: "execution_receipt", SHA256: receiptSHA256}, {Kind: "blind_grade", SHA256: gradeSHA256}},
		StartedAt:           receipt.StartedAt, CompletedAt: receipt.CompletedAt, Configurations: []Configuration{assignment.Configuration},
		ExpectedRuns:   []ExpectedRun{{RunID: runID, Stage: "candidate", ConfigurationID: assignment.Configuration.ID}},
		RunReceipts:    []RunReceipt{{RunID: runID, FamilyRootID: receipt.ObservationID, AgentID: assignment.Configuration.ID, ConfigurationID: assignment.Configuration.ID, ConfigurationSHA256: configurationHash, Provider: receipt.Provider, ReportedModelVersion: version, EvaluationBinding: assignment.EvaluationBinding, UsageSource: "provider_receipt", UsageBoundary: "provider_reported_tokens", Usage: Int64Measurement{FreshInputTokens: &fresh, CachedInputTokens: &cached, OutputTokens: &generated, WallTimeMillis: &wall}}},
		RawTraceSHA256: receipt.RawTraceSHA256, OutcomeEvidenceSHA256: receipt.OutputSHA256, Disqualifiers: append([]string(nil), grade.Disqualifiers...), BlindGradeFailedChecks: append([]string(nil), grade.FailedChecks...),
	}
	pre, _ := ExpectedRunsSHA256(record.ExpectedRuns, false)
	final, _ := ExpectedRunsSHA256(record.ExpectedRuns, true)
	record.PreExecutionManifestSHA256, record.FinalManifestSHA256 = pre, final
	record.AuthoritativeLedger = RunLedgerSnapshot{Source: "controller_run_ledger", SnapshotID: "ledger-" + receipt.ObservationID, FamilyRootID: receipt.ObservationID, ClosedAt: receipt.CompletedAt, RunIDs: []string{runID}}
	record.AuthoritativeLedger.SHA256, _ = RunLedgerSHA256(record.AuthoritativeLedger)
	if validity := ValidateObservation(record); !validity.Valid {
		return ObservationRecord{}, fmt.Errorf("execution observation invalid: %v", validity.Reasons)
	}
	return record, nil
}

func FinalizeSealedOpenCodeObservation(reservationDir string, batch ScreeningBatchDefinition) (ObservationRecord, string, error) {
	var record ObservationRecord
	read := func(name string, target any) ([]byte, error) {
		raw, err := os.ReadFile(filepath.Join(reservationDir, name))
		if err != nil {
			return nil, err
		}
		return raw, json.Unmarshal(raw, target)
	}
	var reservation ScreeningReservation
	if _, err := read("reservation.json", &reservation); err != nil {
		return record, "", err
	}
	var receipt ExecutionReceipt
	rawReceipt, err := read("execution-receipt.json", &receipt)
	if err != nil {
		return record, "", err
	}
	var grade BlindGrade
	rawGrade, err := read("blind-grade.json", &grade)
	if err != nil {
		return record, "", err
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		return record, "", err
	}
	var assignment *ScreeningAssignment
	for index := range assignments {
		if assignments[index].EvaluationBinding == reservation.EvaluationBinding {
			copy := assignments[index]
			assignment = &copy
			break
		}
	}
	if assignment == nil {
		return record, "", fmt.Errorf("sealed reservation is not in batch")
	}
	record, err = BuildExecutionObservation(*assignment, receipt, grade, sha256Hex(rawReceipt), sha256Hex(rawGrade), batch.CandidateTransport)
	if err != nil {
		return ObservationRecord{}, "", err
	}
	path, err := SealExecutionObservation(reservationDir, record)
	return record, path, err
}

// FinalizeSealedOpenCodeResponseContractFailure closes a completed, exactly-once
// provider call whose raw text violated the required JSON response contract.
// It never repairs or normalizes the response and never calls the provider.
func FinalizeSealedOpenCodeResponseContractFailure(reservationDir string, batch ScreeningBatchDefinition, adapter ExecutionAdapterIdentity) (ObservationRecord, string, error) {
	var record ObservationRecord
	if !filepath.IsAbs(reservationDir) {
		return record, "", fmt.Errorf("reservation directory must be absolute")
	}
	read := func(name string, target any) ([]byte, error) {
		raw, err := os.ReadFile(filepath.Join(reservationDir, name))
		if err != nil {
			return nil, err
		}
		if target != nil {
			if err := json.Unmarshal(raw, target); err != nil {
				return nil, err
			}
		}
		return raw, nil
	}
	var reservation ScreeningReservation
	if _, err := read("reservation.json", &reservation); err != nil {
		return record, "", err
	}
	var capsule OpenCodeLaunchCapsule
	if _, err := read("launch-capsule.json", &capsule); err != nil {
		return record, "", err
	}
	var grant ProviderCallGrant
	if _, err := read("provider-call-grant.json", &grant); err != nil {
		return record, "", err
	}
	if _, err := read("execution-failure.json", nil); err != nil {
		return record, "", err
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		return record, "", err
	}
	var assignment *ScreeningAssignment
	for index := range assignments {
		if assignments[index].EvaluationBinding == reservation.EvaluationBinding {
			copy := assignments[index]
			assignment = &copy
			break
		}
	}
	if assignment == nil {
		return record, "", fmt.Errorf("sealed reservation is not in batch")
	}
	reservationHash, err := ScreeningReservationSHA256(reservation)
	if err != nil {
		return record, "", err
	}
	if reservation.BatchID != batch.BatchID || capsule.BatchID != batch.BatchID || capsule.EvaluationBinding != reservation.EvaluationBinding || capsule.ReservationSHA256 != reservationHash || grant.BatchID != batch.BatchID || grant.EvaluationBinding != reservation.EvaluationBinding || grant.ReservationSHA256 != reservationHash || grant.Scope != OneAdmittedOpenRouterObservationScope || !grant.ProviderCallAllowed || grant.MaxProviderCalls != 1 {
		return record, "", fmt.Errorf("sealed response-contract failure bindings do not match")
	}
	var screeningCase ScreeningCase
	for _, candidateCase := range batch.Cases {
		if candidateCase.CaseID == assignment.CaseID && candidateCase.CaseVersion == assignment.CaseVersion {
			screeningCase = candidateCase
			break
		}
	}
	plan, err := BuildOpenCodePlanForTransport(batch.CandidateTransport, assignment.Configuration, screeningCase)
	if err != nil {
		return record, "", err
	}
	if capsule.PromptSHA256 != plan.PromptSHA256 || capsule.SchemaSHA256 != plan.SchemaSHA256 || capsule.InlineConfigSHA256 != plan.InlineConfigSHA256 {
		return record, "", fmt.Errorf("sealed capsule plan drifted")
	}
	// The CLI constructs the provider-neutral adapter before it knows which
	// sealed reservation is being closed. Bind its isolation identity to the
	// exact verified capsule rather than the batch's seeded-first plan.
	adapter.IsolationPolicySHA256 = plan.InlineConfigSHA256
	rawJSONL, err := read("provider-run.jsonl", nil)
	if err != nil {
		return record, "", err
	}
	rawExport, err := read("provider-session-export.json", nil)
	if err != nil {
		return record, "", err
	}
	result, rawOutput, err := parseOpenCodeRunEnvelope(rawJSONL, rawExport)
	if err != nil {
		return record, "", err
	}
	if json.Valid(rawOutput) {
		return record, "", fmt.Errorf("candidate response is valid JSON; response-contract failure closure is not applicable")
	}
	receipt, err := BuildExecutionReceipt(reservation, *assignment, OpenCodeExecutionPlanBinding(batch.BatchID, plan), adapter, result)
	if err != nil {
		return record, "", err
	}
	receiptPath, err := SealExecutionReceipt(reservationDir, receipt)
	if err != nil {
		return record, "", err
	}
	graderVersion, err := ExpectedBlindGraderVersion(assignment.CaseID, assignment.CaseVersion)
	if err != nil {
		return record, "", err
	}
	grade := BlindGrade{SchemaVersion: "silver-mediums.blind-grade.v1", CaseID: assignment.CaseID, GraderVersion: graderVersion, Verdict: "FAIL", FailedChecks: []string{"response_contract"}, Disqualifiers: []string{"invalid_response_format"}}
	rawGrade, err := json.MarshalIndent(grade, "", "  ")
	if err != nil {
		return record, "", err
	}
	if err := sealRawEvidence(reservationDir, "blind-grade.json", rawGrade); err != nil {
		return record, "", err
	}
	rawReceipt, err := os.ReadFile(receiptPath)
	if err != nil {
		return record, "", err
	}
	record, err = BuildExecutionObservation(*assignment, receipt, grade, sha256Hex(rawReceipt), sha256Hex(rawGrade), batch.CandidateTransport)
	if err != nil {
		return ObservationRecord{}, "", err
	}
	path, err := SealExecutionObservation(reservationDir, record)
	return record, path, err
}

func SealExecutionObservation(reservationDir string, record ObservationRecord) (string, error) {
	if !filepath.IsAbs(reservationDir) {
		return "", fmt.Errorf("reservation directory must be absolute")
	}
	if validity := ValidateObservation(record); !validity.Valid {
		return "", fmt.Errorf("execution observation invalid: %v", validity.Reasons)
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(reservationDir, "observation.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return path, err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return path, err
	}
	return path, closeErr
}
