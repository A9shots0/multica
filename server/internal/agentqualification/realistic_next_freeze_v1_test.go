package agentqualification

import "testing"

func TestRealisticNextFreezeIsCompleteReviewedAndExecutable(t *testing.T) {
	freeze, err := CurrentRealisticNextFreeze()
	if err != nil {
		t.Fatal(err)
	}
	if freeze.Status != "frozen_reviewed" || !freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "go" || len(freeze.LocalComponents) != 18 || len(freeze.CommandSources) != 5 {
		t.Fatalf("unexpected reviewed realistic next freeze: %+v", freeze)
	}
}

func TestRealisticNextPendingFreezeCannotAllowProviderCalls(t *testing.T) {
	freeze := RealisticNextFreeze{
		SchemaVersion: RealisticNextFreezeSchemaVersion,
		FreezeID: "realistic-continuity-and-rollout-freeze-v1",
		Status: "pending_independent_review",
		ReviewOutcome: "pending",
		ProviderCallsAllowed: true,
		LocalComponents: map[string]string{},
		CommandSources: map[string]string{},
	}
	if err := ValidateRealisticNextFreeze(freeze); err == nil {
		t.Fatal("pending freeze improperly allowed provider calls")
	}
}
