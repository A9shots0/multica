package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type recoveryBackupReceipt struct {
	RunID       string `json:"run_id"`
	Status      string `json:"status"`
	CompletedAt string `json:"completed_at"`
	Replication string `json:"replication"`
	Artifact    struct {
		SHA256 string `json:"sha256"`
	} `json:"artifact"`
}

type recoveryRestoreReceipt struct {
	RunID       string `json:"run_id"`
	Status      string `json:"status"`
	CompletedAt string `json:"completed_at"`
}

type recoveryOffMachineReceipt struct {
	RunID          string `json:"run_id"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	Status         string `json:"status"`
	VerifiedAt     string `json:"verified_at"`
	KeyCustody     string `json:"key_custody"`
	Reason         string `json:"reason"`
}

type recoveryWatchdogReceipt struct {
	CheckedAt                 string  `json:"checked_at"`
	Platform                  string  `json:"platform"`
	ConsecutiveHealthFailures int     `json:"consecutive_health_failures"`
	IncidentKey               string  `json:"incident_key"`
	BackupAgeHours            float64 `json:"backup_age_hours"`
}

type recoveryStatusResponse struct {
	Status          string   `json:"status"`
	LastBackupAt    string   `json:"last_backup_at,omitempty"`
	BackupAgeHours  *float64 `json:"backup_age_hours,omitempty"`
	LastRestoreAt   string   `json:"last_restore_at,omitempty"`
	Replication     string   `json:"replication"`
	LastCheckedAt   string   `json:"last_checked_at,omitempty"`
	NextAction      string   `json:"next_action"`
	EvidenceCurrent bool     `json:"evidence_current"`
}

func readRecoveryReceipt(path string, target any) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 1<<20 {
		return false
	}
	return json.Unmarshal(data, target) == nil
}

// GetRecoveryStatus returns a deliberately small, owner-only projection of
// host-side disaster-recovery receipts. Paths, command output, phone numbers,
// provider identifiers, and secret-bearing configuration never cross this API.
func (h *Handler) GetRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	directory := os.Getenv("MULTICA_RECOVERY_STATE_DIR")
	if directory == "" {
		writeJSON(w, http.StatusOK, recoveryStatusResponse{
			Status:      "recovery_unproven",
			Replication: "not_configured",
			NextAction:  "Configure and run the first verified backup and isolated recovery rehearsal.",
		})
		return
	}

	var backup recoveryBackupReceipt
	var restore recoveryRestoreReceipt
	var offMachine recoveryOffMachineReceipt
	var watchdog recoveryWatchdogReceipt
	hasBackup := readRecoveryReceipt(filepath.Join(directory, "backup-latest.json"), &backup)
	hasRestore := readRecoveryReceipt(filepath.Join(directory, "restore-latest.json"), &restore)
	hasOffMachine := readRecoveryReceipt(filepath.Join(directory, "off-machine-latest.json"), &offMachine)
	hasWatchdog := readRecoveryReceipt(filepath.Join(directory, "watchdog-state.json"), &watchdog)

	now := time.Now().UTC()
	response := recoveryStatusResponse{
		Status:      "recovery_unproven",
		Replication: "not_configured",
		NextAction:  "Run the first verified backup and isolated recovery rehearsal.",
	}
	var backupCompletedAt time.Time
	backupTimeValid := false
	if hasWatchdog {
		response.LastCheckedAt = watchdog.CheckedAt
	}
	if hasBackup {
		response.LastBackupAt = backup.CompletedAt
		if completedAt, err := time.Parse(time.RFC3339Nano, backup.CompletedAt); err == nil {
			backupCompletedAt = completedAt
			backupTimeValid = true
			age := now.Sub(completedAt).Hours()
			if age < 0 {
				age = 0
			}
			response.BackupAgeHours = &age
		}
	}
	if hasRestore {
		response.LastRestoreAt = restore.CompletedAt
	}
	watchdogCurrent := false
	if hasWatchdog {
		if checkedAt, err := time.Parse(time.RFC3339Nano, watchdog.CheckedAt); err == nil {
			age := now.Sub(checkedAt)
			watchdogCurrent = age >= 0 && age <= 15*time.Minute
		}
	}
	offMachineVerifiedAt, offMachineTimeErr := time.Parse(time.RFC3339Nano, offMachine.VerifiedAt)
	offMachineTimeValid := offMachineTimeErr == nil && backupTimeValid &&
		!offMachineVerifiedAt.Before(backupCompletedAt) && !offMachineVerifiedAt.After(now.Add(5*time.Minute))
	offMachineCurrent := hasOffMachine && offMachine.Status == "verified" && offMachineTimeValid &&
		offMachine.RunID != "" && offMachine.RunID == backup.RunID &&
		offMachine.ArtifactSHA256 != "" && offMachine.ArtifactSHA256 == backup.Artifact.SHA256 &&
		offMachine.KeyCustody == "verified"
	if offMachineCurrent {
		response.Replication = "verified"
	} else if backup.Replication == "copy_verified" {
		response.Replication = "copy_verified"
	}

	if watchdogCurrent && watchdog.Platform == "unavailable" && watchdog.ConsecutiveHealthFailures >= 3 {
		response.Status = "platform_unavailable"
		response.NextAction = "The external watchdog is checking recovery and preserving the incident receipt."
	} else if hasBackup && backup.Status == "failed" {
		response.Status = "backup_failed"
		response.NextAction = "The scheduled backup will retry; inspect the latest recovery receipt if it fails again."
	} else if !hasBackup || backup.Status != "verified" || response.BackupAgeHours == nil || *response.BackupAgeHours > 24 {
		response.Status = "backup_due"
		response.NextAction = "Create a fresh verified recovery point."
	} else if !hasRestore || restore.Status != "verified" || restore.RunID == "" || restore.RunID != backup.RunID {
		response.Status = "recovery_unproven"
		response.NextAction = "Run an isolated recovery rehearsal for the latest backup without touching the active pilot."
	} else if !offMachineCurrent {
		response.Status = "secondary_copy_needed"
		switch {
		case hasOffMachine && offMachine.Status == "sync_pending":
			response.NextAction = "OneDrive is uploading the encrypted recovery point; automatic verification will retry."
		case hasOffMachine && offMachine.Status == "verified" && offMachine.RunID == backup.RunID && offMachine.ArtifactSHA256 == backup.Artifact.SHA256 && offMachine.KeyCustody != "verified":
			response.NextAction = "Save the recovery key in the phone password manager, then confirm separate key custody."
		case hasOffMachine && offMachine.Status == "failed":
			response.NextAction = "Automatic OneDrive verification will retry; inspect recovery status if the failure repeats."
		default:
			response.NextAction = "Choose an off-machine destination and verify artifact and recovery-key custody."
		}
	} else if !watchdogCurrent || watchdog.Platform != "available" || watchdog.ConsecutiveHealthFailures != 0 {
		response.Status = "recovery_unproven"
		response.NextAction = "Restore current external watchdog evidence before calling recovery protected."
	} else {
		response.Status = "protected"
		response.NextAction = "Automatic backups and external health checks continue."
		response.EvidenceCurrent = true
	}
	writeJSON(w, http.StatusOK, response)
}
