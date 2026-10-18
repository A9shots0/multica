package agentqualification

import "testing"

func TestTopologyFreezeV2IsReviewedAndProviderReady(t *testing.T) {
	bundle, err := CurrentTopologyFreezeV2()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := CurrentTopologyExperimentV2()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed || !TopologyExperimentV2ProviderReady(definition) || RequireCompleteTopologyFreezeV2() != nil {
		t.Fatal("topology v2 did not admit provider work after independent review and atomic freeze")
	}
}
