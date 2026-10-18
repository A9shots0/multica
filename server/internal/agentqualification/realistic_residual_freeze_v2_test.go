package agentqualification

import "testing"

func TestRealisticResidualFreezeV2IsCompleteReviewedAndExecutable(t *testing.T) {
	freeze, err := CurrentRealisticResidualFreezeV2()
	if err != nil {
		t.Fatal(err)
	}
	if freeze.Status != "frozen_reviewed" || !freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "go" || len(freeze.LocalComponents) != 5 || len(freeze.CommandSources) != 8 {
		t.Fatalf("unexpected reviewed residual freeze: %+v", freeze)
	}
	if err := RequireRealisticResidualV2ReviewedFreeze(); err != nil {
		t.Fatalf("reviewed residual freeze did not unlock provider calls: %v", err)
	}
}

func TestRealisticResidualFreezeV2RejectsPrematureUnlock(t *testing.T) {
	freeze := RealisticResidualFreezeV2{
		SchemaVersion:               RealisticResidualFreezeV2SchemaVersion,
		FreezeID:                    "realistic-continuity-and-rollout-residual-freeze-v31",
		Status:                      "pending_independent_review",
		ReviewOutcome:               "pending",
		ProviderCallsAllowed:        true,
		UpstreamFixtureFreezeSHA256: RealisticNextFreezeSHA256(),
		LocalComponents:             map[string]string{},
		CommandSources:              map[string]string{},
	}
	if err := ValidateRealisticResidualFreezeV2(freeze); err == nil {
		t.Fatal("pending residual freeze accepted provider unlock")
	}
}
