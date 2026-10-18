package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type realisticNextCommandFakeProvider struct {
	actions []agentqualification.RealisticCandidateAction
	err     error
	calls   int
	key     string
}

func (provider *realisticNextCommandFakeProvider) Next(_ context.Context, _ agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
	provider.calls++
	response := agentqualification.RealisticProviderResponse{RawReceipt: realisticNextCommandReceipt(provider.key, provider.calls), InputTokens: 999, OutputTokens: 999, ReasoningTokens: 999, CostMicros: 999}
	if provider.err != nil {
		response.InputTokens, response.OutputTokens, response.ReasoningTokens, response.CostMicros = 10, 2, 1, 3
		return response, provider.err
	}
	if provider.calls > len(provider.actions) {
		response.ProtocolFailure = "script exhausted"
		return response, nil
	}
	response.Action = provider.actions[provider.calls-1]
	return response, nil
}

func realisticNextCommandReceipt(key string, call int) []byte {
	session := fmt.Sprintf("next-%s-%d", strings.NewReplacer("/", "-", ".", "-").Replace(key), call)
	message := fmt.Sprintf("message-%d", call)
	jsonl := fmt.Sprintf("{\"type\":\"step_start\",\"timestamp\":1000,\"sessionID\":%q,\"part\":{\"sessionID\":%q,\"messageID\":%q,\"type\":\"step-start\"}}\n{\"type\":\"text\",\"timestamp\":1001,\"sessionID\":%q,\"part\":{\"sessionID\":%q,\"messageID\":%q,\"type\":\"text\",\"text\":\"{\\\"ok\\\":true}\"}}\n{\"type\":\"step_finish\",\"timestamp\":1002,\"sessionID\":%q,\"part\":{\"sessionID\":%q,\"messageID\":%q,\"type\":\"step-finish\",\"cost\":0.000003,\"tokens\":{\"input\":10,\"output\":2,\"reasoning\":1,\"cache\":{\"read\":0,\"write\":0}}}}\n", session, session, message, session, session, message, session, session, message)
	exported := fmt.Sprintf(`{"info":{"id":%q},"messages":[{"info":{"id":%q,"sessionID":%q,"role":"assistant","modelID":"fake","providerID":"openrouter","summary":false,"finish":"stop","cost":0.000003,"tokens":{"input":10,"output":2,"reasoning":1,"cache":{"read":0,"write":0}},"error":null},"parts":[{"sessionID":%q,"messageID":%q,"type":"text","text":"{\"ok\":true}"}]}]}`, session, message, session, session, message)
	raw, _ := json.Marshal(map[string]any{"jsonl": jsonl, "export": json.RawMessage(exported), "routing": map[string]any{"allow_fallbacks": false}})
	return raw
}

func realisticNextCommandR05Pass() []agentqualification.RealisticCandidateAction {
	policy := `{"schema_version":"silver-mediums.r05-decision-policy.v1","authority":"owner_receipt","preserve_accepted":true,"preserve_rejected":true,"preserve_scope":true,"preserve_out_of_scope":true,"status":"resumed","needs_you":false,"repeat_completed_checkpoint":false,"production_changed":false,"phone_farm_accessed":false}`
	doc := "DEC-R05-001. Status: resumed. Production changed: false. Phone Farm accessed: false.\n"
	return []agentqualification.RealisticCandidateAction{
		{Kind: "edit", Path: "business/decision_policy.json", Contents: policy}, {Kind: "run_check", Check: agentqualification.R05V1SimulateCheck}, {Kind: "run_check", Check: agentqualification.R05V1VerifyCheck},
		{Kind: "edit", Path: "docs/HANDOFF.md", Contents: doc}, {Kind: "edit", Path: "docs/CURRENT_STATE.md", Contents: doc}, {Kind: "edit", Path: "docs/SYSTEM-MAP.md", Contents: doc},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "DEC-R05-001 proves checkpoint CHK-R05-001 was not repeated and stale chat was not authority.\n"},
		{Kind: "finish", Summary: "bounded decision resumed"},
	}
}

func realisticNextCommandR06Pass() []agentqualification.RealisticCandidateAction {
	return []agentqualification.RealisticCandidateAction{
		{Kind: "edit", Path: "crm/contract.json", Contents: `{"contract":"account.assignment","version":2,"migration":42,"authority":"crm","fields":["account_id","authority_revision"]}`},
		{Kind: "edit", Path: "direct_linkhub/consumer.json", Contents: `{"consumer":"linkhub","contract":"account.assignment","accepted_versions":[1,2],"requires_field":"authority_revision"}`},
		{Kind: "edit", Path: "direct_canopy/consumer.json", Contents: `{"consumer":"canopy","contract":"account.assignment","accepted_versions":[1,2],"requires_field":"authority_revision"}`},
		{Kind: "edit", Path: "tests/expectation.json", Contents: `{"contract":"account.assignment","version":2,"migration":42,"direct_consumers":["linkhub","canopy"]}`},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "CRM then LinkHub then Canopy ordered release. Candidate was deployed but not Live. Authenticated readback failed. Rollback verified the healthy baseline and the requested outcome remains open. Viral Queue is a verification-radius-only historical control and remained unchanged. Phone Farm remained read only, unchanged, and was not accessed.\n"},
		{Kind: "run_check", Check: agentqualification.R06V1ContractCheck}, {Kind: "run_check", Check: agentqualification.R06V1ReleaseCRM}, {Kind: "run_check", Check: agentqualification.R06V1ReleaseLinkHub}, {Kind: "run_check", Check: agentqualification.R06V1ReleaseCanopy}, {Kind: "run_check", Check: agentqualification.R06V1AuthenticatedVerify}, {Kind: "run_check", Check: agentqualification.R06V1Rollback}, {Kind: "run_check", Check: agentqualification.R06V1VerifyBaseline},
		{Kind: "finish", Summary: "rollback verified and outcome open"},
	}
}

func TestRealisticNextPlanIsProviderFreePinnedAndReviewed(t *testing.T) {
	plan, err := realisticNextPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["provider_ready"] != true || plan["group_status"] != "frozen_reviewed" || plan["matrix_status"] != "frozen_reviewed" {
		t.Fatalf("reviewed provider-free plan is inconsistent: %+v", plan)
	}
	plans := plan["plans"].([]map[string]any)
	if len(plans) != 4 {
		t.Fatalf("provider-free plan omitted matrix bindings: %+v", plans)
	}
	for _, binding := range plans {
		if binding["provider_fallbacks"] != false || binding["model"] == "" || binding["effort"] != "medium" || binding["serving_provider"] == "" {
			t.Fatalf("plan is not exact and no-fallback: %+v", binding)
		}
	}
}

func TestRunRealisticNextMatrixStagesPassesSkipsFailuresAndWritesOnce(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticNextRunMatrix()
	matrix.Status = "frozen_reviewed"
	providers := map[string]*realisticNextCommandFakeProvider{}
	factoryCalls := 0
	factory := func(candidate agentqualification.RealisticCandidate, _, _, _ int64) agentqualification.RealisticProvider {
		factoryCalls++
		key := candidate.ConfigurationID
		var provider *realisticNextCommandFakeProvider
		if candidate.ConfigurationID == matrix.Candidates[0].ConfigurationID && factoryCalls == 1 {
			provider = &realisticNextCommandFakeProvider{key: key + "-r05", actions: realisticNextCommandR05Pass()}
		} else if candidate.ConfigurationID == matrix.Candidates[0].ConfigurationID {
			provider = &realisticNextCommandFakeProvider{key: key + "-r06", actions: realisticNextCommandR06Pass()}
		} else {
			provider = &realisticNextCommandFakeProvider{key: key + "-r05", actions: []agentqualification.RealisticCandidateAction{{Kind: "edit", Path: "state/owner-decision.json", Contents: `{}`}}}
		}
		providers[provider.key] = provider
		return provider
	}
	root, _ := filepath.Abs(t.TempDir())
	result, err := runRealisticNextMatrix(context.Background(), root, matrix, 1000, factory)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || result.Retries != 0 || len(result.Observations) != 4 || factoryCalls != 3 {
		t.Fatalf("staged matrix did not complete exactly once: %+v factory=%d", result, factoryCalls)
	}
	if result.Observations[0].Status != "passed" || result.Observations[1].FailureClass != "response_or_tool_protocol_failure" || result.Observations[2].Status != "passed" || result.Observations[3].Status != "skipped" {
		t.Fatalf("stage pass/failure/skip ordering drifted: %+v", result.Observations)
	}
	if result.ProviderCalls != 22 || result.InputTokens != 220 || result.OutputTokens != 44 || result.ReasoningTokens != 22 || result.CostMicros != 66 {
		t.Fatalf("campaign accounting is not receipt-derived and cumulative: %+v", result)
	}
	for _, record := range result.Observations[:3] {
		if record.Run == nil || record.Run.ReceiptPath == "" || record.Run.ReceiptHash == "" {
			t.Fatalf("observation receipt missing: %+v", record)
		}
		if _, err := os.Stat(record.Run.ReceiptPath); err != nil {
			t.Fatal(err)
		}
	}
	again, err := runRealisticNextMatrix(context.Background(), root, matrix, 1000, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		t.Fatal("immutable completed campaign attempted another provider")
		return nil
	})
	if err != nil || again.ProviderCalls != result.ProviderCalls {
		t.Fatalf("immutable completed result was not reused: %+v err=%v", again, err)
	}
}

func TestRunRealisticNextMatrixContinuesObservationInvalidityButHaltsAmbiguousTransport(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticNextRunMatrix()
	matrix.Status = "frozen_reviewed"
	t.Run("observation local preflight", func(t *testing.T) {
		calls := 0
		factory := func(candidate agentqualification.RealisticCandidate, _, _, _ int64) agentqualification.RealisticProvider {
			calls++
			return &realisticNextCommandFakeProvider{key: candidate.ConfigurationID, err: agentqualification.ErrRealisticProviderPreflight}
		}
		root, _ := filepath.Abs(t.TempDir())
		result, err := runRealisticNextMatrix(context.Background(), root, matrix, 0, factory)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != "complete" || calls != 2 || len(result.Observations) != 4 || result.ProviderCalls != 0 || result.Observations[0].FailureClass != "provider_preflight_failure" || result.Observations[1].FailureClass != "provider_preflight_failure" {
			t.Fatalf("observation invalidity did not continue unaffected R05 bindings: %+v calls=%d", result, calls)
		}
	})
	t.Run("ambiguous global halt", func(t *testing.T) {
		calls := 0
		factory := func(candidate agentqualification.RealisticCandidate, _, _, _ int64) agentqualification.RealisticProvider {
			calls++
			return &realisticNextCommandFakeProvider{key: candidate.ConfigurationID, err: errors.New("ambiguous transport")}
		}
		root, _ := filepath.Abs(t.TempDir())
		result, err := runRealisticNextMatrix(context.Background(), root, matrix, 0, factory)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != "global_halt" || result.HaltReason != "ambiguous_transport" || calls != 1 || len(result.Observations) != 1 || result.ProviderCalls != 1 || result.CostMicros != 3 {
			t.Fatalf("ambiguous transport did not halt exactly: %+v calls=%d", result, calls)
		}
	})
}

func TestRunRealisticNextMatrixFailsClosedOnUnfinishedClaimWithoutProviderCall(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticNextRunMatrix()
	matrix.Status = "frozen_reviewed"
	root, _ := filepath.Abs(t.TempDir())
	candidate := matrix.Candidates[0]
	claim := realisticNextProviderClaim{
		SchemaVersion: realisticNextClaimSchema,
		MatrixID:      matrix.MatrixID,
		CandidateID:   candidate.ConfigurationID,
		CaseID:        "R05",
		RunFamilyID:   realisticNextRunFamilyID(matrix.MatrixID, candidate.ConfigurationID, "R05"),
		Status:        "provider_call_reserved",
	}
	if err := writeJSONExclusive(realisticNextClaimPath(root, candidate.ConfigurationID, "R05"), claim); err != nil {
		t.Fatal(err)
	}
	factoryCalls := 0
	result, err := runRealisticNextMatrix(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		factoryCalls++
		return &realisticNextCommandFakeProvider{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "global_halt" || result.HaltReason != "ambiguous_transport" || result.ProviderCalls != 0 || factoryCalls != 0 {
		t.Fatalf("unfinished durable claim did not fail closed before provider creation: %+v factory=%d", result, factoryCalls)
	}
}

func TestRunRealisticNextMatrixPersistsPricedClaimBeforeProviderCreation(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticNextRunMatrix()
	matrix.Status = "frozen_reviewed"
	matrix.Candidates = matrix.Candidates[:1]
	root, _ := filepath.Abs(t.TempDir())
	candidate := matrix.Candidates[0]
	factoryCalls := 0
	result, err := runRealisticNextMatrix(context.Background(), root, matrix, 0, func(got agentqualification.RealisticCandidate, _, _, reservation int64) agentqualification.RealisticProvider {
		factoryCalls++
		var claim realisticNextProviderClaim
		if err := readJSONFile(realisticNextClaimPath(root, got.ConfigurationID, "R05"), &claim); err != nil {
			t.Fatalf("provider factory ran before durable claim: %v", err)
		}
		if claim.RunFamilyID != realisticNextRunFamilyID(matrix.MatrixID, got.ConfigurationID, "R05") || claim.ReservedCostMicros != reservation || claim.ReservedCostMicros != 465_000 || claim.InputMicrosPerMillion != 3_000_000 || claim.GeneratedMicrosPerMillion != 15_000_000 {
			t.Fatalf("durable claim omitted exact identity or price reservation: %+v", claim)
		}
		return &realisticNextCommandFakeProvider{key: candidate.ConfigurationID, err: agentqualification.ErrRealisticProviderPreflight}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || factoryCalls != 1 || len(result.Observations) != 2 || result.Observations[1].Status != "skipped" {
		t.Fatalf("priced pre-call claim path did not complete locally: %+v factory=%d", result, factoryCalls)
	}
}

func TestRunRealisticNextMatrixReconstructsCompletedObservationsWithoutProviderCalls(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticNextRunMatrix()
	matrix.Status = "frozen_reviewed"
	root, _ := filepath.Abs(t.TempDir())
	factoryCalls := 0
	factory := func(candidate agentqualification.RealisticCandidate, _, _, _ int64) agentqualification.RealisticProvider {
		factoryCalls++
		if candidate.ConfigurationID == matrix.Candidates[0].ConfigurationID {
			if factoryCalls == 1 {
				return &realisticNextCommandFakeProvider{key: "restart-first-r05", actions: realisticNextCommandR05Pass()}
			}
			return &realisticNextCommandFakeProvider{key: "restart-first-r06", actions: realisticNextCommandR06Pass()}
		}
		return &realisticNextCommandFakeProvider{key: "restart-second-r05", actions: []agentqualification.RealisticCandidateAction{{Kind: "edit", Path: "state/owner-decision.json", Contents: `{}`}}}
	}
	first, err := runRealisticNextMatrix(context.Background(), root, matrix, 0, factory)
	if err != nil || first.Status != "complete" {
		t.Fatalf("initial campaign failed: %+v err=%v", first, err)
	}
	if err := os.Remove(filepath.Join(root, "campaign-result.json")); err != nil {
		t.Fatal(err)
	}
	replayedFactoryCalls := 0
	replayed, err := runRealisticNextMatrix(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		replayedFactoryCalls++
		return &realisticNextCommandFakeProvider{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayedFactoryCalls != 0 || replayed.Status != "complete" || replayed.ProviderCalls != first.ProviderCalls || replayed.CostMicros != first.CostMicros || len(replayed.Observations) != len(first.Observations) {
		t.Fatalf("restart did not reconstruct sealed observations exactly: first=%+v replayed=%+v factory=%d", first, replayed, replayedFactoryCalls)
	}
}

func TestRealisticNextWorstCaseAdmissionIsPriceDerivedAndPreCall(t *testing.T) {
	matrix, _ := agentqualification.CurrentRealisticNextRunMatrix()
	matrix.Status = "frozen_reviewed"
	claude, err := realisticNextWorstCaseCostMicros(matrix.Candidates[0], matrix)
	if err != nil || claude != 465_000 {
		t.Fatalf("Claude reservation drifted: %d err=%v", claude, err)
	}
	gpt, err := realisticNextWorstCaseCostMicros(matrix.Candidates[1], matrix)
	if err != nil || gpt != 437_500 {
		t.Fatalf("GPT reservation drifted: %d err=%v", gpt, err)
	}
	matrix.MaxIncrementalSpendMicros = claude - 1
	root, _ := filepath.Abs(t.TempDir())
	factoryCalls := 0
	result, err := runRealisticNextMatrix(context.Background(), root, matrix, 0, func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider {
		factoryCalls++
		return &realisticNextCommandFakeProvider{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "global_halt" || result.HaltReason != "budget_breach" || result.ProviderCalls != 0 || result.CostMicros != 0 || factoryCalls != 0 {
		t.Fatalf("worst-case spend was not reserved before provider creation: %+v factory=%d", result, factoryCalls)
	}
}
