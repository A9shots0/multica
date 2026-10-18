package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeSchemaVersion = "silver-mediums.topology-freeze.v1"

type TopologyFreezeBundle struct {
	SchemaVersion          string   `json:"schema_version"`
	BundleID               string   `json:"bundle_id"`
	Status                 string   `json:"status"`
	ExperimentSHA256       string   `json:"experiment_sha256"`
	RunnerSHA256           string   `json:"runner_sha256"`
	ReviewSHA256           string   `json:"review_sha256"`
	CampaignSHA256         string   `json:"campaign_sha256"`
	CampaignExecutorSHA256 string   `json:"campaign_executor_sha256"`
	MainSHA256             string   `json:"main_sha256"`
	ValidationMatrix       []string `json:"validation_matrix"`
	ProviderCallsAllowed   bool     `json:"provider_calls_allowed"`
}

//go:embed topology_freeze_v1.json
var topologyFreezeJSON []byte

//go:embed topology_experiment_v1.json
var topologyFreezeExperimentJSON []byte

//go:embed topology_runner.go
var topologyFreezeRunnerSource []byte

//go:embed topology_review.go
var topologyFreezeReviewSource []byte

//go:embed topology_campaign.go
var topologyFreezeCampaignSource []byte

func CurrentTopologyFreezeBundle() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeJSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeBundle(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeBundle(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeSchemaVersion || bundle.BundleID != "realistic-topology-comparison-v1" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology freeze identity or status is invalid")
	}
	components := map[string][]byte{bundle.ExperimentSHA256: topologyFreezeExperimentJSON, bundle.RunnerSHA256: topologyFreezeRunnerSource, bundle.ReviewSHA256: topologyFreezeReviewSource, bundle.CampaignSHA256: topologyFreezeCampaignSource}
	if len(components) != 4 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) {
		return fmt.Errorf("topology freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("topology freeze component hash mismatch")
		}
	}
	if !exactSet(bundle.ValidationMatrix, "strong_pass", "semantic_paraphrase", "subtle_failure", "opaque_evidence", "missing_required_evidence", "fabricated_evidence", "scope_expansion", "owner_relay", "unbounded_correction", "pair_hash_mismatch", "stage_budget_overrun", "crash_stranded_claim", "candidate_local_failure_continues") {
		return fmt.Errorf("topology freeze validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreeze() error {
	definition, err := CurrentTopologyExperiment()
	if err != nil {
		return err
	}
	bundle, err := CurrentTopologyFreezeBundle()
	if err != nil {
		return err
	}
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return err
	}
	if definition.Status != "frozen_reviewed" || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology provider execution is locked until the complete group is independently reviewed and atomically frozen")
	}
	return nil
}

func RequiredTopologyCampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeBundle()
	return bundle.CampaignExecutorSHA256, err
}

func RequiredTopologyMainSHA256() (string, error) {
	bundle, err := CurrentTopologyFreezeBundle()
	return bundle.MainSHA256, err
}
