package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const ProvisionalRoleConfigurationRehearsalV1Schema = "silver-mediums.provisional-role-configuration-rehearsal.v1"

//go:embed provisional_role_configuration_rehearsal_v1_result.json
var provisionalRoleConfigurationRehearsalV1ResultJSON []byte

type ProvisionalRoleConfigurationRequestV1 struct {
	CaseID               string `json:"case_id"`
	Participant          string `json:"participant"`
	ParticipantKind      string `json:"participant_kind"`
	InlineSkillPacket    string `json:"inline_skill_packet"`
	ExecutionMode        string `json:"execution_mode"`
	ProviderLane         string `json:"provider_lane"`
	Model                string `json:"model"`
	Squad                string `json:"squad"`
	ProviderCalls        int    `json:"provider_calls"`
	CredentialReads      int    `json:"credential_reads"`
	NativeToolExecutions int    `json:"native_tool_executions"`
	RepositoryAccesses   int    `json:"repository_accesses"`
	ProductionAccesses   int    `json:"production_accesses"`
	OutboundActions      int    `json:"outbound_actions"`
	PhoneFarmAccesses    int    `json:"phone_farm_accesses"`
	Retries              int    `json:"retries"`
	Fallbacks            bool   `json:"fallbacks"`
	PermanentSelection   bool   `json:"permanent_selection"`
	AutomaticDispatch    bool   `json:"automatic_dispatch"`
}

type ProvisionalRoleConfigurationRehearsalReceiptV1 struct {
	ScenarioID       string `json:"scenario_id"`
	ExpectedDecision string `json:"expected_decision"`
	ActualDecision   string `json:"actual_decision"`
	Reason           string `json:"reason"`
	Matched          bool   `json:"matched"`
	SideEffects      int    `json:"side_effects"`
}

type ProvisionalRoleConfigurationRehearsalResultV1 struct {
	SchemaVersion         string `json:"schema_version"`
	RunID                 string `json:"run_id"`
	ExecutionMode         string `json:"execution_mode"`
	ConfigurationSHA256   string `json:"configuration_sha256"`
	TraceSHA256           string `json:"trace_sha256"`
	Scenarios             int    `json:"scenarios"`
	Allowed               int    `json:"allowed"`
	Denied                int    `json:"denied"`
	Mismatches            int    `json:"mismatches"`
	ProviderCalls         int    `json:"provider_calls"`
	CredentialReads       int    `json:"credential_reads"`
	NativeToolExecutions  int    `json:"native_tool_executions"`
	RepositoryAccesses    int    `json:"repository_accesses"`
	ProductionAccesses    int    `json:"production_accesses"`
	OutboundActions       int    `json:"outbound_actions"`
	PhoneFarmAccesses     int    `json:"phone_farm_accesses"`
	ModelOrRoleSelections int    `json:"model_or_role_selections"`
	Valid                 bool   `json:"valid"`
}

func CurrentProvisionalRoleConfigurationRehearsalV1() (ProvisionalRoleConfigurationRehearsalResultV1, error) {
	var recorded ProvisionalRoleConfigurationRehearsalResultV1
	if err := strictJSON(provisionalRoleConfigurationRehearsalV1ResultJSON, &recorded); err != nil {
		return recorded, fmt.Errorf("decode provisional role configuration rehearsal: %w", err)
	}
	expected, receipts, err := RunProvisionalRoleConfigurationRehearsalV1()
	if err != nil {
		return recorded, err
	}
	if recorded != expected {
		return recorded, fmt.Errorf("recorded provisional role configuration rehearsal drifted")
	}
	return recorded, ValidateProvisionalRoleConfigurationRehearsalV1(recorded, receipts)
}

func ProvisionalRoleConfigurationRehearsalV1SHA256() string {
	sum := sha256.Sum256(provisionalRoleConfigurationRehearsalV1ResultJSON)
	return hex.EncodeToString(sum[:])
}

func EvaluateProvisionalRoleConfigurationRequestV1(request ProvisionalRoleConfigurationRequestV1) (string, string) {
	configuration, err := CurrentProvisionalRoleConfigurationV1()
	if err != nil {
		return "DENY", "configuration_invalid"
	}
	lifecycle, err := CurrentProvisionalRoleConfigurationLifecycleV2()
	if err != nil {
		return "DENY", "lifecycle_invalid"
	}
	if active, reason := ResolveProvisionalRoleConfigurationLifecycleV2(lifecycle, configuration.ConfigurationID, ProvisionalRoleConfigurationV1SHA256(), time.Now().UTC()); !active {
		return "DENY", reason
	}
	return evaluateProvisionalRoleConfigurationRequestV1Historical(request)
}

func evaluateProvisionalRoleConfigurationRequestV1Historical(request ProvisionalRoleConfigurationRequestV1) (string, string) {
	configuration, err := CurrentProvisionalRoleConfigurationV1()
	if err != nil {
		return "DENY", "configuration_invalid"
	}
	checks := []struct {
		ok     bool
		reason string
	}{
		{request.CaseID == configuration.AllowedCaseIDs[0], "case_not_qualified"},
		{request.Participant == configuration.Participant, "participant_not_qualified"},
		{request.ParticipantKind == configuration.ParticipantKind, "participant_kind_not_qualified"},
		{request.InlineSkillPacket == configuration.InlineSkillPacket, "inline_skill_not_qualified"},
		{request.ExecutionMode == configuration.ExecutionMode, "execution_mode_not_qualified"},
		{request.ProviderLane == configuration.ProviderLane, "provider_lane_unselected"},
		{request.Model == configuration.Model, "model_unselected"},
		{request.Squad == configuration.Squad, "squad_unselected"},
		{request.ProviderCalls == 0, "provider_calls_forbidden"},
		{request.CredentialReads == 0, "credential_reads_forbidden"},
		{request.NativeToolExecutions == 0, "native_tools_forbidden"},
		{request.RepositoryAccesses == 0, "repository_access_forbidden"},
		{request.ProductionAccesses == 0, "production_access_forbidden"},
		{request.OutboundActions == 0, "outbound_actions_forbidden"},
		{request.PhoneFarmAccesses == 0, "phone_farm_windows_access_forbidden"},
		{request.Retries == 0, "retries_forbidden"},
		{!request.Fallbacks, "fallbacks_forbidden"},
		{!request.PermanentSelection, "permanent_selection_forbidden"},
		{!request.AutomaticDispatch, "automatic_dispatch_forbidden"},
	}
	for _, check := range checks {
		if !check.ok {
			return "DENY", check.reason
		}
	}
	return "ALLOW", "exact_provisional_local_r01_binding"
}

func RunProvisionalRoleConfigurationRehearsalV1() (ProvisionalRoleConfigurationRehearsalResultV1, []ProvisionalRoleConfigurationRehearsalReceiptV1, error) {
	configuration, err := CurrentProvisionalRoleConfigurationV1()
	if err != nil {
		return ProvisionalRoleConfigurationRehearsalResultV1{}, nil, err
	}
	exact := ProvisionalRoleConfigurationRequestV1{CaseID: "R01", Participant: configuration.Participant, ParticipantKind: configuration.ParticipantKind, InlineSkillPacket: configuration.InlineSkillPacket, ExecutionMode: configuration.ExecutionMode, ProviderLane: "unselected", Model: "unselected", Squad: "unselected"}
	type scenario struct {
		id, expected, reason string
		request              ProvisionalRoleConfigurationRequestV1
	}
	scenarios := []scenario{{id: "exact-qualified-scope", expected: "ALLOW", reason: "exact_provisional_local_r01_binding", request: exact}}
	add := func(id, reason string, mutate func(*ProvisionalRoleConfigurationRequestV1)) {
		request := exact
		mutate(&request)
		scenarios = append(scenarios, scenario{id: id, expected: "DENY", reason: reason, request: request})
	}
	add("r02", "case_not_qualified", func(v *ProvisionalRoleConfigurationRequestV1) { v.CaseID = "R02" })
	add("separate-specialist", "participant_not_qualified", func(v *ProvisionalRoleConfigurationRequestV1) { v.Participant = "architecture_assurance" })
	add("specialist-kind", "participant_kind_not_qualified", func(v *ProvisionalRoleConfigurationRequestV1) { v.ParticipantKind = "specialist" })
	add("product-ui-skill", "inline_skill_not_qualified", func(v *ProvisionalRoleConfigurationRequestV1) { v.InlineSkillPacket = "product_ui" })
	add("provider-backed-mode", "execution_mode_not_qualified", func(v *ProvisionalRoleConfigurationRequestV1) { v.ExecutionMode = "provider_backed" })
	add("provider-lane", "provider_lane_unselected", func(v *ProvisionalRoleConfigurationRequestV1) { v.ProviderLane = "claude_code_subscription" })
	add("model", "model_unselected", func(v *ProvisionalRoleConfigurationRequestV1) { v.Model = "claude-sonnet-4-6" })
	add("squad", "squad_unselected", func(v *ProvisionalRoleConfigurationRequestV1) { v.Squad = "cross_boundary_change" })
	add("provider-call", "provider_calls_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.ProviderCalls = 1 })
	add("credential", "credential_reads_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.CredentialReads = 1 })
	add("native-tool", "native_tools_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.NativeToolExecutions = 1 })
	add("repository", "repository_access_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.RepositoryAccesses = 1 })
	add("production", "production_access_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.ProductionAccesses = 1 })
	add("outbound", "outbound_actions_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.OutboundActions = 1 })
	add("phone-farm", "phone_farm_windows_access_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.PhoneFarmAccesses = 1 })
	add("retry", "retries_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.Retries = 1 })
	add("fallback", "fallbacks_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.Fallbacks = true })
	add("permanent-selection", "permanent_selection_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.PermanentSelection = true })
	add("automatic-dispatch", "automatic_dispatch_forbidden", func(v *ProvisionalRoleConfigurationRequestV1) { v.AutomaticDispatch = true })
	receipts := make([]ProvisionalRoleConfigurationRehearsalReceiptV1, 0, len(scenarios))
	allowed, denied, mismatches := 0, 0, 0
	for _, candidate := range scenarios {
		decision, reason := evaluateProvisionalRoleConfigurationRequestV1Historical(candidate.request)
		matched := decision == candidate.expected && reason == candidate.reason
		if !matched {
			mismatches++
		}
		if decision == "ALLOW" {
			allowed++
		} else {
			denied++
		}
		receipts = append(receipts, ProvisionalRoleConfigurationRehearsalReceiptV1{ScenarioID: candidate.id, ExpectedDecision: candidate.expected, ActualDecision: decision, Reason: reason, Matched: matched})
	}
	raw, err := json.Marshal(receipts)
	if err != nil {
		return ProvisionalRoleConfigurationRehearsalResultV1{}, nil, err
	}
	sum := sha256.Sum256(raw)
	result := ProvisionalRoleConfigurationRehearsalResultV1{SchemaVersion: ProvisionalRoleConfigurationRehearsalV1Schema, RunID: "item-14-provisional-role-configuration-rehearsal-20260914", ExecutionMode: "deterministic_provider_free_boundary_rehearsal", ConfigurationSHA256: ProvisionalRoleConfigurationV1SHA256(), TraceSHA256: hex.EncodeToString(sum[:]), Scenarios: len(scenarios), Allowed: allowed, Denied: denied, Mismatches: mismatches, Valid: mismatches == 0}
	if err := ValidateProvisionalRoleConfigurationRehearsalV1(result, receipts); err != nil {
		return result, receipts, err
	}
	return result, receipts, nil
}

func ValidateProvisionalRoleConfigurationRehearsalV1(result ProvisionalRoleConfigurationRehearsalResultV1, receipts []ProvisionalRoleConfigurationRehearsalReceiptV1) error {
	if result.SchemaVersion != ProvisionalRoleConfigurationRehearsalV1Schema || result.RunID != "item-14-provisional-role-configuration-rehearsal-20260914" || result.ExecutionMode != "deterministic_provider_free_boundary_rehearsal" || result.ConfigurationSHA256 != ProvisionalRoleConfigurationV1SHA256() || result.Scenarios != 20 || result.Allowed != 1 || result.Denied != 19 || result.Mismatches != 0 || !result.Valid || len(receipts) != result.Scenarios {
		return fmt.Errorf("provisional role configuration rehearsal aggregate drifted")
	}
	if result.ProviderCalls != 0 || result.CredentialReads != 0 || result.NativeToolExecutions != 0 || result.RepositoryAccesses != 0 || result.ProductionAccesses != 0 || result.OutboundActions != 0 || result.PhoneFarmAccesses != 0 || result.ModelOrRoleSelections != 0 {
		return fmt.Errorf("provisional role configuration rehearsal crossed a forbidden boundary")
	}
	seen := map[string]bool{}
	for _, receipt := range receipts {
		if receipt.ScenarioID == "" || seen[receipt.ScenarioID] || !receipt.Matched || receipt.ExpectedDecision != receipt.ActualDecision || receipt.SideEffects != 0 {
			return fmt.Errorf("provisional role configuration rehearsal receipt drifted")
		}
		seen[receipt.ScenarioID] = true
	}
	raw, err := json.Marshal(receipts)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if result.TraceSHA256 != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("provisional role configuration rehearsal trace drifted")
	}
	return nil
}
