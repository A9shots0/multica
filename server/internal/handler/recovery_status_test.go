package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRecoveryTestJSON(t *testing.T, dir, name string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readRecoveryStatus(t *testing.T, h *Handler) recoveryStatusResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	h.GetRecoveryStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/workspaces/test/recovery-status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d", recorder.Code)
	}
	var response recoveryStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestRecoveryStatusRequiresBackupAndRestoreProof(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTICA_RECOVERY_STATE_DIR", dir)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	writeRecoveryTestJSON(t, dir, "backup-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": now,
		"replication": "copy_verified", "artifact": map[string]any{"sha256": "abc"},
	})

	response := readRecoveryStatus(t, &Handler{})
	if response.Status != "recovery_unproven" || response.EvidenceCurrent {
		t.Fatalf("backup without restore = %+v", response)
	}

	writeRecoveryTestJSON(t, dir, "restore-latest.json", map[string]any{"run_id": "run-1", "status": "verified", "completed_at": now})
	writeRecoveryTestJSON(t, dir, "off-machine-latest.json", map[string]any{
		"run_id": "run-1", "artifact_sha256": "abc", "status": "verified", "verified_at": now, "key_custody": "verified",
	})
	writeRecoveryTestJSON(t, dir, "watchdog-state.json", map[string]any{
		"checked_at": now, "platform": "available", "consecutive_health_failures": 0,
	})
	response = readRecoveryStatus(t, &Handler{})
	if response.Status != "protected" || !response.EvidenceCurrent || response.Replication != "verified" {
		t.Fatalf("verified backup and restore = %+v", response)
	}
}

func TestRecoveryStatusDoesNotCallSameMachineProofProtected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTICA_RECOVERY_STATE_DIR", dir)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	writeRecoveryTestJSON(t, dir, "backup-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": now, "replication": "copy_verified",
	})
	writeRecoveryTestJSON(t, dir, "restore-latest.json", map[string]any{"run_id": "run-1", "status": "verified", "completed_at": now})
	response := readRecoveryStatus(t, &Handler{})
	if response.Status != "secondary_copy_needed" || response.EvidenceCurrent {
		t.Fatalf("same-machine-only recovery = %+v", response)
	}
}

func TestRecoveryStatusExplainsOneDriveSyncPending(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTICA_RECOVERY_STATE_DIR", dir)
	now := time.Now().UTC()
	current := now.Add(-time.Minute).Format(time.RFC3339Nano)
	writeRecoveryTestJSON(t, dir, "backup-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": current,
		"replication": "copy_verified", "artifact": map[string]any{"sha256": "abc"},
	})
	writeRecoveryTestJSON(t, dir, "restore-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": current,
	})
	writeRecoveryTestJSON(t, dir, "off-machine-latest.json", map[string]any{
		"run_id": "run-1", "artifact_sha256": "abc", "status": "sync_pending", "reason": "onedrive_upload_not_confirmed",
	})

	response := readRecoveryStatus(t, &Handler{})
	if response.Status != "secondary_copy_needed" {
		t.Fatalf("status = %q, want secondary_copy_needed", response.Status)
	}
	if response.NextAction != "OneDrive is uploading the encrypted recovery point; automatic verification will retry." {
		t.Fatalf("unexpected next action: %q", response.NextAction)
	}
}

func TestRecoveryStatusRequestsSeparateKeyAfterCloudProof(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTICA_RECOVERY_STATE_DIR", dir)
	now := time.Now().UTC()
	current := now.Add(-time.Minute).Format(time.RFC3339Nano)
	writeRecoveryTestJSON(t, dir, "backup-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": current,
		"replication": "copy_verified", "artifact": map[string]any{"sha256": "abc"},
	})
	writeRecoveryTestJSON(t, dir, "restore-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": current,
	})
	writeRecoveryTestJSON(t, dir, "off-machine-latest.json", map[string]any{
		"run_id": "run-1", "artifact_sha256": "abc", "status": "verified", "verified_at": now.Format(time.RFC3339Nano), "key_custody": "pending",
	})

	response := readRecoveryStatus(t, &Handler{})
	if response.Status != "secondary_copy_needed" {
		t.Fatalf("status = %q, want secondary_copy_needed", response.Status)
	}
	if response.NextAction != "Save the recovery key in the phone password manager, then confirm separate key custody." {
		t.Fatalf("unexpected next action: %q", response.NextAction)
	}
}

func TestRecoveryStatusFailsClosedForStaleAndUnavailableStates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTICA_RECOVERY_STATE_DIR", dir)
	stale := time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339Nano)
	writeRecoveryTestJSON(t, dir, "backup-latest.json", map[string]any{"status": "verified", "completed_at": stale})
	writeRecoveryTestJSON(t, dir, "restore-latest.json", map[string]any{"run_id": "run-1", "status": "verified", "completed_at": stale})

	response := readRecoveryStatus(t, &Handler{})
	if response.Status != "backup_due" || response.EvidenceCurrent {
		t.Fatalf("stale backup = %+v", response)
	}

	writeRecoveryTestJSON(t, dir, "watchdog-state.json", map[string]any{
		"checked_at": time.Now().UTC().Format(time.RFC3339Nano), "platform": "unavailable", "consecutive_health_failures": 3,
	})
	response = readRecoveryStatus(t, &Handler{})
	if response.Status != "platform_unavailable" || response.EvidenceCurrent {
		t.Fatalf("platform unavailable = %+v", response)
	}
}

func TestRecoveryStatusRejectsRestoreForDifferentBackup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTICA_RECOVERY_STATE_DIR", dir)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	writeRecoveryTestJSON(t, dir, "backup-latest.json", map[string]any{
		"run_id": "new-run", "status": "verified", "completed_at": now,
	})
	writeRecoveryTestJSON(t, dir, "restore-latest.json", map[string]any{
		"run_id": "old-run", "status": "verified", "completed_at": now,
	})
	response := readRecoveryStatus(t, &Handler{})
	if response.Status != "recovery_unproven" || response.EvidenceCurrent {
		t.Fatalf("mismatched restore = %+v", response)
	}
}

func TestRecoveryStatusRejectsStaleWatchdog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTICA_RECOVERY_STATE_DIR", dir)
	now := time.Now().UTC()
	current := now.Format(time.RFC3339Nano)
	writeRecoveryTestJSON(t, dir, "backup-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": current,
		"artifact": map[string]any{"sha256": "abc"},
	})
	writeRecoveryTestJSON(t, dir, "restore-latest.json", map[string]any{
		"run_id": "run-1", "status": "verified", "completed_at": current,
	})
	writeRecoveryTestJSON(t, dir, "off-machine-latest.json", map[string]any{
		"run_id": "run-1", "artifact_sha256": "abc", "status": "verified", "verified_at": current, "key_custody": "verified",
	})
	writeRecoveryTestJSON(t, dir, "watchdog-state.json", map[string]any{
		"checked_at": now.Add(-16 * time.Minute).Format(time.RFC3339Nano),
		"platform":   "available", "consecutive_health_failures": 0,
	})
	response := readRecoveryStatus(t, &Handler{})
	if response.Status != "recovery_unproven" || response.EvidenceCurrent {
		t.Fatalf("stale watchdog = %+v", response)
	}
}
