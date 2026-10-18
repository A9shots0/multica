package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV8SchemaVersion = "silver-mediums.topology-freeze.v8"

//go:embed topology_freeze_v8.json
var topologyFreezeV8JSON []byte

//go:embed topology_experiment_v8.json
var topologyFreezeV8Experiment []byte

//go:embed topology_runner_v8.go
var topologyFreezeV8Runner []byte

//go:embed topology_review_v8.go
var topologyFreezeV8Review []byte

//go:embed topology_campaign_v8.go
var topologyFreezeV8Campaign []byte

//go:embed topology_executor_plan_v8.go
var topologyFreezeV8ExecutorPlan []byte

//go:embed topology_executor_seal_v8.go
var topologyFreezeV8ExecutorSeal []byte

//go:embed topology_prevalidation_v8.go
var topologyFreezeV8Prevalidation []byte

//go:embed topology_freeze_v7.json
var topologyFreezeV8PreservedV7 []byte

//go:embed realistic_workflow_group_v1.json
var topologyFreezeV8RealisticGroup []byte

//go:embed realistic_fixture_manifest_v1.json
var topologyFreezeV8FixtureManifest []byte

//go:embed realistic_controller.go
var topologyFreezeV8Controller []byte

//go:embed realistic_verifier.go
var topologyFreezeV8Grader []byte

//go:embed realistic_declarations.go
var topologyFreezeV8Declarations []byte

//go:embed realistic_execution.go
var topologyFreezeV8Accounting []byte

//go:embed opencode_transport.go
var topologyFreezeV8Transport []byte

//go:embed opencode_receipt.go
var topologyFreezeV8ReceiptParser []byte

type topologyFreezeV8Sources struct {
	OpenCodeProviderSHA256     string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256      string `json:"opencode_runtime_sha256"`
	CurrentScreeningMainSHA256 string `json:"current_screening_main_sha256"`
	ExecutorPlanSHA256         string `json:"executor_plan_sha256"`
	ExecutorSealSHA256         string `json:"executor_seal_sha256"`
	PrevalidationSHA256        string `json:"prevalidation_sha256"`
	TopologyV7FreezeSHA256     string `json:"topology_v7_freeze_sha256"`
	RealisticGroupSHA256       string `json:"realistic_group_sha256"`
	FixtureManifestSHA256      string `json:"fixture_manifest_sha256"`
	ControllerSHA256           string `json:"controller_sha256"`
	GraderSHA256               string `json:"grader_sha256"`
	DeclarationsSHA256         string `json:"declarations_sha256"`
	AccountingSHA256           string `json:"accounting_sha256"`
	TransportSHA256            string `json:"transport_sha256"`
	ReceiptParserSHA256        string `json:"receipt_parser_sha256"`
}

func CurrentTopologyFreezeV8() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV8JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV8(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV8(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV8SchemaVersion || bundle.BundleID != "realistic-topology-r04-gpt54-claude-v8" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v8 freeze identity or status is invalid")
	}
	var sources topologyFreezeV8Sources
	if err := json.Unmarshal(topologyFreezeV8JSON, &sources); err != nil {
		return err
	}
	components := map[string][]byte{
		bundle.ExperimentSHA256: topologyFreezeV8Experiment, bundle.RunnerSHA256: topologyFreezeV8Runner, bundle.ReviewSHA256: topologyFreezeV8Review, bundle.CampaignSHA256: topologyFreezeV8Campaign,
		sources.ExecutorPlanSHA256: topologyFreezeV8ExecutorPlan, sources.ExecutorSealSHA256: topologyFreezeV8ExecutorSeal, sources.PrevalidationSHA256: topologyFreezeV8Prevalidation, sources.TopologyV7FreezeSHA256: topologyFreezeV8PreservedV7,
		sources.RealisticGroupSHA256: topologyFreezeV8RealisticGroup, sources.FixtureManifestSHA256: topologyFreezeV8FixtureManifest, sources.ControllerSHA256: topologyFreezeV8Controller, sources.GraderSHA256: topologyFreezeV8Grader, sources.DeclarationsSHA256: topologyFreezeV8Declarations, sources.AccountingSHA256: topologyFreezeV8Accounting, sources.TransportSHA256: topologyFreezeV8Transport, sources.ReceiptParserSHA256: topologyFreezeV8ReceiptParser,
	}
	if len(components) != 16 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.CurrentScreeningMainSHA256) {
		return fmt.Errorf("topology v8 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		h := sha256.Sum256(raw)
		if expected != hex.EncodeToString(h[:]) {
			return fmt.Errorf("topology v8 freeze component hash mismatch")
		}
	}
	if _, err := CurrentTopologyFreezeV7(); err != nil {
		return fmt.Errorf("preserved topology-v7 through v1 components drifted: %w", err)
	}
	want := []string{"exact_gpt54_executor_route", "exact_claude_reviewer_route", "unchanged_r04_v1_fixture_task_grader_declarations", "forced_attempt1_zero_call_checkpoint", "same_family_attempt2_resume", "correction_attempt2_without_interruption_replay", "two_counterbalanced_pairs", "execute_15_60000_12000_250000", "review_each_1_20000_5000_100000", "correction_8_40000_10000_200000", "reviewed_arm_25_140000_32000_600000", "campaign_80_400000_1700000", "stage_exhaustion_observation_local_invalid", "cost_checkpoint_before_subsequent_execute_correction_call", "durable_pending_review_crash_accounting", "global_halt_exact_allowlist", "no_fallbacks_retries_owner", "generation_owned_routes_sealer_runner", "provider_free_prevalidation", "recursive_v7_v1_preservation", "pending_provider_lock", "testing_only_no_selection"}
	if !exactSet(bundle.ValidationMatrix, want...) {
		return fmt.Errorf("topology v8 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v8 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreezeV8() error {
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return fmt.Errorf("topology v8 requires complete realistic freeze: %w", err)
	}
	d, err := CurrentTopologyExperimentV8()
	if err != nil {
		return err
	}
	b, err := CurrentTopologyFreezeV8()
	if err != nil {
		return err
	}
	if d.Status != "frozen_reviewed" || b.Status != "frozen_reviewed" || !b.ProviderCallsAllowed {
		return fmt.Errorf("topology v8 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func topologyFreezeV8SourcesValue() (topologyFreezeV8Sources, error) {
	var v topologyFreezeV8Sources
	_, err := CurrentTopologyFreezeV8()
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(topologyFreezeV8JSON, &v)
	return v, err
}
func RequiredTopologyV8CampaignExecutorSHA256() (string, error) {
	b, e := CurrentTopologyFreezeV8()
	return b.CampaignExecutorSHA256, e
}
func RequiredTopologyV8MainSHA256() (string, error) {
	b, e := CurrentTopologyFreezeV8()
	return b.MainSHA256, e
}
func RequiredTopologyV8OpenCodeProviderSHA256() (string, error) {
	v, e := topologyFreezeV8SourcesValue()
	return v.OpenCodeProviderSHA256, e
}
func RequiredTopologyV8OpenCodeRuntimeSHA256() (string, error) {
	v, e := topologyFreezeV8SourcesValue()
	return v.OpenCodeRuntimeSHA256, e
}
func RequiredTopologyV8CurrentScreeningMainSHA256() (string, error) {
	v, e := topologyFreezeV8SourcesValue()
	return v.CurrentScreeningMainSHA256, e
}
