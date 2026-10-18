package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/multica-ai/multica/server/internal/qualificationarchive"
)

const ClaudeSubscriptionStaticV34FreezeSchema = "silver-mediums.claude-subscription-static-freeze.v34"

type ClaudeSubscriptionStaticV34Freeze struct {
	SchemaVersion        string            `json:"schema_version"`
	FreezeID             string            `json:"freeze_id"`
	Status               string            `json:"status"`
	ReviewOutcome        string            `json:"review_outcome"`
	ProviderCallsAllowed bool              `json:"provider_calls_allowed"`
	DefinitionSHA256     string            `json:"definition_sha256"`
	ExecutionOrder       []string          `json:"execution_order"`
	LocalComponents      map[string]string `json:"local_components"`
	SharedComponents     map[string]string `json:"shared_components"`
	CommandSources       map[string]string `json:"command_sources"`
	Documents            map[string]string `json:"documents"`
}

//go:embed claude_subscription_static_v34_freeze.json
var claudeSubscriptionStaticV34FreezeJSON []byte

//go:embed claude_subscription_static_v34.json
var claudeSubscriptionStaticV34DefinitionJSON []byte

//go:embed claude_subscription_static_v34.go
var claudeSubscriptionStaticV34Source []byte

//go:embed claude_subscription_static_v34_test.go
var claudeSubscriptionStaticV34Tests []byte

//go:embed claude_subscription_static_v34_freeze.go
var claudeSubscriptionStaticV34FreezeSource []byte

//go:embed claude_subscription_static_v34_freeze_test.go
var claudeSubscriptionStaticV34FreezeTests []byte

//go:embed transport_catalog_v1.json
var claudeSubscriptionStaticV34TransportCatalog []byte

//go:embed claude_subscription_static_v34_antecedents.json
var claudeSubscriptionStaticV34Antecedents []byte

//go:embed claude_transport.go
var claudeSubscriptionStaticV34ClaudeTransport []byte

//go:embed claude_receipt.go
var claudeSubscriptionStaticV34ClaudeReceipt []byte

//go:embed execution_normalizer.go
var claudeSubscriptionStaticV34ExecutionNormalizer []byte

//go:embed execution_receipt.go
var claudeSubscriptionStaticV34ExecutionReceipt []byte

//go:embed batch.go
var claudeSubscriptionStaticV34CaseSchemaBuilders []byte

//go:embed grader.go
var claudeSubscriptionStaticV34Grader []byte

//go:embed claude_transport_test.go
var claudeSubscriptionStaticV34ClaudeTransportTests []byte

//go:embed claude_receipt_test.go
var claudeSubscriptionStaticV34ClaudeReceiptTests []byte

//go:embed execution_normalizer_test.go
var claudeSubscriptionStaticV34ExecutionNormalizerTests []byte

//go:embed execution_receipt_test.go
var claudeSubscriptionStaticV34ExecutionReceiptTests []byte

//go:embed grader_test.go
var claudeSubscriptionStaticV34GraderTests []byte

//go:embed grader_capability_group_test.go
var claudeSubscriptionStaticV34CapabilityGraderTests []byte

//go:embed grader_t09_v2_test.go
var claudeSubscriptionStaticV34T09V2GraderTests []byte

func (ctxContracts QualificationContracts) CurrentClaudeSubscriptionStaticV34Freeze() (ClaudeSubscriptionStaticV34Freeze, error) {
	var f ClaudeSubscriptionStaticV34Freeze
	if err := json.Unmarshal(claudeSubscriptionStaticV34FreezeJSON, &f); err != nil {
		return f, fmt.Errorf("decode Claude subscription static v34 freeze: %w", err)
	}
	if err := ctxContracts.ValidateClaudeSubscriptionStaticV34Freeze(f); err != nil {
		return ClaudeSubscriptionStaticV34Freeze{}, err
	}
	return f, nil
}

func (ctxContracts QualificationContracts) ValidateClaudeSubscriptionStaticV34Freeze(f ClaudeSubscriptionStaticV34Freeze) error {
	if f.SchemaVersion != ClaudeSubscriptionStaticV34FreezeSchema || f.FreezeID != "claude-subscription-static-breadth-reliability-freeze-v34" {
		return fmt.Errorf("Claude subscription static v34 freeze identity drifted")
	}
	if f.Status != "frozen_reviewed" || f.ReviewOutcome != "passed_after_consolidated_correction" || !f.ProviderCallsAllowed {
		return fmt.Errorf("Claude subscription static v34 review outcome drifted")
	}
	if f.DefinitionSHA256 != ClaudeSubscriptionStaticV34SHA256() {
		return fmt.Errorf("Claude subscription static v34 definition drifted")
	}
	d, err := ClaudeSubscriptionStaticV34()
	if err != nil {
		return err
	}
	if len(f.ExecutionOrder) != len(d.Cases) {
		return fmt.Errorf("Claude subscription static v34 execution order incomplete")
	}
	for i, c := range d.Cases {
		if f.ExecutionOrder[i] != c.CaseID+":"+c.CaseVersion {
			return fmt.Errorf("Claude subscription static v34 execution order drifted")
		}
	}
	local := map[string][]byte{
		"claude_subscription_static_v34.json":             claudeSubscriptionStaticV34DefinitionJSON,
		"claude_subscription_static_v34.go":               claudeSubscriptionStaticV34Source,
		"claude_subscription_static_v34_test.go":          claudeSubscriptionStaticV34Tests,
		"claude_subscription_static_v34_freeze.go":        claudeSubscriptionStaticV34FreezeSource,
		"claude_subscription_static_v34_freeze_test.go":   claudeSubscriptionStaticV34FreezeTests,
		"claude_subscription_static_v34_antecedents.json": claudeSubscriptionStaticV34Antecedents,
		"transport_catalog_v1.json":                       claudeSubscriptionStaticV34TransportCatalog,
	}
	if ctxContracts.Historical() {
		for name := range local {
			raw, err := qualificationarchive.Read("claude-v34", "server/internal/agentqualification/"+name, f.LocalComponents[name])
			if err != nil {
				return err
			}
			local[name] = raw
		}
	}

	if len(f.LocalComponents) != len(local) {
		return fmt.Errorf("Claude subscription static v34 component set drifted")
	}
	for name, raw := range local {
		h := sha256.Sum256(raw)
		if f.LocalComponents[name] != hex.EncodeToString(h[:]) {
			return fmt.Errorf("Claude subscription static v34 component drift: %s", name)
		}
	}
	shared := map[string][]byte{
		"claude_transport.go":             claudeSubscriptionStaticV34ClaudeTransport,
		"claude_receipt.go":               claudeSubscriptionStaticV34ClaudeReceipt,
		"execution_normalizer.go":         claudeSubscriptionStaticV34ExecutionNormalizer,
		"execution_receipt.go":            claudeSubscriptionStaticV34ExecutionReceipt,
		"batch.go":                        claudeSubscriptionStaticV34CaseSchemaBuilders,
		"grader.go":                       claudeSubscriptionStaticV34Grader,
		"claude_transport_test.go":        claudeSubscriptionStaticV34ClaudeTransportTests,
		"claude_receipt_test.go":          claudeSubscriptionStaticV34ClaudeReceiptTests,
		"execution_normalizer_test.go":    claudeSubscriptionStaticV34ExecutionNormalizerTests,
		"execution_receipt_test.go":       claudeSubscriptionStaticV34ExecutionReceiptTests,
		"grader_test.go":                  claudeSubscriptionStaticV34GraderTests,
		"grader_capability_group_test.go": claudeSubscriptionStaticV34CapabilityGraderTests,
		"grader_t09_v2_test.go":           claudeSubscriptionStaticV34T09V2GraderTests,
	}
	if ctxContracts.Historical() {
		for name := range shared {
			raw, err := qualificationarchive.Read("claude-v34", "server/internal/agentqualification/"+name, f.SharedComponents[name])
			if err != nil {
				return err
			}
			shared[name] = raw
		}
	}

	if len(f.SharedComponents) != len(shared) {
		return fmt.Errorf("Claude subscription static v34 shared component set drifted")
	}
	for name, raw := range shared {
		h := sha256.Sum256(raw)
		if f.SharedComponents[name] != hex.EncodeToString(h[:]) {
			return fmt.Errorf("Claude subscription static v34 shared component drift: %s", name)
		}
	}
	for _, name := range []string{"claude-subscription-static-v34/main.go", "claude-subscription-static-v34/lifecycle.go", "claude-subscription-static-v34/lifecycle_test.go"} {
		if !validSHA256(f.CommandSources[name]) {
			return fmt.Errorf("Claude subscription static v34 command hash missing: %s", name)
		}
	}
	if len(f.CommandSources) != 3 || len(f.Documents) != 1 || !validSHA256(f.Documents["docs/silver-mediums/ITEM-14-CLAUDE-SUBSCRIPTION-STATIC-V34-PREVALIDATION.md"]) {
		return fmt.Errorf("Claude subscription static v34 command/doc freeze incomplete")
	}
	if ctxContracts.Historical() {
		for name, digest := range f.CommandSources {
			if _, err := qualificationarchive.Read("claude-v34", "server/cmd/"+name, digest); err != nil {
				return err
			}
		}
		for name, digest := range f.Documents {
			if _, err := qualificationarchive.Read("claude-v34", name, digest); err != nil {
				return err
			}
		}
	}

	return nil
}

func ClaudeSubscriptionStaticV34FreezeSHA256() string {
	h := sha256.Sum256(claudeSubscriptionStaticV34FreezeJSON)
	return hex.EncodeToString(h[:])
}
