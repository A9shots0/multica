package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	result, receipts, err := agentqualification.RunProvisionalRoleConfigurationRehearsalV2()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	evaluatedAt := time.Now().UTC()
	currentDecision, currentReason, err := agentqualification.EvaluateCurrentProvisionalRoleConfigurationV2At(evaluatedAt)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output := struct {
		Result          agentqualification.ProvisionalRoleConfigurationRehearsalResultV2    `json:"result"`
		Receipts        []agentqualification.ProvisionalRoleConfigurationRehearsalReceiptV2 `json:"receipts"`
		CurrentDecision string                                                              `json:"current_decision"`
		CurrentReason   string                                                              `json:"current_reason"`
		EvaluatedAt     string                                                              `json:"evaluated_at"`
	}{Result: result, Receipts: receipts, CurrentDecision: currentDecision, CurrentReason: currentReason, EvaluatedAt: evaluatedAt.Format(time.RFC3339)}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
