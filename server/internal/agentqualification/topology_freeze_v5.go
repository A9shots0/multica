package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV5SchemaVersion = "silver-mediums.topology-freeze.v5"

//go:embed topology_freeze_v5.json
var topologyFreezeV5JSON []byte

//go:embed topology_experiment_v5.json
var topologyFreezeV5Experiment []byte

//go:embed topology_runner_v5.go
var topologyFreezeV5Runner []byte

//go:embed topology_review_v5.go
var topologyFreezeV5Review []byte

//go:embed topology_campaign_v5.go
var topologyFreezeV5Campaign []byte

//go:embed topology_freeze_v4.json
var topologyFreezeV5PreservedV4 []byte

type topologyFreezeV5Sources struct {
	OpenCodeProviderSHA256 string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256  string `json:"opencode_runtime_sha256"`
	TopologyV4FreezeSHA256 string `json:"topology_v4_freeze_sha256"`
}

func CurrentTopologyFreezeV5() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV5JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV5(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV5(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV5SchemaVersion || bundle.BundleID != "realistic-topology-r03-gpt54-repeat-v5" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v5 freeze identity or status is invalid")
	}
	components := map[string][]byte{bundle.ExperimentSHA256: topologyFreezeV5Experiment, bundle.RunnerSHA256: topologyFreezeV5Runner, bundle.ReviewSHA256: topologyFreezeV5Review, bundle.CampaignSHA256: topologyFreezeV5Campaign}
	if len(components) != 4 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) {
		return fmt.Errorf("topology v5 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("topology v5 freeze component hash mismatch")
		}
	}
	var sources topologyFreezeV5Sources
	if err := json.Unmarshal(topologyFreezeV5JSON, &sources); err != nil || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.TopologyV4FreezeSHA256) {
		return fmt.Errorf("topology v5 source hashes are incomplete")
	}
	v4Hash := sha256.Sum256(topologyFreezeV5PreservedV4)
	if hex.EncodeToString(v4Hash[:]) != sources.TopologyV4FreezeSHA256 {
		return fmt.Errorf("preserved topology-v4 freeze bytes drifted")
	}
	if _, err := CurrentTopologyFreezeV4(); err != nil {
		return fmt.Errorf("preserved topology-v4 components drifted: %w", err)
	}
	if !exactSet(bundle.ValidationMatrix, "exact_gpt54_openai_medium", "fresh_same_model_reviewer", "same_tool_free_review_contract", "generation_bound_reviewer_validation", "executor_protocol_failure_candidate_local", "correction_protocol_failure_candidate_local", "provider_budget_breach_global_halt", "evidence_corruption_global_halt", "unchanged_r03_v2_fixture_and_grader", "no_fallbacks", "no_transport_retries", "local_only_no_external_side_effects", "four_fresh_counterbalanced_observations", "reviewer_16000_total_cap", "reviewed_arm_covers_all_substages", "campaign_410000_total_cap", "incremental_spend_below_2_usd", "paid_response_sealed_before_stop", "exact_receipt_accounting", "exclusive_claim", "crash_no_retry", "one_correction_ceiling", "v4_byte_preservation", "recursive_v4_v3_v2_v1_command_source_admission") {
		return fmt.Errorf("topology v5 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v5 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreezeV5() error {
	definition, err := CurrentTopologyExperimentV5()
	if err != nil {
		return err
	}
	bundle, err := CurrentTopologyFreezeV5()
	if err != nil {
		return err
	}
	if definition.Status != "frozen_reviewed" || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v5 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func RequiredTopologyV5CampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV5()
	return bundle.CampaignExecutorSHA256, err
}
func RequiredTopologyV5MainSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV5()
	return bundle.MainSHA256, err
}
func topologyFreezeV5SourcesValue() (topologyFreezeV5Sources, error) {
	var value topologyFreezeV5Sources
	_, err := CurrentTopologyFreezeV5()
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(topologyFreezeV5JSON, &value)
	return value, err
}
func RequiredTopologyV5OpenCodeProviderSHA256() (string, error) {
	value, err := topologyFreezeV5SourcesValue()
	return value.OpenCodeProviderSHA256, err
}
func RequiredTopologyV5OpenCodeRuntimeSHA256() (string, error) {
	value, err := topologyFreezeV5SourcesValue()
	return value.OpenCodeRuntimeSHA256, err
}
