package agentqualification

import "testing"

func TestProtocolAblationV32AtomicFreezeIsGOAndExact(t *testing.T) {
	freeze, err := CurrentProtocolAblationV32Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "go" || !freeze.ProviderCallsAllowed || len(ProtocolAblationV32FreezeSHA256()) != 64 {
		t.Fatalf("v32 atomic freeze is not GO: %+v", freeze)
	}
	if err := RequireProtocolAblationV32ReviewedFreeze(); err != nil {
		t.Fatalf("exact reviewed v32 sources did not unlock: %v", err)
	}
}

func TestProtocolAblationV32FreezeRejectsAnyHashDrift(t *testing.T) {
	freeze, err := CurrentProtocolAblationV32Freeze()
	if err != nil {
		t.Fatal(err)
	}
	freeze.LocalComponents = cloneStringMap(freeze.LocalComponents)
	freeze.LocalComponents["protocol_ablation_v32.go"] = "drift"
	if err := ValidateProtocolAblationV32Freeze(freeze); err == nil {
		t.Fatal("v32 local source drift was accepted")
	}
	freeze, _ = CurrentProtocolAblationV32Freeze()
	freeze.CommandSources = cloneStringMap(freeze.CommandSources)
	freeze.CommandSources["protocol-ablation-v32/execute.go"] = "drift"
	if err := ValidateProtocolAblationV32Freeze(freeze); err == nil {
		t.Fatal("v32 command source drift was accepted")
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
