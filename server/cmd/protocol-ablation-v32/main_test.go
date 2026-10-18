package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestProtocolAblationV32PlanIsProviderFreeAndComplete(t *testing.T) {
	plan, err := protocolAblationV32Plan()
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["provider_calls_allowed"] != true || plan["status"] != "frozen_reviewed" || plan["calibration_only"] != true || plan["causal_or_reliability_proof"] != false {
		t.Fatalf("v32 plan escaped its provider-free calibration boundary: %+v", plan)
	}
	bindings := plan["bindings"].([]agentqualification.ProtocolAblationV32ProviderPlan)
	if len(bindings) != 4 {
		t.Fatalf("v32 plan omitted paired bindings: %+v", bindings)
	}
	for _, binding := range bindings {
		if binding.ProviderFallbacks || binding.CaseID != "P32" || binding.Model == "" || binding.Effort != "medium" || binding.InlineConfigSHA256 == "" || binding.AcceptanceContractSHA256 == "" || binding.GeneratedTokenCeiling != 20_000 {
			t.Fatalf("v32 binding is not exact and capped: %+v", binding)
		}
	}
}
