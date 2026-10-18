package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV7SchemaVersion = "silver-mediums.topology-freeze.v7"

//go:embed topology_freeze_v7.json
var topologyFreezeV7JSON []byte

//go:embed topology_experiment_v7.json
var topologyFreezeV7Experiment []byte

//go:embed topology_runner_v7.go
var topologyFreezeV7Runner []byte

//go:embed topology_review_v7.go
var topologyFreezeV7Review []byte

//go:embed topology_campaign_v7.go
var topologyFreezeV7Campaign []byte

//go:embed topology_executor_plan_v7.go
var topologyFreezeV7ExecutorPlan []byte

//go:embed topology_executor_seal_v7.go
var topologyFreezeV7ExecutorSeal []byte

//go:embed topology_prevalidation_v7.go
var topologyFreezeV7Prevalidation []byte

//go:embed topology_freeze_v6.json
var topologyFreezeV7PreservedV6 []byte

type topologyFreezeV7Sources struct {
	OpenCodeProviderSHA256 string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256  string `json:"opencode_runtime_sha256"`
	ExecutorPlanSHA256     string `json:"executor_plan_sha256"`
	ExecutorSealSHA256     string `json:"executor_seal_sha256"`
	PrevalidationSHA256    string `json:"prevalidation_sha256"`
	TopologyV6FreezeSHA256 string `json:"topology_v6_freeze_sha256"`
}

func CurrentTopologyFreezeV7() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV7JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV7(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV7(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV7SchemaVersion || bundle.BundleID != "realistic-topology-r03-mercury-claude-v7" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v7 freeze identity or status is invalid")
	}
	components := map[string][]byte{bundle.ExperimentSHA256: topologyFreezeV7Experiment, bundle.RunnerSHA256: topologyFreezeV7Runner, bundle.ReviewSHA256: topologyFreezeV7Review, bundle.CampaignSHA256: topologyFreezeV7Campaign}
	if len(components) != 4 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) {
		return fmt.Errorf("topology v7 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("topology v7 freeze component hash mismatch")
		}
	}
	var sources topologyFreezeV7Sources
	if err := json.Unmarshal(topologyFreezeV7JSON, &sources); err != nil || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.ExecutorPlanSHA256) || !validSHA256(sources.ExecutorSealSHA256) || !validSHA256(sources.PrevalidationSHA256) || !validSHA256(sources.TopologyV6FreezeSHA256) {
		return fmt.Errorf("topology v7 source hashes are incomplete")
	}
	for expected, raw := range map[string][]byte{sources.ExecutorPlanSHA256: topologyFreezeV7ExecutorPlan, sources.ExecutorSealSHA256: topologyFreezeV7ExecutorSeal, sources.PrevalidationSHA256: topologyFreezeV7Prevalidation} {
		hash := sha256.Sum256(raw)
		if hex.EncodeToString(hash[:]) != expected {
			return fmt.Errorf("topology v7 generation-bound source drifted")
		}
	}
	v6Hash := sha256.Sum256(topologyFreezeV7PreservedV6)
	if hex.EncodeToString(v6Hash[:]) != sources.TopologyV6FreezeSHA256 {
		return fmt.Errorf("preserved topology-v6 freeze bytes drifted")
	}
	if _, err := CurrentTopologyFreezeV6(); err != nil {
		return fmt.Errorf("preserved topology-v6 components drifted: %w", err)
	}
	if !exactSet(bundle.ValidationMatrix, "a_executor_id_model_effort_provider_mutation_rejected", "b_plan_and_sealer_exact_same_executor", "c_reviewer_route_fallback_order_tools_instructions_exact", "d_preclaim_failure_zero_claim_zero_call", "e_zero_call_runtime_invalid_persisted_claim_cleared_continue", "f_malformed_zero_call_invalid_rejected", "g_v6_bytes_and_recursive_v6_v1_sources_preserved", "h_paid_partial_protocol_caps_and_recovery_preserved", "i_execute_and_correction_use_generation_bound_runner", "v7_stage_overcap_finish_and_nonfinish_exact", "v7_invalid_safety_hash_envelope", "complete_reviewer_plan_binding", "resolved_config_validated_without_transport_alias", "require_complete_realistic_freeze", "provider_source_hash_gates", "unchanged_r03_v2_fixture_and_grader", "four_fresh_counterbalanced_observations", "reviewer_16000_total_cap", "reviewed_arm_140000_covers_all_substages", "campaign_430000_total_cap", "incremental_spend_1300000_micros", "no_fallbacks", "no_transport_retries", "zero_owner_interventions", "testing_only_no_selection") {
		return fmt.Errorf("topology v7 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v7 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreezeV7() error {
	return requireCompleteTopologyFreezeV7WithRealisticGate(RequireCompleteRealisticFreeze)
}

func requireCompleteTopologyFreezeV7WithRealisticGate(realisticGate func() error) error {
	if realisticGate == nil {
		return fmt.Errorf("topology v7 realistic freeze gate is unavailable")
	}
	if err := realisticGate(); err != nil {
		return fmt.Errorf("topology v7 requires the complete realistic freeze: %w", err)
	}
	definition, err := CurrentTopologyExperimentV7()
	if err != nil {
		return err
	}
	bundle, err := CurrentTopologyFreezeV7()
	if err != nil {
		return err
	}
	if definition.Status != "frozen_reviewed" || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v7 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func RequiredTopologyV7CampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV7()
	return bundle.CampaignExecutorSHA256, err
}
func RequiredTopologyV7MainSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV7()
	return bundle.MainSHA256, err
}
func topologyFreezeV7SourcesValue() (topologyFreezeV7Sources, error) {
	var value topologyFreezeV7Sources
	_, err := CurrentTopologyFreezeV7()
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(topologyFreezeV7JSON, &value)
	return value, err
}
func RequiredTopologyV7OpenCodeProviderSHA256() (string, error) {
	value, err := topologyFreezeV7SourcesValue()
	return value.OpenCodeProviderSHA256, err
}
func RequiredTopologyV7OpenCodeRuntimeSHA256() (string, error) {
	value, err := topologyFreezeV7SourcesValue()
	return value.OpenCodeRuntimeSHA256, err
}
