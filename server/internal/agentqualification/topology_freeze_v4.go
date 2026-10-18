package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV4SchemaVersion = "silver-mediums.topology-freeze.v4"

//go:embed topology_freeze_v4.json
var topologyFreezeV4JSON []byte

//go:embed topology_experiment_v4.json
var topologyFreezeV4Experiment []byte

//go:embed topology_runner_v4.go
var topologyFreezeV4Runner []byte

//go:embed topology_review.go
var topologyFreezeV4Review []byte

//go:embed topology_campaign_v4.go
var topologyFreezeV4Campaign []byte

//go:embed topology_freeze_v3.json
var topologyFreezeV4PreservedV3 []byte

type topologyFreezeV4Sources struct {
	OpenCodeProviderSHA256 string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256  string `json:"opencode_runtime_sha256"`
	TopologyV3FreezeSHA256 string `json:"topology_v3_freeze_sha256"`
}

func CurrentTopologyFreezeV4() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV4JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV4(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV4(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV4SchemaVersion || bundle.BundleID != "realistic-topology-r03-gpt54-repeat-v4" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v4 freeze identity or status is invalid")
	}
	components := map[string][]byte{bundle.ExperimentSHA256: topologyFreezeV4Experiment, bundle.RunnerSHA256: topologyFreezeV4Runner, bundle.ReviewSHA256: topologyFreezeV4Review, bundle.CampaignSHA256: topologyFreezeV4Campaign}
	if len(components) != 4 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) {
		return fmt.Errorf("topology v4 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("topology v4 freeze component hash mismatch")
		}
	}
	var sources topologyFreezeV4Sources
	if err := json.Unmarshal(topologyFreezeV4JSON, &sources); err != nil || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.TopologyV3FreezeSHA256) {
		return fmt.Errorf("topology v4 source hashes are incomplete")
	}
	v3Hash := sha256.Sum256(topologyFreezeV4PreservedV3)
	if hex.EncodeToString(v3Hash[:]) != sources.TopologyV3FreezeSHA256 {
		return fmt.Errorf("preserved topology-v3 freeze bytes drifted")
	}
	if _, err := CurrentTopologyFreezeV3(); err != nil {
		return fmt.Errorf("preserved topology-v3 components drifted: %w", err)
	}
	if !exactSet(bundle.ValidationMatrix, "exact_gpt54_openai_medium", "fresh_same_model_reviewer", "same_tool_free_review_contract", "unchanged_r03_v2_fixture_and_grader", "no_fallbacks", "no_transport_retries", "local_only_no_external_side_effects", "four_fresh_counterbalanced_observations", "reviewer_16000_total_cap", "reviewed_arm_covers_all_substages", "campaign_410000_total_cap", "incremental_spend_below_2_usd", "paid_response_sealed_before_stop", "exact_receipt_accounting", "exclusive_claim", "crash_no_retry", "one_correction_ceiling", "v3_byte_preservation", "recursive_v3_v2_v1_command_source_admission") {
		return fmt.Errorf("topology v4 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v4 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreezeV4() error {
	definition, err := CurrentTopologyExperimentV4()
	if err != nil {
		return err
	}
	bundle, err := CurrentTopologyFreezeV4()
	if err != nil {
		return err
	}
	if definition.Status != "frozen_reviewed" || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v4 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func RequiredTopologyV4CampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV4()
	return bundle.CampaignExecutorSHA256, err
}
func RequiredTopologyV4MainSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV4()
	return bundle.MainSHA256, err
}
func topologyFreezeV4SourcesValue() (topologyFreezeV4Sources, error) {
	var value topologyFreezeV4Sources
	_, err := CurrentTopologyFreezeV4()
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(topologyFreezeV4JSON, &value)
	return value, err
}
func RequiredTopologyV4OpenCodeProviderSHA256() (string, error) {
	value, err := topologyFreezeV4SourcesValue()
	return value.OpenCodeProviderSHA256, err
}
func RequiredTopologyV4OpenCodeRuntimeSHA256() (string, error) {
	value, err := topologyFreezeV4SourcesValue()
	return value.OpenCodeRuntimeSHA256, err
}
