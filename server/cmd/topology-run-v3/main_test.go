package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestTopologyV3AdmissionIsReadyAfterReview(t *testing.T) {
	definition, err := agentqualification.CurrentTopologyExperimentV3()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := agentqualification.CurrentTopologyFreezeV3()
	if err != nil {
		t.Fatal(err)
	}
	if !topologyV3ProviderReady(definition, bundle) {
		t.Fatal("reviewed topology v3 did not enable provider execution")
	}
}

func TestTopologyV3TransportSealsRawProviderIdentityAndUsage(t *testing.T) {
	candidate := agentqualification.RealisticCandidate{Model: "openrouter/test/model", ServingProvider: "fixture provider"}
	routing := realisticOpenCodeRoutingEvidence{ResolvedConfigSHA256: strings.Repeat("a", 64), InlineConfigSHA256: strings.Repeat("b", 64), DirectProvider: "openrouter", DirectProviderEvidence: "opencode_export_provider_id", RequestedUpstreamProvider: "fixture provider", ProviderOrder: []string{"fixture provider"}, AllowFallbacks: false, ActualUpstreamProviderState: "unavailable_from_provider_metadata"}
	rawJSONL := []byte(strings.Join([]string{
		`{"type":"step_start","timestamp":1000,"sessionID":"ses","part":{"sessionID":"ses","type":"step-start"}}`,
		`{"type":"text","timestamp":1100,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"read\",\"path\":\"TASK.md\"}}"}}`,
		`{"type":"step_finish","timestamp":1200,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}}`,
	}, "\n"))
	rawExport := []byte(`{"info":{"id":"ses"},"messages":[{"info":{"id":"msg","sessionID":"ses","role":"assistant","modelID":"test/model","providerID":"openrouter","summary":false,"finish":"stop","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}},"error":null},"parts":[{"id":"text","sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"read\",\"path\":\"TASK.md\"}}","metadata":{"openrouter_metadata":{"endpoints":{"available":[{"provider":"Fixture Provider","selected":true}]}}}},{"id":"finish","sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}]}]}`)
	response, err := translateRealisticOpenCodeArtifact(candidate, routing, rawJSONL, rawExport)
	if err != nil {
		t.Fatal(err)
	}
	if response.InputTokens != 13 || response.OutputTokens != 4 || response.ReasoningTokens != 2 || response.CostMicros != 1000 || !strings.Contains(string(response.RawReceipt), `"jsonl"`) || !strings.Contains(string(response.RawReceipt), `"export"`) || !strings.Contains(string(response.RawReceipt), `"actual_upstream_provider_state":"reported_by_openrouter_metadata"`) {
		t.Fatalf("paid response was not provenance-complete and sealed: %+v", response)
	}
}

func TestTopologyV3FreezePinsCommandSources(t *testing.T) {
	bundle, err := agentqualification.CurrentTopologyFreezeV3()
	if err != nil {
		t.Fatal(err)
	}
	_, current, _, _ := runtime.Caller(0)
	providerHash, err := agentqualification.RequiredTopologyV3OpenCodeProviderSHA256()
	if err != nil {
		t.Fatal(err)
	}
	runtimeHash, err := agentqualification.RequiredTopologyV3OpenCodeRuntimeSHA256()
	if err != nil {
		t.Fatal(err)
	}
	for path, expected := range map[string]string{
		filepath.Join(filepath.Dir(current), "topology_v3_cli.go"):    bundle.CampaignExecutorSHA256,
		filepath.Join(filepath.Dir(current), "main.go"):               bundle.MainSHA256,
		filepath.Join(filepath.Dir(current), "realistic_execute.go"):  providerHash,
		filepath.Join(filepath.Dir(current), "openrouter_execute.go"): runtimeHash,
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(raw)
		if hex.EncodeToString(hash[:]) != expected {
			t.Fatalf("topology v3 command source drifted: %s", path)
		}
	}
}

func TestTopologyV3PlanAndRecoveryAreProviderFree(t *testing.T) {
	root := t.TempDir()
	plan, err := topologyV3Plan(root)
	if err != nil {
		t.Fatal(err)
	}
	next := plan["next_observation"].(*agentqualification.TopologyExperimentObservation)
	if next == nil || next.ObservationID != "v3-r03-p1-direct" || plan["provider_called"] != false {
		t.Fatalf("unexpected provider-free plan: %+v", plan)
	}
	recovered, err := recoverTopologyV3Observation(context.Background(), root)
	if err != nil || recovered["status"] != "nothing_to_recover" || recovered["provider_called"] != false {
		t.Fatalf("empty recovery was not a provider-free no-op: %+v err=%v", recovered, err)
	}
}

func TestTopologyV3WrongObservationFailsBeforeClaimAndProvider(t *testing.T) {
	root := t.TempDir()
	_, err := executeTopologyV3Observation(context.Background(), root, "not-the-next-observation")
	if err == nil {
		t.Fatal("wrong observation was admitted")
	}
	if _, statErr := os.Stat(filepath.Join(root, "campaign.json")); !os.IsNotExist(statErr) {
		t.Fatal("invalid execution wrote a claim before admission")
	}
}

func topologyV3ReviewerEnvelope(text, selectedProvider string) ([]byte, []byte) {
	jsonl := []byte(strings.Join([]string{
		`{"type":"step_start","timestamp":1000,"sessionID":"ses","part":{"sessionID":"ses","type":"step-start"}}`,
		`{"type":"text","timestamp":1100,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"text","text":` + string(mustJSON(text)) + `}}`,
		`{"type":"step_finish","timestamp":1200,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":0,"write":0}}}}`,
	}, "\n"))
	export := []byte(`{"info":{"id":"ses"},"messages":[{"info":{"id":"msg","sessionID":"ses","role":"assistant","modelID":"anthropic/claude-sonnet-5","providerID":"openrouter","summary":false,"finish":"stop","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":0,"write":0}},"error":null},"parts":[{"id":"text","sessionID":"ses","messageID":"msg","type":"text","text":` + string(mustJSON(text)) + `,"metadata":{"openrouter_metadata":{"endpoints":{"available":[{"provider":` + string(mustJSON(selectedProvider)) + `,"selected":true}]}}}},{"id":"finish","sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":0,"write":0}}}]}]}`)
	return jsonl, export
}

func mustJSON(value string) []byte { raw, _ := json.Marshal(value); return raw }

func TestTopologyV3MalformedAndProviderMismatchReviewerResponsesRemainSealed(t *testing.T) {
	definition, err := agentqualification.CurrentTopologyExperimentV2()
	if err != nil {
		t.Fatal(err)
	}
	routing := realisticOpenCodeRoutingEvidence{ResolvedConfigSHA256: strings.Repeat("a", 64), InlineConfigSHA256: strings.Repeat("b", 64), DirectProvider: "openrouter", RequestedUpstreamProvider: "anthropic", ProviderOrder: []string{"anthropic"}}
	for _, tc := range []struct {
		name, text, provider string
		wantRouteError       bool
	}{{"malformed", "not-json", "Anthropic", false}, {"provider-mismatch", `{}`, "Other Provider", true}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			jsonl, export := topologyV3ReviewerEnvelope(tc.text, tc.provider)
			_, candidate, stage, sealErr := sealAndParseTopologyReviewResponse(root, definition, routing, jsonl, export, agentqualification.TopologyStageReceipt{RunID: "run", Stage: "review_1"}, time.Now())
			if tc.wantRouteError != (sealErr != nil) {
				t.Fatalf("unexpected route result: %v", sealErr)
			}
			if stage.ProviderCalls != 1 || stage.InputTokens+stage.OutputTokens+stage.ReasoningTokens != 16 || stage.CostMicros != 1000 {
				t.Fatalf("paid usage was lost: %+v", stage)
			}
			if _, err := os.Stat(filepath.Join(root, "provider-receipt.json")); err != nil {
				t.Fatal("paid response was not sealed before validation")
			}
			if !tc.wantRouteError {
				if _, err := agentqualification.ParseTopologyReviewVerdict(candidate); err == nil {
					t.Fatal("malformed reviewer unexpectedly passed")
				}
			}
		})
	}
}

func TestTopologyV3CrashRecoveryReconstructsPaidStageAccounting(t *testing.T) {
	root := t.TempDir()
	jsonl, export := topologyV3ReviewerEnvelope("not-json", "Anthropic")
	reviewRoot := filepath.Join(root, "review-1")
	if err := os.MkdirAll(reviewRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(realisticRawProviderReceipt{JSONL: string(jsonl), Export: export})
	if err := os.WriteFile(filepath.Join(reviewRoot, "provider-receipt.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeState := topologyV3ObservationRuntime{ClaimID: "claim", Controller: agentqualification.RealisticControllerResult{ProviderCalls: 2, InputTokens: 20, OutputTokens: 8, ReasoningTokens: 4, CostMicros: 2000}}
	receipts, exact := reconstructTopologyV3CrashReceipts(root, runtimeState)
	result := aggregateTopologyResult(agentqualification.TopologyExperimentResult{StageReceipts: receipts})
	if !exact || result.ProviderCalls != 3 || result.TotalTokens != 48 || result.CostMicros != 3000 {
		t.Fatalf("durable paid stages were not reconstructed: exact=%v result=%+v", exact, result)
	}
}

func TestTopologyV3AdmissionRejectsTopologyV2CommandByteDrift(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	sourceDir := filepath.Join(filepath.Dir(current), "..", "topology-run-v2")
	temp := t.TempDir()
	expected := map[string]string{}
	for _, name := range []string{"topology_v2_cli.go", "main.go"} {
		raw, err := os.ReadFile(filepath.Join(sourceDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(temp, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		expected[name] = sha256Hex(raw)
	}
	if err := verifyTopologyCommandSourcesAt(temp, expected["topology_v2_cli.go"], expected["main.go"]); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temp, "topology_v2_cli.go"), []byte("drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyTopologyCommandSourcesAt(temp, expected["topology_v2_cli.go"], expected["main.go"]); err == nil {
		t.Fatal("topology-v1 source drift was admitted")
	}
}
