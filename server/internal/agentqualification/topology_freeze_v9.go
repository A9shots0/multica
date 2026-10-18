package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TopologyFreezeV9SchemaVersion = "silver-mediums.topology-freeze.v9"

//go:embed topology_freeze_v9.json
var topologyFreezeV9JSON []byte

//go:embed topology_experiment_v9.json
var topologyFreezeV9Experiment []byte

//go:embed topology_runner_v9.go
var topologyFreezeV9Runner []byte

//go:embed topology_review_v9.go
var topologyFreezeV9Review []byte

//go:embed topology_campaign_v9.go
var topologyFreezeV9Campaign []byte

//go:embed topology_executor_plan_v9.go
var topologyFreezeV9ExecutorPlan []byte

//go:embed topology_executor_seal_v9.go
var topologyFreezeV9ExecutorSeal []byte

//go:embed topology_prevalidation_v9.go
var topologyFreezeV9Prevalidation []byte

//go:embed topology_freeze_v8.json
var topologyFreezeV9PreservedV8 []byte

//go:embed realistic_workflow_group_v1.json
var topologyFreezeV9RealisticGroup []byte

//go:embed realistic_fixture_manifest_v1.json
var topologyFreezeV9FixtureManifest []byte

//go:embed realistic_controller.go
var topologyFreezeV9Controller []byte

//go:embed realistic_verifier.go
var topologyFreezeV9Grader []byte

//go:embed realistic_declarations.go
var topologyFreezeV9Declarations []byte

//go:embed realistic_execution.go
var topologyFreezeV9Accounting []byte

//go:embed opencode_transport.go
var topologyFreezeV9Transport []byte

//go:embed opencode_receipt.go
var topologyFreezeV9ReceiptParser []byte

type topologyFreezeV9Sources struct {
	OpenCodeProviderSHA256     string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256      string `json:"opencode_runtime_sha256"`
	CurrentScreeningMainSHA256 string `json:"current_screening_main_sha256"`
	ExecutorPlanSHA256         string `json:"executor_plan_sha256"`
	ExecutorSealSHA256         string `json:"executor_seal_sha256"`
	PrevalidationSHA256        string `json:"prevalidation_sha256"`
	TopologyV8FreezeSHA256     string `json:"topology_v8_freeze_sha256"`
	RealisticGroupSHA256       string `json:"realistic_group_sha256"`
	FixtureManifestSHA256      string `json:"fixture_manifest_sha256"`
	ControllerSHA256           string `json:"controller_sha256"`
	GraderSHA256               string `json:"grader_sha256"`
	DeclarationsSHA256         string `json:"declarations_sha256"`
	AccountingSHA256           string `json:"accounting_sha256"`
	TransportSHA256            string `json:"transport_sha256"`
	ReceiptParserSHA256        string `json:"receipt_parser_sha256"`
}

func CurrentTopologyFreezeV9() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(topologyFreezeV9JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateTopologyFreezeV9(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateTopologyFreezeV9(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != TopologyFreezeV9SchemaVersion || bundle.BundleID != "realistic-topology-r04-claude-gpt54-v9" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("topology v9 freeze identity or status is invalid")
	}
	var sources topologyFreezeV9Sources
	if err := json.Unmarshal(topologyFreezeV9JSON, &sources); err != nil {
		return err
	}
	components := map[string][]byte{
		bundle.ExperimentSHA256: topologyFreezeV9Experiment, bundle.RunnerSHA256: topologyFreezeV9Runner, bundle.ReviewSHA256: topologyFreezeV9Review, bundle.CampaignSHA256: topologyFreezeV9Campaign,
		sources.ExecutorPlanSHA256: topologyFreezeV9ExecutorPlan, sources.ExecutorSealSHA256: topologyFreezeV9ExecutorSeal, sources.PrevalidationSHA256: topologyFreezeV9Prevalidation, sources.TopologyV8FreezeSHA256: topologyFreezeV9PreservedV8,
		sources.RealisticGroupSHA256: topologyFreezeV9RealisticGroup, sources.FixtureManifestSHA256: topologyFreezeV9FixtureManifest, sources.ControllerSHA256: topologyFreezeV9Controller, sources.GraderSHA256: topologyFreezeV9Grader, sources.DeclarationsSHA256: topologyFreezeV9Declarations, sources.AccountingSHA256: topologyFreezeV9Accounting, sources.TransportSHA256: topologyFreezeV9Transport, sources.ReceiptParserSHA256: topologyFreezeV9ReceiptParser,
	}
	if len(components) != 16 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) || !validSHA256(sources.OpenCodeProviderSHA256) || !validSHA256(sources.OpenCodeRuntimeSHA256) || !validSHA256(sources.CurrentScreeningMainSHA256) {
		return fmt.Errorf("topology v9 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		h := sha256.Sum256(raw)
		if expected != hex.EncodeToString(h[:]) {
			return fmt.Errorf("topology v9 freeze component hash mismatch")
		}
	}
	if _, err := CurrentTopologyFreezeV8(); err != nil {
		return fmt.Errorf("preserved topology-v8 through v1 components drifted: %w", err)
	}
	want := []string{"exact_claude_executor_route", "exact_gpt54_reviewer_route", "unchanged_r04_v1_fixture_task_grader_declarations", "forced_attempt1_zero_call_checkpoint", "same_family_attempt2_resume", "correction_attempt2_without_interruption_replay", "two_counterbalanced_pairs", "execute_15_60000_12000_250000", "review_each_1_20000_5000_100000", "correction_8_40000_10000_200000", "reviewed_arm_25_140000_32000_600000", "campaign_80_400000_1700000", "stage_exhaustion_observation_local_invalid", "cost_checkpoint_before_subsequent_execute_correction_call", "durable_pending_review_crash_accounting", "global_halt_exact_allowlist", "no_fallbacks_retries_owner", "generation_owned_routes_sealer_runner", "provider_free_prevalidation", "recursive_v8_v1_preservation", "one_independent_review_one_consolidated_correction_atomic_freeze", "testing_only_no_selection"}
	if !exactSet(bundle.ValidationMatrix, want...) {
		return fmt.Errorf("topology v9 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("topology v9 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteTopologyFreezeV9() error {
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return fmt.Errorf("topology v9 requires complete realistic freeze: %w", err)
	}
	d, err := CurrentTopologyExperimentV9()
	if err != nil {
		return err
	}
	b, err := CurrentTopologyFreezeV9()
	if err != nil {
		return err
	}
	if d.Status != "frozen_reviewed" || b.Status != "frozen_reviewed" || !b.ProviderCallsAllowed {
		return fmt.Errorf("topology v9 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func topologyFreezeV9SourcesValue() (topologyFreezeV9Sources, error) {
	var v topologyFreezeV9Sources
	_, err := CurrentTopologyFreezeV9()
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(topologyFreezeV9JSON, &v)
	return v, err
}
func RequiredTopologyV9CampaignExecutorSHA256() (string, error) {
	b, e := CurrentTopologyFreezeV9()
	return b.CampaignExecutorSHA256, e
}
func RequiredTopologyV9MainSHA256() (string, error) {
	b, e := CurrentTopologyFreezeV9()
	return b.MainSHA256, e
}
func RequiredTopologyV9OpenCodeProviderSHA256() (string, error) {
	v, e := topologyFreezeV9SourcesValue()
	return v.OpenCodeProviderSHA256, e
}
func RequiredTopologyV9OpenCodeRuntimeSHA256() (string, error) {
	v, e := topologyFreezeV9SourcesValue()
	return v.OpenCodeRuntimeSHA256, e
}
func RequiredTopologyV9CurrentScreeningMainSHA256() (string, error) {
	v, e := topologyFreezeV9SourcesValue()
	return v.CurrentScreeningMainSHA256, e
}
