package agentqualification

import (
	"fmt"
	"strings"
)

const (
	GuardedExactModelTransportVersionV1                 = "silver-mediums.exact-model-transport.v1"
	GuardedExactModelTransportAnthropicMessagesDirectV1 = "anthropic_messages_direct_once_v1"
	GuardedExactModelEndpointAnthropicMessagesV1        = "https://api.anthropic.com/v1/messages"
	GuardedExactModelRequestPolicyV1                    = "exact_messages_json_global_standard_no_tools_v1"
	GuardedExactModelRedirectPolicyDenyV1               = "deny_redirects_v1"
	GuardedExactModelResponseEvidenceV1                 = "required_response_model_field_v1"
	GuardedExactModelCredentialKindV1                   = "anthropic_api_key_v1"
	GuardedExactModelCredentialPurposeV1                = "anthropic_api_key_messages_once_v1"
	GuardedExactModelCredentialCustodyV1                = "daemon_memory_single_request_v1"
	GuardedExactModelCredentialReadPolicyV1             = "read_once_no_persist_v1"
	GuardedExactModelServiceTierV1                      = "standard_only"
	GuardedExactModelInferenceGeoV1                     = "global"
	GuardedExactModelMaxOutputTokensV1                  = 64
	GuardedExactModelMaxInputReservationTokensV1        = 100_000
	GuardedExactModelMaxCostUSDMicrosV1                 = 1_000_000
	GuardedExactModelInputPriceUSDMicrosPerTokenV1      = 3
	GuardedExactModelOutputPriceUSDMicrosPerTokenV1     = 15
	GuardedExactModelRequestTimeoutMSV1                 = 60_000
)

// GuardedExactModelTransportV1 is a provider-free candidate transport policy.
// It deliberately excludes Claude Code: the CLI can report multiple model
// identities only after its internal requests have occurred. A direct Messages
// request lets the controller enforce the sole request's model before it is
// handed to the HTTP transport and verify the provider response independently.
// This policy is not execution authority and contains no credential.
type GuardedExactModelTransportV1 struct {
	Version               string `json:"version"`
	Kind                  string `json:"kind"`
	Endpoint              string `json:"endpoint"`
	Model                 string `json:"model"`
	PromptSHA256          string `json:"prompt_sha256"`
	ProviderRequestLimit  int    `json:"provider_request_limit"`
	RetryLimit            int    `json:"retry_limit"`
	RequestTimeoutMS      int    `json:"request_timeout_ms"`
	MaxOutputTokens       int    `json:"max_output_tokens"`
	MaxInputTokens        int    `json:"max_input_tokens"`
	MaxCostUSDMicros      int64  `json:"max_cost_usd_micros"`
	InputPriceMicros      int64  `json:"input_price_usd_micros_per_token"`
	OutputPriceMicros     int64  `json:"output_price_usd_micros_per_token"`
	RequestPolicy         string `json:"request_policy"`
	ServiceTier           string `json:"service_tier"`
	InferenceGeo          string `json:"inference_geo"`
	RedirectPolicy        string `json:"redirect_policy"`
	ResponseModelEvidence string `json:"response_model_evidence"`
	CredentialKind        string `json:"credential_kind"`
	CredentialPurpose     string `json:"credential_purpose"`
	CredentialCustody     string `json:"credential_custody"`
	CredentialReadPolicy  string `json:"credential_read_policy"`
}

func NewGuardedExactModelTransportV1(model, prompt string) (GuardedExactModelTransportV1, error) {
	policy := GuardedExactModelTransportV1{
		Version:               GuardedExactModelTransportVersionV1,
		Kind:                  GuardedExactModelTransportAnthropicMessagesDirectV1,
		Endpoint:              GuardedExactModelEndpointAnthropicMessagesV1,
		Model:                 model,
		PromptSHA256:          GuardedPromptSHA256V1(prompt),
		ProviderRequestLimit:  1,
		RetryLimit:            0,
		RequestTimeoutMS:      GuardedExactModelRequestTimeoutMSV1,
		MaxOutputTokens:       GuardedExactModelMaxOutputTokensV1,
		MaxInputTokens:        GuardedExactModelMaxInputReservationTokensV1,
		MaxCostUSDMicros:      GuardedExactModelMaxCostUSDMicrosV1,
		InputPriceMicros:      GuardedExactModelInputPriceUSDMicrosPerTokenV1,
		OutputPriceMicros:     GuardedExactModelOutputPriceUSDMicrosPerTokenV1,
		RequestPolicy:         GuardedExactModelRequestPolicyV1,
		ServiceTier:           GuardedExactModelServiceTierV1,
		InferenceGeo:          GuardedExactModelInferenceGeoV1,
		RedirectPolicy:        GuardedExactModelRedirectPolicyDenyV1,
		ResponseModelEvidence: GuardedExactModelResponseEvidenceV1,
		CredentialKind:        GuardedExactModelCredentialKindV1,
		CredentialPurpose:     GuardedExactModelCredentialPurposeV1,
		CredentialCustody:     GuardedExactModelCredentialCustodyV1,
		CredentialReadPolicy:  GuardedExactModelCredentialReadPolicyV1,
	}
	if err := ValidateGuardedExactModelTransportV1(policy, model, prompt); err != nil {
		return GuardedExactModelTransportV1{}, err
	}
	return policy, nil
}

func ValidateGuardedExactModelTransportV1(policy GuardedExactModelTransportV1, expectedModel, prompt string) error {
	if policy.Version != GuardedExactModelTransportVersionV1 ||
		policy.Kind != GuardedExactModelTransportAnthropicMessagesDirectV1 ||
		policy.Endpoint != GuardedExactModelEndpointAnthropicMessagesV1 ||
		policy.ProviderRequestLimit != 1 || policy.RetryLimit != 0 ||
		policy.RequestTimeoutMS != GuardedExactModelRequestTimeoutMSV1 ||
		policy.MaxOutputTokens != GuardedExactModelMaxOutputTokensV1 ||
		policy.MaxInputTokens != GuardedExactModelMaxInputReservationTokensV1 ||
		policy.MaxCostUSDMicros != GuardedExactModelMaxCostUSDMicrosV1 ||
		policy.InputPriceMicros != GuardedExactModelInputPriceUSDMicrosPerTokenV1 ||
		policy.OutputPriceMicros != GuardedExactModelOutputPriceUSDMicrosPerTokenV1 ||
		policy.RequestPolicy != GuardedExactModelRequestPolicyV1 ||
		policy.ServiceTier != GuardedExactModelServiceTierV1 ||
		policy.InferenceGeo != GuardedExactModelInferenceGeoV1 ||
		policy.RedirectPolicy != GuardedExactModelRedirectPolicyDenyV1 ||
		policy.ResponseModelEvidence != GuardedExactModelResponseEvidenceV1 ||
		policy.CredentialKind != GuardedExactModelCredentialKindV1 ||
		policy.CredentialPurpose != GuardedExactModelCredentialPurposeV1 ||
		policy.CredentialCustody != GuardedExactModelCredentialCustodyV1 ||
		policy.CredentialReadPolicy != GuardedExactModelCredentialReadPolicyV1 {
		return fmt.Errorf("%w: invalid exact-model transport policy", ErrGuardedDispatchInvalid)
	}
	reservedCost := int64(policy.MaxInputTokens)*policy.InputPriceMicros + int64(policy.MaxOutputTokens)*policy.OutputPriceMicros
	if reservedCost < 1 || reservedCost > policy.MaxCostUSDMicros {
		return fmt.Errorf("%w: exact-model transport cost reservation exceeds authority", ErrGuardedDispatchInvalid)
	}
	if strings.TrimSpace(policy.Model) == "" || strings.ContainsRune(policy.Model, '\x00') ||
		policy.Model != expectedModel || strings.TrimSpace(prompt) == "" ||
		GuardedPromptSHA256V1(prompt) != policy.PromptSHA256 {
		return fmt.Errorf("%w: exact-model transport identity mismatch", ErrGuardedDispatchInvalid)
	}
	return nil
}
