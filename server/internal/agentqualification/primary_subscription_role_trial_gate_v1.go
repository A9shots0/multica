package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
)

const PrimarySubscriptionRoleTrialGateSchemaV1 = "silver-mediums.primary-subscription-role-trial-gate.v1"

type PrimarySubscriptionRoleTrialSourceHashesV1 struct {
	RoleSquadOperatingContract string `json:"role_squad_operating_contract"`
	RoleSquadRehearsalResult   string `json:"role_squad_rehearsal_result"`
	RealisticFixtureManifest   string `json:"realistic_fixture_manifest"`
	RoleQualificationCaseGroup string `json:"role_qualification_case_group"`
	CodexSubscriptionBoundary  string `json:"codex_subscription_boundary"`
	TransportCatalog           string `json:"transport_catalog"`
	ClaudeExactModelLimitation string `json:"claude_exact_model_limitation"`
	GuardedChildExecution      string `json:"guarded_child_execution"`
	RealisticFixtureController string `json:"realistic_fixture_controller"`
}

type PrimarySubscriptionRoleTrialObservationV1 struct {
	ObservationID       string   `json:"observation_id"`
	Lane                string   `json:"lane"`
	CaseID              string   `json:"case_id"`
	Arm                 string   `json:"arm"`
	Participants        []string `json:"participants"`
	InlineSkillPackets  []string `json:"inline_skill_packets"`
	ProviderFreeVerdict bool     `json:"provider_free_verdict"`
}

type PrimarySubscriptionRoleTrialProposalV1 struct {
	Purpose                                 string                                      `json:"purpose"`
	WorkflowFixtures                        []string                                    `json:"workflow_fixtures"`
	StructuralArms                          []string                                    `json:"structural_arms"`
	SubscriptionLanes                       []string                                    `json:"subscription_lanes"`
	MaximumCLISessions                      int                                         `json:"maximum_cli_sessions"`
	Retries                                 int                                         `json:"retries"`
	Fallbacks                               bool                                        `json:"fallbacks"`
	Redirects                               bool                                        `json:"redirects"`
	FreshDisposableFixturePerArm            bool                                        `json:"fresh_disposable_fixture_per_arm"`
	TemporaryPrimaryModelBindingRequired    bool                                        `json:"temporary_primary_model_binding_required"`
	ProviderNativeToolsAllowed              bool                                        `json:"provider_native_tools_allowed"`
	ControllerMediatedActionsOnly           bool                                        `json:"controller_mediated_actions_only"`
	IndependentQualityVerdictRequired       bool                                        `json:"independent_quality_verdict_required"`
	IndependentQualityVerdictMode           string                                      `json:"independent_quality_verdict_mode"`
	AutomaticCorrectionReplays              int                                         `json:"automatic_correction_replays"`
	PostBatchProviderFreeCorrectionsMaximum int                                         `json:"post_batch_provider_free_corrections_maximum"`
	AllowedExecutorActionsAfterAuthority    []string                                    `json:"allowed_executor_actions_after_authority"`
	AllowedContributorActionsAfterAuthority []string                                    `json:"allowed_contributor_actions_after_authority"`
	R02BrowserBoundary                      string                                      `json:"r02_browser_boundary"`
	SessionAccountingRule                   string                                      `json:"session_accounting_rule"`
	ClaudeExclusionRule                     string                                      `json:"claude_exclusion_rule"`
	InvocationPlan                          []PrimarySubscriptionRoleTrialObservationV1 `json:"invocation_plan"`
	AlwaysForbidden                         []string                                    `json:"always_forbidden"`
}

type PrimarySubscriptionRoleTrialConstraintV1 struct {
	Lane        string `json:"lane"`
	Fact        string `json:"fact"`
	Consequence string `json:"consequence"`
}

type PrimarySubscriptionRoleTrialGateV1 struct {
	SchemaVersion                        string                                     `json:"schema_version"`
	GateID                               string                                     `json:"gate_id"`
	Status                               string                                     `json:"status"`
	ProviderCallsAllowed                 bool                                       `json:"provider_calls_allowed"`
	CredentialReadsAllowed               bool                                       `json:"credential_reads_allowed"`
	ToolExecutionAllowed                 bool                                       `json:"tool_execution_allowed"`
	FixtureRepositoryAccessAllowed       bool                                       `json:"fixture_repository_access_allowed"`
	ProductionActionsAllowed             bool                                       `json:"production_actions_allowed"`
	PermanentModelOrSeatSelectionAllowed bool                                       `json:"permanent_model_or_seat_selection_allowed"`
	SourceHashes                         PrimarySubscriptionRoleTrialSourceHashesV1 `json:"source_hashes"`
	ProposedTrial                        PrimarySubscriptionRoleTrialProposalV1     `json:"proposed_trial"`
	KnownTransportConstraints            []PrimarySubscriptionRoleTrialConstraintV1 `json:"known_transport_constraints"`
	OwnerDecisionsRequired               []string                                   `json:"owner_decisions_required"`
	AdvancementRule                      string                                     `json:"advancement_rule"`
	RejectionRule                        string                                     `json:"rejection_rule"`
}

//go:embed primary_subscription_role_trial_gate_v1.json
var primarySubscriptionRoleTrialGateV1JSON []byte

//go:embed guarded_child_execution_v1.go
var guardedChildExecutionV1Source []byte

//go:embed realistic_fixture.go
var realisticFixtureControllerSource []byte

func (ctxContracts QualificationContracts) CurrentPrimarySubscriptionRoleTrialGateV1() (PrimarySubscriptionRoleTrialGateV1, error) {
	var gate PrimarySubscriptionRoleTrialGateV1
	if err := strictJSON(primarySubscriptionRoleTrialGateV1JSON, &gate); err != nil {
		return gate, fmt.Errorf("decode primary subscription role trial gate: %w", err)
	}
	if err := ctxContracts.ValidatePrimarySubscriptionRoleTrialGateV1(gate); err != nil {
		return PrimarySubscriptionRoleTrialGateV1{}, err
	}
	return gate, nil
}

func PrimarySubscriptionRoleTrialGateV1SHA256() string {
	return sha256Hex(primarySubscriptionRoleTrialGateV1JSON)
}

func (ctxContracts QualificationContracts) ValidatePrimarySubscriptionRoleTrialGateV1(gate PrimarySubscriptionRoleTrialGateV1) error {
	if gate.SchemaVersion != PrimarySubscriptionRoleTrialGateSchemaV1 ||
		gate.GateID != "item-14-primary-subscription-role-trial-owner-gate-20260914" ||
		gate.Status != "owner_authority_required_not_executed" {
		return fmt.Errorf("primary subscription role trial gate identity drifted")
	}
	if gate.ProviderCallsAllowed || gate.CredentialReadsAllowed || gate.ToolExecutionAllowed || gate.FixtureRepositoryAccessAllowed ||
		gate.ProductionActionsAllowed || gate.PermanentModelOrSeatSelectionAllowed {
		return fmt.Errorf("primary subscription role trial authority opened")
	}
	if err := ctxContracts.validatePrimarySubscriptionRoleTrialSourcesV1(gate.SourceHashes); err != nil {
		return err
	}
	p := gate.ProposedTrial
	if p.Purpose != "first bounded comparison of shared executor plus exact skills versus separately invoked conditional participants" ||
		!slices.Equal(p.WorkflowFixtures, []string{"R01", "R02"}) ||
		!slices.Equal(p.StructuralArms, []string{"shared_executor_with_exact_skill_packet", "conditional_specialist_then_shared_executor"}) ||
		!slices.Equal(p.SubscriptionLanes, []string{"claude_code_subscription", "codex_subscription"}) ||
		p.MaximumCLISessions != 12 || p.Retries != 0 || p.Fallbacks || p.Redirects || !p.FreshDisposableFixturePerArm ||
		!p.TemporaryPrimaryModelBindingRequired || p.ProviderNativeToolsAllowed || !p.ControllerMediatedActionsOnly ||
		!p.IndependentQualityVerdictRequired || p.IndependentQualityVerdictMode != "provider_free_hidden_verifier" ||
		p.AutomaticCorrectionReplays != 0 || p.PostBatchProviderFreeCorrectionsMaximum != 1 {
		return fmt.Errorf("primary subscription role trial proposal drifted")
	}
	if !slices.Equal(p.AllowedExecutorActionsAfterAuthority, []string{"read_fixture", "edit_fixture", "run_allowlisted_fixture_tests", "request_local_r02_browser_evidence"}) ||
		!slices.Equal(p.AllowedContributorActionsAfterAuthority, []string{"read_fixture", "return_named_boundary_or_product_evidence"}) ||
		p.R02BrowserBoundary != "controller_owned_in_app_browser_local_loopback_only" ||
		p.SessionAccountingRule != "one fresh CLI process per named participant invocation; no relaunch; record requested and observed model identities plus aggregate usage by model" ||
		p.ClaudeExclusionRule != "if owner excludes Claude skip all four claude observations and cap execution at the six Codex participant sessions" ||
		!slices.Equal(p.AlwaysForbidden, []string{"real_repository_access", "provider_native_tool_execution", "production_access", "business_system_access", "non_subscription_credentials", "non_provider_network", "outbound_business_action", "phone_farm_access", "browser_except_controller_owned_in_app_local_r02", "deployment", "automatic_replay", "permanent_role_or_model_selection"}) {
		return fmt.Errorf("primary subscription role trial capability boundary drifted")
	}
	if err := validatePrimarySubscriptionRoleTrialPlanV1(p.InvocationPlan, p.MaximumCLISessions); err != nil {
		return err
	}
	if len(gate.KnownTransportConstraints) != 2 || gate.KnownTransportConstraints[0].Lane != "claude_code_subscription" ||
		gate.KnownTransportConstraints[1].Lane != "codex_subscription" || gate.KnownTransportConstraints[0].Fact == "" ||
		gate.KnownTransportConstraints[0].Consequence == "" || gate.KnownTransportConstraints[1].Fact == "" ||
		gate.KnownTransportConstraints[1].Consequence == "" {
		return fmt.Errorf("primary subscription role trial transport constraints drifted")
	}
	if !slices.Equal(gate.OwnerDecisionsRequired, []string{"authorize_controller_mediated_disposable_fixture_subscription_trial", "accept_claude_code_dual_model_attribution_or_exclude_claude_lane"}) ||
		gate.AdvancementRule != "a passing first slice permits repeated configuration trials but cannot select a permanent seat or model" ||
		gate.RejectionRule != "without both owner decisions no provider session subscription access fixture copy browser evidence or controller-mediated action may start" {
		return fmt.Errorf("primary subscription role trial owner gate drifted")
	}
	return nil
}

func (ctxContracts QualificationContracts) validatePrimarySubscriptionRoleTrialSourcesV1(h PrimarySubscriptionRoleTrialSourceHashesV1) error {
	rehearsalSum := sha256.Sum256(roleSquadRehearsalResultV1JSON)
	if h.RoleSquadOperatingContract != RoleSquadOperatingContractV1SHA256() ||
		h.RoleSquadRehearsalResult != hex.EncodeToString(rehearsalSum[:]) ||
		h.RealisticFixtureManifest != sha256Hex(realisticFixtureManifestV1JSON) ||
		h.RoleQualificationCaseGroup != sha256Hex(roleQualificationCaseGroupV1JSON) ||
		h.CodexSubscriptionBoundary != CodexSubscriptionBoundaryV1SHA256() ||
		h.TransportCatalog != sha256Hex(transportCatalogJSON) ||
		h.ClaudeExactModelLimitation != "5effece488f08a13a14cc22d6b61de6bb5e399658dcde18a8379b4f37949dc94" ||
		h.GuardedChildExecution != sha256Hex(ctxContracts.guardedChild()) ||
		h.RealisticFixtureController != sha256Hex(realisticFixtureControllerSource) {
		return fmt.Errorf("primary subscription role trial source drifted")
	}
	if _, err := ctxContracts.CurrentRoleSquadRehearsalResultV1(); err != nil {
		return err
	}
	if _, err := CurrentRealisticFixtureManifest(); err != nil {
		return err
	}
	if _, err := CurrentRoleQualificationCaseGroupV1(); err != nil {
		return err
	}
	if _, err := CurrentCodexSubscriptionBoundaryV1(); err != nil {
		return err
	}
	if _, err := CurrentTransportCatalog(); err != nil {
		return err
	}
	return nil
}

func validatePrimarySubscriptionRoleTrialPlanV1(plan []PrimarySubscriptionRoleTrialObservationV1, maximumSessions int) error {
	expected := []PrimarySubscriptionRoleTrialObservationV1{
		{"claude-r01-inline", "claude_code_subscription", "R01", "shared_executor_with_exact_skill_packet", []string{"shared_windows_executor"}, []string{"architecture_assurance"}, true},
		{"claude-r01-separate", "claude_code_subscription", "R01", "conditional_specialist_then_shared_executor", []string{"architecture_assurance", "shared_windows_executor"}, []string{}, true},
		{"claude-r02-inline", "claude_code_subscription", "R02", "shared_executor_with_exact_skill_packet", []string{"shared_windows_executor"}, []string{"product_ui"}, true},
		{"claude-r02-separate", "claude_code_subscription", "R02", "conditional_specialist_then_shared_executor", []string{"product_ui", "shared_windows_executor"}, []string{}, true},
		{"codex-r01-inline", "codex_subscription", "R01", "shared_executor_with_exact_skill_packet", []string{"shared_windows_executor"}, []string{"architecture_assurance"}, true},
		{"codex-r01-separate", "codex_subscription", "R01", "conditional_specialist_then_shared_executor", []string{"architecture_assurance", "shared_windows_executor"}, []string{}, true},
		{"codex-r02-inline", "codex_subscription", "R02", "shared_executor_with_exact_skill_packet", []string{"shared_windows_executor"}, []string{"product_ui"}, true},
		{"codex-r02-separate", "codex_subscription", "R02", "conditional_specialist_then_shared_executor", []string{"product_ui", "shared_windows_executor"}, []string{}, true},
	}
	if len(plan) != len(expected) {
		return fmt.Errorf("primary subscription role trial plan length drifted")
	}
	sessions := 0
	for i := range plan {
		actual, want := plan[i], expected[i]
		if actual.ObservationID != want.ObservationID || actual.Lane != want.Lane || actual.CaseID != want.CaseID ||
			actual.Arm != want.Arm || !slices.Equal(actual.Participants, want.Participants) ||
			!slices.Equal(actual.InlineSkillPackets, want.InlineSkillPackets) || actual.ProviderFreeVerdict != want.ProviderFreeVerdict {
			return fmt.Errorf("primary subscription role trial observation %d drifted", i)
		}
		sessions += len(actual.Participants)
	}
	if sessions != maximumSessions {
		return fmt.Errorf("primary subscription role trial session total drifted")
	}
	return nil
}
