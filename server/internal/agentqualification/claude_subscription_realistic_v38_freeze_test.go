package agentqualification

import "testing"

func TestClaudeSubscriptionRealisticV38ExecutionFreezeBindsReviewedHarness(t *testing.T) {
	freeze, err := CurrentClaudeSubscriptionRealisticV38Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if !freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "accepted_after_one_consolidated_correction_no_second_review" || freeze.PreparedDefinitionSHA256 != ClaudeSubscriptionRealisticV38SHA256() {
		t.Fatalf("freeze=%+v", freeze)
	}
	if err := RequireClaudeSubscriptionRealisticV38Freeze(); err != nil {
		t.Fatalf("reviewed execution freeze rejected: %v", err)
	}
	drifted := freeze
	drifted.ProviderCallsAllowed = false
	if err := ValidateClaudeSubscriptionRealisticV38Freeze(drifted); err == nil {
		t.Fatal("freeze drift was accepted")
	}
}
