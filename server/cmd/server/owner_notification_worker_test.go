package main

import "testing"

func TestOwnerNotificationWorkerRequiresExplicitLiveTestGate(t *testing.T) {
	t.Setenv("MULTICA_OWNER_SMS_MODE", "live_test")
	t.Setenv("MULTICA_OWNER_SMS_LIVE_TEST_ENABLED", "false")
	if _, err := ownerNotificationWorkerFromEnv(nil); err == nil {
		t.Fatal("live_test started without explicit gate")
	}
}

func TestOwnerNotificationWorkerRequiresVerifiedDestinationForLiveModes(t *testing.T) {
	t.Setenv("MULTICA_OWNER_SMS_MODE", "live_test")
	t.Setenv("MULTICA_OWNER_SMS_LIVE_TEST_ENABLED", "true")
	t.Setenv("MULTICA_OWNER_SMS_DESTINATION_VERIFIED", "false")
	if _, err := ownerNotificationWorkerFromEnv(nil); err == nil {
		t.Fatal("live_test started without an explicitly verified owner destination")
	}
}

func TestOwnerNotificationRecipientMustBeOwner(t *testing.T) {
	if !ownerNotificationRecipientRoleAllowed("owner") {
		t.Fatal("owner role rejected")
	}
	for _, role := range []string{"admin", "member", "guest", ""} {
		if ownerNotificationRecipientRoleAllowed(role) {
			t.Fatalf("non-owner role %q accepted", role)
		}
	}
}

func TestOwnerNotificationWorkerDryRunNeedsNoSecrets(t *testing.T) {
	t.Setenv("MULTICA_OWNER_SMS_MODE", "dry_run")
	t.Setenv("MULTICA_OWNER_SMS_DESTINATION", "+15550000001")
	t.Setenv("TELNYX_API_KEY", "")
	worker, err := ownerNotificationWorkerFromEnv(nil)
	if err != nil || worker == nil || worker.Mode != "dry_run" || worker.DestinationKey != "owner_primary" {
		t.Fatalf("worker = %#v, %v", worker, err)
	}
}

func TestDestinationFingerprintDoesNotExposeNumber(t *testing.T) {
	fingerprint := ownerNotificationDestinationFingerprint("+15550000001")
	if len(fingerprint) != 64 || fingerprint == "+15550000001" {
		t.Fatalf("fingerprint = %q", fingerprint)
	}
}
