package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const ClaudeSubscriptionRealisticV36FreezeSchema = "silver-mediums.claude-subscription-realistic-freeze.v36"

const (
	claudeSubscriptionRealisticV36PreparedDefinitionSHA256 = "572a649ad9e65d9ab3106aeeaf941d8e373322aba7befed686b6861ebf66eb44"
	claudeSubscriptionRealisticV36ReviewBindingSHA256      = "0a3b6ac8c7e98eeeade2bd42778578a89bfb61d8bc6ddb94ae2a5ad1d3331eb6"
)

type ClaudeSubscriptionRealisticV36FreezeRoute struct {
	Model             string `json:"model"`
	ReasoningEffort   string `json:"reasoning_effort"`
	Transport         string `json:"transport"`
	RuntimeProvider   string `json:"runtime_provider"`
	ServingProvider   string `json:"serving_provider"`
	ProviderFallbacks bool   `json:"provider_fallbacks"`
	RuntimeVersion    string `json:"runtime_version"`
	ExecutableSHA256  string `json:"executable_sha256"`
}

type ClaudeSubscriptionRealisticV36Freeze struct {
	SchemaVersion            string                                    `json:"schema_version"`
	FreezeID                 string                                    `json:"freeze_id"`
	Status                   string                                    `json:"status"`
	ReviewOutcome            string                                    `json:"review_outcome"`
	ProviderCallsAllowed     bool                                      `json:"provider_calls_allowed"`
	PreparedDefinitionSHA256 string                                    `json:"prepared_definition_sha256"`
	ReviewBindingSHA256      string                                    `json:"review_binding_sha256"`
	ExecutionOrder           []string                                  `json:"execution_order"`
	Route                    ClaudeSubscriptionRealisticV36FreezeRoute `json:"route"`
	CaseCaps                 map[string]ClaudeRealisticV36Caps         `json:"case_caps"`
	CampaignCaps             ClaudeRealisticV36Caps                    `json:"campaign_caps"`
	FrozenAntecedents        map[string]string                         `json:"frozen_antecedents"`
	LocalComponents          map[string]string                         `json:"local_components"`
	CommandSources           map[string]string                         `json:"command_sources"`
	Documents                map[string]string                         `json:"documents"`
}

//go:embed claude_subscription_realistic_v36_freeze.json
var claudeSubscriptionRealisticV36FreezeJSON []byte

//go:embed claude_subscription_realistic_v36.json
var claudeSubscriptionRealisticV36DefinitionForFreeze []byte

//go:embed claude_subscription_realistic_v36.go
var claudeSubscriptionRealisticV36SourceForFreeze []byte

//go:embed claude_subscription_realistic_v36_seal.go
var claudeSubscriptionRealisticV36SealerForFreeze []byte

//go:embed claude_subscription_realistic_v36_test.go
var claudeSubscriptionRealisticV36TestsForFreeze []byte

//go:embed claude_subscription_realistic_v36_freeze.go
var claudeSubscriptionRealisticV36FreezeSource []byte

//go:embed claude_subscription_realistic_v36_freeze_test.go
var claudeSubscriptionRealisticV36FreezeTests []byte

func CurrentClaudeSubscriptionRealisticV36Freeze() (ClaudeSubscriptionRealisticV36Freeze, error) {
	var freeze ClaudeSubscriptionRealisticV36Freeze
	if err := json.Unmarshal(claudeSubscriptionRealisticV36FreezeJSON, &freeze); err != nil {
		return freeze, fmt.Errorf("decode v36 freeze: %w", err)
	}
	if err := ValidateClaudeSubscriptionRealisticV36Freeze(freeze); err != nil {
		return ClaudeSubscriptionRealisticV36Freeze{}, err
	}
	return freeze, nil
}

func ValidateClaudeSubscriptionRealisticV36Freeze(freeze ClaudeSubscriptionRealisticV36Freeze) error {
	if freeze.SchemaVersion != ClaudeSubscriptionRealisticV36FreezeSchema || freeze.FreezeID != "claude-subscription-realistic-r03-r04-freeze-v36" || freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "one_consolidated_correction_after_independent_hold_no_second_review" || !freeze.ProviderCallsAllowed {
		return fmt.Errorf("v36 freeze identity/outcome drifted")
	}
	if freeze.PreparedDefinitionSHA256 != claudeSubscriptionRealisticV36PreparedDefinitionSHA256 || freeze.PreparedDefinitionSHA256 != ClaudeSubscriptionRealisticV36SHA256() || freeze.ReviewBindingSHA256 != claudeSubscriptionRealisticV36ReviewBindingSHA256 {
		return fmt.Errorf("v36 prepared review binding drifted")
	}
	d, err := ClaudeSubscriptionRealisticV36()
	if err != nil {
		return err
	}
	if len(freeze.ExecutionOrder) != len(d.ExecutionOrder) {
		return fmt.Errorf("v36 freeze order incomplete")
	}
	for i := range d.ExecutionOrder {
		if freeze.ExecutionOrder[i] != d.ExecutionOrder[i] {
			return fmt.Errorf("v36 freeze order drifted")
		}
	}
	wantRoute := ClaudeSubscriptionRealisticV36FreezeRoute{Model: d.Configuration.Model, ReasoningEffort: d.Configuration.ReasoningEffort, Transport: d.Transport, RuntimeProvider: d.Configuration.RuntimeProvider, ServingProvider: d.Configuration.ServingProvider, ProviderFallbacks: false, RuntimeVersion: d.RuntimeIdentity.VersionOutput, ExecutableSHA256: d.RuntimeIdentity.ExecutableSHA256}
	if freeze.Route != wantRoute || freeze.CampaignCaps != d.CampaignCaps || !equalClaudeV36Caps(freeze.CaseCaps, d.CaseCaps) || !equalClaudeV36Strings(freeze.FrozenAntecedents, d.FrozenAntecedents) {
		return fmt.Errorf("v36 frozen route, caps, or antecedents drifted")
	}
	local := map[string][]byte{
		"claude_subscription_realistic_v36.json":           claudeSubscriptionRealisticV36DefinitionForFreeze,
		"claude_subscription_realistic_v36.go":             claudeSubscriptionRealisticV36SourceForFreeze,
		"claude_subscription_realistic_v36_seal.go":        claudeSubscriptionRealisticV36SealerForFreeze,
		"claude_subscription_realistic_v36_test.go":        claudeSubscriptionRealisticV36TestsForFreeze,
		"claude_subscription_realistic_v36_freeze.go":      claudeSubscriptionRealisticV36FreezeSource,
		"claude_subscription_realistic_v36_freeze_test.go": claudeSubscriptionRealisticV36FreezeTests,
	}
	if len(freeze.LocalComponents) != len(local) {
		return fmt.Errorf("v36 local freeze set incomplete")
	}
	for name, raw := range local {
		hash := sha256.Sum256(raw)
		if freeze.LocalComponents[name] != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("v36 local component drift: %s", name)
		}
	}
	for _, name := range []string{"claude-subscription-realistic-v36/main.go", "claude-subscription-realistic-v36/lifecycle.go", "claude-subscription-realistic-v36/provider.go", "claude-subscription-realistic-v36/lifecycle_test.go"} {
		if !validSHA256(freeze.CommandSources[name]) {
			return fmt.Errorf("v36 command hash missing: %s", name)
		}
	}
	if len(freeze.CommandSources) != 4 || len(freeze.Documents) != 1 || !validSHA256(freeze.Documents["docs/silver-mediums/ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V36-PREVALIDATION.md"]) {
		return fmt.Errorf("v36 command/document freeze incomplete")
	}
	return nil
}

func equalClaudeV36Caps(a, b map[string]ClaudeRealisticV36Caps) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range b {
		if a[key] != value {
			return false
		}
	}
	return true
}

func equalClaudeV36Strings(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range b {
		if a[key] != value {
			return false
		}
	}
	return true
}

func ClaudeSubscriptionRealisticV36FreezeSHA256() string {
	hash := sha256.Sum256(claudeSubscriptionRealisticV36FreezeJSON)
	return hex.EncodeToString(hash[:])
}
