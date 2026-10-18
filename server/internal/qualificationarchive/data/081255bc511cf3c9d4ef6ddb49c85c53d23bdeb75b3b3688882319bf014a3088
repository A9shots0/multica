package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const ClaudeSubscriptionStaticV35Schema = "silver-mediums.claude-subscription-static.v35"

type ClaudeSubscriptionStaticV35Definition struct {
	SchemaVersion              string                                     `json:"schema_version"`
	BatchID                    string                                     `json:"batch_id"`
	Status                     string                                     `json:"status"`
	ProviderCallsAllowed       bool                                       `json:"provider_calls_allowed"`
	Transport                  string                                     `json:"transport"`
	RepeatOfBatchID            string                                     `json:"repeat_of_batch_id"`
	RepeatIndex                int                                        `json:"repeat_index"`
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

type ClaudeSubscriptionStaticV35AntecedentResult struct {
	CaseID        string `json:"case_id"`
	CaseVersion   string `json:"case_version"`
	Result        string `json:"result"`
	ReceiptSHA256 string `json:"receipt_sha256"`
	GradeSHA256   string `json:"grade_sha256"`
}

type ClaudeSubscriptionStaticV35Antecedents struct {
	SchemaVersion                   string                                        `json:"schema_version"`
	BatchID                         string                                        `json:"batch_id"`
	PriorBatchID                    string                                        `json:"prior_batch_id"`
	PriorDefinitionSHA256           string                                        `json:"prior_definition_sha256"`
	PriorFreezeSHA256               string                                        `json:"prior_freeze_sha256"`
	PriorLedgerSHA256               string                                        `json:"prior_ledger_sha256"`
	PriorEvidenceRoot               string                                        `json:"prior_evidence_root"`
	PriorStatus                     string                                        `json:"prior_status"`
	PriorGlobalHalt                 bool                                          `json:"prior_global_halt"`
	PriorCandidateEliminated        bool                                          `json:"prior_candidate_eliminated"`
	PriorProviderInvocations        int                                           `json:"prior_provider_invocations"`
	PriorInputTokens                int64                                         `json:"prior_input_tokens"`
	PriorOutputTokens               int64                                         `json:"prior_output_tokens"`
	PriorProviderReportedCostMicros int64                                         `json:"prior_provider_reported_cost_micros"`
	PriorResults                    []ClaudeSubscriptionStaticV35AntecedentResult `json:"prior_results"`
}

//go:embed claude_subscription_static_v35.json
var claudeSubscriptionStaticV35JSON []byte

//go:embed claude_subscription_static_v35_antecedents.json
var claudeSubscriptionStaticV35AntecedentsJSON []byte

func ClaudeSubscriptionStaticV35() (ClaudeSubscriptionStaticV35Definition, error) {
	var d ClaudeSubscriptionStaticV35Definition
	if err := json.Unmarshal(claudeSubscriptionStaticV35JSON, &d); err != nil {
		return d, err
	}
	if err := ValidateClaudeSubscriptionStaticV35(d); err != nil {
		return ClaudeSubscriptionStaticV35Definition{}, err
	}
	return d, nil
}

func ClaudeSubscriptionStaticV35AntecedentManifest() (ClaudeSubscriptionStaticV35Antecedents, error) {
	var a ClaudeSubscriptionStaticV35Antecedents
	if err := json.Unmarshal(claudeSubscriptionStaticV35AntecedentsJSON, &a); err != nil {
		return a, err
	}
	if a.BatchID != "claude-subscription-static-reliability-repeat-v35" || a.PriorBatchID != "claude-subscription-static-breadth-reliability-v34" || a.PriorDefinitionSHA256 != ClaudeSubscriptionStaticV34SHA256() || a.PriorStatus != "complete" || a.PriorGlobalHalt || a.PriorCandidateEliminated || a.PriorProviderInvocations != 6 || len(a.PriorResults) != 6 {
		return a, fmt.Errorf("v35 antecedent drifted")
	}
	for _, r := range a.PriorResults {
		if r.Result != "pass" || r.ReceiptSHA256 == "" || r.GradeSHA256 == "" {
			return a, fmt.Errorf("v35 antecedent result incomplete")
		}
	}
	return a, nil
}

func ClaudeSubscriptionStaticV35SHA256() string { return sha256Hex(claudeSubscriptionStaticV35JSON) }
func ClaudeSubscriptionStaticV35AntecedentsSHA256() string {
	return sha256Hex(claudeSubscriptionStaticV35AntecedentsJSON)
}
func ClaudeSubscriptionStaticV35ReviewBindingSHA256() string {
	return sha256Hex(append(append([]byte{}, claudeSubscriptionStaticV35JSON...), claudeSubscriptionStaticV35AntecedentsJSON...))
}

func ValidateClaudeSubscriptionStaticV35(d ClaudeSubscriptionStaticV35Definition) error {
	if d.SchemaVersion != ClaudeSubscriptionStaticV35Schema || d.BatchID != "claude-subscription-static-reliability-repeat-v35" || d.Status != "prepared_for_independent_review" || d.ProviderCallsAllowed {
		return fmt.Errorf("v35 review lock drifted")
	}
	if d.RepeatOfBatchID != "claude-subscription-static-breadth-reliability-v34" || d.RepeatIndex != 1 || d.Transport != ClaudeCodeSubscriptionToolFreeTransport || d.BillingBoundary != "anthropic_subscription" || d.ProductionAllowed || d.ExternalSideEffectsAllowed || d.AdvancementAllowed {
		return fmt.Errorf("v35 boundary drifted")
	}
	if err := ValidateConfiguration(d.Configuration); err != nil {
		return err
	}
	if d.Configuration.ID != "screen.generalist.claude46.medium.subscription.calibration.v29" || d.Configuration.Model != "claude-sonnet-4-6" || d.Configuration.ReasoningEffort != "medium" || d.Configuration.RuntimeProvider != "claude_code_subscription" || d.Configuration.ServingProvider != "anthropic" || d.Configuration.ProviderFallbacks == nil || *d.Configuration.ProviderFallbacks {
		return fmt.Errorf("v35 configuration drifted")
	}
	expected := []string{"T12:v4", "T11:v4", "T08:v3", "T16:v3", "T04:v1", "T09:v2"}
	if len(d.Cases) != len(expected) {
		return fmt.Errorf("v35 case count drifted")
	}
	for i, c := range d.Cases {
		if c.CaseID+":"+c.CaseVersion != expected[i] {
			return fmt.Errorf("v35 case order drifted at %d", i)
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
		return fmt.Errorf("v35 caps drifted")
	}
	if d.RuntimeIdentity.VersionOutput != "2.1.270 (Claude Code)" || d.RuntimeIdentity.ExecutablePath != `C:\Users\a9sho\AppData\Roaming\npm\claude.cmd` || d.RuntimeIdentity.ExecutableSHA256 != "7999fba95dbffe167d9e0a043f29057979a0518ebe89b60c4fcfc6401ea8c424" {
		return fmt.Errorf("v35 runtime identity drifted")
	}
	if len(d.StopRules) != 5 {
		return fmt.Errorf("v35 stop policy incomplete")
	}
	return nil
}
