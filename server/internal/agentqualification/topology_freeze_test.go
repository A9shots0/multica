package agentqualification

import "testing"

func TestTopologyFreezePinsCompleteReviewedGroup(t *testing.T) {
	bundle, err := CurrentTopologyFreezeBundle()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := CurrentTopologyExperiment()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status == "frozen_reviewed" && (!bundle.ProviderCallsAllowed || !TopologyExperimentProviderReady(definition)) {
		t.Fatal("reviewed topology bundle did not unlock exact provider entrypoint")
	}
}
