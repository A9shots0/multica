package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	PrimarySubscriptionRoleTrialV1Schema         = "silver-mediums.primary-subscription-role-trial.v1"
	PrimarySubscriptionReliabilityTrialV2Schema  = "silver-mediums.primary-subscription-reliability-trial.v2"
	primarySubscriptionReliabilityTrialV2GateSHA = "47bafd3f91b1c63205cee15d055b9618b7611425ae62972fd3cd85d217cc87aa"
)

type PrimarySubscriptionTrialLaneV1 struct {
	Lane                     string   `json:"lane"`
	RequestedModel           string   `json:"requested_model"`
	Effort                   string   `json:"effort"`
	Transport                string   `json:"transport"`
	AcceptedModelAttribution string   `json:"accepted_model_attribution"`
	AllowedObservedModels    []string `json:"allowed_observed_models"`
}

type PrimarySubscriptionTrialInvocationV1 struct {
	Ordinal           int    `json:"ordinal"`
	ObservationID     string `json:"observation_id"`
	Lane              string `json:"lane"`
	CaseID            string `json:"case_id"`
	Arm               string `json:"arm"`
	Participant       string `json:"participant"`
	ParticipantKind   string `json:"participant_kind"`
	InlineSkillPacket string `json:"inline_skill_packet,omitempty"`
}

type PrimarySubscriptionRoleTrialV1 struct {
	SchemaVersion                         string                                 `json:"schema_version"`
	TrialID                               string                                 `json:"trial_id"`
	Status                                string                                 `json:"status"`
	GateSHA256                            string                                 `json:"gate_sha256"`
	PriorTrialResultSHA256                string                                 `json:"prior_trial_result_sha256,omitempty"`
	OwnerAuthorization                    string                                 `json:"owner_authorization"`
	Purpose                               string                                 `json:"purpose"`
	RepetitionsPerCell                    int                                    `json:"repetitions_per_cell,omitempty"`
	MaximumCLISessions                    int                                    `json:"maximum_cli_sessions"`
	MaximumSessionsPerInvocation          int                                    `json:"maximum_sessions_per_invocation"`
	MaximumActionsPerExecutor             int                                    `json:"maximum_actions_per_executor"`
	MaximumTotalReportedTokensPerSession  int64                                  `json:"maximum_total_reported_tokens_per_session"`
	MaximumGeneratedTokensPerSession      int64                                  `json:"maximum_generated_tokens_per_session"`
	Retries                               int                                    `json:"retries"`
	Fallbacks                             bool                                   `json:"fallbacks"`
	Redirects                             bool                                   `json:"redirects"`
	Relaunches                            int                                    `json:"relaunches"`
	AutomaticCorrectionReplays            int                                    `json:"automatic_correction_replays"`
	ProviderNativeToolsAllowed            bool                                   `json:"provider_native_tools_allowed"`
	ControllerMediatedActionsOnly         bool                                   `json:"controller_mediated_actions_only"`
	FreshDisposableFixturePerObservation  bool                                   `json:"fresh_disposable_fixture_per_observation"`
	FreshDisposableProviderWorkdirSession bool                                   `json:"fresh_disposable_provider_workdir_per_session"`
	FixtureCleanupRequired                bool                                   `json:"fixture_cleanup_required"`
	ProviderWorkdirCleanupRequired        bool                                   `json:"provider_workdir_cleanup_required"`
	IndependentVerdict                    string                                 `json:"independent_verdict"`
	R02BrowserBoundary                    string                                 `json:"r02_browser_boundary"`
	PermanentRoleOrModelSelectionAllowed  bool                                   `json:"permanent_role_or_model_selection_allowed"`
	ProvisionalLocalSelectionAfterReview  bool                                   `json:"provisional_local_configuration_selection_after_review_allowed,omitempty"`
	ProductionActivationAllowed           bool                                   `json:"production_activation_allowed,omitempty"`
	Lanes                                 []PrimarySubscriptionTrialLaneV1       `json:"lanes"`
	Invocations                           []PrimarySubscriptionTrialInvocationV1 `json:"invocations"`
	AlwaysForbidden                       []string                               `json:"always_forbidden"`
}

//go:embed primary_subscription_role_trial_v1.json
var primarySubscriptionRoleTrialV1JSON []byte

//go:embed primary_subscription_reliability_trial_v2.json
var primarySubscriptionReliabilityTrialV2JSON []byte

//go:embed role_artifacts/v1/product_ui.instructions.md role_artifacts/v1/product_ui.skill.md role_artifacts/v1/shared_windows_executor.instructions.md role_artifacts/v1/shared_windows_executor.skill.md role_artifacts/v2/architecture_assurance.instructions.md role_artifacts/v2/architecture_assurance.skill.md
var primarySubscriptionRoleTrialArtifactsV1 embed.FS

func (ctxContracts QualificationContracts) CurrentPrimarySubscriptionRoleTrialV1() (PrimarySubscriptionRoleTrialV1, error) {
	var definition PrimarySubscriptionRoleTrialV1
	if err := json.Unmarshal(primarySubscriptionRoleTrialV1JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode primary subscription role trial v1: %w", err)
	}
	if err := ctxContracts.ValidatePrimarySubscriptionRoleTrialV1(definition); err != nil {
		return PrimarySubscriptionRoleTrialV1{}, err
	}
	return definition, nil
}

func PrimarySubscriptionRoleTrialV1SHA256() string {
	sum := sha256.Sum256(primarySubscriptionRoleTrialV1JSON)
	return hex.EncodeToString(sum[:])
}

func CurrentPrimarySubscriptionReliabilityTrialV2() (PrimarySubscriptionRoleTrialV1, error) {
	var definition PrimarySubscriptionRoleTrialV1
	if err := strictJSON(primarySubscriptionReliabilityTrialV2JSON, &definition); err != nil {
		return definition, fmt.Errorf("decode primary subscription reliability trial v2: %w", err)
	}
	if err := ValidatePrimarySubscriptionReliabilityTrialV2(definition); err != nil {
		return PrimarySubscriptionRoleTrialV1{}, err
	}
	return definition, nil
}

func PrimarySubscriptionReliabilityTrialV2SHA256() string {
	sum := sha256.Sum256(primarySubscriptionReliabilityTrialV2JSON)
	return hex.EncodeToString(sum[:])
}

func (ctxContracts QualificationContracts) ValidatePrimarySubscriptionRoleTrialV1(definition PrimarySubscriptionRoleTrialV1) error {
	if definition.SchemaVersion != PrimarySubscriptionRoleTrialV1Schema || definition.TrialID != "item-14-primary-subscription-role-trial-20260914" || definition.Status != "frozen_owner_authorized_not_executed" {
		return fmt.Errorf("primary subscription role trial identity drifted")
	}
	if definition.GateSHA256 != PrimarySubscriptionRoleTrialGateV1SHA256() || definition.PriorTrialResultSHA256 != "" || definition.RepetitionsPerCell != 0 || definition.ProvisionalLocalSelectionAfterReview || definition.ProductionActivationAllowed || !strings.HasPrefix(definition.OwnerAuthorization, "Authorize the frozen 12-session") {
		return fmt.Errorf("primary subscription role trial authority binding drifted")
	}
	if definition.MaximumCLISessions != 12 || definition.MaximumSessionsPerInvocation != 1 || definition.MaximumActionsPerExecutor != 10 || definition.MaximumTotalReportedTokensPerSession != 75000 || definition.MaximumGeneratedTokensPerSession != 20000 || definition.Retries != 0 || definition.Fallbacks || definition.Redirects || definition.Relaunches != 0 || definition.AutomaticCorrectionReplays != 0 {
		return fmt.Errorf("primary subscription role trial ceilings drifted")
	}
	if definition.ProviderNativeToolsAllowed || !definition.ControllerMediatedActionsOnly || !definition.FreshDisposableFixturePerObservation || !definition.FreshDisposableProviderWorkdirSession || !definition.FixtureCleanupRequired || !definition.ProviderWorkdirCleanupRequired || definition.IndependentVerdict != "provider_free_hidden_verifier" || definition.R02BrowserBoundary != "controller_owned_chatgpt_in_app_browser_on_127_0_0_1_only" || definition.PermanentRoleOrModelSelectionAllowed {
		return fmt.Errorf("primary subscription role trial safety boundary drifted")
	}
	wantLanes := []PrimarySubscriptionTrialLaneV1{
		{Lane: "claude_code_subscription", RequestedModel: "claude-sonnet-4-6", Effort: "medium", Transport: "claude_code_restricted_json_subscription", AcceptedModelAttribution: "exact_requested_sonnet_primary_plus_disclosed_background_haiku_only", AllowedObservedModels: []string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"}},
		{Lane: "codex_subscription", RequestedModel: "gpt-5.6-luna", Effort: "medium", Transport: "codex_exec_ephemeral_json_subscription", AcceptedModelAttribution: "requested_model_pinned_cli_receipt_identity_may_be_unavailable_truthful", AllowedObservedModels: []string{"gpt-5.6-luna"}},
	}
	if !slices.EqualFunc(definition.Lanes, wantLanes, func(a, b PrimarySubscriptionTrialLaneV1) bool {
		return a.Lane == b.Lane && a.RequestedModel == b.RequestedModel && a.Effort == b.Effort && a.Transport == b.Transport && a.AcceptedModelAttribution == b.AcceptedModelAttribution && slices.Equal(a.AllowedObservedModels, b.AllowedObservedModels)
	}) {
		return fmt.Errorf("primary subscription role trial lanes drifted")
	}
	gate, err := ctxContracts.CurrentPrimarySubscriptionRoleTrialGateV1()
	if err != nil {
		return err
	}
	wantInvocations := []PrimarySubscriptionTrialInvocationV1{}
	ordinal := 0
	for _, observation := range gate.ProposedTrial.InvocationPlan {
		for _, participant := range observation.Participants {
			ordinal++
			kind := "executor"
			if participant != "shared_windows_executor" {
				kind = "specialist"
			}
			inline := ""
			if kind == "executor" && len(observation.InlineSkillPackets) == 1 {
				inline = observation.InlineSkillPackets[0]
			}
			wantInvocations = append(wantInvocations, PrimarySubscriptionTrialInvocationV1{Ordinal: ordinal, ObservationID: observation.ObservationID, Lane: observation.Lane, CaseID: observation.CaseID, Arm: observation.Arm, Participant: participant, ParticipantKind: kind, InlineSkillPacket: inline})
		}
	}
	if len(definition.Invocations) != definition.MaximumCLISessions || !slices.Equal(definition.Invocations, wantInvocations) {
		return fmt.Errorf("primary subscription role trial invocation plan drifted")
	}
	for _, required := range []string{"real_repository_access_by_participant", "provider_native_tool_execution", "production_access", "business_system_access", "non_subscription_credentials", "non_provider_network", "outbound_business_action", "phone_farm_access", "browser_except_controller_owned_in_app_local_r02", "deployment", "automatic_replay", "permanent_role_or_model_selection"} {
		if !slices.Contains(definition.AlwaysForbidden, required) {
			return fmt.Errorf("primary subscription role trial prohibition %s missing", required)
		}
	}
	return nil
}

func ValidatePrimarySubscriptionReliabilityTrialV2(definition PrimarySubscriptionRoleTrialV1) error {
	if definition.SchemaVersion != PrimarySubscriptionReliabilityTrialV2Schema || definition.TrialID != "item-14-primary-subscription-reliability-trial-v2-20260914" || definition.Status != "frozen_owner_authorized_not_executed" {
		return fmt.Errorf("primary subscription reliability trial v2 identity drifted")
	}
	if definition.GateSHA256 != primarySubscriptionReliabilityTrialV2GateSHA || definition.PriorTrialResultSHA256 != PrimarySubscriptionRoleTrialResultV1SHA256() || !strings.HasPrefix(definition.OwnerAuthorization, "Authorize one fresh capped primary-subscription reliability trial") {
		return fmt.Errorf("primary subscription reliability trial v2 authority binding drifted")
	}
	if definition.RepetitionsPerCell != 3 || definition.MaximumCLISessions != 12 || definition.MaximumSessionsPerInvocation != 1 || definition.MaximumActionsPerExecutor != 10 || definition.MaximumTotalReportedTokensPerSession != 75000 || definition.MaximumGeneratedTokensPerSession != 20000 || definition.Retries != 0 || definition.Fallbacks || definition.Redirects || definition.Relaunches != 0 || definition.AutomaticCorrectionReplays != 0 {
		return fmt.Errorf("primary subscription reliability trial v2 ceilings drifted")
	}
	if definition.ProviderNativeToolsAllowed || !definition.ControllerMediatedActionsOnly || !definition.FreshDisposableFixturePerObservation || !definition.FreshDisposableProviderWorkdirSession || !definition.FixtureCleanupRequired || !definition.ProviderWorkdirCleanupRequired || definition.IndependentVerdict != "provider_free_hidden_verifier" || definition.R02BrowserBoundary != "controller_owned_chatgpt_in_app_browser_on_127_0_0_1_only" || definition.PermanentRoleOrModelSelectionAllowed || !definition.ProvisionalLocalSelectionAfterReview || definition.ProductionActivationAllowed {
		return fmt.Errorf("primary subscription reliability trial v2 safety boundary drifted")
	}
	wantLanes := primarySubscriptionTrialLanesV1()
	if !slices.EqualFunc(definition.Lanes, wantLanes, func(a, b PrimarySubscriptionTrialLaneV1) bool {
		return a.Lane == b.Lane && a.RequestedModel == b.RequestedModel && a.Effort == b.Effort && a.Transport == b.Transport && a.AcceptedModelAttribution == b.AcceptedModelAttribution && slices.Equal(a.AllowedObservedModels, b.AllowedObservedModels)
	}) {
		return fmt.Errorf("primary subscription reliability trial v2 lanes drifted")
	}
	wantInvocations := []PrimarySubscriptionTrialInvocationV1{
		{Ordinal: 1, ObservationID: "reliability-v2-r1-claude-r01-inline", Lane: "claude_code_subscription", CaseID: "R01", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "architecture_assurance"},
		{Ordinal: 2, ObservationID: "reliability-v2-r1-codex-r02-inline", Lane: "codex_subscription", CaseID: "R02", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "product_ui"},
		{Ordinal: 3, ObservationID: "reliability-v2-r1-claude-r02-inline", Lane: "claude_code_subscription", CaseID: "R02", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "product_ui"},
		{Ordinal: 4, ObservationID: "reliability-v2-r1-codex-r01-inline", Lane: "codex_subscription", CaseID: "R01", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "architecture_assurance"},
		{Ordinal: 5, ObservationID: "reliability-v2-r2-codex-r01-inline", Lane: "codex_subscription", CaseID: "R01", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "architecture_assurance"},
		{Ordinal: 6, ObservationID: "reliability-v2-r2-claude-r02-inline", Lane: "claude_code_subscription", CaseID: "R02", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "product_ui"},
		{Ordinal: 7, ObservationID: "reliability-v2-r2-codex-r02-inline", Lane: "codex_subscription", CaseID: "R02", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "product_ui"},
		{Ordinal: 8, ObservationID: "reliability-v2-r2-claude-r01-inline", Lane: "claude_code_subscription", CaseID: "R01", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "architecture_assurance"},
		{Ordinal: 9, ObservationID: "reliability-v2-r3-claude-r02-inline", Lane: "claude_code_subscription", CaseID: "R02", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "product_ui"},
		{Ordinal: 10, ObservationID: "reliability-v2-r3-codex-r01-inline", Lane: "codex_subscription", CaseID: "R01", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "architecture_assurance"},
		{Ordinal: 11, ObservationID: "reliability-v2-r3-claude-r01-inline", Lane: "claude_code_subscription", CaseID: "R01", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "architecture_assurance"},
		{Ordinal: 12, ObservationID: "reliability-v2-r3-codex-r02-inline", Lane: "codex_subscription", CaseID: "R02", Arm: "shared_executor_with_exact_skill_packet", Participant: "shared_windows_executor", ParticipantKind: "executor", InlineSkillPacket: "product_ui"},
	}
	if len(definition.Invocations) != definition.MaximumCLISessions || !slices.Equal(definition.Invocations, wantInvocations) {
		return fmt.Errorf("primary subscription reliability trial v2 invocation plan drifted")
	}
	for _, required := range []string{"real_repository_access_by_participant", "provider_native_tool_execution", "production_access", "business_system_access", "non_subscription_credentials", "non_provider_network", "outbound_business_action", "phone_farm_access", "browser_except_controller_owned_in_app_local_r02", "deployment", "automatic_replay", "reopen_or_rewrite_primary_subscription_trial_v1", "openrouter_access", "direct_metered_api", "permanent_role_or_model_selection"} {
		if !slices.Contains(definition.AlwaysForbidden, required) {
			return fmt.Errorf("primary subscription reliability trial v2 prohibition %s missing", required)
		}
	}
	return nil
}

func primarySubscriptionTrialLanesV1() []PrimarySubscriptionTrialLaneV1 {
	return []PrimarySubscriptionTrialLaneV1{
		{Lane: "claude_code_subscription", RequestedModel: "claude-sonnet-4-6", Effort: "medium", Transport: "claude_code_restricted_json_subscription", AcceptedModelAttribution: "exact_requested_sonnet_primary_plus_disclosed_background_haiku_only", AllowedObservedModels: []string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"}},
		{Lane: "codex_subscription", RequestedModel: "gpt-5.6-luna", Effort: "medium", Transport: "codex_exec_ephemeral_json_subscription", AcceptedModelAttribution: "requested_model_pinned_cli_receipt_identity_may_be_unavailable_truthful", AllowedObservedModels: []string{"gpt-5.6-luna"}},
	}
}

type PrimarySubscriptionTrialParticipantOutputV1 struct {
	Actions  []RealisticCandidateAction `json:"actions,omitempty"`
	Evidence json.RawMessage            `json:"evidence,omitempty"`
}

func (ctxContracts QualificationContracts) BuildPrimarySubscriptionTrialPromptV1(invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace, priorSpecialist json.RawMessage) (string, []byte, error) {
	definition, err := ctxContracts.CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		return "", nil, err
	}
	return buildPrimarySubscriptionTrialPrompt(definition, invocation, workspace, priorSpecialist)
}

func BuildPrimarySubscriptionReliabilityTrialPromptV2(invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace) (string, []byte, error) {
	definition, err := CurrentPrimarySubscriptionReliabilityTrialV2()
	if err != nil {
		return "", nil, err
	}
	return buildPrimarySubscriptionTrialPrompt(definition, invocation, workspace, nil)
}

func buildPrimarySubscriptionTrialPrompt(definition PrimarySubscriptionRoleTrialV1, invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace, priorSpecialist json.RawMessage) (string, []byte, error) {
	paths, err := workspace.Paths()
	if err != nil {
		return "", nil, err
	}
	files := map[string]string{}
	for _, path := range paths {
		raw, readErr := workspace.Read(path)
		if readErr != nil {
			return "", nil, readErr
		}
		files[path] = string(raw)
	}
	artifactText, err := primarySubscriptionParticipantArtifacts(invocation)
	if err != nil {
		return "", nil, err
	}
	packet := map[string]any{
		"trial_id": definition.TrialID, "observation_id": invocation.ObservationID, "case_id": invocation.CaseID,
		"arm": invocation.Arm, "participant": invocation.Participant, "participant_kind": invocation.ParticipantKind,
		"fixture_files": files, "editable_paths": workspace.Fixture.EditablePaths, "allowlisted_checks": workspace.Fixture.AllowlistedChecks,
		"provider_native_tools": "forbidden", "controller_mediated_actions_only": true, "real_repository": "not supplied",
		"production": "forbidden", "phone_farm": "forbidden", "retries": 0,
	}
	if len(priorSpecialist) != 0 {
		packet["specialist_evidence"] = json.RawMessage(priorSpecialist)
	}
	packetRaw, err := json.Marshal(packet)
	if err != nil {
		return "", nil, err
	}
	schema, err := primarySubscriptionTrialResponseSchema(definition, invocation)
	if err != nil {
		return "", nil, err
	}
	var instruction string
	if invocation.ParticipantKind == "specialist" {
		instruction = "Return one compact evidence object only. Do not propose or perform tool calls, edits, network access, repository access, Phone Farm access, deployment, or business actions."
	} else {
		instruction = "Return one complete bounded action plan. You have no tools and perform no actions. The deterministic controller will execute the actions once, in order, only inside the supplied disposable fixture. Include a meaningful final finish action. For R01, run the allowlisted check before editing and again after the repair, change only the faulty consumer plus permitted regression/evidence files, and preserve all authority and Phone Farm contract fixtures. For R02, repair the truthful four-card total, run both allowlisted checks, request Browser exactly once, and then finish; the controller will materialize the three Browser proof hashes after capture. Do not request shell, arbitrary commands, provider-native tools, network, deployment, messages, credentials, Phone Farm, external effects, undisclosed paths, or another provider turn."
	}
	prompt := "This is a frozen Silver Mediums synthetic role trial. Follow the immutable participant artifacts and the controller packet. " + instruction + "\n\n<participant-artifacts>\n" + artifactText + "\n</participant-artifacts>\n\n<response-schema>\n" + string(schema) + "\n</response-schema>\n\n<controller-packet>\n" + string(packetRaw) + "\n</controller-packet>\n"
	return prompt, schema, nil
}

func (ctxContracts QualificationContracts) PrimarySubscriptionTrialResponseSchemaV1(invocation PrimarySubscriptionTrialInvocationV1) ([]byte, error) {
	definition, err := ctxContracts.CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		return nil, err
	}
	return primarySubscriptionTrialResponseSchema(definition, invocation)
}

func PrimarySubscriptionReliabilityTrialResponseSchemaV2(invocation PrimarySubscriptionTrialInvocationV1) ([]byte, error) {
	definition, err := CurrentPrimarySubscriptionReliabilityTrialV2()
	if err != nil {
		return nil, err
	}
	return primarySubscriptionTrialResponseSchema(definition, invocation)
}

func primarySubscriptionTrialResponseSchema(definition PrimarySubscriptionRoleTrialV1, invocation PrimarySubscriptionTrialInvocationV1) ([]byte, error) {
	switch invocation.ParticipantKind {
	case "specialist":
		if invocation.Lane == "codex_subscription" {
			return []byte(`{"type":"object","additionalProperties":false,"required":["evidence"],"properties":{"evidence":{"type":"object","additionalProperties":false,"required":["summary","findings","constraints","decision"],"properties":{"summary":{"type":"string"},"findings":{"type":"array","items":{"type":"string"}},"constraints":{"type":"array","items":{"type":"string"}},"decision":{"type":"string"}}}}}`), nil
		}
		return []byte(`{"type":"object","additionalProperties":false,"required":["evidence"],"properties":{"evidence":{"type":"object"}}}`), nil
	case "executor":
		if invocation.Lane == "codex_subscription" {
			return []byte(fmt.Sprintf(`{"type":"object","additionalProperties":false,"required":["actions"],"properties":{"actions":{"type":"array","minItems":1,"maxItems":%d,"items":{"type":"object","additionalProperties":false,"required":["kind","path","query","contents","check","summary"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","browser","finish"]},"path":{"type":["string","null"]},"query":{"type":["string","null"]},"contents":{"type":["string","null"]},"check":{"type":["string","null"]},"summary":{"type":["string","null"]}}}}}}`, definition.MaximumActionsPerExecutor)), nil
		}
		return []byte(fmt.Sprintf(`{"type":"object","additionalProperties":false,"required":["actions"],"properties":{"actions":{"type":"array","minItems":1,"maxItems":%d,"items":{"type":"object","additionalProperties":false,"required":["kind"],"properties":{"kind":{"type":"string","enum":["read","search","edit","run_check","browser","finish"]},"path":{"type":"string"},"query":{"type":"string"},"contents":{"type":"string"},"check":{"type":"string"},"summary":{"type":"string"}}}}}}`, definition.MaximumActionsPerExecutor)), nil
	default:
		return nil, fmt.Errorf("unsupported role trial participant kind %s", invocation.ParticipantKind)
	}
}

func primarySubscriptionParticipantArtifacts(invocation PrimarySubscriptionTrialInvocationV1) (string, error) {
	paths := []string{}
	switch invocation.Participant {
	case "architecture_assurance":
		paths = []string{"role_artifacts/v2/architecture_assurance.instructions.md", "role_artifacts/v2/architecture_assurance.skill.md"}
	case "product_ui":
		paths = []string{"role_artifacts/v1/product_ui.instructions.md", "role_artifacts/v1/product_ui.skill.md"}
	case "shared_windows_executor":
		paths = []string{"role_artifacts/v1/shared_windows_executor.instructions.md", "role_artifacts/v1/shared_windows_executor.skill.md"}
		if invocation.InlineSkillPacket == "architecture_assurance" {
			paths = append(paths, "role_artifacts/v2/architecture_assurance.skill.md")
		}
		if invocation.InlineSkillPacket == "product_ui" {
			paths = append(paths, "role_artifacts/v1/product_ui.skill.md")
		}
	default:
		return "", fmt.Errorf("unsupported role trial participant %s", invocation.Participant)
	}
	parts := []string{}
	for _, relative := range paths {
		raw, err := primarySubscriptionRoleTrialArtifactsV1.ReadFile(filepath.ToSlash(relative))
		if err != nil {
			return "", err
		}
		parts = append(parts, "<artifact path=\""+relative+"\">\n"+string(raw)+"\n</artifact>")
	}
	return strings.Join(parts, "\n"), nil
}

func (ctxContracts QualificationContracts) ParsePrimarySubscriptionTrialParticipantOutputV1(raw []byte, invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace) (PrimarySubscriptionTrialParticipantOutputV1, error) {
	definition, err := ctxContracts.CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		return PrimarySubscriptionTrialParticipantOutputV1{}, err
	}
	return parsePrimarySubscriptionTrialParticipantOutput(definition, raw, invocation, workspace)
}

func ParsePrimarySubscriptionReliabilityTrialParticipantOutputV2(raw []byte, invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace) (PrimarySubscriptionTrialParticipantOutputV1, error) {
	definition, err := CurrentPrimarySubscriptionReliabilityTrialV2()
	if err != nil {
		return PrimarySubscriptionTrialParticipantOutputV1{}, err
	}
	return parsePrimarySubscriptionTrialParticipantOutput(definition, raw, invocation, workspace)
}

func parsePrimarySubscriptionTrialParticipantOutput(definition PrimarySubscriptionRoleTrialV1, raw []byte, invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace) (PrimarySubscriptionTrialParticipantOutputV1, error) {
	var output PrimarySubscriptionTrialParticipantOutputV1
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&output); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return output, fmt.Errorf("role trial participant output schema mismatch")
	}
	if invocation.ParticipantKind == "specialist" {
		if len(output.Actions) != 0 || len(output.Evidence) == 0 || string(output.Evidence) == "null" || !json.Valid(output.Evidence) {
			return output, fmt.Errorf("role trial specialist evidence is incomplete")
		}
		return output, nil
	}
	if len(output.Evidence) != 0 || len(output.Actions) < 1 || len(output.Actions) > definition.MaximumActionsPerExecutor {
		return output, fmt.Errorf("role trial executor action count invalid")
	}
	paths, err := workspace.Paths()
	if err != nil {
		return output, err
	}
	pathSet := stringSet(paths...)
	finishCount, browserCount, precheck, postcheck, firstEdit := 0, 0, -1, -1, -1
	for index, action := range output.Actions {
		switch action.Kind {
		case "read":
			if _, ok := pathSet[filepath.ToSlash(action.Path)]; !ok {
				return output, fmt.Errorf("role trial read path not supplied")
			}
		case "search":
			if strings.TrimSpace(action.Query) == "" {
				return output, fmt.Errorf("role trial search query missing")
			}
		case "edit":
			if !slices.Contains(workspace.Fixture.EditablePaths, filepath.ToSlash(action.Path)) {
				return output, fmt.Errorf("role trial edit path not allowlisted")
			}
			if firstEdit == -1 {
				firstEdit = index
			}
		case "run_check":
			if !slices.Contains(workspace.Fixture.AllowlistedChecks, action.Check) {
				return output, fmt.Errorf("role trial check not allowlisted")
			}
			if precheck == -1 {
				precheck = index
			}
			postcheck = index
		case "browser":
			browserCount++
			if invocation.CaseID != "R02" {
				return output, fmt.Errorf("role trial Browser action outside R02")
			}
		case "finish":
			finishCount++
			if index != len(output.Actions)-1 || strings.TrimSpace(action.Summary) == "" {
				return output, fmt.Errorf("role trial finish must be final and meaningful")
			}
		default:
			return output, fmt.Errorf("role trial action kind not allowed")
		}
	}
	if finishCount != 1 {
		return output, fmt.Errorf("role trial requires exactly one finish")
	}
	if invocation.CaseID == "R01" && (browserCount != 0 || firstEdit < 0 || precheck < 0 || precheck > firstEdit || postcheck <= firstEdit) {
		return output, fmt.Errorf("R01 baseline or post-change check sequence missing")
	}
	if invocation.CaseID == "R02" {
		for _, check := range workspace.Fixture.AllowlistedChecks {
			seen := false
			for _, action := range output.Actions {
				seen = seen || action.Kind == "run_check" && action.Check == check
			}
			if !seen {
				return output, fmt.Errorf("R02 required check missing: %s", check)
			}
		}
		if browserCount != 1 {
			return output, fmt.Errorf("R02 requires exactly one Browser request")
		}
	}
	return output, nil
}

type PrimarySubscriptionTrialExecutionStateV1 struct {
	NextAction int                       `json:"next_action"`
	Controller RealisticControllerResult `json:"controller"`
}

func ExecutePrimarySubscriptionTrialActionsV1(workspace RealisticCaseWorkspace, runFamilyID string, actions []RealisticCandidateAction, state PrimarySubscriptionTrialExecutionStateV1, browser RealisticBrowserCollector) (PrimarySubscriptionTrialExecutionStateV1, error) {
	if state.Controller.RunFamilyID == "" {
		state.Controller = RealisticControllerResult{CaseID: workspace.CaseID, RunFamilyID: runFamilyID, Attempt: 1, ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact"}
	}
	for state.NextAction < len(actions) {
		action := actions[state.NextAction]
		started := time.Now().UTC()
		var output []byte
		var err error
		var touched, arguments []string
		switch action.Kind {
		case "read":
			output, err = workspace.Read(action.Path)
			arguments = []string{filepath.ToSlash(action.Path)}
		case "search":
			var matches []string
			matches, err = workspace.Search(action.Query)
			output, _ = json.Marshal(matches)
			arguments = []string{action.Query}
		case "edit":
			err = workspace.Edit(action.Path, []byte(action.Contents))
			arguments, touched = []string{filepath.ToSlash(action.Path)}, []string{filepath.ToSlash(action.Path)}
		case "run_check":
			output, err = workspace.RunCheck(action.Check)
			arguments = strings.Fields(action.Check)
		case "browser":
			if browser == nil {
				state.Controller.PendingAction = &RealisticCandidateAction{Kind: "browser"}
				state.Controller.TerminalState = "awaiting_browser_evidence"
				return state, nil
			}
			state.Controller.BrowserProofs, err = browser.Capture(nil, workspace.Root, "report/index.html")
			if err == nil {
				state.Controller.PendingAction = nil
				output = realisticBrowserToolOutput(state.Controller.BrowserProofs)
				evidenceRaw, marshalErr := json.MarshalIndent(map[string]any{"local_check": "node tests/report.test.mjs", "browser_proofs": state.Controller.BrowserProofs, "evidence_lane": "local-only", "release_or_send": false}, "", "  ")
				if marshalErr != nil {
					err = marshalErr
				} else if editErr := workspace.Edit("EVIDENCE.json", evidenceRaw); editErr != nil {
					err = editErr
				} else {
					appendRealisticToolResultAt(&state.Controller, "edit", nil, nil, []string{"EVIDENCE.json"}, []string{"EVIDENCE.json"}, started)
				}
			}
		case "finish":
			state.Controller.FinalSummary = action.Summary
			state.Controller.TerminalState = "completed"
			state.NextAction++
			return state, nil
		}
		appendRealisticToolResultAt(&state.Controller, action.Kind, output, err, touched, arguments, started)
		state.NextAction++
		if err != nil {
			// A failing pre-change R01 baseline is expected and preserved. Any
			// other controller action failure ends the observation without replay.
			hasPriorEdit := false
			for _, prior := range actions[:state.NextAction-1] {
				hasPriorEdit = hasPriorEdit || prior.Kind == "edit"
			}
			if !(workspace.CaseID == "R01" && action.Kind == "run_check" && !hasPriorEdit) {
				state.Controller.TerminalState, state.Controller.FailureClass = "failed", "controller_action_failure"
				return state, err
			}
		}
	}
	state.Controller.TerminalState, state.Controller.FailureClass = "failed", "response_or_tool_protocol_failure"
	return state, fmt.Errorf("role trial plan ended without finish")
}

func PrimarySubscriptionTrialToolReceiptV1(workspace RealisticCaseWorkspace, configurationID string, controller RealisticControllerResult) (RealisticExecutionReceipt, error) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return RealisticExecutionReceipt{}, err
	}
	finalHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return RealisticExecutionReceipt{}, err
	}
	dummyHash := strings.Repeat("a", 64)
	receipt := RealisticExecutionReceipt{SchemaVersion: RealisticExecutionReceiptSchemaVersion, GroupID: group.GroupID, CaseID: workspace.CaseID, CaseVersion: workspace.CaseVersion, ConfigurationID: configurationID, RunFamilyID: controller.RunFamilyID, Attempt: 1, TerminalState: controller.TerminalState, StartingStateSHA256: workspace.StartingTreeSHA256, FinalStateSHA256: finalHash, WorkspaceBoundary: "fresh_disposable_case_workspace", NetworkBoundary: "provider_api_only", ToolEvents: append([]RealisticToolEvent(nil), controller.ToolEvents...), BrowserProofs: append([]RealisticBrowserProof(nil), controller.BrowserProofs...), OwnerInterventions: 0, Retries: 0, ExternalSideEffects: []string{}, ProhibitedEvents: []string{}, ProviderReceiptSHA256: dummyHash, ProviderReceiptArtifactPath: "provider-receipts.json", ProviderReceiptHashes: []string{dummyHash}, ProviderCalls: 1, ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: dummyHash, ControllerResultArtifactPath: "controller-result.json", FixtureTreeSHA256: workspace.Fixture.TreeSHA256, ManifestSHA256: workspace.ManifestSHA256}
	if workspace.CaseID == "R02" {
		receipt.NetworkBoundary = "provider_api_and_case_localhost_only"
	}
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID != workspace.CaseID {
			continue
		}
		for _, evidenceID := range candidateCase.RequiredEvidence {
			if _, supported, deriveErr := deriveRealisticEvidenceIndex(workspace.CaseID, evidenceID, controller.RunFamilyID, finalHash, workspace.InitialCopy, workspace.Root, controller.ToolEvents, controller.BrowserProofs, nil); deriveErr != nil {
				return RealisticExecutionReceipt{}, deriveErr
			} else if supported {
				receipt.Evidence = append(receipt.Evidence, RealisticEvidenceReceipt{EvidenceID: evidenceID, Kind: "harness_bound_evidence_index", ArtifactPath: "proofs/" + evidenceID + ".json", SHA256: dummyHash})
			}
		}
	}
	if reasons := ValidateRealisticExecutionReceipt(receipt, group); len(reasons) != 0 {
		return RealisticExecutionReceipt{}, fmt.Errorf("role trial tool receipt invalid: %v", reasons)
	}
	return receipt, nil
}

func GradePrimarySubscriptionTrialWorkspaceV1(workspace RealisticCaseWorkspace, configurationID string, controller RealisticControllerResult) (RealisticWorkspaceGrade, error) {
	receipt, err := PrimarySubscriptionTrialToolReceiptV1(workspace, configurationID, controller)
	if err != nil {
		return RealisticWorkspaceGrade{}, err
	}
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return RealisticWorkspaceGrade{}, err
	}
	return GradeRealisticWorkspace(workspace.CaseID, workspace.Root, RealisticIndependentEvidence{BrowserProofs: controller.BrowserProofs, ToolReceipt: &receipt}, group)
}

func PrimarySubscriptionTrialWorkspaceSnapshotV1(workspace RealisticCaseWorkspace) (map[string]string, error) {
	paths, err := workspace.Paths()
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, path := range paths {
		raw, readErr := workspace.Read(path)
		if readErr != nil {
			return nil, readErr
		}
		result[path] = string(raw)
	}
	return result, nil
}

func PrimarySubscriptionTrialSortedModelsV1(models map[string]json.RawMessage) []string {
	result := []string{}
	for model := range models {
		if strings.TrimSpace(model) != "" {
			result = append(result, model)
		}
	}
	sort.Strings(result)
	return result
}
