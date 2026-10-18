package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	GuardedModelOnlyShadowVersionV1                               = "silver-mediums.model-only-shadow.v1"
	GuardedCredentialCapsuleVersionV1                             = "silver-mediums.credential-capsule.v1"
	GuardedCredentialSourceClaudeCodeSubscriptionV1               = "claude_code_subscription_profile"
	GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1 = "claude_code_subscription_inference_once"
	GuardedCredentialCopyPolicyReadOnlyByteCopyOnceV1             = "read_only_byte_copy_once"
	GuardedCredentialCleanupPolicyImmediateVerifiedAbsenceV1      = "immediate_delete_and_verify_absence"
)

// GuardedCredentialCapsuleV1 binds a single credential copy operation without
// containing, parsing, hashing, logging, or otherwise exposing credential
// contents. The source path is controller-owned immutable task authority.
type GuardedCredentialCapsuleV1 struct {
	Version       string `json:"version"`
	SourcePath    string `json:"source_path"`
	SourceKind    string `json:"source_kind"`
	Purpose       string `json:"purpose"`
	CopyPolicy    string `json:"copy_policy"`
	CleanupPolicy string `json:"cleanup_policy"`
}

// GuardedModelOnlyShadowV1 binds the exact prompt rendered from immutable role
// artifacts and fixes the provider lifecycle to one turn, one call, no retry.
type GuardedModelOnlyShadowV1 struct {
	Version              string                     `json:"version"`
	Prompt               string                     `json:"prompt"`
	PromptSHA256         string                     `json:"prompt_sha256"`
	RenderedPromptSHA256 string                     `json:"rendered_prompt_sha256"`
	ProviderCallLimit    int                        `json:"provider_call_limit"`
	RetryLimit           int                        `json:"retry_limit"`
	MaxTurns             int                        `json:"max_turns"`
	Credential           GuardedCredentialCapsuleV1 `json:"credential"`
}

func GuardedPromptSHA256V1(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// NewGuardedModelOnlyShadowV1 constructs authority without reading the source
// credential. The rendered digest covers the exact immutable role pair and
// fixed harmless task prompt that will be written to provider stdin.
func NewGuardedModelOnlyShadowV1(prompt, credentialSourcePath string, binding RoleArtifactBindingV2) (GuardedModelOnlyShadowV1, error) {
	shadow := GuardedModelOnlyShadowV1{
		Version:           GuardedModelOnlyShadowVersionV1,
		Prompt:            prompt,
		PromptSHA256:      GuardedPromptSHA256V1(prompt),
		ProviderCallLimit: 1, RetryLimit: 0, MaxTurns: 1,
		Credential: GuardedCredentialCapsuleV1{
			Version:       GuardedCredentialCapsuleVersionV1,
			SourcePath:    credentialSourcePath,
			SourceKind:    GuardedCredentialSourceClaudeCodeSubscriptionV1,
			Purpose:       GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1,
			CopyPolicy:    GuardedCredentialCopyPolicyReadOnlyByteCopyOnceV1,
			CleanupPolicy: GuardedCredentialCleanupPolicyImmediateVerifiedAbsenceV1,
		},
	}
	rendered, err := RenderGuardedModelOnlyPromptV1(binding, shadow.Prompt)
	if err != nil {
		return GuardedModelOnlyShadowV1{}, err
	}
	shadow.RenderedPromptSHA256 = GuardedPromptSHA256V1(rendered)
	return shadow, nil
}

// RenderGuardedModelOnlyPromptV1 renders only controller-pinned artifact bytes
// and the bound harmless prompt. No issue body, repository data, runtime brief,
// task context, or host data is admitted.
func RenderGuardedModelOnlyPromptV1(binding RoleArtifactBindingV2, prompt string) (string, error) {
	instructions, skill, err := LoadBoundRoleArtifactPairV2(binding)
	if err != nil {
		return "", fmt.Errorf("load guarded model-only artifacts: %w", err)
	}
	return "<guarded-role-instructions>\n" + string(instructions) +
		"\n</guarded-role-instructions>\n<guarded-role-skill>\n" + string(skill) +
		"\n</guarded-role-skill>\n<guarded-task-prompt>\n" + prompt +
		"\n</guarded-task-prompt>", nil
}

func validateGuardedModelOnlyShadowV1(shadow *GuardedModelOnlyShadowV1, binding RoleArtifactBindingV2, provider string, credentialPurposes []string) error {
	if shadow == nil || shadow.Version != GuardedModelOnlyShadowVersionV1 || shadow.ProviderCallLimit != 1 || shadow.RetryLimit != 0 || shadow.MaxTurns != 1 {
		return fmt.Errorf("%w: invalid model-only lifecycle", ErrGuardedDispatchInvalid)
	}
	if provider != "claude" {
		return fmt.Errorf("%w: model-only credential path requires Claude", ErrGuardedDispatchInvalid)
	}
	if strings.TrimSpace(shadow.Prompt) == "" || strings.ContainsRune(shadow.Prompt, '\x00') || len(shadow.Prompt) > 16*1024 || GuardedPromptSHA256V1(shadow.Prompt) != shadow.PromptSHA256 {
		return fmt.Errorf("%w: model-only prompt digest mismatch", ErrGuardedDispatchInvalid)
	}
	credential := shadow.Credential
	if credential.Version != GuardedCredentialCapsuleVersionV1 || credential.SourceKind != GuardedCredentialSourceClaudeCodeSubscriptionV1 || credential.Purpose != GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1 || credential.CopyPolicy != GuardedCredentialCopyPolicyReadOnlyByteCopyOnceV1 || credential.CleanupPolicy != GuardedCredentialCleanupPolicyImmediateVerifiedAbsenceV1 {
		return fmt.Errorf("%w: credential capsule policy mismatch", ErrGuardedDispatchInvalid)
	}
	if strings.TrimSpace(credential.SourcePath) == "" || !filepath.IsAbs(credential.SourcePath) || filepath.Clean(credential.SourcePath) != credential.SourcePath {
		return fmt.Errorf("%w: credential capsule source is not an exact absolute path", ErrGuardedDispatchInvalid)
	}
	if len(credentialPurposes) != 1 || credentialPurposes[0] != credential.Purpose {
		return fmt.Errorf("%w: credential purpose is not exactly bound", ErrGuardedDispatchInvalid)
	}
	rendered, err := RenderGuardedModelOnlyPromptV1(binding, shadow.Prompt)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGuardedDispatchInvalid, err)
	}
	if GuardedPromptSHA256V1(rendered) != shadow.RenderedPromptSHA256 {
		return fmt.Errorf("%w: rendered model-only prompt digest mismatch", ErrGuardedDispatchInvalid)
	}
	return nil
}
