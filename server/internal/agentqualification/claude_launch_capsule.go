package agentqualification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ClaudeLaunchCapsuleSchemaVersion = "silver-mediums.claude-launch-capsule.v1"

type ClaudeLaunchCapsule struct {
	SchemaVersion       string   `json:"schema_version"`
	State               string   `json:"state"`
	ProviderCallAllowed bool     `json:"provider_call_allowed"`
	BatchID             string   `json:"batch_id"`
	EvaluationBinding   string   `json:"evaluation_binding"`
	ReservationSHA256   string   `json:"reservation_sha256"`
	CandidateID         string   `json:"candidate_id"`
	CaseID              string   `json:"case_id"`
	CaseVersion         string   `json:"case_version"`
	Model               string   `json:"model"`
	ReasoningEffort     string   `json:"reasoning_effort"`
	PromptSHA256        string   `json:"prompt_sha256"`
	SchemaSHA256        string   `json:"output_schema_sha256"`
	ArgsTemplate        []string `json:"args_template"`
}

// ReserveClaudeLaunchCapsule stops at a write-once dry-run artifact. It
// validates the exact plan before taking the subscription-capacity reservation
// and has no process-launch or provider-call capability.
func ReserveClaudeLaunchCapsule(root string, batch ScreeningBatchDefinition, ledger BatchUsageLedger, baseline, current CapacitySnapshot, evaluatedAt, reservedAt string) (PreparedScreeningLaunch, ScreeningReservation, ClaudeLaunchCapsule, string, error) {
	prepared, err := PrepareNextScreeningLaunch(batch, ledger, baseline, current, evaluatedAt)
	if err != nil || !prepared.Admission.CanLaunch || prepared.Assignment == nil {
		return prepared, ScreeningReservation{}, ClaudeLaunchCapsule{}, "", err
	}
	if batch.CandidateTransport != ClaudeCodeToolFreeTransport {
		return prepared, ScreeningReservation{}, ClaudeLaunchCapsule{}, "", fmt.Errorf("batch does not use the Claude tool-free transport")
	}
	var screeningCase ScreeningCase
	caseFound := false
	for _, candidateCase := range batch.Cases {
		if candidateCase.CaseID == prepared.Assignment.CaseID && candidateCase.CaseVersion == prepared.Assignment.CaseVersion {
			screeningCase, caseFound = candidateCase, true
			break
		}
	}
	if !caseFound {
		return prepared, ScreeningReservation{}, ClaudeLaunchCapsule{}, "", fmt.Errorf("prepared assignment case is not in batch")
	}
	plan, err := BuildClaudeScreeningPlan(prepared.Assignment.Configuration, screeningCase)
	if err != nil {
		return prepared, ScreeningReservation{}, ClaudeLaunchCapsule{}, "", err
	}
	reservedPrepared, reservation, reservationDir, err := ReserveNextScreeningLaunch(root, batch, ledger, baseline, current, evaluatedAt, reservedAt)
	if err != nil {
		return reservedPrepared, ScreeningReservation{}, ClaudeLaunchCapsule{}, reservationDir, err
	}
	if reservedPrepared.Assignment == nil || reservedPrepared.Assignment.EvaluationBinding != prepared.Assignment.EvaluationBinding {
		return reservedPrepared, reservation, ClaudeLaunchCapsule{}, reservationDir, fmt.Errorf("assignment changed before atomic reservation")
	}
	reservationHash, err := ScreeningReservationSHA256(reservation)
	if err != nil {
		return reservedPrepared, reservation, ClaudeLaunchCapsule{}, reservationDir, err
	}
	capsule := ClaudeLaunchCapsule{
		SchemaVersion: ClaudeLaunchCapsuleSchemaVersion, State: "ready_no_inference", ProviderCallAllowed: false,
		BatchID: batch.BatchID, EvaluationBinding: reservation.EvaluationBinding, ReservationSHA256: reservationHash,
		CandidateID: plan.CandidateID, CaseID: plan.CaseID, CaseVersion: plan.CaseVersion, Model: plan.Model,
		ReasoningEffort: plan.Effort, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256,
		ArgsTemplate: append([]string(nil), plan.ArgsTemplate...),
	}
	raw, err := json.MarshalIndent(capsule, "", "  ")
	if err != nil {
		return reservedPrepared, reservation, ClaudeLaunchCapsule{}, reservationDir, err
	}
	path := filepath.Join(reservationDir, "launch-capsule.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return reservedPrepared, reservation, ClaudeLaunchCapsule{}, reservationDir, err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return reservedPrepared, reservation, ClaudeLaunchCapsule{}, reservationDir, err
	}
	if closeErr != nil {
		return reservedPrepared, reservation, ClaudeLaunchCapsule{}, reservationDir, closeErr
	}
	return reservedPrepared, reservation, capsule, reservationDir, nil
}
