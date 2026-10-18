package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const ClaudeSubscriptionRealisticV37FreezeSchema = "silver-mediums.claude-subscription-realistic-freeze.v37"

const (
	claudeSubscriptionRealisticV37CommandMainSHA256   = "af9685c34424867c47f14e35dd9ab6ddcecd6d061daa3003488417b9f49fc59e"
	claudeSubscriptionRealisticV37CommandTestSHA256   = "3063b0bcd33f7827d0b8f31031577d87210cc23207ad356060610899dced8dd9"
	claudeSubscriptionRealisticV37LifecycleSHA256     = "af4c172bedbfb8649749b29362ca0ef49da5bd185ab99155b6f6257e5aa53bc4"
	claudeSubscriptionRealisticV37LifecycleTestSHA256 = "6a5790e653c047244b63502414025de9e335d7e81d07eb86caaeca42a979842d"
	claudeSubscriptionRealisticV37ProviderSHA256      = "dba941f2aa33f4a5b841a17442ce6f3e5c1ebd2e47df7d3f6e12113641c057a5"
	claudeSubscriptionRealisticV37PrevalidationSHA256 = "d351afc5b0a6ec83ab4f936824573388c9c15574ed9e3e9bb45c8f70edc6fc75"
)

type ClaudeSubscriptionRealisticV37Freeze struct {
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

//go:embed claude_subscription_realistic_v37_freeze.json
var claudeSubscriptionRealisticV37FreezeJSON []byte

//go:embed claude_subscription_realistic_v37.go
var claudeSubscriptionRealisticV37SourceForFreeze []byte

//go:embed claude_subscription_realistic_v37_seal.go
var claudeSubscriptionRealisticV37SealerForFreeze []byte

//go:embed claude_subscription_realistic_v37_test.go
var claudeSubscriptionRealisticV37TestsForFreeze []byte

//go:embed claude_subscription_realistic_v37_freeze.go
var claudeSubscriptionRealisticV37FreezeSource []byte

//go:embed claude_subscription_realistic_v37_freeze_test.go
var claudeSubscriptionRealisticV37FreezeTests []byte

func CurrentClaudeSubscriptionRealisticV37Freeze() (ClaudeSubscriptionRealisticV37Freeze, error) {
	var freeze ClaudeSubscriptionRealisticV37Freeze
	if err := json.Unmarshal(claudeSubscriptionRealisticV37FreezeJSON, &freeze); err != nil {
		return freeze, fmt.Errorf("decode v37 review lock: %w", err)
	}
	if err := ValidateClaudeSubscriptionRealisticV37Freeze(freeze); err != nil {
		return ClaudeSubscriptionRealisticV37Freeze{}, err
	}
	return freeze, nil
}

func ValidateClaudeSubscriptionRealisticV37Freeze(freeze ClaudeSubscriptionRealisticV37Freeze) error {
	if freeze.SchemaVersion != ClaudeSubscriptionRealisticV37FreezeSchema || freeze.FreezeID != "claude-subscription-realistic-r03-phased-freeze-v37" || freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "accepted_after_one_consolidated_correction_no_second_review" || !freeze.ProviderCallsAllowed {
		return fmt.Errorf("v37 execution-freeze identity drifted")
	}
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		return err
	}
	if freeze.PreparedDefinitionSHA256 != ClaudeSubscriptionRealisticV37SHA256() || len(freeze.ExecutionOrder) != 1 || freeze.ExecutionOrder[0] != "R03:v2" || freeze.Topology != d.Topology || freeze.Caps != d.Caps {
		return fmt.Errorf("v37 definition, order, topology, or caps drifted")
	}
	wantRoute := ClaudeSubscriptionRealisticV36FreezeRoute{Model: d.Configuration.Model, ReasoningEffort: d.Configuration.ReasoningEffort, Transport: d.Transport, RuntimeProvider: d.Configuration.RuntimeProvider, ServingProvider: d.Configuration.ServingProvider, ProviderFallbacks: false, RuntimeVersion: d.RuntimeIdentity.VersionOutput, ExecutableSHA256: d.RuntimeIdentity.ExecutableSHA256}
	if freeze.Route != wantRoute || !equalClaudeV36Strings(freeze.FrozenAntecedents, d.FrozenAntecedents) {
		return fmt.Errorf("v37 route or antecedent drifted")
	}
	local := map[string][]byte{
		"claude_subscription_realistic_v37.go":             claudeSubscriptionRealisticV37SourceForFreeze,
		"claude_subscription_realistic_v37_seal.go":        claudeSubscriptionRealisticV37SealerForFreeze,
		"claude_subscription_realistic_v37_test.go":        claudeSubscriptionRealisticV37TestsForFreeze,
		"claude_subscription_realistic_v37_freeze.go":      claudeSubscriptionRealisticV37FreezeSource,
		"claude_subscription_realistic_v37_freeze_test.go": claudeSubscriptionRealisticV37FreezeTests,
	}
	if len(freeze.LocalComponents) != len(local) {
		return fmt.Errorf("v37 local review-lock set incomplete")
	}
	for name, raw := range local {
		hash := sha256.Sum256(raw)
		if freeze.LocalComponents[name] != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("v37 local component drift: %s", name)
		}
	}
	wantCommands := map[string]string{
		"claude-subscription-realistic-v37/main.go":           claudeSubscriptionRealisticV37CommandMainSHA256,
		"claude-subscription-realistic-v37/main_test.go":      claudeSubscriptionRealisticV37CommandTestSHA256,
		"claude-subscription-realistic-v37/lifecycle.go":      claudeSubscriptionRealisticV37LifecycleSHA256,
		"claude-subscription-realistic-v37/lifecycle_test.go": claudeSubscriptionRealisticV37LifecycleTestSHA256,
		"claude-subscription-realistic-v37/provider.go":       claudeSubscriptionRealisticV37ProviderSHA256,
	}
	if !equalClaudeV36Strings(freeze.CommandSources, wantCommands) || len(freeze.Documents) != 1 || freeze.Documents["docs/silver-mediums/ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V37-PREVALIDATION.md"] != claudeSubscriptionRealisticV37PrevalidationSHA256 {
		return fmt.Errorf("v37 command/document review lock incomplete")
	}
	return nil
}

func ClaudeSubscriptionRealisticV37FreezeSHA256() string {
	hash := sha256.Sum256(claudeSubscriptionRealisticV37FreezeJSON)
	return hex.EncodeToString(hash[:])
}
