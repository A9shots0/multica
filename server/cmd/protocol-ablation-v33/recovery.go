package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type v33RecoveryResult struct {
	SessionID, CurrentRootInventorySHA256, ExportSHA256, ContinuationRoot string
	Seeded, ProviderCalled                                                bool
	Ledger                                                                agentqualification.ProtocolAblationV32Ledger
}

type v33Export struct {
	Info struct {
		ID, Directory string
		Cost          json.Number
		Tokens        v33AbortedTokens
	} `json:"info"`
	Messages []struct {
		Info  json.RawMessage   `json:"info"`
		Parts []json.RawMessage `json:"parts"`
	} `json:"messages"`
}

type v33AbortedTokens struct {
	Total     *int64 `json:"total,omitempty"`
	Input     int64  `json:"input"`
	Output    int64  `json:"output"`
	Reasoning int64  `json:"reasoning"`
	Cache     struct {
		Read, Write int64
	} `json:"cache"`
}

func recoverV33FromSession(ctx context.Context, executable, sessionID, currentRoot, continuationRoot string) (v33RecoveryResult, error) {
	if executable == "" || sessionID == "" {
		return v33RecoveryResult{}, errors.New("v33 recovery requires exact OpenCode executable and session")
	}
	out, err := exec.CommandContext(ctx, executable, "export", sessionID).Output()
	if err != nil {
		return v33RecoveryResult{}, fmt.Errorf("v33 local export: %w", err)
	}
	return recoverV33FromExport(out, sessionID, currentRoot, continuationRoot)
}

func recoverV33TimeoutFromSession(ctx context.Context, executable, sessionID, sourceRoot, continuationRoot string) (v33RecoveryResult, error) {
	if executable == "" || sessionID == "" {
		return v33RecoveryResult{}, errors.New("v33 timeout recovery requires exact OpenCode executable and session")
	}
	out, err := exec.CommandContext(ctx, executable, "export", sessionID).Output()
	if err != nil {
		return v33RecoveryResult{}, fmt.Errorf("v33 local timeout export: %w", err)
	}
	return recoverV33TimeoutFromExport(out, sessionID, sourceRoot, continuationRoot)
}

func recoverV33ReadinessFailure(sourceRoot, continuationRoot string) (v33RecoveryResult, error) {
	result := v33RecoveryResult{ContinuationRoot: continuationRoot, ProviderCalled: false}
	if err := verifyV33CommandFreeze(); err != nil {
		return result, err
	}
	before, err := inventoryV33(sourceRoot)
	if err != nil {
		return result, err
	}
	result.CurrentRootInventorySHA256 = before
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return result, err
	}
	order, err := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	if err != nil || len(order) != 4 {
		return result, errors.New("v33 readiness recovery frozen order unavailable")
	}
	ledgerRaw, err := os.ReadFile(filepath.Join(sourceRoot, "ledger.json"))
	if err != nil {
		return result, err
	}
	var halted agentqualification.ProtocolAblationV32Ledger
	if json.Unmarshal(ledgerRaw, &halted) != nil || !halted.GlobalHalt || halted.HaltReason != "runtime_failure" || len(halted.Claims) != 2 {
		return result, errors.New("v33 readiness source is not exact pre-claim runtime halt")
	}
	ledger := agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{}, Usage: halted.Usage}
	for _, b := range order[:2] {
		claim := halted.Claims[b.BindingID]
		if claim.Status != "completed" || claim.Completed == nil {
			return result, errors.New("v33 readiness source lacks both closed Claude cells")
		}
		ledger.Claims[b.BindingID] = claim
	}
	gpt := order[2]
	for _, rel := range []string{filepath.Join("claims", safeV33(gpt.BindingID)+".claim"), filepath.Join("preflight", safeV33(gpt.BindingID)+".json"), filepath.Join("observations", safeV33(gpt.BindingID), "observation.json"), filepath.Join("raw", safeV33(gpt.BindingID))} {
		if _, err := os.Stat(filepath.Join(sourceRoot, rel)); !errors.Is(err, os.ErrNotExist) {
			return result, fmt.Errorf("v33 readiness source contains GPT attempt evidence: %s", rel)
		}
	}
	haltRel := filepath.Join("halts", safeV33(gpt.BindingID)+".json")
	haltRaw, err := os.ReadFile(filepath.Join(sourceRoot, haltRel))
	if err != nil {
		return result, errors.New("v33 readiness halt evidence missing")
	}
	result.Ledger = ledger
	if existing, err := os.ReadFile(filepath.Join(continuationRoot, "ledger.json")); err == nil {
		var replay agentqualification.ProtocolAblationV32Ledger
		if json.Unmarshal(existing, &replay) != nil || !equalJSONV33(existing, mustV33(ledger)) {
			return result, errors.New("v33 readiness continuation replay differs")
		}
		result.Ledger, result.Seeded = replay, false
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	} else {
		parent := filepath.Dir(continuationRoot)
		if err := os.MkdirAll(parent, 0700); err != nil {
			return result, err
		}
		stage, err := os.MkdirTemp(parent, ".v33-continuation-readiness-")
		if err != nil {
			return result, err
		}
		committed := false
		defer func() {
			if !committed {
				_ = os.RemoveAll(stage)
			}
		}()
		for _, b := range order[:2] {
			for _, rel := range []string{filepath.Join("claims", safeV33(b.BindingID)+".claim"), filepath.Join("preflight", safeV33(b.BindingID)+".json"), filepath.Join("observations", safeV33(b.BindingID), "observation.json"), filepath.Join("raw", safeV33(b.BindingID), "provider-01.json")} {
				raw, err := os.ReadFile(filepath.Join(sourceRoot, rel))
				if err != nil {
					return result, err
				}
				if err := writeV33Once(filepath.Join(stage, rel), raw); err != nil {
					return result, err
				}
			}
		}
		for _, dir := range []string{"recovery", "recovery-timeout"} {
			for _, name := range []string{"opencode-export.txt", "source-root-ledger.json", "recovery.json"} {
				rel := filepath.Join(dir, name)
				raw, err := os.ReadFile(filepath.Join(sourceRoot, rel))
				if err != nil {
					return result, err
				}
				if err := writeV33Once(filepath.Join(stage, rel), raw); err != nil {
					return result, err
				}
			}
		}
		meta := map[string]any{"provider_called": false, "source_root_inventory_sha256": before, "classification": "local_readiness_transport_failure_before_claim", "gpt_sessions_created": 0, "gpt_provider_turns": 0, "gpt_tokens": 0, "gpt_cost_usd_exact": "0", "source_halt_sha256": hashV33(haltRaw)}
		for path, raw := range map[string][]byte{
			filepath.Join(stage, "recovery-readiness", "source-root-ledger.json"): ledgerRaw,
			filepath.Join(stage, "recovery-readiness", "source-halt.json"):        haltRaw,
			filepath.Join(stage, "recovery-readiness", "recovery.json"):           mustV33(meta),
			filepath.Join(stage, "ledger.json"):                                   mustV33(ledger),
		} {
			if err := writeV33Once(path, raw); err != nil {
				return result, err
			}
		}
		if _, err := os.Stat(continuationRoot); !errors.Is(err, os.ErrNotExist) {
			return result, errors.New("v33 readiness continuation root already appeared")
		}
		if err := os.Rename(stage, continuationRoot); err != nil {
			return result, err
		}
		committed, result.Seeded = true, true
	}
	after, err := inventoryV33(sourceRoot)
	if err != nil || after != before {
		return result, errors.New("v33 readiness source changed during recovery")
	}
	return result, nil
}

func recoverV33TimeoutFromExport(exportRaw []byte, sessionID, sourceRoot, continuationRoot string) (v33RecoveryResult, error) {
	result := v33RecoveryResult{SessionID: sessionID, ContinuationRoot: continuationRoot, ProviderCalled: false, ExportSHA256: hashV33(exportRaw)}
	if err := verifyV33CommandFreeze(); err != nil {
		return result, err
	}
	before, err := inventoryV33(sourceRoot)
	if err != nil {
		return result, err
	}
	result.CurrentRootInventorySHA256 = before
	jsonStart := bytes.IndexByte(exportRaw, '{')
	if jsonStart < 0 {
		return result, errors.New("v33 timeout export lacks JSON")
	}
	var exported v33Export
	if err := json.Unmarshal(exportRaw[jsonStart:], &exported); err != nil || exported.Info.ID != sessionID || len(exported.Messages) != 2 {
		return result, errors.New("v33 timeout export/session envelope mismatch")
	}
	if exported.Info.Cost.String() != "0" || !zeroV33AbortedTokens(exported.Info.Tokens) {
		return result, errors.New("v33 timeout session recorded usage or cost")
	}
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return result, err
	}
	order, err := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	if err != nil || len(order) != 4 {
		return result, errors.New("v33 frozen order unavailable")
	}
	completedBinding, binding := order[0], order[1]
	ledgerRaw, err := os.ReadFile(filepath.Join(sourceRoot, "ledger.json"))
	if err != nil {
		return result, err
	}
	var halted agentqualification.ProtocolAblationV32Ledger
	if json.Unmarshal(ledgerRaw, &halted) != nil || !halted.GlobalHalt || halted.HaltReason != "transport_failure" || len(halted.Claims) != 2 || halted.Claims[binding.BindingID].Status != "pending" {
		return result, errors.New("v33 timeout source is not the exact second-cell transport halt")
	}
	first := halted.Claims[completedBinding.BindingID]
	if first.Status != "completed" || first.Completed == nil || first.Completed.Usage != halted.Usage {
		return result, errors.New("v33 timeout source completed accounting drift")
	}
	preflightRaw, err := os.ReadFile(filepath.Join(sourceRoot, "preflight", safeV33(binding.BindingID)+".json"))
	if err != nil {
		return result, err
	}
	var preflight agentqualification.OpenCodeStructuredV33Preflight
	if json.Unmarshal(preflightRaw, &preflight) != nil || agentqualification.ValidateOpenCodeStructuredV33Ready(preflight) != nil {
		return result, errors.New("v33 timeout source preflight invalid")
	}
	var user struct {
		Role, ID, SessionID, Agent string
		Tools                      map[string]bool
		Model                      struct{ ProviderID, ModelID, Variant string }
		Format                     struct {
			Type       string
			Schema     json.RawMessage
			RetryCount int
		}
	}
	if json.Unmarshal(exported.Messages[0].Info, &user) != nil || user.Role != "user" || user.ID == "" || user.SessionID != sessionID || user.Agent != "qualification" || len(user.Tools) != 1 || user.Tools["*"] || user.Model.ProviderID != "openrouter" || user.Model.ModelID != strings.TrimPrefix(binding.Candidate.Model, "openrouter/") || user.Model.Variant != binding.Candidate.Effort || user.Format.Type != "json_schema" || user.Format.RetryCount != 0 || len(exported.Messages[0].Parts) != 1 {
		return result, errors.New("v33 timeout exported request identity drift")
	}
	var textPart struct{ Type, Text string }
	if json.Unmarshal(exported.Messages[0].Parts[0], &textPart) != nil || textPart.Type != "text" {
		return result, errors.New("v33 timeout exported prompt missing")
	}
	plan, err := agentqualification.BuildProtocolAblationV32OpenCodePlan(d, binding)
	if err != nil || textPart.Text != plan.Prompt || !equalJSONV33(user.Format.Schema, plan.Schema) {
		return result, errors.New("v33 timeout exported prompt/schema drift")
	}
	capsule, err := agentqualification.BuildOpenCodeStructuredV33CapsuleForPlan(d, binding, plan, sessionID, user.ID, exported.Info.Directory, preflight.ResolvedConfigSHA256)
	if err != nil {
		return result, err
	}
	if capsule.HTTPTimeoutSeconds != agentqualification.OpenCodeStructuredV33HTTPTimeoutSeconds {
		return result, errors.New("v33 timeout capsule limit drift")
	}
	assistantRaw, _ := json.Marshal(struct {
		Info  json.RawMessage   `json:"info"`
		Parts []json.RawMessage `json:"parts"`
	}{exported.Messages[1].Info, exported.Messages[1].Parts})
	if err := validateV33AbortedAssistant(exported.Messages[1].Info, exported.Messages[1].Parts, sessionID, user.ID, exported.Info.Directory, binding); err != nil {
		return result, err
	}
	observation := agentqualification.ProtocolAblationV32Observation{
		BindingID:        binding.BindingID,
		Outcome:          "invalid",
		FailureClass:     "transport_timeout",
		FailureScope:     "observation",
		ProviderIdentity: agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: strings.TrimPrefix(binding.Candidate.Model, "openrouter/"), ReportedEffort: binding.Candidate.Effort, ReportedServingProvider: binding.Candidate.ServingProvider, ResolvedConfigSHA256: preflight.ResolvedConfigSHA256, ServingProviderEvidence: "enforced_single_route_no_fallback"},
		RawReceipts:      []json.RawMessage{assistantRaw},
		GraderResult:     agentqualification.RealisticNextCalibrationResult{CaseID: "P32"},
	}
	observation.EvidenceSHA256 = agentqualification.ProtocolAblationV32ObservationEvidenceSHA(observation)
	ledger := agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{completedBinding.BindingID: first}, Usage: halted.Usage}
	decision, _, err := agentqualification.BeginProtocolAblationV32Observation(d, &ledger, binding)
	if err != nil || decision != "launch" {
		return result, fmt.Errorf("v33 timeout continuation claim: %w", err)
	}
	if err := agentqualification.CompleteOpenCodeStructuredV33TransportTimeout(d, &ledger, binding, observation); err != nil {
		return result, err
	}
	result.Ledger = ledger
	if existing, err := os.ReadFile(filepath.Join(continuationRoot, "ledger.json")); err == nil {
		var replay agentqualification.ProtocolAblationV32Ledger
		if json.Unmarshal(existing, &replay) != nil || !equalJSONV33(existing, mustV33(ledger)) {
			return result, errors.New("v33 timeout continuation replay differs")
		}
		existingExport, err := os.ReadFile(filepath.Join(continuationRoot, "recovery-timeout", "opencode-export.txt"))
		if err != nil || !bytes.Equal(existingExport, exportRaw) {
			return result, errors.New("v33 timeout continuation export differs")
		}
		result.Ledger, result.Seeded = replay, false
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	} else {
		parent := filepath.Dir(continuationRoot)
		if err := os.MkdirAll(parent, 0700); err != nil {
			return result, err
		}
		stage, err := os.MkdirTemp(parent, ".v33-continuation-timeout-")
		if err != nil {
			return result, err
		}
		committed := false
		defer func() {
			if !committed {
				_ = os.RemoveAll(stage)
			}
		}()
		for _, rel := range []string{
			filepath.Join("recovery", "opencode-export.txt"), filepath.Join("recovery", "source-root-ledger.json"), filepath.Join("recovery", "recovery.json"),
			filepath.Join("preflight", safeV33(completedBinding.BindingID)+".json"), filepath.Join("claims", safeV33(completedBinding.BindingID)+".claim"),
			filepath.Join("raw", safeV33(completedBinding.BindingID), "provider-01.json"), filepath.Join("observations", safeV33(completedBinding.BindingID), "observation.json"),
		} {
			raw, err := os.ReadFile(filepath.Join(sourceRoot, rel))
			if err != nil {
				return result, fmt.Errorf("v33 timeout copy prior evidence %s: %w", rel, err)
			}
			if err := writeV33Once(filepath.Join(stage, rel), raw); err != nil {
				return result, fmt.Errorf("v33 timeout copy prior evidence %s: %w", rel, err)
			}
		}
		meta := map[string]any{"session_id": sessionID, "provider_called": false, "source_root_inventory_sha256": before, "export_sha256": result.ExportSHA256, "classification": "invalid_unscored_transport_timeout", "error": "MessageAbortedError: Aborted", "retries": 0, "provider_turns": 0, "tokens": 0, "cost_usd_exact": "0"}
		for path, raw := range map[string][]byte{
			filepath.Join(stage, "recovery-timeout", "opencode-export.txt"):                      exportRaw,
			filepath.Join(stage, "recovery-timeout", "source-root-ledger.json"):                  ledgerRaw,
			filepath.Join(stage, "recovery-timeout", "recovery.json"):                            mustV33(meta),
			filepath.Join(stage, "preflight", safeV33(binding.BindingID)+".json"):                preflightRaw,
			filepath.Join(stage, "claims", safeV33(binding.BindingID)+".claim"):                  []byte(binding.BindingID + "\n"),
			filepath.Join(stage, "raw", safeV33(binding.BindingID), "provider-01.json"):          assistantRaw,
			filepath.Join(stage, "observations", safeV33(binding.BindingID), "observation.json"): mustV33(observation),
			filepath.Join(stage, "ledger.json"):                                                  mustV33(ledger),
		} {
			if err := writeV33Once(path, raw); err != nil {
				return result, err
			}
		}
		if _, err := os.Stat(continuationRoot); !errors.Is(err, os.ErrNotExist) {
			return result, errors.New("v33 timeout continuation root already appeared")
		}
		if err := os.Rename(stage, continuationRoot); err != nil {
			return result, err
		}
		committed, result.Seeded = true, true
	}
	after, err := inventoryV33(sourceRoot)
	if err != nil || after != before {
		return result, errors.New("v33 timeout source root changed during recovery")
	}
	return result, nil
}

func zeroV33AbortedTokens(t v33AbortedTokens) bool {
	return t.Total == nil && t.Input == 0 && t.Output == 0 && t.Reasoning == 0 && t.Cache.Read == 0 && t.Cache.Write == 0
}

func validateV33AbortedAssistant(infoRaw json.RawMessage, parts []json.RawMessage, sessionID, requestID, workspace string, binding agentqualification.ProtocolAblationV32Binding) error {
	var info struct {
		ID, SessionID, Role, ParentID, ModelID, ProviderID, Mode, Agent, Variant, Finish string
		Path                                                                             struct{ Cwd, Root string }
		Cost                                                                             json.Number
		Tokens                                                                           v33AbortedTokens
		Time                                                                             struct{ Created, Completed int64 }
		Error                                                                            struct {
			Name string
			Data struct{ Message string }
		}
		Structured json.RawMessage
	}
	if json.Unmarshal(infoRaw, &info) != nil || !strings.HasPrefix(info.ID, "msg_") || info.SessionID != sessionID || info.Role != "assistant" || info.ParentID != requestID || info.ModelID != strings.TrimPrefix(binding.Candidate.Model, "openrouter/") || info.ProviderID != "openrouter" || info.Mode != "qualification" || info.Agent != "qualification" || info.Variant != binding.Candidate.Effort || info.Finish != "" || info.Path.Cwd != workspace || info.Path.Root == "" || info.Cost.String() != "0" || !zeroV33AbortedTokens(info.Tokens) || info.Time.Created <= 0 || info.Time.Completed < info.Time.Created || info.Error.Name != "MessageAbortedError" || info.Error.Data.Message != "Aborted" || len(bytes.TrimSpace(info.Structured)) != 0 {
		return errors.New("v33 timeout assistant is not exact zero-usage MessageAbortedError")
	}
	starts := 0
	ids := map[string]bool{}
	for _, raw := range parts {
		var p struct{ ID, SessionID, MessageID, Type string }
		if json.Unmarshal(raw, &p) != nil || p.ID == "" || ids[p.ID] || p.SessionID != sessionID || p.MessageID != info.ID {
			return errors.New("v33 timeout malformed/unrelated part")
		}
		ids[p.ID] = true
		switch p.Type {
		case "step-start":
			starts++
		case "reasoning", "text":
		case "step-finish", "retry", "tool", "subtask", "patch":
			return fmt.Errorf("v33 timeout forbidden %s part", p.Type)
		default:
			return fmt.Errorf("v33 timeout unknown %s part", p.Type)
		}
	}
	if starts != 1 {
		return errors.New("v33 timeout does not prove exactly one started turn")
	}
	return nil
}

func recoverV33FromExport(exportRaw []byte, sessionID, currentRoot, continuationRoot string) (v33RecoveryResult, error) {
	result := v33RecoveryResult{SessionID: sessionID, ContinuationRoot: continuationRoot, ProviderCalled: false, ExportSHA256: hashV33(exportRaw)}
	if err := verifyV33CommandFreeze(); err != nil {
		return result, err
	}
	before, err := inventoryV33(currentRoot)
	if err != nil {
		return result, err
	}
	result.CurrentRootInventorySHA256 = before
	jsonStart := bytes.IndexByte(exportRaw, '{')
	if jsonStart < 0 {
		return result, errors.New("v33 export lacks JSON")
	}
	var exported v33Export
	if err := json.Unmarshal(exportRaw[jsonStart:], &exported); err != nil || exported.Info.ID != sessionID || len(exported.Messages) != 2 {
		return result, errors.New("v33 export/session envelope mismatch")
	}
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return result, err
	}
	order, err := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	if err != nil || len(order) != 4 {
		return result, errors.New("v33 frozen order unavailable")
	}
	binding := order[0]
	ledgerRaw, err := os.ReadFile(filepath.Join(currentRoot, "ledger.json"))
	if err != nil {
		return result, err
	}
	var halted agentqualification.ProtocolAblationV32Ledger
	if json.Unmarshal(ledgerRaw, &halted) != nil || !halted.GlobalHalt || halted.HaltReason != "transport_failure" || halted.Claims[binding.BindingID].Status != "pending" || halted.Usage != (agentqualification.ProtocolAblationV32Usage{}) {
		return result, errors.New("v33 source root is not the exact zero-accounting pending halt")
	}
	preflightRaw, err := os.ReadFile(filepath.Join(currentRoot, "preflight", safeV33(binding.BindingID)+".json"))
	if err != nil {
		return result, err
	}
	var preflight agentqualification.OpenCodeStructuredV33Preflight
	if json.Unmarshal(preflightRaw, &preflight) != nil || agentqualification.ValidateOpenCodeStructuredV33Ready(preflight) != nil {
		return result, errors.New("v33 source preflight invalid")
	}
	var user struct {
		Role, ID, SessionID, Agent string
		Tools                      map[string]bool
		Model                      struct{ ProviderID, ModelID, Variant string }
		Format                     struct {
			Type       string
			Schema     json.RawMessage
			RetryCount int
		}
	}
	if json.Unmarshal(exported.Messages[0].Info, &user) != nil || user.Role != "user" || user.ID == "" || user.SessionID != sessionID || user.Agent != "qualification" || user.Model.ProviderID != "openrouter" || user.Model.ModelID != strings.TrimPrefix(binding.Candidate.Model, "openrouter/") || user.Model.Variant != binding.Candidate.Effort || user.Tools["*"] || user.Format.Type != "json_schema" || user.Format.RetryCount != 0 || len(exported.Messages[0].Parts) != 1 {
		return result, errors.New("v33 exported request identity drift")
	}
	var textPart struct{ Type, Text string }
	if json.Unmarshal(exported.Messages[0].Parts[0], &textPart) != nil || textPart.Type != "text" {
		return result, errors.New("v33 exported prompt missing")
	}
	plan, err := agentqualification.BuildProtocolAblationV32OpenCodePlan(d, binding)
	if err != nil || textPart.Text != plan.Prompt || !equalJSONV33(user.Format.Schema, plan.Schema) {
		return result, errors.New("v33 exported prompt/schema drift")
	}
	capsule, err := agentqualification.BuildOpenCodeStructuredV33CapsuleForPlan(d, binding, plan, sessionID, user.ID, exported.Info.Directory, preflight.ResolvedConfigSHA256)
	if err != nil {
		return result, err
	}
	assistantRaw, _ := json.Marshal(struct {
		Info  json.RawMessage   `json:"info"`
		Parts []json.RawMessage `json:"parts"`
	}{exported.Messages[1].Info, exported.Messages[1].Parts})
	receipt, err := agentqualification.ParseOpenCodeStructuredV33HTTPResponse(capsule, 200, "application/json", assistantRaw)
	if err != nil {
		return result, err
	}
	if err := agentqualification.ValidateOpenCodeStructuredV33ReceiptForPlan(d, binding, plan, capsule, receipt); err != nil || receipt.Outcome != "structured_output_error" || receipt.StructuredErr == nil || receipt.StructuredErr.Retries != 0 {
		return result, fmt.Errorf("v33 export is not exact zero-retry structured-output failure: %w", err)
	}
	receiptRaw := mustV33(receipt)
	observation := agentqualification.ProtocolAblationV32Observation{
		BindingID:        binding.BindingID,
		Outcome:          "failed",
		FailureClass:     "response_or_tool_protocol_failure",
		FailureScope:     "observation",
		Usage:            receipt.Usage,
		ProviderIdentity: agentqualification.ProtocolAblationV32ProviderIdentity{DirectProvider: "openrouter", ReportedModel: receipt.ModelID, ReportedEffort: receipt.Variant, ReportedServingProvider: binding.Candidate.ServingProvider, ResolvedConfigSHA256: preflight.ResolvedConfigSHA256, ServingProviderEvidence: "enforced_single_route_no_fallback"},
		RawReceipts:      []json.RawMessage{receiptRaw},
		GraderResult:     agentqualification.RealisticNextCalibrationResult{CaseID: "P32", Verdict: agentqualification.RealisticNextCandidateFailure, Reasons: []string{"structured_output_error_zero_retry"}},
	}
	observation.EvidenceSHA256 = agentqualification.ProtocolAblationV32ObservationEvidenceSHA(observation)
	ledger := agentqualification.ProtocolAblationV32Ledger{}
	decision, _, err := agentqualification.BeginProtocolAblationV32Observation(d, &ledger, binding)
	if err != nil || decision != "launch" {
		return result, fmt.Errorf("v33 recovery claim: %w", err)
	}
	if err := agentqualification.CompleteProtocolAblationV32Observation(d, &ledger, binding, observation); err != nil {
		return result, err
	}
	result.Ledger = ledger
	if existing, err := os.ReadFile(filepath.Join(continuationRoot, "ledger.json")); err == nil {
		var replay agentqualification.ProtocolAblationV32Ledger
		if json.Unmarshal(existing, &replay) != nil {
			return result, errors.New("v33 continuation replay is corrupt")
		}
		claim := replay.Claims[binding.BindingID]
		if replay.GlobalHalt || replay.Usage != ledger.Usage || claim.Status != "completed" || claim.Completed == nil || claim.Completed.EvidenceSHA256 != observation.EvidenceSHA256 || claim.Completed.FailureClass != observation.FailureClass || len(claim.Completed.RawReceipts) != 1 || !equalJSONV33(claim.Completed.RawReceipts[0], receiptRaw) {
			gotEvidence, gotFailure, rawCount, rawEqual := "", "", 0, false
			if claim.Completed != nil {
				gotEvidence, gotFailure, rawCount = claim.Completed.EvidenceSHA256, claim.Completed.FailureClass, len(claim.Completed.RawReceipts)
				if rawCount == 1 {
					rawEqual = equalJSONV33(claim.Completed.RawReceipts[0], receiptRaw)
				}
			}
			return result, fmt.Errorf("v33 continuation replay differs: halt=%v usage=%+v want_usage=%+v claim=%s completed_nil=%v evidence=%s/%s failure=%s/%s raw_count=%d raw_equal=%v", replay.GlobalHalt, replay.Usage, ledger.Usage, claim.Status, claim.Completed == nil, gotEvidence, observation.EvidenceSHA256, gotFailure, observation.FailureClass, rawCount, rawEqual)
		}
		existingExport, err := os.ReadFile(filepath.Join(continuationRoot, "recovery", "opencode-export.txt"))
		if err != nil || !bytes.Equal(existingExport, exportRaw) {
			return result, errors.New("v33 continuation export differs")
		}
		result.Ledger = replay
		result.Seeded = false
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	} else {
		parent := filepath.Dir(continuationRoot)
		if err := os.MkdirAll(parent, 0700); err != nil {
			return result, err
		}
		stage, err := os.MkdirTemp(parent, ".v33-continuation-")
		if err != nil {
			return result, err
		}
		committed := false
		defer func() {
			if !committed {
				_ = os.RemoveAll(stage)
			}
		}()
		meta := map[string]any{"session_id": sessionID, "provider_called": false, "source_root_inventory_sha256": before, "export_sha256": result.ExportSHA256, "classification": "candidate_local_response_or_tool_protocol_failure", "cost_usd_exact": "0.0246365"}
		for path, raw := range map[string][]byte{
			filepath.Join(stage, "recovery", "opencode-export.txt"):                              exportRaw,
			filepath.Join(stage, "recovery", "source-root-ledger.json"):                          ledgerRaw,
			filepath.Join(stage, "recovery", "recovery.json"):                                    mustV33(meta),
			filepath.Join(stage, "preflight", safeV33(binding.BindingID)+".json"):                preflightRaw,
			filepath.Join(stage, "claims", safeV33(binding.BindingID)+".claim"):                  []byte(binding.BindingID + "\n"),
			filepath.Join(stage, "raw", safeV33(binding.BindingID), "provider-01.json"):          receiptRaw,
			filepath.Join(stage, "observations", safeV33(binding.BindingID), "observation.json"): mustV33(observation),
			filepath.Join(stage, "ledger.json"):                                                  mustV33(ledger),
		} {
			if err := writeV33Once(path, raw); err != nil {
				return result, err
			}
		}
		if _, err := os.Stat(continuationRoot); !errors.Is(err, os.ErrNotExist) {
			return result, errors.New("v33 continuation root already appeared")
		}
		if err := os.Rename(stage, continuationRoot); err != nil {
			return result, err
		}
		committed = true
		result.Seeded = true
	}
	after, err := inventoryV33(currentRoot)
	if err != nil || after != before {
		return result, errors.New("v33 source root changed during recovery")
	}
	return result, nil
}

func equalJSONV33(a, b []byte) bool {
	var ca, cb bytes.Buffer
	return json.Compact(&ca, a) == nil && json.Compact(&cb, b) == nil && bytes.Equal(ca.Bytes(), cb.Bytes())
}

func inventoryV33(root string) (string, error) {
	entries := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		entries = append(entries, filepath.ToSlash(rel)+"|"+hashV33(raw))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	return hashV33([]byte(strings.Join(entries, "\n"))), nil
}
