package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

const RoleSquadOperatingContractV1Schema = "silver-mediums.role-squad-operating-contract.v1"
const RoleSquadOperatingValidationV1Schema = "silver-mediums.role-squad-operating-validation.v1"

type RoleSquadOperatingSourceHashesV1 struct {
	RoleCatalog        string `json:"role_catalog"`
	SystemGraph        string `json:"system_graph"`
	QualificationCases string `json:"qualification_cases"`
	RoleArtifactsV1    string `json:"role_artifacts_v1"`
	RoleArtifactsV2    string `json:"role_artifacts_v2"`
}

type RoleSquadOperatingCountsV1 struct {
	Roles                           int `json:"roles"`
	ControllerOnlyOutcomeIdentities int `json:"controller_only_outcome_identities"`
	MaintainedCandidateCores        int `json:"maintained_candidate_cores"`
	ConditionalCandidatePackets     int `json:"conditional_candidate_packets"`
	ExternalMacRoles                int `json:"external_mac_roles"`
	SkillOnlyPackets                int `json:"skill_only_packets"`
	Squads                          int `json:"squads"`
	SystemJourneys                  int `json:"system_journeys"`
	LockedCases                     int `json:"locked_cases"`
}

type RoleSquadExecutionClassesV1 struct {
	ControllerOnlyOutcomeIdentities []string `json:"controller_only_outcome_identities"`
	MaintainedCandidateCores        []string `json:"maintained_candidate_cores"`
	ConditionalCandidatePackets     []string `json:"conditional_candidate_packets"`
	ExternalMacRoles                []string `json:"external_mac_roles"`
	SkillOnlyPackets                []string `json:"skill_only_packets"`
}

type RoleSquadCollaborationRulesV1 struct {
	ControllerOwnsAllMoves                                 bool `json:"controller_owns_all_moves"`
	OneAccountableOutcome                                  bool `json:"one_accountable_outcome"`
	OnePrimaryExecutorPerHost                              bool `json:"one_primary_executor_per_host"`
	MaxConcurrentWorkers                                   int  `json:"max_concurrent_workers"`
	MaxAutomaticCorrections                                int  `json:"max_automatic_corrections"`
	MaxReviewAttempts                                      int  `json:"max_review_attempts"`
	ReviewerMustBeDistinct                                 bool `json:"reviewer_must_be_distinct"`
	ParticipantsAreOrderedStages                           bool `json:"participants_are_ordered_stages"`
	MikaRelayForbidden                                     bool `json:"mika_relay_forbidden"`
	SpecialistFanoutRequiresUniqueNamedEvidence            bool `json:"specialist_fanout_requires_unique_named_evidence"`
	JourneyRequiredFunctionDoesNotAutomaticallyCreateASeat bool `json:"journey_required_function_does_not_automatically_create_a_separate_seat"`
	HistoricalConfigurationLabelsDoNotCreateSeats          bool `json:"historical_configuration_labels_do_not_create_seats"`
	SharedSkillPacketIsNotIndependentReview                bool `json:"shared_skill_packet_is_not_independent_review"`
}

type RoleSquadUIRuleV1 struct {
	ID                   string   `json:"id"`
	Condition            string   `json:"condition"`
	Participants         []string `json:"participants"`
	SharedExecutorSkills []string `json:"shared_executor_skills"`
	OwnerDecision        bool     `json:"owner_decision"`
	ExecutionAllowed     bool     `json:"execution_allowed"`
}

type RoleSquadCaseRouteV1 struct {
	CaseID               string   `json:"case_id"`
	SquadID              string   `json:"squad_id"`
	AccountableIdentity  string   `json:"accountable_identity"`
	Host                 string   `json:"host"`
	Participants         []string `json:"participants"`
	SharedExecutorSkills []string `json:"shared_executor_skills"`
	ReleaseStage         bool     `json:"release_stage"`
}

type RoleSquadAdditionalRouteV1 struct {
	ID           string   `json:"id"`
	SquadID      string   `json:"squad_id"`
	Participants []string `json:"participants"`
	Purpose      string   `json:"purpose"`
}

type RoleSquadOperatingContractV1 struct {
	SchemaVersion                 string                           `json:"schema_version"`
	ContractID                    string                           `json:"contract_id"`
	Status                        string                           `json:"status"`
	ReviewOutcome                 string                           `json:"review_outcome"`
	ConsolidatedCorrections       int                              `json:"consolidated_corrections"`
	ProviderCallsAllowed          bool                             `json:"provider_calls_allowed"`
	CredentialReadsAllowed        bool                             `json:"credential_reads_allowed"`
	ProductionActionsAllowed      bool                             `json:"production_actions_allowed"`
	PermanentSeatSelectionAllowed bool                             `json:"permanent_seat_selection_allowed"`
	ModelSelectionAllowed         bool                             `json:"model_selection_allowed"`
	SourceHashes                  RoleSquadOperatingSourceHashesV1 `json:"source_hashes"`
	Counts                        RoleSquadOperatingCountsV1       `json:"counts"`
	ExecutionClasses              RoleSquadExecutionClassesV1      `json:"execution_classes"`
	CollaborationRules            RoleSquadCollaborationRulesV1    `json:"collaboration_rules"`
	UIRoutingRules                []RoleSquadUIRuleV1              `json:"ui_routing_rules"`
	CaseRoutes                    []RoleSquadCaseRouteV1           `json:"case_routes"`
	AdditionalRoutes              []RoleSquadAdditionalRouteV1     `json:"additional_routes"`
	AdversarialDenials            []string                         `json:"adversarial_denials"`
}

type RoleSquadOperatingValidationV1 struct {
	SchemaVersion             string `json:"schema_version"`
	RunID                     string `json:"run_id"`
	ExecutionMode             string `json:"execution_mode"`
	ReviewOutcome             string `json:"review_outcome"`
	ConsolidatedCorrections   int    `json:"consolidated_corrections"`
	ContractSHA256            string `json:"contract_sha256"`
	RolesCovered              int    `json:"roles_covered"`
	OutcomeIdentitiesCovered  int    `json:"outcome_identities_covered"`
	SquadsCovered             int    `json:"squads_covered"`
	SystemJourneysCovered     int    `json:"system_journeys_covered"`
	LockedCasesCovered        int    `json:"locked_cases_covered"`
	UIRoutingRulesCovered     int    `json:"ui_routing_rules_covered"`
	AdversarialDenialsCovered int    `json:"adversarial_denials_covered"`
	ProviderRequests          int    `json:"provider_requests"`
	CredentialReads           int    `json:"credential_reads"`
	ProductionActions         int    `json:"production_actions"`
	PermanentSeatSelections   int    `json:"permanent_seat_selections"`
	ModelSelections           int    `json:"model_selections"`
	Valid                     bool   `json:"valid"`
}

//go:embed role_squad_operating_contract_v1.json
var roleSquadOperatingContractV1JSON []byte

//go:embed role_squad_operating_contract_v1_result.json
var roleSquadOperatingValidationV1JSON []byte

func (ctxContracts QualificationContracts) CurrentRoleSquadOperatingContractV1() (RoleSquadOperatingContractV1, error) {
	var contract RoleSquadOperatingContractV1
	if err := json.Unmarshal(roleSquadOperatingContractV1JSON, &contract); err != nil {
		return contract, fmt.Errorf("decode role squad operating contract v1: %w", err)
	}
	if err := ctxContracts.ValidateRoleSquadOperatingContractV1(contract); err != nil {
		return RoleSquadOperatingContractV1{}, err
	}
	return contract, nil
}

func RoleSquadOperatingContractV1SHA256() string {
	sum := sha256.Sum256(roleSquadOperatingContractV1JSON)
	return hex.EncodeToString(sum[:])
}

func (ctxContracts QualificationContracts) CurrentRoleSquadOperatingValidationV1() (RoleSquadOperatingValidationV1, error) {
	var recorded RoleSquadOperatingValidationV1
	if err := strictJSON(roleSquadOperatingValidationV1JSON, &recorded); err != nil {
		return recorded, fmt.Errorf("decode role squad operating validation v1: %w", err)
	}
	expected, err := ctxContracts.FakeRoleSquadOperatingValidationV1()
	if err != nil {
		return RoleSquadOperatingValidationV1{}, err
	}
	if recorded != expected {
		return RoleSquadOperatingValidationV1{}, fmt.Errorf("role squad operating validation result drifted")
	}
	return recorded, nil
}

func (ctxContracts QualificationContracts) ValidateRoleSquadOperatingContractV1(contract RoleSquadOperatingContractV1) error {
	if contract.SchemaVersion != RoleSquadOperatingContractV1Schema || strings.TrimSpace(contract.ContractID) == "" ||
		contract.Status != "accepted_provider_free_candidate" || contract.ReviewOutcome != "self_adversarial_revise_p1_no_p0" ||
		contract.ConsolidatedCorrections != 1 {
		return fmt.Errorf("role squad operating contract identity drifted")
	}
	if contract.ProviderCallsAllowed || contract.CredentialReadsAllowed || contract.ProductionActionsAllowed ||
		contract.PermanentSeatSelectionAllowed || contract.ModelSelectionAllowed {
		return fmt.Errorf("role squad operating contract expanded authority")
	}
	roles, err := ctxContracts.roles()
	if err != nil {
		return err
	}
	graph, err := systemgraph.Default()
	if err != nil {
		return err
	}
	cases, err := Cases()
	if err != nil {
		return err
	}
	if err := ValidateRoleQualificationPrevalidationV1(); err != nil {
		return fmt.Errorf("validate v1 role artifacts: %w", err)
	}
	v2Bindings, err := EmbeddedRoleArtifactBindingsV2()
	if err != nil {
		return err
	}
	if err := ValidateRoleArtifactBindingsV2(v2Bindings); err != nil {
		return fmt.Errorf("validate v2 role artifacts: %w", err)
	}
	if contract.SourceHashes.RoleCatalog != ctxContracts.rolesSHA256() ||
		contract.SourceHashes.SystemGraph != systemgraph.GraphArtifactSHA256() ||
		contract.SourceHashes.QualificationCases != qualificationCasesSHA256() ||
		contract.SourceHashes.RoleArtifactsV1 != sha256Hex(roleQualificationFreezeArtifactManifestV1) ||
		contract.SourceHashes.RoleArtifactsV2 != localShadowRoleArtifactBindingsV2SHA256() {
		return fmt.Errorf("role squad operating source hash drifted")
	}
	if contract.Counts.Roles != len(roles.Roles) || contract.Counts.Squads != len(roles.Squads) ||
		contract.Counts.SystemJourneys != len(graph.Journeys) || contract.Counts.LockedCases != len(cases) {
		return fmt.Errorf("role squad operating source count drifted")
	}
	if err := validateRoleExecutionClassesV1(contract, roles); err != nil {
		return err
	}
	rules := contract.CollaborationRules
	if !rules.ControllerOwnsAllMoves || !rules.OneAccountableOutcome || !rules.OnePrimaryExecutorPerHost ||
		rules.MaxConcurrentWorkers != 2 || rules.MaxAutomaticCorrections != 1 || rules.MaxReviewAttempts != 2 ||
		!rules.ReviewerMustBeDistinct || !rules.ParticipantsAreOrderedStages || !rules.MikaRelayForbidden ||
		!rules.SpecialistFanoutRequiresUniqueNamedEvidence || !rules.JourneyRequiredFunctionDoesNotAutomaticallyCreateASeat ||
		!rules.HistoricalConfigurationLabelsDoNotCreateSeats || !rules.SharedSkillPacketIsNotIndependentReview {
		return fmt.Errorf("collaboration rules drifted")
	}
	if err := validateRoleSquadUIRulesV1(contract.UIRoutingRules); err != nil {
		return err
	}
	if err := validateRoleSquadCaseRoutesV1(contract, roles, cases); err != nil {
		return err
	}
	if err := validateRoleSquadJourneyCoverageV1(graph, cases); err != nil {
		return err
	}
	if len(contract.AdditionalRoutes) != 1 || contract.AdditionalRoutes[0].ID != "A01" ||
		contract.AdditionalRoutes[0].SquadID != "independent_audit" ||
		!slices.Equal(contract.AdditionalRoutes[0].Participants, []string{"company_auditor", "quality_checker"}) ||
		contract.AdditionalRoutes[0].Purpose != "bounded company audit plus evidence-completeness review" {
		return fmt.Errorf("independent audit route drifted")
	}
	wantDenials := []string{
		"controller_outcome_identity_attempts_execution",
		"quality_checker_shares_agent_or_configuration_with_implementation",
		"windows_attempts_phone_farm_technical_work",
		"mika_attempts_workflow_movement_or_status_polling",
		"specialist_adds_no_unique_named_evidence",
		"more_than_two_workers_run_concurrently",
		"second_automatic_correction_is_requested",
		"shared_skill_is_presented_as_independent_review",
	}
	if !slices.Equal(contract.AdversarialDenials, wantDenials) {
		return fmt.Errorf("adversarial denial coverage drifted")
	}
	return nil
}

func validateRoleExecutionClassesV1(contract RoleSquadOperatingContractV1, roles systemgraph.RoleCatalog) error {
	classes := contract.ExecutionClasses
	sets := [][]string{
		classes.ControllerOnlyOutcomeIdentities,
		classes.MaintainedCandidateCores,
		classes.ConditionalCandidatePackets,
		classes.ExternalMacRoles,
	}
	seen := map[string]struct{}{}
	for _, values := range sets {
		if !sort.StringsAreSorted(values) {
			return fmt.Errorf("role execution class is not canonical")
		}
		for _, roleID := range values {
			if _, duplicate := seen[roleID]; duplicate {
				return fmt.Errorf("role %s appears in multiple execution classes", roleID)
			}
			if _, ok := roles.Role(roleID); !ok {
				return fmt.Errorf("execution class references missing role %s", roleID)
			}
			seen[roleID] = struct{}{}
		}
	}
	if len(seen) != len(roles.Roles) || len(classes.ControllerOnlyOutcomeIdentities) != contract.Counts.ControllerOnlyOutcomeIdentities ||
		len(classes.MaintainedCandidateCores) != contract.Counts.MaintainedCandidateCores ||
		len(classes.ConditionalCandidatePackets) != contract.Counts.ConditionalCandidatePackets ||
		len(classes.ExternalMacRoles) != contract.Counts.ExternalMacRoles ||
		len(classes.SkillOnlyPackets) != contract.Counts.SkillOnlyPackets ||
		!slices.Equal(classes.SkillOnlyPackets, []string{"mac_alert_analysis"}) {
		return fmt.Errorf("role execution class coverage is incomplete")
	}
	for _, role := range roles.Roles {
		_, controllerOnly := seenControllerRoleV1(classes.ControllerOnlyOutcomeIdentities, role.ID)
		if (role.Function == "accountable_outcome_owner") != controllerOnly {
			return fmt.Errorf("controller-only outcome identity classification drifted for %s", role.ID)
		}
	}
	return nil
}

func seenControllerRoleV1(values []string, value string) (int, bool) {
	index, found := slices.BinarySearch(values, value)
	return index, found
}

func validateRoleSquadUIRulesV1(rules []RoleSquadUIRuleV1) error {
	if len(rules) != 3 {
		return fmt.Errorf("exactly three UI routing rules are required")
	}
	if rules[0].ID != "ui-exact-acceptance" || !slices.Equal(rules[0].Participants, []string{"shared_windows_executor", "quality_checker"}) ||
		!slices.Equal(rules[0].SharedExecutorSkills, []string{"product_ui"}) || rules[0].OwnerDecision || !rules[0].ExecutionAllowed {
		return fmt.Errorf("exact UI acceptance route drifted")
	}
	if rules[1].ID != "ui-ambiguous-experience" || !slices.Equal(rules[1].Participants, []string{"product_ui", "shared_windows_executor", "quality_checker"}) ||
		len(rules[1].SharedExecutorSkills) != 0 || rules[1].OwnerDecision || !rules[1].ExecutionAllowed {
		return fmt.Errorf("ambiguous UI route drifted")
	}
	if rules[2].ID != "ui-consequential-taste" || !slices.Equal(rules[2].Participants, []string{"mika_owner_interface"}) ||
		len(rules[2].SharedExecutorSkills) != 0 || !rules[2].OwnerDecision || rules[2].ExecutionAllowed {
		return fmt.Errorf("owner taste route drifted")
	}
	return nil
}

func validateRoleSquadCaseRoutesV1(contract RoleSquadOperatingContractV1, roles systemgraph.RoleCatalog, cases []Case) error {
	if len(contract.CaseRoutes) != len(cases) {
		return fmt.Errorf("locked case route coverage is incomplete")
	}
	caseByID := make(map[string]Case, len(cases))
	for _, item := range cases {
		caseByID[item.ID] = item
	}
	bindingByCapability := map[string]string{}
	for _, binding := range roles.AccountableActorBindings {
		bindingByCapability[binding.Capability] = binding.RoleID
	}
	seen := map[string]struct{}{}
	for _, route := range contract.CaseRoutes {
		item, ok := caseByID[route.CaseID]
		if !ok {
			return fmt.Errorf("unknown locked case route %s", route.CaseID)
		}
		if _, duplicate := seen[route.CaseID]; duplicate {
			return fmt.Errorf("duplicate locked case route %s", route.CaseID)
		}
		seen[route.CaseID] = struct{}{}
		wantParticipants, err := participantRolesForFunctionsV1(item.TargetFunctions)
		if err != nil {
			return fmt.Errorf("case %s: %w", route.CaseID, err)
		}
		if !slices.Equal(route.Participants, wantParticipants) {
			return fmt.Errorf("case %s participant route drifted", route.CaseID)
		}
		if len(route.Participants) == 0 {
			return fmt.Errorf("case %s has no staged participants", route.CaseID)
		}
		participantSet := make(map[string]struct{}, len(route.Participants))
		primaryExecutors := 0
		for _, roleID := range route.Participants {
			if _, duplicate := participantSet[roleID]; duplicate {
				return fmt.Errorf("case %s repeats participant %s", route.CaseID, roleID)
			}
			participantSet[roleID] = struct{}{}
			role, ok := roles.Role(roleID)
			if !ok || role.Function == "accountable_outcome_owner" {
				return fmt.Errorf("case %s uses a controller-only or missing executable role %s", route.CaseID, roleID)
			}
			if role.Function == "primary_executor" {
				primaryExecutors++
			}
		}
		if primaryExecutors > 1 {
			return fmt.Errorf("case %s has more than one primary executor", route.CaseID)
		}
		if route.Host == "mac_studio_only" {
			if route.CaseID != "T05" || slices.Contains(route.Participants, "shared_windows_executor") {
				return fmt.Errorf("case %s violates the Phone Farm host boundary", route.CaseID)
			}
		} else if route.Host != "alienware_windows" {
			return fmt.Errorf("case %s has an unknown host", route.CaseID)
		}
		if (slices.Contains(route.Participants, "phonefarm_application_specialist") ||
			slices.Contains(route.Participants, "phone_fleet_reliability")) && route.Host != "mac_studio_only" {
			return fmt.Errorf("case %s moves a Phone Farm role outside the Mac Studio boundary", route.CaseID)
		}
		wantAccountable := accountableIdentityForCaseV1(item, bindingByCapability)
		if route.AccountableIdentity != wantAccountable {
			return fmt.Errorf("case %s accountable identity drifted", route.CaseID)
		}
		containsRelease := slices.Contains(route.Participants, "release_runtime_assurance")
		if route.ReleaseStage != containsRelease {
			return fmt.Errorf("case %s release-stage claim drifted", route.CaseID)
		}
		if route.SquadID != squadForCaseV1(route.CaseID) {
			return fmt.Errorf("case %s squad route drifted", route.CaseID)
		}
		if route.Participants[len(route.Participants)-1] != "quality_checker" {
			return fmt.Errorf("case %s lacks a final independent reviewer", route.CaseID)
		}
		for _, skill := range route.SharedExecutorSkills {
			if !slices.Contains(route.Participants, "shared_windows_executor") {
				return fmt.Errorf("case %s assigns a shared skill without the shared executor", route.CaseID)
			}
			if slices.Contains(route.Participants, skill) {
				return fmt.Errorf("case %s duplicates a separate role as a shared skill", route.CaseID)
			}
			if !slices.Contains([]string{"architecture_assurance", "product_ui", "security_privacy_review"}, skill) {
				return fmt.Errorf("case %s uses an unknown shared specialist skill %s", route.CaseID, skill)
			}
		}
	}
	if len(seen) != len(cases) {
		return fmt.Errorf("locked case route coverage is incomplete")
	}
	return nil
}

func squadForCaseV1(caseID string) string {
	if slices.Contains([]string{"T09", "T11", "T13"}, caseID) {
		return "standard_outcome_delivery"
	}
	return "cross_boundary_change"
}

func validateRoleSquadJourneyCoverageV1(graph systemgraph.Catalog, cases []Case) error {
	coveredCapabilities := map[string]struct{}{}
	for _, item := range cases {
		for _, capability := range item.RequiredCapabilities {
			coveredCapabilities[capability] = struct{}{}
		}
	}
	for _, journey := range graph.Journeys {
		if _, covered := coveredCapabilities[journey.AccountableCapability]; !covered {
			return fmt.Errorf("journey %s capability %s has no locked qualification case", journey.ID, journey.AccountableCapability)
		}
	}
	return nil
}

func participantRolesForFunctionsV1(functions []string) ([]string, error) {
	roleByFunction := map[string]string{
		"owner_interface":                  "mika_owner_interface",
		"primary_executor":                 "shared_windows_executor",
		"data_sync_integrity":              "data_sync_integrity",
		"quality_checker":                  "quality_checker",
		"phonefarm_application_specialist": "phonefarm_application_specialist",
		"phone_fleet_reliability":          "phone_fleet_reliability",
		"release_runtime_assurance":        "release_runtime_assurance",
		"product_ui":                       "product_ui",
		"architecture_assurance":           "architecture_assurance",
		"security_privacy_review":          "security_privacy_review",
	}
	result := make([]string, 0, len(functions))
	for _, function := range functions {
		roleID, ok := roleByFunction[function]
		if !ok {
			return nil, fmt.Errorf("unmapped target function %s", function)
		}
		result = append(result, roleID)
	}
	return result, nil
}

func accountableIdentityForCaseV1(item Case, bindings map[string]string) string {
	if len(item.RequiredCapabilities) != 1 {
		return "controller_unbound_until_first_divergence"
	}
	capability := item.RequiredCapabilities[0]
	if capability == "cross_domain_reasoning" {
		return "controller_unbound_until_first_divergence"
	}
	if capability == "security_privacy_reasoning" {
		return "controller_read_only_preflight"
	}
	return bindings[capability]
}

func qualificationCasesSHA256() string {
	sum := sha256.Sum256(casesJSON)
	return hex.EncodeToString(sum[:])
}

func (ctxContracts QualificationContracts) FakeRoleSquadOperatingValidationV1() (RoleSquadOperatingValidationV1, error) {
	contract, err := ctxContracts.CurrentRoleSquadOperatingContractV1()
	if err != nil {
		return RoleSquadOperatingValidationV1{}, err
	}
	journeys := map[string]struct{}{}
	graph, _ := systemgraph.Default()
	for _, journey := range graph.Journeys {
		journeys[journey.ID] = struct{}{}
	}
	return RoleSquadOperatingValidationV1{
		SchemaVersion:             RoleSquadOperatingValidationV1Schema,
		RunID:                     "item-14-role-squad-provider-free-validation-20260914",
		ExecutionMode:             "deterministic_provider_free",
		ReviewOutcome:             contract.ReviewOutcome,
		ConsolidatedCorrections:   contract.ConsolidatedCorrections,
		ContractSHA256:            RoleSquadOperatingContractV1SHA256(),
		RolesCovered:              contract.Counts.Roles,
		OutcomeIdentitiesCovered:  contract.Counts.ControllerOnlyOutcomeIdentities,
		SquadsCovered:             contract.Counts.Squads,
		SystemJourneysCovered:     len(journeys),
		LockedCasesCovered:        len(contract.CaseRoutes),
		UIRoutingRulesCovered:     len(contract.UIRoutingRules),
		AdversarialDenialsCovered: len(contract.AdversarialDenials),
		Valid:                     true,
	}, nil
}
