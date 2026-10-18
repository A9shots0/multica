package agentqualification

import (
	"errors"
	"strings"
	"testing"
)

func TestTopologyFreezeV7IsReviewedAndProviderReady(t *testing.T) {
	bundle, err := CurrentTopologyFreezeV7()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := CurrentTopologyExperimentV7()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed || !TopologyExperimentV7ProviderReady(definition) || RequireCompleteTopologyFreezeV7() != nil {
		t.Fatal("reviewed topology v7 did not admit provider work")
	}
}

func TestTopologyFreezeV7FailsClosedWhenRealisticFreezeIsIncomplete(t *testing.T) {
	err := requireCompleteTopologyFreezeV7WithRealisticGate(func() error { return errors.New("realistic freeze drift") })
	if err == nil || !strings.Contains(err.Error(), "complete realistic freeze") {
		t.Fatalf("topology v7 ignored its upstream realistic freeze: %v", err)
	}
}
