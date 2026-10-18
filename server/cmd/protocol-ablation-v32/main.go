package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

//go:embed main.go
var protocolV32MainSource []byte

//go:embed execute.go
var protocolV32ExecuteSource []byte

//go:embed main_test.go
var protocolV32MainTestSource []byte

//go:embed execute_test.go
var protocolV32ExecuteTestSource []byte

func main() {
	if len(os.Args) == 4 && os.Args[1] == "recover-v3-initialize-v4" {
		ledger, err := recoverProtocolV32V3AndInitializeV4(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(ledger)
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "initialize-v3" {
		ledger, err := initializeProtocolV32V3(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(ledger)
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "recover-session" {
		record, err := recoverProtocolV32Session(context.Background(), os.Args[2], os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(record)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "execute" {
		artifactRoot := "artifacts/protocol-ablation-v32"
		if len(os.Args) > 2 {
			artifactRoot = os.Args[2]
		}
		ledger, err := executeProtocolAblationV32Live(context.Background(), artifactRoot)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(ledger)
		return
	}
	plan, err := protocolAblationV32Plan()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func protocolAblationV32Plan() (map[string]any, error) {
	definition, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return nil, err
	}
	freeze, err := agentqualification.CurrentProtocolAblationV32Freeze()
	if err != nil {
		return nil, err
	}
	bindings, err := agentqualification.ProtocolAblationV32ExecutionOrder(definition)
	if err != nil {
		return nil, err
	}
	plans := make([]agentqualification.ProtocolAblationV32ProviderPlan, 0, len(bindings))
	for _, binding := range bindings {
		plan, err := agentqualification.BuildProtocolAblationV32ProviderFreePlan(definition, binding)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return map[string]any{
		"status":                      definition.Status,
		"provider_called":             false,
		"provider_calls_allowed":      definition.ProviderCallsAllowed,
		"experiment_id":               definition.ExperimentID,
		"case_id":                     definition.Case.CaseID,
		"source_case_id":              definition.Case.SourceCaseID,
		"bindings":                    plans,
		"max_combined_tokens":         definition.MaxCombinedTokens,
		"max_combined_cost_micros":    definition.MaxCombinedCostMicros,
		"max_provider_turns":          definition.MaxCombinedProviderTurns,
		"retries":                     definition.Retries,
		"provider_fallbacks":          definition.ProviderFallbacks,
		"calibration_only":            true,
		"causal_or_reliability_proof": false,
		"interpretations": map[string]any{
			"equal_reservation":     "Both arms retain the same 75,000-token and price-derived cost reservation; this controls maximum resource exposure, not actual token equality.",
			"capability_normalized": "The loop receives ten sequential turns because R05 requires at least eight actions; the one-shot arm receives one response large enough for ten conditional steps. Results compare usable protocol capability, not equal provider-call counts.",
		},
		"source_hashes":         protocolV32SourceHashes(),
		"freeze_review_outcome": freeze.ReviewOutcome,
		"freeze_sha256":         agentqualification.ProtocolAblationV32FreezeSHA256(),
	}, nil
}

func protocolV32SourceHashes() map[string]string {
	hashes := agentqualification.ProtocolAblationV32SourceHashes()
	for name, raw := range map[string][]byte{"cmd/protocol-ablation-v32/main.go": protocolV32MainSource, "cmd/protocol-ablation-v32/execute.go": protocolV32ExecuteSource, "cmd/protocol-ablation-v32/main_test.go": protocolV32MainTestSource, "cmd/protocol-ablation-v32/execute_test.go": protocolV32ExecuteTestSource} {
		digest := sha256.Sum256(raw)
		hashes[name] = hex.EncodeToString(digest[:])
	}
	return hashes
}

func verifyProtocolV32CommandFreeze() error {
	required, err := agentqualification.RequiredProtocolAblationV32CommandSources()
	if err != nil {
		return err
	}
	actual := map[string][]byte{"protocol-ablation-v32/main.go": protocolV32MainSource, "protocol-ablation-v32/execute.go": protocolV32ExecuteSource, "protocol-ablation-v32/main_test.go": protocolV32MainTestSource, "protocol-ablation-v32/execute_test.go": protocolV32ExecuteTestSource}
	if len(required) != len(actual) {
		return fmt.Errorf("v32 frozen command source set drifted")
	}
	for name, raw := range actual {
		digest := sha256.Sum256(raw)
		if required[name] != hex.EncodeToString(digest[:]) {
			return fmt.Errorf("v32 frozen command source drifted: %s", name)
		}
	}
	return nil
}
