package agentqualification

import (
	"testing"
)

func TestProvisionalRoleConfigurationV1ExactScopeAndDenials(t *testing.T) {
	configuration, err := CurrentProvisionalRoleConfigurationV1()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.ProviderLane != "unselected" || configuration.Model != "unselected" || configuration.Squad != "unselected" || configuration.ProductionActivationAllowed {
		t.Fatalf("provisional configuration expanded: %+v", configuration)
	}
	result, receipts, err := RunProvisionalRoleConfigurationRehearsalV1()
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed != 1 || result.Denied != 19 || len(receipts) != 20 {
		t.Fatalf("unexpected provisional rehearsal: %+v", result)
	}
	if recorded, err := CurrentProvisionalRoleConfigurationRehearsalV1(); err != nil || recorded != result {
		t.Fatalf("recorded provisional rehearsal mismatch: %+v %v", recorded, err)
	}
}

func TestProvisionalRoleConfigurationV1RejectsMutations(t *testing.T) {
	configuration, _ := CurrentProvisionalRoleConfigurationV1()
	mutations := []func(*ProvisionalRoleConfigurationV1){
		func(v *ProvisionalRoleConfigurationV1) { v.AllowedCaseIDs[0] = "R02" },
		func(v *ProvisionalRoleConfigurationV1) { v.InlineSkillPacket = "product_ui" },
		func(v *ProvisionalRoleConfigurationV1) { v.ProviderLane = "claude_code_subscription" },
		func(v *ProvisionalRoleConfigurationV1) { v.Model = "claude-sonnet-4-6" },
		func(v *ProvisionalRoleConfigurationV1) { v.ProviderCallsAllowed = true },
		func(v *ProvisionalRoleConfigurationV1) { v.ProductionActivationAllowed = true },
	}
	for index, mutate := range mutations {
		copy := configuration
		copy.AllowedCaseIDs = append([]string(nil), configuration.AllowedCaseIDs...)
		mutate(&copy)
		if err := ValidateProvisionalRoleConfigurationV1(copy); err == nil {
			t.Fatalf("provisional configuration mutation %d passed", index)
		}
	}
}
