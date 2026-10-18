package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV6SchemaVersion = "silver-mediums.topology-freeze.v6"

//go:embed topology_freeze_v6.json
var topologyFreezeV6JSON []byte

//go:embed topology_experiment_v6.json
var topologyFreezeV6Experiment []byte

//go:embed topology_runner_v6.go
var topologyFreezeV6Runner []byte

//go:embed topology_review_v5.go
var topologyFreezeV6Review []byte

//go:embed topology_campaign_v6.go
var topologyFreezeV6Campaign []byte

//go:embed topology_freeze_v5.json
var topologyFreezeV6PreservedV5 []byte

type topologyFreezeV6Sources struct {
	OpenCodeProviderSHA256 string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256  string `json:"opencode_runtime_sha256"`
	TopologyV5FreezeSHA256 string `json:"topology_v5_freeze_sha256"`
}

func CurrentTopologyFreezeV6() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV6JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV6(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV6(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV6SchemaVersion || bundle.BundleID != "realistic-topology-r03-mercury-claude-v6" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v6 freeze identity or status is invalid")
	}
	components := map[string][]byte{bundle.ExperimentSHA256: topologyFreezeV6Experiment, bundle.RunnerSHA256: topologyFreezeV6Runner, bundle.ReviewSHA256: topologyFreezeV6Review, bundle.CampaignSHA256: topologyFreezeV6Campaign}
	if len(components) != 4 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) {
		return fmt.Errorf("topology v6 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("topology v6 freeze component hash mismatch")
		}
	}
	var sources topologyFreezeV6Sources
	if err := json.Unmarshal(topologyFreezeV6JSON, &sources); err != nil || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.TopologyV5FreezeSHA256) {
		return fmt.Errorf("topology v6 source hashes are incomplete")
	}
	v5Hash := sha256.Sum256(topologyFreezeV6PreservedV5)
	if hex.EncodeToString(v5Hash[:]) != sources.TopologyV5FreezeSHA256 {
		return fmt.Errorf("preserved topology-v5 freeze bytes drifted")
	}
	if _, err := CurrentTopologyFreezeV5(); err != nil {
		return fmt.Errorf("preserved topology-v5 components drifted: %w", err)
	}
	if !exactSet(bundle.ValidationMatrix, "exact_mercury25_inception_screening_default_executor", "exact_claude_sonnet5_anthropic_medium_reviewer", "cross_model_reviewer_independence", "generation_bound_reviewer_validation", "executor_protocol_failure_candidate_local", "correction_protocol_failure_candidate_local", "provider_budget_breach_global_halt", "evidence_corruption_global_halt", "harness_imperfection_observation_local", "unchanged_r03_v2_fixture_and_grader", "r03_reference_pass", "r03_subtle_fail_rejected", "r03_paraphrase_pass", "r03_adversarial_fail_rejected", "no_fallbacks", "no_transport_retries", "local_only_no_external_side_effects", "four_fresh_counterbalanced_observations", "reviewer_16000_total_cap", "reviewed_arm_140000_covers_all_substages", "campaign_430000_total_cap", "incremental_spend_1300000_micros", "paid_response_sealed_before_stop", "exact_receipt_accounting", "exclusive_claim", "crash_no_retry", "one_correction_ceiling", "v5_byte_preservation", "recursive_v5_v4_v3_v2_v1_command_source_admission") {
		return fmt.Errorf("topology v6 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v6 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreezeV6() error {
	return requireCompleteTopologyFreezeV6WithRealisticGate(RequireCompleteRealisticFreeze)
}

func requireCompleteTopologyFreezeV6WithRealisticGate(realisticGate func() error) error {
	if realisticGate == nil {
		return fmt.Errorf("topology v6 realistic freeze gate is unavailable")
	}
	if err := realisticGate(); err != nil {
		return fmt.Errorf("topology v6 requires the complete realistic freeze: %w", err)
	}
	definition, err := CurrentTopologyExperimentV6()
	if err != nil {
		return err
	}
	bundle, err := CurrentTopologyFreezeV6()
	if err != nil {
		return err
	}
	if definition.Status != "frozen_reviewed" || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v6 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func RequiredTopologyV6CampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV6()
	return bundle.CampaignExecutorSHA256, err
}
func RequiredTopologyV6MainSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV6()
	return bundle.MainSHA256, err
}
func topologyFreezeV6SourcesValue() (topologyFreezeV6Sources, error) {
	var value topologyFreezeV6Sources
	_, err := CurrentTopologyFreezeV6()
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(topologyFreezeV6JSON, &value)
	return value, err
}
func RequiredTopologyV6OpenCodeProviderSHA256() (string, error) {
	value, err := topologyFreezeV6SourcesValue()
	return value.OpenCodeProviderSHA256, err
}
func RequiredTopologyV6OpenCodeRuntimeSHA256() (string, error) {
	value, err := topologyFreezeV6SourcesValue()
	return value.OpenCodeRuntimeSHA256, err
}
