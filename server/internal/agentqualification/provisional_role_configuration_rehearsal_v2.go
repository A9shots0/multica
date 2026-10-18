package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

const ProvisionalRoleConfigurationRehearsalV2Schema = "silver-mediums.provisional-role-configuration-rehearsal.v2"

//go:embed provisional_role_configuration_rehearsal_v2_result.json
var provisionalRoleConfigurationRehearsalV2ResultJSON []byte

type ProvisionalRoleConfigurationRequestV2 struct {
	ConfigurationID                          string   `json:"configuration_id"`
	ConfigurationSHA256                      string   `json:"configuration_sha256"`
	CaseFamily                               string   `json:"case_family"`
	WorkflowKind                             string   `json:"workflow_kind"`
	RouteStages                              []string `json:"route_stages"`
	Participant                              string   `json:"participant"`
	ParticipantArtifactBindingID             string   `json:"participant_artifact_binding_id"`
	InlineSkillPacket                        string   `json:"inline_skill_packet"`
	InlineSkillArtifactBindingID             string   `json:"inline_skill_artifact_binding_id"`
	IntakeRole                               string   `json:"intake_role"`
	ReviewerRole                             string   `json:"reviewer_role"`
	ReviewerArtifactBindingID                string   `json:"reviewer_artifact_binding_id"`
	ReviewerDistinct                         bool     `json:"reviewer_distinct"`
	IndependentReviewRequired                bool     `json:"independent_review_required"`
	MaximumCorrections                       int      `json:"maximum_corrections"`
	MaximumProviderTasks                     int      `json:"maximum_provider_tasks"`
	ExecutionMode                            string   `json:"execution_mode"`
	TransportLane                            string   `json:"transport_lane"`
	RuntimeProvider                          string   `json:"runtime_provider"`
	RequestedPrimaryModel                    string   `json:"requested_primary_model"`
	AllowedObservedModels                    []string `json:"allowed_observed_models"`
	ReasoningEffort                          string   `json:"reasoning_effort"`
	ExactSingleModelClaim                    bool     `json:"exact_single_model_claim"`
	SeparatePerRunAuthorityRequired          bool     `json:"separate_per_run_authority_required"`
	RuntimeBindingSchema                     string   `json:"runtime_binding_schema"`
	CapabilityScopeSchema                    string   `json:"capability_scope_schema"`
	ProviderCalls                            int      `json:"provider_calls"`
	CredentialReads                          int      `json:"credential_reads"`
	ProviderChildToolExecutions              int      `json:"provider_child_tool_executions"`
	ProviderChildRepositoryAccesses          int      `json:"provider_child_repository_accesses"`
	DeterministicBrokerRepositoryRead        bool     `json:"deterministic_broker_repository_read"`
	DeterministicBrokerAdditiveEvidenceWrite bool     `json:"deterministic_broker_additive_evidence_write"`
	ExistingFileMutation                     bool     `json:"existing_file_mutation"`
	AutomaticDispatch                        bool     `json:"automatic_dispatch"`
	MaxAttemptsPerTask                       int      `json:"max_attempts_per_task"`
	Retries                                  int      `json:"retries"`
	Fallbacks                                bool     `json:"fallbacks"`
	TaskScopedTemporarySquad                 bool     `json:"task_scoped_temporary_squad"`
	PermanentRoleOrSeatSelection             bool     `json:"permanent_role_or_seat_selection"`
	PermanentSquadActivation                 bool     `json:"permanent_squad_activation"`
	ProductionAccesses                       int      `json:"production_accesses"`
	OutboundActions                          int      `json:"outbound_actions"`
	PhoneFarmWindowsAccesses                 int      `json:"phone_farm_windows_accesses"`
}

type ProvisionalRoleConfigurationRehearsalReceiptV2 struct {
	Ordinal             int    `json:"ordinal"`
	ScenarioID          string `json:"scenario_id"`
	ScenarioNonce       string `json:"scenario_nonce"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	RequestSHA256       string `json:"request_sha256"`
	LifecycleSHA256     string `json:"lifecycle_sha256"`
	EvaluationTime      string `json:"evaluation_time"`
	ExpectedDecision    string `json:"expected_decision"`
	ActualDecision      string `json:"actual_decision"`
	Reason              string `json:"reason"`
	Matched             bool   `json:"matched"`
	SideEffects         int    `json:"side_effects"`
}

type ProvisionalRoleConfigurationRehearsalResultV2 struct {
	SchemaVersion                   string `json:"schema_version"`
	RunID                           string `json:"run_id"`
	ExecutionMode                   string `json:"execution_mode"`
	ConfigurationSHA256             string `json:"configuration_sha256"`
	LifecycleSHA256                 string `json:"lifecycle_sha256"`
	TraceSHA256                     string `json:"trace_sha256"`
	Scenarios                       int    `json:"scenarios"`
	AllowedPrepare                  int    `json:"allowed_prepare"`
	Denied                          int    `json:"denied"`
	Mismatches                      int    `json:"mismatches"`
	ProvisionalDefaults             int    `json:"provisional_defaults"`
	ProviderCalls                   int    `json:"provider_calls"`
	CredentialReads                 int    `json:"credential_reads"`
	NativeToolExecutions            int    `json:"native_tool_executions"`
	ProviderChildRepositoryAccesses int    `json:"provider_child_repository_accesses"`
	ProductionAccesses              int    `json:"production_accesses"`
	OutboundActions                 int    `json:"outbound_actions"`
	PhoneFarmWindowsAccesses        int    `json:"phone_farm_windows_accesses"`
	PermanentSelections             int    `json:"permanent_selections"`
	Valid                           bool   `json:"valid"`
}

type provisionalRoleConfigurationScenarioV2 struct {
	id             string
	expected       string
	reason         string
	request        ProvisionalRoleConfigurationRequestV2
	lifecycle      ProvisionalRoleConfigurationLifecycleV2
	evaluationTime time.Time
}

func CurrentProvisionalRoleConfigurationRehearsalV2() (ProvisionalRoleConfigurationRehearsalResultV2, error) {
	var recorded ProvisionalRoleConfigurationRehearsalResultV2
	if err := strictJSON(provisionalRoleConfigurationRehearsalV2ResultJSON, &recorded); err != nil {
		return recorded, fmt.Errorf("decode provisional role configuration rehearsal v2: %w", err)
	}
	expected, receipts, err := RunProvisionalRoleConfigurationRehearsalV2()
	if err != nil {
		return recorded, err
	}
	if recorded != expected {
		return recorded, fmt.Errorf("recorded provisional role configuration rehearsal v2 drifted")
	}
	return recorded, ValidateProvisionalRoleConfigurationRehearsalV2(recorded, receipts)
}

func ProvisionalRoleConfigurationRehearsalV2SHA256() string {
	sum := sha256.Sum256(provisionalRoleConfigurationRehearsalV2ResultJSON)
	return hex.EncodeToString(sum[:])
}

func EvaluateProvisionalRoleConfigurationRequestV2(request ProvisionalRoleConfigurationRequestV2) (string, string) {
	lifecycle, err := CurrentProvisionalRoleConfigurationLifecycleV2()
	if err != nil {
		return "DENY", "lifecycle_invalid"
	}
	return EvaluateProvisionalRoleConfigurationRequestV2At(request, lifecycle, time.Now().UTC())
}

func EvaluateCurrentProvisionalRoleConfigurationV2At(now time.Time) (string, string, error) {
	configuration, err := CurrentProvisionalRoleConfigurationV2()
	if err != nil {
		return "DENY", "configuration_invalid", err
	}
	lifecycle, err := CurrentProvisionalRoleConfigurationLifecycleV2()
	if err != nil {
		return "DENY", "lifecycle_invalid", err
	}
	decision, reason := EvaluateProvisionalRoleConfigurationRequestV2At(exactProvisionalRoleConfigurationRequestV2(configuration), lifecycle, now.UTC())
	return decision, reason, nil
}

func EvaluateProvisionalRoleConfigurationRequestV2At(request ProvisionalRoleConfigurationRequestV2, lifecycle ProvisionalRoleConfigurationLifecycleV2, now time.Time) (string, string) {
	configuration, err := CurrentProvisionalRoleConfigurationV2()
	if err != nil {
		return "DENY", "configuration_invalid"
	}
	if request.ConfigurationID != configuration.ConfigurationID {
		return "DENY", "configuration_id_mismatch"
	}
	if request.ConfigurationSHA256 != ProvisionalRoleConfigurationV2SHA256() {
		return "DENY", "configuration_hash_mismatch"
	}
	if active, reason := ResolveProvisionalRoleConfigurationLifecycleV2(lifecycle, request.ConfigurationID, request.ConfigurationSHA256, now); !active {
		return "DENY", reason
	}
	checks := []struct {
		ok     bool
		reason string
	}{
		{request.CaseFamily == configuration.CaseFamily, "case_family_not_qualified"},
		{request.WorkflowKind == configuration.WorkflowKind, "workflow_kind_not_qualified"},
		{slices.Equal(request.RouteStages, configuration.RouteStages), "route_stage_drift"},
		{request.Participant == configuration.Participant, "participant_not_qualified"},
		{request.ParticipantArtifactBindingID == configuration.ParticipantArtifactBindingID, "participant_binding_drift"},
		{request.InlineSkillPacket == configuration.InlineSkillPacket, "inline_skill_not_qualified"},
		{request.InlineSkillArtifactBindingID == configuration.InlineSkillArtifactBindingID, "inline_skill_binding_drift"},
		{request.IntakeRole == configuration.IntakeRole, "intake_role_drift"},
		{request.ReviewerRole == configuration.ReviewerRole, "reviewer_role_drift"},
		{request.ReviewerArtifactBindingID == configuration.ReviewerArtifactBindingID, "reviewer_binding_drift"},
		{request.ReviewerDistinct, "reviewer_not_distinct"},
		{request.IndependentReviewRequired == configuration.IndependentReviewRequired, "independent_review_required"},
		{request.MaximumCorrections == configuration.MaximumCorrections, "correction_not_qualified"},
		{request.MaximumProviderTasks == configuration.MaximumProviderTasks, "provider_task_ceiling_drift"},
		{request.ExecutionMode == configuration.ExecutionMode, "execution_mode_not_qualified"},
		{request.TransportLane == configuration.TransportLane, "transport_lane_drift"},
		{request.RuntimeProvider == configuration.RuntimeProvider, "runtime_provider_drift"},
		{request.RequestedPrimaryModel == configuration.RequestedPrimaryModel, "requested_model_drift"},
		{slices.Equal(request.AllowedObservedModels, configuration.AllowedObservedModels), "observed_model_set_drift"},
		{request.ReasoningEffort == configuration.ReasoningEffort, "reasoning_effort_drift"},
		{!request.ExactSingleModelClaim, "exact_single_model_claim_forbidden"},
		{request.SeparatePerRunAuthorityRequired, "separate_per_run_authority_required"},
		{request.RuntimeBindingSchema == configuration.RequiredRuntimeBindingSchema, "runtime_binding_schema_drift"},
		{request.CapabilityScopeSchema == configuration.RequiredCapabilityScopeSchema, "capability_scope_schema_drift"},
		{request.ProviderCalls == 0, "configuration_does_not_authorize_provider_call"},
		{request.CredentialReads == 0, "configuration_does_not_authorize_credential_read"},
		{request.ProviderChildToolExecutions == 0, "provider_child_tools_forbidden"},
		{request.ProviderChildRepositoryAccesses == 0, "provider_child_repository_access_forbidden"},
		{request.DeterministicBrokerRepositoryRead, "broker_repository_read_required"},
		{request.DeterministicBrokerAdditiveEvidenceWrite, "broker_additive_evidence_write_required"},
		{!request.ExistingFileMutation, "existing_file_mutation_forbidden"},
		{!request.AutomaticDispatch, "automatic_dispatch_forbidden"},
		{request.MaxAttemptsPerTask == 1, "max_attempts_drift"},
		{request.Retries == 0, "retries_forbidden"},
		{!request.Fallbacks, "fallbacks_forbidden"},
		{request.TaskScopedTemporarySquad, "persistent_squad_forbidden"},
		{!request.PermanentRoleOrSeatSelection, "permanent_role_or_seat_forbidden"},
		{!request.PermanentSquadActivation, "permanent_squad_activation_forbidden"},
		{request.ProductionAccesses == 0, "production_access_forbidden"},
		{request.OutboundActions == 0, "outbound_actions_forbidden"},
		{request.PhoneFarmWindowsAccesses == 0, "phone_farm_windows_access_forbidden"},
	}
	for _, check := range checks {
		if !check.ok {
			return "DENY", check.reason
		}
	}
	return "ALLOW_PREPARE", "exact_reversible_local_r01_operating_default"
}

func exactProvisionalRoleConfigurationRequestV2(configuration ProvisionalRoleConfigurationV2) ProvisionalRoleConfigurationRequestV2 {
	return ProvisionalRoleConfigurationRequestV2{
		ConfigurationID: configuration.ConfigurationID, ConfigurationSHA256: ProvisionalRoleConfigurationV2SHA256(), CaseFamily: configuration.CaseFamily,
		WorkflowKind: configuration.WorkflowKind, RouteStages: append([]string(nil), configuration.RouteStages...), Participant: configuration.Participant,
		ParticipantArtifactBindingID: configuration.ParticipantArtifactBindingID, InlineSkillPacket: configuration.InlineSkillPacket, InlineSkillArtifactBindingID: configuration.InlineSkillArtifactBindingID,
		IntakeRole: configuration.IntakeRole, ReviewerRole: configuration.ReviewerRole, ReviewerArtifactBindingID: configuration.ReviewerArtifactBindingID, ReviewerDistinct: true,
		IndependentReviewRequired: configuration.IndependentReviewRequired, MaximumCorrections: configuration.MaximumCorrections, MaximumProviderTasks: configuration.MaximumProviderTasks,
		ExecutionMode: configuration.ExecutionMode, TransportLane: configuration.TransportLane, RuntimeProvider: configuration.RuntimeProvider, RequestedPrimaryModel: configuration.RequestedPrimaryModel,
		AllowedObservedModels: append([]string(nil), configuration.AllowedObservedModels...), ReasoningEffort: configuration.ReasoningEffort, SeparatePerRunAuthorityRequired: true,
		RuntimeBindingSchema: configuration.RequiredRuntimeBindingSchema, CapabilityScopeSchema: configuration.RequiredCapabilityScopeSchema, DeterministicBrokerRepositoryRead: true,
		DeterministicBrokerAdditiveEvidenceWrite: true, MaxAttemptsPerTask: 1, TaskScopedTemporarySquad: true,
	}
}

func cloneProvisionalRoleConfigurationLifecycleV2(lifecycle ProvisionalRoleConfigurationLifecycleV2) ProvisionalRoleConfigurationLifecycleV2 {
	copy := lifecycle
	copy.Retired = append([]ProvisionalRoleConfigurationTombstoneV2(nil), lifecycle.Retired...)
	copy.Revoked = append([]ProvisionalRoleConfigurationRevocationV2(nil), lifecycle.Revoked...)
	return copy
}

func buildProvisionalRoleConfigurationScenariosV2() ([]provisionalRoleConfigurationScenarioV2, error) {
	configuration, err := CurrentProvisionalRoleConfigurationV2()
	if err != nil {
		return nil, err
	}
	lifecycle, err := CurrentProvisionalRoleConfigurationLifecycleV2()
	if err != nil {
		return nil, err
	}
	exact := exactProvisionalRoleConfigurationRequestV2(configuration)
	inside := time.Date(2026, 9, 15, 13, 30, 0, 0, time.UTC)
	scenarios := []provisionalRoleConfigurationScenarioV2{{id: "exact-reversible-local-default", expected: "ALLOW_PREPARE", reason: "exact_reversible_local_r01_operating_default", request: exact, lifecycle: lifecycle, evaluationTime: inside}}
	add := func(id, reason string, mutate func(*ProvisionalRoleConfigurationRequestV2)) {
		request := exact
		request.RouteStages = append([]string(nil), exact.RouteStages...)
		request.AllowedObservedModels = append([]string(nil), exact.AllowedObservedModels...)
		mutate(&request)
		scenarios = append(scenarios, provisionalRoleConfigurationScenarioV2{id: id, expected: "DENY", reason: reason, request: request, lifecycle: lifecycle, evaluationTime: inside})
	}
	addLifecycle := func(id, expected, reason string, at time.Time, mutate func(*ProvisionalRoleConfigurationLifecycleV2)) {
		state := cloneProvisionalRoleConfigurationLifecycleV2(lifecycle)
		if mutate != nil {
			mutate(&state)
		}
		scenarios = append(scenarios, provisionalRoleConfigurationScenarioV2{id: id, expected: expected, reason: reason, request: exact, lifecycle: state, evaluationTime: at})
	}
	add("configuration-id", "configuration_id_mismatch", func(v *ProvisionalRoleConfigurationRequestV2) { v.ConfigurationID = "other" })
	add("configuration-hash", "configuration_hash_mismatch", func(v *ProvisionalRoleConfigurationRequestV2) { v.ConfigurationSHA256 = string(make([]byte, 64)) })
	addLifecycle("before-valid", "DENY", "configuration_not_yet_valid", time.Date(2026, 9, 15, 13, 14, 59, 0, time.UTC), nil)
	addLifecycle("valid-from-boundary", "ALLOW_PREPARE", "exact_reversible_local_r01_operating_default", time.Date(2026, 9, 15, 13, 15, 0, 0, time.UTC), nil)
	addLifecycle("exact-expiry-boundary", "DENY", "configuration_expired", time.Date(2026, 9, 22, 13, 15, 0, 0, time.UTC), nil)
	addLifecycle("after-expiry", "DENY", "configuration_expired", time.Date(2026, 9, 22, 13, 15, 1, 0, time.UTC), nil)
	addLifecycle("revoked", "DENY", "configuration_revoked", inside, func(v *ProvisionalRoleConfigurationLifecycleV2) {
		v.Revoked = []ProvisionalRoleConfigurationRevocationV2{{ConfigurationID: v.Active.ConfigurationID, ConfigurationSHA256: v.Active.ConfigurationSHA256, RevokedAt: inside.Format(time.RFC3339), Reason: "test_revocation"}}
	})
	addLifecycle("retired", "DENY", "configuration_retired", inside, func(v *ProvisionalRoleConfigurationLifecycleV2) { v.Active.Status = "retired" })
	add("case-family", "case_family_not_qualified", func(v *ProvisionalRoleConfigurationRequestV2) { v.CaseFamily = "R02" })
	add("workflow-kind", "workflow_kind_not_qualified", func(v *ProvisionalRoleConfigurationRequestV2) { v.WorkflowKind = "general_repository_work" })
	add("route-stages", "route_stage_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.RouteStages[0] = "persistent_leader" })
	add("participant", "participant_not_qualified", func(v *ProvisionalRoleConfigurationRequestV2) { v.Participant = "product_ui" })
	add("participant-binding", "participant_binding_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.ParticipantArtifactBindingID = "other" })
	add("inline-skill", "inline_skill_not_qualified", func(v *ProvisionalRoleConfigurationRequestV2) { v.InlineSkillPacket = "product_ui" })
	add("inline-skill-binding", "inline_skill_binding_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.InlineSkillArtifactBindingID = "other" })
	add("intake-role", "intake_role_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.IntakeRole = "persistent_leader" })
	add("reviewer-role", "reviewer_role_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.ReviewerRole = "shared_windows_executor" })
	add("reviewer-binding", "reviewer_binding_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.ReviewerArtifactBindingID = "other" })
	add("reviewer-not-distinct", "reviewer_not_distinct", func(v *ProvisionalRoleConfigurationRequestV2) { v.ReviewerDistinct = false })
	add("review-disabled", "independent_review_required", func(v *ProvisionalRoleConfigurationRequestV2) { v.IndependentReviewRequired = false })
	add("correction", "correction_not_qualified", func(v *ProvisionalRoleConfigurationRequestV2) { v.MaximumCorrections = 1 })
	add("task-count", "provider_task_ceiling_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.MaximumProviderTasks = 4 })
	add("execution-mode", "execution_mode_not_qualified", func(v *ProvisionalRoleConfigurationRequestV2) { v.ExecutionMode = "automatic" })
	add("transport-lane", "transport_lane_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.TransportLane = "codex_subscription" })
	add("runtime-provider", "runtime_provider_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.RuntimeProvider = "openrouter" })
	add("requested-model", "requested_model_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.RequestedPrimaryModel = "claude-opus-4-1" })
	add("observed-models", "observed_model_set_drift", func(v *ProvisionalRoleConfigurationRequestV2) {
		v.AllowedObservedModels = append(v.AllowedObservedModels, "claude-opus-4-1")
	})
	add("reasoning-effort", "reasoning_effort_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.ReasoningEffort = "high" })
	add("exact-model-claim", "exact_single_model_claim_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.ExactSingleModelClaim = true })
	add("per-run-authority", "separate_per_run_authority_required", func(v *ProvisionalRoleConfigurationRequestV2) { v.SeparatePerRunAuthorityRequired = false })
	add("runtime-schema", "runtime_binding_schema_drift", func(v *ProvisionalRoleConfigurationRequestV2) {
		v.RuntimeBindingSchema = "silver-mediums.qualified-runtime-binding.v1"
	})
	add("capability-schema", "capability_scope_schema_drift", func(v *ProvisionalRoleConfigurationRequestV2) {
		v.CapabilityScopeSchema = "silver-mediums.task-capability-scope.v2"
	})
	add("provider-call", "configuration_does_not_authorize_provider_call", func(v *ProvisionalRoleConfigurationRequestV2) { v.ProviderCalls = 1 })
	add("credential-read", "configuration_does_not_authorize_credential_read", func(v *ProvisionalRoleConfigurationRequestV2) { v.CredentialReads = 1 })
	add("provider-child-tool", "provider_child_tools_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.ProviderChildToolExecutions = 1 })
	add("provider-child-repository", "provider_child_repository_access_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.ProviderChildRepositoryAccesses = 1 })
	add("broker-read", "broker_repository_read_required", func(v *ProvisionalRoleConfigurationRequestV2) { v.DeterministicBrokerRepositoryRead = false })
	add("broker-write", "broker_additive_evidence_write_required", func(v *ProvisionalRoleConfigurationRequestV2) { v.DeterministicBrokerAdditiveEvidenceWrite = false })
	add("existing-mutation", "existing_file_mutation_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.ExistingFileMutation = true })
	add("automatic-dispatch", "automatic_dispatch_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.AutomaticDispatch = true })
	add("attempts", "max_attempts_drift", func(v *ProvisionalRoleConfigurationRequestV2) { v.MaxAttemptsPerTask = 2 })
	add("retry", "retries_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.Retries = 1 })
	add("fallback", "fallbacks_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.Fallbacks = true })
	add("persistent-squad", "persistent_squad_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.TaskScopedTemporarySquad = false })
	add("permanent-seat", "permanent_role_or_seat_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.PermanentRoleOrSeatSelection = true })
	add("permanent-squad", "permanent_squad_activation_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.PermanentSquadActivation = true })
	add("production", "production_access_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.ProductionAccesses = 1 })
	add("outbound", "outbound_actions_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.OutboundActions = 1 })
	add("phone-farm", "phone_farm_windows_access_forbidden", func(v *ProvisionalRoleConfigurationRequestV2) { v.PhoneFarmWindowsAccesses = 1 })
	return scenarios, nil
}

func canonicalSHA256V2(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func expectedProvisionalRoleConfigurationReceiptsV2(scenarios []provisionalRoleConfigurationScenarioV2) ([]ProvisionalRoleConfigurationRehearsalReceiptV2, error) {
	receipts := make([]ProvisionalRoleConfigurationRehearsalReceiptV2, 0, len(scenarios))
	for index, candidate := range scenarios {
		decision, reason := EvaluateProvisionalRoleConfigurationRequestV2At(candidate.request, candidate.lifecycle, candidate.evaluationTime)
		requestHash, err := canonicalSHA256V2(candidate.request)
		if err != nil {
			return nil, err
		}
		lifecycleHash, err := canonicalSHA256V2(candidate.lifecycle)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, ProvisionalRoleConfigurationRehearsalReceiptV2{
			Ordinal: index + 1, ScenarioID: candidate.id, ScenarioNonce: fmt.Sprintf("item14-provisional-v2-%02d", index+1), ConfigurationSHA256: candidate.request.ConfigurationSHA256,
			RequestSHA256: requestHash, LifecycleSHA256: lifecycleHash, EvaluationTime: candidate.evaluationTime.UTC().Format(time.RFC3339), ExpectedDecision: candidate.expected,
			ActualDecision: decision, Reason: reason, Matched: decision == candidate.expected && reason == candidate.reason,
		})
	}
	return receipts, nil
}

func RunProvisionalRoleConfigurationRehearsalV2() (ProvisionalRoleConfigurationRehearsalResultV2, []ProvisionalRoleConfigurationRehearsalReceiptV2, error) {
	scenarios, err := buildProvisionalRoleConfigurationScenariosV2()
	if err != nil {
		return ProvisionalRoleConfigurationRehearsalResultV2{}, nil, err
	}
	receipts, err := expectedProvisionalRoleConfigurationReceiptsV2(scenarios)
	if err != nil {
		return ProvisionalRoleConfigurationRehearsalResultV2{}, nil, err
	}
	allowed, denied, mismatches := 0, 0, 0
	for _, receipt := range receipts {
		if receipt.ActualDecision == "ALLOW_PREPARE" {
			allowed++
		} else {
			denied++
		}
		if !receipt.Matched {
			mismatches++
		}
	}
	trace, err := canonicalSHA256V2(receipts)
	if err != nil {
		return ProvisionalRoleConfigurationRehearsalResultV2{}, nil, err
	}
	result := ProvisionalRoleConfigurationRehearsalResultV2{
		SchemaVersion: ProvisionalRoleConfigurationRehearsalV2Schema, RunID: "item-14-provisional-role-configuration-v2-rehearsal-20260915",
		ExecutionMode: "deterministic_provider_free_local_operating_boundary_rehearsal", ConfigurationSHA256: ProvisionalRoleConfigurationV2SHA256(),
		LifecycleSHA256: ProvisionalRoleConfigurationLifecycleV2SHA256(), TraceSHA256: trace, Scenarios: len(scenarios), AllowedPrepare: allowed, Denied: denied,
		Mismatches: mismatches, ProvisionalDefaults: 1, Valid: mismatches == 0,
	}
	if err := ValidateProvisionalRoleConfigurationRehearsalV2(result, receipts); err != nil {
		return result, receipts, err
	}
	return result, receipts, nil
}

func ValidateProvisionalRoleConfigurationRehearsalV2(result ProvisionalRoleConfigurationRehearsalResultV2, receipts []ProvisionalRoleConfigurationRehearsalReceiptV2) error {
	if result.SchemaVersion != ProvisionalRoleConfigurationRehearsalV2Schema || result.RunID != "item-14-provisional-role-configuration-v2-rehearsal-20260915" || result.ExecutionMode != "deterministic_provider_free_local_operating_boundary_rehearsal" || result.ConfigurationSHA256 != ProvisionalRoleConfigurationV2SHA256() || result.LifecycleSHA256 != ProvisionalRoleConfigurationLifecycleV2SHA256() || result.Scenarios != 50 || result.AllowedPrepare != 2 || result.Denied != 48 || result.Mismatches != 0 || result.ProvisionalDefaults != 1 || !result.Valid || len(receipts) != result.Scenarios {
		return fmt.Errorf("provisional role configuration rehearsal v2 aggregate drifted")
	}
	if result.ProviderCalls != 0 || result.CredentialReads != 0 || result.NativeToolExecutions != 0 || result.ProviderChildRepositoryAccesses != 0 || result.ProductionAccesses != 0 || result.OutboundActions != 0 || result.PhoneFarmWindowsAccesses != 0 || result.PermanentSelections != 0 {
		return fmt.Errorf("provisional role configuration rehearsal v2 crossed a forbidden boundary")
	}
	scenarios, err := buildProvisionalRoleConfigurationScenariosV2()
	if err != nil {
		return err
	}
	expected, err := expectedProvisionalRoleConfigurationReceiptsV2(scenarios)
	if err != nil {
		return err
	}
	allowed, denied, mismatches := 0, 0, 0
	for index, receipt := range receipts {
		if receipt != expected[index] || receipt.SideEffects != 0 {
			return fmt.Errorf("provisional role configuration rehearsal v2 receipt %d drifted", index+1)
		}
		if receipt.ActualDecision == "ALLOW_PREPARE" {
			allowed++
		} else {
			denied++
		}
		if !receipt.Matched {
			mismatches++
		}
	}
	if allowed != result.AllowedPrepare || denied != result.Denied || mismatches != result.Mismatches {
		return fmt.Errorf("provisional role configuration rehearsal v2 recomputed aggregate drifted")
	}
	trace, err := canonicalSHA256V2(receipts)
	if err != nil {
		return err
	}
	if result.TraceSHA256 != trace {
		return fmt.Errorf("provisional role configuration rehearsal trace drifted")
	}
	return nil
}
