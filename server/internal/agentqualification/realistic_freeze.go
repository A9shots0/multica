package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const RealisticFreezeBundleSchemaVersion = "silver-mediums.realistic-freeze-bundle.v1"

type RealisticFreezeBundle struct {
	SchemaVersion                    string   `json:"schema_version"`
	BundleID                         string   `json:"bundle_id"`
	Status                           string   `json:"status"`
	GroupSHA256                      string   `json:"group_sha256"`
	FixtureManifestSHA256            string   `json:"fixture_manifest_sha256"`
	RunMatrixSHA256                  string   `json:"run_matrix_sha256"`
	NextRunMatrixSHA256              string   `json:"next_run_matrix_sha256"`
	FollowingRunMatrixSHA256         string   `json:"following_run_matrix_sha256"`
	CorrectedRunMatrixSHA256         string   `json:"corrected_run_matrix_sha256"`
	ContinuationRunMatrixSHA256      string   `json:"continuation_run_matrix_sha256"`
	InventoryRunMatrixSHA256         string   `json:"inventory_run_matrix_sha256"`
	PostBatchRunMatrixSHA256         string   `json:"post_batch_run_matrix_sha256"`
	R04RunMatrixSHA256               string   `json:"r04_run_matrix_sha256"`
	BrowserRunMatrixSHA256           string   `json:"browser_run_matrix_sha256"`
	ReliabilityRunMatrixSHA256       string   `json:"reliability_run_matrix_sha256"`
	ReliabilityBrowserMatrixSHA256   string   `json:"reliability_browser_matrix_sha256"`
	SurvivorCapabilityMatrixSHA256   string   `json:"survivor_capability_matrix_sha256"`
	AffectedReliabilityMatrixSHA256  string   `json:"affected_reliability_matrix_sha256"`
	RunnerSHA256                     string   `json:"runner_sha256"`
	FixtureLoaderSHA256              string   `json:"fixture_loader_sha256"`
	ControllerSHA256                 string   `json:"controller_sha256"`
	BrowserBridgeSHA256              string   `json:"browser_bridge_sha256"`
	OpenCodePlanSHA256               string   `json:"opencode_plan_sha256"`
	OpenCodeTransportValidatorSHA256 string   `json:"opencode_transport_validator_sha256"`
	OpenCodeReceiptParserSHA256      string   `json:"opencode_receipt_parser_sha256"`
	OpenCodeRuntimeSHA256            string   `json:"opencode_runtime_sha256"`
	BrowserServerSHA256              string   `json:"browser_server_sha256"`
	ExecutorSHA256                   string   `json:"executor_sha256"`
	CampaignExecutorSHA256           string   `json:"campaign_executor_sha256"`
	CampaignSHA256                   string   `json:"campaign_sha256"`
	VerifierSHA256                   string   `json:"verifier_sha256"`
	ReceiptValidatorSHA256           string   `json:"receipt_validator_sha256"`
	ReceiptSealerSHA256              string   `json:"receipt_sealer_sha256"`
	FailureSealerSHA256              string   `json:"failure_sealer_sha256"`
	EvidenceDeriverSHA256            string   `json:"evidence_deriver_sha256"`
	DeclarationsSHA256               string   `json:"declarations_sha256"`
	ValidationMatrix                 []string `json:"validation_matrix"`
	ProviderCallsAllowed             bool     `json:"provider_calls_allowed"`
}

//go:embed realistic_freeze_v1.json
var realisticFreezeV1JSON []byte

//go:embed realistic_workflow_group_v1.json
var realisticFreezeGroupJSON []byte

//go:embed realistic_fixture_manifest_v1.json
var realisticFreezeFixtureManifestJSON []byte

//go:embed realistic_matrix_v1.json
var realisticFreezeMatrixJSON []byte

//go:embed realistic_matrix_v2.json
var realisticFreezeMatrixV2JSON []byte

//go:embed realistic_matrix_v3.json
var realisticFreezeMatrixV3JSON []byte

//go:embed realistic_matrix_v4.json
var realisticFreezeMatrixV4JSON []byte

//go:embed realistic_matrix_v5.json
var realisticFreezeMatrixV5JSON []byte

//go:embed realistic_matrix_v6.json
var realisticFreezeMatrixV6JSON []byte

//go:embed realistic_matrix_v7.json
var realisticFreezeMatrixV7JSON []byte

//go:embed realistic_matrix_v8.json
var realisticFreezeMatrixV8JSON []byte

//go:embed realistic_matrix_v9.json
var realisticFreezeMatrixV9JSON []byte

//go:embed realistic_matrix_v10.json
var realisticFreezeMatrixV10JSON []byte

//go:embed realistic_matrix_v11.json
var realisticFreezeMatrixV11JSON []byte

//go:embed realistic_matrix_v13.json
var realisticFreezeMatrixV13JSON []byte

//go:embed realistic_matrix_v14.json
var realisticFreezeMatrixV14JSON []byte

//go:embed realistic_runner.go
var realisticFreezeRunnerSource []byte

//go:embed realistic_fixture.go
var realisticFreezeFixtureLoaderSource []byte

//go:embed realistic_controller.go
var realisticFreezeControllerSource []byte

//go:embed realistic_browser_bridge.go
var realisticFreezeBrowserBridgeSource []byte

//go:embed realistic_opencode.go
var realisticFreezeOpenCodePlanSource []byte

//go:embed opencode_transport.go
var realisticFreezeOpenCodeTransportValidatorSource []byte

//go:embed opencode_receipt.go
var realisticFreezeOpenCodeReceiptParserSource []byte

//go:embed realistic_campaign.go
var realisticFreezeCampaignSource []byte

//go:embed realistic_verifier.go
var realisticFreezeVerifierSource []byte

//go:embed realistic_execution.go
var realisticFreezeReceiptValidatorSource []byte

//go:embed realistic_seal.go
var realisticFreezeReceiptSealerSource []byte

//go:embed realistic_failure_seal.go
var realisticFreezeFailureSealerSource []byte

//go:embed realistic_evidence.go
var realisticFreezeEvidenceDeriverSource []byte

//go:embed realistic_declarations.go
var realisticFreezeDeclarationsSource []byte

func CurrentRealisticFreezeBundle() (RealisticFreezeBundle, error) {
	var bundle RealisticFreezeBundle
	if err := json.Unmarshal(realisticFreezeV1JSON, &bundle); err != nil {
		return bundle, fmt.Errorf("decode realistic freeze bundle: %w", err)
	}
	if err := ValidateRealisticFreezeBundle(bundle); err != nil {
		return RealisticFreezeBundle{}, err
	}
	return bundle, nil
}

func ValidateRealisticFreezeBundle(bundle RealisticFreezeBundle) error {
	if bundle.SchemaVersion != RealisticFreezeBundleSchemaVersion || bundle.BundleID != "realistic-workflow-foundation-v2" {
		return fmt.Errorf("realistic freeze bundle identity is incomplete")
	}
	if bundle.Status != "pending_independent_review" && bundle.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic freeze status is invalid")
	}
	want := map[string][]byte{
		bundle.GroupSHA256:                      realisticFreezeGroupJSON,
		bundle.FixtureManifestSHA256:            realisticFreezeFixtureManifestJSON,
		bundle.RunMatrixSHA256:                  realisticFreezeMatrixJSON,
		bundle.NextRunMatrixSHA256:              realisticFreezeMatrixV2JSON,
		bundle.FollowingRunMatrixSHA256:         realisticFreezeMatrixV3JSON,
		bundle.CorrectedRunMatrixSHA256:         realisticFreezeMatrixV4JSON,
		bundle.ContinuationRunMatrixSHA256:      realisticFreezeMatrixV5JSON,
		bundle.InventoryRunMatrixSHA256:         realisticFreezeMatrixV6JSON,
		bundle.PostBatchRunMatrixSHA256:         realisticFreezeMatrixV7JSON,
		bundle.R04RunMatrixSHA256:               realisticFreezeMatrixV8JSON,
		bundle.BrowserRunMatrixSHA256:           realisticFreezeMatrixV9JSON,
		bundle.ReliabilityRunMatrixSHA256:       realisticFreezeMatrixV10JSON,
		bundle.ReliabilityBrowserMatrixSHA256:   realisticFreezeMatrixV11JSON,
		bundle.SurvivorCapabilityMatrixSHA256:   realisticFreezeMatrixV13JSON,
		bundle.AffectedReliabilityMatrixSHA256:  realisticFreezeMatrixV14JSON,
		bundle.RunnerSHA256:                     realisticFreezeRunnerSource,
		bundle.FixtureLoaderSHA256:              realisticFreezeFixtureLoaderSource,
		bundle.ControllerSHA256:                 realisticFreezeControllerSource,
		bundle.BrowserBridgeSHA256:              realisticFreezeBrowserBridgeSource,
		bundle.OpenCodePlanSHA256:               realisticFreezeOpenCodePlanSource,
		bundle.OpenCodeTransportValidatorSHA256: realisticFreezeOpenCodeTransportValidatorSource,
		bundle.OpenCodeReceiptParserSHA256:      realisticFreezeOpenCodeReceiptParserSource,
		bundle.CampaignSHA256:                   realisticFreezeCampaignSource,
		bundle.VerifierSHA256:                   realisticFreezeVerifierSource,
		bundle.ReceiptValidatorSHA256:           realisticFreezeReceiptValidatorSource,
		bundle.ReceiptSealerSHA256:              realisticFreezeReceiptSealerSource,
		bundle.FailureSealerSHA256:              realisticFreezeFailureSealerSource,
		bundle.EvidenceDeriverSHA256:            realisticFreezeEvidenceDeriverSource,
		bundle.DeclarationsSHA256:               realisticFreezeDeclarationsSource,
	}
	if len(want) != 29 || !validSHA256(bundle.ExecutorSHA256) || !validSHA256(bundle.CampaignExecutorSHA256) || !validSHA256(bundle.OpenCodeRuntimeSHA256) || !validSHA256(bundle.BrowserServerSHA256) {
		return fmt.Errorf("realistic freeze hashes are incomplete or duplicated")
	}
	for expected, raw := range want {
		hash := sha256.Sum256(raw)
		if expected != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("realistic freeze component hash mismatch")
		}
	}
	if !exactSet(bundle.ValidationMatrix, "strong_pass", "semantic_paraphrase", "subtle_failure", "adversarial_instruction", "fabricated_evidence", "scope_expansion", "missing_evidence", "invalid_starting_state") {
		return fmt.Errorf("realistic freeze validation matrix is incomplete")
	}
	if bundle.Status == "pending_independent_review" && bundle.ProviderCallsAllowed {
		return fmt.Errorf("realistic provider calls unlocked before review")
	}
	if bundle.Status == "frozen_reviewed" && !bundle.ProviderCallsAllowed {
		return fmt.Errorf("reviewed realistic bundle is not executable")
	}
	return nil
}

func RequiredRealisticExecutorSHA256() (string, error) {
	bundle, err := CurrentRealisticFreezeBundle()
	if err != nil {
		return "", err
	}
	return bundle.ExecutorSHA256, nil
}

func RequiredRealisticCampaignExecutorSHA256() (string, error) {
	bundle, err := CurrentRealisticFreezeBundle()
	if err != nil {
		return "", err
	}
	return bundle.CampaignExecutorSHA256, nil
}

func RequiredRealisticOpenCodeRuntimeSHA256() (string, error) {
	bundle, err := CurrentRealisticFreezeBundle()
	if err != nil {
		return "", err
	}
	return bundle.OpenCodeRuntimeSHA256, nil
}

func RequiredRealisticBrowserServerSHA256() (string, error) {
	bundle, err := CurrentRealisticFreezeBundle()
	if err != nil {
		return "", err
	}
	return bundle.BrowserServerSHA256, nil
}

func RequireCompleteRealisticFreeze() error {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return err
	}
	manifest, err := CurrentRealisticFixtureManifest()
	if err != nil {
		return err
	}
	matrix, err := CurrentRealisticRunMatrix()
	if err != nil {
		return err
	}
	bundle, err := CurrentRealisticFreezeBundle()
	if err != nil {
		return err
	}
	if group.Status != manifest.Status || group.Status != matrix.Status || group.Status != bundle.Status || bundle.Status != "frozen_reviewed" || !bundle.ProviderCallsAllowed {
		return fmt.Errorf("realistic workflow provider execution is locked until the complete bundle is independently reviewed and atomically frozen")
	}
	return nil
}
