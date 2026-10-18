package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const ClaudeSubscriptionCalibrationSchemaVersion = "silver-mediums.claude-subscription-calibration.v1"

type ClaudeCalibrationCaps struct {
	MaxProviderInvocations int   `json:"max_provider_invocations"`
	MaxTotalReportedTokens int64 `json:"max_total_reported_tokens"`
	MaxOutputTokens        int64 `json:"max_output_tokens"`
	MaxWallSeconds         int   `json:"max_wall_seconds"`
	MaxRetries             int   `json:"max_retries"`
	MaxOwnerInterventions  int   `json:"max_owner_interventions"`
}

type ClaudeSubscriptionCalibrationDefinition struct {
	SchemaVersion              string                `json:"schema_version"`
	CalibrationID              string                `json:"calibration_id"`
	Status                     string                `json:"status"`
	Purpose                    string                `json:"purpose"`
	Transport                  string                `json:"transport"`
	Case                       ScreeningCase         `json:"case"`
	Configuration              Configuration         `json:"configuration"`
	Caps                       ClaudeCalibrationCaps `json:"caps"`
	BillingBoundary            string                `json:"billing_boundary"`
	CapacityEvidence           string                `json:"capacity_evidence"`
	ProductionAllowed          bool                  `json:"production_allowed"`
	ExternalSideEffectsAllowed bool                  `json:"external_side_effects_allowed"`
	AdvancementAllowed         bool                  `json:"advancement_allowed"`
	StopRules                  []string              `json:"stop_rules"`
}

//go:embed claude_calibration_v29.json
var claudeCalibrationV29JSON []byte

func ClaudeSubscriptionCalibrationV29() (ClaudeSubscriptionCalibrationDefinition, error) {
	var definition ClaudeSubscriptionCalibrationDefinition
	if err := json.Unmarshal(claudeCalibrationV29JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode Claude subscription calibration: %w", err)
	}
	if err := ValidateClaudeSubscriptionCalibration(definition); err != nil {
		return definition, err
	}
	return definition, nil
}

func ClaudeSubscriptionCalibrationV29SHA256() string {
	return sha256Hex(claudeCalibrationV29JSON)
}

func ValidateClaudeSubscriptionCalibration(definition ClaudeSubscriptionCalibrationDefinition) error {
	if definition.SchemaVersion != ClaudeSubscriptionCalibrationSchemaVersion || definition.CalibrationID != "claude-subscription-transport-v29" {
		return fmt.Errorf("Claude calibration identity is not pinned")
	}
	if definition.Status != "pending_independent_review" && definition.Status != "frozen_reviewed" {
		return fmt.Errorf("Claude calibration status invalid")
	}
	if definition.Transport != ClaudeCodeSubscriptionToolFreeTransport || definition.Case.CaseID != "T11" || definition.Case.CaseVersion != "v4" {
		return fmt.Errorf("Claude calibration transport or case drifted")
	}
	if err := ValidateConfiguration(definition.Configuration); err != nil {
		return err
	}
	if definition.Configuration.ID != "screen.generalist.claude46.medium.subscription.calibration.v29" || definition.Configuration.Model != "claude-sonnet-4-6" || definition.Configuration.ReasoningEffort != "medium" || definition.Configuration.RuntimeProvider != "claude_code_subscription" {
		return fmt.Errorf("Claude calibration configuration drifted")
	}
	plan, err := BuildClaudeSubscriptionScreeningPlan(definition.Configuration, definition.Case)
	if err != nil {
		return err
	}
	if plan.Transport != definition.Transport {
		return fmt.Errorf("Claude calibration plan transport mismatch")
	}
	if definition.Caps.MaxProviderInvocations != 1 || definition.Caps.MaxTotalReportedTokens != 15000 || definition.Caps.MaxOutputTokens != 2000 || definition.Caps.MaxWallSeconds != 120 || definition.Caps.MaxRetries != 0 || definition.Caps.MaxOwnerInterventions != 0 {
		return fmt.Errorf("Claude calibration caps drifted")
	}
	if definition.BillingBoundary != "anthropic_subscription" || definition.CapacityEvidence != "owner_reported_usage_reset_not_authoritative_telemetry" || definition.ProductionAllowed || definition.ExternalSideEffectsAllowed || definition.AdvancementAllowed {
		return fmt.Errorf("Claude calibration authority boundary drifted")
	}
	return nil
}
