package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const ClaudeSubscriptionRealisticV36Schema = "silver-mediums.claude-subscription-realistic.v36"

type ClaudeRealisticV36Caps struct {
	MaxProviderTurns        int   `json:"max_provider_turns"`
	MaxTotalReportedTokens  int64 `json:"max_total_reported_tokens"`
	MaxGeneratedTokens      int64 `json:"max_generated_tokens"`
	MaxCostEquivalentMicros int64 `json:"max_cost_equivalent_micros"`
}

type ClaudeRealisticV36Configuration struct {
	ID                string `json:"id"`
	Model             string `json:"model"`
	ReasoningEffort   string `json:"reasoning_effort"`
	RuntimeProvider   string `json:"runtime_provider"`
	ServingProvider   string `json:"serving_provider"`
	ProviderFallbacks bool   `json:"provider_fallbacks"`
}

type ClaudeRealisticV36CasePolicy struct {
	R03PassGatesR04                   bool   `json:"r03_pass_gates_r04"`
	CandidateFailureSkipsR04          bool   `json:"candidate_failure_skips_r04"`
	ObservationLocalInvalidityRunsR04 bool   `json:"observation_local_invalidity_continues_r04"`
	R04Attempt1                       string `json:"r04_attempt_1"`
	R04Attempt2                       string `json:"r04_attempt_2"`
	Retries                           int    `json:"retries"`
	Fallbacks                         int    `json:"fallbacks"`
	OwnerInterventions                int    `json:"owner_interventions"`
}

type ClaudeRealisticV36RuntimeIdentity struct {
	VersionOutput    string `json:"version_output"`
	ExecutablePath   string `json:"executable_path"`
	ExecutableSHA256 string `json:"executable_sha256"`
	RouteEvidence    string `json:"route_evidence"`
}

type ClaudeSubscriptionRealisticV36Definition struct {
	SchemaVersion              string                            `json:"schema_version"`
	BatchID                    string                            `json:"batch_id"`
	Status                     string                            `json:"status"`
	ProviderCallsAllowed       bool                              `json:"provider_calls_allowed"`
	Transport                  string                            `json:"transport"`
	Topology                   string                            `json:"topology"`
	Configuration              ClaudeRealisticV36Configuration   `json:"configuration"`
	ExecutionOrder             []string                          `json:"execution_order"`
	CasePolicy                 ClaudeRealisticV36CasePolicy      `json:"case_policy"`
	CaseCaps                   map[string]ClaudeRealisticV36Caps `json:"case_caps"`
	CampaignCaps               ClaudeRealisticV36Caps            `json:"campaign_caps"`
	RuntimeIdentity            ClaudeRealisticV36RuntimeIdentity `json:"runtime_identity"`
	FrozenAntecedents          map[string]string                 `json:"frozen_antecedents"`
	ValidationReferences       []string                          `json:"validation_references"`
	GlobalHaltReasons          []string                          `json:"global_halt_reasons"`
	CandidateFailureReasons    []string                          `json:"candidate_failure_reasons"`
	ObservationLocalReasons    []string                          `json:"observation_local_invalid_reasons"`
	ProductionAllowed          bool                              `json:"production_allowed"`
	ExternalSideEffectsAllowed bool                              `json:"external_side_effects_allowed"`
	RoleSelectionAllowed       bool                              `json:"role_selection_allowed"`
}

//go:embed claude_subscription_realistic_v36.json
var claudeSubscriptionRealisticV36JSON []byte

func ClaudeSubscriptionRealisticV36() (ClaudeSubscriptionRealisticV36Definition, error) {
	var d ClaudeSubscriptionRealisticV36Definition
	if err := json.Unmarshal(claudeSubscriptionRealisticV36JSON, &d); err != nil {
		return d, err
	}
	if err := ValidateClaudeSubscriptionRealisticV36(d); err != nil {
		return ClaudeSubscriptionRealisticV36Definition{}, err
	}
	return d, nil
}

func ClaudeSubscriptionRealisticV36SHA256() string {
	return sha256Hex(claudeSubscriptionRealisticV36JSON)
}

func RequireClaudeSubscriptionRealisticV36Freeze() error {
	freeze, err := CurrentClaudeSubscriptionRealisticV36Freeze()
	if err != nil {
		return fmt.Errorf("v36 provider calls remain locked: %w", err)
	}
	if !freeze.ProviderCallsAllowed {
		return fmt.Errorf("v36 provider calls remain locked")
	}
	return nil
}

func ValidateClaudeSubscriptionRealisticV36(d ClaudeSubscriptionRealisticV36Definition) error {
	if d.SchemaVersion != ClaudeSubscriptionRealisticV36Schema || d.BatchID != "claude-subscription-realistic-r03-r04-v36" || d.Status != "prepared_for_independent_review" || d.ProviderCallsAllowed {
		return fmt.Errorf("v36 review lock drifted")
	}
	if d.Transport != ClaudeCodeSubscriptionToolFreeTransport || d.Topology != "direct_candidate_to_deterministic_controller_no_reviewer_no_correction" || d.ProductionAllowed || d.ExternalSideEffectsAllowed || d.RoleSelectionAllowed {
		return fmt.Errorf("v36 execution boundary drifted")
	}
	c := d.Configuration
	if c.ID != "realistic.generalist.claude46.medium.subscription.v36" || c.Model != "claude-sonnet-4-6" || c.ReasoningEffort != "medium" || c.RuntimeProvider != "claude_code_subscription" || c.ServingProvider != "anthropic" || c.ProviderFallbacks {
		return fmt.Errorf("v36 configuration drifted")
	}
	if strings.Join(d.ExecutionOrder, ",") != "R03:v2,R04:v1" || !d.CasePolicy.R03PassGatesR04 || !d.CasePolicy.CandidateFailureSkipsR04 || !d.CasePolicy.ObservationLocalInvalidityRunsR04 || d.CasePolicy.Retries != 0 || d.CasePolicy.Fallbacks != 0 || d.CasePolicy.OwnerInterventions != 0 {
		return fmt.Errorf("v36 order or gate policy drifted")
	}
	r03, r04 := d.CaseCaps["R03:v2"], d.CaseCaps["R04:v1"]
	if r03 != (ClaudeRealisticV36Caps{15, 75000, 20000, 200000}) || r04 != (ClaudeRealisticV36Caps{15, 60000, 12000, 250000}) || d.CampaignCaps != (ClaudeRealisticV36Caps{30, 135000, 32000, 450000}) {
		return fmt.Errorf("v36 caps drifted")
	}
	if d.CasePolicy.R04Attempt1 != "harness_forced_interruption_zero_provider_calls" || d.CasePolicy.R04Attempt2 != "exactly_once_resume_same_run_family" {
		return fmt.Errorf("v36 R04 lifecycle drifted")
	}
	if len(d.FrozenAntecedents) != 14 || len(d.ValidationReferences) != 6 || len(d.GlobalHaltReasons) != 4 || len(d.CandidateFailureReasons) != 3 || len(d.ObservationLocalReasons) != 2 {
		return fmt.Errorf("v36 validation or stop policy incomplete")
	}
	if d.RuntimeIdentity.VersionOutput != "2.1.270 (Claude Code)" || d.RuntimeIdentity.ExecutableSHA256 != "7999fba95dbffe167d9e0a043f29057979a0518ebe89b60c4fcfc6401ea8c424" {
		return fmt.Errorf("v36 runtime identity drifted")
	}
	return nil
}

func BuildClaudeSubscriptionRealisticV36ActionPlan(request RealisticProviderRequest) (ClaudeScreeningPlan, error) {
	d, err := ClaudeSubscriptionRealisticV36()
	if err != nil {
		return ClaudeScreeningPlan{}, err
	}
	wantVersion := map[string]string{"R03": "v2", "R04": "v1"}[request.CaseID]
	if wantVersion == "" || request.CaseVersion != wantVersion || request.RunFamilyID == "" || request.Attempt < 1 || request.MaxSteps != 15 {
		return ClaudeScreeningPlan{}, fmt.Errorf("v36 realistic request identity drifted")
	}
	if !exactSet(request.AllowedActions, "read", "search", "edit", "run_check", "browser", "checkpoint_and_interrupt", "finish") {
		return ClaudeScreeningPlan{}, fmt.Errorf("v36 controller action boundary drifted")
	}
	wantChecks := []string{"python -m unittest discover -s tests -v", "local-release-simulator verify", "local-release-simulator rollback"}
	if request.CaseID == "R04" {
		wantChecks = []string{"python resume.py", "python -m unittest discover -s tests -v"}
	}
	if !exactSet(request.AllowedChecks, wantChecks...) {
		return ClaudeScreeningPlan{}, fmt.Errorf("v36 check boundary drifted")
	}
	rawRequest, err := json.Marshal(request)
	if err != nil {
		return ClaudeScreeningPlan{}, err
	}
	schema := []byte(`{"type":"object","additionalProperties":false,"required":["action"],"properties":{"action":{"type":"object","additionalProperties":false,"required":["kind"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","finish"]},"path":{"type":"string"},"query":{"type":"string"},"contents":{"type":"string"},"check":{"type":"string"},"summary":{"type":"string"}}}}}`)
	prompt := "You are the decision source inside a sealed synthetic qualification workspace. Return exactly one JSON action matching the schema. You cannot execute tools: a deterministic controller alone performs one allowlisted synthetic action and returns its real result on the next turn. Use only disclosed paths, checks, task text, and prior results. Never request Browser, deployment, messages, credentials, Phone Farm, host files, arbitrary shell, or external network. Do not invent evidence. Finish only after the bounded local outcome is proven. For R03, rollback restores safety but does not complete the requested successor. For R04, use durable workspace truth, not chat, and never repeat completed work.\n\n<response-schema>\n" + string(schema) + "\n</response-schema>\n\n<controller-request>\n" + string(rawRequest) + "\n</controller-request>\n"
	args := []string{"-p", "--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--no-session-persistence", "--permission-prompts", "none", "--output-format", "json", "--json-schema", "<generated-schema>", "--max-turns", "1", "--model", d.Configuration.Model, "--effort", d.Configuration.ReasoningEffort}
	return ClaudeScreeningPlan{Transport: d.Transport, CandidateID: d.Configuration.ID, CaseID: request.CaseID, CaseVersion: request.CaseVersion, Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort, Prompt: prompt, PromptSHA256: sha256Hex([]byte(prompt)), Schema: schema, SchemaSHA256: sha256Hex(schema), ArgsTemplate: args}, nil
}

type ClaudeRealisticV36ProviderReceipt struct {
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

func ParseClaudeSubscriptionRealisticV36Response(request RealisticProviderRequest, plan ClaudeScreeningPlan, raw []byte, startedAt, completedAt string) (RealisticProviderResponse, error) {
	d, err := ClaudeSubscriptionRealisticV36()
	if err != nil {
		return RealisticProviderResponse{}, err
	}
	if plan.CandidateID != d.Configuration.ID || plan.Model != d.Configuration.Model || plan.Effort != d.Configuration.ReasoningEffort || plan.Transport != d.Transport {
		return RealisticProviderResponse{}, fmt.Errorf("v36 Claude plan identity drifted")
	}
	result, candidateRaw, err := ParseClaudeCodeSubscriptionJSONArtifact(raw, startedAt, completedAt)
	if err != nil {
		return RealisticProviderResponse{}, err
	}
	if result.ReportedModel != d.Configuration.Model || result.Usage.InputTokens == nil || result.Usage.OutputTokens == nil || result.Usage.ProviderReportedCostMicros == nil {
		return RealisticProviderResponse{}, fmt.Errorf("v36 Claude receipt identity or accounting incomplete")
	}
	requestRaw, _ := json.Marshal(request)
	receipt := ClaudeRealisticV36ProviderReceipt{
		SchemaVersion: "silver-mediums.claude-realistic-provider-receipt.v36", ProviderCallID: request.ProviderCallID,
		RequestSHA256: sha256Hex(requestRaw), PromptSHA256: plan.PromptSHA256, OutputSchemaSHA256: plan.SchemaSHA256,
		Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort, ServingProvider: d.Configuration.ServingProvider,
		Transport: d.Transport, RequestJSON: append(json.RawMessage(nil), requestRaw...), RawClaudeJSON: append(json.RawMessage(nil), raw...), InputTokens: *result.Usage.InputTokens,
		OutputTokens: *result.Usage.OutputTokens, CostMicros: *result.Usage.ProviderReportedCostMicros,
	}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		return RealisticProviderResponse{}, err
	}
	response := RealisticProviderResponse{RawReceipt: receiptRaw, RawReceiptSHA256: sha256Hex(receiptRaw), InputTokens: receipt.InputTokens, OutputTokens: receipt.OutputTokens, CostMicros: receipt.CostMicros, AccountingStatus: "exact", TokenAccountingStatus: "exact", CostAccountingStatus: "exact"}
	var candidate struct {
		Action RealisticCandidateAction `json:"action"`
	}
	decoder := json.NewDecoder(bytes.NewReader(candidateRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&candidate); err != nil {
		response.ProtocolFailure = "candidate action schema mismatch"
		return response, nil
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		response.ProtocolFailure = "candidate action has trailing JSON"
		return response, nil
	}
	if !contains([]string{"read", "search", "edit", "run_check", "finish"}, candidate.Action.Kind) {
		response.ProtocolFailure = "candidate action kind is not allowed"
		return response, nil
	}
	response.Action = candidate.Action
	return response, nil
}

func ParseClaudeRealisticV36ProviderReceipt(raw []byte) (ClaudeRealisticV36ProviderReceipt, error) {
	var receipt ClaudeRealisticV36ProviderReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return receipt, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return receipt, fmt.Errorf("v36 provider receipt has trailing JSON")
	}
	if receipt.SchemaVersion != "silver-mediums.claude-realistic-provider-receipt.v36" || receipt.ProviderCallID == "" || !validSHA256(receipt.RequestSHA256) || !validSHA256(receipt.PromptSHA256) || !validSHA256(receipt.OutputSchemaSHA256) || receipt.Model != "claude-sonnet-4-6" || receipt.Effort != "medium" || receipt.ServingProvider != "anthropic" || receipt.Transport != ClaudeCodeSubscriptionToolFreeTransport || !json.Valid(receipt.RequestJSON) || !json.Valid(receipt.RawClaudeJSON) || receipt.InputTokens < 0 || receipt.OutputTokens < 0 || receipt.CostMicros < 0 {
		return receipt, fmt.Errorf("v36 provider receipt is incomplete")
	}
	requestHash := sha256Hex(receipt.RequestJSON)
	var request RealisticProviderRequest
	requestDecoder := json.NewDecoder(bytes.NewReader(receipt.RequestJSON))
	requestDecoder.DisallowUnknownFields()
	if requestHash != receipt.RequestSHA256 || requestDecoder.Decode(&request) != nil || requestDecoder.Decode(&struct{}{}) != io.EOF || request.ProviderCallID != receipt.ProviderCallID {
		return receipt, fmt.Errorf("v36 provider request binding does not replay")
	}
	plan, err := BuildClaudeSubscriptionRealisticV36ActionPlan(request)
	if err != nil || plan.PromptSHA256 != receipt.PromptSHA256 || plan.SchemaSHA256 != receipt.OutputSchemaSHA256 {
		return receipt, fmt.Errorf("v36 provider plan binding does not replay")
	}
	parsed, _, err := ParseClaudeCodeSubscriptionJSONArtifact(receipt.RawClaudeJSON, "2026-09-13T00:00:00Z", "2026-09-13T00:00:01Z")
	if err != nil || parsed.Usage.InputTokens == nil || parsed.Usage.OutputTokens == nil || parsed.Usage.ProviderReportedCostMicros == nil || *parsed.Usage.InputTokens != receipt.InputTokens || *parsed.Usage.OutputTokens != receipt.OutputTokens || *parsed.Usage.ProviderReportedCostMicros != receipt.CostMicros || parsed.ReportedModel != receipt.Model {
		return receipt, fmt.Errorf("v36 provider receipt does not replay")
	}
	return receipt, nil
}
