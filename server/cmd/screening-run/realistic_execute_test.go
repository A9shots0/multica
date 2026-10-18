package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestTranslateRealisticOpenCodeArtifactBindsIdentityUsageAndRawEvidence(t *testing.T) {
	candidate := agentqualification.RealisticCandidate{Model: "openrouter/test/model", ServingProvider: "fixture provider"}
	rawJSONL := []byte(strings.Join([]string{
		`{"type":"step_start","timestamp":1000,"sessionID":"ses","part":{"sessionID":"ses","type":"step-start"}}`,
		`{"type":"text","timestamp":1100,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"read\",\"path\":\"TASK.md\"}}"}}`,
		`{"type":"step_finish","timestamp":1200,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}}`,
	}, "\n"))
	rawExport := []byte(`{"info":{"id":"ses"},"messages":[{"info":{"id":"msg","sessionID":"ses","role":"assistant","modelID":"test/model","providerID":"openrouter","summary":false,"finish":"stop","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}},"error":null},"parts":[{"id":"text","sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"read\",\"path\":\"TASK.md\"}}","metadata":{"openrouter_metadata":{"endpoints":{"available":[{"provider":"Fixture Provider","selected":true}]}}}},{"id":"finish","sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}]}]}`)

	response, err := translateRealisticOpenCodeArtifact(candidate, fixtureRealisticRouting(candidate), rawJSONL, rawExport)
	if err != nil {
		t.Fatal(err)
	}
	if response.Action.Kind != "read" || response.Action.Path != "TASK.md" || response.InputTokens != 13 || response.OutputTokens != 4 || response.ReasoningTokens != 2 || response.CostMicros != 1000 {
		t.Fatalf("unexpected translated response: %+v", response)
	}
	if !strings.Contains(string(response.RawReceipt), `"jsonl"`) || !strings.Contains(string(response.RawReceipt), `"export"`) || !strings.Contains(string(response.RawReceipt), `"resolved_config_sha256"`) || !strings.Contains(string(response.RawReceipt), `"actual_upstream_provider_state":"reported_by_openrouter_metadata"`) {
		t.Fatal("raw provider evidence was not preserved")
	}
}

func TestRealisticOpenCodeRoutingEvidenceComesFromValidatedResolvedConfig(t *testing.T) {
	matrix, err := agentqualification.CurrentRealisticRunMatrix()
	if err != nil {
		t.Fatal(err)
	}
	candidate := matrix.Candidates[0]
	plan, err := agentqualification.BuildOpenCodeRealisticActionPlan(candidate, agentqualification.RealisticProviderRequest{
		ProviderCallID: "fixture-call", ProviderTurn: 1, CaseID: "R01", RunFamilyID: "fixture-family", Attempt: 1,
		Task: "fixture", AllowedActions: []string{"read"}, MaxSteps: 15,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := realisticOpenCodeRoutingEvidenceFor(plan, plan.InlineConfig)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.ResolvedConfigSHA256 == "" || evidence.InlineConfigSHA256 != plan.InlineConfigSHA256 || evidence.RequestedUpstreamProvider != candidate.ServingProvider || len(evidence.ProviderOrder) != 1 || evidence.ProviderOrder[0] != candidate.ServingProvider || evidence.AllowFallbacks {
		t.Fatalf("validated route evidence is incomplete: %+v", evidence)
	}
	tampered := []byte(strings.Replace(string(plan.InlineConfig), `"allow_fallbacks":false`, `"allow_fallbacks":true`, 1))
	if _, err := realisticOpenCodeRoutingEvidenceFor(plan, tampered); err == nil {
		t.Fatal("fallback-enabled resolved config produced trusted route evidence")
	}
}

func TestTranslateRealisticOpenCodeArtifactPreservesUnavailableUpstreamTruth(t *testing.T) {
	candidate := agentqualification.RealisticCandidate{Model: "openrouter/test/model", ServingProvider: "fixture provider"}
	rawJSONL := []byte(strings.Join([]string{
		`{"type":"step_start","timestamp":1000,"sessionID":"ses","part":{"sessionID":"ses","type":"step-start"}}`,
		`{"type":"text","timestamp":1100,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"read\",\"path\":\"TASK.md\"}}"}}`,
		`{"type":"step_finish","timestamp":1200,"sessionID":"ses","part":{"sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}}`,
	}, "\n"))
	rawExport := []byte(`{"info":{"id":"ses"},"messages":[{"info":{"id":"msg","sessionID":"ses","role":"assistant","modelID":"test/model","providerID":"openrouter","summary":false,"finish":"stop","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}},"error":null},"parts":[{"id":"text","sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"read\",\"path\":\"TASK.md\"}}"},{"id":"finish","sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}]}]}`)

	response, err := translateRealisticOpenCodeArtifact(candidate, fixtureRealisticRouting(candidate), rawJSONL, rawExport)
	if err != nil {
		t.Fatal(err)
	}
	var envelope realisticRawProviderReceipt
	if err := json.Unmarshal(response.RawReceipt, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Routing.ActualUpstreamProvider != "" || envelope.Routing.ActualUpstreamProviderState != "unavailable_from_provider_metadata" || envelope.Routing.DirectProvider != "openrouter" || envelope.Routing.RequestedUpstreamProvider != candidate.ServingProvider || envelope.Routing.AllowFallbacks {
		t.Fatalf("unavailable upstream truth was not preserved: %+v", envelope.Routing)
	}
}

func fixtureRealisticRouting(candidate agentqualification.RealisticCandidate) realisticOpenCodeRoutingEvidence {
	return realisticOpenCodeRoutingEvidence{
		ResolvedConfigSHA256: strings.Repeat("a", 64), InlineConfigSHA256: strings.Repeat("b", 64),
		DirectProvider: "openrouter", DirectProviderEvidence: "opencode_export_provider_id",
		RequestedUpstreamProvider: candidate.ServingProvider, ProviderOrder: []string{candidate.ServingProvider},
		AllowFallbacks: false, ActualUpstreamProviderState: "unavailable_from_provider_metadata",
	}
}

func TestRealisticPartialProviderResponsePreservesJSONLAndKnownUsage(t *testing.T) {
	rawJSONL := []byte("{\"type\":\"step_start\"}\n{\"type\":\"step_finish\",\"part\":{\"cost\":0.0019,\"tokens\":{\"input\":12,\"output\":4,\"reasoning\":2,\"cache\":{\"read\":3,\"write\":1}}}}")
	response := realisticPartialProviderResponse(rawJSONL, []byte(`{"partial":true}`), errors.New("transport interrupted"))
	if response.AccountingStatus != "partial" || response.TokenAccountingStatus != "reported" || response.CostAccountingStatus != "reported" || response.InputTokens != 16 || response.OutputTokens != 4 || response.ReasoningTokens != 2 || response.CostMicros != 1900 {
		t.Fatalf("known partial usage was not preserved: %+v", response)
	}
	if !strings.Contains(string(response.RawReceipt), `step_finish`) || !strings.Contains(string(response.RawReceipt), `transport interrupted`) {
		t.Fatalf("raw partial transport evidence was lost: %s", response.RawReceipt)
	}
}

func TestWriteJSONAtomicReplacesOnlyCompleteCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "run-state.json")
	if err := writeJSONAtomic(path, map[string]int{"provider_calls": 1}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(path, map[string]int{"provider_calls": 2}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]int
	if err := json.Unmarshal(raw, &state); err != nil || state["provider_calls"] != 2 {
		t.Fatalf("atomic checkpoint was incomplete: state=%v err=%v", state, err)
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), filepath.Base(path)+".tmp-*"))
	if err != nil || len(temps) != 0 {
		t.Fatalf("checkpoint temp artifact leaked: %v err=%v", temps, err)
	}
}

func TestRealisticCLIInitializesR04CheckpointWithGeneratedRunFamily(t *testing.T) {
	runFamilyID, err := newRunFamilyID("fixture-candidate", "R04")
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(parent, "R04", runFamilyID)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		RunFamilyID string `json:"run_family_id"`
	}
	raw, err := os.ReadFile(filepath.Join(workspace.Root, "state", "checkpoint.json"))
	if err != nil || json.Unmarshal(raw, &checkpoint) != nil {
		t.Fatalf("CLI-prepared R04 checkpoint is unreadable: %v", err)
	}
	if checkpoint.RunFamilyID != runFamilyID {
		t.Fatalf("CLI run family and R04 checkpoint diverged: run=%s checkpoint=%s", runFamilyID, checkpoint.RunFamilyID)
	}
	if workspace.StartingTreeSHA256 == workspace.Fixture.TreeSHA256 {
		t.Fatal("run-specific R04 starting tree was not distinguished from the frozen fixture template")
	}
}

func TestCampaignMutationSerializesConcurrentAccounting(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	const workers = 12
	var wait sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := mutateRealisticCampaign(root, func(state agentqualification.RealisticCampaignState) (agentqualification.RealisticCampaignState, error) {
				state.TotalProviderCalls++
				state.TotalTokens += 10
				return state, nil
			})
			errorsSeen <- err
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := loadRealisticCampaign(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != workers || state.TotalProviderCalls != workers || state.TotalTokens != workers*10 {
		t.Fatalf("concurrent campaign accounting was lost: %+v", state)
	}
}

func TestCampaignClaimAllowsOnlyOneConcurrentObservationOwner(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	matrix, _ := agentqualification.CurrentRealisticRunMatrix()
	candidateID := matrix.Candidates[0].ConfigurationID
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, claimID := range []string{"claim-a", "claim-b"} {
		wait.Add(1)
		go func(id string) {
			defer wait.Done()
			_, err := claimRealisticObservation(root, matrix, candidateID, "R01", id)
			results <- err
		}(claimID)
	}
	wait.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("exclusive observation claim had %d winners", successes)
	}
}

func TestTranslateRealisticOpenCodeArtifactRejectsServingProviderDrift(t *testing.T) {
	candidate := agentqualification.RealisticCandidate{Model: "openrouter/test/model", ServingProvider: "wrong"}
	rawJSONL := []byte("not a provider receipt")
	if _, err := translateRealisticOpenCodeArtifact(candidate, fixtureRealisticRouting(candidate), rawJSONL, []byte(`{}`)); err == nil {
		t.Fatal("invalid transport should be rejected")
	}
}

func TestTranslateRealisticOpenCodeArtifactRejectsReportedUpstreamDrift(t *testing.T) {
	candidate := agentqualification.RealisticCandidate{Model: "openrouter/test/model", ServingProvider: "expected"}
	rawJSONL := []byte("{\"type\":\"step_start\",\"timestamp\":1000,\"sessionID\":\"ses\",\"part\":{\"sessionID\":\"ses\",\"type\":\"step-start\"}}\n{\"type\":\"text\",\"timestamp\":1100,\"sessionID\":\"ses\",\"part\":{\"sessionID\":\"ses\",\"messageID\":\"msg\",\"type\":\"text\",\"text\":\"{\\\"action\\\":{\\\"kind\\\":\\\"finish\\\",\\\"summary\\\":\\\"done\\\"}}\"}}\n{\"type\":\"step_finish\",\"timestamp\":1200,\"sessionID\":\"ses\",\"part\":{\"sessionID\":\"ses\",\"messageID\":\"msg\",\"type\":\"step-finish\",\"cost\":0.001,\"tokens\":{\"input\":10,\"output\":4,\"reasoning\":2,\"cache\":{\"read\":3,\"write\":0}}}}")
	rawExport := []byte(`{"info":{"id":"ses"},"messages":[{"info":{"id":"msg","sessionID":"ses","role":"assistant","modelID":"test/model","providerID":"openrouter","summary":false,"finish":"stop","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}},"error":null},"parts":[{"id":"text","sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"finish\",\"summary\":\"done\"}}","metadata":{"openrouter_metadata":{"endpoints":{"available":[{"provider":"different","selected":true}]}}}},{"id":"finish","sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}]}]}`)
	response, err := translateRealisticOpenCodeArtifact(candidate, fixtureRealisticRouting(candidate), rawJSONL, rawExport)
	if !errors.Is(err, agentqualification.ErrRealisticEvidenceIntegrity) {
		t.Fatalf("reported upstream drift was not rejected: %v", err)
	}
	var receipt realisticRawProviderReceipt
	if json.Unmarshal(response.RawReceipt, &receipt) != nil || receipt.Routing.RequestedUpstreamProvider != "expected" || receipt.Routing.ActualUpstreamProvider != "different" || receipt.Routing.AllowFallbacks || receipt.Routing.ResolvedConfigSHA256 == "" || receipt.Routing.InlineConfigSHA256 == "" {
		t.Fatalf("provider drift failure did not preserve requested and actual routing evidence: %+v", receipt.Routing)
	}
}
