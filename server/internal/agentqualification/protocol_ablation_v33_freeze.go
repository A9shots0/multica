package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const ProtocolAblationV33FreezeSchema = "silver-mediums.protocol-ablation-freeze.v33"

type ProtocolAblationV33Freeze struct {
	SchemaVersion            string            `json:"schema_version"`
	FreezeID                 string            `json:"freeze_id"`
	Status                   string            `json:"status"`
	ProviderCallsAllowed     bool              `json:"provider_calls_allowed"`
	V32FreezeSHA256          string            `json:"v32_freeze_sha256"`
	R05FixtureSHA256         string            `json:"r05_fixture_sha256"`
	R05GraderSHA256          string            `json:"r05_grader_sha256"`
	OpenCodeVersion          string            `json:"opencode_version"`
	OpenCodeExecutableSHA256 string            `json:"opencode_executable_sha256"`
	OpenCodeOpenAPISHA256    string            `json:"opencode_openapi_sha256"`
	ExecutionOrder           []string          `json:"execution_order"`
	MaxCombinedTokens        int64             `json:"max_combined_tokens"`
	MaxCombinedCostMicros    int64             `json:"max_combined_cost_micros"`
	MaxCombinedProviderTurns int               `json:"max_combined_provider_turns"`
	Retries                  int               `json:"retries"`
	ProviderFallbacks        bool              `json:"provider_fallbacks"`
	LocalComponents          map[string]string `json:"local_components"`
	CommandSources           map[string]string `json:"command_sources"`
	Documents                map[string]string `json:"documents"`
}

//go:embed protocol_ablation_v33_freeze.json
var v33FreezeJSON []byte

//go:embed opencode_structured_v33.go
var v33StructuredSource []byte

//go:embed opencode_structured_v33_lifecycle.go
var v33LifecycleSource []byte

//go:embed opencode_structured_v33_test.go
var v33StructuredTests []byte

//go:embed protocol_ablation_v33_freeze.go
var v33FreezeSource []byte

//go:embed protocol_ablation_v33_freeze_test.go
var v33FreezeTests []byte

func CurrentProtocolAblationV33Freeze() (ProtocolAblationV33Freeze, error) {
	var f ProtocolAblationV33Freeze
	if err := json.Unmarshal(v33FreezeJSON, &f); err != nil {
		return f, err
	}
	if err := ValidateProtocolAblationV33Freeze(f); err != nil {
		return ProtocolAblationV33Freeze{}, err
	}
	return f, nil
}
func ValidateProtocolAblationV33Freeze(f ProtocolAblationV33Freeze) error {
	if f.SchemaVersion != ProtocolAblationV33FreezeSchema || f.FreezeID != "realistic-r05-opencode-structured-freeze-v33" || f.Status != "frozen_ready_not_executed" || !f.ProviderCallsAllowed {
		return fmt.Errorf("v33 freeze identity/lock drift")
	}
	if f.V32FreezeSHA256 != ProtocolAblationV32FreezeSHA256() || f.R05FixtureSHA256 != "406c40e75d2a39cecc816b3f4506234c3720161aa9cb1e6c56e214b6a1eb721f" || f.R05GraderSHA256 != "1454ccbf164d8152779ecc2768e46132f42df53dee7e36e1c099d777e87bf07a" {
		return fmt.Errorf("v33 inherited freeze drift")
	}
	if f.OpenCodeVersion != OpenCodeStructuredV33Version || f.OpenCodeExecutableSHA256 != OpenCodeStructuredV33ExecutableSHA256 || f.OpenCodeOpenAPISHA256 != OpenCodeStructuredV33OpenAPISHA256 {
		return fmt.Errorf("v33 runtime identity drift")
	}
	d, _ := CurrentProtocolAblationV32()
	order, _ := ProtocolAblationV32ExecutionOrder(d)
	if len(f.ExecutionOrder) != 4 || len(order) != 4 {
		return fmt.Errorf("v33 four-cell order missing")
	}
	for i := range order {
		if f.ExecutionOrder[i] != order[i].BindingID {
			return fmt.Errorf("v33 execution order drift")
		}
	}
	if f.MaxCombinedTokens != d.MaxCombinedTokens || f.MaxCombinedCostMicros != d.MaxCombinedCostMicros || f.MaxCombinedProviderTurns != d.MaxCombinedProviderTurns || f.Retries != 0 || f.ProviderFallbacks {
		return fmt.Errorf("v33 budget/retry/fallback drift")
	}
	local := map[string][]byte{"opencode_structured_v33.go": v33StructuredSource, "opencode_structured_v33_lifecycle.go": v33LifecycleSource, "opencode_structured_v33_test.go": v33StructuredTests, "protocol_ablation_v33_freeze.go": v33FreezeSource, "protocol_ablation_v33_freeze_test.go": v33FreezeTests}
	if len(f.LocalComponents) != len(local) {
		return fmt.Errorf("v33 local component set drift")
	}
	for name, raw := range local {
		h := sha256.Sum256(raw)
		if f.LocalComponents[name] != hex.EncodeToString(h[:]) {
			return fmt.Errorf("v33 local component drift: %s", name)
		}
	}
	if len(f.CommandSources) != 5 || len(f.Documents) != 1 || !validSHA256(f.Documents["docs/silver-mediums/ITEM-14-OPENCODE-STRUCTURED-OUTPUT-V33-PREVALIDATION.md"]) {
		return fmt.Errorf("v33 command/doc freeze incomplete")
	}
	for _, name := range []string{"protocol-ablation-v33/main.go", "protocol-ablation-v33/execute.go", "protocol-ablation-v33/main_test.go", "protocol-ablation-v33/recovery.go", "protocol-ablation-v33/recovery_test.go"} {
		if !validSHA256(f.CommandSources[name]) {
			return fmt.Errorf("v33 command hash missing: %s", name)
		}
	}
	return nil
}
func RequireProtocolAblationV33Freeze(commands map[string]string) error {
	f, err := CurrentProtocolAblationV33Freeze()
	if err != nil {
		return err
	}
	if len(commands) != len(f.CommandSources) {
		return fmt.Errorf("v33 command set drift")
	}
	for n, h := range commands {
		if f.CommandSources[n] != h {
			return fmt.Errorf("v33 command drift: %s", n)
		}
	}
	return nil
}
func ProtocolAblationV33FreezeSHA256() string {
	h := sha256.Sum256(v33FreezeJSON)
	return hex.EncodeToString(h[:])
}
