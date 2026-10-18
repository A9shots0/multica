package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const ProtocolAblationV32FreezeSchema = "silver-mediums.protocol-ablation-freeze.v32"

type ProtocolAblationV32Freeze struct {
	SchemaVersion        string            `json:"schema_version"`
	FreezeID             string            `json:"freeze_id"`
	Status               string            `json:"status"`
	ReviewOutcome        string            `json:"review_outcome"`
	ProviderCallsAllowed bool              `json:"provider_calls_allowed"`
	UpstreamFreezeSHA256 string            `json:"upstream_freeze_sha256"`
	R05FixtureSHA256     string            `json:"r05_fixture_sha256"`
	R05GraderSHA256      string            `json:"r05_grader_sha256"`
	LocalComponents      map[string]string `json:"local_components"`
	CommandSources       map[string]string `json:"command_sources"`
}

//go:embed protocol_ablation_v32_freeze.json
var protocolAblationV32FreezeJSON []byte

//go:embed protocol_ablation_v32.json
var protocolAblationV32FreezeDefinitionJSON []byte

//go:embed protocol_ablation_v32.go
var protocolAblationV32FreezeSource []byte

//go:embed protocol_ablation_v32_test.go
var protocolAblationV32FreezeTests []byte

//go:embed protocol_ablation_v32_freeze.go
var protocolAblationV32FreezeGateSource []byte

//go:embed protocol_ablation_v32_freeze_test.go
var protocolAblationV32FreezeGateTests []byte

func CurrentProtocolAblationV32Freeze() (ProtocolAblationV32Freeze, error) {
	var freeze ProtocolAblationV32Freeze
	if err := json.Unmarshal(protocolAblationV32FreezeJSON, &freeze); err != nil {
		return freeze, fmt.Errorf("decode v32 freeze: %w", err)
	}
	if err := ValidateProtocolAblationV32Freeze(freeze); err != nil {
		return ProtocolAblationV32Freeze{}, err
	}
	return freeze, nil
}

func ValidateProtocolAblationV32Freeze(freeze ProtocolAblationV32Freeze) error {
	if freeze.SchemaVersion != ProtocolAblationV32FreezeSchema || freeze.FreezeID != "realistic-r05-protocol-ablation-freeze-v32" || freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "go" || !freeze.ProviderCallsAllowed {
		return fmt.Errorf("v32 freeze identity or GO decision drifted")
	}
	if freeze.UpstreamFreezeSHA256 != RealisticNextFreezeSHA256() || freeze.R05FixtureSHA256 != "406c40e75d2a39cecc816b3f4506234c3720161aa9cb1e6c56e214b6a1eb721f" || freeze.R05GraderSHA256 != "1454ccbf164d8152779ecc2768e46132f42df53dee7e36e1c099d777e87bf07a" {
		return fmt.Errorf("v32 upstream fixture or grader freeze drifted")
	}
	local := map[string][]byte{
		"protocol_ablation_v32.json":           protocolAblationV32FreezeDefinitionJSON,
		"protocol_ablation_v32.go":             protocolAblationV32FreezeSource,
		"protocol_ablation_v32_test.go":        protocolAblationV32FreezeTests,
		"protocol_ablation_v32_freeze.go":      protocolAblationV32FreezeGateSource,
		"protocol_ablation_v32_freeze_test.go": protocolAblationV32FreezeGateTests,
	}
	if len(freeze.LocalComponents) != len(local) {
		return fmt.Errorf("v32 frozen local component set drifted")
	}
	for name, raw := range local {
		digest := sha256.Sum256(raw)
		if freeze.LocalComponents[name] != hex.EncodeToString(digest[:]) {
			return fmt.Errorf("v32 frozen local component drifted: %s", name)
		}
	}
	wantCommands := []string{"protocol-ablation-v32/main.go", "protocol-ablation-v32/main_test.go", "protocol-ablation-v32/execute.go", "protocol-ablation-v32/execute_test.go"}
	if len(freeze.CommandSources) != len(wantCommands) {
		return fmt.Errorf("v32 frozen command source set drifted")
	}
	for _, name := range wantCommands {
		if !validSHA256(freeze.CommandSources[name]) {
			return fmt.Errorf("v32 frozen command hash is missing: %s", name)
		}
	}
	return nil
}

func RequireProtocolAblationV32ReviewedFreeze() error {
	d, err := CurrentProtocolAblationV32()
	if err != nil {
		return err
	}
	freeze, err := CurrentProtocolAblationV32Freeze()
	if err != nil {
		return err
	}
	if d.Status != "frozen_reviewed" || !d.ProviderCallsAllowed || freeze.Status != "frozen_reviewed" || freeze.ReviewOutcome != "go" || !freeze.ProviderCallsAllowed {
		return fmt.Errorf("v32 provider calls remain locked")
	}
	return nil
}

func RequiredProtocolAblationV32CommandSources() (map[string]string, error) {
	freeze, err := CurrentProtocolAblationV32Freeze()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(freeze.CommandSources))
	for name, hash := range freeze.CommandSources {
		result[name] = hash
	}
	return result, nil
}

func ProtocolAblationV32FreezeSHA256() string {
	digest := sha256.Sum256(protocolAblationV32FreezeJSON)
	return hex.EncodeToString(digest[:])
}
