package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV2SchemaVersion = "silver-mediums.topology-freeze.v2"

//go:embed topology_freeze_v2.json
var topologyFreezeV2JSON []byte

//go:embed topology_experiment_v2.json
var topologyFreezeV2Experiment []byte

//go:embed topology_runner_v2.go
var topologyFreezeV2Runner []byte

//go:embed topology_review.go
var topologyFreezeV2Review []byte

//go:embed topology_campaign_v2.go
var topologyFreezeV2Campaign []byte

//go:embed topology_freeze_v1.json
var topologyFreezeV2PreservedV1 []byte

func CurrentTopologyFreezeV2() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV2JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV2(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV2(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV2SchemaVersion || bundle.BundleID != "realistic-topology-r03-repeat-v2" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v2 freeze identity or status is invalid")
	}
	components := map[string][]byte{bundle.ExperimentSHA256: topologyFreezeV2Experiment, bundle.RunnerSHA256: topologyFreezeV2Runner, bundle.ReviewSHA256: topologyFreezeV2Review, bundle.CampaignSHA256: topologyFreezeV2Campaign}
	if len(components) != 4 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) {
		return fmt.Errorf("topology v2 freeze hashes are incomplete or duplicated")
	}
	var sources struct {
		OpenCodeProviderSHA256 string `json:"opencode_provider_sha256"`
		OpenCodeRuntimeSHA256  string `json:"opencode_runtime_sha256"`
		TopologyV1FreezeSHA256 string `json:"topology_v1_freeze_sha256"`
	}
	if err := json.Unmarshal(topologyFreezeV2JSON, &sources); err != nil || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.TopologyV1FreezeSHA256) {
		return fmt.Errorf("topology v2 transport source hashes are incomplete")
	}
	v1Hash := sha256.Sum256(topologyFreezeV2PreservedV1)
	if hex.EncodeToString(v1Hash[:]) != sources.TopologyV1FreezeSHA256 {
		return fmt.Errorf("preserved topology-v1 freeze bytes drifted")
	}
	if _, err := CurrentTopologyFreezeBundle(); err != nil {
		return fmt.Errorf("preserved topology-v1 components drifted: %w", err)
	}
	for expected, raw := range components {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("topology v2 freeze component hash mismatch")
		}
	}
	if !exactSet(bundle.ValidationMatrix, "exact_model_provider_effort", "no_fallbacks", "no_transport_retries", "two_fresh_matched_pairs", "counterbalanced_order", "pair_hash_mismatch", "paid_response_sealed_before_stop", "exact_receipt_accounting", "exclusive_claim", "crash_no_retry", "reviewer_correction_ceiling", "v1_byte_preservation") {
		return fmt.Errorf("topology v2 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v2 provider admission does not match review status")
	}
	return nil
}

func RequiredTopologyV2OpenCodeProviderSHA256() (string, error) {
	if _, err := CurrentTopologyFreezeV2(); err != nil {
		return "", err
	}
	var value struct {
		SHA256 string `json:"opencode_provider_sha256"`
	}
	if err := json.Unmarshal(topologyFreezeV2JSON, &value); err != nil {
		return "", err
	}
	return value.SHA256, nil
}

func RequiredTopologyV2OpenCodeRuntimeSHA256() (string, error) {
	if _, err := CurrentTopologyFreezeV2(); err != nil {
		return "", err
	}
	var value struct {
		SHA256 string `json:"opencode_runtime_sha256"`
	}
	if err := json.Unmarshal(topologyFreezeV2JSON, &value); err != nil {
		return "", err
	}
	return value.SHA256, nil
}

func RequireCompleteTopologyFreezeV2() error {
	definition, err := CurrentTopologyExperimentV2()
	if err != nil {
		return err
	}
	bundle, err := CurrentTopologyFreezeV2()
	if err != nil {
		return err
	}
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return err
	}
	if definition.Status != "frozen_reviewed" || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v2 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func RequiredTopologyV2CampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV2()
	if err != nil {
		return "", err
	}
	return bundle.CampaignExecutorSHA256, nil
}

func RequiredTopologyV2MainSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeV2()
	if err != nil {
		return "", err
	}
	return bundle.MainSHA256, nil
}
