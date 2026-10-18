package agentqualification

import "testing"

func TestCodexSubscriptionBoundaryV1SealsProviderFreeUnsuitableResult(t *testing.T) {
	evidence, err := CurrentCodexSubscriptionBoundaryV1()
	if err != nil {
		t.Fatal(err)
	}
	if CodexSubscriptionBoundaryV1SHA256() == "" {
		t.Fatal("Codex boundary evidence is not hashable")
	}
	if evidence.RealCredentialReads != 0 || evidence.RealProviderRequests != 0 {
		t.Fatalf("provider-free boundary contacted a real lane: %+v", evidence)
	}
	if evidence.CapturedRequest.PreventiveToolFree || len(evidence.CapturedRequest.AdditionalToolNames) != 3 {
		t.Fatalf("tool exposure was hidden: %+v", evidence.CapturedRequest)
	}
}

func TestCodexSubscriptionBoundaryV1FailsClosedOnToolOrAuthorityDrift(t *testing.T) {
	base, err := CurrentCodexSubscriptionBoundaryV1()
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*CodexSubscriptionBoundaryV1){
		"claimed tool free": func(v *CodexSubscriptionBoundaryV1) { v.CapturedRequest.PreventiveToolFree = true },
		"hidden tool": func(v *CodexSubscriptionBoundaryV1) {
			v.CapturedRequest.AdditionalToolNames = v.CapturedRequest.AdditionalToolNames[:2]
		},
		"retry":           func(v *CodexSubscriptionBoundaryV1) { v.CapturedRequest.RequestMaxRetries = 1 },
		"duplicate":       func(v *CodexSubscriptionBoundaryV1) { v.CapturedRequest.RequestCount = 2 },
		"model drift":     func(v *CodexSubscriptionBoundaryV1) { v.CapturedRequest.Model = "gpt-5.6-sol" },
		"credential read": func(v *CodexSubscriptionBoundaryV1) { v.RealCredentialReads = 1 },
		"provider call":   func(v *CodexSubscriptionBoundaryV1) { v.RealProviderRequests = 1 },
		"promoted":        func(v *CodexSubscriptionBoundaryV1) { v.Status = "qualified" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			changed := base
			changed.CapturedRequest.AdditionalToolNames = append([]string(nil), base.CapturedRequest.AdditionalToolNames...)
			mutate(&changed)
			if err := ValidateCodexSubscriptionBoundaryV1(changed); err == nil {
				t.Fatal("mutated boundary evidence was accepted")
			}
		})
	}
}
