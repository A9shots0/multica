package agentqualification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type OpenCodeProviderRunner func(plan OpenCodeScreeningPlan) (rawJSONL, rawExport []byte, err error)

const ProviderCallGrantSchemaVersion = "silver-mediums.provider-call-grant.v1"

const OneAdmittedOpenRouterObservationScope = "one_admitted_openrouter_observation"

// ProviderCallGrantInput is the caller's narrow authorization to cross the
// provider boundary. The executor seals it against the exact reservation; a
// dry-run capsule never silently becomes executable.
type ProviderCallGrantInput struct {
	BatchID          string `json:"batch_id"`
	GrantedBy        string `json:"granted_by"`
	Scope            string `json:"scope"`
	MaxProviderCalls int    `json:"max_provider_calls"`
	GrantedAt        string `json:"granted_at"`
}

type ProviderCallGrant struct {
	SchemaVersion       string `json:"schema_version"`
	BatchID             string `json:"batch_id"`
	EvaluationBinding   string `json:"evaluation_binding"`
	ReservationSHA256   string `json:"reservation_sha256"`
	GrantedBy           string `json:"granted_by"`
	Scope               string `json:"scope"`
	MaxProviderCalls    int    `json:"max_provider_calls"`
	GrantedAt           string `json:"granted_at"`
	ProviderCallAllowed bool   `json:"provider_call_allowed"`
}

type CompletedOpenCodeLaunch struct {
	Prepared             PreparedScreeningLaunch `json:"prepared"`
	Reservation          ScreeningReservation    `json:"reservation"`
	Capsule              OpenCodeLaunchCapsule   `json:"capsule"`
	Grant                ProviderCallGrant       `json:"grant"`
	Receipt              ExecutionReceipt        `json:"receipt"`
	CandidateResponse    json.RawMessage         `json:"candidate_response"`
	ReservationDir       string                  `json:"reservation_dir"`
	ExecutionReceiptPath string                  `json:"execution_receipt_path"`
}

// ReconcileOpenCodeReservedRun normalizes already-sealed provider evidence
// after a deterministic parser correction. It cannot call a provider and it
// refuses evidence that is not bound to the immutable reservation and batch.
func ReconcileOpenCodeReservedRun(reservationDir string, batch ScreeningBatchDefinition, adapter ExecutionAdapterIdentity) (CompletedOpenCodeLaunch, error) {
	var completed CompletedOpenCodeLaunch
	if !filepath.IsAbs(reservationDir) {
		return completed, fmt.Errorf("reservation directory must be absolute")
	}
	readJSON := func(name string, target any) error {
		raw, err := os.ReadFile(filepath.Join(reservationDir, name))
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, target)
	}
	var reservation ScreeningReservation
	var capsule OpenCodeLaunchCapsule
	var grant ProviderCallGrant
	if err := readJSON("reservation.json", &reservation); err != nil {
		return completed, err
	}
	if err := readJSON("launch-capsule.json", &capsule); err != nil {
		return completed, err
	}
	if err := readJSON("provider-call-grant.json", &grant); err != nil {
		return completed, err
	}
	reservationHash, err := ScreeningReservationSHA256(reservation)
	if err != nil {
		return completed, err
	}
	if reservation.BatchID != batch.BatchID || capsule.BatchID != batch.BatchID || capsule.EvaluationBinding != reservation.EvaluationBinding || capsule.ReservationSHA256 != reservationHash || grant.BatchID != batch.BatchID || grant.EvaluationBinding != reservation.EvaluationBinding || grant.ReservationSHA256 != reservationHash || !grant.ProviderCallAllowed || grant.MaxProviderCalls != 1 {
		return completed, fmt.Errorf("sealed execution bindings do not match")
	}
	if len(batch.APIBudgets) != 1 || adapter.Provider != batch.APIBudgets[0].Provider || adapter.AccountLane != batch.APIBudgets[0].ProjectLane || adapter.BillingBoundary != "openrouter_metered" || adapter.ToolPolicy != "none" {
		return completed, fmt.Errorf("OpenCode adapter does not match the admitted metered lane")
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		return completed, err
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
		return completed, fmt.Errorf("reservation is not a batch assignment")
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
		return completed, err
	}
	if capsule.PromptSHA256 != plan.PromptSHA256 || capsule.SchemaSHA256 != plan.SchemaSHA256 || capsule.InlineConfigSHA256 != plan.InlineConfigSHA256 {
		return completed, fmt.Errorf("sealed capsule plan drifted")
	}
	rawJSONL, err := os.ReadFile(filepath.Join(reservationDir, "provider-run.jsonl"))
	if err != nil {
		return completed, err
	}
	rawExport, err := os.ReadFile(filepath.Join(reservationDir, "provider-session-export.json"))
	if err != nil {
		return completed, err
	}
	result, response, err := ParseOpenCodeRunArtifact(rawJSONL, rawExport)
	if err != nil {
		return completed, err
	}
	receipt, err := BuildExecutionReceipt(reservation, *assignment, OpenCodeExecutionPlanBinding(batch.BatchID, plan), adapter, result)
	if err != nil {
		return completed, err
	}
	receiptPath, err := SealExecutionReceipt(reservationDir, receipt)
	if err != nil {
		return completed, err
	}
	rawReceipt, err := os.ReadFile(receiptPath)
	if err != nil {
		return completed, err
	}
	marker, _ := json.MarshalIndent(map[string]any{"state": "reconciled", "reason": "wrapper_interrupted_after_raw_evidence_seal", "provider_called": false, "execution_receipt_sha256": sha256Hex(rawReceipt)}, "", "  ")
	if err := sealRawEvidence(reservationDir, "execution-reconciliation.json", marker); err != nil {
		return completed, err
	}
	completed = CompletedOpenCodeLaunch{Reservation: reservation, Capsule: capsule, Grant: grant, Receipt: receipt, CandidateResponse: response, ReservationDir: reservationDir, ExecutionReceiptPath: receiptPath}
	return completed, nil
}

// GradeSealedOpenCodeEvidence applies the candidate-blind deterministic grader
// to the already-sealed response and writes the grade exactly once.
func GradeSealedOpenCodeEvidence(reservationDir, caseID string) (BlindGrade, string, error) {
	return GradeSealedOpenCodeEvidenceVersion(reservationDir, caseID, "")
}

func GradeSealedOpenCodeEvidenceVersion(reservationDir, caseID, caseVersion string) (BlindGrade, string, error) {
	var grade BlindGrade
	if !filepath.IsAbs(reservationDir) {
		return grade, "", fmt.Errorf("reservation directory must be absolute")
	}
	rawJSONL, err := os.ReadFile(filepath.Join(reservationDir, "provider-run.jsonl"))
	if err != nil {
		return grade, "", err
	}
	rawExport, err := os.ReadFile(filepath.Join(reservationDir, "provider-session-export.json"))
	if err != nil {
		return grade, "", err
	}
	_, response, err := ParseOpenCodeRunArtifact(rawJSONL, rawExport)
	if err != nil {
		return grade, "", err
	}
	if caseVersion == "" {
		grade, err = GradeBlind(caseID, response)
	} else {
		grade, err = GradeBlindVersion(caseID, caseVersion, response)
	}
	if err != nil {
		return grade, "", err
	}
	rawGrade, err := json.MarshalIndent(grade, "", "  ")
	if err != nil {
		return grade, "", err
	}
	path := filepath.Join(reservationDir, "blind-grade.json")
	if err := sealRawEvidence(reservationDir, "blind-grade.json", rawGrade); err != nil {
		return grade, path, err
	}
	return grade, path, nil
}

func GradeSealedOpenCodeEvidenceForBatch(reservationDir string, batch ScreeningBatchDefinition) (BlindGrade, string, error) {
	var reservation ScreeningReservation
	raw, err := os.ReadFile(filepath.Join(reservationDir, "reservation.json"))
	if err != nil {
		return BlindGrade{}, "", err
	}
	if err := json.Unmarshal(raw, &reservation); err != nil {
		return BlindGrade{}, "", err
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		return BlindGrade{}, "", err
	}
	for _, assignment := range assignments {
		if assignment.EvaluationBinding == reservation.EvaluationBinding && assignment.CaseID == reservation.CaseID && assignment.Configuration.ID == reservation.CandidateID {
			return GradeSealedOpenCodeEvidenceVersion(reservationDir, assignment.CaseID, assignment.CaseVersion)
		}
	}
	return BlindGrade{}, "", fmt.Errorf("sealed reservation is not in batch")
}

// ExecuteOpenCodeReservedRun is the sole tested bridge from a valid dry-run
// capsule to one provider invocation. It never retries. A runner error or any
// partial evidence remains in the reservation directory and therefore blocks
// automatic replay as ambiguous.
func ExecuteOpenCodeReservedRun(root string, batch ScreeningBatchDefinition, ledger BatchUsageLedger, spend APISpendSnapshot, evaluatedAt, reservedAt string, resolvedConfig []byte, adapter ExecutionAdapterIdentity, grantInput ProviderCallGrantInput, runner OpenCodeProviderRunner) (CompletedOpenCodeLaunch, error) {
	var completed CompletedOpenCodeLaunch
	if runner == nil {
		return completed, fmt.Errorf("OpenCode provider runner is required")
	}
	if len(batch.APIBudgets) != 1 || adapter.Provider != batch.APIBudgets[0].Provider || adapter.AccountLane != batch.APIBudgets[0].ProjectLane || adapter.BillingBoundary != "openrouter_metered" || adapter.ToolPolicy != "none" {
		return completed, fmt.Errorf("OpenCode adapter does not match the admitted metered lane")
	}
	if grantInput.BatchID != batch.BatchID || grantInput.GrantedBy != "owner" || grantInput.Scope != OneAdmittedOpenRouterObservationScope || grantInput.MaxProviderCalls != 1 {
		return completed, fmt.Errorf("provider-call grant is missing or broader than one admitted observation")
	}
	if _, err := time.Parse(time.RFC3339Nano, grantInput.GrantedAt); err != nil {
		return completed, fmt.Errorf("provider-call grant timestamp invalid: %w", err)
	}
	prepared, reservation, capsule, reservationDir, err := ReserveOpenCodeLaunchCapsule(root, batch, ledger, spend, evaluatedAt, reservedAt, resolvedConfig)
	completed.Prepared, completed.Reservation, completed.Capsule, completed.ReservationDir = prepared, reservation, capsule, reservationDir
	if err != nil {
		return completed, err
	}
	if prepared.Assignment == nil || reservationDir == "" || capsule.ProviderCallAllowed {
		return completed, fmt.Errorf("OpenCode launch was not safely reserved")
	}
	reservationHash, err := ScreeningReservationSHA256(reservation)
	if err != nil {
		return completed, err
	}
	grant := ProviderCallGrant{
		SchemaVersion: ProviderCallGrantSchemaVersion, BatchID: batch.BatchID,
		EvaluationBinding: reservation.EvaluationBinding, ReservationSHA256: reservationHash,
		GrantedBy: grantInput.GrantedBy, Scope: grantInput.Scope, MaxProviderCalls: grantInput.MaxProviderCalls,
		GrantedAt: grantInput.GrantedAt, ProviderCallAllowed: true,
	}
	rawGrant, err := json.MarshalIndent(grant, "", "  ")
	if err != nil {
		return completed, err
	}
	if err := sealRawEvidence(reservationDir, "provider-call-grant.json", rawGrant); err != nil {
		return completed, err
	}
	completed.Grant = grant
	var screeningCase ScreeningCase
	for _, candidateCase := range batch.Cases {
		if candidateCase.CaseID == prepared.Assignment.CaseID && candidateCase.CaseVersion == prepared.Assignment.CaseVersion {
			screeningCase = candidateCase
			break
		}
	}
	plan, err := BuildOpenCodePlanForTransport(batch.CandidateTransport, prepared.Assignment.Configuration, screeningCase)
	if err != nil {
		return completed, err
	}
	rawJSONL, rawExport, runErr := runner(plan)
	if err := sealRawEvidence(reservationDir, "provider-run.jsonl", rawJSONL); err != nil {
		return completed, err
	}
	if err := sealRawEvidence(reservationDir, "provider-session-export.json", rawExport); err != nil {
		return completed, err
	}
	if runErr != nil {
		failure := map[string]any{"state": "ambiguous", "error": runErr.Error(), "raw_jsonl_sha256": sha256Hex(rawJSONL), "raw_export_sha256": sha256Hex(rawExport)}
		rawFailure, _ := json.MarshalIndent(failure, "", "  ")
		_ = sealRawEvidence(reservationDir, "execution-failure.json", rawFailure)
		return completed, fmt.Errorf("OpenCode provider invocation failed and is not retryable: %w", runErr)
	}
	result, response, err := ParseOpenCodeRunArtifact(rawJSONL, rawExport)
	if err != nil {
		failure := map[string]any{"state": "invalid", "error": err.Error(), "raw_jsonl_sha256": sha256Hex(rawJSONL), "raw_export_sha256": sha256Hex(rawExport)}
		rawFailure, _ := json.MarshalIndent(failure, "", "  ")
		_ = sealRawEvidence(reservationDir, "execution-failure.json", rawFailure)
		return completed, err
	}
	receipt, err := BuildExecutionReceipt(reservation, *prepared.Assignment, OpenCodeExecutionPlanBinding(batch.BatchID, plan), adapter, result)
	if err != nil {
		return completed, err
	}
	receiptPath, err := SealExecutionReceipt(reservationDir, receipt)
	if err != nil {
		return completed, err
	}
	completed.Receipt, completed.CandidateResponse, completed.ExecutionReceiptPath = receipt, response, receiptPath
	return completed, nil
}

func sealRawEvidence(dir, name string, raw []byte) error {
	if !filepath.IsAbs(dir) || filepath.Base(name) != name || len(raw) == 0 {
		return fmt.Errorf("raw evidence path or content invalid")
	}
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
