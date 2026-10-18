package agentqualification

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed claude_calibration_v29_result.json
var claudeCalibrationV29ResultJSON []byte

func TestClaudeCalibrationV29ResultPreservesZeroCallInvalidity(t *testing.T) {
	var result struct {
		Status             string `json:"status"`
		TestValidity       string `json:"test_validity"`
		ProcessAttempts    int    `json:"process_launch_attempts"`
		ProviderCalls      int    `json:"provider_calls"`
		Retries            int    `json:"retries"`
		Tokens             int64  `json:"tokens"`
		CandidateAdvances  bool   `json:"candidate_advances"`
		TransportQualified bool   `json:"transport_qualified"`
	}
	if err := json.Unmarshal(claudeCalibrationV29ResultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "invalid_transport_harness" || result.TestValidity != "invalid" || result.ProcessAttempts != 1 || result.ProviderCalls != 0 || result.Retries != 0 || result.Tokens != 0 || result.CandidateAdvances || result.TransportQualified {
		t.Fatalf("v29 invalid evidence was weakened: %+v", result)
	}
}
