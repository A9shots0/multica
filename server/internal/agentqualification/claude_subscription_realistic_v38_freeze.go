package agentqualification

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const ClaudeSubscriptionRealisticV38FreezeSchema = "silver-mediums.claude-subscription-realistic-freeze.v38"

const (
	claudeSubscriptionRealisticV38CommandMainSHA256   = "77d7751d59bcc4260aec6a7c7b9a09dd1944669eef627efd4e81e680b6886d4f"
	claudeSubscriptionRealisticV38CommandTestSHA256   = "d6e0c4b2e8eee8b551c4ca2eb76c16451370174922e5dbcd2685f01be2f03937"
	claudeSubscriptionRealisticV38LifecycleSHA256     = "1cc2eca2f8bbb51b02be71c5169b556cd6a58f35a7045c1c689c4db0992e87cb"
	claudeSubscriptionRealisticV38LifecycleTestSHA256 = "c05301da384f646b092753c406ffadaf49112c3d36800a280c8dd86a8d5e9d85"
	claudeSubscriptionRealisticV38ProviderSHA256      = "e7000a342ed22fb2049cfc5247770d4f38fd5b481133f5bc53349b1ac2438d2f"
	claudeSubscriptionRealisticV38PrevalidationSHA256 = "7461ff704aa9666b3bfca445806687adaf61cf5a5399cb81a8d36ea53056448f"
)

type ClaudeSubscriptionRealisticV38Freeze struct {
	SchemaVersion            string                                    `json:"schema_version"`
	FreezeID                 string                                    `json:"freeze_id"`
	Status                   string                                    `json:"status"`
	ReviewOutcome            string                                    `json:"review_outcome"`
	ProviderCallsAllowed     bool                                      `json:"provider_calls_allowed"`
	PreparedDefinitionSHA256 string                                    `json:"prepared_definition_sha256"`
	ExecutionOrder           []string                                  `json:"execution_order"`
	Topology                 string                                    `json:"topology"`
	Route                    ClaudeSubscriptionRealisticV36FreezeRoute `json:"route"`
	Caps                     ClaudeRealisticV37Caps                    `json:"caps"`
	FrozenAntecedents        map[string]string                         `json:"frozen_antecedents"`
	LocalComponents          map[string]string                         `json:"local_components"`
	CommandSources           map[string]string                         `json:"command_sources"`
	Documents                map[string]string                         `json:"documents"`
}

//go:embed claude_subscription_realistic_v38_freeze.json
var claudeSubscriptionRealisticV38FreezeJSON []byte

//go:embed claude_subscription_realistic_v38.go
var claudeSubscriptionRealisticV38SourceForFreeze []byte

//go:embed claude_subscription_realistic_v38_seal.go
var claudeSubscriptionRealisticV38SealerForFreeze []byte

//go:embed claude_subscription_realistic_v38_test.go
var claudeSubscriptionRealisticV38TestsForFreeze []byte

//go:embed claude_subscription_realistic_v38_freeze.go
var claudeSubscriptionRealisticV38FreezeSource []byte

//go:embed claude_subscription_realistic_v38_freeze_test.go
var claudeSubscriptionRealisticV38FreezeTests []byte

func CurrentClaudeSubscriptionRealisticV38Freeze() (ClaudeSubscriptionRealisticV38Freeze, error) {
	var freeze ClaudeSubscriptionRealisticV38Freeze
	decoder := json.NewDecoder(bytes.NewReader(claudeSubscriptionRealisticV38FreezeJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&freeze); err != nil {
		return freeze, fmt.Errorf("decode v38 correction freeze: %w", err)
	}
	if err := ValidateClaudeSubscriptionRealisticV38Freeze(freeze); err != nil {
		return ClaudeSubscriptionRealisticV38Freeze{}, err
	}
	return freeze, nil
}

func ValidateClaudeSubscriptionRealisticV38Freeze(freeze ClaudeSubscriptionRealisticV38Freeze) error {
	if freeze.SchemaVersion != ClaudeSubscriptionRealisticV38FreezeSchema || freeze.FreezeID != "claude-subscription-realistic-r04-phased-freeze-v38" || freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "accepted_after_one_consolidated_correction_no_second_review" || !freeze.ProviderCallsAllowed {
		return fmt.Errorf("v38 correction-freeze identity drifted")
	}
	d, err := ClaudeSubscriptionRealisticV38()
	if err != nil {
		return err
	}
	if freeze.PreparedDefinitionSHA256 != ClaudeSubscriptionRealisticV38SHA256() || len(freeze.ExecutionOrder) != 1 || freeze.ExecutionOrder[0] != "R04:v1" || freeze.Topology != d.Topology || freeze.Caps != d.Caps {
		return fmt.Errorf("v38 definition, order, topology, or caps drifted")
	}
	wantRoute := ClaudeSubscriptionRealisticV36FreezeRoute{Model: d.Configuration.Model, ReasoningEffort: d.Configuration.ReasoningEffort, Transport: d.Transport, RuntimeProvider: d.Configuration.RuntimeProvider, ServingProvider: d.Configuration.ServingProvider, ProviderFallbacks: false, RuntimeVersion: d.RuntimeIdentity.VersionOutput, ExecutableSHA256: d.RuntimeIdentity.ExecutableSHA256}
	if freeze.Route != wantRoute || !equalClaudeV36Strings(freeze.FrozenAntecedents, d.FrozenAntecedents) {
		return fmt.Errorf("v38 route or antecedent drifted")
	}
	local := map[string][]byte{
		"claude_subscription_realistic_v38.go":             claudeSubscriptionRealisticV38SourceForFreeze,
		"claude_subscription_realistic_v38_seal.go":        claudeSubscriptionRealisticV38SealerForFreeze,
		"claude_subscription_realistic_v38_test.go":        claudeSubscriptionRealisticV38TestsForFreeze,
		"claude_subscription_realistic_v38_freeze.go":      claudeSubscriptionRealisticV38FreezeSource,
		"claude_subscription_realistic_v38_freeze_test.go": claudeSubscriptionRealisticV38FreezeTests,
	}
	if len(freeze.LocalComponents) != len(local) {
		return fmt.Errorf("v38 local correction-freeze set incomplete")
	}
	for name, raw := range local {
		hash := sha256.Sum256(raw)
		if freeze.LocalComponents[name] != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("v38 local component drift: %s", name)
		}
	}
	wantCommands := map[string]string{
		"claude-subscription-realistic-v38/main.go":           claudeSubscriptionRealisticV38CommandMainSHA256,
		"claude-subscription-realistic-v38/main_test.go":      claudeSubscriptionRealisticV38CommandTestSHA256,
		"claude-subscription-realistic-v38/lifecycle.go":      claudeSubscriptionRealisticV38LifecycleSHA256,
		"claude-subscription-realistic-v38/lifecycle_test.go": claudeSubscriptionRealisticV38LifecycleTestSHA256,
		"claude-subscription-realistic-v38/provider.go":       claudeSubscriptionRealisticV38ProviderSHA256,
	}
	if !equalClaudeV36Strings(freeze.CommandSources, wantCommands) || len(freeze.Documents) != 1 || freeze.Documents["docs/silver-mediums/ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V38-PREVALIDATION.md"] != claudeSubscriptionRealisticV38PrevalidationSHA256 {
		return fmt.Errorf("v38 command/document correction freeze incomplete")
	}
	return nil
}

func ClaudeSubscriptionRealisticV38FreezeSHA256() string {
	hash := sha256.Sum256(claudeSubscriptionRealisticV38FreezeJSON)
	return hex.EncodeToString(hash[:])
}
