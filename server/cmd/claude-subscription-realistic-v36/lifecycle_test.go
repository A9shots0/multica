package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type v36ProviderFunc func(context.Context, agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error)

func (f v36ProviderFunc) Next(ctx context.Context, request agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
	return f(ctx, request)
}

func fixtureResponseV36(t *testing.T, request agentqualification.RealisticProviderRequest, action agentqualification.RealisticCandidateAction, input, output, cost int64) agentqualification.RealisticProviderResponse {
	t.Helper()
	plan, err := agentqualification.BuildClaudeSubscriptionRealisticV36ActionPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	rawMap := map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": "{}", "structured_output": map[string]any{"action": action}, "session_id": request.ProviderCallID, "total_cost_usd": float64(cost) / 1_000_000, "usage": map[string]any{"input_tokens": input, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": output}, "modelUsage": map[string]any{"claude-sonnet-4-6": map[string]any{}}}
	raw, _ := json.Marshal(rawMap)
	response, err := agentqualification.ParseClaudeSubscriptionRealisticV36Response(request, plan, raw, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func successfulProviderV36(t *testing.T, calls *int) agentqualification.RealisticProvider {
	t.Helper()
	turns := map[string]int{}
	return v36ProviderFunc(func(_ context.Context, r agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
		*calls++
		turns[r.CaseID]++
		var actions []agentqualification.RealisticCandidateAction
		if r.CaseID == "R03" {
			actions = []agentqualification.RealisticCandidateAction{
				{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
				{Kind: "edit", Path: "app/access.py", Contents: "def employee_pages():\n    return [\"posting\", \"schedule\"]\n\ndef manager_pages():\n    return [\"posting\", \"schedule\", \"review\"]\n"},
				{Kind: "edit", Path: "release/state.json", Contents: `{"requested_outcome":"employee_pages_and_new_gallery","baseline":{"revision":"v190","healthy":true,"employee_pages":true,"gallery_count":6},"candidate":{"revision":"v195","deployed":true,"employee_pages":false,"gallery_count":10},"rollback":{"revision":"v190","verified":true},"outcome_status":"open"}`},
				{Kind: "edit", Path: "release/receipt.json", Contents: `{"candidate":"v195","rollback":"v190","baseline_restored":true,"requested_outcome":"open","evidence_stage":"local-simulation"}`},
				{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
				{Kind: "run_check", Check: "local-release-simulator verify"},
				{Kind: "edit", Path: "EVIDENCE.md", Contents: "Local simulation restored v190, made access safe by default, and verified current plus historical journeys. Requested gallery successor outcome remains open.\n"},
				{Kind: "finish", Summary: "Safe baseline restored and release contract verified; requested gallery successor outcome remains open."},
			}
		} else {
			actions = []agentqualification.RealisticCandidateAction{
				{Kind: "run_check", Check: "python resume.py"},
				{Kind: "edit", Path: "HANDOFF.md", Contents: "Run family " + r.RunFamilyID + " resumed from durable checkpoint. Legacy completion claim is superseded. Remaining verification and final local output completed exactly once.\n"},
				{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
				{Kind: "finish", Summary: "Durable same-family continuation completed exactly once; superseded status rejected."},
			}
		}
		return fixtureResponseV36(t, r, actions[turns[r.CaseID]-1], 10, 2, 1), nil
	})
}

func TestV36PlanIsProviderFreeAndExecutionLocked(t *testing.T) {
	plan, err := planV36()
	if err != nil {
		t.Fatal(err)
	}
	if plan["provider_called"] != false || plan["provider_calls_allowed"] != true || plan["status"] != "frozen_reviewed" {
		t.Fatalf("plan=%+v", plan)
	}
	calls := 0
	_, err = executeV36(context.Background(), t.TempDir(), v36ProviderFunc(func(context.Context, agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
		calls++
		return agentqualification.RealisticProviderResponse{}, nil
	}), func() error { return errors.New("review pending") })
	if err == nil || calls != 0 {
		t.Fatalf("locked execution called provider: %d %v", calls, err)
	}
}

func TestV36R03CandidateFailureSkipsR04(t *testing.T) {
	calls := 0
	provider := v36ProviderFunc(func(_ context.Context, r agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
		calls++
		return fixtureResponseV36(t, r, agentqualification.RealisticCandidateAction{Kind: "finish", Summary: "unsupported completion"}, 10, 2, 1), nil
	})
	ledger, err := executeV36(context.Background(), t.TempDir(), provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !ledger.CandidateEliminated || ledger.Claims["R03"].Outcome != "fail" || ledger.Claims["R04"].Status != "skipped" {
		t.Fatalf("calls=%d ledger=%+v", calls, ledger)
	}
}

func TestV36ObservationLocalInvalidityStillRunsR04(t *testing.T) {
	calls := 0
	provider := v36ProviderFunc(func(_ context.Context, r agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
		calls++
		if r.CaseID == "R03" {
			return agentqualification.RealisticProviderResponse{}, failV36(v36ObservationInvalid, errors.New("fixture transport"))
		}
		return fixtureResponseV36(t, r, agentqualification.RealisticCandidateAction{Kind: "finish", Summary: "unsupported"}, 10, 2, 1), nil
	})
	ledger, err := executeV36(context.Background(), t.TempDir(), provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || ledger.GlobalHalt || ledger.Claims["R03"].Outcome != "invalid" || ledger.Claims["R04"].Status != "completed" {
		t.Fatalf("calls=%d ledger=%+v", calls, ledger)
	}
}

func TestV36ResourceFailureEliminatesOnlyCandidate(t *testing.T) {
	calls := 0
	provider := v36ProviderFunc(func(_ context.Context, r agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
		calls++
		return fixtureResponseV36(t, r, agentqualification.RealisticCandidateAction{Kind: "read", Path: "TASK.md"}, 10, 2, 200001), nil
	})
	ledger, err := executeV36(context.Background(), t.TempDir(), provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || ledger.GlobalHalt || ledger.Claims["R03"].FailureClass != "observation_resource_exhaustion" || ledger.Claims["R04"].Status != "skipped" {
		t.Fatalf("calls=%d ledger=%+v", calls, ledger)
	}
}

func TestV36CompleteBatchProvesR03ThenExactOnceR04(t *testing.T) {
	calls := 0
	provider := successfulProviderV36(t, &calls)
	root := t.TempDir()
	ledger, err := executeV36(context.Background(), root, provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 12 || ledger.Status != "complete" || ledger.CandidateEliminated || ledger.Claims["R03"].Outcome != "pass" || ledger.Claims["R04"].Outcome != "pass" || ledger.Claims["R04"].Attempts != 2 || ledger.Claims["R04"].Retries != 0 {
		t.Fatalf("calls=%d ledger=%+v", calls, ledger)
	}
	if replay, err := replayV36(root); err != nil || replay["provider_called"] != false {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

func TestV36ReplayDetectsWriteOnceDriftWithoutProviderCall(t *testing.T) {
	root := t.TempDir()
	provider := v36ProviderFunc(func(_ context.Context, r agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
		return fixtureResponseV36(t, r, agentqualification.RealisticCandidateAction{Kind: "finish", Summary: "unsupported"}, 10, 2, 1), nil
	})
	ledger, err := executeV36(context.Background(), root, provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replayV36(root); err != nil {
		t.Fatal(err)
	}
	caseID := "R03"
	path := filepath.Join(root, "observations", caseID, "result.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := replayV36(root); err == nil {
		t.Fatalf("tampered result accepted; ledger=%+v", ledger)
	}
}

func TestV36CredentialClassificationIsStructuredNotSubstringBased(t *testing.T) {
	real := []byte(`{"type":"result","subtype":"error_during_execution","is_error":true,"duration_api_ms":0,"num_turns":0,"result":"Not logged in · Please run /login","modelUsage":{}}`)
	if !claudeV36StructuredCredentialFailure(real) {
		t.Fatal("real signed-out shape missed")
	}
	for _, spoof := range [][]byte{[]byte(`authentication failed`), []byte(`{"type":"result","subtype":"success","is_error":false,"duration_api_ms":0,"num_turns":0,"result":"Not logged in · Please run /login","modelUsage":{}}`), []byte(`{"type":"result","subtype":"error_during_execution","is_error":true,"duration_api_ms":1,"num_turns":1,"result":"Not logged in · Please run /login","modelUsage":{"claude-sonnet-4-6":{}}}`)} {
		if claudeV36StructuredCredentialFailure(spoof) {
			t.Fatalf("spoof classified: %s", spoof)
		}
	}
}

func TestV36PendingCrashClosesWithoutDuplicateCallOrR04Admission(t *testing.T) {
	d, err := agentqualification.ClaudeSubscriptionRealisticV36()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	ledgerPath := filepath.Join(root, "ledger.json")
	ledger, err := loadLedgerV36(ledgerPath, d)
	if err != nil {
		t.Fatal(err)
	}
	runFamily := hashV36(d.BatchID + "|00|R03:v2")
	cell := filepath.Join(root, "observations", "R03")
	if err := os.MkdirAll(filepath.Join(cell, "workspace"), 0o700); err != nil {
		t.Fatal(err)
	}
	workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(filepath.Join(cell, "workspace"), "R03", runFamily)
	if err != nil {
		t.Fatal(err)
	}
	runtimeState := v36Runtime{Workspace: workspace}
	if err := writeJSONExclusiveV36(filepath.Join(cell, "runtime.json"), runtimeState); err != nil {
		t.Fatal(err)
	}
	claim := v36Claim{CaseID: "R03", CaseVersion: "v2", RunFamilyID: runFamily, Status: "pending"}
	claim.RuntimeSHA256, _ = fileHashV36(filepath.Join(cell, "runtime.json"))
	ledger.Claims["R03"] = claim
	if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
		t.Fatal(err)
	}
	runtimeState.Controller = agentqualification.RealisticControllerResult{CaseID: "R03", RunFamilyID: runFamily, Attempt: 1, ProviderCalls: 1, ProviderAccountingStatus: "in_flight", ProviderTokenAccountingStatus: "in_flight", ProviderCostAccountingStatus: "in_flight", PendingProviderCall: &agentqualification.RealisticProviderCallCheckpoint{ProviderCallID: runFamily + ":attempt-1:turn-1", ProviderTurn: 1, RequestSHA256: hashV36("request")}}
	if err := checkpointV36(cell, &runtimeState); err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err = executeV36(context.Background(), root, v36ProviderFunc(func(context.Context, agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
		calls++
		return agentqualification.RealisticProviderResponse{}, nil
	}), func() error { return nil })
	if err == nil || calls != 0 {
		t.Fatalf("pending recovery duplicated provider call: calls=%d err=%v", calls, err)
	}
	if err := readJSONV36(ledgerPath, &ledger); err != nil {
		t.Fatal(err)
	}
	r03 := ledger.Claims["R03"]
	if !ledger.GlobalHalt || r03.Status != "completed" || r03.Outcome != "invalid" || r03.Usage.ProviderTurns != 15 || r03.Usage.UnknownTurns != 15 || r03.Usage.ConservativeReservedTokens != 75000 {
		t.Fatalf("pending claim not conservatively closed: %+v", ledger)
	}
	if _, exists := ledger.Claims["R04"]; exists {
		t.Fatal("R04 was admitted after unresolved R03 pending claim")
	}
}

func TestV36ReplayRejectsForgedLedgerResultControllerReceiptAndGrade(t *testing.T) {
	calls := 0
	root := t.TempDir()
	if _, err := executeV36(context.Background(), root, successfulProviderV36(t, &calls), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 12 {
		t.Fatalf("calls=%d", calls)
	}
	files := map[string]string{
		"ledger_accounting": filepath.Join(root, "ledger.json"),
		"result_claim":      filepath.Join(root, "observations", "R03", "result.json"),
		"controller":        filepath.Join(root, "observations", "R03", "controller-result.json"),
		"receipt":           filepath.Join(root, "observations", "R03", "evidence", "execution-receipt.json"),
		"grade":             filepath.Join(root, "observations", "R03", "grade.json"),
	}
	for name, path := range files {
		t.Run(name, func(t *testing.T) {
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			forged := append([]byte(nil), original...)
			forged[len(forged)/2] ^= 1
			if err := os.WriteFile(path, forged, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := replayV36(root); err == nil {
				t.Fatalf("forged %s accepted", name)
			}
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := replayV36(root); err != nil {
				t.Fatalf("restored evidence rejected: %v", err)
			}
		})
	}
}

func TestV36R04ForgedAttemptHistoryRejected(t *testing.T) {
	calls := 0
	root := t.TempDir()
	if _, err := executeV36(context.Background(), root, successfulProviderV36(t, &calls), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	cell := filepath.Join(root, "observations", "R04")
	var runtimeState v36Runtime
	var controller agentqualification.RealisticControllerResult
	if err := readJSONV36(filepath.Join(cell, "runtime.json"), &runtimeState); err != nil {
		t.Fatal(err)
	}
	if err := readJSONV36(filepath.Join(cell, "controller-result.json"), &controller); err != nil {
		t.Fatal(err)
	}
	controller.AttemptHistory[0].ProviderCalls = 1
	if _, _, err := agentqualification.SealClaudeSubscriptionRealisticV36Execution(filepath.Join(t.TempDir(), "evidence"), runtimeState.Workspace, controller); err == nil {
		t.Fatal("forged R04 attempt history accepted")
	}
}

func TestV36CheckedArithmeticRejectsOverflow(t *testing.T) {
	if _, ok := checkedAddInt64V36(math.MaxInt64, 1); ok {
		t.Fatal("int64 overflow accepted")
	}
	usage := v36Usage{InputTokens: math.MaxInt64}
	if err := addUsageV36(&usage, v36Usage{InputTokens: 1}); err == nil {
		t.Fatal("usage overflow accepted")
	}
	if _, ok := admitV36(v36Usage{InputTokens: math.MaxInt64}, agentqualification.ClaudeRealisticV36Caps{MaxProviderTurns: 1, MaxTotalReportedTokens: 1, MaxGeneratedTokens: 1, MaxCostEquivalentMicros: 1}, agentqualification.ClaudeRealisticV36Caps{MaxProviderTurns: 2, MaxTotalReportedTokens: math.MaxInt64, MaxGeneratedTokens: math.MaxInt64, MaxCostEquivalentMicros: math.MaxInt64}); ok {
		t.Fatal("cap admission overflow was not classified")
	}
}
