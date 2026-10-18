package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const RealisticResidualFreezeV2SchemaVersion = "silver-mediums.realistic-residual-freeze.v2"

type RealisticResidualFreezeV2 struct {
	SchemaVersion               string            `json:"schema_version"`
	FreezeID                    string            `json:"freeze_id"`
	Status                      string            `json:"status"`
	ReviewOutcome               string            `json:"review_outcome"`
	ProviderCallsAllowed        bool              `json:"provider_calls_allowed"`
	UpstreamFixtureFreezeSHA256 string            `json:"upstream_fixture_freeze_sha256"`
	LocalComponents             map[string]string `json:"local_components"`
	CommandSources              map[string]string `json:"command_sources"`
}

//go:embed realistic_residual_freeze_v2.json
var realisticResidualFreezeV2JSON []byte

//go:embed realistic_residual_matrix_v2.json
var realisticResidualFreezeMatrixJSON []byte

//go:embed realistic_residual_matrix_v2.go
var realisticResidualFreezeMatrixSource []byte

//go:embed realistic_residual_matrix_v2_test.go
var realisticResidualFreezeMatrixTests []byte

//go:embed realistic_residual_freeze_v2.go
var realisticResidualFreezeSource []byte

//go:embed realistic_residual_freeze_v2_test.go
var realisticResidualFreezeTests []byte

func CurrentRealisticResidualFreezeV2() (RealisticResidualFreezeV2, error) {
	var freeze RealisticResidualFreezeV2
	if err := json.Unmarshal(realisticResidualFreezeV2JSON, &freeze); err != nil {
		return freeze, fmt.Errorf("decode realistic residual freeze v2: %w", err)
	}
	if err := ValidateRealisticResidualFreezeV2(freeze); err != nil {
		return RealisticResidualFreezeV2{}, err
	}
	return freeze, nil
}

func ValidateRealisticResidualFreezeV2(freeze RealisticResidualFreezeV2) error {
	if freeze.SchemaVersion != RealisticResidualFreezeV2SchemaVersion || freeze.FreezeID != "realistic-continuity-and-rollout-residual-freeze-v31" {
		return fmt.Errorf("realistic residual freeze v2 identity drifted")
	}
	if freeze.Status != "pending_independent_review" && freeze.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic residual freeze v2 status is invalid")
	}
	if freeze.Status == "pending_independent_review" && (freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "pending") {
		return fmt.Errorf("realistic residual v2 provider calls unlocked before review")
	}
	if freeze.Status == "frozen_reviewed" && (!freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "go") {
		return fmt.Errorf("realistic residual v2 reviewed freeze is not executable")
	}
	if freeze.UpstreamFixtureFreezeSHA256 != RealisticNextFreezeSHA256() {
		return fmt.Errorf("realistic residual v2 upstream R05/R06 freeze drifted")
	}
	local := map[string][]byte{
		"realistic_residual_matrix_v2.json":    realisticResidualFreezeMatrixJSON,
		"realistic_residual_matrix_v2.go":      realisticResidualFreezeMatrixSource,
		"realistic_residual_matrix_v2_test.go": realisticResidualFreezeMatrixTests,
		"realistic_residual_freeze_v2.go":      realisticResidualFreezeSource,
		"realistic_residual_freeze_v2_test.go": realisticResidualFreezeTests,
	}
	if len(freeze.LocalComponents) != len(local) {
		return fmt.Errorf("realistic residual v2 local component set drifted")
	}
	for name, raw := range local {
		hash := sha256.Sum256(raw)
		if freeze.LocalComponents[name] != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("realistic residual v2 component hash mismatch: %s", name)
		}
	}
	wantCommands := []string{"realistic_residual_execute_v2.go", "realistic_residual_execute_v2_test.go", "realistic_residual_entry_v2.go", "realistic_next_execute_v1.go", "realistic_execute.go", "openrouter_execute.go", "realistic-residual-run-v2/main.go", "realistic-residual-run-v2/main_test.go"}
	if len(freeze.CommandSources) != len(wantCommands) {
		return fmt.Errorf("realistic residual v2 command component set drifted")
	}
	for _, name := range wantCommands {
		if !validSHA256(freeze.CommandSources[name]) {
			return fmt.Errorf("realistic residual v2 command hash is missing: %s", name)
		}
	}
	return nil
}

func RequireRealisticResidualV2ReviewedFreeze() error {
	if err := RequireRealisticNextReviewedFreeze(); err != nil {
		return fmt.Errorf("realistic residual v2 upstream freeze: %w", err)
	}
	matrix, err := CurrentRealisticResidualRunMatrixV2()
	if err != nil {
		return err
	}
	freeze, err := CurrentRealisticResidualFreezeV2()
	if err != nil {
		return err
	}
	if matrix.Status != "frozen_reviewed" || freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "go" || !freeze.ProviderCallsAllowed {
		return fmt.Errorf("realistic residual v2 provider calls remain locked pending independent review")
	}
	return nil
}

func RequiredRealisticResidualV2CommandSources() (map[string]string, error) {
	freeze, err := CurrentRealisticResidualFreezeV2()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(freeze.CommandSources))
	for name, hash := range freeze.CommandSources {
		result[name] = hash
	}
	return result, nil
}

func RealisticResidualFreezeV2SHA256() string {
	hash := sha256.Sum256(realisticResidualFreezeV2JSON)
	return hex.EncodeToString(hash[:])
}
