package agentqualification

import "testing"

func TestTopologyFreezeV5IsReviewedAndProviderReady(t *testing.T) {
	bundle, err := CurrentTopologyFreezeV5()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := CurrentTopologyExperimentV5()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed || !TopologyExperimentV5ProviderReady(definition) || RequireCompleteTopologyFreezeV5() != nil {
		t.Fatal("reviewed topology v5 did not admit provider work")
	}
}
