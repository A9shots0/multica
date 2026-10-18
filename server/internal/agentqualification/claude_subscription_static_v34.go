package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const ClaudeSubscriptionStaticV34Schema = "silver-mediums.claude-subscription-static.v34"

type ClaudeSubscriptionStaticV34Caps struct {
	MaxProviderInvocations        int   `json:"max_provider_invocations"`
	MaxTotalReportedTokensPerCell int64 `json:"max_total_reported_tokens_per_cell"`
	MaxOutputTokensPerCell        int64 `json:"max_output_tokens_per_cell"`
	MaxCampaignReportedTokens     int64 `json:"max_campaign_reported_tokens"`
	MaxWallSeconds                int   `json:"max_wall_seconds"`
	MaxRetries                    int   `json:"max_retries"`
}

type ClaudeSubscriptionStaticV34RuntimeIdentity struct {
	VersionOutput    string `json:"version_output"`
	ExecutablePath   string `json:"executable_path"`
	ExecutableSHA256 string `json:"executable_sha256"`
	RouteEvidence    string `json:"route_evidence"`
}

type ClaudeSubscriptionStaticV34Definition struct {
	SchemaVersion              string                                     `json:"schema_version"`
	BatchID                    string                                     `json:"batch_id"`
	Status                     string                                     `json:"status"`
	ProviderCallsAllowed       bool                                       `json:"provider_calls_allowed"`
	Transport                  string                                     `json:"transport"`
	Configuration              Configuration                              `json:"configuration"`
	Cases                      []ScreeningCase                            `json:"cases"`
	Caps                       ClaudeSubscriptionStaticV34Caps            `json:"caps"`
	BillingBoundary            string                                     `json:"billing_boundary"`
	RuntimeIdentity            ClaudeSubscriptionStaticV34RuntimeIdentity `json:"runtime_identity"`
	ProductionAllowed          bool                                       `json:"production_allowed"`
	ExternalSideEffectsAllowed bool                                       `json:"external_side_effects_allowed"`
	AdvancementAllowed         bool                                       `json:"advancement_allowed"`
	StopRules                  []string                                   `json:"stop_rules"`
}

//go:embed claude_subscription_static_v34.json
var claudeSubscriptionStaticV34JSON []byte

func ClaudeSubscriptionStaticV34() (ClaudeSubscriptionStaticV34Definition, error) {
	var d ClaudeSubscriptionStaticV34Definition
	if err := json.Unmarshal(claudeSubscriptionStaticV34JSON, &d); err != nil {
		return d, fmt.Errorf("decode Claude subscription static v34: %w", err)
	}
	if err := ValidateClaudeSubscriptionStaticV34(d); err != nil {
		return ClaudeSubscriptionStaticV34Definition{}, err
	}
	return d, nil
}

func ClaudeSubscriptionStaticV34SHA256() string { return sha256Hex(claudeSubscriptionStaticV34JSON) }

func ValidateClaudeSubscriptionStaticV34(d ClaudeSubscriptionStaticV34Definition) error {
	if d.SchemaVersion != ClaudeSubscriptionStaticV34Schema || d.BatchID != "claude-subscription-static-breadth-reliability-v34" || d.Status != "frozen_reviewed" || !d.ProviderCallsAllowed {
		return fmt.Errorf("Claude subscription v34 identity or review lock drifted")
	}
	if d.Transport != ClaudeCodeSubscriptionToolFreeTransport || d.BillingBoundary != "anthropic_subscription" || d.ProductionAllowed || d.ExternalSideEffectsAllowed || d.AdvancementAllowed {
		return fmt.Errorf("Claude subscription v34 boundary drifted")
	}
	if err := ValidateConfiguration(d.Configuration); err != nil {
		return err
	}
	if d.Configuration.ID != "screen.generalist.claude46.medium.subscription.calibration.v29" || d.Configuration.Model != "claude-sonnet-4-6" || d.Configuration.ReasoningEffort != "medium" || d.Configuration.RuntimeProvider != "claude_code_subscription" || d.Configuration.ServingProvider != "anthropic" || d.Configuration.ProviderFallbacks == nil || *d.Configuration.ProviderFallbacks {
		return fmt.Errorf("Claude subscription v34 configuration drifted")
	}
	expected := []string{"T12:v4", "T11:v4", "T08:v3", "T16:v3", "T04:v1", "T09:v2"}
	if len(d.Cases) != len(expected) {
		return fmt.Errorf("Claude subscription v34 case count drifted")
	}
	for i, c := range d.Cases {
		if c.CaseID+":"+c.CaseVersion != expected[i] {
			return fmt.Errorf("Claude subscription v34 case order drifted at %d", i)
		}
		candidate, err := EmbeddedScreeningPacketVersionSHA256(c.CaseID, c.CaseVersion, "candidate")
		if err != nil || candidate != c.CandidatePacketSHA256 {
			return fmt.Errorf("candidate fixture drift for %s", expected[i])
		}
		grader, err := EmbeddedScreeningPacketVersionSHA256(c.CaseID, c.CaseVersion, "grader")
		if err != nil || grader != c.GraderPacketSHA256 {
			return fmt.Errorf("grader fixture drift for %s", expected[i])
		}
		if _, err := BuildClaudeSubscriptionScreeningPlan(d.Configuration, c); err != nil {
			return err
		}
	}
	if d.Caps.MaxProviderInvocations != 6 || d.Caps.MaxTotalReportedTokensPerCell != 15000 || d.Caps.MaxOutputTokensPerCell != 2000 || d.Caps.MaxCampaignReportedTokens != 90000 || d.Caps.MaxWallSeconds != 900 || d.Caps.MaxRetries != 0 {
		return fmt.Errorf("Claude subscription v34 caps drifted")
	}
	if d.RuntimeIdentity.VersionOutput != "2.1.270 (Claude Code)" || d.RuntimeIdentity.ExecutablePath != `C:\Users\a9sho\AppData\Roaming\npm\claude.cmd` || d.RuntimeIdentity.ExecutableSHA256 != "7999fba95dbffe167d9e0a043f29057979a0518ebe89b60c4fcfc6401ea8c424" || d.RuntimeIdentity.RouteEvidence != "signed_in_claude_code_subscription_via_proven_non_bare_stdin_transport; requested_model_claude-sonnet-4-6; direct_anthropic_subscription_boundary; no_api_key_or_credential_material_recorded" {
		return fmt.Errorf("Claude subscription v34 runtime identity drifted")
	}
	if len(d.StopRules) != 5 {
		return fmt.Errorf("Claude subscription v34 stop policy incomplete")
	}
	return nil
}
