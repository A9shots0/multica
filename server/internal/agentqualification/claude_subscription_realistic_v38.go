package agentqualification

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ClaudeSubscriptionRealisticV38Schema = "silver-mediums.claude-subscription-realistic.v38"

const (
	ClaudeRealisticV38FailureProtocol = "response_or_tool_protocol_failure"
	ClaudeRealisticV38FailureResource = "observation_resource_exhaustion"
	ClaudeRealisticV38FailureEvidence = "evidence_corruption"
)

type ClaudeRealisticV38Error struct {
	Class string
	Cause error
}

func (failure *ClaudeRealisticV38Error) Error() string {
	return failure.Class + ": " + failure.Cause.Error()
}

func (failure *ClaudeRealisticV38Error) Unwrap() error { return failure.Cause }

func FailClaudeRealisticV38(class string, cause error) error {
	return &ClaudeRealisticV38Error{Class: class, Cause: cause}
}

type ClaudeSubscriptionRealisticV38Definition struct {
	SchemaVersion              string                            `json:"schema_version"`
	BatchID                    string                            `json:"batch_id"`
	Status                     string                            `json:"status"`
	Transport                  string                            `json:"transport"`
	Topology                   string                            `json:"topology"`
	ProviderCallsAllowed       bool                              `json:"provider_calls_allowed"`
	Configuration              ClaudeRealisticV36Configuration   `json:"configuration"`
	ExecutionOrder             []string                          `json:"execution_order"`
	Caps                       ClaudeRealisticV37Caps            `json:"caps"`
	RuntimeIdentity            ClaudeRealisticV36RuntimeIdentity `json:"runtime_identity"`
	FrozenAntecedents          map[string]string                 `json:"frozen_antecedents"`
	Attempt1                   string                            `json:"attempt_1"`
	Attempt2                   string                            `json:"attempt_2"`
	ProductionAllowed          bool                              `json:"production_allowed"`
	ExternalSideEffectsAllowed bool                              `json:"external_side_effects_allowed"`
	RoleSelectionAllowed       bool                              `json:"role_selection_allowed"`
}

type ClaudeRealisticV38ProviderReceipt ClaudeRealisticV37ProviderReceipt

//go:embed claude_subscription_realistic_v38.json
var claudeSubscriptionRealisticV38JSON []byte

func ClaudeSubscriptionRealisticV38() (ClaudeSubscriptionRealisticV38Definition, error) {
	var d ClaudeSubscriptionRealisticV38Definition
	dec := json.NewDecoder(bytes.NewReader(claudeSubscriptionRealisticV38JSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, err
	}
	if d.SchemaVersion != ClaudeSubscriptionRealisticV38Schema || d.BatchID != "claude-subscription-realistic-r04-phased-plan-v38" || d.Status != "reviewed_frozen" || !d.ProviderCallsAllowed || d.Transport != ClaudeCodeSubscriptionToolFreeTransport || d.Topology != "harness_interruption_then_bounded_phased_candidate_plans_to_deterministic_controller" || d.Configuration.ID != "realistic.recovery.claude46.medium.subscription.phased-plan.v38" || d.Configuration.Model != "claude-sonnet-4-6" || d.Configuration.ReasoningEffort != "medium" || d.Configuration.RuntimeProvider != "claude_code_subscription" || d.Configuration.ServingProvider != "anthropic" || d.Configuration.ProviderFallbacks || len(d.ExecutionOrder) != 1 || d.ExecutionOrder[0] != "R04:v1" || d.Caps != (ClaudeRealisticV37Caps{5, 4, 20, 60000, 12000, 250000, 0}) || d.Attempt1 != "harness_forced_interruption_zero_provider_calls" || d.Attempt2 != "exactly_once_resume_same_run_family" || d.ProductionAllowed || d.ExternalSideEffectsAllowed || d.RoleSelectionAllowed || len(d.FrozenAntecedents) != 4 {
		return d, fmt.Errorf("v38 definition drifted")
	}
	return d, nil
}
func ClaudeSubscriptionRealisticV38SHA256() string {
	return sha256Hex(claudeSubscriptionRealisticV38JSON)
}
func RequireClaudeSubscriptionRealisticV38Freeze() error {
	_, err := CurrentClaudeSubscriptionRealisticV38Freeze()
	return err
}

type noopV38Provider struct{}

func (noopV38Provider) Next(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
	return RealisticProviderResponse{}, fmt.Errorf("unexpected provider call")
}

func InitializeClaudeSubscriptionRealisticV38(workspace RealisticCaseWorkspace, runFamily string, checkpoint RealisticControllerCheckpoint) (ClaudeRealisticV37ExecutionState, error) {
	first, err := RunRealisticCandidate(context.Background(), workspace, runFamily, 1, noopV38Provider{}, nil, checkpoint)
	if err != nil || first.TerminalState != "interrupted" || first.ProviderCalls != 0 || len(first.AttemptHistory) != 1 {
		return ClaudeRealisticV37ExecutionState{}, fmt.Errorf("v38 forced interruption failed: %w", err)
	}
	first.Attempt = 2
	first.TerminalState = ""
	return ClaudeRealisticV37ExecutionState{Controller: first}, nil
}

func BuildClaudeSubscriptionRealisticV38PhaseRequest(workspace RealisticCaseWorkspace, runFamily string, state ClaudeRealisticV37ExecutionState) (ClaudeScreeningPlan, RealisticProviderRequest, error) {
	d, err := ClaudeSubscriptionRealisticV38()
	if err != nil {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, err
	}
	phase := state.CompletedPhases + 1
	if workspace.CaseID != "R04" || workspace.CaseVersion != "v1" || runFamily == "" || phase < 1 || phase > 5 || state.PendingPlan != nil || len(state.Cumulative) != state.CompletedPhases || state.Controller.Attempt != 2 || len(state.Controller.AttemptHistory) != 1 || state.Controller.AttemptHistory[0].Attempt != 1 || state.Controller.AttemptHistory[0].TerminalState != "interrupted" || state.Controller.AttemptHistory[0].ProviderCalls != 0 {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v38 state identity invalid")
	}
	offset, actions := 0, 0
	for i, e := range state.Cumulative {
		if e.Phase != i+1 || e.ToolResultCount < 0 || offset+e.ToolResultCount > len(state.Controller.ToolResults) || !validSHA256(e.ResultsSHA256) || !validSHA256(e.WorkspaceTreeSHA256) {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v38 cumulative invalid")
		}
		raw, _ := json.Marshal(state.Controller.ToolResults[offset : offset+e.ToolResultCount])
		if sha256Hex(raw) != e.ResultsSHA256 {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v38 result hash drift")
		}
		offset += e.ToolResultCount
		actions += e.ActionCount
	}
	if offset != len(state.Controller.ToolResults) || actions != state.TotalActions {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v38 cumulative counts drift")
	}
	tree, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, err
	}
	if phase == 1 {
		if tree != workspace.StartingTreeSHA256 || len(state.PriorPhaseResults) != 0 {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v38 initial tree drift")
		}
	} else {
		last := state.Cumulative[len(state.Cumulative)-1]
		raw, _ := json.Marshal(state.PriorPhaseResults)
		if sha256Hex(raw) != last.ResultsSHA256 || tree != last.WorkspaceTreeSHA256 {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v38 prior state drift")
		}
	}
	task, err := workspace.Read("TASK.md")
	if err != nil {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, err
	}
	paths, err := workspace.Paths()
	if err != nil {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, err
	}
	ledger, _ := json.Marshal(state.Cumulative)
	req := RealisticProviderRequest{ProviderCallID: fmt.Sprintf("%s:attempt-2:phase-%d", runFamily, phase), ProviderTurn: phase, CaseID: "R04", CaseVersion: "v1", RunFamilyID: runFamily, Attempt: 2, Task: string(task) + "\n\nCumulative completed-phase ledger (hashes only): " + string(ledger), WorkspacePaths: paths, PreviousResults: append([]RealisticToolResult(nil), state.PriorPhaseResults...), AllowedActions: []string{"read", "search", "edit", "run_check", "finish"}, AllowedChecks: append([]string(nil), workspace.Fixture.AllowlistedChecks...), MaxSteps: 4}
	plan, err := buildClaudeSubscriptionRealisticV38Plan(d, req)
	return plan, req, err
}

func buildClaudeSubscriptionRealisticV38Plan(d ClaudeSubscriptionRealisticV38Definition, req RealisticProviderRequest) (ClaudeScreeningPlan, error) {
	raw, _ := json.Marshal(req)
	schema := []byte(`{"type":"object","additionalProperties":false,"required":["actions"],"properties":{"actions":{"type":"array","minItems":1,"maxItems":4,"items":{"type":"object","additionalProperties":false,"required":["kind"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","finish"]},"path":{"type":"string"},"query":{"type":"string"},"contents":{"type":"string"},"check":{"type":"string"},"summary":{"type":"string"}}}}}}`)
	prompt := "Return the next bounded phase for sealed synthetic R04. You have no tools. Use durable workspace truth, reject superseded status, never repeat completed work, and finish only after exact-family attempt-2 recovery is proven. No Browser, network, deployment, messages, credentials, Phone Farm, external effects, or undisclosed paths.\n\n<response-schema>\n" + string(schema) + "\n</response-schema>\n<workspace-request>\n" + string(raw) + "\n</workspace-request>\n"
	args := []string{"-p", "--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--no-session-persistence", "--permission-prompts", "none", "--output-format", "json", "--json-schema", "<generated-schema>", "--max-turns", "1", "--model", d.Configuration.Model, "--effort", d.Configuration.ReasoningEffort}
	return ClaudeScreeningPlan{Transport: d.Transport, CandidateID: d.Configuration.ID, CaseID: "R04", CaseVersion: "v1", Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)), Schema: schema, SchemaSHA256: sha256Hex(schema), ArgsTemplate: args}, nil
}

func ParseClaudeSubscriptionRealisticV38Response(req RealisticProviderRequest, plan ClaudeScreeningPlan, raw []byte, started, completed string) (ClaudeRealisticV37Plan, []byte, error) {
	d, err := ClaudeSubscriptionRealisticV38()
	if err != nil {
		return ClaudeRealisticV37Plan{}, nil, err
	}
	result, candidateRaw, err := ParseClaudeCodeSubscriptionJSONArtifact(raw, started, completed)
	if err != nil {
		return ClaudeRealisticV37Plan{}, nil, err
	}
	if result.ReportedModel != d.Configuration.Model || result.Usage.InputTokens == nil || result.Usage.OutputTokens == nil || result.Usage.ProviderReportedCostMicros == nil {
		return ClaudeRealisticV37Plan{}, nil, fmt.Errorf("v38 accounting missing")
	}
	var candidate ClaudeRealisticV37Plan
	dec := json.NewDecoder(bytes.NewReader(candidateRaw))
	dec.DisallowUnknownFields()
	if dec.Decode(&candidate) != nil || dec.Decode(&struct{}{}) != io.EOF {
		return candidate, nil, fmt.Errorf("v38 plan schema")
	}
	if err := ValidateClaudeSubscriptionRealisticV38Plan(candidate, workspacePathSet(req.WorkspacePaths), req.AllowedChecks, req.ProviderTurn == 5); err != nil {
		return candidate, nil, err
	}
	reqRaw, _ := json.Marshal(req)
	receipt := ClaudeRealisticV38ProviderReceipt{SchemaVersion: "silver-mediums.claude-realistic-provider-receipt.v38", ProviderCallID: req.ProviderCallID, RequestSHA256: sha256Hex(reqRaw), PromptSHA256: plan.PromptSHA256, OutputSchemaSHA256: plan.SchemaSHA256, Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort, ServingProvider: "anthropic", Transport: d.Transport, RequestJSON: reqRaw, RawClaudeJSON: raw, InputTokens: *result.Usage.InputTokens, OutputTokens: *result.Usage.OutputTokens, CostMicros: *result.Usage.ProviderReportedCostMicros}
	receiptRaw, _ := json.Marshal(receipt)
	return candidate, receiptRaw, nil
}

func ValidateClaudeSubscriptionRealisticV38Plan(plan ClaudeRealisticV37Plan, paths map[string]struct{}, checks []string, final bool) error {
	if len(plan.Actions) < 1 || len(plan.Actions) > 4 {
		return fmt.Errorf("v38 action count")
	}
	finish := 0
	for i, a := range plan.Actions {
		if !contains([]string{"read", "search", "edit", "run_check", "finish"}, a.Kind) {
			return fmt.Errorf("v38 action kind")
		}
		p := filepath.ToSlash(a.Path)
		if a.Kind == "read" {
			if _, ok := paths[p]; !ok {
				return fmt.Errorf("v38 read path")
			}
		}
		if a.Kind == "edit" {
			if _, ok := paths[p]; !ok && p != "HANDOFF.md" {
				return fmt.Errorf("v38 edit path")
			}
		}
		if a.Kind == "run_check" && !contains(checks, a.Check) {
			return fmt.Errorf("v38 check")
		}
		if a.Kind == "finish" {
			finish++
			if i != len(plan.Actions)-1 || strings.TrimSpace(a.Summary) == "" {
				return fmt.Errorf("v38 finish")
			}
		}
	}
	if finish > 1 || (final && finish != 1) {
		return fmt.Errorf("v38 finish policy")
	}
	return nil
}

func ExecuteClaudeSubscriptionRealisticV38Phase(workspace RealisticCaseWorkspace, runFamily string, plan ClaudeRealisticV37Plan, receiptRaw []byte, state ClaudeRealisticV37ExecutionState, checkpoint ClaudeRealisticV37Checkpoint) (ClaudeRealisticV37ExecutionState, error) {
	d, err := ClaudeSubscriptionRealisticV38()
	if err != nil {
		return state, err
	}
	if checkpoint == nil {
		return state, fmt.Errorf("v38 durable checkpoint required")
	}
	phase := state.CompletedPhases + 1
	planHash, receiptHash := sha256Hex(mustJSONV37(plan)), sha256Hex(receiptRaw)
	receipt, replayed, err := ParseClaudeRealisticV38ProviderReceipt(receiptRaw)
	if err != nil || !bytes.Equal(mustJSONV37(replayed), mustJSONV37(plan)) {
		return state, fmt.Errorf("v38 raw plan replay")
	}
	var receiptRequest RealisticProviderRequest
	requestDecoder := json.NewDecoder(bytes.NewReader(receipt.RequestJSON))
	requestDecoder.DisallowUnknownFields()
	if requestDecoder.Decode(&receiptRequest) != nil || requestDecoder.Decode(&struct{}{}) != io.EOF || receiptRequest.RunFamilyID != runFamily {
		return state, fmt.Errorf("v38 receipt request replay")
	}
	if state.PendingPlan == nil {
		transport, req, err := BuildClaudeSubscriptionRealisticV38PhaseRequest(workspace, runFamily, state)
		if err != nil {
			return state, err
		}
		expectedRequest, _ := json.Marshal(req)
		if !bytes.Equal(receipt.RequestJSON, expectedRequest) || receipt.PromptSHA256 != transport.PromptSHA256 || receipt.OutputSchemaSHA256 != transport.SchemaSHA256 {
			return state, fmt.Errorf("v38 provider request is not bound to persisted state")
		}
		admittedActions, actionOK := checkedAddClaudeV37Int(state.TotalActions, len(plan.Actions))
		if !actionOK || admittedActions > d.Caps.MaxTotalActions {
			return state, FailClaudeRealisticV38(ClaudeRealisticV38FailureResource, fmt.Errorf("v38 action resource ceiling"))
		}
		var ok bool
		state.Controller.InputTokens, ok = checkedAddClaudeV36(state.Controller.InputTokens, receipt.InputTokens)
		if !ok {
			return state, FailClaudeRealisticV38(ClaudeRealisticV38FailureEvidence, fmt.Errorf("v38 accounting overflow"))
		}
		state.Controller.OutputTokens, ok = checkedAddClaudeV36(state.Controller.OutputTokens, receipt.OutputTokens)
		if !ok {
			return state, FailClaudeRealisticV38(ClaudeRealisticV38FailureEvidence, fmt.Errorf("v38 accounting overflow"))
		}
		state.Controller.CostMicros, ok = checkedAddClaudeV36(state.Controller.CostMicros, receipt.CostMicros)
		if !ok {
			return state, FailClaudeRealisticV38(ClaudeRealisticV38FailureEvidence, fmt.Errorf("v38 accounting overflow"))
		}
		state.Controller.ProviderCalls, ok = checkedAddClaudeV37Int(state.Controller.ProviderCalls, 1)
		if !ok {
			return state, FailClaudeRealisticV38(ClaudeRealisticV38FailureEvidence, fmt.Errorf("v38 provider call accounting overflow"))
		}
		state.Controller.ProviderAccountingStatus = "exact"
		state.Controller.ProviderTokenAccountingStatus = "exact"
		state.Controller.ProviderCostAccountingStatus = "exact"
		state.Controller.ProviderReceipts = append(state.Controller.ProviderReceipts, append([]byte(nil), receiptRaw...))
		state.Controller.ProviderReceiptHashes = append(state.Controller.ProviderReceiptHashes, receiptHash)
		total, ok := checkedAddClaudeV36(state.Controller.InputTokens, state.Controller.OutputTokens)
		if !ok || state.Controller.ProviderCalls > d.Caps.MaxProviderInvocations || total > d.Caps.MaxTotalReportedTokens || state.Controller.OutputTokens > d.Caps.MaxGeneratedTokens || state.Controller.CostMicros > d.Caps.MaxCostMicros {
			if checkpointErr := checkpoint(state); checkpointErr != nil {
				return state, checkpointErr
			}
			return state, FailClaudeRealisticV38(ClaudeRealisticV38FailureResource, fmt.Errorf("v38 resource ceiling"))
		}
		state.PendingPhase = phase
		state.PendingPlan = &plan
		state.PendingPlanSHA256 = planHash
		state.PendingReceipt = append([]byte(nil), receiptRaw...)
		state.PendingReceiptSHA256 = receiptHash
		state.PendingResultStart = len(state.Controller.ToolResults)
		state.NextAction = 0
		if err := checkpoint(state); err != nil {
			return state, err
		}
	} else {
		phase = state.PendingPhase
		if state.PendingPlanSHA256 != planHash || sha256Hex(mustJSONV37(state.PendingPlan)) != planHash || state.PendingReceiptSHA256 != receiptHash || sha256Hex(state.PendingReceipt) != receiptHash {
			return state, fmt.Errorf("v38 pending phase drift")
		}
		remaining, actionOK := checkedAddClaudeV37Int(state.TotalActions, len(plan.Actions)-state.NextAction)
		if !actionOK || state.NextAction < 0 || state.NextAction > len(plan.Actions) || remaining > d.Caps.MaxTotalActions {
			return state, fmt.Errorf("v38 pending action accounting drift")
		}
	}
	start := state.PendingResultStart
	for state.NextAction < len(plan.Actions) {
		a := plan.Actions[state.NextAction]
		started := time.Now().UTC()
		var out []byte
		var e error
		var touched, args []string
		switch a.Kind {
		case "read":
			out, e = workspace.Read(a.Path)
		case "search":
			var m []string
			m, e = workspace.Search(a.Query)
			out = []byte(strings.Join(m, "\n"))
		case "edit":
			e = workspace.Edit(a.Path, []byte(a.Contents))
			touched = []string{a.Path}
			out = []byte("edited " + a.Path)
		case "run_check":
			out, e = workspace.runCheckWithActionID(a.Check, fmt.Sprintf("%s:attempt-2:phase:%d:action:%d:check:%s", runFamily, phase, state.NextAction+1, a.Check))
			args = strings.Fields(a.Check)
		case "finish":
			state.Controller.TerminalState = "completed"
			state.Controller.FinalSummary = a.Summary
		}
		if a.Kind != "finish" {
			appendRealisticToolResultAt(&state.Controller, a.Kind, out, e, touched, args, started)
		}
		totalActions, actionOK := checkedAddClaudeV37Int(state.TotalActions, 1)
		if !actionOK || totalActions > d.Caps.MaxTotalActions {
			return state, FailClaudeRealisticV38(ClaudeRealisticV38FailureEvidence, fmt.Errorf("v38 action accounting overflow"))
		}
		state.TotalActions = totalActions
		state.NextAction++
		if e != nil {
			state.Controller.TerminalState = "failed"
			state.Controller.FailureClass = "deterministic_action_failure"
		}
		if err := checkpoint(state); err != nil {
			return state, err
		}
		if e != nil {
			return state, nil
		}
	}
	tree, _ := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	results := append([]RealisticToolResult(nil), state.Controller.ToolResults[start:]...)
	rawResults, _ := json.Marshal(results)
	state.Cumulative = append(state.Cumulative, ClaudeRealisticV37CumulativeEntry{Phase: phase, ActionCount: len(plan.Actions), ToolResultCount: len(results), ResultsSHA256: sha256Hex(rawResults), WorkspaceTreeSHA256: tree})
	state.PriorPhaseResults = results
	state.CompletedPhases = phase
	state.PendingPhase = 0
	state.PendingPlan = nil
	state.PendingPlanSHA256 = ""
	state.PendingReceipt = nil
	state.PendingReceiptSHA256 = ""
	state.PendingResultStart = 0
	state.NextAction = 0
	if state.Controller.TerminalState == "completed" {
		total, _ := checkedAddClaudeV36(state.Controller.InputTokens, state.Controller.OutputTokens)
		state.Controller.AttemptHistory = append(state.Controller.AttemptHistory, RealisticAttemptCheckpoint{Attempt: 2, TerminalState: "completed", ProviderCalls: state.Controller.ProviderCalls, TotalTokens: total, CostMicros: state.Controller.CostMicros, WorkspaceTreeSHA256: tree})
	} else {
		state.Controller.TerminalState = "phase_completed"
	}
	return state, nil
}

func parseClaudeRealisticV38Receipt(raw []byte, req RealisticProviderRequest, transport ClaudeScreeningPlan) (ClaudeRealisticV38ProviderReceipt, ClaudeRealisticV37Plan, error) {
	var receipt ClaudeRealisticV38ProviderReceipt
	var empty ClaudeRealisticV37Plan
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&receipt) != nil || dec.Decode(&struct{}{}) != io.EOF {
		return receipt, empty, fmt.Errorf("v38 receipt schema")
	}
	reqRaw, _ := json.Marshal(req)
	if receipt.SchemaVersion != "silver-mediums.claude-realistic-provider-receipt.v38" {
		return receipt, empty, fmt.Errorf("v38 receipt schema binding")
	}
	// encoding/json may normalize escape sequences when a RawMessage is nested
	// inside the paid-phase artifact. Bind the canonical decoded request bytes;
	// the enclosing receipt and paid-phase artifact retain their exact raw hashes.
	if sha256Hex(reqRaw) != receipt.RequestSHA256 || receipt.ProviderCallID != req.ProviderCallID {
		return receipt, empty, fmt.Errorf("v38 receipt request binding")
	}
	if receipt.PromptSHA256 != transport.PromptSHA256 || receipt.OutputSchemaSHA256 != transport.SchemaSHA256 {
		return receipt, empty, fmt.Errorf("v38 receipt transport binding")
	}
	if receipt.Model != "claude-sonnet-4-6" || receipt.Effort != "medium" || receipt.ServingProvider != "anthropic" || receipt.Transport != ClaudeCodeSubscriptionToolFreeTransport {
		return receipt, empty, fmt.Errorf("v38 receipt route binding")
	}
	if receipt.InputTokens < 0 || receipt.OutputTokens < 0 || receipt.CostMicros < 0 {
		return receipt, empty, fmt.Errorf("v38 receipt accounting binding")
	}
	result, candidateRaw, err := ParseClaudeCodeSubscriptionJSONArtifact(receipt.RawClaudeJSON, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil || result.ReportedModel != receipt.Model || result.Usage.InputTokens == nil || *result.Usage.InputTokens != receipt.InputTokens || result.Usage.OutputTokens == nil || *result.Usage.OutputTokens != receipt.OutputTokens || result.Usage.ProviderReportedCostMicros == nil || *result.Usage.ProviderReportedCostMicros != receipt.CostMicros {
		return receipt, empty, fmt.Errorf("v38 receipt accounting replay")
	}
	var candidate ClaudeRealisticV37Plan
	pdec := json.NewDecoder(bytes.NewReader(candidateRaw))
	pdec.DisallowUnknownFields()
	if pdec.Decode(&candidate) != nil || pdec.Decode(&struct{}{}) != io.EOF || ValidateClaudeSubscriptionRealisticV38Plan(candidate, workspacePathSet(req.WorkspacePaths), req.AllowedChecks, req.ProviderTurn == 5) != nil {
		return receipt, empty, fmt.Errorf("v38 candidate replay")
	}
	return receipt, candidate, nil
}

func ParseClaudeRealisticV38ProviderReceipt(raw []byte) (ClaudeRealisticV38ProviderReceipt, ClaudeRealisticV37Plan, error) {
	var receipt ClaudeRealisticV38ProviderReceipt
	var request RealisticProviderRequest
	var empty ClaudeRealisticV37Plan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return receipt, empty, fmt.Errorf("v38 receipt schema")
	}
	requestDecoder := json.NewDecoder(bytes.NewReader(receipt.RequestJSON))
	requestDecoder.DisallowUnknownFields()
	if requestDecoder.Decode(&request) != nil || requestDecoder.Decode(&struct{}{}) != io.EOF {
		return receipt, empty, fmt.Errorf("v38 request schema")
	}
	d, err := ClaudeSubscriptionRealisticV38()
	if err != nil {
		return receipt, empty, err
	}
	if request.CaseID != "R04" || request.CaseVersion != "v1" || request.Attempt != 2 || request.ProviderTurn < 1 || request.ProviderTurn > d.Caps.MaxProviderInvocations || request.MaxSteps != d.Caps.MaxActionsPerPhase || request.ProviderCallID != fmt.Sprintf("%s:attempt-2:phase-%d", request.RunFamilyID, request.ProviderTurn) {
		return receipt, empty, fmt.Errorf("v38 request identity")
	}
	transport, err := buildClaudeSubscriptionRealisticV38Plan(d, request)
	if err != nil {
		return receipt, empty, err
	}
	return parseClaudeRealisticV38Receipt(raw, request, transport)
}
