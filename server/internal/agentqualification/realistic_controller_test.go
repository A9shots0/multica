package agentqualification

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func acceptRealisticCheckpoint(RealisticControllerResult) error { return nil }

type scriptedRealisticProvider struct {
	actions []RealisticCandidateAction
	index   int
}

type realisticProviderFunc func(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error)

func (provider realisticProviderFunc) Next(ctx context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	return provider(ctx, request)
}

type scriptedBrowserCollector struct {
	pending bool
	proofs  []RealisticBrowserProof
	paths   []string
}

func (collector *scriptedBrowserCollector) Capture(_ context.Context, _ string, path string) ([]RealisticBrowserProof, error) {
	collector.paths = append(collector.paths, path)
	if collector.pending {
		collector.pending = false
		return nil, ErrRealisticBrowserEvidencePending
	}
	return collector.proofs, nil
}

func (provider *scriptedRealisticProvider) Next(_ context.Context, _ RealisticProviderRequest) (RealisticProviderResponse, error) {
	action := provider.actions[provider.index]
	provider.index++
	return RealisticProviderResponse{Action: action, RawReceipt: []byte(`{"fixture":true}`), InputTokens: 10, OutputTokens: 5, CostMicros: 1}, nil
}

func TestRealisticControllerUsesOnlyNarrowTools(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{
		{Kind: "read", Path: "crm/authority.json"},
		{Kind: "edit", Path: "consumer_faulty/normalize.py", Contents: "def normalize_role(role: str) -> str:\n    return role\n"},
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "finish", Summary: "local checks passed"},
	}}
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "fixture-run", 1, provider, nil, acceptRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if result.TerminalState != "completed" || result.ProviderCalls != 4 || len(result.ToolEvents) != 3 || result.CostMicros != 4 {
		t.Fatalf("unexpected controller result: %+v", result)
	}
}

func TestRealisticControllerEliminatesProtocolFailureWithoutRetry(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "shell", Check: "curl https://example.com"}}}
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "fixture-run", 1, provider, nil, acceptRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if result.TerminalState != "failed" || result.FailureClass != "response_or_tool_protocol_failure" || result.ProviderCalls != 1 {
		t.Fatalf("protocol failure did not eliminate one candidate: %+v", result)
	}
}

func TestRealisticControllerTreatsCredentialFailureAsExactZeroCallGlobalHalt(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := realisticProviderFunc(func(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
		return RealisticProviderResponse{}, ErrRealisticCredentialFailure
	})
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "credential-failure", 1, provider, nil, acceptRealisticCheckpoint)
	if !errors.Is(err, ErrRealisticCredentialFailure) || result.TerminalState != "invalid" || result.FailureClass != "credential_failure" || result.ProviderCalls != 0 || result.InputTokens != 0 || result.OutputTokens != 0 || result.CostMicros != 0 {
		t.Fatalf("credential failure was not an exact zero-call global halt: result=%+v err=%v", result, err)
	}
}

func TestRealisticControllerTreatsObservationTokenExhaustionAsCandidateLocal(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := realisticProviderFunc(func(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
		return RealisticProviderResponse{Action: RealisticCandidateAction{Kind: "read", Path: "TASK.md"}, RawReceipt: []byte(`{"fixture":true}`), InputTokens: 75001, OutputTokens: 1, CostMicros: 1}, nil
	})
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "token-ceiling", 1, provider, nil, acceptRealisticCheckpoint)
	if err != nil || result.TerminalState != "failed" || result.FailureClass != "response_or_tool_protocol_failure" || result.ProviderCalls != 1 {
		t.Fatalf("observation token exhaustion was not candidate-local: result=%+v err=%v", result, err)
	}
}

func TestRealisticControllerEnforcesGeneratedTokenCeilingAcrossAllCalls(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	call := 0
	provider := realisticProviderFunc(func(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
		call++
		action := RealisticCandidateAction{Kind: "read", Path: "TASK.md"}
		if call > 1 {
			action.Path = "crm/authority.json"
		}
		return RealisticProviderResponse{Action: action, RawReceipt: []byte(`{"fixture":true}`), InputTokens: 1, OutputTokens: 11000, CostMicros: 1}, nil
	})
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "generated-token-ceiling", 1, provider, nil, acceptRealisticCheckpoint)
	if err == nil || result.TerminalState != "invalid" || result.FailureClass != "budget_breach" || result.ProviderCalls != 2 || result.OutputTokens != 22000 {
		t.Fatalf("cumulative generated-token ceiling was not enforced: result=%+v err=%v", result, err)
	}
}

func TestRealisticControllerSupportsOneBoundedInterruption(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspaceForRun(parent, "R04", "same-family")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "checkpoint_and_interrupt"}}}
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "same-family", 1, provider, nil, acceptRealisticCheckpoint)
	if err != nil || result.TerminalState != "interrupted" || result.ProviderCalls != 0 || len(result.AttemptHistory) != 1 {
		t.Fatalf("bounded interruption failed: result=%+v err=%v", result, err)
	}
}

func TestRealisticControllerResumesSameRunFamilyAfterBoundedInterruption(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspaceForRun(parent, "R04", "same-family")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "run_check", Check: "python resume.py"}, {Kind: "finish", Summary: "resumed once"}}}
	first, err := runRealisticCandidateFrozen(context.Background(), workspace, "same-family", 1, provider, nil, acceptRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	second, err := resumeRealisticCandidateFrozen(context.Background(), workspace, first, provider, nil, acceptRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if second.TerminalState != "completed" || second.Attempt != 2 || second.RunFamilyID != "same-family" || second.ProviderCalls != 2 || second.CostMicros != 2 || len(second.ProviderReceiptHashes) != 2 || len(second.AttemptHistory) != 2 {
		t.Fatalf("interruption accounting or continuation drifted: %+v", second)
	}
	if len(second.ToolEvents) != 1 || second.ToolEvents[0].ExitCode != 0 || strings.Join(second.ToolEvents[0].Arguments, " ") != "python resume.py" || !exactSet(second.ToolEvents[0].TouchedPaths, "state/resume-action.json", "state/step_counts.json", "FINAL-OUTPUT.json") {
		t.Fatalf("R04 resume was not recorded as one successful harness event: %+v", second.ToolEvents)
	}
	if _, err := resumeRealisticCandidateFrozen(context.Background(), workspace, second, provider, nil, acceptRealisticCheckpoint); err == nil {
		t.Fatal("completed family resumed a third time")
	}
}

func TestRealisticControllerPausesForIndependentBrowserEvidenceWithoutExtraProviderCall(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R02")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "browser", Path: "report/index.html"}, {Kind: "finish", Summary: "browser evidence received"}}}
	browser := &scriptedBrowserCollector{pending: true, proofs: []RealisticBrowserProof{{Viewport: "narrow"}}}
	first, err := runRealisticCandidateFrozen(context.Background(), workspace, "browser-family", 1, provider, browser, acceptRealisticCheckpoint)
	if err != nil || first.TerminalState != "awaiting_browser_evidence" || first.ProviderCalls != 1 || first.PendingAction == nil {
		t.Fatalf("Browser handoff did not pause cleanly: result=%+v err=%v", first, err)
	}
	second, err := resumeRealisticCandidateFrozen(context.Background(), workspace, first, provider, browser, acceptRealisticCheckpoint)
	if err != nil || second.TerminalState != "completed" || second.ProviderCalls != 2 || len(second.ToolEvents) != 1 {
		t.Fatalf("Browser continuation failed: result=%+v err=%v", second, err)
	}
}

func TestR02BrowserActionUsesFixtureOwnedEntrypoint(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R02")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "browser", Query: "verify the local report"}, {Kind: "finish", Summary: "browser evidence received"}}}
	browser := &scriptedBrowserCollector{pending: true, proofs: []RealisticBrowserProof{{Viewport: "narrow"}}}
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "r02-entrypoint", 1, provider, browser, acceptRealisticCheckpoint)
	if err != nil || result.TerminalState != "awaiting_browser_evidence" || result.PendingAction == nil {
		t.Fatalf("query-only Browser readiness did not reach the fixture entrypoint: result=%+v err=%v", result, err)
	}
	result, err = resumeRealisticCandidateFrozen(context.Background(), workspace, result, provider, browser, acceptRealisticCheckpoint)
	if err != nil || result.TerminalState != "completed" || len(browser.paths) != 2 || browser.paths[0] != "report/index.html" || browser.paths[1] != "report/index.html" {
		t.Fatalf("query-only Browser readiness did not preserve the fixture entrypoint across resume: paths=%v result=%+v err=%v", browser.paths, result, err)
	}
}

func TestRealisticBrowserResultExposesUsabilityAndExactHashes(t *testing.T) {
	proofs := []RealisticBrowserProof{
		{Viewport: "narrow", RenderedTotal: 47, RenderedCardCount: 4, HorizontalOverflow: true, ScreenshotSHA256: strings.Repeat("a", 64), DOMSnapshotSHA256: strings.Repeat("b", 64)},
		{Viewport: "medium", RenderedTotal: 47, RenderedCardCount: 4, ScreenshotSHA256: strings.Repeat("c", 64), DOMSnapshotSHA256: strings.Repeat("d", 64)},
		{Viewport: "wide", RenderedTotal: 47, RenderedCardCount: 4, ScreenshotSHA256: strings.Repeat("e", 64), DOMSnapshotSHA256: strings.Repeat("f", 64)},
	}
	output := string(realisticBrowserToolOutput(proofs))
	if !strings.Contains(output, "failed_verification_correct_and_capture_again_before_finishing") || !strings.Contains(output, `"horizontal_overflow":true`) || !strings.Contains(output, strings.Repeat("a", 64)) {
		t.Fatalf("Browser result hid correction facts or exact evidence hashes: %s", output)
	}
	proofs[0].HorizontalOverflow = false
	if output := string(realisticBrowserToolOutput(proofs)); !strings.Contains(output, "passed_verification_save_exact_proof_hashes") {
		t.Fatalf("usable Browser result did not expose pass status: %s", output)
	}
}

func TestRealisticPublicEntrypointRunsOnlyAfterCompleteFreeze(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "finish"}}}
	result, err := RunRealisticCandidate(context.Background(), workspace, "reviewed", 1, provider, nil, acceptRealisticCheckpoint)
	if err != nil || result.ProviderCalls != 1 || result.TerminalState != "completed" {
		t.Fatalf("reviewed public provider entrypoint did not run through the frozen harness: result=%+v err=%v", result, err)
	}
}

func TestRealisticControllerResumeDoesNotRepeatDurablyRecordedPaidTurn(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "finish", Summary: "paid once"}}}
	var durable RealisticControllerResult
	crashAfterCommit := func(result RealisticControllerResult) error {
		durable = result
		if result.PendingPaidTurn != nil {
			return errors.New("simulated process crash after durable commit")
		}
		return nil
	}
	first, err := runRealisticCandidateFrozen(context.Background(), workspace, "crash-family", 1, provider, nil, crashAfterCommit)
	if !errors.Is(err, ErrRealisticCheckpointDurability) || first.ProviderCalls != 1 || durable.PendingPaidTurn == nil {
		t.Fatalf("paid turn was not recoverably checkpointed: result=%+v durable=%+v err=%v", first, durable, err)
	}
	providerCallsBeforeResume := provider.index
	resumed, err := resumeRealisticCandidateFrozen(context.Background(), workspace, durable, provider, nil, acceptRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if provider.index != providerCallsBeforeResume || resumed.ProviderCalls != 1 || resumed.TerminalState != "completed" || resumed.FinalSummary != "paid once" {
		t.Fatalf("resume repeated the paid turn or lost its result: provider_calls=%d result=%+v", provider.index, resumed)
	}
}

func TestRealisticControllerCheckpointsProgressBeforeNextPaidTurn(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "read", Path: "crm/authority.json"}, {Kind: "finish", Summary: "done"}}}
	checkpoints := []RealisticControllerResult{}
	checkpoint := func(result RealisticControllerResult) error {
		checkpoints = append(checkpoints, result)
		return nil
	}
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "ordered-family", 1, provider, nil, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if result.TerminalState != "completed" || len(checkpoints) != 6 {
		t.Fatalf("unexpected checkpoint sequence: result=%+v checkpoints=%d", result, len(checkpoints))
	}
	if checkpoints[0].ProviderCalls != 1 || checkpoints[0].PendingProviderCall == nil || checkpoints[0].PendingPaidTurn != nil {
		t.Fatalf("provider call was not checkpointed before dispatch: %+v", checkpoints[0])
	}
	if checkpoints[1].ProviderCalls != 1 || checkpoints[1].PendingProviderCall != nil || checkpoints[1].PendingPaidTurn == nil || len(checkpoints[1].ToolResults) != 0 {
		t.Fatalf("paid turn was not checkpointed before tool progress: %+v", checkpoints[1])
	}
	if checkpoints[2].ProviderCalls != 1 || checkpoints[2].PendingPaidTurn != nil || len(checkpoints[2].ToolResults) != 1 {
		t.Fatalf("tool progress was not checkpointed before the next paid turn: %+v", checkpoints[2])
	}
	if checkpoints[3].ProviderCalls != 2 || checkpoints[3].PendingProviderCall == nil || len(checkpoints[3].ToolResults) != 1 {
		t.Fatalf("second provider dispatch ordering drifted: %+v", checkpoints[3])
	}
	if checkpoints[4].ProviderCalls != 2 || checkpoints[4].PendingPaidTurn == nil || checkpoints[4].PendingProviderCall != nil {
		t.Fatalf("second paid turn ordering drifted: %+v", checkpoints[4])
	}
	if checkpoints[5].TerminalState != "completed" || checkpoints[5].PendingPaidTurn != nil || checkpoints[5].PendingProviderCall != nil {
		t.Fatalf("terminal result was not checkpointed: %+v", checkpoints[5])
	}
}

func TestRealisticControllerResumeAfterProgressCheckpointStartsAtNextTurn(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := &scriptedRealisticProvider{actions: []RealisticCandidateAction{{Kind: "read", Path: "crm/authority.json"}, {Kind: "finish", Summary: "continued"}}}
	var durable RealisticControllerResult
	checkpointCount := 0
	crashAfterProgress := func(result RealisticControllerResult) error {
		checkpointCount++
		durable = result
		if checkpointCount == 3 {
			return errors.New("simulated process crash after progress commit")
		}
		return nil
	}
	first, err := runRealisticCandidateFrozen(context.Background(), workspace, "progress-crash-family", 1, provider, nil, crashAfterProgress)
	if !errors.Is(err, ErrRealisticCheckpointDurability) || first.ProviderCalls != 1 || len(durable.ToolResults) != 1 || durable.PendingPaidTurn != nil {
		t.Fatalf("completed progress was not recoverably checkpointed: result=%+v durable=%+v err=%v", first, durable, err)
	}
	resumed, err := resumeRealisticCandidateFrozen(context.Background(), workspace, durable, provider, nil, acceptRealisticCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if provider.index != 2 || resumed.ProviderCalls != 2 || len(resumed.ToolResults) != 1 || resumed.TerminalState != "completed" {
		t.Fatalf("resume repeated completed progress or skipped the next turn: provider_calls=%d result=%+v", provider.index, resumed)
	}
}

func TestRealisticControllerWillNotBlindlyRerunInFlightProviderCall(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	providerCalls := 0
	provider := realisticProviderFunc(func(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
		providerCalls++
		return RealisticProviderResponse{}, errors.New("provider must not be called")
	})
	var durable RealisticControllerResult
	crashBeforeProviderReturns := func(result RealisticControllerResult) error {
		durable = result
		if result.PendingProviderCall != nil {
			return errors.New("simulated crash with provider outcome unknown")
		}
		return nil
	}
	first, err := runRealisticCandidateFrozen(context.Background(), workspace, "in-flight-family", 1, provider, nil, crashBeforeProviderReturns)
	if !errors.Is(err, ErrRealisticCheckpointDurability) || providerCalls != 0 || durable.PendingProviderCall == nil || durable.ProviderCalls != 1 {
		t.Fatalf("in-flight identity was not durable before dispatch: result=%+v durable=%+v calls=%d err=%v", first, durable, providerCalls, err)
	}
	resumed, err := resumeRealisticCandidateFrozen(context.Background(), workspace, durable, provider, nil, acceptRealisticCheckpoint)
	if !errors.Is(err, ErrRealisticProviderOutcomeAmbiguous) || providerCalls != 0 || resumed.FailureClass != "provider_transport_failure" || resumed.AmbiguousProviderCalls != 1 {
		t.Fatalf("resume blindly reran or hid ambiguous provider outcome: result=%+v calls=%d err=%v", resumed, providerCalls, err)
	}
}

func TestRealisticControllerDisclosesExactBoundedChecks(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	provider := realisticProviderFunc(func(_ context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
		if len(request.AllowedChecks) != 1 || request.AllowedChecks[0] != "python -m unittest discover -s tests -v" {
			t.Fatalf("bounded check contract was hidden or changed: %+v", request.AllowedChecks)
		}
		for _, requiredPath := range []string{"TASK.md", "crm/authority.json", "consumer_faulty/normalize.py", "consumer_good/normalize.py", "phone_farm_contract/expected.json", "tests/test_contract.py"} {
			if !contains(request.WorkspacePaths, requiredPath) {
				t.Fatalf("candidate-visible workspace inventory omitted %s: %+v", requiredPath, request.WorkspacePaths)
			}
		}
		return RealisticProviderResponse{Action: RealisticCandidateAction{Kind: "finish", Summary: "done"}, RawReceipt: []byte(`{"provider":"fixture"}`), InputTokens: 1, OutputTokens: 1, CostMicros: 1, AccountingStatus: "exact", TokenAccountingStatus: "exact", CostAccountingStatus: "exact"}, nil
	})
	if _, err := runRealisticCandidateFrozen(context.Background(), workspace, "check-contract-family", 1, provider, nil, acceptRealisticCheckpoint); err != nil {
		t.Fatal(err)
	}
}

func TestRealisticControllerPreservesPartialChargedTransportFailure(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, _ := PrepareRealisticCaseWorkspace(parent, "R01")
	providerCalls := 0
	provider := realisticProviderFunc(func(_ context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
		providerCalls++
		if request.ProviderCallID == "" || request.ProviderTurn != 1 {
			t.Fatalf("provider call identity missing: %+v", request)
		}
		return RealisticProviderResponse{RawReceipt: []byte(`{"jsonl":"partial"}`), InputTokens: 31, OutputTokens: 7, ReasoningTokens: 3, CostMicros: 19, AccountingStatus: "partial", TokenAccountingStatus: "reported", CostAccountingStatus: "reported"}, errors.New("connection dropped after charged response")
	})
	var durable RealisticControllerResult
	result, err := runRealisticCandidateFrozen(context.Background(), workspace, "transport-family", 1, provider, nil, func(next RealisticControllerResult) error {
		durable = next
		return nil
	})
	if err == nil || providerCalls != 1 || result.TerminalState != "invalid" || result.FailureClass != "provider_transport_failure" {
		t.Fatalf("transport failure classification drifted: result=%+v calls=%d err=%v", result, providerCalls, err)
	}
	if durable.ProviderCalls != 1 || durable.AmbiguousProviderCalls != 1 || durable.ProviderAccountingStatus != "partial" || durable.ProviderTokenAccountingStatus != "reported" || durable.ProviderCostAccountingStatus != "reported" || durable.InputTokens != 31 || durable.OutputTokens != 7 || durable.ReasoningTokens != 3 || durable.CostMicros != 19 || len(durable.ProviderReceipts) != 1 || durable.PendingProviderCall != nil {
		t.Fatalf("partial charged transport evidence was not durably preserved: %+v", durable)
	}
	if _, resumeErr := resumeRealisticCandidateFrozen(context.Background(), workspace, durable, provider, nil, acceptRealisticCheckpoint); resumeErr == nil || providerCalls != 1 {
		t.Fatalf("terminal transport failure was retried: calls=%d err=%v", providerCalls, resumeErr)
	}
}
