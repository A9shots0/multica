package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/multica-ai/multica/server/internal/qualificationarchive"
)

const ClaudeSubscriptionStaticV35FreezeSchema = "silver-mediums.claude-subscription-static-freeze.v35"

const (
	claudeSubscriptionStaticV35PreparedDefinitionSHA256 = "584a8f0848c40466db03457678a5fed09c063cced3ddd3ca98ddb27fc575c1d1"
	claudeSubscriptionStaticV35PreparedAntecedentSHA256 = "70d47c9287a0a01dab0966298d3563a4df28af531227389c632a50cbb5d030ee"
	claudeSubscriptionStaticV35PreparedReviewBinding    = "b54a7dec0b2dc36a73d5c8d540616ee61c3b1703cbeb067d38d5df39e33026b2"
)

type ClaudeSubscriptionStaticV35FreezeRoute struct {
	Model             string `json:"model"`
	ReasoningEffort   string `json:"reasoning_effort"`
	Transport         string `json:"transport"`
	RuntimeProvider   string `json:"runtime_provider"`
	ServingProvider   string `json:"serving_provider"`
	ProviderFallbacks bool   `json:"provider_fallbacks"`
	RuntimeVersion    string `json:"runtime_version"`
	ExecutableSHA256  string `json:"executable_sha256"`
	BillingBoundary   string `json:"billing_boundary"`
}

type ClaudeSubscriptionStaticV35Freeze struct {
	SchemaVersion            string                                 `json:"schema_version"`
	FreezeID                 string                                 `json:"freeze_id"`
	Status                   string                                 `json:"status"`
	ReviewOutcome            string                                 `json:"review_outcome"`
	ProviderCallsAllowed     bool                                   `json:"provider_calls_allowed"`
	PreparedDefinitionSHA256 string                                 `json:"prepared_definition_sha256"`
	PreparedAntecedentSHA256 string                                 `json:"prepared_antecedent_sha256"`
	PreparedReviewBinding    string                                 `json:"prepared_review_binding_sha256"`
	ExecutionOrder           []string                               `json:"execution_order"`
	Route                    ClaudeSubscriptionStaticV35FreezeRoute `json:"route"`
	Caps                     ClaudeSubscriptionStaticV34Caps        `json:"caps"`
	LocalComponents          map[string]string                      `json:"local_components"`
	SharedComponents         map[string]string                      `json:"shared_components"`
	CommandSources           map[string]string                      `json:"command_sources"`
	Documents                map[string]string                      `json:"documents"`
}

//go:embed claude_subscription_static_v35_freeze.json
var claudeSubscriptionStaticV35FreezeJSON []byte

//go:embed claude_subscription_static_v35.json
var claudeSubscriptionStaticV35DefinitionForFreeze []byte

//go:embed claude_subscription_static_v35_antecedents.json
var claudeSubscriptionStaticV35AntecedentsForFreeze []byte

//go:embed claude_subscription_static_v35.go
var claudeSubscriptionStaticV35SourceForFreeze []byte

//go:embed claude_subscription_static_v35_test.go
var claudeSubscriptionStaticV35TestsForFreeze []byte

//go:embed claude_subscription_static_v35_freeze.go
var claudeSubscriptionStaticV35FreezeSource []byte

//go:embed claude_subscription_static_v35_freeze_test.go
var claudeSubscriptionStaticV35FreezeTests []byte

//go:embed transport_catalog_v1.json
var claudeSubscriptionStaticV35TransportCatalog []byte

//go:embed claude_transport.go
var claudeSubscriptionStaticV35ClaudeTransport []byte

//go:embed claude_receipt.go
var claudeSubscriptionStaticV35ClaudeReceipt []byte

//go:embed execution_normalizer.go
var claudeSubscriptionStaticV35ExecutionNormalizer []byte

//go:embed execution_receipt.go
var claudeSubscriptionStaticV35ExecutionReceipt []byte

//go:embed batch.go
var claudeSubscriptionStaticV35CaseSchemaBuilders []byte

//go:embed grader.go
var claudeSubscriptionStaticV35Grader []byte

//go:embed claude_transport_test.go
var claudeSubscriptionStaticV35ClaudeTransportTests []byte

//go:embed claude_receipt_test.go
var claudeSubscriptionStaticV35ClaudeReceiptTests []byte

//go:embed execution_normalizer_test.go
var claudeSubscriptionStaticV35ExecutionNormalizerTests []byte

//go:embed execution_receipt_test.go
var claudeSubscriptionStaticV35ExecutionReceiptTests []byte

//go:embed grader_test.go
var claudeSubscriptionStaticV35GraderTests []byte

//go:embed grader_capability_group_test.go
var claudeSubscriptionStaticV35CapabilityGraderTests []byte

//go:embed grader_t09_v2_test.go
var claudeSubscriptionStaticV35T09V2GraderTests []byte

func (ctxContracts QualificationContracts) CurrentClaudeSubscriptionStaticV35Freeze() (ClaudeSubscriptionStaticV35Freeze, error) {
	var freeze ClaudeSubscriptionStaticV35Freeze
	if err := json.Unmarshal(claudeSubscriptionStaticV35FreezeJSON, &freeze); err != nil {
		return freeze, fmt.Errorf("decode Claude subscription static v35 freeze: %w", err)
	}
	if err := ctxContracts.ValidateClaudeSubscriptionStaticV35Freeze(freeze); err != nil {
		return ClaudeSubscriptionStaticV35Freeze{}, err
	}
	return freeze, nil
}

func (ctxContracts QualificationContracts) ValidateClaudeSubscriptionStaticV35Freeze(freeze ClaudeSubscriptionStaticV35Freeze) error {
	if freeze.SchemaVersion != ClaudeSubscriptionStaticV35FreezeSchema || freeze.FreezeID != "claude-subscription-static-reliability-repeat-freeze-v35" {
		return fmt.Errorf("Claude subscription static v35 freeze identity drifted")
	}
	if freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "one_consolidated_correction_after_independent_hold_no_second_review" || !freeze.ProviderCallsAllowed {
		return fmt.Errorf("Claude subscription static v35 freeze outcome drifted")
	}
	if freeze.PreparedDefinitionSHA256 != claudeSubscriptionStaticV35PreparedDefinitionSHA256 || freeze.PreparedDefinitionSHA256 != ClaudeSubscriptionStaticV35SHA256() ||
		freeze.PreparedAntecedentSHA256 != claudeSubscriptionStaticV35PreparedAntecedentSHA256 || freeze.PreparedAntecedentSHA256 != ClaudeSubscriptionStaticV35AntecedentsSHA256() ||
		freeze.PreparedReviewBinding != claudeSubscriptionStaticV35PreparedReviewBinding || freeze.PreparedReviewBinding != ClaudeSubscriptionStaticV35ReviewBindingSHA256() {
		return fmt.Errorf("Claude subscription static v35 prepared review binding drifted")
	}
	definition, err := ClaudeSubscriptionStaticV35()
	if err != nil {
		return err
	}
	if definition.Status != "prepared_for_independent_review" || definition.ProviderCallsAllowed {
		return fmt.Errorf("Claude subscription static v35 prepared definition was mutated")
	}
	if len(freeze.ExecutionOrder) != len(definition.Cases) {
		return fmt.Errorf("Claude subscription static v35 execution order incomplete")
	}
	for index, screeningCase := range definition.Cases {
		if freeze.ExecutionOrder[index] != screeningCase.CaseID+":"+screeningCase.CaseVersion {
			return fmt.Errorf("Claude subscription static v35 execution order drifted")
		}
	}
	wantRoute := ClaudeSubscriptionStaticV35FreezeRoute{
		Model: definition.Configuration.Model, ReasoningEffort: definition.Configuration.ReasoningEffort,
		Transport: definition.Transport, RuntimeProvider: definition.Configuration.RuntimeProvider,
		ServingProvider: definition.Configuration.ServingProvider, ProviderFallbacks: false,
		RuntimeVersion: definition.RuntimeIdentity.VersionOutput, ExecutableSHA256: definition.RuntimeIdentity.ExecutableSHA256,
		BillingBoundary: definition.BillingBoundary,
	}
	if freeze.Route != wantRoute || freeze.Caps != definition.Caps {
		return fmt.Errorf("Claude subscription static v35 route or caps drifted")
	}
	local := map[string][]byte{
		"claude_subscription_static_v35.json":             claudeSubscriptionStaticV35DefinitionForFreeze,
		"claude_subscription_static_v35_antecedents.json": claudeSubscriptionStaticV35AntecedentsForFreeze,
		"claude_subscription_static_v35.go":               claudeSubscriptionStaticV35SourceForFreeze,
		"claude_subscription_static_v35_test.go":          claudeSubscriptionStaticV35TestsForFreeze,
		"claude_subscription_static_v35_freeze.go":        claudeSubscriptionStaticV35FreezeSource,
		"claude_subscription_static_v35_freeze_test.go":   claudeSubscriptionStaticV35FreezeTests,
		"transport_catalog_v1.json":                       claudeSubscriptionStaticV35TransportCatalog,
	}
	if ctxContracts.Historical() {
		for name := range local {
			raw, err := qualificationarchive.Read("claude-v35", "server/internal/agentqualification/"+name, freeze.LocalComponents[name])
			if err != nil {
				return err
			}
			local[name] = raw
		}
	}

	if err := validateClaudeV35FrozenSet(freeze.LocalComponents, local, "local component"); err != nil {
		return err
	}
	shared := map[string][]byte{
		"claude_transport.go":             claudeSubscriptionStaticV35ClaudeTransport,
		"claude_receipt.go":               claudeSubscriptionStaticV35ClaudeReceipt,
		"execution_normalizer.go":         claudeSubscriptionStaticV35ExecutionNormalizer,
		"execution_receipt.go":            claudeSubscriptionStaticV35ExecutionReceipt,
		"batch.go":                        claudeSubscriptionStaticV35CaseSchemaBuilders,
		"grader.go":                       claudeSubscriptionStaticV35Grader,
		"claude_transport_test.go":        claudeSubscriptionStaticV35ClaudeTransportTests,
		"claude_receipt_test.go":          claudeSubscriptionStaticV35ClaudeReceiptTests,
		"execution_normalizer_test.go":    claudeSubscriptionStaticV35ExecutionNormalizerTests,
		"execution_receipt_test.go":       claudeSubscriptionStaticV35ExecutionReceiptTests,
		"grader_test.go":                  claudeSubscriptionStaticV35GraderTests,
		"grader_capability_group_test.go": claudeSubscriptionStaticV35CapabilityGraderTests,
		"grader_t09_v2_test.go":           claudeSubscriptionStaticV35T09V2GraderTests,
	}
	if ctxContracts.Historical() {
		for name := range shared {
			raw, err := qualificationarchive.Read("claude-v35", "server/internal/agentqualification/"+name, freeze.SharedComponents[name])
			if err != nil {
				return err
			}
			shared[name] = raw
		}
	}

	if err := validateClaudeV35FrozenSet(freeze.SharedComponents, shared, "shared component"); err != nil {
		return err
	}
	for _, name := range []string{"claude-subscription-static-v35/main.go", "claude-subscription-static-v35/lifecycle.go", "claude-subscription-static-v35/lifecycle_test.go"} {
		if !validSHA256(freeze.CommandSources[name]) {
			return fmt.Errorf("Claude subscription static v35 command hash missing: %s", name)
		}
	}
	if len(freeze.CommandSources) != 3 || len(freeze.Documents) != 1 || !validSHA256(freeze.Documents["docs/silver-mediums/ITEM-14-CLAUDE-SUBSCRIPTION-STATIC-V35-PREVALIDATION.md"]) {
		return fmt.Errorf("Claude subscription static v35 command or document freeze incomplete")
	}
	if ctxContracts.Historical() {
		for name, digest := range freeze.CommandSources {
			if _, err := qualificationarchive.Read("claude-v35", "server/cmd/"+name, digest); err != nil {
				return err
			}
		}
		for name, digest := range freeze.Documents {
			if _, err := qualificationarchive.Read("claude-v35", name, digest); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateClaudeV35FrozenSet(recorded map[string]string, actual map[string][]byte, label string) error {
	if len(recorded) != len(actual) {
		return fmt.Errorf("Claude subscription static v35 %s set drifted", label)
	}
	for name, raw := range actual {
		hash := sha256.Sum256(raw)
		if recorded[name] != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("Claude subscription static v35 %s drift: %s", label, name)
		}
	}
	return nil
}

func ClaudeSubscriptionStaticV35FreezeSHA256() string {
	hash := sha256.Sum256(claudeSubscriptionStaticV35FreezeJSON)
	return hex.EncodeToString(hash[:])
}
