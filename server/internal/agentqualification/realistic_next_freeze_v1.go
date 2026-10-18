package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const RealisticNextFreezeSchemaVersion = "silver-mediums.realistic-next-freeze.v1"

type RealisticNextFreeze struct {
	SchemaVersion        string            `json:"schema_version"`
	FreezeID             string            `json:"freeze_id"`
	Status               string            `json:"status"`
	ReviewOutcome        string            `json:"review_outcome"`
	ProviderCallsAllowed bool              `json:"provider_calls_allowed"`
	LocalComponents      map[string]string `json:"local_components"`
	CommandSources       map[string]string `json:"command_sources"`
}

//go:embed realistic_next_freeze_v1.json
var realisticNextFreezeV1JSON []byte

//go:embed realistic_next_group_v1.json
var realisticNextFreezeGroupJSON []byte

//go:embed realistic_next_group_v1.go
var realisticNextFreezeGroupSource []byte

//go:embed realistic_next_group_v1_test.go
var realisticNextFreezeGroupTests []byte

//go:embed realistic_next_matrix_v1.json
var realisticNextFreezeMatrixJSON []byte

//go:embed realistic_next_matrix_v1.go
var realisticNextFreezeMatrixSource []byte

//go:embed realistic_next_matrix_v1_test.go
var realisticNextFreezeMatrixTests []byte

//go:embed realistic_r05_fixture_v1.json
var realisticNextFreezeR05JSON []byte

//go:embed realistic_r05_fixture_v1.go
var realisticNextFreezeR05Source []byte

//go:embed realistic_r05_fixture_v1_test.go
var realisticNextFreezeR05Tests []byte

//go:embed realistic_r06_fixture_v1.go
var realisticNextFreezeR06Source []byte

//go:embed realistic_r06_fixture_v1_test.go
var realisticNextFreezeR06Tests []byte

//go:embed realistic_next_calibration_v1.go
var realisticNextFreezeCalibrationSource []byte

//go:embed realistic_next_calibration_v1_test.go
var realisticNextFreezeCalibrationTests []byte

//go:embed realistic_next_harness_v1.go
var realisticNextFreezeHarnessSource []byte

//go:embed realistic_next_harness_v1_test.go
var realisticNextFreezeHarnessTests []byte

//go:embed realistic_next_opencode_v1.go
var realisticNextFreezeOpenCodeSource []byte

//go:embed opencode_receipt.go
var realisticNextFreezeReceiptParserSource []byte

//go:embed opencode_transport.go
var realisticNextFreezeTransportSource []byte

func CurrentRealisticNextFreeze() (RealisticNextFreeze, error) {
	var freeze RealisticNextFreeze
	if err := json.Unmarshal(realisticNextFreezeV1JSON, &freeze); err != nil {
		return freeze, fmt.Errorf("decode realistic next freeze: %w", err)
	}
	if err := ValidateRealisticNextFreeze(freeze); err != nil {
		return RealisticNextFreeze{}, err
	}
	return freeze, nil
}

func ValidateRealisticNextFreeze(freeze RealisticNextFreeze) error {
	if freeze.SchemaVersion != RealisticNextFreezeSchemaVersion || freeze.FreezeID != "realistic-continuity-and-rollout-freeze-v1" {
		return fmt.Errorf("realistic next freeze identity drifted")
	}
	if freeze.Status != "pending_independent_review" && freeze.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic next freeze status is invalid")
	}
	if freeze.Status == "pending_independent_review" && (freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "pending") {
		return fmt.Errorf("realistic next provider calls unlocked before review")
	}
	if freeze.Status == "frozen_reviewed" && (!freeze.ProviderCallsAllowed || freeze.ReviewOutcome != "go") {
		return fmt.Errorf("realistic next reviewed freeze is not executable")
	}
	local := map[string][]byte{
		"realistic_next_group_v1.json":          realisticNextFreezeGroupJSON,
		"realistic_next_group_v1.go":            realisticNextFreezeGroupSource,
		"realistic_next_group_v1_test.go":       realisticNextFreezeGroupTests,
		"realistic_next_matrix_v1.json":         realisticNextFreezeMatrixJSON,
		"realistic_next_matrix_v1.go":           realisticNextFreezeMatrixSource,
		"realistic_next_matrix_v1_test.go":      realisticNextFreezeMatrixTests,
		"realistic_r05_fixture_v1.json":         realisticNextFreezeR05JSON,
		"realistic_r05_fixture_v1.go":           realisticNextFreezeR05Source,
		"realistic_r05_fixture_v1_test.go":      realisticNextFreezeR05Tests,
		"realistic_r06_fixture_v1.go":           realisticNextFreezeR06Source,
		"realistic_r06_fixture_v1_test.go":      realisticNextFreezeR06Tests,
		"realistic_next_calibration_v1.go":      realisticNextFreezeCalibrationSource,
		"realistic_next_calibration_v1_test.go": realisticNextFreezeCalibrationTests,
		"realistic_next_harness_v1.go":          realisticNextFreezeHarnessSource,
		"realistic_next_harness_v1_test.go":     realisticNextFreezeHarnessTests,
		"realistic_next_opencode_v1.go":         realisticNextFreezeOpenCodeSource,
		"opencode_receipt.go":                   realisticNextFreezeReceiptParserSource,
		"opencode_transport.go":                 realisticNextFreezeTransportSource,
	}
	if len(freeze.LocalComponents) != len(local) {
		return fmt.Errorf("realistic next local freeze component set drifted")
	}
	for name, raw := range local {
		hash := sha256.Sum256(raw)
		if freeze.LocalComponents[name] != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("realistic next component hash mismatch: %s", name)
		}
	}
	wantCommands := []string{"realistic_next_execute_v1.go", "realistic_next_execute_v1_test.go", "main.go", "realistic_execute.go", "openrouter_execute.go"}
	if len(freeze.CommandSources) != len(wantCommands) {
		return fmt.Errorf("realistic next command freeze component set drifted")
	}
	for _, name := range wantCommands {
		if !validSHA256(freeze.CommandSources[name]) {
			return fmt.Errorf("realistic next command hash is missing: %s", name)
		}
	}
	return nil
}

func RequiredRealisticNextCommandSources() (map[string]string, error) {
	freeze, err := CurrentRealisticNextFreeze()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(freeze.CommandSources))
	for name, hash := range freeze.CommandSources {
		result[name] = hash
	}
	return result, nil
}

func RealisticNextFreezeSHA256() string {
	hash := sha256.Sum256(realisticNextFreezeV1JSON)
	return hex.EncodeToString(hash[:])
}
