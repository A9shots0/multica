package agentqualification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type protocolV32FakeProvider struct {
	binding   ProtocolAblationV32Binding
	responses []json.RawMessage
	calls     int
}

func (p *protocolV32FakeProvider) Call(_ context.Context, _ OpenCodeScreeningPlan) (ProtocolAblationV32ProviderResponse, error) {
	response := ProtocolAblationV32ProviderResponse{CandidateJSON: p.responses[p.calls], RawReceipt: json.RawMessage(`{"receipt":true}`), ProviderIdentity: ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: strings.TrimPrefix(p.binding.Candidate.Model, "openrouter/"), ReportedEffort: p.binding.Candidate.Effort, ReportedServingProvider: p.binding.Candidate.ServingProvider, ResolvedConfigSHA256: strings.Repeat("b", 64)}, Usage: ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 10, OutputTokens: 5, CostMicros: 2}}
	p.calls++
	return response, nil
}

func TestProtocolAblationV32CommonPacketAndCounterbalance(t *testing.T) {
	d, err := CurrentProtocolAblationV32()
	if err != nil {
		t.Fatal(err)
	}
	ordered, err := ProtocolAblationV32ExecutionOrder(d)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"protocol-v32.claudesonnet5.medium.one-action-loop-v1", "protocol-v32.claudesonnet5.medium.one-shot-plan-v1", "protocol-v32.gpt54.medium.one-shot-plan-v1", "protocol-v32.gpt54.medium.one-action-loop-v1"}
	var first []byte
	for i, binding := range ordered {
		if binding.BindingID != want[i] {
			t.Fatalf("counterbalanced order drifted: got %s want %s", binding.BindingID, want[i])
		}
		plan, err := BuildProtocolAblationV32ProviderFreePlan(d, binding)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = append([]byte(nil), plan.InitialPacket...)
		} else if !bytes.Equal(first, plan.InitialPacket) {
			t.Fatal("A/B common initial packet bytes differ")
		}
		var decoded ProtocolAblationV32InitialPacket
		if err := json.Unmarshal(plan.InitialPacket, &decoded); err != nil || decoded.CaseID != "P32" || decoded.Task == "" || len(decoded.FixtureFiles) == 0 || len(decoded.AllowedActions) != 5 || len(decoded.AllowedChecks) != 2 {
			t.Fatalf("common packet is incomplete: %+v err=%v", decoded, err)
		}
		opencodePlan, err := BuildProtocolAblationV32OpenCodePlan(d, binding)
		if err != nil || opencodePlan.PromptSHA256 != plan.PromptSHA256 || opencodePlan.SchemaSHA256 != plan.SchemaSHA256 || opencodePlan.InlineConfigSHA256 != plan.InlineConfigSHA256 || opencodePlan.Model != binding.Candidate.Model || opencodePlan.Effort != binding.Candidate.Effort || opencodePlan.ProviderFallbacks == nil || *opencodePlan.ProviderFallbacks {
			t.Fatalf("executable OpenCode plan drifted: %+v err=%v", opencodePlan, err)
		}
	}
}

func TestProtocolAblationV32DefinitionIsPairedProviderLockedAndStrictlyCapped(t *testing.T) {
	d, err := CurrentProtocolAblationV32()
	if err != nil {
		t.Fatal(err)
	}
	bindings := ProtocolAblationV32Bindings(d)
	if len(bindings) != 4 || !d.ProviderCallsAllowed || d.Status != "frozen_reviewed" || d.Retries != 0 || d.ProviderFallbacks {
		t.Fatalf("v32 is not a locked paired no-retry ablation: %+v", d)
	}
	var cost int64
	var turns int
	for _, binding := range bindings {
		cost += ProtocolAblationV32CostReservation(d, binding.Candidate)
		turns += binding.Protocol.MaxProviderTurns
	}
	if cost != d.MaxCombinedCostMicros || turns != d.MaxCombinedProviderTurns || int64(len(bindings))*d.PerObservationTotalTokenReservation != d.MaxCombinedTokens {
		t.Fatalf("combined caps are not exactly reservation-derived: cost=%d turns=%d", cost, turns)
	}
	if ProtocolAblationV32MinimumLoopTurns() != 8 || d.Protocols[0].MaxProviderTurns != ProtocolAblationV32MinimumLoopTurns()+2 {
		t.Fatalf("loop ceiling did not correct the invalid six-turn draft: %+v", d.Protocols[0])
	}
}

func TestProtocolAblationV32PlansDifferOnlyByProtocolWithinEachModel(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	bindings := ProtocolAblationV32Bindings(d)
	for index := 0; index < len(bindings); index += 2 {
		loop, err := BuildProtocolAblationV32ProviderFreePlan(d, bindings[index])
		if err != nil {
			t.Fatal(err)
		}
		oneShot, err := BuildProtocolAblationV32ProviderFreePlan(d, bindings[index+1])
		if err != nil {
			t.Fatal(err)
		}
		if !ProtocolAblationV32ProtocolDifference(loop, oneShot) {
			t.Fatalf("paired plans changed more than protocol: loop=%+v one-shot=%+v", loop, oneShot)
		}
		if !strings.Contains(string(oneShot.ResponseSchema), "on_success") || !strings.Contains(string(oneShot.ResponseSchema), "on_failure") || !strings.Contains(string(oneShot.ResponseSchema), `"maxItems":10`) {
			t.Fatalf("one-shot arm is not a complete bounded conditional plan: %s", oneShot.ResponseSchema)
		}
	}
}

func TestProtocolAblationV32UsesIdenticalFixtureChecksAndGraderEvidence(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	results := make([]RealisticNextCalibrationResult, 0, 2)
	for range d.Protocols {
		workspace, err := PrepareRealisticNextR05V1(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		policy := `{"schema_version":"silver-mediums.r05-decision-policy.v1","authority":"owner_receipt","preserve_accepted":true,"preserve_rejected":true,"preserve_scope":true,"preserve_out_of_scope":true,"status":"resumed","needs_you":false,"repeat_completed_checkpoint":false,"production_changed":false,"phone_farm_accessed":false}`
		if err := os.WriteFile(filepath.Join(workspace.Root, "business", "decision_policy.json"), []byte(policy), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := r05V1RunHarnessCheck(workspace.Root, R05V1SimulateCheck, workspace.r05); err != nil {
			t.Fatal(err)
		}
		if _, err := r05V1RunHarnessCheck(workspace.Root, R05V1VerifyCheck, workspace.r05); err != nil {
			t.Fatal(err)
		}
		doc := []byte("DEC-R05-001. Status: resumed. Production changed: false. Phone Farm accessed: false.\n")
		for _, relative := range []string{"docs/HANDOFF.md", "docs/CURRENT_STATE.md", "docs/SYSTEM-MAP.md"} {
			if err := os.WriteFile(filepath.Join(workspace.Root, filepath.FromSlash(relative)), doc, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(workspace.Root, "EVIDENCE.md"), []byte("DEC-R05-001 proves checkpoint CHK-R05-001 was not repeated and stale chat was not authority.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		results = append(results, EvaluateProtocolAblationV32(workspace.InitialCopy, workspace.Root, workspace.r05))
	}
	if results[0].Verdict != RealisticNextPass || results[1].Verdict != RealisticNextPass || results[0].CaseID != "P32" || ProtocolAblationV32EvidenceSHA(results[0]) != ProtocolAblationV32EvidenceSHA(results[1]) {
		t.Fatalf("protocol arms do not share exact acceptance evidence: %+v", results)
	}
}

func TestProtocolAblationV32ConditionalPlanUsesSameControllerAndCanExpressCompleteOutcome(t *testing.T) {
	workspace, err := PrepareRealisticNextR05V1(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	policy := `{"schema_version":"silver-mediums.r05-decision-policy.v1","authority":"owner_receipt","preserve_accepted":true,"preserve_rejected":true,"preserve_scope":true,"preserve_out_of_scope":true,"status":"resumed","needs_you":false,"repeat_completed_checkpoint":false,"production_changed":false,"phone_farm_accessed":false}`
	doc := "DEC-R05-001. Status: resumed. Production changed: false. Phone Farm accessed: false.\n"
	actions := []RealisticCandidateAction{
		{Kind: "edit", Path: "business/decision_policy.json", Contents: policy},
		{Kind: "run_check", Check: R05V1SimulateCheck},
		{Kind: "run_check", Check: R05V1VerifyCheck},
		{Kind: "edit", Path: "docs/HANDOFF.md", Contents: doc},
		{Kind: "edit", Path: "docs/CURRENT_STATE.md", Contents: doc},
		{Kind: "edit", Path: "docs/SYSTEM-MAP.md", Contents: doc},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "DEC-R05-001 proves checkpoint CHK-R05-001 was not repeated and stale chat was not authority.\n"},
		{Kind: "finish", Summary: "bounded decision resumed"},
	}
	steps := make([]ProtocolAblationV32ConditionalStep, 0, len(actions))
	for index, action := range actions {
		next := "complete"
		if index+1 < len(actions) {
			next = string(rune('a' + index + 1))
		}
		steps = append(steps, ProtocolAblationV32ConditionalStep{ID: string(rune('a' + index)), Action: action, OnSuccess: next, OnFailure: "fail"})
	}
	result := ExecuteProtocolAblationV32ConditionalPlan(t.Context(), workspace, ProtocolAblationV32ConditionalPlan{EntryStep: "a", Steps: steps})
	if result.FailureClass != "" || result.Verdict.Verdict != RealisticNextPass || result.Verdict.CaseID != "P32" || len(result.ToolResults) != 7 {
		t.Fatalf("conditional plan did not use the shared executor/grader successfully: %+v", result)
	}
}

func TestProtocolAblationV32StrongPassHasIdenticalExecutedActionsAndGraderAcrossArms(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	bindings := ProtocolAblationV32Bindings(d)[:2]
	policy := `{"schema_version":"silver-mediums.r05-decision-policy.v1","authority":"owner_receipt","preserve_accepted":true,"preserve_rejected":true,"preserve_scope":true,"preserve_out_of_scope":true,"status":"resumed","needs_you":false,"repeat_completed_checkpoint":false,"production_changed":false,"phone_farm_accessed":false}`
	doc := "DEC-R05-001. Status: resumed. Production changed: false. Phone Farm accessed: false.\n"
	actions := []RealisticCandidateAction{
		{Kind: "edit", Path: "business/decision_policy.json", Contents: policy},
		{Kind: "run_check", Check: R05V1SimulateCheck}, {Kind: "run_check", Check: R05V1VerifyCheck},
		{Kind: "edit", Path: "docs/HANDOFF.md", Contents: doc}, {Kind: "edit", Path: "docs/CURRENT_STATE.md", Contents: doc}, {Kind: "edit", Path: "docs/SYSTEM-MAP.md", Contents: doc},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "DEC-R05-001 proves checkpoint CHK-R05-001 was not repeated and stale chat was not authority.\n"},
		{Kind: "finish", Summary: "bounded decision resumed"},
	}
	loopResponses := make([]json.RawMessage, 0, len(actions))
	steps := make([]ProtocolAblationV32ConditionalStep, 0, len(actions))
	for index, action := range actions {
		raw, _ := json.Marshal(struct {
			Action RealisticCandidateAction `json:"action"`
		}{action})
		loopResponses = append(loopResponses, raw)
		next := "complete"
		if index+1 < len(actions) {
			next = fmt.Sprintf("step-%d", index+2)
		}
		steps = append(steps, ProtocolAblationV32ConditionalStep{ID: fmt.Sprintf("step-%d", index+1), Action: action, OnSuccess: next, OnFailure: "fail"})
	}
	oneShotRaw, _ := json.Marshal(ProtocolAblationV32ConditionalPlan{EntryStep: "step-1", Steps: steps})
	providers := []*protocolV32FakeProvider{{binding: bindings[0], responses: loopResponses}, {binding: bindings[1], responses: []json.RawMessage{oneShotRaw}}}
	observations := make([]ProtocolAblationV32Observation, 0, 2)
	for index, binding := range bindings {
		workspace, err := PrepareRealisticNextR05V1(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		observation, err := RunProtocolAblationV32Observation(t.Context(), d, binding, workspace, providers[index])
		if err != nil || observation.Outcome != "pass" {
			t.Fatalf("strong pass failed for %s: %+v err=%v", binding.Protocol.ProtocolID, observation, err)
		}
		ledger := ProtocolAblationV32Ledger{}
		_, _, _ = BeginProtocolAblationV32Observation(d, &ledger, binding)
		if err := CompleteProtocolAblationV32Observation(d, &ledger, binding, observation); err != nil {
			t.Fatalf("strong pass receipt was rejected: %v", err)
		}
		observations = append(observations, observation)
	}
	if !reflect.DeepEqual(observations[0].Actions, observations[1].Actions) || !reflect.DeepEqual(observations[0].GraderResult, observations[1].GraderResult) {
		t.Fatalf("protocol arms did not produce identical executed-action/grader receipts: loop=%+v plan=%+v", observations[0], observations[1])
	}
	if len(observations[0].RawReceipts) != 8 || len(observations[1].RawReceipts) != 1 {
		t.Fatalf("provider-call protocol distinction was not preserved: loop=%d plan=%d", len(observations[0].RawReceipts), len(observations[1].RawReceipts))
	}
}

func TestProtocolAblationV32OneShotEnvelopeFitsGeneratedTokenCeiling(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	steps := make([]ProtocolAblationV32ConditionalStep, 10)
	for index := range steps {
		steps[index] = ProtocolAblationV32ConditionalStep{ID: strings.Repeat("i", 64), Action: RealisticCandidateAction{Kind: "edit", Path: strings.Repeat("p", 128), Query: strings.Repeat("q", 128), Contents: strings.Repeat("c", 512), Check: strings.Repeat("k", 128), Summary: strings.Repeat("s", 256)}, OnSuccess: strings.Repeat("u", 64), OnFailure: strings.Repeat("f", 64)}
	}
	raw, err := json.Marshal(ProtocolAblationV32ConditionalPlan{EntryStep: strings.Repeat("e", 64), Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > ProtocolAblationV32MaximalPlanEnvelopeBytes() || int64(len(raw)) > d.PerObservationGeneratedTokenCeiling {
		t.Fatalf("maximal valid one-shot envelope exceeds its conservative output capacity: bytes=%d ceiling=%d", len(raw), d.PerObservationGeneratedTokenCeiling)
	}
}

func TestProtocolAblationV32AccountingStopAndZeroRepeatRecovery(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	bindings := ProtocolAblationV32Bindings(d)
	ledger := ProtocolAblationV32Ledger{}
	for _, binding := range bindings {
		decision, _, err := BeginProtocolAblationV32Observation(d, &ledger, binding)
		if err != nil || decision != "launch" {
			t.Fatalf("valid binding was not admitted: %s %v", decision, err)
		}
		observation := sealProtocolAblationV32Observation(binding, ProtocolAblationV32Observation{BindingID: binding.BindingID, Outcome: "pass", Usage: ProtocolAblationV32Usage{ProviderTurns: binding.Protocol.MaxProviderTurns, InputTokens: 55_000, OutputTokens: 20_000, CostMicros: ProtocolAblationV32CostReservation(d, binding.Candidate)}})
		if err := CompleteProtocolAblationV32Observation(d, &ledger, binding, observation); err != nil {
			t.Fatal(err)
		}
	}
	if ledger.GlobalHalt || ledger.Usage.ProviderTurns != d.MaxCombinedProviderTurns || ledger.Usage.InputTokens+ledger.Usage.OutputTokens+ledger.Usage.ReasoningTokens != d.MaxCombinedTokens || ledger.Usage.CostMicros != d.MaxCombinedCostMicros {
		t.Fatalf("v32 exact accounting did not close: %+v", ledger)
	}
	raw, _ := json.Marshal(ledger)
	var restored ProtocolAblationV32Ledger
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	decision, reused, err := BeginProtocolAblationV32Observation(d, &restored, bindings[0])
	if err != nil || decision != "reuse" || reused == nil || reused.EvidenceSHA256 == "" {
		t.Fatalf("completed observation was not reused without a repeat: %s %+v %v", decision, reused, err)
	}

	local := ProtocolAblationV32Ledger{}
	decision, _, _ = BeginProtocolAblationV32Observation(d, &local, bindings[0])
	if decision != "launch" {
		t.Fatal("local-failure fixture was not admitted")
	}
	failed := sealProtocolAblationV32Observation(bindings[0], ProtocolAblationV32Observation{BindingID: bindings[0].BindingID, Outcome: "failed", FailureClass: "competence_failure", Usage: ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 10, OutputTokens: 2, CostMicros: 10}})
	if err := CompleteProtocolAblationV32Observation(d, &local, bindings[0], failed); err != nil || local.GlobalHalt || !ProtocolAblationV32ObservationLocalFailure(failed.FailureClass) {
		t.Fatalf("candidate failure incorrectly halted the batch: %+v err=%v", local, err)
	}

	pending := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &pending, bindings[0])
	raw, _ = json.Marshal(pending)
	_ = json.Unmarshal(raw, &pending)
	decision, _, _ = BeginProtocolAblationV32Observation(d, &pending, bindings[0])
	if decision != "halt" || !pending.GlobalHalt || pending.HaltReason != "ambiguous_transport" {
		t.Fatalf("pending restart did not fail closed without a repeat: %+v", pending)
	}
}

func TestProtocolAblationV32GlobalFailureAndBudgetAdmissionHalt(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	binding := ProtocolAblationV32Bindings(d)[0]
	ledger := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &ledger, binding)
	unsafe := sealProtocolAblationV32Observation(binding, ProtocolAblationV32Observation{BindingID: binding.BindingID, Outcome: "invalid", FailureClass: "unsafe_execution", Usage: ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 1, OutputTokens: 1, CostMicros: 1}})
	if err := CompleteProtocolAblationV32Observation(d, &ledger, binding, unsafe); err != nil || !ledger.GlobalHalt || ledger.HaltReason != "unsafe_execution" {
		t.Fatalf("unsafe execution did not halt globally: %+v err=%v", ledger, err)
	}

	budget := ProtocolAblationV32Ledger{Usage: ProtocolAblationV32Usage{CostMicros: d.MaxCombinedCostMicros}}
	decision, _, _ := BeginProtocolAblationV32Observation(d, &budget, binding)
	if decision != "halt" || budget.HaltReason != "budget_breach" || len(budget.Claims) != 0 {
		t.Fatalf("budget admission did not stop before a claim/provider: %+v", budget)
	}
}

func TestProtocolAblationV32RejectsIdentityAndFailureClassDrift(t *testing.T) {
	d, err := CurrentProtocolAblationV32()
	if err != nil {
		t.Fatal(err)
	}
	drifted := d
	drifted.Candidates = append([]ProtocolAblationV32Candidate(nil), d.Candidates...)
	drifted.Candidates[0].ConfigurationID = "unsealed-candidate"
	if err := ValidateProtocolAblationV32(drifted); err == nil {
		t.Fatal("candidate identity drift was accepted")
	}

	binding := ProtocolAblationV32Bindings(d)[0]
	ledger := ProtocolAblationV32Ledger{}
	decision, _, err := BeginProtocolAblationV32Observation(d, &ledger, binding)
	if err != nil || decision != "launch" {
		t.Fatalf("observation was not admitted: decision=%s err=%v", decision, err)
	}
	unknown := ProtocolAblationV32Observation{
		BindingID:      binding.BindingID,
		Outcome:        "failed",
		FailureClass:   "invented_failure_class",
		Usage:          ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 1, OutputTokens: 1, CostMicros: 1},
		EvidenceSHA256: "deliberately-wrong",
	}
	if err := CompleteProtocolAblationV32Observation(d, &ledger, binding, unknown); err == nil {
		t.Fatal("unknown failure class was accepted")
	}
	if ledger.Claims[binding.BindingID].Status != "pending" || ledger.Usage.ProviderTurns != 0 {
		t.Fatalf("rejected completion mutated durable accounting: %+v", ledger)
	}
}

func TestProtocolAblationV32LedgerFailClosedAndCapBreachReceipt(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	binding := ProtocolAblationV32Bindings(d)[0]
	halted := ProtocolAblationV32Ledger{GlobalHalt: true, HaltReason: "evidence_integrity_failure"}
	if decision, _, err := BeginProtocolAblationV32Observation(d, &halted, binding); err != nil || decision != "halt" || len(halted.Claims) != 0 {
		t.Fatalf("existing global halt was not honored: %+v decision=%s err=%v", halted, decision, err)
	}
	drifted := binding
	drifted.BindingID += ".drift"
	if _, _, err := BeginProtocolAblationV32Observation(d, &ProtocolAblationV32Ledger{}, drifted); err == nil {
		t.Fatal("non-matrix binding was admitted")
	}

	negative := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &negative, binding)
	bad := sealProtocolAblationV32Observation(binding, ProtocolAblationV32Observation{BindingID: binding.BindingID, Outcome: "pass", Usage: ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: -1}})
	if err := CompleteProtocolAblationV32Observation(d, &negative, binding, bad); err == nil || negative.Usage.ProviderTurns != 0 {
		t.Fatalf("negative usage was not rejected without accounting mutation: %+v err=%v", negative, err)
	}

	breach := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &breach, binding)
	over := sealProtocolAblationV32Observation(binding, ProtocolAblationV32Observation{BindingID: binding.BindingID, Outcome: "pass", Usage: ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: d.PerObservationTotalTokenReservation + 1, CostMicros: 1}})
	if err := CompleteProtocolAblationV32Observation(d, &breach, binding, over); err != nil || !breach.GlobalHalt || breach.HaltReason != "budget_breach" || breach.Usage.InputTokens != d.PerObservationTotalTokenReservation+1 || breach.Claims[binding.BindingID].Status != "completed" || breach.Claims[binding.BindingID].Completed.FailureClass != "budget_breach" {
		t.Fatalf("cap-breach observation was not preserved/accounted before halt: %+v err=%v", breach, err)
	}

	normalized := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &normalized, binding)
	legacy := sealProtocolAblationV32Observation(binding, ProtocolAblationV32Observation{BindingID: binding.BindingID, Outcome: "invalid", FailureClass: "invalid_test", Usage: ProtocolAblationV32Usage{ProviderTurns: 1}})
	if err := CompleteProtocolAblationV32Observation(d, &normalized, binding, legacy); err != nil || normalized.HaltReason != "test_invalidity" || normalized.Claims[binding.BindingID].Completed.FailureClass != "test_invalidity" {
		t.Fatalf("test-invalidity vocabulary was not normalized: %+v err=%v", normalized, err)
	}

	localInvalid := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &localInvalid, binding)
	parserDefect := sealProtocolAblationV32Observation(binding, ProtocolAblationV32Observation{BindingID: binding.BindingID, Outcome: "invalid", FailureClass: "test_invalidity", FailureScope: "observation", Usage: ProtocolAblationV32Usage{ProviderTurns: 1}})
	if err := CompleteProtocolAblationV32Observation(d, &localInvalid, binding, parserDefect); err != nil || localInvalid.GlobalHalt {
		t.Fatalf("observation-specific parser invalidity halted unrelated bindings: %+v err=%v", localInvalid, err)
	}
	next := ProtocolAblationV32Bindings(d)[1]
	if decision, _, err := BeginProtocolAblationV32Observation(d, &localInvalid, next); err != nil || decision != "launch" {
		t.Fatalf("unaffected binding did not continue after local invalidity: decision=%s err=%v", decision, err)
	}
}

func sealProtocolAblationV32Observation(binding ProtocolAblationV32Binding, observation ProtocolAblationV32Observation) ProtocolAblationV32Observation {
	observation.ProviderIdentity = ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: strings.TrimPrefix(binding.Candidate.Model, "openrouter/"), ReportedEffort: binding.Candidate.Effort, ReportedServingProvider: binding.Candidate.ServingProvider, ResolvedConfigSHA256: strings.Repeat("a", 64)}
	observation.RawReceipts = []json.RawMessage{json.RawMessage(`{"receipt":true}`)}
	observation.GraderResult.CaseID = "P32"
	observation.EvidenceSHA256 = ProtocolAblationV32ObservationEvidenceSHA(observation)
	return observation
}

func TestProtocolV32ServingProviderEvidenceModes(t *testing.T) {
	d, err := CurrentProtocolAblationV32()
	if err != nil {
		t.Fatal(err)
	}
	binding := ProtocolAblationV32Bindings(d)[0]
	makeObservation := func(provider, mode string) ProtocolAblationV32Observation {
		observation := ProtocolAblationV32Observation{BindingID: binding.BindingID, Outcome: "failed", FailureClass: "response_or_tool_protocol_failure", Usage: ProtocolAblationV32Usage{ProviderTurns: 1}}
		observation.ProviderIdentity = ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: strings.TrimPrefix(binding.Candidate.Model, "openrouter/"), ReportedEffort: binding.Candidate.Effort, ReportedServingProvider: provider, ResolvedConfigSHA256: strings.Repeat("a", 64), ServingProviderEvidence: mode}
		observation.RawReceipts = []json.RawMessage{json.RawMessage(`{"resolved_config_sha256":"` + strings.Repeat("a", 64) + `"}`)}
		observation.GraderResult.CaseID = "P32"
		observation.EvidenceSHA256 = ProtocolAblationV32ObservationEvidenceSHA(observation)
		return observation
	}
	ledger := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &ledger, binding)
	if err := CompleteProtocolAblationV32Observation(d, &ledger, binding, makeObservation("", "enforced_single_route_no_fallback")); err != nil {
		t.Fatalf("exact enforced route was rejected: %v", err)
	}
	contradictory := ProtocolAblationV32Ledger{}
	_, _, _ = BeginProtocolAblationV32Observation(d, &contradictory, binding)
	if err := CompleteProtocolAblationV32Observation(d, &contradictory, binding, makeObservation("openai", "reported_serving_provider")); err == nil {
		t.Fatal("contradictory serving provider was accepted")
	}
}
