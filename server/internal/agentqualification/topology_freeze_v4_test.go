package agentqualification

import "testing"

func TestTopologyFreezeV4IsReviewedAndProviderReady(t *testing.T) {
	bundle, err := CurrentTopologyFreezeV4()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := CurrentTopologyExperimentV4()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed || !TopologyExperimentV4ProviderReady(definition) || RequireCompleteTopologyFreezeV4() != nil {
		t.Fatal("reviewed topology v4 did not admit provider work")
	}
}
