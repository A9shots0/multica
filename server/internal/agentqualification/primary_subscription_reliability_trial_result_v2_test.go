package agentqualification

import "testing"

func TestPrimarySubscriptionReliabilityTrialResultV2PreservesProvisionalBoundary(t *testing.T) {
	result, err := CurrentPrimarySubscriptionReliabilityTrialResultV2()
	if err != nil {
		t.Fatal(err)
	}
	if result.Passes != 6 || result.Failed != 6 || result.ProvisionalConfiguration != "local_r01_inline_shared_executor_exact_skill_provider_unselected" || result.ModelSelection != "none" || result.PermanentRoleSelection != "none" || result.ProductionActivation != "forbidden" {
		t.Fatalf("unexpected reliability v2 result: %+v", result)
	}
	mutations := []func(*PrimarySubscriptionReliabilityTrialResultV2){
		func(v *PrimarySubscriptionReliabilityTrialResultV2) { v.Passes++ },
		func(v *PrimarySubscriptionReliabilityTrialResultV2) { v.Cells[0].Passes-- },
		func(v *PrimarySubscriptionReliabilityTrialResultV2) { v.IndependentReviewSHA256 = "altered" },
		func(v *PrimarySubscriptionReliabilityTrialResultV2) { v.ModelSelection = "selected" },
		func(v *PrimarySubscriptionReliabilityTrialResultV2) { v.ProductionActivation = "allowed" },
	}
	for index, mutate := range mutations {
		copy := result
		copy.Cells = append([]PrimarySubscriptionReliabilityTrialCellV2(nil), result.Cells...)
		mutate(&copy)
		if err := ValidatePrimarySubscriptionReliabilityTrialResultV2(copy); err == nil {
			t.Fatalf("reliability v2 result mutation %d passed", index)
		}
	}
}
