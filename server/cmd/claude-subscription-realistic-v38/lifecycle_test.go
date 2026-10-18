package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type v38ProviderFunc func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error)

func (function v38ProviderFunc) Plan(ctx context.Context, plan agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
	return function(ctx, plan, request)
}

func successfulV38Plan(runFamily string) agentqualification.ClaudeRealisticV37Plan {
	return agentqualification.ClaudeRealisticV37Plan{Actions: []agentqualification.RealisticCandidateAction{
		{Kind: "run_check", Check: "python resume.py"},
		{Kind: "edit", Path: "HANDOFF.md", Contents: "Run family " + runFamily + " resumed from its durable checkpoint. Legacy completion is superseded; remaining verification completed exactly once.\n"},
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "finish", Summary: "Same-family attempt-2 recovery completed exactly once; superseded status rejected."},
	}}
}

func TestV38LifecycleCompletesR04WithHarnessInterruptionThenOnePaidPhase(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	calls := 0
	provider := v38ProviderFunc(func(_ context.Context, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		var preCall v38Ledger
		if err := readJSONV38(filepath.Join(root, "ledger.json"), &preCall); err != nil || preCall.Claim == nil || !preCall.Claim.InFlight || preCall.Claim.RequestSHA256 == "" || preCall.Claim.Usage.ReservedReportedTokens != 60000 || preCall.Claim.Usage.ReservedGeneratedTokens != 12000 || preCall.Claim.Usage.ReservedCostMicros != 250000 {
			t.Fatalf("paid call was not conservatively admitted first: ledger=%+v err=%v", preCall, err)
		}
		candidate := successfulV38Plan(request.RunFamilyID)
		return candidate, fixtureReceiptV38(t, transport, request, candidate, 20, 4, 1), nil
	})
	ledger, err := executeV38(context.Background(), root, provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || ledger.Status != "complete" || ledger.GlobalHalt || ledger.CandidateEliminated || ledger.Claim == nil || ledger.Claim.Outcome != "pass" || ledger.Claim.Usage.ProviderInvocations != 1 || ledger.Claim.Retries != 0 {
		t.Fatalf("calls=%d ledger=%+v", calls, ledger)
	}
	for _, path := range []string{"result.json", "grade.json", filepath.Join("evidence", "execution-receipt.json")} {
		if _, err := os.Stat(filepath.Join(root, "observations", "R04", path)); err != nil {
			t.Fatalf("missing sealed %s: %v", path, err)
		}
	}
	ledger, err = executeV38(context.Background(), root, v38ProviderFunc(func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		return agentqualification.ClaudeRealisticV37Plan{}, nil, errors.New("closed replay called provider")
	}), func() error { return nil })
	if err != nil || calls != 1 || ledger.Status != "complete" {
		t.Fatalf("closed replay failed: calls=%d ledger=%+v err=%v", calls, ledger, err)
	}
}

func TestV38LifecyclePreservesAndReplaysExactProviderFailureAccounting(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	rawStdout := []byte(`{"type":"result","subtype":"success","is_error":false,"result":"{}","structured_output":{},"total_cost_usd":0.000007,"usage":{"input_tokens":11,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":5},"modelUsage":{"claude-sonnet-4-6":{}}}`)
	provider := v38ProviderFunc(func(_ context.Context, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		receipt := buildV38ProviderFailureReceipt(transport, request, rawStdout, []byte("schema rejected"), time.Unix(1, 0).UTC(), time.Unix(2, 0).UTC(), v38Protocol)
		return agentqualification.ClaudeRealisticV37Plan{}, receipt, failV38(v38Protocol, errors.New("schema rejected"))
	})
	ledger, err := executeV38(context.Background(), root, provider, func() error { return nil })
	if err != nil || !ledger.CandidateEliminated || ledger.Claim == nil || ledger.Claim.Outcome != "failed" || ledger.Claim.Usage.ProviderInvocations != 1 || ledger.Claim.Usage.InputTokens != 16 || ledger.Claim.Usage.OutputTokens != 5 || ledger.Claim.Usage.CostMicros != 7 || ledger.Claim.Usage.AmbiguousInvocations != 0 {
		t.Fatalf("exact failed-call accounting lost: ledger=%+v err=%v", ledger, err)
	}
	var preserved v38ProviderFailureReceipt
	if err := readJSONV38(filepath.Join(root, "observations", "R04", "phase-01-failure.json"), &preserved); err != nil || string(preserved.RawStdout) != string(rawStdout) || string(preserved.RawStderr) != "schema rejected" || preserved.AccountingStatus != "exact" {
		t.Fatalf("raw failed-call evidence lost: receipt=%+v err=%v", preserved, err)
	}
	calls := 0
	_, err = executeV38(context.Background(), root, v38ProviderFunc(func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		return agentqualification.ClaudeRealisticV37Plan{}, nil, errors.New("must not call")
	}), func() error { return nil })
	if err != nil || calls != 0 {
		t.Fatalf("exact failure replay failed: calls=%d err=%v", calls, err)
	}
}

func TestV38LifecycleConservativelyAccountsUnparseableProviderFailure(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	rawStdout := []byte("unparseable-but-possibly-billed")
	ledger, err := executeV38(context.Background(), root, v38ProviderFunc(func(_ context.Context, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		receipt := buildV38ProviderFailureReceipt(transport, request, rawStdout, []byte("transport failed"), time.Unix(1, 0).UTC(), time.Unix(2, 0).UTC(), v38Transport)
		return agentqualification.ClaudeRealisticV37Plan{}, receipt, failV38(v38Transport, errors.New("transport failed"))
	}), func() error { return nil })
	if err != nil || ledger.Claim == nil || ledger.Claim.Outcome != "invalid" || ledger.Claim.Usage.AmbiguousInvocations != 1 || ledger.Claim.Usage.ReservedReportedTokens != 60000 || ledger.Claim.Usage.ReservedGeneratedTokens != 12000 || ledger.Claim.Usage.ReservedCostMicros != 250000 {
		t.Fatalf("conservative failed-call accounting lost: ledger=%+v err=%v", ledger, err)
	}
	var preserved v38ProviderFailureReceipt
	if err := readJSONV38(filepath.Join(root, "observations", "R04", "phase-01-failure.json"), &preserved); err != nil || string(preserved.RawStdout) != string(rawStdout) || preserved.AccountingStatus != "conservative" {
		t.Fatalf("conservative evidence lost: receipt=%+v err=%v", preserved, err)
	}
}

func TestV38ClosedReplayRejectsPaidPhaseDrift(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	provider := v38ProviderFunc(func(_ context.Context, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		candidate := successfulV38Plan(request.RunFamilyID)
		return candidate, fixtureReceiptV38(t, transport, request, candidate, 20, 4, 1), nil
	})
	if _, err := executeV38(context.Background(), root, provider, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	phasePath := filepath.Join(root, "observations", "R04", "phase-01-paid.json")
	var phase v38PaidPhaseArtifact
	if err := readJSONV38(phasePath, &phase); err != nil {
		t.Fatal(err)
	}
	phase.Plan.Actions[0].Check = strings.ToUpper(phase.Plan.Actions[0].Check)
	if err := writeAtomicV38(phasePath, phase); err != nil {
		t.Fatal(err)
	}
	ledger, err := executeV38(context.Background(), root, provider, func() error { return nil })
	if err == nil || !ledger.GlobalHalt || ledger.GlobalHaltReason != string(v38Evidence) {
		t.Fatalf("paid phase drift accepted: ledger=%+v err=%v", ledger, err)
	}
}

func TestV38LifecycleResumesPaidActionsWithoutRepeatingProviderCall(t *testing.T) {
	d, _ := agentqualification.ClaudeSubscriptionRealisticV38()
	root := filepath.Clean(t.TempDir())
	cell := filepath.Join(root, "observations", "R04")
	if err := os.MkdirAll(filepath.Join(cell, "workspace"), 0o700); err != nil {
		t.Fatal(err)
	}
	runFamily := hashV38(d.BatchID + "|R04:v1")
	workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(filepath.Join(cell, "workspace"), "R04", runFamily)
	if err != nil {
		t.Fatal(err)
	}
	state, err := agentqualification.InitializeClaudeSubscriptionRealisticV38(workspace, runFamily, func(agentqualification.RealisticControllerResult) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	transport, request, _ := agentqualification.BuildClaudeSubscriptionRealisticV38PhaseRequest(workspace, runFamily, state)
	phaseOne := agentqualification.ClaudeRealisticV37Plan{Actions: []agentqualification.RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}, {Kind: "search", Query: "checkpoint"}}}
	receipt := fixtureReceiptV38(t, transport, request, phaseOne, 10, 2, 1)
	phasePath := filepath.Join(cell, "phase-01-paid.json")
	if err := writeExclusiveV38(phasePath, v38PaidPhaseArtifact{Request: request, Plan: phaseOne, Receipt: receipt}); err != nil {
		t.Fatal(err)
	}
	phaseHash, _ := fileHashV38(phasePath)
	errCrash := errors.New("fixture crash")
	checkpointed, err := agentqualification.ExecuteClaudeSubscriptionRealisticV38Phase(workspace, runFamily, phaseOne, receipt, state, func(next agentqualification.ClaudeRealisticV37ExecutionState) error {
		if next.NextAction == 1 {
			return errCrash
		}
		return nil
	})
	if !errors.Is(err, errCrash) || checkpointed.PendingPlan == nil || checkpointed.NextAction != 1 || checkpointed.Controller.ProviderCalls != 1 {
		t.Fatalf("checkpoint=%+v err=%v", checkpointed, err)
	}
	runtimeState := v38Runtime{Workspace: workspace, State: checkpointed}
	if err := writeExclusiveV38(filepath.Join(cell, "runtime.json"), runtimeState); err != nil {
		t.Fatal(err)
	}
	runtimeHash, _ := fileHashV38(filepath.Join(cell, "runtime.json"))
	admissionHash := writeV38FixtureAdmission(t, cell)
	ledger := v38Ledger{SchemaVersion: v38LedgerSchema, BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV38SHA256(), Status: "running", Claim: &v38Claim{CaseID: "R04", CaseVersion: "v1", RunFamilyID: runFamily, Status: "pending", Phase: 1, InFlight: true, RuntimeSHA256: runtimeHash, AdmissionSHA256: admissionHash, RequestSHA256: hashV38Bytes(mustJSONV38(request)), PaidPhases: []v38PaidPhaseBinding{{Phase: 1, Path: "phase-01-paid.json", SHA256: phaseHash, RequestSHA256: hashV38Bytes(mustJSONV38(request))}}, Usage: usageV38(checkpointed.Controller)}}
	if err := writeExclusiveV38(filepath.Join(root, "ledger.json"), ledger); err != nil {
		t.Fatal(err)
	}
	calls := 0
	ledger, err = executeV38(context.Background(), root, v38ProviderFunc(func(_ context.Context, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		candidate := successfulV38Plan(request.RunFamilyID)
		return candidate, fixtureReceiptV38(t, transport, request, candidate, 10, 2, 1), nil
	}), func() error { return nil })
	if err != nil || calls != 1 || ledger.Claim == nil || ledger.Claim.Usage.ProviderInvocations != 2 {
		t.Fatalf("duplicate phase-one call or lost accounting: calls=%d ledger=%+v err=%v", calls, ledger, err)
	}
}

func TestV38LifecycleConservativelyClosesAmbiguousInFlight(t *testing.T) {
	d, _ := agentqualification.ClaudeSubscriptionRealisticV38()
	root := filepath.Clean(t.TempDir())
	cell := filepath.Join(root, "observations", "R04")
	if err := os.MkdirAll(filepath.Join(cell, "workspace"), 0o700); err != nil {
		t.Fatal(err)
	}
	runFamily := hashV38(d.BatchID + "|R04:v1")
	workspace, _ := agentqualification.PrepareRealisticCaseWorkspaceForRun(filepath.Join(cell, "workspace"), "R04", runFamily)
	state, _ := agentqualification.InitializeClaudeSubscriptionRealisticV38(workspace, runFamily, func(agentqualification.RealisticControllerResult) error { return nil })
	runtimeState := v38Runtime{Workspace: workspace, State: state}
	if err := writeExclusiveV38(filepath.Join(cell, "runtime.json"), runtimeState); err != nil {
		t.Fatal(err)
	}
	runtimeHash, _ := fileHashV38(filepath.Join(cell, "runtime.json"))
	admissionHash := writeV38FixtureAdmission(t, cell)
	reserved, err := admitV38ProviderCall(d, state.Controller)
	if err != nil {
		t.Fatal(err)
	}
	ledger := v38Ledger{SchemaVersion: v38LedgerSchema, BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV38SHA256(), Status: "running", Claim: &v38Claim{CaseID: "R04", CaseVersion: "v1", RunFamilyID: runFamily, Status: "pending", Phase: 1, InFlight: true, RuntimeSHA256: runtimeHash, AdmissionSHA256: admissionHash, Usage: reserved}}
	if err := writeExclusiveV38(filepath.Join(root, "ledger.json"), ledger); err != nil {
		t.Fatal(err)
	}
	calls := 0
	ledger, err = executeV38(context.Background(), root, v38ProviderFunc(func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		return agentqualification.ClaudeRealisticV37Plan{}, nil, nil
	}), func() error { return nil })
	if err != nil || calls != 0 || ledger.Claim.Outcome != "invalid" || ledger.Claim.FailureClass != string(v38Transport) || ledger.Claim.Usage.AmbiguousInvocations != 1 || ledger.Claim.Usage.ReservedReportedTokens != d.Caps.MaxTotalReportedTokens {
		t.Fatalf("calls=%d ledger=%+v err=%v", calls, ledger, err)
	}
}

func TestV38StopPolicyAndCredentialGlobalHalt(t *testing.T) {
	for _, local := range []v38StopClass{v38Competence, v38Protocol, v38Resource, v38Transport} {
		if local.global() {
			t.Fatalf("local stop became global: %s", local)
		}
	}
	for _, global := range []v38StopClass{v38Credential, v38Unsafe, v38Evidence, v38Campaign} {
		if !global.global() {
			t.Fatalf("global stop became local: %s", global)
		}
	}
	root := filepath.Clean(t.TempDir())
	ledger, err := executeV38(context.Background(), root, v38ProviderFunc(func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		return agentqualification.ClaudeRealisticV37Plan{}, nil, agentqualification.ErrRealisticCredentialFailure
	}), func() error { return nil })
	if err == nil || !ledger.GlobalHalt || ledger.GlobalHaltReason != string(v38Credential) || ledger.Claim == nil || ledger.Claim.Outcome != "invalid" {
		t.Fatalf("ledger=%+v err=%v", ledger, err)
	}
}

func fixtureReceiptV38(t *testing.T, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest, candidate agentqualification.ClaudeRealisticV37Plan, input, output, cost int64) []byte {
	t.Helper()
	candidateRaw, _ := json.Marshal(candidate)
	rawEnvelope := map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"result": string(candidateRaw), "structured_output": candidate, "session_id": request.ProviderCallID,
		"total_cost_usd": float64(cost) / 1_000_000,
		"usage":          map[string]any{"input_tokens": input, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": output},
		"modelUsage":     map[string]any{"claude-sonnet-4-6": map[string]any{}},
	}
	raw, _ := json.Marshal(rawEnvelope)
	parsed, receipt, err := agentqualification.ParseClaudeSubscriptionRealisticV38Response(request, transport, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	parsedRaw, _ := json.Marshal(parsed)
	if string(parsedRaw) != string(candidateRaw) {
		t.Fatalf("candidate changed")
	}
	return receipt
}

func writeV38FixtureAdmission(t *testing.T, cell string) string {
	t.Helper()
	path := filepath.Join(cell, "admission.json")
	if err := writeExclusiveV38(path, map[string]any{"fixture": true}); err != nil {
		t.Fatal(err)
	}
	hash, err := fileHashV38(path)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}
