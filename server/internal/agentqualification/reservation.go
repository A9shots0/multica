package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const ScreeningReservationSchemaVersion = "silver-mediums.screening-reservation.v1"

var ErrScreeningAssignmentReserved = errors.New("screening assignment already reserved or ambiguous")

// ScreeningReservation is an immutable, pre-provider-call claim. The directory
// creation is the cross-process compare-and-swap. If a process crashes after
// claiming it, the claim remains and blocks automatic retry as ambiguous.
type ScreeningReservation struct {
	SchemaVersion        string `json:"schema_version"`
	BatchID              string `json:"batch_id"`
	EvaluationBinding    string `json:"evaluation_binding"`
	CandidateID          string `json:"candidate_id"`
	CaseID               string `json:"case_id"`
	AdmissionInputSHA256 string `json:"admission_input_sha256"`
	ReservedAt           string `json:"reserved_at"`
	State                string `json:"state"`
}

type reservationAdmissionInput struct {
	Batch       ScreeningBatchDefinition `json:"batch"`
	Ledger      BatchUsageLedger         `json:"ledger"`
	Baseline    CapacitySnapshot         `json:"baseline"`
	Current     CapacitySnapshot         `json:"current"`
	EvaluatedAt string                   `json:"evaluated_at"`
}

type apiReservationAdmissionInput struct {
	Batch       ScreeningBatchDefinition `json:"batch"`
	Ledger      BatchUsageLedger         `json:"ledger"`
	Snapshot    APISpendSnapshot         `json:"snapshot"`
	EvaluatedAt string                   `json:"evaluated_at"`
}

// ReserveNextScreeningLaunch is the only filesystem adapter allowed to turn a
// prepared assignment into an exclusive local claim. It reruns the complete
// fail-closed gate immediately before an atomic reservation.
func ReserveNextScreeningLaunch(root string, batch ScreeningBatchDefinition, ledger BatchUsageLedger, baseline, current CapacitySnapshot, evaluatedAt, reservedAt string) (PreparedScreeningLaunch, ScreeningReservation, string, error) {
	prepared, err := PrepareNextScreeningLaunch(batch, ledger, baseline, current, evaluatedAt)
	if err != nil {
		return PreparedScreeningLaunch{}, ScreeningReservation{}, "", err
	}
	if !prepared.Admission.CanLaunch || prepared.Assignment == nil {
		return prepared, ScreeningReservation{}, "", nil
	}
	if !filepath.IsAbs(root) {
		return prepared, ScreeningReservation{}, "", fmt.Errorf("reservation root must be absolute")
	}
	if _, err := time.Parse(time.RFC3339Nano, reservedAt); err != nil {
		return prepared, ScreeningReservation{}, "", fmt.Errorf("reserved_at invalid: %w", err)
	}
	input := reservationAdmissionInput{Batch: batch, Ledger: ledger, Baseline: baseline, Current: current, EvaluatedAt: evaluatedAt}
	rawInput, err := json.Marshal(input)
	if err != nil {
		return prepared, ScreeningReservation{}, "", err
	}
	inputHash := sha256.Sum256(rawInput)
	return sealScreeningReservation(root, prepared, hex.EncodeToString(inputHash[:]), reservedAt)
}

func ReserveNextScreeningAPILaunch(root string, batch ScreeningBatchDefinition, ledger BatchUsageLedger, snapshot APISpendSnapshot, evaluatedAt, reservedAt string) (PreparedScreeningLaunch, ScreeningReservation, string, error) {
	prepared, err := PrepareNextScreeningAPILaunch(batch, ledger, snapshot, evaluatedAt)
	if err != nil {
		return PreparedScreeningLaunch{}, ScreeningReservation{}, "", err
	}
	if !prepared.Admission.CanLaunch || prepared.Assignment == nil {
		return prepared, ScreeningReservation{}, "", nil
	}
	input := apiReservationAdmissionInput{Batch: batch, Ledger: ledger, Snapshot: snapshot, EvaluatedAt: evaluatedAt}
	rawInput, err := json.Marshal(input)
	if err != nil {
		return prepared, ScreeningReservation{}, "", err
	}
	inputHash := sha256.Sum256(rawInput)
	return sealScreeningReservation(root, prepared, hex.EncodeToString(inputHash[:]), reservedAt)
}

func sealScreeningReservation(root string, prepared PreparedScreeningLaunch, admissionInputSHA256, reservedAt string) (PreparedScreeningLaunch, ScreeningReservation, string, error) {
	if !filepath.IsAbs(root) {
		return prepared, ScreeningReservation{}, "", fmt.Errorf("reservation root must be absolute")
	}
	if _, err := time.Parse(time.RFC3339Nano, reservedAt); err != nil {
		return prepared, ScreeningReservation{}, "", fmt.Errorf("reserved_at invalid: %w", err)
	}
	record := ScreeningReservation{
		SchemaVersion: ScreeningReservationSchemaVersion, BatchID: prepared.Assignment.BatchID,
		EvaluationBinding: prepared.Assignment.EvaluationBinding,
		CandidateID:       prepared.Assignment.Configuration.ID, CaseID: prepared.Assignment.CaseID,
		AdmissionInputSHA256: admissionInputSHA256, ReservedAt: reservedAt, State: "reserved",
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return prepared, ScreeningReservation{}, "", err
	}
	bindingHash := sha256.Sum256([]byte(record.EvaluationBinding))
	claimDir := filepath.Join(root, hex.EncodeToString(bindingHash[:]))
	if err := os.Mkdir(claimDir, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return prepared, ScreeningReservation{}, claimDir, ErrScreeningAssignmentReserved
		}
		return prepared, ScreeningReservation{}, claimDir, err
	}
	recordRaw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return prepared, ScreeningReservation{}, claimDir, err
	}
	file, err := os.OpenFile(filepath.Join(claimDir, "reservation.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return prepared, ScreeningReservation{}, claimDir, err
	}
	if _, err = file.Write(recordRaw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return prepared, ScreeningReservation{}, claimDir, err
	}
	if closeErr != nil {
		return prepared, ScreeningReservation{}, claimDir, closeErr
	}
	return prepared, record, claimDir, nil
}
