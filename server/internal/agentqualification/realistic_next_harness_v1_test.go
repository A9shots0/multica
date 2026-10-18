package agentqualification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type realisticNextFakeProvider struct {
	actions      []RealisticCandidateAction
	preflightErr error
	err          error
	calls        int
}

func (provider *realisticNextFakeProvider) Preflight(context.Context) error {
	return provider.preflightErr
}

func (provider *realisticNextFakeProvider) Next(_ context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	provider.calls++
	response := RealisticProviderResponse{InputTokens: 999, OutputTokens: 999, ReasoningTokens: 999, CostMicros: 999, AccountingStatus: "partial", TokenAccountingStatus: "reported", CostAccountingStatus: "reported", RawReceipt: realisticNextFakeRawReceipt(provider.calls, request.CaseID)}
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

func realisticNextFakeRawReceipt(call int, caseID string) []byte {
	session, message := fmt.Sprintf("session-%s-%d", strings.ToLower(caseID), call), fmt.Sprintf("message-%d", call)
	jsonl := fmt.Sprintf("{\"type\":\"step_start\",\"timestamp\":%d,\"sessionID\":%q,\"part\":{\"messageID\":%q,\"sessionID\":%q,\"type\":\"step-start\"}}\n{\"type\":\"text\",\"timestamp\":%d,\"sessionID\":%q,\"part\":{\"messageID\":%q,\"sessionID\":%q,\"type\":\"text\",\"text\":\"{\\\"ok\\\":true}\"}}\n{\"type\":\"step_finish\",\"timestamp\":%d,\"sessionID\":%q,\"part\":{\"messageID\":%q,\"sessionID\":%q,\"type\":\"step-finish\",\"cost\":0.000003,\"tokens\":{\"input\":10,\"output\":2,\"reasoning\":1,\"cache\":{\"read\":0,\"write\":0}}}}\n", 1000+call*10, session, message, session, 1001+call*10, session, message, session, 1002+call*10, session, message, session)
	exported := fmt.Sprintf(`{"info":{"id":%q},"messages":[{"info":{"id":%q,"sessionID":%q,"role":"assistant","modelID":"fake-v1","providerID":"fake","summary":false,"finish":"stop","cost":0.000003,"tokens":{"input":10,"output":2,"reasoning":1,"cache":{"read":0,"write":0}},"error":null},"parts":[{"messageID":%q,"sessionID":%q,"type":"text","text":"{\"ok\":true}"}]}]}`, session, message, session, message, session)
	raw, _ := json.Marshal(realisticRawProviderEnvelope{JSONL: jsonl, Export: json.RawMessage(exported)})
	return raw
}

func realisticNextTestLimits(maxSteps int) RealisticNextLimits {
	return RealisticNextLimits{MaxSteps: maxSteps, MaxInputTokens: 10000, MaxOutputTokens: 10000, MaxReasoningTokens: 10000, MaxCostMicros: 10000}
}

func realisticNextR05PassingActions() []RealisticCandidateAction {
	doc := "# Canonical truth\n\nDEC-R05-001\nStatus: resumed\nProduction changed: false\nPhone Farm accessed: false\n"
	return []RealisticCandidateAction{
		{Kind: "read", Path: "state/owner-decision.json"},
		{Kind: "search", Query: "must_not_repeat"},
		{Kind: "edit", Path: "business/decision_policy.json", Contents: realisticNextR05ValidPolicyJSON()},
		{Kind: "run_check", Check: R05V1SimulateCheck},
		{Kind: "edit", Path: "docs/HANDOFF.md", Contents: doc},
		{Kind: "edit", Path: "docs/CURRENT_STATE.md", Contents: doc},
		{Kind: "edit", Path: "docs/SYSTEM-MAP.md", Contents: doc},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "DEC-R05-001 proves checkpoint CHK-R05-001 was not repeated; stale chat was not authority.\n"},
		{Kind: "run_check", Check: R05V1VerifyCheck},
		{Kind: "finish", Summary: "Owner decision applied and canonical documentation reconciled."},
	}
}

func realisticNextR06PassingActions() []RealisticCandidateAction {
	return []RealisticCandidateAction{
		{Kind: "edit", Path: "crm/contract.json", Contents: `{"contract":"account.assignment","version":2,"migration":42,"authority":"crm","fields":["account_id","authority_revision"]}`},
		{Kind: "edit", Path: "direct_linkhub/consumer.json", Contents: `{"consumer":"linkhub","contract":"account.assignment","accepted_versions":[1,2],"requires_field":"authority_revision"}`},
		{Kind: "edit", Path: "direct_canopy/consumer.json", Contents: `{"consumer":"canopy","contract":"account.assignment","accepted_versions":[1,2],"requires_field":"authority_revision"}`},
		{Kind: "edit", Path: "tests/expectation.json", Contents: `{"contract":"account.assignment","version":2,"migration":42,"direct_consumers":["linkhub","canopy"]}`},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "CRM then LinkHub then Canopy ordered release. Candidate was deployed but not Live. Authenticated readback failed. Rollback verified the healthy baseline and the requested outcome remains open. Viral Queue is a verification-radius-only historical control and remained unchanged. Phone Farm remained read only, unchanged, and was not accessed.\n"},
		{Kind: "run_check", Check: R06V1ContractCheck},
		{Kind: "run_check", Check: R06V1ReleaseCRM},
		{Kind: "run_check", Check: R06V1ReleaseLinkHub},
		{Kind: "run_check", Check: R06V1ReleaseCanopy},
		{Kind: "run_check", Check: R06V1AuthenticatedVerify},
		{Kind: "run_check", Check: R06V1Rollback},
		{Kind: "run_check", Check: R06V1VerifyBaseline},
		{Kind: "finish", Summary: "Candidate failed authenticated verification; baseline verified and outcome remains open."},
	}
}

func TestRealisticNextHarnessR05PassesWithExactTraceAndImmutableReceipt(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticNextR05V1(parent)
	if err != nil {
		t.Fatal(err)
	}
	provider := &realisticNextFakeProvider{actions: realisticNextR05PassingActions()}
	receipts := filepath.Join(parent, "receipts")
	batch, err := RunRealisticNextBatch(context.Background(), []RealisticNextBinding{{Workspace: workspace, RunFamilyID: "r05-pass", Provider: provider}}, realisticNextTestLimits(12), receipts, acceptInMemoryRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Halted || len(batch.Runs) != 1 || batch.Runs[0].Controller.TerminalState != "passed" {
		t.Fatalf("R05 did not pass: %+v", batch)
	}
	run := batch.Runs[0]
	if len(run.Controller.ToolResults) != 9 || len(run.Controller.ToolEvents) != 9 || batch.ProviderCalls != 10 || batch.InputTokens != 100 || batch.OutputTokens != 20 || batch.ReasoningTokens != 10 || batch.CostMicros != 30 {
		t.Fatalf("R05 trace/accounting drifted: %+v", batch)
	}
	if run.Controller.ProviderAccountingStatus != "exact" || run.Controller.ProviderTokenAccountingStatus != "exact" || run.Controller.ProviderCostAccountingStatus != "exact" {
		t.Fatalf("successful receipt accounting was not derived and closed exact: %+v", run.Controller)
	}
	if got := run.Controller.ToolEvents[3].TouchedPaths; len(got) != 1 || got[0] != "state/final-state.json" {
		t.Fatalf("R05 generated-state trace is not exact: %v", got)
	}
	receipt, err := os.ReadFile(run.ReceiptPath)
	if err != nil || len(receipt) == 0 || run.ReceiptHash == "" {
		t.Fatalf("R05 receipt missing: path=%s hash=%s err=%v", run.ReceiptPath, run.ReceiptHash, err)
	}
	if _, _, err := SealRealisticNextReceipt(receipts, run); err == nil {
		t.Fatal("immutable receipt was overwritten")
	}
}

func TestRealisticNextHarnessR05RejectsCandidateArtifactsWithoutExecutedChecks(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticNextR05V1(parent)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		t.Fatal(err)
	}
	rawFinal, _ := json.MarshalIndent(fixture.ExpectedFinalState, "", "  ")
	doc := "DEC-R05-001\nStatus: resumed\nProduction changed: false\nPhone Farm accessed: false\n"
	provider := &realisticNextFakeProvider{actions: []RealisticCandidateAction{
		{Kind: "edit", Path: "business/decision_policy.json", Contents: realisticNextR05ValidPolicyJSON()},
		{Kind: "edit", Path: "state/final-state.json", Contents: string(rawFinal) + "\n"},
		{Kind: "edit", Path: "docs/HANDOFF.md", Contents: doc},
		{Kind: "edit", Path: "docs/CURRENT_STATE.md", Contents: doc},
		{Kind: "edit", Path: "docs/SYSTEM-MAP.md", Contents: doc},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "DEC-R05-001 governed the decision; checkpoint CHK-R05-001 was not repeated; stale chat was not authority. Fake checks passed.\n"},
		{Kind: "finish", Summary: "all checks passed"},
	}}
	run, err := RunRealisticNextCandidate(context.Background(), workspace, "r05-forged-proof", provider, realisticNextTestLimits(8), acceptInMemoryRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if run.Controller.TerminalState != "failed" || run.Controller.FailureClass != "competence_failure" || !strings.Contains(run.Controller.FinalSummary, "missing_harness_check_receipts") {
		t.Fatalf("candidate-authored evidence passed without executed checks: %+v", run)
	}
}

func TestRealisticNextHarnessR06PassesThroughFailedVerificationAndRollback(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticNextR06V1(parent)
	if err != nil {
		t.Fatal(err)
	}
	provider := &realisticNextFakeProvider{actions: realisticNextR06PassingActions()}
	run, err := RunRealisticNextCandidate(context.Background(), workspace, "r06-pass", provider, realisticNextTestLimits(16), acceptInMemoryRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if run.Controller.TerminalState != "passed" || run.GlobalHalt || provider.calls != 13 {
		t.Fatalf("R06 did not pass: %+v calls=%d", run, provider.calls)
	}
	if len(run.Controller.ToolResults) != 12 || run.Controller.ToolResults[9].Success {
		t.Fatalf("R06 exact trace did not preserve the expected failed authenticated check: %+v", run.Controller.ToolResults)
	}
	if got := run.Controller.ToolEvents[10].TouchedPaths; len(got) != 2 || got[0] != "release/state.json" || got[1] != "release/receipt.json" {
		t.Fatalf("R06 rollback trace is not exact: %v", got)
	}
	state, err := readR06V1State(workspace.Root)
	if err != nil || !state.Rollback.Performed || !state.Rollback.Verified || state.Live || state.OutcomeStatus != "open" {
		t.Fatalf("R06 final state is not truthful: %+v err=%v", state, err)
	}
}

func TestRealisticNextBatchContinuesAfterCandidateProtocolFailure(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	first, _ := PrepareRealisticNextR05V1(parent)
	second, _ := PrepareRealisticNextR05V1(parent)
	bad := &realisticNextFakeProvider{actions: []RealisticCandidateAction{{Kind: "edit", Path: "state/owner-decision.json", Contents: `{}`}}}
	incomplete := &realisticNextFakeProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "done"}}}
	batch, err := RunRealisticNextBatch(context.Background(), []RealisticNextBinding{{Workspace: first, RunFamilyID: "bad-protocol", Provider: bad}, {Workspace: second, RunFamilyID: "bad-competence", Provider: incomplete}}, realisticNextTestLimits(3), filepath.Join(parent, "receipts"), acceptInMemoryRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Halted || len(batch.Runs) != 2 || batch.Runs[0].Controller.FailureClass != "response_or_tool_protocol_failure" || batch.Runs[1].Controller.FailureClass != "competence_failure" || batch.ProviderCalls != 2 || batch.CostMicros != 6 {
		t.Fatalf("candidate-local failure incorrectly halted batch or accounting drifted: %+v", batch)
	}
}

func TestRealisticNextBatchGloballyHaltsOnlyForCampaignIntegrityClasses(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider *realisticNextFakeProvider
		limits   RealisticNextLimits
		want     string
	}{
		{name: "credential", provider: &realisticNextFakeProvider{preflightErr: ErrRealisticCredentialFailure}, limits: realisticNextTestLimits(3), want: "credential_failure"},
		{name: "unsafe", provider: &realisticNextFakeProvider{err: ErrRealisticNextUnsafeExecution}, limits: realisticNextTestLimits(3), want: "unsafe_execution"},
		{name: "evidence", provider: &realisticNextFakeProvider{err: ErrRealisticEvidenceIntegrity}, limits: realisticNextTestLimits(3), want: "evidence_integrity_failure"},
		{name: "ambiguous", provider: &realisticNextFakeProvider{err: errors.New("transport ended without outcome")}, limits: realisticNextTestLimits(3), want: "ambiguous_transport"},
		{name: "budget", provider: &realisticNextFakeProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "done"}}}, limits: RealisticNextLimits{MaxSteps: 3, MaxCostMicros: 2}, want: "budget_breach"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, _ := filepath.Abs(t.TempDir())
			first, _ := PrepareRealisticNextR05V1(parent)
			second, _ := PrepareRealisticNextR05V1(parent)
			unused := &realisticNextFakeProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "done"}}}
			batch, err := RunRealisticNextBatch(context.Background(), []RealisticNextBinding{{Workspace: first, RunFamilyID: test.name + "-first", Provider: test.provider}, {Workspace: second, RunFamilyID: test.name + "-second", Provider: unused}}, test.limits, filepath.Join(parent, "receipts"), acceptInMemoryRealisticCheckpoint)
			if err != nil {
				t.Fatal(err)
			}
			wantCost := int64(0)
			wantCalls := 1
			if test.name == "credential" {
				wantCalls = 0
			}
			if test.name == "ambiguous" || test.name == "budget" {
				wantCost = 3
			}
			if !batch.Halted || batch.HaltReason != test.want || len(batch.Runs) != 1 || unused.calls != 0 || batch.ProviderCalls != wantCalls || batch.CostMicros != wantCost {
				t.Fatalf("global halt/accounting failed: %+v unused=%d", batch, unused.calls)
			}
			if _, err := os.Stat(batch.Runs[0].ReceiptPath); err != nil || !strings.HasSuffix(batch.Runs[0].ReceiptPath, "receipt.json") {
				t.Fatalf("terminal receipt missing after global halt: %s err=%v", batch.Runs[0].ReceiptPath, err)
			}
		})
	}
}

func TestRealisticNextPreflightAndInvalidFixtureAreZeroCall(t *testing.T) {
	for _, test := range []struct {
		name          string
		prepare       func(string) (RealisticNextWorkspace, error)
		mutate        func(RealisticNextWorkspace)
		preflight     error
		nextErr       error
		wantNextCalls int
		wantFailure   string
	}{
		{name: "provider_preflight", prepare: PrepareRealisticNextR05V1, preflight: ErrRealisticProviderPreflight, wantFailure: "provider_preflight_failure"},
		{name: "next_credential_preflight", prepare: PrepareRealisticNextR05V1, nextErr: ErrRealisticCredentialFailure, wantNextCalls: 1, wantFailure: "credential_failure"},
		{name: "next_transport_preflight", prepare: PrepareRealisticNextR05V1, nextErr: ErrRealisticProviderPreflight, wantNextCalls: 1, wantFailure: "provider_preflight_failure"},
		{name: "invalid_fixture", prepare: PrepareRealisticNextR06V1, mutate: func(workspace RealisticNextWorkspace) {
			_ = os.WriteFile(filepath.Join(workspace.InitialCopy, "TASK.md"), []byte("drift"), 0o600)
		}, wantFailure: "evidence_integrity_failure"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, _ := filepath.Abs(t.TempDir())
			workspace, err := test.prepare(parent)
			if err != nil {
				t.Fatal(err)
			}
			if test.mutate != nil {
				test.mutate(workspace)
			}
			provider := &realisticNextFakeProvider{preflightErr: test.preflight, err: test.nextErr}
			run, err := RunRealisticNextCandidate(context.Background(), workspace, test.name, provider, realisticNextTestLimits(3), acceptInMemoryRealisticCheckpoint)
			if err != nil {
				t.Fatal(err)
			}
			if run.Controller.ProviderCalls != 0 || provider.calls != test.wantNextCalls || run.Controller.FailureClass != test.wantFailure || run.Controller.ProviderAccountingStatus != "exact" || run.Controller.ProviderTokenAccountingStatus != "exact" || run.Controller.ProviderCostAccountingStatus != "exact" {
				t.Fatalf("pre-request failure was not zero-call or was misclassified: %+v calls=%d", run, provider.calls)
			}
		})
	}
}

func TestRealisticNextBatchBudgetIsCumulativeAcrossBindings(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	first, _ := PrepareRealisticNextR05V1(parent)
	second, _ := PrepareRealisticNextR05V1(parent)
	one := &realisticNextFakeProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "incomplete"}}}
	two := &realisticNextFakeProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "incomplete"}}}
	batch, err := RunRealisticNextBatch(context.Background(), []RealisticNextBinding{{Workspace: first, RunFamilyID: "cumulative-1", Provider: one}, {Workspace: second, RunFamilyID: "cumulative-2", Provider: two}}, RealisticNextLimits{MaxSteps: 2, MaxCostMicros: 5}, filepath.Join(parent, "receipts"), acceptInMemoryRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if !batch.Halted || batch.HaltReason != "budget_breach" || len(batch.Runs) != 2 || batch.ProviderCalls != 2 || batch.CostMicros != 6 || one.calls != 1 || two.calls != 1 {
		t.Fatalf("batch budget was not enforced cumulatively: %+v", batch)
	}
}
