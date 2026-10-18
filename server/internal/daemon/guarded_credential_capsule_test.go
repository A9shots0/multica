package daemon

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestGuardedCredentialCapsuleCopiesOnceAndCleans(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.credentials.json")
	if err := os.WriteFile(sourcePath, []byte("fake-only-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir := t.TempDir()
	capsule := agentqualification.GuardedCredentialCapsuleV1{
		Version:       agentqualification.GuardedCredentialCapsuleVersionV1,
		SourcePath:    sourcePath,
		SourceKind:    agentqualification.GuardedCredentialSourceClaudeCodeSubscriptionV1,
		Purpose:       agentqualification.GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1,
		CopyPolicy:    agentqualification.GuardedCredentialCopyPolicyReadOnlyByteCopyOnceV1,
		CleanupPolicy: agentqualification.GuardedCredentialCleanupPolicyImmediateVerifiedAbsenceV1,
	}
	opens := 0
	lease, err := acquireGuardedCredentialCapsule(capsule, configDir, func(path string) (io.ReadCloser, error) {
		opens++
		return os.Open(path)
	})
	if err != nil {
		t.Fatal(err)
	}
	if opens != 1 {
		t.Fatalf("credential source opens = %d, want 1", opens)
	}
	destination := filepath.Join(configDir, ".credentials.json")
	if raw, err := os.ReadFile(destination); err != nil || string(raw) != "fake-only-secret" {
		t.Fatalf("fake destination = %q, error = %v", raw, err)
	}
	if err := lease.cleanupAndVerify(); err != nil {
		t.Fatal(err)
	}
	if err := lease.cleanupAndVerify(); err != nil {
		t.Fatalf("idempotent cleanup: %v", err)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("credential remains after cleanup: %v", err)
	}
}

func TestGuardedCredentialCapsuleRejectsPolicyDriftBeforeOpen(t *testing.T) {
	capsule := agentqualification.GuardedCredentialCapsuleV1{
		Version:       agentqualification.GuardedCredentialCapsuleVersionV1,
		SourcePath:    filepath.Join(t.TempDir(), "never-opened"),
		SourceKind:    agentqualification.GuardedCredentialSourceClaudeCodeSubscriptionV1,
		Purpose:       "altered-purpose",
		CopyPolicy:    agentqualification.GuardedCredentialCopyPolicyReadOnlyByteCopyOnceV1,
		CleanupPolicy: agentqualification.GuardedCredentialCleanupPolicyImmediateVerifiedAbsenceV1,
	}
	opens := 0
	if _, err := acquireGuardedCredentialCapsule(capsule, t.TempDir(), func(string) (io.ReadCloser, error) {
		opens++
		return nil, nil
	}); err == nil {
		t.Fatal("altered credential capsule was accepted")
	}
	if opens != 0 {
		t.Fatalf("altered capsule opened source %d times", opens)
	}
}
