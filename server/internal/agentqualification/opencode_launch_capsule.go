package agentqualification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const OpenCodeLaunchCapsuleSchemaVersion = "silver-mediums.opencode-launch-capsule.v1"

// OpenCodeLaunchCapsule is the final write-once artifact before a provider
// call. It contains only immutable bindings and placeholder command/environment
// templates; no credential, prompt body, provider call, or paid work occurs.
type OpenCodeLaunchCapsule struct {
	SchemaVersion        string   `json:"schema_version"`
	State                string   `json:"state"`
	ProviderCallAllowed  bool     `json:"provider_call_allowed"`
	BatchID              string   `json:"batch_id"`
	EvaluationBinding    string   `json:"evaluation_binding"`
	ReservationSHA256    string   `json:"reservation_sha256"`
	CandidateID          string   `json:"candidate_id"`
	CaseID               string   `json:"case_id"`
	CaseVersion          string   `json:"case_version"`
	Model                string   `json:"model"`
	ReasoningEffort      string   `json:"reasoning_effort"`
	PromptSHA256         string   `json:"prompt_sha256"`
	SchemaSHA256         string   `json:"output_schema_sha256"`
	InlineConfigSHA256   string   `json:"inline_config_sha256"`
	ResolvedConfigSHA256 string   `json:"resolved_config_sha256"`
	ArgsTemplate         []string `json:"args_template"`
	EnvironmentTemplate  []string `json:"environment_template"`
}

// ReserveOpenCodeLaunchCapsule validates the exact plan and fully resolved
// configuration before taking the atomic reservation. It then seals a dry-run
// capsule beside that reservation. This function deliberately cannot execute
// OpenCode or any other provider.
func ReserveOpenCodeLaunchCapsule(root string, batch ScreeningBatchDefinition, ledger BatchUsageLedger, spend APISpendSnapshot, evaluatedAt, reservedAt string, resolvedConfig []byte) (PreparedScreeningLaunch, ScreeningReservation, OpenCodeLaunchCapsule, string, error) {
	prepared, err := PrepareNextScreeningAPILaunch(batch, ledger, spend, evaluatedAt)
	if err != nil || !prepared.Admission.CanLaunch || prepared.Assignment == nil {
		return prepared, ScreeningReservation{}, OpenCodeLaunchCapsule{}, "", err
	}
	if batch.CandidateTransport != OpenCodeToolFreeTransport && batch.CandidateTransport != OpenCodeToolFreeTransportV2 && batch.CandidateTransport != OpenCodeToolFreeTransportV3 && batch.CandidateTransport != OpenCodeToolFreeTransportV4 && batch.CandidateTransport != OpenCodeToolFreeTransportV5 && batch.CandidateTransport != OpenCodeToolFreeTransportV6 {
		return prepared, ScreeningReservation{}, OpenCodeLaunchCapsule{}, "", fmt.Errorf("batch does not use the OpenCode tool-free transport")
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
		return prepared, ScreeningReservation{}, OpenCodeLaunchCapsule{}, "", fmt.Errorf("prepared assignment case is not in batch")
	}
	plan, err := BuildOpenCodePlanForTransport(batch.CandidateTransport, prepared.Assignment.Configuration, screeningCase)
	if err != nil {
		return prepared, ScreeningReservation{}, OpenCodeLaunchCapsule{}, "", err
	}
	resolvedHash, err := ValidateOpenCodeResolvedConfig(plan, resolvedConfig)
	if err != nil {
		return prepared, ScreeningReservation{}, OpenCodeLaunchCapsule{}, "", err
	}

	reservedPrepared, reservation, reservationDir, err := ReserveNextScreeningAPILaunch(root, batch, ledger, spend, evaluatedAt, reservedAt)
	if err != nil {
		return reservedPrepared, ScreeningReservation{}, OpenCodeLaunchCapsule{}, reservationDir, err
	}
	if reservedPrepared.Assignment == nil || reservedPrepared.Assignment.EvaluationBinding != prepared.Assignment.EvaluationBinding {
		return reservedPrepared, reservation, OpenCodeLaunchCapsule{}, reservationDir, fmt.Errorf("assignment changed before atomic reservation")
	}
	reservationHash, err := ScreeningReservationSHA256(reservation)
	if err != nil {
		return reservedPrepared, reservation, OpenCodeLaunchCapsule{}, reservationDir, err
	}
	capsule := OpenCodeLaunchCapsule{
		SchemaVersion: OpenCodeLaunchCapsuleSchemaVersion, State: "ready_no_inference", ProviderCallAllowed: false,
		BatchID: batch.BatchID, EvaluationBinding: reservation.EvaluationBinding, ReservationSHA256: reservationHash,
		CandidateID: plan.CandidateID, CaseID: plan.CaseID, CaseVersion: plan.CaseVersion, Model: plan.Model,
		ReasoningEffort: plan.Effort, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256,
		InlineConfigSHA256: plan.InlineConfigSHA256, ResolvedConfigSHA256: resolvedHash,
		ArgsTemplate: append([]string(nil), plan.ArgsTemplate...), EnvironmentTemplate: append([]string(nil), plan.EnvironmentTemplate...),
	}
	raw, err := json.MarshalIndent(capsule, "", "  ")
	if err != nil {
		return reservedPrepared, reservation, OpenCodeLaunchCapsule{}, reservationDir, err
	}
	path := filepath.Join(reservationDir, "launch-capsule.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return reservedPrepared, reservation, OpenCodeLaunchCapsule{}, reservationDir, err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return reservedPrepared, reservation, OpenCodeLaunchCapsule{}, reservationDir, err
	}
	if closeErr != nil {
		return reservedPrepared, reservation, OpenCodeLaunchCapsule{}, reservationDir, closeErr
	}
	return reservedPrepared, reservation, capsule, reservationDir, nil
}

func BuildOpenCodePlanForTransport(transport string, configuration Configuration, screeningCase ScreeningCase) (OpenCodeScreeningPlan, error) {
	switch transport {
	case OpenCodeToolFreeTransport:
		return BuildOpenCodeScreeningPlan(configuration, screeningCase)
	case OpenCodeToolFreeTransportV2:
		return BuildOpenCodeScreeningPlanV2(configuration, screeningCase)
	case OpenCodeToolFreeTransportV3:
		return BuildOpenCodeScreeningPlanV3(configuration, screeningCase)
	case OpenCodeToolFreeTransportV4:
		return BuildOpenCodeScreeningPlanV4(configuration, screeningCase)
	case OpenCodeToolFreeTransportV5:
		return BuildOpenCodeScreeningPlanV5(configuration, screeningCase)
	case OpenCodeToolFreeTransportV6:
		return BuildOpenCodeScreeningPlanV6(configuration, screeningCase)
	default:
		return OpenCodeScreeningPlan{}, fmt.Errorf("OpenCode transport %q is not registered", transport)
	}
}
