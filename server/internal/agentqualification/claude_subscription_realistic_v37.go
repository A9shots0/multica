package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ClaudeSubscriptionRealisticV37Schema = "silver-mediums.claude-subscription-realistic.v37"

type ClaudeRealisticV37Caps struct {
	MaxProviderInvocations int   `json:"max_provider_invocations"`
	MaxActionsPerPhase     int   `json:"max_actions_per_phase"`
	MaxTotalActions        int   `json:"max_total_actions"`
	MaxTotalReportedTokens int64 `json:"max_total_reported_tokens"`
	MaxGeneratedTokens     int64 `json:"max_generated_tokens"`
	MaxCostMicros          int64 `json:"max_cost_equivalent_micros"`
	MaxRetries             int   `json:"max_retries"`
}

type ClaudeSubscriptionRealisticV37Definition struct {
	SchemaVersion              string                            `json:"schema_version"`
	BatchID                    string                            `json:"batch_id"`
	Status                     string                            `json:"status"`
	ProviderCallsAllowed       bool                              `json:"provider_calls_allowed"`
	Transport                  string                            `json:"transport"`
	Topology                   string                            `json:"topology"`
	Configuration              ClaudeRealisticV36Configuration   `json:"configuration"`
	ExecutionOrder             []string                          `json:"execution_order"`
	Caps                       ClaudeRealisticV37Caps            `json:"caps"`
	RuntimeIdentity            ClaudeRealisticV36RuntimeIdentity `json:"runtime_identity"`
	FrozenAntecedents          map[string]string                 `json:"frozen_antecedents"`
	ProductionAllowed          bool                              `json:"production_allowed"`
	ExternalSideEffectsAllowed bool                              `json:"external_side_effects_allowed"`
	RoleSelectionAllowed       bool                              `json:"role_selection_allowed"`
}

type ClaudeRealisticV37Plan struct {
	Actions []RealisticCandidateAction `json:"actions"`
}

type ClaudeRealisticV37CumulativeEntry struct {
	Phase               int    `json:"phase"`
	ActionCount         int    `json:"action_count"`
	ToolResultCount     int    `json:"tool_result_count"`
	ResultsSHA256       string `json:"results_sha256"`
	WorkspaceTreeSHA256 string `json:"workspace_tree_sha256"`
}

type ClaudeRealisticV37ExecutionState struct {
	CompletedPhases      int                                 `json:"completed_phases"`
	TotalActions         int                                 `json:"total_actions"`
	PendingPhase         int                                 `json:"pending_phase,omitempty"`
	PendingPlan          *ClaudeRealisticV37Plan             `json:"pending_plan,omitempty"`
	PendingPlanSHA256    string                              `json:"pending_plan_sha256,omitempty"`
	PendingReceipt       []byte                              `json:"pending_receipt,omitempty"`
	PendingReceiptSHA256 string                              `json:"pending_receipt_sha256,omitempty"`
	PendingResultStart   int                                 `json:"pending_result_start,omitempty"`
	NextAction           int                                 `json:"next_action"`
	Cumulative           []ClaudeRealisticV37CumulativeEntry `json:"cumulative"`
	PriorPhaseResults    []RealisticToolResult               `json:"prior_phase_results,omitempty"`
	Controller           RealisticControllerResult           `json:"controller"`
}

type ClaudeRealisticV37Checkpoint func(ClaudeRealisticV37ExecutionState) error

type ClaudeRealisticV37ProviderReceipt struct {
	SchemaVersion      string          `json:"schema_version"`
	ProviderCallID     string          `json:"provider_call_id"`
	RequestSHA256      string          `json:"request_sha256"`
	PromptSHA256       string          `json:"prompt_sha256"`
	OutputSchemaSHA256 string          `json:"output_schema_sha256"`
	Model              string          `json:"model"`
	Effort             string          `json:"effort"`
	ServingProvider    string          `json:"serving_provider"`
	Transport          string          `json:"transport"`
	RequestJSON        json.RawMessage `json:"request_json"`
	RawClaudeJSON      json.RawMessage `json:"raw_claude_json"`
	InputTokens        int64           `json:"input_tokens"`
	OutputTokens       int64           `json:"output_tokens"`
	CostMicros         int64           `json:"cost_equivalent_micros"`
}

//go:embed claude_subscription_realistic_v37.json
var claudeSubscriptionRealisticV37JSON []byte

func ClaudeSubscriptionRealisticV37() (ClaudeSubscriptionRealisticV37Definition, error) {
	var definition ClaudeSubscriptionRealisticV37Definition
	decoder := json.NewDecoder(bytes.NewReader(claudeSubscriptionRealisticV37JSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return definition, err
	}
	if err := ValidateClaudeSubscriptionRealisticV37(definition); err != nil {
		return ClaudeSubscriptionRealisticV37Definition{}, err
	}
	return definition, nil
}

func ClaudeSubscriptionRealisticV37SHA256() string {
	return sha256Hex(claudeSubscriptionRealisticV37JSON)
}

func ValidateClaudeSubscriptionRealisticV37(d ClaudeSubscriptionRealisticV37Definition) error {
	if d.SchemaVersion != ClaudeSubscriptionRealisticV37Schema || d.BatchID != "claude-subscription-realistic-r03-phased-plan-v37" || d.Status != "frozen_reviewed" || !d.ProviderCallsAllowed {
		return fmt.Errorf("v37 reviewed definition drifted")
	}
	if d.Transport != ClaudeCodeSubscriptionToolFreeTransport || d.Topology != "bounded_phased_candidate_plans_to_deterministic_controller_no_reviewer_no_correction" || d.ProductionAllowed || d.ExternalSideEffectsAllowed || d.RoleSelectionAllowed {
		return fmt.Errorf("v37 execution boundary drifted")
	}
	if d.Configuration.ID != "realistic.generalist.claude46.medium.subscription.phased-plan.v37" || d.Configuration.Model != "claude-sonnet-4-6" || d.Configuration.ReasoningEffort != "medium" || d.Configuration.RuntimeProvider != "claude_code_subscription" || d.Configuration.ServingProvider != "anthropic" || d.Configuration.ProviderFallbacks {
		return fmt.Errorf("v37 route drifted")
	}
	if len(d.ExecutionOrder) != 1 || d.ExecutionOrder[0] != "R03:v2" || d.Caps != (ClaudeRealisticV37Caps{5, 4, 20, 75000, 20000, 200000, 0}) {
		return fmt.Errorf("v37 case or caps drifted")
	}
	if d.RuntimeIdentity.VersionOutput != "2.1.270 (Claude Code)" || d.RuntimeIdentity.ExecutableSHA256 != "7999fba95dbffe167d9e0a043f29057979a0518ebe89b60c4fcfc6401ea8c424" || len(d.FrozenAntecedents) != 4 {
		return fmt.Errorf("v37 runtime or antecedent binding drifted")
	}
	return nil
}

func RequireClaudeSubscriptionRealisticV37Freeze() error {
	_, err := CurrentClaudeSubscriptionRealisticV37Freeze()
	return err
}

func BuildClaudeSubscriptionRealisticV37PlanRequest(workspace RealisticCaseWorkspace, runFamilyID string) (ClaudeScreeningPlan, RealisticProviderRequest, error) {
	return BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, runFamilyID, ClaudeRealisticV37ExecutionState{})
}

func BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace RealisticCaseWorkspace, runFamilyID string, state ClaudeRealisticV37ExecutionState) (ClaudeScreeningPlan, RealisticProviderRequest, error) {
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, err
	}
	phase := state.CompletedPhases + 1
	if workspace.CaseID != "R03" || workspace.CaseVersion != "v2" || strings.TrimSpace(runFamilyID) == "" || phase < 1 || phase > d.Caps.MaxProviderInvocations || len(state.Cumulative) != state.CompletedPhases || state.PendingPlan != nil || state.PendingPhase != 0 {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v37 accepts only frozen R03:v2")
	}
	resultOffset, actionTotal := 0, 0
	for index, entry := range state.Cumulative {
		if entry.Phase != index+1 || entry.ActionCount < 1 || entry.ActionCount > d.Caps.MaxActionsPerPhase || entry.ToolResultCount < 0 || entry.ToolResultCount > entry.ActionCount || resultOffset+entry.ToolResultCount > len(state.Controller.ToolResults) || !validSHA256(entry.ResultsSHA256) || !validSHA256(entry.WorkspaceTreeSHA256) {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v37 cumulative ledger invalid")
		}
		phaseResults := state.Controller.ToolResults[resultOffset : resultOffset+entry.ToolResultCount]
		raw, marshalErr := json.Marshal(phaseResults)
		if marshalErr != nil || sha256Hex(raw) != entry.ResultsSHA256 {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v37 cumulative result hash drifted")
		}
		resultOffset += entry.ToolResultCount
		actionTotal += entry.ActionCount
	}
	if resultOffset != len(state.Controller.ToolResults) || actionTotal != state.TotalActions {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v37 cumulative counts drifted")
	}
	currentTree, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return ClaudeScreeningPlan{}, RealisticProviderRequest{}, err
	}
	if phase == 1 {
		if currentTree != workspace.StartingTreeSHA256 || len(state.PriorPhaseResults) != 0 {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v37 starting state drifted")
		}
	} else {
		last := state.Cumulative[len(state.Cumulative)-1]
		priorRaw, marshalErr := json.Marshal(state.PriorPhaseResults)
		if marshalErr != nil || sha256Hex(priorRaw) != last.ResultsSHA256 || currentTree != last.WorkspaceTreeSHA256 {
			return ClaudeScreeningPlan{}, RealisticProviderRequest{}, fmt.Errorf("v37 prior phase or workspace drifted")
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
	ledgerRaw, _ := json.Marshal(state.Cumulative)
	phaseTask := string(task) + "\n\nCumulative completed-phase ledger (hashes only): " + string(ledgerRaw)
	request := RealisticProviderRequest{ProviderCallID: fmt.Sprintf("%s:attempt-1:phase-%d", runFamilyID, phase), ProviderTurn: phase, CaseID: "R03", CaseVersion: "v2", RunFamilyID: runFamilyID, Attempt: 1, Task: phaseTask, WorkspacePaths: paths, PreviousResults: append([]RealisticToolResult(nil), state.PriorPhaseResults...), AllowedActions: []string{"read", "search", "edit", "run_check", "finish"}, AllowedChecks: append([]string(nil), workspace.Fixture.AllowlistedChecks...), MaxSteps: d.Caps.MaxActionsPerPhase}
	plan, err := buildClaudeSubscriptionRealisticV37Plan(d, request)
	return plan, request, err
}

func buildClaudeSubscriptionRealisticV37Plan(d ClaudeSubscriptionRealisticV37Definition, request RealisticProviderRequest) (ClaudeScreeningPlan, error) {
	requestRaw, err := json.Marshal(request)
	if err != nil {
		return ClaudeScreeningPlan{}, err
	}
	schema := []byte(`{"type":"object","additionalProperties":false,"required":["actions"],"properties":{"actions":{"type":"array","minItems":1,"maxItems":4,"items":{"type":"object","additionalProperties":false,"required":["kind"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","finish"]},"path":{"type":"string"},"query":{"type":"string"},"contents":{"type":"string"},"check":{"type":"string"},"summary":{"type":"string"}}}}}}`)
	prompt := "Return the next bounded phase for sealed synthetic R03. You have no tools and perform no actions. The deterministic controller executes at most four actions exactly once in order. You receive the compact hash-only ledger for all older phases and full tool outputs only from the immediately prior phase. A finish action, when used, must be final and appear once; otherwise omit finish and the controller may request another paid phase, up to five total. Do not repeat completed work. Do not request Browser, shell, network, deployment, messages, credentials, Phone Farm, external effects, or undisclosed paths. Complete the bounded outcome: restore safety, prevent regression, verify current and historical journeys, preserve truthful evidence, and leave the successor outcome open.\n\n<response-schema>\n" + string(schema) + "\n</response-schema>\n\n<workspace-request>\n" + string(requestRaw) + "\n</workspace-request>\n"
	args := []string{"-p", "--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--no-session-persistence", "--permission-prompts", "none", "--output-format", "json", "--json-schema", "<generated-schema>", "--max-turns", "1", "--model", d.Configuration.Model, "--effort", d.Configuration.ReasoningEffort}
	return ClaudeScreeningPlan{Transport: d.Transport, CandidateID: d.Configuration.ID, CaseID: "R03", CaseVersion: "v2", Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)), Schema: schema, SchemaSHA256: sha256Hex(schema), ArgsTemplate: args}, nil
}

func ParseClaudeSubscriptionRealisticV37Response(request RealisticProviderRequest, plan ClaudeScreeningPlan, raw []byte, startedAt, completedAt string) (ClaudeRealisticV37Plan, []byte, error) {
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		return ClaudeRealisticV37Plan{}, nil, err
	}
	if plan.CandidateID != d.Configuration.ID || plan.Model != d.Configuration.Model || plan.Effort != d.Configuration.ReasoningEffort || plan.Transport != d.Transport || request.CaseID != "R03" || request.CaseVersion != "v2" || request.ProviderTurn < 1 || request.ProviderTurn > d.Caps.MaxProviderInvocations || request.MaxSteps != d.Caps.MaxActionsPerPhase {
		return ClaudeRealisticV37Plan{}, nil, fmt.Errorf("v37 plan/request identity drifted")
	}
	result, candidateRaw, err := ParseClaudeCodeSubscriptionJSONArtifact(raw, startedAt, completedAt)
	if err != nil {
		return ClaudeRealisticV37Plan{}, nil, err
	}
	if result.ReportedModel != d.Configuration.Model || result.Usage.InputTokens == nil || result.Usage.OutputTokens == nil || result.Usage.ProviderReportedCostMicros == nil || *result.Usage.InputTokens < 0 || *result.Usage.OutputTokens < 0 || *result.Usage.ProviderReportedCostMicros < 0 {
		return ClaudeRealisticV37Plan{}, nil, fmt.Errorf("v37 provider identity/accounting incomplete")
	}
	requestRaw, _ := json.Marshal(request)
	receipt := ClaudeRealisticV37ProviderReceipt{SchemaVersion: "silver-mediums.claude-realistic-provider-receipt.v37", ProviderCallID: request.ProviderCallID, RequestSHA256: sha256Hex(requestRaw), PromptSHA256: plan.PromptSHA256, OutputSchemaSHA256: plan.SchemaSHA256, Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort, ServingProvider: d.Configuration.ServingProvider, Transport: d.Transport, RequestJSON: requestRaw, RawClaudeJSON: append(json.RawMessage(nil), raw...), InputTokens: *result.Usage.InputTokens, OutputTokens: *result.Usage.OutputTokens, CostMicros: *result.Usage.ProviderReportedCostMicros}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		return ClaudeRealisticV37Plan{}, nil, err
	}
	var candidate ClaudeRealisticV37Plan
	decoder := json.NewDecoder(bytes.NewReader(candidateRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&candidate); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return ClaudeRealisticV37Plan{}, receiptRaw, fmt.Errorf("v37 plan schema mismatch")
	}
	if err := ValidateClaudeSubscriptionRealisticV37Plan(candidate, workspacePathSet(request.WorkspacePaths), request.AllowedChecks, d.Caps.MaxActionsPerPhase, request.ProviderTurn == d.Caps.MaxProviderInvocations); err != nil {
		return ClaudeRealisticV37Plan{}, receiptRaw, err
	}
	total, ok := checkedAddClaudeV36(*result.Usage.InputTokens, *result.Usage.OutputTokens)
	if !ok || total > d.Caps.MaxTotalReportedTokens || *result.Usage.OutputTokens > d.Caps.MaxGeneratedTokens || *result.Usage.ProviderReportedCostMicros > d.Caps.MaxCostMicros {
		return ClaudeRealisticV37Plan{}, receiptRaw, fmt.Errorf("v37 provider resource ceiling exceeded")
	}
	return candidate, receiptRaw, nil
}

func ValidateClaudeSubscriptionRealisticV37Plan(plan ClaudeRealisticV37Plan, paths map[string]struct{}, allowedChecks []string, maxActions int, finalPhase bool) error {
	if len(plan.Actions) < 1 || len(plan.Actions) > maxActions {
		return fmt.Errorf("v37 action count invalid")
	}
	finishCount := 0
	for index, action := range plan.Actions {
		if !contains([]string{"read", "search", "edit", "run_check", "finish"}, action.Kind) {
			return fmt.Errorf("v37 action kind not allowed")
		}
		switch action.Kind {
		case "read", "edit":
			normalized := filepath.ToSlash(action.Path)
			_, exists := paths[normalized]
			if action.Kind == "read" && !exists {
				return fmt.Errorf("v37 read path does not exist: %s", action.Path)
			}
			if action.Kind == "edit" && !exists && normalized != "release/receipt.json" && normalized != "EVIDENCE.md" {
				return fmt.Errorf("v37 creation path not allowed: %s", action.Path)
			}
		case "run_check":
			if !contains(allowedChecks, action.Check) {
				return fmt.Errorf("v37 check not allowlisted")
			}
		case "finish":
			finishCount++
			if index != len(plan.Actions)-1 || strings.TrimSpace(action.Summary) == "" {
				return fmt.Errorf("v37 finish must be final and meaningful")
			}
		}
	}
	if finishCount > 1 || (finalPhase && finishCount != 1) {
		return fmt.Errorf("v37 finish policy invalid")
	}
	return nil
}

func ExecuteClaudeSubscriptionRealisticV37Plan(workspace RealisticCaseWorkspace, runFamilyID string, plan ClaudeRealisticV37Plan, providerReceipt []byte) (RealisticControllerResult, error) {
	state, err := ExecuteClaudeSubscriptionRealisticV37Phase(workspace, runFamilyID, 1, plan, providerReceipt, ClaudeRealisticV37ExecutionState{}, func(ClaudeRealisticV37ExecutionState) error { return nil })
	if err != nil {
		return state.Controller, err
	}
	if state.Controller.TerminalState != "completed" {
		return state.Controller, fmt.Errorf("v37 single phase did not finish")
	}
	return state.Controller, nil
}

// ExecuteClaudeSubscriptionRealisticV37Phase never invokes a provider. The
// paid phase plan and receipt are durably checkpointed before action zero. A
// restart supplies the same state, plan, and receipt and resumes at NextAction
// without spending another provider invocation.
func ExecuteClaudeSubscriptionRealisticV37Phase(workspace RealisticCaseWorkspace, runFamilyID string, phase int, plan ClaudeRealisticV37Plan, providerReceipt []byte, state ClaudeRealisticV37ExecutionState, checkpoint ClaudeRealisticV37Checkpoint) (ClaudeRealisticV37ExecutionState, error) {
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		return state, err
	}
	if checkpoint == nil || phase < 1 || phase > d.Caps.MaxProviderInvocations || workspace.CaseID != "R03" || workspace.CaseVersion != "v2" || runFamilyID == "" {
		return state, fmt.Errorf("v37 phase identity invalid")
	}
	paths, err := workspace.Paths()
	if err != nil {
		return state, err
	}
	if err := ValidateClaudeSubscriptionRealisticV37Plan(plan, workspacePathSet(paths), workspace.Fixture.AllowlistedChecks, d.Caps.MaxActionsPerPhase, phase == d.Caps.MaxProviderInvocations); err != nil {
		return state, err
	}
	receipt, receiptPlan, err := parseClaudeRealisticV37ProviderReceiptAndPlan(providerReceipt)
	if err != nil {
		return state, err
	}
	receiptPlanRaw, _ := json.Marshal(receiptPlan)
	var request RealisticProviderRequest
	if json.Unmarshal(receipt.RequestJSON, &request) != nil || request.ProviderCallID != receipt.ProviderCallID || request.ProviderTurn != phase || request.RunFamilyID != runFamilyID || request.CaseID != "R03" || request.CaseVersion != "v2" || !bytes.Equal(receiptPlanRaw, mustJSONV37(plan)) {
		return state, fmt.Errorf("v37 phase receipt/request mismatch")
	}
	if state.PendingPlan == nil {
		expectedTransport, expectedRequest, expectedErr := BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(workspace, runFamilyID, state)
		expectedRaw, _ := json.Marshal(expectedRequest)
		if expectedErr != nil || !bytes.Equal(expectedRaw, receipt.RequestJSON) || expectedTransport.PromptSHA256 != receipt.PromptSHA256 || expectedTransport.SchemaSHA256 != receipt.OutputSchemaSHA256 {
			return state, fmt.Errorf("v37 provider request is not bound to persisted state")
		}
	}
	planRaw, _ := json.Marshal(plan)
	planHash, receiptHash := sha256Hex(planRaw), sha256Hex(providerReceipt)
	if state.PendingPlan == nil {
		totalActions, actionsOK := checkedAddClaudeV37Int(state.TotalActions, len(plan.Actions))
		if !actionsOK || phase != state.CompletedPhases+1 || state.Controller.TerminalState == "completed" || totalActions > d.Caps.MaxTotalActions {
			return state, fmt.Errorf("v37 phase admission invalid")
		}
		if state.Controller.CaseID == "" {
			state.Controller = RealisticControllerResult{CaseID: "R03", RunFamilyID: runFamilyID, Attempt: 1, ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact"}
		}
		var ok bool
		state.Controller.InputTokens, ok = checkedAddClaudeV36(state.Controller.InputTokens, receipt.InputTokens)
		if !ok {
			return state, fmt.Errorf("v37 input accounting overflow")
		}
		state.Controller.OutputTokens, ok = checkedAddClaudeV36(state.Controller.OutputTokens, receipt.OutputTokens)
		if !ok {
			return state, fmt.Errorf("v37 output accounting overflow")
		}
		state.Controller.CostMicros, ok = checkedAddClaudeV36(state.Controller.CostMicros, receipt.CostMicros)
		if !ok {
			return state, fmt.Errorf("v37 cost accounting overflow")
		}
		providerCalls, callsOK := checkedAddClaudeV37Int(state.Controller.ProviderCalls, 1)
		if !callsOK {
			return state, fmt.Errorf("v37 provider call accounting overflow")
		}
		state.Controller.ProviderCalls = providerCalls
		state.Controller.ProviderReceipts = append(state.Controller.ProviderReceipts, append([]byte(nil), providerReceipt...))
		state.Controller.ProviderReceiptHashes = append(state.Controller.ProviderReceiptHashes, receiptHash)
		total, totalOK := checkedAddClaudeV36(state.Controller.InputTokens, state.Controller.OutputTokens)
		if !totalOK || state.Controller.ProviderCalls > d.Caps.MaxProviderInvocations || total > d.Caps.MaxTotalReportedTokens || state.Controller.OutputTokens > d.Caps.MaxGeneratedTokens || state.Controller.CostMicros > d.Caps.MaxCostMicros {
			return state, fmt.Errorf("v37 cumulative resource ceiling exceeded")
		}
		state.PendingPhase, state.PendingPlan, state.PendingPlanSHA256 = phase, &plan, planHash
		state.PendingReceipt, state.PendingReceiptSHA256 = append([]byte(nil), providerReceipt...), receiptHash
		state.PendingResultStart, state.NextAction = len(state.Controller.ToolResults), 0
		if err := checkpoint(state); err != nil {
			return state, err
		}
	} else {
		storedPlanRaw, marshalErr := json.Marshal(state.PendingPlan)
		if marshalErr != nil || state.PendingPhase != phase || state.PendingPlanSHA256 != planHash || sha256Hex(storedPlanRaw) != planHash || state.PendingReceiptSHA256 != receiptHash || sha256Hex(state.PendingReceipt) != receiptHash || state.CompletedPhases != phase-1 {
			return state, fmt.Errorf("v37 pending paid phase drifted")
		}
	}
	for state.NextAction < len(plan.Actions) {
		action := plan.Actions[state.NextAction]
		if action.Kind == "finish" {
			state.Controller.TerminalState, state.Controller.FinalSummary = "completed", action.Summary
			state.NextAction++
			state.TotalActions++
			if err := checkpoint(state); err != nil {
				return state, err
			}
			continue
		}
		started := time.Now().UTC()
		var output []byte
		var actionErr error
		var touched, arguments []string
		switch action.Kind {
		case "read":
			output, actionErr = workspace.Read(action.Path)
		case "search":
			var matches []string
			matches, actionErr = workspace.Search(action.Query)
			output = []byte(strings.Join(matches, "\n"))
		case "edit":
			actionErr = workspace.Edit(action.Path, []byte(action.Contents))
			touched, output = []string{action.Path}, []byte("edited "+action.Path)
		case "run_check":
			actionID := fmt.Sprintf("%s:phase:%d:action:%d:check:%s", runFamilyID, phase, state.NextAction+1, action.Check)
			output, actionErr = workspace.runCheckWithActionID(action.Check, actionID)
			arguments = strings.Fields(action.Check)
		}
		appendRealisticToolResultAt(&state.Controller, action.Kind, output, actionErr, touched, arguments, started)
		state.NextAction++
		state.TotalActions++
		if err := checkpoint(state); err != nil {
			return state, err
		}
		if actionErr != nil {
			state.Controller.TerminalState, state.Controller.FailureClass = "failed", "deterministic_action_failure"
			return state, nil
		}
	}
	finalHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return state, err
	}
	phaseResults := append([]RealisticToolResult(nil), state.Controller.ToolResults[state.PendingResultStart:]...)
	resultsRaw, _ := json.Marshal(phaseResults)
	state.Cumulative = append(state.Cumulative, ClaudeRealisticV37CumulativeEntry{Phase: phase, ActionCount: len(plan.Actions), ToolResultCount: len(phaseResults), ResultsSHA256: sha256Hex(resultsRaw), WorkspaceTreeSHA256: finalHash})
	state.PriorPhaseResults = phaseResults
	state.CompletedPhases = phase
	state.PendingPhase, state.PendingPlan, state.PendingPlanSHA256 = 0, nil, ""
	state.PendingReceipt, state.PendingReceiptSHA256 = nil, ""
	state.PendingResultStart, state.NextAction = 0, 0
	if state.Controller.TerminalState == "completed" {
		total, ok := checkedAddClaudeV36(state.Controller.InputTokens, state.Controller.OutputTokens)
		if !ok {
			return state, fmt.Errorf("v37 attempt accounting overflow")
		}
		state.Controller.AttemptHistory = []RealisticAttemptCheckpoint{{Attempt: 1, TerminalState: "completed", ProviderCalls: state.Controller.ProviderCalls, TotalTokens: total, CostMicros: state.Controller.CostMicros, WorkspaceTreeSHA256: finalHash}}
	} else {
		state.Controller.TerminalState = "phase_completed"
	}
	if err := checkpoint(state); err != nil {
		return state, err
	}
	return state, nil
}

func ParseClaudeRealisticV37ProviderReceipt(raw []byte) (ClaudeRealisticV37ProviderReceipt, error) {
	receipt, _, err := parseClaudeRealisticV37ProviderReceiptAndPlan(raw)
	return receipt, err
}

func parseClaudeRealisticV37ProviderReceiptAndPlan(raw []byte) (ClaudeRealisticV37ProviderReceipt, ClaudeRealisticV37Plan, error) {
	var receipt ClaudeRealisticV37ProviderReceipt
	var candidate ClaudeRealisticV37Plan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return receipt, candidate, fmt.Errorf("v37 provider receipt invalid")
	}
	if receipt.SchemaVersion != "silver-mediums.claude-realistic-provider-receipt.v37" || receipt.ProviderCallID == "" || !json.Valid(receipt.RequestJSON) || sha256Hex(receipt.RequestJSON) != receipt.RequestSHA256 || !json.Valid(receipt.RawClaudeJSON) || receipt.Model != "claude-sonnet-4-6" || receipt.Effort != "medium" || receipt.ServingProvider != "anthropic" || receipt.Transport != ClaudeCodeSubscriptionToolFreeTransport || receipt.InputTokens < 0 || receipt.OutputTokens < 0 || receipt.CostMicros < 0 {
		return receipt, candidate, fmt.Errorf("v37 provider receipt binding invalid")
	}
	var request RealisticProviderRequest
	requestDecoder := json.NewDecoder(bytes.NewReader(receipt.RequestJSON))
	requestDecoder.DisallowUnknownFields()
	if requestDecoder.Decode(&request) != nil || requestDecoder.Decode(&struct{}{}) != io.EOF || request.ProviderCallID != receipt.ProviderCallID || request.CaseID != "R03" || request.CaseVersion != "v2" || request.ProviderTurn < 1 || request.ProviderTurn > 5 || request.Attempt != 1 || request.MaxSteps != 4 {
		return receipt, candidate, fmt.Errorf("v37 provider request does not replay")
	}
	d, err := ClaudeSubscriptionRealisticV37()
	if err != nil {
		return receipt, candidate, err
	}
	plan, err := buildClaudeSubscriptionRealisticV37Plan(d, request)
	if err != nil || plan.PromptSHA256 != receipt.PromptSHA256 || plan.SchemaSHA256 != receipt.OutputSchemaSHA256 {
		return receipt, candidate, fmt.Errorf("v37 provider plan does not replay")
	}
	parsed, candidateRaw, err := ParseClaudeCodeSubscriptionJSONArtifact(receipt.RawClaudeJSON, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil || parsed.ReportedModel != receipt.Model || parsed.Usage.InputTokens == nil || parsed.Usage.OutputTokens == nil || parsed.Usage.ProviderReportedCostMicros == nil || *parsed.Usage.InputTokens != receipt.InputTokens || *parsed.Usage.OutputTokens != receipt.OutputTokens || *parsed.Usage.ProviderReportedCostMicros != receipt.CostMicros {
		return receipt, candidate, fmt.Errorf("v37 provider accounting does not replay")
	}
	planDecoder := json.NewDecoder(bytes.NewReader(candidateRaw))
	planDecoder.DisallowUnknownFields()
	if planDecoder.Decode(&candidate) != nil || planDecoder.Decode(&struct{}{}) != io.EOF || ValidateClaudeSubscriptionRealisticV37Plan(candidate, workspacePathSet(request.WorkspacePaths), request.AllowedChecks, request.MaxSteps, request.ProviderTurn == 5) != nil {
		return receipt, ClaudeRealisticV37Plan{}, fmt.Errorf("v37 raw candidate plan does not replay")
	}
	return receipt, candidate, nil
}

func mustJSONV37(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func workspacePathSet(paths []string) map[string]struct{} {
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		result[filepath.ToSlash(path)] = struct{}{}
	}
	return result
}

func checkedAddClaudeV37Int(left, right int) (int, bool) {
	if right > 0 && left > int(^uint(0)>>1)-right {
		return 0, false
	}
	if right < 0 && left < -int(^uint(0)>>1)-1-right {
		return 0, false
	}
	return left + right, true
}
