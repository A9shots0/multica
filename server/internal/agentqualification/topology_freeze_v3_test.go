package agentqualification

import "testing"

func TestTopologyFreezeV3IsReviewedAndProviderReady(t *testing.T) {
	bundle, err := CurrentTopologyFreezeV3()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := CurrentTopologyExperimentV3()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed || !TopologyExperimentV3ProviderReady(definition) || RequireCompleteTopologyFreezeV3() != nil {
		t.Fatal("reviewed topology v3 did not admit provider work")
	}
}
