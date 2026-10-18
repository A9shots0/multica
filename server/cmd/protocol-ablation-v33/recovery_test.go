package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func v33RecoveryFixture(t *testing.T, currentRoot string) ([]byte, string) {
	t.Helper()
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		t.Fatal(err)
	}
	order, _ := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	b := order[0]
	plan, _ := agentqualification.BuildProtocolAblationV32OpenCodePlan(d, b)
	sessionID, requestID, assistantID := "ses_recovery", "msg_request", "msg_assistant"
	workspace := filepath.Join(currentRoot, "workspaces", safeV33(b.BindingID), "realistic-r05-v1-test")
	resolved := strings.Repeat("a", 64)
	preflight := agentqualification.OpenCodeStructuredV33Preflight{Version: agentqualification.OpenCodeStructuredV33Version, ExecutableSHA256: agentqualification.OpenCodeStructuredV33ExecutableSHA256, OpenAPISHA256: agentqualification.OpenCodeStructuredV33OpenAPISHA256, InlineConfigSHA256: plan.InlineConfigSHA256, ResolvedConfigSHA256: resolved, BaseURL: "http://127.0.0.1:1", Ready: true}
	ledger := agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{b.BindingID: {BindingID: b.BindingID, Status: "pending"}}, GlobalHalt: true, HaltReason: "transport_failure"}
	if err := writeV33Atomic(filepath.Join(currentRoot, "ledger.json"), ledger); err != nil {
		t.Fatal(err)
	}
	if err := writeV33Once(filepath.Join(currentRoot, "preflight", safeV33(b.BindingID)+".json"), mustV33(preflight)); err != nil {
		t.Fatal(err)
	}
	userInfo := map[string]any{"role": "user", "id": requestID, "sessionID": sessionID, "agent": "qualification", "tools": map[string]bool{"*": false}, "model": map[string]string{"providerID": "openrouter", "modelID": strings.TrimPrefix(b.Candidate.Model, "openrouter/"), "variant": b.Candidate.Effort}, "format": map[string]any{"type": "json_schema", "schema": json.RawMessage(plan.Schema), "retryCount": 0}}
	assistantInfo := map[string]any{"id": assistantID, "sessionID": sessionID, "role": "assistant", "parentID": requestID, "modelID": strings.TrimPrefix(b.Candidate.Model, "openrouter/"), "providerID": "openrouter", "mode": "qualification", "agent": "qualification", "variant": b.Candidate.Effort, "path": map[string]string{"cwd": workspace, "root": currentRoot}, "cost": 0.01, "tokens": map[string]any{"total": 17, "input": 10, "output": 2, "reasoning": 3, "cache": map[string]int{"read": 1, "write": 1}}, "time": map[string]int64{"created": 1, "completed": 2}, "finish": "stop", "error": map[string]any{"name": "StructuredOutputError", "data": map[string]any{"message": "Model did not produce structured output", "retries": 0}}}
	exported := map[string]any{
		"info": map[string]string{"id": sessionID, "directory": workspace},
		"messages": []any{
			map[string]any{"info": userInfo, "parts": []any{map[string]string{"type": "text", "text": plan.Prompt}}},
			map[string]any{"info": assistantInfo, "parts": []any{
				map[string]any{"id": "prt_start", "sessionID": sessionID, "messageID": assistantID, "type": "step-start"},
				map[string]any{"id": "prt_finish", "sessionID": sessionID, "messageID": assistantID, "type": "step-finish", "reason": "stop", "cost": 0.01, "tokens": map[string]any{"total": 17, "input": 10, "output": 2, "reasoning": 3, "cache": map[string]int{"read": 1, "write": 1}}},
			}},
		},
	}
	raw, _ := json.Marshal(exported)
	return append([]byte("Exporting session: "+sessionID+"\n"), raw...), sessionID
}

func TestV33RecoverySeedsAtomicallyAndReplaysWithoutProvider(t *testing.T) {
	base := t.TempDir()
	currentRoot := filepath.Join(base, "current")
	continuationRoot := filepath.Join(base, "new", "continuation")
	raw, sessionID := v33RecoveryFixture(t, currentRoot)
	before, _ := inventoryV33(currentRoot)
	first, err := recoverV33FromExport(raw, sessionID, currentRoot, continuationRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Seeded || first.ProviderCalled || first.Ledger.GlobalHalt || first.Ledger.Usage != (agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 12, OutputTokens: 2, ReasoningTokens: 3, CostMicros: 10_000}) {
		t.Fatalf("bad recovered ledger: %+v", first)
	}
	claim := first.Ledger.Claims["protocol-v32.claudesonnet5.medium.one-action-loop-v1"]
	if claim.Completed == nil || claim.Completed.FailureClass != "response_or_tool_protocol_failure" || claim.Completed.FailureScope != "observation" {
		t.Fatalf("bad local classification: %+v", claim)
	}
	second, err := recoverV33FromExport(raw, sessionID, currentRoot, continuationRoot)
	if err != nil || second.Seeded || second.ProviderCalled {
		t.Fatalf("replay was not zero-call reuse: %+v %v", second, err)
	}
	after, _ := inventoryV33(currentRoot)
	if before != after {
		t.Fatal("current root changed")
	}
	if _, err := os.Stat(filepath.Join(continuationRoot, "recovery", "opencode-export.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestV33RecoveryRejectsTokenTotalDrift(t *testing.T) {
	base := t.TempDir()
	currentRoot := filepath.Join(base, "current")
	raw, sessionID := v33RecoveryFixture(t, currentRoot)
	raw = []byte(strings.Replace(string(raw), `"total":17`, `"total":18`, 1))
	if _, err := recoverV33FromExport(raw, sessionID, currentRoot, filepath.Join(base, "continuation")); err == nil {
		t.Fatal("token-total drift accepted")
	}
}

func v33TimeoutRecoveryFixture(t *testing.T, sourceRoot string) ([]byte, string) {
	t.Helper()
	d, _ := agentqualification.CurrentProtocolAblationV32()
	order, _ := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	b := order[1]
	plan, _ := agentqualification.BuildProtocolAblationV32OpenCodePlan(d, b)
	sessionID, requestID, assistantID := "ses_timeout", "msg_timeout_request", "msg_timeout_assistant"
	workspace := filepath.Join(sourceRoot, "workspaces", safeV33(b.BindingID), "realistic-r05-v1-test")
	userInfo := map[string]any{"role": "user", "id": requestID, "sessionID": sessionID, "agent": "qualification", "tools": map[string]bool{"*": false}, "model": map[string]string{"providerID": "openrouter", "modelID": strings.TrimPrefix(b.Candidate.Model, "openrouter/"), "variant": b.Candidate.Effort}, "format": map[string]any{"type": "json_schema", "schema": json.RawMessage(plan.Schema), "retryCount": 0}}
	zeroTokens := map[string]any{"input": 0, "output": 0, "reasoning": 0, "cache": map[string]int{"read": 0, "write": 0}}
	assistantInfo := map[string]any{"id": assistantID, "sessionID": sessionID, "role": "assistant", "parentID": requestID, "modelID": strings.TrimPrefix(b.Candidate.Model, "openrouter/"), "providerID": "openrouter", "mode": "qualification", "agent": "qualification", "variant": b.Candidate.Effort, "path": map[string]string{"cwd": workspace, "root": filepath.Dir(sourceRoot)}, "cost": 0, "tokens": zeroTokens, "time": map[string]int64{"created": 1, "completed": 2}, "error": map[string]any{"name": "MessageAbortedError", "data": map[string]string{"message": "Aborted"}}}
	exported := map[string]any{
		"info": map[string]any{"id": sessionID, "directory": workspace, "cost": 0, "tokens": zeroTokens},
		"messages": []any{
			map[string]any{"info": userInfo, "parts": []any{map[string]string{"type": "text", "text": plan.Prompt}}},
			map[string]any{"info": assistantInfo, "parts": []any{
				map[string]any{"id": "prt_timeout_start", "sessionID": sessionID, "messageID": assistantID, "type": "step-start"},
				map[string]any{"id": "prt_timeout_reasoning", "sessionID": sessionID, "messageID": assistantID, "type": "reasoning", "text": ""},
				map[string]any{"id": "prt_timeout_text", "sessionID": sessionID, "messageID": assistantID, "type": "text", "text": ""},
			}},
		},
	}
	raw, _ := json.Marshal(exported)
	return append([]byte("Exporting session: "+sessionID+"\n"), raw...), sessionID
}

func TestV33TimeoutRecoveryClosesSecondCellInvalidWithoutUsageOrRetry(t *testing.T) {
	base := t.TempDir()
	initial := filepath.Join(base, "initial")
	firstRoot := filepath.Join(base, "continuation-01")
	firstExport, firstSession := v33RecoveryFixture(t, initial)
	if _, err := recoverV33FromExport(firstExport, firstSession, initial, firstRoot); err != nil {
		t.Fatal(err)
	}
	d, _ := agentqualification.CurrentProtocolAblationV32()
	order, _ := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	b := order[1]
	ledgerRaw, _ := os.ReadFile(filepath.Join(firstRoot, "ledger.json"))
	var halted agentqualification.ProtocolAblationV32Ledger
	_ = json.Unmarshal(ledgerRaw, &halted)
	halted.Claims[b.BindingID] = agentqualification.ProtocolAblationV32Claim{BindingID: b.BindingID, Status: "pending"}
	halted.GlobalHalt, halted.HaltReason = true, "transport_failure"
	if err := writeV33Atomic(filepath.Join(firstRoot, "ledger.json"), halted); err != nil {
		t.Fatal(err)
	}
	plan, _ := agentqualification.BuildProtocolAblationV32OpenCodePlan(d, b)
	preflight := agentqualification.OpenCodeStructuredV33Preflight{Version: agentqualification.OpenCodeStructuredV33Version, ExecutableSHA256: agentqualification.OpenCodeStructuredV33ExecutableSHA256, OpenAPISHA256: agentqualification.OpenCodeStructuredV33OpenAPISHA256, InlineConfigSHA256: plan.InlineConfigSHA256, ResolvedConfigSHA256: strings.Repeat("b", 64), BaseURL: "http://127.0.0.1:1", Ready: true}
	if err := writeV33Once(filepath.Join(firstRoot, "preflight", safeV33(b.BindingID)+".json"), mustV33(preflight)); err != nil {
		t.Fatal(err)
	}
	timeoutExport, timeoutSession := v33TimeoutRecoveryFixture(t, firstRoot)
	secondRoot := filepath.Join(base, "continuation-02")
	before, _ := inventoryV33(firstRoot)
	got, err := recoverV33TimeoutFromExport(timeoutExport, timeoutSession, firstRoot, secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Seeded || got.ProviderCalled || got.Ledger.GlobalHalt || got.Ledger.Usage != halted.Usage || len(got.Ledger.Claims) != 2 {
		t.Fatalf("bad timeout recovery: %+v", got)
	}
	completed := got.Ledger.Claims[b.BindingID].Completed
	if completed == nil || completed.Outcome != "invalid" || completed.FailureClass != "transport_timeout" || completed.FailureScope != "observation" || completed.Usage != (agentqualification.ProtocolAblationV32Usage{}) {
		t.Fatalf("bad timeout observation: %+v", completed)
	}
	replay, err := recoverV33TimeoutFromExport(timeoutExport, timeoutSession, firstRoot, secondRoot)
	if err != nil || replay.Seeded || replay.ProviderCalled {
		t.Fatalf("timeout replay failed: %+v %v", replay, err)
	}
	after, _ := inventoryV33(firstRoot)
	if before != after {
		t.Fatal("continuation-01 changed")
	}
	bad := []byte(strings.Replace(string(timeoutExport), `"type":"reasoning"`, `"type":"retry"`, 1))
	if _, err := recoverV33TimeoutFromExport(bad, timeoutSession, firstRoot, filepath.Join(base, "bad")); err == nil {
		t.Fatal("retry part accepted")
	}
}

func TestV33ReadinessRecoverySeedsOnlyClosedClaudeCells(t *testing.T) {
	base := t.TempDir()
	initial, firstRoot, secondRoot := filepath.Join(base, "initial"), filepath.Join(base, "continuation-01"), filepath.Join(base, "continuation-02")
	firstExport, firstSession := v33RecoveryFixture(t, initial)
	if _, err := recoverV33FromExport(firstExport, firstSession, initial, firstRoot); err != nil {
		t.Fatal(err)
	}
	d, _ := agentqualification.CurrentProtocolAblationV32()
	order, _ := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	oneShot := order[1]
	raw, _ := os.ReadFile(filepath.Join(firstRoot, "ledger.json"))
	var halted agentqualification.ProtocolAblationV32Ledger
	_ = json.Unmarshal(raw, &halted)
	halted.Claims[oneShot.BindingID] = agentqualification.ProtocolAblationV32Claim{BindingID: oneShot.BindingID, Status: "pending"}
	halted.GlobalHalt, halted.HaltReason = true, "transport_failure"
	_ = writeV33Atomic(filepath.Join(firstRoot, "ledger.json"), halted)
	plan, _ := agentqualification.BuildProtocolAblationV32OpenCodePlan(d, oneShot)
	preflight := agentqualification.OpenCodeStructuredV33Preflight{Version: agentqualification.OpenCodeStructuredV33Version, ExecutableSHA256: agentqualification.OpenCodeStructuredV33ExecutableSHA256, OpenAPISHA256: agentqualification.OpenCodeStructuredV33OpenAPISHA256, InlineConfigSHA256: plan.InlineConfigSHA256, ResolvedConfigSHA256: strings.Repeat("c", 64), BaseURL: "http://127.0.0.1:1", Ready: true}
	_ = writeV33Once(filepath.Join(firstRoot, "preflight", safeV33(oneShot.BindingID)+".json"), mustV33(preflight))
	timeoutExport, timeoutSession := v33TimeoutRecoveryFixture(t, firstRoot)
	if _, err := recoverV33TimeoutFromExport(timeoutExport, timeoutSession, firstRoot, secondRoot); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(secondRoot, "ledger.json"))
	_ = json.Unmarshal(raw, &halted)
	halted.GlobalHalt, halted.HaltReason = true, "runtime_failure"
	_ = writeV33Atomic(filepath.Join(secondRoot, "ledger.json"), halted)
	gpt := order[2]
	_ = writeV33Once(filepath.Join(secondRoot, "halts", safeV33(gpt.BindingID)+".json"), []byte(`{"event":"runtime_failure"}`))
	before, _ := inventoryV33(secondRoot)
	thirdRoot := filepath.Join(base, "continuation-03")
	got, err := recoverV33ReadinessFailure(secondRoot, thirdRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Seeded || got.ProviderCalled || got.Ledger.GlobalHalt || len(got.Ledger.Claims) != 2 {
		t.Fatalf("bad readiness recovery: %+v", got)
	}
	if _, ok := got.Ledger.Claims[gpt.BindingID]; ok {
		t.Fatal("fabricated GPT claim")
	}
	replay, err := recoverV33ReadinessFailure(secondRoot, thirdRoot)
	if err != nil || replay.Seeded || replay.ProviderCalled {
		t.Fatalf("bad readiness replay: %+v %v", replay, err)
	}
	after, _ := inventoryV33(secondRoot)
	if before != after {
		t.Fatal("continuation-02 changed")
	}
}
