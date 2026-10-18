package agentqualification

import "testing"

func TestGuardedExactModelTransportV1BindsOneDirectRequest(t *testing.T) {
	const model = "fake-exact-model-v1"
	const prompt = "fixed provider-free prompt"
	policy, err := NewGuardedExactModelTransportV1(model, prompt)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateGuardedExactModelTransportV1(policy, model, prompt); err != nil {
		t.Fatal(err)
	}

	mutations := map[string]func(*GuardedExactModelTransportV1){
		"version":         func(value *GuardedExactModelTransportV1) { value.Version = "altered" },
		"claude code":     func(value *GuardedExactModelTransportV1) { value.Kind = "claude_code_process_v1" },
		"endpoint":        func(value *GuardedExactModelTransportV1) { value.Endpoint = "https://example.invalid/v1/messages" },
		"model":           func(value *GuardedExactModelTransportV1) { value.Model = "other-model" },
		"prompt":          func(value *GuardedExactModelTransportV1) { value.PromptSHA256 = GuardedPromptSHA256V1("other") },
		"request limit":   func(value *GuardedExactModelTransportV1) { value.ProviderRequestLimit = 2 },
		"retry":           func(value *GuardedExactModelTransportV1) { value.RetryLimit = 1 },
		"timeout":         func(value *GuardedExactModelTransportV1) { value.RequestTimeoutMS++ },
		"output ceiling":  func(value *GuardedExactModelTransportV1) { value.MaxOutputTokens++ },
		"input ceiling":   func(value *GuardedExactModelTransportV1) { value.MaxInputTokens++ },
		"cost ceiling":    func(value *GuardedExactModelTransportV1) { value.MaxCostUSDMicros++ },
		"input price":     func(value *GuardedExactModelTransportV1) { value.InputPriceMicros++ },
		"output price":    func(value *GuardedExactModelTransportV1) { value.OutputPriceMicros++ },
		"request policy":  func(value *GuardedExactModelTransportV1) { value.RequestPolicy = "tools_allowed" },
		"service tier":    func(value *GuardedExactModelTransportV1) { value.ServiceTier = "auto" },
		"inference geo":   func(value *GuardedExactModelTransportV1) { value.InferenceGeo = "us" },
		"redirect":        func(value *GuardedExactModelTransportV1) { value.RedirectPolicy = "follow" },
		"evidence":        func(value *GuardedExactModelTransportV1) { value.ResponseModelEvidence = "trust_request" },
		"credential kind": func(value *GuardedExactModelTransportV1) { value.CredentialKind = "other" },
		"credential":      func(value *GuardedExactModelTransportV1) { value.CredentialPurpose = "other" },
		"custody":         func(value *GuardedExactModelTransportV1) { value.CredentialCustody = "ambient" },
		"read policy":     func(value *GuardedExactModelTransportV1) { value.CredentialReadPolicy = "repeatable" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := policy
			mutate(&candidate)
			if err := ValidateGuardedExactModelTransportV1(candidate, model, prompt); err == nil {
				t.Fatal("altered exact-model transport was accepted")
			}
		})
	}
	if err := ValidateGuardedExactModelTransportV1(policy, model, prompt+" altered"); err == nil {
		t.Fatal("altered prompt was accepted")
	}
}
