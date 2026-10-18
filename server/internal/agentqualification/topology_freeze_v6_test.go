package agentqualification

import (
	"errors"
	"strings"
	"testing"
)

func TestTopologyFreezeV6IsReviewedAndProviderReady(t *testing.T) {
	bundle, err := CurrentTopologyFreezeV6()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := CurrentTopologyExperimentV6()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed || !TopologyExperimentV6ProviderReady(definition) || RequireCompleteTopologyFreezeV6() != nil {
		t.Fatal("reviewed topology v6 did not admit provider work")
	}
}

func TestTopologyFreezeV6FailsClosedWhenRealisticFreezeIsIncomplete(t *testing.T) {
	err := requireCompleteTopologyFreezeV6WithRealisticGate(func() error { return errors.New("realistic freeze drift") })
	if err == nil || !strings.Contains(err.Error(), "complete realistic freeze") {
		t.Fatalf("topology v6 ignored its upstream realistic freeze: %v", err)
	}
}
