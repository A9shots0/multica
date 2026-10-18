package agentqualification

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

const ProtocolAblationV32Schema = "silver-mediums.protocol-ablation.v32"

type ProtocolAblationV32Case struct {
	CaseID           string   `json:"case_id"`
	CaseVersion      string   `json:"case_version"`
	SourceCaseID     string   `json:"source_case_id"`
	SourceFixtureID  string   `json:"source_fixture_id"`
	BusinessFacts    string   `json:"business_facts"`
	AcceptanceChecks []string `json:"acceptance_checks"`
	Grader           string   `json:"grader"`
}

type ProtocolAblationV32Candidate struct {
	RealisticCandidate
	InputMicrosPerMillion     int64 `json:"input_micros_per_million"`
	GeneratedMicrosPerMillion int64 `json:"generated_micros_per_million"`
}

type ProtocolAblationV32Protocol struct {
	ProtocolID            string `json:"protocol_id"`
	ResponseInterface     string `json:"response_interface"`
	MaxProviderTurns      int    `json:"max_provider_turns"`
	MaxActionsPerResponse int    `json:"max_actions_per_response"`
}

type ProtocolAblationV32Definition struct {
	SchemaVersion                       string                         `json:"schema_version"`
	ExperimentID                        string                         `json:"experiment_id"`
	Status                              string                         `json:"status"`
	ProviderCallsAllowed                bool                           `json:"provider_calls_allowed"`
	Case                                ProtocolAblationV32Case        `json:"case"`
	Candidates                          []ProtocolAblationV32Candidate `json:"candidates"`
	Protocols                           []ProtocolAblationV32Protocol  `json:"protocols"`
	Transport                           string                         `json:"transport"`
	ProviderFallbacks                   bool                           `json:"provider_fallbacks"`
	Retries                             int                            `json:"retries"`
	PerObservationTotalTokenReservation int64                          `json:"per_observation_total_token_reservation"`
	PerObservationGeneratedTokenCeiling int64                          `json:"per_observation_generated_token_ceiling"`
	MaxCombinedTokens                   int64                          `json:"max_combined_tokens"`
	MaxCombinedCostMicros               int64                          `json:"max_combined_cost_micros"`
	MaxCombinedProviderTurns            int                            `json:"max_combined_provider_turns"`
	MeasurementBoundary                 string                         `json:"measurement_boundary"`
	StopPolicy                          string                         `json:"stop_policy"`
	AdvancementPolicy                   string                         `json:"advancement_policy"`
}

type ProtocolAblationV32Binding struct {
	BindingID string                       `json:"binding_id"`
	Candidate ProtocolAblationV32Candidate `json:"candidate"`
	Protocol  ProtocolAblationV32Protocol  `json:"protocol"`
}

type ProtocolAblationV32Usage struct {
	ProviderTurns   int   `json:"provider_turns"`
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	CostMicros      int64 `json:"cost_micros"`
}

type ProtocolAblationV32Observation struct {
	BindingID        string                              `json:"binding_id"`
	Outcome          string                              `json:"outcome"`
	FailureClass     string                              `json:"failure_class,omitempty"`
	FailureScope     string                              `json:"failure_scope,omitempty"`
	Usage            ProtocolAblationV32Usage            `json:"usage"`
	ProviderIdentity ProtocolAblationV32ProviderIdentity `json:"provider_identity"`
	RawReceipts      []json.RawMessage                   `json:"raw_receipts"`
	Actions          []RealisticCandidateAction          `json:"actions"`
	GraderResult     RealisticNextCalibrationResult      `json:"grader_result"`
	EvidenceSHA256   string                              `json:"evidence_sha256"`
}

type ProtocolAblationV32ProviderIdentity struct {
	DirectProvider          string `json:"direct_provider"`
	ReportedModel           string `json:"reported_model"`
	ReportedEffort          string `json:"reported_effort"`
	ReportedServingProvider string `json:"reported_serving_provider"`
	ResolvedConfigSHA256    string `json:"resolved_config_sha256"`
	ServingProviderEvidence string `json:"serving_provider_evidence"`
}

type ProtocolAblationV32InitialPacket struct {
	CaseID             string             `json:"case_id"`
	CaseVersion        string             `json:"case_version"`
	Task               string             `json:"task"`
	FixtureFiles       []R05V1FixtureFile `json:"fixture_files"`
	EditablePaths      []string           `json:"editable_paths"`
	ImmutablePaths     []string           `json:"immutable_paths"`
	AllowedActions     []string           `json:"allowed_actions"`
	AllowedChecks      []string           `json:"allowed_checks"`
	AcceptanceBoundary string             `json:"acceptance_boundary"`
	FixtureTreeSHA256  string             `json:"fixture_tree_sha256"`
	Grader             string             `json:"grader"`
}

type ProtocolAblationV32Claim struct {
	BindingID string                          `json:"binding_id"`
	Status    string                          `json:"status"`
	Completed *ProtocolAblationV32Observation `json:"completed,omitempty"`
}

type ProtocolAblationV32Ledger struct {
	Claims     map[string]ProtocolAblationV32Claim `json:"claims"`
	Usage      ProtocolAblationV32Usage            `json:"usage"`
	GlobalHalt bool                                `json:"global_halt"`
	HaltReason string                              `json:"halt_reason,omitempty"`
}

//go:embed protocol_ablation_v32.json
var protocolAblationV32JSON []byte

//go:embed protocol_ablation_v32.go
var protocolAblationV32Source []byte

//go:embed protocol_ablation_v32_test.go
var protocolAblationV32TestSource []byte

func ProtocolAblationV32SourceHashes() map[string]string {
	return map[string]string{
		"protocol_ablation_v32.json":    hashRealisticNextBytes(protocolAblationV32JSON),
		"protocol_ablation_v32.go":      hashRealisticNextBytes(protocolAblationV32Source),
		"protocol_ablation_v32_test.go": hashRealisticNextBytes(protocolAblationV32TestSource),
	}
}

func CurrentProtocolAblationV32() (ProtocolAblationV32Definition, error) {
	var definition ProtocolAblationV32Definition
	if err := json.Unmarshal(protocolAblationV32JSON, &definition); err != nil {
		return definition, err
	}
	return definition, ValidateProtocolAblationV32(definition)
}

func ValidateProtocolAblationV32(d ProtocolAblationV32Definition) error {
	validLock := (d.Status == "draft_prevalidation" && !d.ProviderCallsAllowed) || (d.Status == "frozen_reviewed" && d.ProviderCallsAllowed)
	if d.SchemaVersion != ProtocolAblationV32Schema || d.ExperimentID != "realistic-r05-protocol-ablation-v32" || !validLock {
		return fmt.Errorf("v32 identity or pre-freeze lock drifted")
	}
	fixture, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		return err
	}
	wantCase := ProtocolAblationV32Case{CaseID: "P32", CaseVersion: "v1", SourceCaseID: "R05", SourceFixtureID: fixture.FixtureID, BusinessFacts: "identical_to_frozen_R05_v1", AcceptanceChecks: append([]string(nil), fixture.AllowlistedChecks...), Grader: "EvaluateRealisticNextR05"}
	if !reflect.DeepEqual(d.Case, wantCase) {
		return fmt.Errorf("v32 case no longer binds the unchanged R05 facts and grader")
	}
	freeze, err := CurrentRealisticNextFreeze()
	if err != nil || freeze.LocalComponents["realistic_r05_fixture_v1.json"] != "406c40e75d2a39cecc816b3f4506234c3720161aa9cb1e6c56e214b6a1eb721f" || freeze.LocalComponents["realistic_r05_fixture_v1.go"] != "ae6dfa595bb84d4c62ffb9ac510fe80efc6ef1c4a3dbd1acb9622b2f50173c31" || freeze.LocalComponents["realistic_next_calibration_v1.go"] != "1454ccbf164d8152779ecc2768e46132f42df53dee7e36e1c099d777e87bf07a" {
		return fmt.Errorf("v32 R05 fixture or grader source is not hash-bound to the reviewed freeze")
	}
	if len(d.Candidates) != 2 || d.Candidates[0].ConfigurationID != "protocol-v32.claudesonnet5.medium" || d.Candidates[0].Model != "openrouter/anthropic/claude-sonnet-5" || d.Candidates[0].Effort != "medium" || d.Candidates[0].ServingProvider != "anthropic" || d.Candidates[0].InputMicrosPerMillion != 3_000_000 || d.Candidates[0].GeneratedMicrosPerMillion != 15_000_000 || d.Candidates[1].ConfigurationID != "protocol-v32.gpt54.medium" || d.Candidates[1].Model != "openrouter/openai/gpt-5.4" || d.Candidates[1].Effort != "medium" || d.Candidates[1].ServingProvider != "openai" || d.Candidates[1].InputMicrosPerMillion != 2_500_000 || d.Candidates[1].GeneratedMicrosPerMillion != 15_000_000 {
		return fmt.Errorf("v32 exact candidate routing or pinned prices drifted")
	}
	if len(d.Protocols) != 2 || d.Protocols[0] != (ProtocolAblationV32Protocol{"one-action-loop-v1", "single_bounded_action", 10, 1}) || d.Protocols[1] != (ProtocolAblationV32Protocol{"one-shot-plan-v1", "one_shot_structured_bounded_action_plan", 1, 10}) {
		return fmt.Errorf("v32 protocol arms drifted")
	}
	if d.Transport != "isolated_opencode_openrouter" || d.ProviderFallbacks || d.Retries != 0 || d.PerObservationTotalTokenReservation != 75_000 || d.PerObservationGeneratedTokenCeiling != 20_000 || d.MaxCombinedTokens != 300_000 || d.MaxCombinedCostMicros != 1_805_000 || d.MaxCombinedProviderTurns != 22 {
		return fmt.Errorf("v32 transport or combined accounting cap drifted")
	}
	if d.MeasurementBoundary != "disposable_synthetic_workspace_only_no_browser_no_production_no_outbound_no_owner_no_phone_farm" || d.StopPolicy != "candidate_competence_or_protocol_failure_is_observation_local; global_halt_only_for_invalid_test_unsafe_execution_credential_failure_budget_breach_or_evidence_or_transport_corruption" || d.AdvancementPolicy != "protocol_evidence_only_no_role_selection_or_production_promotion" {
		return fmt.Errorf("v32 safety or stop boundary drifted")
	}
	for _, candidate := range d.Candidates {
		for _, protocol := range d.Protocols {
			if ProtocolAblationV32CostReservation(d, candidate) <= 0 {
				return fmt.Errorf("v32 cost reservation is invalid")
			}
			_ = protocol
		}
	}
	return nil
}

func ProtocolAblationV32Bindings(d ProtocolAblationV32Definition) []ProtocolAblationV32Binding {
	bindings := make([]ProtocolAblationV32Binding, 0, len(d.Candidates)*len(d.Protocols))
	for _, candidate := range d.Candidates {
		for _, protocol := range d.Protocols {
			bindings = append(bindings, ProtocolAblationV32Binding{BindingID: candidate.ConfigurationID + "." + protocol.ProtocolID, Candidate: candidate, Protocol: protocol})
		}
	}
	return bindings
}

func ProtocolAblationV32CostReservation(d ProtocolAblationV32Definition, candidate ProtocolAblationV32Candidate) int64 {
	generated := d.PerObservationGeneratedTokenCeiling
	input := d.PerObservationTotalTokenReservation - generated
	n := input*candidate.InputMicrosPerMillion + generated*candidate.GeneratedMicrosPerMillion
	return (n + 999_999) / 1_000_000
}

func EvaluateProtocolAblationV32(startRoot, finalRoot string, evidence *R05V1HarnessEvidence) RealisticNextCalibrationResult {
	result := EvaluateRealisticNextR05(startRoot, finalRoot, evidence)
	result.CaseID = "P32"
	return result
}

func ProtocolAblationV32EvidenceSHA(result RealisticNextCalibrationResult) string {
	raw, _ := json.Marshal(result)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func BeginProtocolAblationV32Observation(d ProtocolAblationV32Definition, ledger *ProtocolAblationV32Ledger, binding ProtocolAblationV32Binding) (string, *ProtocolAblationV32Observation, error) {
	if err := ValidateProtocolAblationV32(d); err != nil {
		return "halt", nil, err
	}
	if ledger.Claims == nil {
		ledger.Claims = map[string]ProtocolAblationV32Claim{}
	}
	if ledger.GlobalHalt {
		return "halt", nil, nil
	}
	if !protocolAblationV32BindingMember(d, binding) {
		return "halt", nil, fmt.Errorf("v32 binding is not an exact matrix member")
	}
	if existing, ok := ledger.Claims[binding.BindingID]; ok {
		if existing.Status == "completed" && existing.Completed != nil {
			copy := *existing.Completed
			return "reuse", &copy, nil
		}
		ledger.GlobalHalt, ledger.HaltReason = true, "ambiguous_transport"
		return "halt", nil, nil
	}
	reservedTokens, reservedCost, reservedTurns := ledger.Usage.InputTokens+ledger.Usage.OutputTokens+ledger.Usage.ReasoningTokens, ledger.Usage.CostMicros, ledger.Usage.ProviderTurns
	for _, claim := range ledger.Claims {
		if claim.Status == "pending" {
			ledger.GlobalHalt, ledger.HaltReason = true, "ambiguous_transport"
			return "halt", nil, nil
		}
	}
	if reservedTokens+d.PerObservationTotalTokenReservation > d.MaxCombinedTokens || reservedCost+ProtocolAblationV32CostReservation(d, binding.Candidate) > d.MaxCombinedCostMicros || reservedTurns+binding.Protocol.MaxProviderTurns > d.MaxCombinedProviderTurns {
		ledger.GlobalHalt, ledger.HaltReason = true, "budget_breach"
		return "halt", nil, nil
	}
	ledger.Claims[binding.BindingID] = ProtocolAblationV32Claim{BindingID: binding.BindingID, Status: "pending"}
	return "launch", nil, nil
}

func CompleteProtocolAblationV32Observation(d ProtocolAblationV32Definition, ledger *ProtocolAblationV32Ledger, binding ProtocolAblationV32Binding, observation ProtocolAblationV32Observation) error {
	if !protocolAblationV32BindingMember(d, binding) {
		return fmt.Errorf("v32 binding is not an exact matrix member")
	}
	claim, ok := ledger.Claims[binding.BindingID]
	if !ok || claim.Status != "pending" || observation.BindingID != binding.BindingID || observation.EvidenceSHA256 == "" || observation.Usage.ProviderTurns < 1 || observation.Usage.ProviderTurns > binding.Protocol.MaxProviderTurns {
		return fmt.Errorf("v32 completion does not match its exclusive claim")
	}
	if observation.Usage.ProviderTurns < 0 || observation.Usage.InputTokens < 0 || observation.Usage.OutputTokens < 0 || observation.Usage.ReasoningTokens < 0 || observation.Usage.CostMicros < 0 {
		return fmt.Errorf("v32 usage fields cannot be negative")
	}
	identity := observation.ProviderIdentity
	expectedUpstreamModel := strings.TrimPrefix(binding.Candidate.Model, "openrouter/")
	servingExact := strings.EqualFold(identity.ReportedServingProvider, binding.Candidate.ServingProvider)
	servingEnforced := identity.ReportedServingProvider == "" && identity.ServingProviderEvidence == "enforced_single_route_no_fallback"
	if identity.DirectProvider != "openrouter" || identity.ReportedModel != expectedUpstreamModel || identity.ReportedEffort != binding.Candidate.Effort || (!servingExact && !servingEnforced) || (identity.ReportedServingProvider != "" && !servingExact) || len(identity.ResolvedConfigSHA256) != sha256.Size*2 || len(observation.RawReceipts) < 1 {
		return fmt.Errorf("v32 resolved or actual provider identity is not exact")
	}
	resolvedRouteProof := false
	for _, receipt := range observation.RawReceipts {
		if len(receipt) == 0 || !json.Valid(receipt) {
			return fmt.Errorf("v32 raw provider receipt is not valid JSON")
		}
		var envelope struct {
			ResolvedConfigSHA256 string `json:"resolved_config_sha256"`
		}
		if json.Unmarshal(receipt, &envelope) == nil && envelope.ResolvedConfigSHA256 == identity.ResolvedConfigSHA256 {
			resolvedRouteProof = true
		}
	}
	if servingEnforced && !resolvedRouteProof {
		return fmt.Errorf("v32 enforced serving route lacks exact resolved-config receipt")
	}
	if observation.GraderResult.CaseID != d.Case.CaseID {
		return fmt.Errorf("v32 grader result is not bound to the case")
	}
	if observation.FailureClass == "invalid_test" {
		observation.FailureClass = "test_invalidity"
	}
	if observation.FailureClass == "test_invalidity" && observation.FailureScope == "" {
		observation.FailureScope = "global"
	}
	if expected := ProtocolAblationV32ObservationEvidenceSHA(observation); expected != observation.EvidenceSHA256 {
		return fmt.Errorf("v32 observation evidence hash mismatch")
	}
	total := observation.Usage.InputTokens + observation.Usage.OutputTokens + observation.Usage.ReasoningTokens
	generated := observation.Usage.OutputTokens + observation.Usage.ReasoningTokens
	capBreach := total > d.PerObservationTotalTokenReservation || generated > d.PerObservationGeneratedTokenCeiling || observation.Usage.CostMicros > ProtocolAblationV32CostReservation(d, binding.Candidate)
	if capBreach {
		observation.Outcome, observation.FailureClass = "invalid", "budget_breach"
	}
	if !protocolAblationV32OutcomeAndFailureClassValid(observation.Outcome, observation.FailureClass, observation.FailureScope) {
		return fmt.Errorf("v32 observation outcome or failure class is not recognized")
	}
	claim.Status, claim.Completed = "completed", &observation
	ledger.Claims[binding.BindingID] = claim
	ledger.Usage.ProviderTurns += observation.Usage.ProviderTurns
	ledger.Usage.InputTokens += observation.Usage.InputTokens
	ledger.Usage.OutputTokens += observation.Usage.OutputTokens
	ledger.Usage.ReasoningTokens += observation.Usage.ReasoningTokens
	ledger.Usage.CostMicros += observation.Usage.CostMicros
	if observation.FailureClass != "test_invalidity" || observation.FailureScope != "observation" {
		if contains([]string{"test_invalidity", "unsafe_execution", "credential_failure", "budget_breach", "evidence_integrity_failure", "ambiguous_transport"}, observation.FailureClass) {
			ledger.GlobalHalt, ledger.HaltReason = true, observation.FailureClass
		}
	}
	return nil
}

func protocolAblationV32OutcomeAndFailureClassValid(outcome, failureClass, failureScope string) bool {
	outcome = strings.TrimSpace(outcome)
	failureClass = strings.TrimSpace(failureClass)
	switch outcome {
	case "pass":
		return failureClass == ""
	case "failed":
		return ProtocolAblationV32ObservationLocalFailure(failureClass)
	case "invalid":
		if failureClass == "test_invalidity" {
			return failureScope == "observation" || failureScope == "global"
		}
		return failureScope == "" && contains([]string{"unsafe_execution", "credential_failure", "budget_breach", "evidence_integrity_failure", "ambiguous_transport"}, failureClass)
	default:
		return false
	}
}

func protocolAblationV32BindingMember(d ProtocolAblationV32Definition, binding ProtocolAblationV32Binding) bool {
	for _, exact := range ProtocolAblationV32Bindings(d) {
		if reflect.DeepEqual(exact, binding) {
			return true
		}
	}
	return false
}

func ProtocolAblationV32ObservationEvidenceSHA(observation ProtocolAblationV32Observation) string {
	raw, _ := json.Marshal(struct {
		ProviderIdentity ProtocolAblationV32ProviderIdentity `json:"provider_identity"`
		RawReceipts      []json.RawMessage                   `json:"raw_receipts"`
		Actions          []RealisticCandidateAction          `json:"actions"`
		GraderResult     RealisticNextCalibrationResult      `json:"grader_result"`
	}{observation.ProviderIdentity, observation.RawReceipts, observation.Actions, observation.GraderResult})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

type ProtocolAblationV32ProviderPlan struct {
	BindingID                string          `json:"binding_id"`
	ProtocolID               string          `json:"protocol_id"`
	CaseID                   string          `json:"case_id"`
	Model                    string          `json:"model"`
	Effort                   string          `json:"effort"`
	ServingProvider          string          `json:"serving_provider"`
	ProviderFallbacks        bool            `json:"provider_fallbacks"`
	Transport                string          `json:"transport"`
	MaxProviderTurns         int             `json:"max_provider_turns"`
	ResponseSchema           json.RawMessage `json:"response_schema"`
	SchemaSHA256             string          `json:"schema_sha256"`
	PromptSHA256             string          `json:"prompt_sha256"`
	CanonicalTaskSHA256      string          `json:"canonical_task_sha256"`
	InlineConfigSHA256       string          `json:"inline_config_sha256"`
	GeneratedTokenCeiling    int64           `json:"generated_token_ceiling"`
	AcceptanceContractSHA256 string          `json:"acceptance_contract_sha256"`
	InitialPacket            json.RawMessage `json:"initial_packet"`
	InitialPacketSHA256      string          `json:"initial_packet_sha256"`
	ProtocolInstruction      string          `json:"protocol_instruction"`
	R05FixtureSourceSHA256   string          `json:"r05_fixture_source_sha256"`
	R05GraderSourceSHA256    string          `json:"r05_grader_source_sha256"`
}

type ProtocolAblationV32ConditionalStep struct {
	ID        string                   `json:"id"`
	Action    RealisticCandidateAction `json:"action"`
	OnSuccess string                   `json:"on_success"`
	OnFailure string                   `json:"on_failure"`
}

type ProtocolAblationV32ConditionalPlan struct {
	EntryStep string                               `json:"entry_step"`
	Steps     []ProtocolAblationV32ConditionalStep `json:"steps"`
}

type ProtocolAblationV32PlanResult struct {
	Verdict         RealisticNextCalibrationResult `json:"verdict"`
	ToolResults     []RealisticToolResult          `json:"tool_results"`
	ExecutedActions []RealisticCandidateAction     `json:"executed_actions"`
	FailureClass    string                         `json:"failure_class,omitempty"`
}

type ProtocolAblationV32ProviderResponse struct {
	CandidateJSON    json.RawMessage                     `json:"candidate_json"`
	RawReceipt       json.RawMessage                     `json:"raw_receipt"`
	ProviderIdentity ProtocolAblationV32ProviderIdentity `json:"provider_identity"`
	Usage            ProtocolAblationV32Usage            `json:"usage"`
}

type ProtocolAblationV32Provider interface {
	Call(context.Context, OpenCodeScreeningPlan) (ProtocolAblationV32ProviderResponse, error)
}

func RunProtocolAblationV32Observation(ctx context.Context, d ProtocolAblationV32Definition, binding ProtocolAblationV32Binding, workspace RealisticNextWorkspace, provider ProtocolAblationV32Provider) (ProtocolAblationV32Observation, error) {
	observation := ProtocolAblationV32Observation{BindingID: binding.BindingID}
	plan, err := BuildProtocolAblationV32OpenCodePlan(d, binding)
	if err != nil {
		return observation, err
	}
	var identity *ProtocolAblationV32ProviderIdentity
	call := func(callPlan OpenCodeScreeningPlan) (json.RawMessage, error) {
		response, err := provider.Call(ctx, callPlan)
		if err != nil {
			return nil, err
		}
		if identity == nil {
			copy := response.ProviderIdentity
			identity = &copy
		} else if !reflect.DeepEqual(*identity, response.ProviderIdentity) {
			return nil, fmt.Errorf("v32 provider identity changed within one observation")
		}
		if len(response.RawReceipt) == 0 || !json.Valid(response.RawReceipt) {
			return nil, fmt.Errorf("v32 provider returned an invalid raw receipt")
		}
		observation.RawReceipts = append(observation.RawReceipts, append(json.RawMessage(nil), response.RawReceipt...))
		observation.Usage.ProviderTurns++
		observation.Usage.InputTokens += response.Usage.InputTokens
		observation.Usage.OutputTokens += response.Usage.OutputTokens
		observation.Usage.ReasoningTokens += response.Usage.ReasoningTokens
		observation.Usage.CostMicros += response.Usage.CostMicros
		return response.CandidateJSON, nil
	}

	if binding.Protocol.ProtocolID == "one-shot-plan-v1" {
		raw, err := call(plan)
		if err != nil {
			return observation, err
		}
		var conditional ProtocolAblationV32ConditionalPlan
		if err := decodeProtocolAblationV32Strict(raw, &conditional); err != nil {
			observation.Outcome, observation.FailureClass = "failed", "response_or_tool_protocol_failure"
		} else {
			result := ExecuteProtocolAblationV32ConditionalPlan(ctx, workspace, conditional)
			observation.GraderResult, observation.FailureClass = result.Verdict, result.FailureClass
			observation.Actions = append(observation.Actions, result.ExecutedActions...)
		}
	} else {
		controllerResults := []RealisticToolResult{}
		for turn := 0; turn < binding.Protocol.MaxProviderTurns; turn++ {
			callPlan := plan
			if turn > 0 {
				feedback, _ := json.Marshal(controllerResults[len(controllerResults)-1])
				callPlan.Prompt += "\n<controller_result>" + string(feedback) + "</controller_result>"
				callPlan.PromptSHA256 = hashRealisticNextBytes([]byte(callPlan.Prompt))
			}
			raw, err := call(callPlan)
			if err != nil {
				return observation, err
			}
			var response struct {
				Action RealisticCandidateAction `json:"action"`
			}
			if err := decodeProtocolAblationV32Strict(raw, &response); err != nil || response.Action.Kind == "" {
				observation.Outcome, observation.FailureClass = "failed", "response_or_tool_protocol_failure"
				break
			}
			observation.Actions = append(observation.Actions, response.Action)
			if response.Action.Kind == "finish" {
				observation.GraderResult = realisticNextEvaluate(workspace)
				observation.GraderResult.CaseID = "P32"
				if observation.GraderResult.Verdict != RealisticNextPass {
					observation.Outcome, observation.FailureClass = "failed", "competence_failure"
				}
				break
			}
			started := time.Unix(0, int64(len(controllerResults))).UTC()
			output, touched, arguments, actionErr, protocolFailure := realisticNextExecute(ctx, workspace, response.Action)
			controller := RealisticControllerResult{ToolResults: controllerResults}
			appendRealisticToolResultAt(&controller, response.Action.Kind, output, actionErr, touched, arguments, started)
			controllerResults = controller.ToolResults
			if protocolFailure {
				observation.Outcome, observation.FailureClass = "failed", "response_or_tool_protocol_failure"
				break
			}
		}
		if observation.GraderResult.CaseID == "" {
			observation.GraderResult = realisticNextEvaluate(workspace)
			observation.GraderResult.CaseID = "P32"
		}
	}
	if identity == nil {
		return observation, fmt.Errorf("v32 observation made no provider call")
	}
	observation.ProviderIdentity = *identity
	if observation.GraderResult.CaseID == "" {
		observation.GraderResult = realisticNextEvaluate(workspace)
		observation.GraderResult.CaseID = "P32"
	}
	if observation.Outcome == "" {
		if observation.FailureClass == "" && observation.GraderResult.Verdict == RealisticNextPass {
			observation.Outcome = "pass"
		} else {
			observation.Outcome = "failed"
			if observation.FailureClass == "" {
				observation.FailureClass = "competence_failure"
			}
		}
	}
	observation.EvidenceSHA256 = ProtocolAblationV32ObservationEvidenceSHA(observation)
	return observation, nil
}

func decodeProtocolAblationV32Strict(raw []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing response JSON")
	}
	return nil
}

func BuildProtocolAblationV32ProviderFreePlan(d ProtocolAblationV32Definition, binding ProtocolAblationV32Binding) (ProtocolAblationV32ProviderPlan, error) {
	if err := ValidateProtocolAblationV32(d); err != nil {
		return ProtocolAblationV32ProviderPlan{}, err
	}
	single := binding.Protocol.ProtocolID == "one-action-loop-v1"
	actionSchema := `{"type":"object","additionalProperties":false,"required":["kind"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","finish"]},"path":{"type":"string","maxLength":128},"query":{"type":"string","maxLength":128},"contents":{"type":"string","maxLength":512},"check":{"type":"string","maxLength":128},"summary":{"type":"string","maxLength":256}}}`
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["action"],"properties":{"action":` + actionSchema + `}}`)
	if !single {
		// The one-shot arm supplies a complete conditional program once. The
		// deterministic controller still executes and observes every action; a
		// check failure follows on_failure instead of blindly continuing.
		schema = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["entry_step","steps"],"properties":{"entry_step":{"type":"string","maxLength":64},"steps":{"type":"array","minItems":1,"maxItems":10,"items":{"type":"object","additionalProperties":false,"required":["id","action","on_success","on_failure"],"properties":{"id":{"type":"string","maxLength":64},"action":` + actionSchema + `,"on_success":{"type":"string","maxLength":64},"on_failure":{"type":"string","maxLength":64}}}}}}`)
	}
	fixture, _ := CurrentR05V1SyntheticFixture()
	packet, err := ProtocolAblationV32CanonicalInitialPacket(d)
	if err != nil {
		return ProtocolAblationV32ProviderPlan{}, err
	}
	var task string
	for _, file := range fixture.Files {
		if file.Path == "TASK.md" {
			task = file.Content
			break
		}
	}
	taskHash := sha256.Sum256([]byte(task))
	schemaHash := sha256.Sum256(schema)
	protocolInstruction := "protocol=" + binding.Protocol.ResponseInterface + "\nReturn only the bounded structured response. The deterministic controller alone executes actions and reports check outcomes."
	prompt := "<common_initial_packet>\n" + string(packet) + "\n</common_initial_packet>\n<protocol_instruction>\n" + protocolInstruction + "\n</protocol_instruction>"
	promptHash := sha256.Sum256([]byte(prompt))
	configRaw, err := protocolAblationV32InlineConfig(binding)
	if err != nil {
		return ProtocolAblationV32ProviderPlan{}, err
	}
	configHash := sha256.Sum256(configRaw)
	contractRaw, _ := json.Marshal(map[string]any{"fixture_tree_sha256": fixture.FixtureTreeSHA256, "editable_paths": fixture.EditablePaths, "allowlisted_checks": fixture.AllowlistedChecks, "grader": d.Case.Grader, "failure_classes": []string{"competence_failure", "response_or_tool_protocol_failure", "test_invalidity", "unsafe_execution", "credential_failure", "budget_breach", "evidence_integrity_failure", "ambiguous_transport"}})
	contractHash := sha256.Sum256(contractRaw)
	packetHash := sha256.Sum256(packet)
	return ProtocolAblationV32ProviderPlan{BindingID: binding.BindingID, ProtocolID: binding.Protocol.ProtocolID, CaseID: d.Case.CaseID, Model: binding.Candidate.Model, Effort: binding.Candidate.Effort, ServingProvider: binding.Candidate.ServingProvider, ProviderFallbacks: false, Transport: d.Transport, MaxProviderTurns: binding.Protocol.MaxProviderTurns, ResponseSchema: schema, SchemaSHA256: hex.EncodeToString(schemaHash[:]), PromptSHA256: hex.EncodeToString(promptHash[:]), CanonicalTaskSHA256: hex.EncodeToString(taskHash[:]), InlineConfigSHA256: hex.EncodeToString(configHash[:]), GeneratedTokenCeiling: d.PerObservationGeneratedTokenCeiling, AcceptanceContractSHA256: hex.EncodeToString(contractHash[:]), InitialPacket: append(json.RawMessage(nil), packet...), InitialPacketSHA256: hex.EncodeToString(packetHash[:]), ProtocolInstruction: protocolInstruction, R05FixtureSourceSHA256: "ae6dfa595bb84d4c62ffb9ac510fe80efc6ef1c4a3dbd1acb9622b2f50173c31", R05GraderSourceSHA256: "1454ccbf164d8152779ecc2768e46132f42df53dee7e36e1c099d777e87bf07a"}, nil
}

func ProtocolAblationV32CanonicalInitialPacket(d ProtocolAblationV32Definition) ([]byte, error) {
	if err := ValidateProtocolAblationV32(d); err != nil {
		return nil, err
	}
	fixture, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		return nil, err
	}
	var task string
	for _, file := range fixture.Files {
		if file.Path == "TASK.md" {
			task = file.Content
			break
		}
	}
	packet := ProtocolAblationV32InitialPacket{
		CaseID: d.Case.CaseID, CaseVersion: d.Case.CaseVersion, Task: task,
		FixtureFiles: append([]R05V1FixtureFile(nil), fixture.Files...), EditablePaths: append([]string(nil), fixture.EditablePaths...),
		ImmutablePaths: append([]string(nil), fixture.ImmutablePaths...), AllowedActions: []string{"read", "search", "edit", "run_check", "finish"},
		AllowedChecks:      append([]string(nil), fixture.AllowlistedChecks...),
		AcceptanceBoundary: "pass only when the unchanged R05 grader verifies the requested bounded outcome; no browser, production, outbound, owner, or Phone Farm actions",
		FixtureTreeSHA256:  fixture.FixtureTreeSHA256, Grader: d.Case.Grader,
	}
	return json.Marshal(packet)
}

func ProtocolAblationV32ExecutionOrder(d ProtocolAblationV32Definition) ([]ProtocolAblationV32Binding, error) {
	if err := ValidateProtocolAblationV32(d); err != nil {
		return nil, err
	}
	all := ProtocolAblationV32Bindings(d)
	byID := make(map[string]ProtocolAblationV32Binding, len(all))
	for _, binding := range all {
		byID[binding.BindingID] = binding
	}
	ids := []string{
		"protocol-v32.claudesonnet5.medium.one-action-loop-v1",
		"protocol-v32.claudesonnet5.medium.one-shot-plan-v1",
		"protocol-v32.gpt54.medium.one-shot-plan-v1",
		"protocol-v32.gpt54.medium.one-action-loop-v1",
	}
	ordered := make([]ProtocolAblationV32Binding, 0, len(ids))
	for _, id := range ids {
		binding, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("v32 counterbalanced binding %s is missing", id)
		}
		ordered = append(ordered, binding)
	}
	return ordered, nil
}

func BuildProtocolAblationV32OpenCodePlan(d ProtocolAblationV32Definition, binding ProtocolAblationV32Binding) (OpenCodeScreeningPlan, error) {
	public, err := BuildProtocolAblationV32ProviderFreePlan(d, binding)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	if !protocolAblationV32BindingMember(d, binding) {
		return OpenCodeScreeningPlan{}, fmt.Errorf("v32 execution binding is not exact")
	}
	fallbacks := false
	inline, err := protocolAblationV32InlineConfig(binding)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	prompt := "<common_initial_packet>\n" + string(public.InitialPacket) + "\n</common_initial_packet>\n<protocol_instruction>\n" + public.ProtocolInstruction + "\n</protocol_instruction>"
	return OpenCodeScreeningPlan{
		Transport: OpenCodeRealisticActionTransport, CandidateID: binding.BindingID, CaseID: d.Case.CaseID, CaseVersion: d.Case.CaseVersion,
		Model: binding.Candidate.Model, Effort: binding.Candidate.Effort, Prompt: prompt, PromptSHA256: hashRealisticNextBytes([]byte(prompt)), Schema: append([]byte(nil), public.ResponseSchema...), SchemaSHA256: hashRealisticNextBytes(public.ResponseSchema), InlineConfig: inline, InlineConfigSHA256: hashRealisticNextBytes(inline),
		ArgsTemplate:        []string{"--pure", "run", "--format", "json", "--model", binding.Candidate.Model, "--variant", binding.Candidate.Effort, "--agent", "qualification", "--title", "protocol-v32-<claim>", "--dir", "<ephemeral-r05-workspace>", "<exact-prompt-via-stdin>"},
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", d.PerObservationGeneratedTokenCeiling)},
		ServingProvider:     binding.Candidate.ServingProvider, ProviderFallbacks: &fallbacks,
	}, nil
}

func protocolAblationV32InlineConfig(binding ProtocolAblationV32Binding) ([]byte, error) {
	modelID := strings.TrimPrefix(binding.Candidate.Model, "openrouter/")
	if modelID == binding.Candidate.Model || modelID == "" {
		return nil, fmt.Errorf("v32 model must use an exact OpenRouter id")
	}
	model := openCodeModelConfig{Options: map[string]any{"provider": map[string]any{"order": []string{binding.Candidate.ServingProvider}, "allow_fallbacks": false}}}
	config := openCodeQualificationConfig{
		Schema: "https://opencode.ai/config.json", Autoupdate: false, Share: "disabled", Snapshot: false,
		Plugin: []string{}, Instructions: []string{}, Permission: "deny", Tools: map[string]bool{"*": false},
		Compaction: map[string]bool{"auto": false, "prune": false}, DefaultAgent: "qualification",
		Agent:    map[string]openCodeQualificationAgent{"qualification": {Description: "Pinned v32 protocol-ablation candidate", Mode: "primary", Model: binding.Candidate.Model, Variant: binding.Candidate.Effort, Prompt: "Use only the byte-identical common packet and selected protocol interface. Never use built-in tools or hidden context.", Steps: 2, Permission: "deny", Tools: map[string]bool{"*": false}}},
		Provider: map[string]openCodeProviderConfig{"openrouter": {Options: map[string]any{"name": "OpenRouter"}, Headers: map[string]string{"X-OpenRouter-Metadata": "enabled"}, Whitelist: []string{modelID}, Models: map[string]openCodeModelConfig{modelID: model}}},
	}
	return json.Marshal(config)
}

func ProtocolAblationV32MinimumLoopTurns() int { return 8 }

func ProtocolAblationV32ProtocolDifference(a, b ProtocolAblationV32ProviderPlan) bool {
	return a.Model == b.Model && a.Effort == b.Effort && a.ServingProvider == b.ServingProvider && a.Transport == b.Transport && a.InlineConfigSHA256 == b.InlineConfigSHA256 && a.GeneratedTokenCeiling == b.GeneratedTokenCeiling && a.AcceptanceContractSHA256 == b.AcceptanceContractSHA256 && !a.ProviderFallbacks && !b.ProviderFallbacks && a.CaseID == b.CaseID && a.CanonicalTaskSHA256 == b.CanonicalTaskSHA256 && a.InitialPacketSHA256 == b.InitialPacketSHA256 && bytes.Equal(a.InitialPacket, b.InitialPacket) && a.R05FixtureSourceSHA256 == b.R05FixtureSourceSHA256 && a.R05GraderSourceSHA256 == b.R05GraderSourceSHA256 && a.ProtocolID != b.ProtocolID && a.ProtocolInstruction != b.ProtocolInstruction && a.SchemaSHA256 != b.SchemaSHA256 && a.PromptSHA256 != b.PromptSHA256
}

func ProtocolAblationV32ObservationLocalFailure(class string) bool {
	return strings.TrimSpace(class) == "competence_failure" || strings.TrimSpace(class) == "response_or_tool_protocol_failure"
}

// ExecuteProtocolAblationV32ConditionalPlan interprets the one provider
// response with the same action executor and grader as the loop protocol.
// The model proposes branches; only the controller observes check outcomes.
func ExecuteProtocolAblationV32ConditionalPlan(ctx context.Context, workspace RealisticNextWorkspace, plan ProtocolAblationV32ConditionalPlan) ProtocolAblationV32PlanResult {
	result := ProtocolAblationV32PlanResult{}
	if len(plan.Steps) < 1 || len(plan.Steps) > 10 || strings.TrimSpace(plan.EntryStep) == "" {
		result.FailureClass = "response_or_tool_protocol_failure"
		return result
	}
	steps := map[string]ProtocolAblationV32ConditionalStep{}
	for _, step := range plan.Steps {
		if strings.TrimSpace(step.ID) == "" || len(step.ID) > 64 || step.OnSuccess == "" || step.OnFailure == "" {
			result.FailureClass = "response_or_tool_protocol_failure"
			return result
		}
		if _, duplicate := steps[step.ID]; duplicate {
			result.FailureClass = "response_or_tool_protocol_failure"
			return result
		}
		steps[step.ID] = step
	}
	current := plan.EntryStep
	visited := map[string]bool{}
	for len(visited) < len(plan.Steps) {
		if current == "fail" {
			result.FailureClass = "competence_failure"
			result.Verdict = realisticNextEvaluate(workspace)
			result.Verdict.CaseID = "P32"
			return result
		}
		step, ok := steps[current]
		if !ok || visited[current] {
			result.FailureClass = "response_or_tool_protocol_failure"
			return result
		}
		visited[current] = true
		result.ExecutedActions = append(result.ExecutedActions, step.Action)
		if step.Action.Kind == "finish" {
			if strings.TrimSpace(step.Action.Summary) == "" || step.OnSuccess != "complete" {
				result.FailureClass = "response_or_tool_protocol_failure"
				return result
			}
			result.Verdict = realisticNextEvaluate(workspace)
			result.Verdict.CaseID = "P32"
			if result.Verdict.Verdict != RealisticNextPass {
				result.FailureClass = "competence_failure"
			}
			return result
		}
		started := time.Unix(0, int64(len(result.ToolResults))).UTC()
		output, touched, arguments, actionErr, protocolFailure := realisticNextExecute(ctx, workspace, step.Action)
		controller := RealisticControllerResult{ToolResults: result.ToolResults}
		appendRealisticToolResultAt(&controller, step.Action.Kind, output, actionErr, touched, arguments, started)
		result.ToolResults = controller.ToolResults
		if protocolFailure {
			result.FailureClass = "response_or_tool_protocol_failure"
			return result
		}
		if actionErr != nil {
			current = step.OnFailure
		} else {
			current = step.OnSuccess
		}
	}
	result.FailureClass = "response_or_tool_protocol_failure"
	return result
}

func ProtocolAblationV32MaximalPlanEnvelopeBytes() int { return 16_000 }
