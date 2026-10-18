package agentqualification

import "testing"

func TestRealisticFreezePinsAllComponentsAndUnlocksReviewedBundle(t *testing.T) {
	bundle, err := CurrentRealisticFreezeBundle()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		t.Fatalf("unexpected reviewed freeze state: %+v", bundle)
	}
	if err := RequireCompleteRealisticFreeze(); err != nil {
		t.Fatalf("reviewed provider execution did not unlock atomically: %v", err)
	}
}

func TestRealisticFreezeRejectsHashDrift(t *testing.T) {
	bundle, _ := CurrentRealisticFreezeBundle()
	bundle.RunnerSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := ValidateRealisticFreezeBundle(bundle); err == nil {
		t.Fatal("freeze bundle accepted runner drift")
	}
}
