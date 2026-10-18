package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type v37ProviderFunc func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error)

func (function v37ProviderFunc) Plan(ctx context.Context, plan agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
	return function(ctx, plan, request)
}

func TestV37LifecycleCompletesR03WithTwoPaidPhases(t *testing.T) {
	calls := 0
	provider := v37ProviderFunc(func(_ context.Context, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		var candidate agentqualification.ClaudeRealisticV37Plan
		if request.ProviderTurn == 1 {
			candidate.Actions = []agentqualification.RealisticCandidateAction{
				{Kind: "edit", Path: "app/access.py", Contents: "def employee_pages():\n    return ['posting', 'schedule']\n\ndef manager_pages():\n    return employee_pages() + ['review']\n"},
				{Kind: "edit", Path: "release/state.json", Contents: `{"requested_outcome":"employee_pages_and_new_gallery","baseline":{"revision":"v190","healthy":true,"employee_pages":true,"gallery_count":6},"candidate":{"revision":"v195","deployed":true,"employee_pages":false,"gallery_count":10},"rollback":{"revision":"v190","verified":true},"outcome_status":"open"}`},
				{Kind: "edit", Path: "release/receipt.json", Contents: `{"candidate":"v195","rollback":"v190","baseline_restored":true,"requested_outcome":"open","evidence_stage":"local-simulation"}`},
				{Kind: "edit", Path: "EVIDENCE.md", Contents: "The missing optional build flag revived the access regression. This local-only rollback restores and verifies v190; the requested gallery successor remains open until independently verified.\n"},
			}
		} else {
			candidate.Actions = []agentqualification.RealisticCandidateAction{
				{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
				{Kind: "run_check", Check: "local-release-simulator rollback"},
				{Kind: "run_check", Check: "local-release-simulator verify"},
				{Kind: "finish", Summary: "Safe simulated baseline restored and verified; requested gallery successor outcome remains open."},
			}
		}
		return candidate, fixtureReceiptV37(t, transport, request, candidate, 20, 4, 1), nil
	})
	root := filepath.Clean(t.TempDir())
	ledger, err := executeV37(context.Background(), root, provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || ledger.Status != "complete" || ledger.GlobalHalt || ledger.CandidateEliminated || ledger.Claim == nil || ledger.Claim.Outcome != "pass" || ledger.Claim.Usage.ProviderInvocations != 2 || ledger.Claim.Retries != 0 {
		t.Fatalf("calls=%d ledger=%+v", calls, ledger)
	}
	for _, path := range []string{"result.json", "grade.json", filepath.Join("evidence", "execution-receipt.json")} {
		if _, err := os.Stat(filepath.Join(root, "observations", "R03", path)); err != nil {
			t.Fatalf("missing sealed %s: %v", path, err)
		}
	}
}

func TestV37LifecycleConservativelyClosesAmbiguousInFlightWithoutProviderCall(t *testing.T) {
	d, _ := agentqualification.ClaudeSubscriptionRealisticV37()
	root := filepath.Clean(t.TempDir())
	cell := filepath.Join(root, "observations", "R03")
	if err := os.MkdirAll(filepath.Join(cell, "workspace"), 0o700); err != nil {
		t.Fatal(err)
	}
	runFamily := hashV37(d.BatchID + "|R03:v2")
	workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(filepath.Join(cell, "workspace"), "R03", runFamily)
	if err != nil {
		t.Fatal(err)
	}
	runtimeState := v37Runtime{Workspace: workspace}
	if err := writeExclusiveV37(filepath.Join(cell, "runtime.json"), runtimeState); err != nil {
		t.Fatal(err)
	}
	runtimeHash, _ := fileHashV37(filepath.Join(cell, "runtime.json"))
	ledger := v37Ledger{SchemaVersion: v37LedgerSchema, BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV37SHA256(), Status: "running", Claim: &v37Claim{CaseID: "R03", CaseVersion: "v2", RunFamilyID: runFamily, Status: "pending", Phase: 1, InFlight: true, RequestSHA256: hashV37("request"), RuntimeSHA256: runtimeHash}}
	if err := writeExclusiveV37(filepath.Join(root, "ledger.json"), ledger); err != nil {
		t.Fatal(err)
	}
	calls := 0
	ledger, err = executeV37(context.Background(), root, v37ProviderFunc(func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		return agentqualification.ClaudeRealisticV37Plan{}, nil, nil
	}), func() error { return nil })
	if err != nil || calls != 0 || ledger.Claim.Outcome != "invalid" || ledger.Claim.FailureClass != string(v37Transport) || ledger.Claim.Usage.AmbiguousInvocations != d.Caps.MaxProviderInvocations || ledger.Claim.Usage.ReservedReportedTokens != d.Caps.MaxTotalReportedTokens {
		t.Fatalf("calls=%d ledger=%+v err=%v", calls, ledger, err)
	}
	if _, err := os.Stat(filepath.Join(cell, "result.json")); err != nil {
		t.Fatalf("invalid observation was not sealed: %v", err)
	}
}

func TestV37LifecycleResumesCheckpointedPaidActionsWithoutRepeatingProviderCall(t *testing.T) {
	d, _ := agentqualification.ClaudeSubscriptionRealisticV37()
	root := filepath.Clean(t.TempDir())
	cell := filepath.Join(root, "observations", "R03")
	if err := os.MkdirAll(filepath.Join(cell, "workspace"), 0o700); err != nil {
		t.Fatal(err)
	}
	runFamily := hashV37(d.BatchID + "|R03:v2")
	workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(filepath.Join(cell, "workspace"), "R03", runFamily)
	if err != nil {
		t.Fatal(err)
	}
	transport, request, _ := agentqualification.BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, runFamily, agentqualification.ClaudeRealisticV37ExecutionState{})
	phaseOne := agentqualification.ClaudeRealisticV37Plan{Actions: []agentqualification.RealisticCandidateAction{{Kind: "read", Path: "TASK.md"}, {Kind: "search", Query: "rollback"}}}
	receipt := fixtureReceiptV37(t, transport, request, phaseOne, 10, 2, 1)
	errCrash := errors.New("fixture crash")
	checkpointed, err := agentqualification.ExecuteClaudeSubscriptionRealisticV37Phase(workspace, runFamily, 1, phaseOne, receipt, agentqualification.ClaudeRealisticV37ExecutionState{}, func(state agentqualification.ClaudeRealisticV37ExecutionState) error {
		if state.NextAction == 1 {
			return errCrash
		}
		return nil
	})
	if !errors.Is(err, errCrash) || checkpointed.PendingPlan == nil || checkpointed.NextAction != 1 {
		t.Fatalf("checkpoint=%+v err=%v", checkpointed, err)
	}
	runtimeState := v37Runtime{Workspace: workspace, State: checkpointed}
	if err := writeExclusiveV37(filepath.Join(cell, "runtime.json"), runtimeState); err != nil {
		t.Fatal(err)
	}
	runtimeHash, _ := fileHashV37(filepath.Join(cell, "runtime.json"))
	ledger := v37Ledger{SchemaVersion: v37LedgerSchema, BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV37SHA256(), Status: "running", Claim: &v37Claim{CaseID: "R03", CaseVersion: "v2", RunFamilyID: runFamily, Status: "pending", Phase: 1, InFlight: true, RuntimeSHA256: runtimeHash}}
	if err := writeExclusiveV37(filepath.Join(root, "ledger.json"), ledger); err != nil {
		t.Fatal(err)
	}
	calls := 0
	ledger, err = executeV37(context.Background(), root, v37ProviderFunc(func(_ context.Context, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		candidate := agentqualification.ClaudeRealisticV37Plan{Actions: []agentqualification.RealisticCandidateAction{{Kind: "finish", Summary: "Requested outcome remains open."}}}
		return candidate, fixtureReceiptV37(t, transport, request, candidate, 10, 2, 1), nil
	}), func() error { return nil })
	if err != nil || calls != 1 || ledger.Claim == nil || ledger.Claim.Usage.ProviderInvocations != 2 {
		t.Fatalf("duplicate phase-one call or lost accounting: calls=%d ledger=%+v err=%v", calls, ledger, err)
	}
}

func TestV37StopPolicySeparatesCandidateAndGlobalFailures(t *testing.T) {
	for _, local := range []v37StopClass{v37Competence, v37Protocol, v37Resource, v37Transport} {
		if local.global() {
			t.Fatalf("candidate/observation-local stop became global: %s", local)
		}
	}
	for _, global := range []v37StopClass{v37Credential, v37Unsafe, v37Evidence, v37Campaign} {
		if !global.global() {
			t.Fatalf("global stop became local: %s", global)
		}
	}
	if got := classifyV37(agentqualification.ErrRealisticCredentialFailure); got != v37Credential {
		t.Fatalf("credential classified %s", got)
	}
	if got := classifyV37(failV37(v37Protocol, errors.New("fixture"))); got != v37Protocol {
		t.Fatalf("protocol classified %s", got)
	}
}

func TestV37CredentialFailureGloballyHaltsAndSealsInvalidObservation(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	ledger, err := executeV37(context.Background(), root, v37ProviderFunc(func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		return agentqualification.ClaudeRealisticV37Plan{}, nil, agentqualification.ErrRealisticCredentialFailure
	}), func() error { return nil })
	if err == nil || !ledger.GlobalHalt || ledger.GlobalHaltReason != string(v37Credential) || ledger.Claim == nil || ledger.Claim.Outcome != "invalid" {
		t.Fatalf("ledger=%+v err=%v", ledger, err)
	}
	if _, err := os.Stat(filepath.Join(root, "observations", "R03", "result.json")); err != nil {
		t.Fatalf("credential-invalid observation was not sealed: %v", err)
	}
}

func fixtureReceiptV37(t *testing.T, transport agentqualification.ClaudeScreeningPlan, request agentqualification.RealisticProviderRequest, candidate agentqualification.ClaudeRealisticV37Plan, input, output, cost int64) []byte {
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
	parsed, receipt, err := agentqualification.ParseClaudeSubscriptionRealisticV37Response(request, transport, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	parsedRaw, _ := json.Marshal(parsed)
	if string(parsedRaw) != string(candidateRaw) {
		t.Fatalf("candidate changed: %s != %s", parsedRaw, candidateRaw)
	}
	return receipt
}
