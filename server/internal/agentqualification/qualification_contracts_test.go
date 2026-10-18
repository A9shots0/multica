package agentqualification

import "testing"

func historicalContracts(t *testing.T) QualificationContracts {
	t.Helper()
	c, err := HistoricalQualificationContracts()
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestCurrentQualificationNeverFallsBackToHistoricalInputs(t *testing.T) {
	if _, err := CurrentOpenRouterDirectShadowDefinitionV2(); err == nil {
		t.Fatal("current source drift was silently replaced with historical custody")
	}
	if _, err := CurrentPrimarySubscriptionRoleTrialGateV1(); err == nil {
		t.Fatal("current guarded child drift was silently replaced with historical custody")
	}
	c := historicalContracts(t)
	if !c.Historical() {
		t.Fatal("historical contract must be explicitly labelled")
	}
	if _, err := c.CurrentOpenRouterDirectShadowDefinitionV2(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CurrentPrimarySubscriptionRoleTrialGateV1(); err != nil {
		t.Fatal(err)
	}
}
