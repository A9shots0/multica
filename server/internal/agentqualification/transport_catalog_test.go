package agentqualification

import "testing"

func TestTransportCatalogPreservesEveryOwnerLaneWithoutAssumingCompatibility(t *testing.T) {
	catalog, err := CurrentTransportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Transports) != 6 {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	if len(catalog.QualifiedTransports()) != 0 {
		t.Fatal("a transport was silently qualified")
	}
	seenSecondCodex := false
	seenCodexFailClosed := false
	seenProviderNeutralOpenCode := false
	for _, transport := range catalog.Transports {
		if transport.ID == "codex_cli_subscription" && contains(transport.AccountLanes, "secondary_subscription") {
			seenSecondCodex = true
			seenCodexFailClosed = transport.Status == "held" && transport.PreventiveToolPolicy == "provider_free_capture_proved_tools_remain_exposed"
		}
		if transport.ID == "opencode_cli" && transport.BillingBoundary == "provider_specific_unverified" && contains(transport.AccountLanes, "local_openrouter_credential") {
			seenProviderNeutralOpenCode = true
		}
	}
	if !seenSecondCodex {
		t.Fatal("second Codex subscription capacity lane was omitted")
	}
	if !seenCodexFailClosed {
		t.Fatal("Codex subscription tool exposure was not preserved fail closed")
	}
	if !seenProviderNeutralOpenCode {
		t.Fatal("OpenCode client was conflated with an unverified subscription/provider lane")
	}
}

func TestTransportCatalogCannotQualifyUnknownIsolationOrUsage(t *testing.T) {
	catalog, err := CurrentTransportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	catalog.Transports[2].Status = "qualified"
	catalog.Transports[2].PreventiveToolPolicy = "unknown"
	if err := ValidateTransportCatalog(catalog); err == nil {
		t.Fatal("unknown transport evidence was qualified")
	}
}
