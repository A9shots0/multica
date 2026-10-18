package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const R04EffortFreezeV2SchemaVersion = "silver-mediums.r04-effort-freeze.v2"

//go:embed r04_effort_freeze_v2.json
var r04EffortFreezeV2JSON []byte

//go:embed r04_effort_experiment_v2.json
var r04EffortFreezeV2Experiment []byte

//go:embed r04_effort_runner_v2.go
var r04EffortFreezeV2Runner []byte

//go:embed r04_effort_no_review_v2.go
var r04EffortFreezeV2NoReview []byte

//go:embed r04_effort_campaign_v2.go
var r04EffortFreezeV2Campaign []byte

//go:embed r04_effort_executor_plan_v2.go
var r04EffortFreezeV2ExecutorPlan []byte

//go:embed r04_effort_executor_seal_v2.go
var r04EffortFreezeV2ExecutorSeal []byte

//go:embed r04_effort_prevalidation_v2.go
var r04EffortFreezeV2Prevalidation []byte

//go:embed r04_effort_freeze_v1.json
var r04EffortFreezeV2PreservedEffortV1 []byte

//go:embed topology_freeze_v9.json
var r04EffortFreezeV2PreservedV9 []byte

//go:embed realistic_workflow_group_v1.json
var r04EffortFreezeV2RealisticGroup []byte

//go:embed realistic_fixture_manifest_v1.json
var r04EffortFreezeV2FixtureManifest []byte

//go:embed realistic_controller.go
var r04EffortFreezeV2Controller []byte

//go:embed realistic_verifier.go
var r04EffortFreezeV2Grader []byte

//go:embed realistic_declarations.go
var r04EffortFreezeV2Declarations []byte

//go:embed realistic_execution.go
var r04EffortFreezeV2Accounting []byte

//go:embed opencode_transport.go
var r04EffortFreezeV2Transport []byte

//go:embed opencode_receipt.go
var r04EffortFreezeV2ReceiptParser []byte

type r04EffortFreezeV2Sources struct {
	OpenCodeProviderSHA256     string `json:"opencode_provider_sha256"`
	OpenCodeRuntimeSHA256      string `json:"opencode_runtime_sha256"`
	CurrentScreeningMainSHA256 string `json:"current_screening_main_sha256"`
	ExecutorPlanSHA256         string `json:"executor_plan_sha256"`
	ExecutorSealSHA256         string `json:"executor_seal_sha256"`
	PrevalidationSHA256        string `json:"prevalidation_sha256"`
	R04EffortV1FreezeSHA256    string `json:"r04_effort_v1_freeze_sha256"`
	TopologyV9FreezeSHA256     string `json:"topology_v9_freeze_sha256"`
	RealisticGroupSHA256       string `json:"realistic_group_sha256"`
	FixtureManifestSHA256      string `json:"fixture_manifest_sha256"`
	ControllerSHA256           string `json:"controller_sha256"`
	GraderSHA256               string `json:"grader_sha256"`
	DeclarationsSHA256         string `json:"declarations_sha256"`
	AccountingSHA256           string `json:"accounting_sha256"`
	TransportSHA256            string `json:"transport_sha256"`
	ReceiptParserSHA256        string `json:"receipt_parser_sha256"`
}

func CurrentR04EffortFreezeV2() (TopologyFreezeBundle, error) {
	var bundle TopologyFreezeBundle
	if err := json.Unmarshal(r04EffortFreezeV2JSON, &bundle); err != nil {
		return bundle, err
	}
	if err := ValidateR04EffortFreezeV2(bundle); err != nil {
		return TopologyFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateR04EffortFreezeV2(bundle TopologyFreezeBundle) error {
	if bundle.SchemaVersion != R04EffortFreezeV2SchemaVersion || bundle.BundleID != "realistic-r04-gpt54-high-effort-v2" || bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("R04 effort v1 freeze identity or status is invalid")
	}
	var s r04EffortFreezeV2Sources
	if err := json.Unmarshal(r04EffortFreezeV2JSON, &s); err != nil {
		return err
	}
	components := map[string][]byte{
		bundle.ExperimentSHA256: r04EffortFreezeV2Experiment, bundle.RunnerSHA256: r04EffortFreezeV2Runner, bundle.ReviewSHA256: r04EffortFreezeV2NoReview, bundle.CampaignSHA256: r04EffortFreezeV2Campaign,
		s.ExecutorPlanSHA256: r04EffortFreezeV2ExecutorPlan, s.ExecutorSealSHA256: r04EffortFreezeV2ExecutorSeal, s.PrevalidationSHA256: r04EffortFreezeV2Prevalidation, s.R04EffortV1FreezeSHA256: r04EffortFreezeV2PreservedEffortV1, s.TopologyV9FreezeSHA256: r04EffortFreezeV2PreservedV9,
		s.RealisticGroupSHA256: r04EffortFreezeV2RealisticGroup, s.FixtureManifestSHA256: r04EffortFreezeV2FixtureManifest, s.ControllerSHA256: r04EffortFreezeV2Controller, s.GraderSHA256: r04EffortFreezeV2Grader, s.DeclarationsSHA256: r04EffortFreezeV2Declarations, s.AccountingSHA256: r04EffortFreezeV2Accounting, s.TransportSHA256: r04EffortFreezeV2Transport, s.ReceiptParserSHA256: r04EffortFreezeV2ReceiptParser,
	}
	if len(components) != 17 || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.MainSHA256) || !validSHA256(s.OpenCodeProviderSHA256) || !validSHA256(s.OpenCodeRuntimeSHA256) || !validSHA256(s.CurrentScreeningMainSHA256) {
		return fmt.Errorf("R04 effort v1 freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range components {
		h := sha256.Sum256(raw)
		if expected != hex.EncodeToString(h[:]) {
			return fmt.Errorf("R04 effort v1 freeze component hash mismatch")
		}
	}
	if _, err := CurrentTopologyFreezeV9(); err != nil {
		return fmt.Errorf("preserved topology-v9 through v1 components drifted: %w", err)
	}
	if _, err := CurrentR04EffortFreezeV1(); err != nil {
		return fmt.Errorf("preserved R04 effort v1 freeze drifted: %w", err)
	}
	want := []string{"exact_gpt54_high_executor_route", "unchanged_r04_v1_fixture_task_grader_declarations", "forced_attempt1_zero_call_checkpoint", "same_family_attempt2_resume", "two_independent_isolated_repeats", "direct_only_no_review_correction_retry", "observation_15_75000_30000_350000", "observation_timeout_8_minutes", "campaign_30_150000_700000", "candidate_failure_local", "nonsafety_harness_transport_stage_recovery_local_invalid", "global_halt_exact_allowlist", "no_fallbacks_owner_browser_external", "provider_free_prevalidation", "recursive_v9_v1_preservation", "pending_provider_lock", "testing_only_no_selection"}
	if !exactSet(bundle.ValidationMatrix, want...) {
		return fmt.Errorf("R04 effort v1 validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed || bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("R04 effort v1 provider admission does not match review status")
	}
	return nil
}

func RequireCompleteR04EffortFreezeV2() error {
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return fmt.Errorf("R04 effort v1 requires complete realistic freeze: %w", err)
	}
	d, err := CurrentR04EffortExperimentV2()
	if err != nil {
		return err
	}
	b, err := CurrentR04EffortFreezeV2()
	if err != nil {
		return err
	}
	if d.Status != "frozen_reviewed" || b.Status != "frozen_reviewed" || !b.ProviderCallsAllowed {
		return fmt.Errorf("R04 effort v1 provider execution is locked until independent review and atomic freeze")
	}
	return nil
}

func r04EffortFreezeV2SourcesValue() (r04EffortFreezeV2Sources, error) {
	var v r04EffortFreezeV2Sources
	_, err := CurrentR04EffortFreezeV2()
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(r04EffortFreezeV2JSON, &v)
	return v, err
}
func RequiredR04EffortV2CampaignExecutorSHA256() (string, error) {
	b, e := CurrentR04EffortFreezeV2()
	return b.CampaignExecutorSHA256, e
}
func RequiredR04EffortV2MainSHA256() (string, error) {
	b, e := CurrentR04EffortFreezeV2()
	return b.MainSHA256, e
}
func RequiredR04EffortV2OpenCodeProviderSHA256() (string, error) {
	v, e := r04EffortFreezeV2SourcesValue()
	return v.OpenCodeProviderSHA256, e
}
func RequiredR04EffortV2OpenCodeRuntimeSHA256() (string, error) {
	v, e := r04EffortFreezeV2SourcesValue()
	return v.OpenCodeRuntimeSHA256, e
}
func RequiredR04EffortV2CurrentScreeningMainSHA256() (string, error) {
	v, e := r04EffortFreezeV2SourcesValue()
	return v.CurrentScreeningMainSHA256, e
}
