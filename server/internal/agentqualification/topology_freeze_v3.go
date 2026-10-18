package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV3SchemaVersion = "silver-mediums.topology-freeze.v3"

//go:embed topology_freeze_v3.json
var topologyFreezeV3JSON []byte

//go:embed topology_experiment_v3.json
var topologyFreezeV3Experiment []byte

//go:embed topology_runner_v3.go
var topologyFreezeV3Runner []byte

//go:embed topology_review.go
var topologyFreezeV3Review []byte

//go:embed topology_campaign_v3.go
var topologyFreezeV3Campaign []byte

//go:embed topology_freeze_v2.json
var topologyFreezeV3PreservedV2 []byte

type topologyFreezeV3Sources struct {
	OpenCodeProviderSHA256 string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256  string `json:"opencode_runtime_sha256"`
	TopologyV2FreezeSHA256 string `json:"topology_v2_freeze_sha256"`
}

func CurrentTopologyFreezeV3() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV3JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV3(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV3(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV3SchemaVersion || bundle.BundleID != "realistic-topology-r03-repeat-v3" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v3 freeze identity or status is invalid")
	}
	components := map[string][]byte{bundle.ExperimentSHA256: topologyFreezeV3Experiment, bundle.RunnerSHA256: topologyFreezeV3Runner, bundle.ReviewSHA256: topologyFreezeV3Review, bundle.CampaignSHA256: topologyFreezeV3Campaign}
	if len(components) != 4 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) {
		return fmt.Errorf("topology v3 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("topology v3 freeze component hash mismatch")
		}
	}
	var sources topologyFreezeV3Sources
	if err := json.Unmarshal(topologyFreezeV3JSON, &sources); err != nil || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.TopologyV2FreezeSHA256) {
		return fmt.Errorf("topology v3 source hashes are incomplete")
	}
	v2Hash := sha256.Sum256(topologyFreezeV3PreservedV2)
	if hex.EncodeToString(v2Hash[:]) != sources.TopologyV2FreezeSHA256 {
		return fmt.Errorf("preserved topology-v2 freeze bytes drifted")
	}
	if _, err := CurrentTopologyFreezeV2(); err != nil {
		return fmt.Errorf("preserved topology-v2 components drifted: %w", err)
	}
	if !exactSet(bundle.ValidationMatrix, "exact_model_provider_effort", "unchanged_r03_v2_fixture_and_grader", "no_fallbacks", "no_transport_retries", "four_fresh_counterbalanced_observations", "reviewer_16000_total_cap", "reviewed_arm_covers_all_substages", "campaign_covers_all_arms", "paid_response_sealed_before_stop", "exact_receipt_accounting", "exclusive_claim", "crash_no_retry", "one_correction_ceiling", "v2_byte_preservation") {
		return fmt.Errorf("topology v3 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v3 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreezeV3() error {
	definition, err := CurrentTopologyExperimentV3()
	if err != nil {
		return err
	}
	bundle, err := CurrentTopologyFreezeV3()
	if err != nil {
		return err
	}
	if definition.Status != "frozen_reviewed" || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v3 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func RequiredTopologyV3CampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV3()
	return bundle.CampaignExecutorSHA256, err
}
func RequiredTopologyV3MainSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV3()
	return bundle.MainSHA256, err
}
func topologyFreezeV3SourcesValue() (topologyFreezeV3Sources, error) {
	var value topologyFreezeV3Sources
	_, err := CurrentTopologyFreezeV3()
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(topologyFreezeV3JSON, &value)
	return value, err
}
func RequiredTopologyV3OpenCodeProviderSHA256() (string, error) {
	value, err := topologyFreezeV3SourcesValue()
	return value.OpenCodeProviderSHA256, err
}
func RequiredTopologyV3OpenCodeRuntimeSHA256() (string, error) {
	value, err := topologyFreezeV3SourcesValue()
	return value.OpenCodeRuntimeSHA256, err
}
