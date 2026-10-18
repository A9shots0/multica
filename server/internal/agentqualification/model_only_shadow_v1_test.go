package agentqualification

import (
	"path/filepath"
	"testing"
)

func TestGuardedModelOnlyShadowBindsExactRenderedPrompt(t *testing.T) {
	binding, err := ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		t.Fatal(err)
	}
	shadow, err := NewGuardedModelOnlyShadowV1("fixed harmless prompt", filepath.Join(t.TempDir(), "credential.json"), binding)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderGuardedModelOnlyPromptV1(binding, shadow.Prompt)
	if err != nil {
		t.Fatal(err)
	}
	if GuardedPromptSHA256V1(rendered) != shadow.RenderedPromptSHA256 {
		t.Fatal("rendered prompt hash was not bound")
	}
	if err := validateGuardedModelOnlyShadowV1(&shadow, binding, "claude", []string{GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1}); err != nil {
		t.Fatal(err)
	}

	mutations := map[string]func(*GuardedModelOnlyShadowV1, *[]string){
		"payload": func(value *GuardedModelOnlyShadowV1, _ *[]string) { value.Prompt += " altered" },
		"rendered digest": func(value *GuardedModelOnlyShadowV1, _ *[]string) {
			value.RenderedPromptSHA256 = GuardedPromptSHA256V1("altered")
		},
		"source":      func(value *GuardedModelOnlyShadowV1, _ *[]string) { value.Credential.SourcePath = "relative.json" },
		"copy policy": func(value *GuardedModelOnlyShadowV1, _ *[]string) { value.Credential.CopyPolicy = "copy-many" },
		"purpose":     func(_ *GuardedModelOnlyShadowV1, purposes *[]string) { *purposes = []string{"altered"} },
		"call limit":  func(value *GuardedModelOnlyShadowV1, _ *[]string) { value.ProviderCallLimit = 2 },
		"retry limit": func(value *GuardedModelOnlyShadowV1, _ *[]string) { value.RetryLimit = 1 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := shadow
			purposes := []string{GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1}
			mutate(&candidate, &purposes)
			if err := validateGuardedModelOnlyShadowV1(&candidate, binding, "claude", purposes); err == nil {
				t.Fatal("altered model-only authority was accepted")
			}
		})
	}
}
