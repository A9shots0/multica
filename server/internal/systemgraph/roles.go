package systemgraph

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

var ErrRoleActionDenied = errors.New("role action denied")

type RoleProfile struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Function              string   `json:"function"`
	Lifecycle             string   `json:"lifecycle"`
	OutcomeCapabilities   []string `json:"outcome_capabilities,omitempty"`
	RequiredSkills        []string `json:"required_skills,omitempty"`
	AllowedActions        []string `json:"allowed_actions"`
	ProhibitedActions     []string `json:"prohibited_actions"`
	AllowedHosts          []string `json:"allowed_hosts,omitempty"`
	IndependentFrom       []string `json:"independent_from,omitempty"`
	InstructionInvariants []string `json:"instruction_invariants"`
	ConfigurationID       string   `json:"configuration_id,omitempty"`
	ConfigurationStatus   string   `json:"configuration_status,omitempty"`
}

type SquadStage struct {
	Name             string   `json:"name"`
	EligibleRoles    []string `json:"eligible_roles"`
	Required         bool     `json:"required"`
	InvocationPolicy string   `json:"invocation_policy"`
}

type SquadTemplate struct {
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	Stages               []SquadStage `json:"stages"`
	MaxConcurrentWorkers int          `json:"max_concurrent_workers"`
	MaxCorrectionCycles  int          `json:"max_correction_cycles"`
	ControllerOwnsMoves  bool         `json:"controller_owns_moves"`
}

type RoleCatalog struct {
	Version                  string                    `json:"version"`
	Roles                    []RoleProfile             `json:"roles"`
	AccountableActorBindings []AccountableActorBinding `json:"accountable_actor_bindings"`
	Squads                   []SquadTemplate           `json:"squads"`
}

type AccountableActorBinding struct {
	Capability          string `json:"capability"`
	RoleID              string `json:"role_id"`
	ConfigurationID     string `json:"configuration_id"`
	AcceptanceAuthority string `json:"acceptance_authority"`
	Precedence          string `json:"precedence"`
	Fallback            string `json:"fallback"`
	RebindRule          string `json:"rebind_rule"`
}

//go:embed roles.json
var roleCatalogJSON []byte

var (
	rolesOnce    sync.Once
	rolesCatalog RoleCatalog
	rolesErr     error
)

func DefaultRoles() (RoleCatalog, error) {
	rolesOnce.Do(func() {
		decoder := json.NewDecoder(bytes.NewReader(roleCatalogJSON))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&rolesCatalog); err != nil {
			rolesErr = fmt.Errorf("decode embedded role catalog: %w", err)
			return
		}
		if err := ensureJSONEOF(decoder); err != nil {
			rolesErr = fmt.Errorf("decode embedded role catalog: %w", err)
			return
		}
		graph, err := Default()
		if err != nil {
			rolesErr = err
			return
		}
		rolesErr = rolesCatalog.Validate(graph)
	})
	return rolesCatalog, rolesErr
}

func (c RoleCatalog) Validate(graph Catalog) error {
	if c.Version == "" {
		return fmt.Errorf("role catalog version is required")
	}
	roles := make(map[string]RoleProfile, len(c.Roles))
	capabilities := make(map[string][]string)
	for _, role := range c.Roles {
		if role.ID == "" || role.Name == "" || role.Function == "" || role.Lifecycle == "" {
			return fmt.Errorf("role identity, function, and lifecycle are required: %+v", role)
		}
		if _, exists := roles[role.ID]; exists {
			return fmt.Errorf("duplicate role %s", role.ID)
		}
		roles[role.ID] = role
		for _, capability := range role.OutcomeCapabilities {
			capabilities[capability] = append(capabilities[capability], role.ID)
		}
		if role.Function == "independent_outcome_assurance" {
			for _, action := range []string{"edit_source", "deploy", "approve_own_work", "change_acceptance"} {
				if slices.Contains(role.AllowedActions, action) {
					return fmt.Errorf("review role %s may not allow %s", role.ID, action)
				}
			}
		}
		if slices.Contains(role.OutcomeCapabilities, "phonefarm_operation") &&
			(role.Function == "primary_executor" || role.Function == "boundary_contributor") {
			if !slices.Contains(role.AllowedHosts, "mac_studio_only") || slices.Contains(role.AllowedHosts, "windows") {
				return fmt.Errorf("Phone Farm role %s violates Mac-only boundary", role.ID)
			}
		}
		if role.Function == "owner_interface" {
			for _, action := range []string{"move_workflow", "approve_release", "deploy", "edit_source"} {
				if slices.Contains(role.AllowedActions, action) {
					return fmt.Errorf("owner interface %s owns forbidden action %s", role.ID, action)
				}
			}
		}
	}
	for _, journey := range graph.Journeys {
		if len(capabilities[journey.AccountableCapability]) == 0 {
			return fmt.Errorf("journey %s has no eligible accountable role for %s", journey.ID, journey.AccountableCapability)
		}
	}
	bindings := make(map[string]AccountableActorBinding, len(c.AccountableActorBindings))
	for _, binding := range c.AccountableActorBindings {
		if binding.Capability == "" || binding.RoleID == "" || binding.ConfigurationID == "" || binding.AcceptanceAuthority == "" || binding.Precedence == "" || binding.Fallback == "" || binding.RebindRule == "" {
			return fmt.Errorf("accountable actor binding is incomplete: %+v", binding)
		}
		role, ok := roles[binding.RoleID]
		if !ok || !slices.Contains(role.OutcomeCapabilities, binding.Capability) {
			return fmt.Errorf("binding %s references ineligible role %s", binding.Capability, binding.RoleID)
		}
		if _, exists := bindings[binding.Capability]; exists {
			return fmt.Errorf("duplicate accountable actor binding for %s", binding.Capability)
		}
		bindings[binding.Capability] = binding
	}
	for _, journey := range graph.Journeys {
		if _, ok := bindings[journey.AccountableCapability]; !ok {
			return fmt.Errorf("journey %s has no concrete accountable actor binding", journey.ID)
		}
	}
	for _, squad := range c.Squads {
		if squad.ID == "" || len(squad.Stages) == 0 || !squad.ControllerOwnsMoves {
			return fmt.Errorf("squad %s must have stages and controller-owned movement", squad.ID)
		}
		if squad.MaxConcurrentWorkers < 1 || squad.MaxCorrectionCycles < 1 || squad.MaxCorrectionCycles > 3 {
			return fmt.Errorf("squad %s has unsafe worker/correction limits", squad.ID)
		}
		for _, stage := range squad.Stages {
			for _, roleID := range stage.EligibleRoles {
				if _, ok := roles[roleID]; !ok {
					return fmt.Errorf("squad %s stage %s references missing role %s", squad.ID, stage.Name, roleID)
				}
			}
		}
	}
	return nil
}

func (c RoleCatalog) EligibleFor(capability string) []RoleProfile {
	var result []RoleProfile
	for _, role := range c.Roles {
		if slices.Contains(role.OutcomeCapabilities, capability) {
			result = append(result, role)
		}
	}
	return result
}

func (c RoleCatalog) Role(roleID string) (RoleProfile, bool) {
	for _, role := range c.Roles {
		if role.ID == roleID {
			return role, true
		}
	}
	return RoleProfile{}, false
}

// Authorize is the mechanical capability boundary used before a role is
// allowed to perform an action. Prompt text can narrow this set but can never
// expand it.
func (c RoleCatalog) Authorize(roleID, action, host string) error {
	role, ok := c.Role(roleID)
	if !ok {
		return fmt.Errorf("%w: unknown role %s", ErrRoleActionDenied, roleID)
	}
	if !slices.Contains(role.AllowedActions, action) || slices.Contains(role.ProhibitedActions, action) {
		return fmt.Errorf("%w: %s cannot %s", ErrRoleActionDenied, roleID, action)
	}
	if host != "" && len(role.AllowedHosts) > 0 && !slices.Contains(role.AllowedHosts, host) {
		return fmt.Errorf("%w: %s cannot act on host %s", ErrRoleActionDenied, roleID, host)
	}
	return nil
}

type ExecutionAssignment struct {
	Stage               string `json:"stage"`
	RoleID              string `json:"role_id"`
	ConfigurationID     string `json:"configuration_id"`
	Host                string `json:"host,omitempty"`
	QualificationStatus string `json:"qualification_status"`
}

type ExecutionPlan struct {
	Mode             string                `json:"mode"`
	AccountableActor *AccountableActor     `json:"accountable_actor,omitempty"`
	Assignments      []ExecutionAssignment `json:"assignments"`
	Unfilled         []string              `json:"unfilled_functions,omitempty"`
	Admission        string                `json:"admission"`
}

// BuildExecutionPlan resolves a graph route into the smallest eligible role
// set. It does not dispatch work; the workflow controller owns that transition.
func (c RoleCatalog) BuildExecutionPlan(route RoutePlan, statusResolvers ...func(string) string) ExecutionPlan {
	plan := ExecutionPlan{Mode: route.Mode, Admission: "denied_until_all_configurations_qualified"}
	statusFor := func(string) string { return "unresolved" }
	if len(statusResolvers) > 0 && statusResolvers[0] != nil {
		statusFor = statusResolvers[0]
	}
	for _, binding := range c.AccountableActorBindings {
		if binding.Capability == route.AccountableCapability {
			plan.AccountableActor = &AccountableActor{RoleID: binding.RoleID, ConfigurationID: binding.ConfigurationID, AcceptanceAuthority: binding.AcceptanceAuthority, Precedence: binding.Precedence, Fallback: binding.Fallback, RebindRule: binding.RebindRule}
			break
		}
	}
	primaryRole := "shared_windows_executor"
	primaryHost := "alienware_windows"
	executorConfigurations := map[string]string{
		"identity_access_authority":                "windows_builder.identity_access.v1",
		"linkhub_customer_journey":                 "windows_builder.linkhub_customer.v1",
		"media_availability":                       "windows_builder.media_availability.v1",
		"content_supply":                           "windows_builder.content_supply.v1",
		"outreach_conversation":                    "windows_builder.outreach.v1",
		"reporting_client_communication":           "windows_builder.reporting.v1",
		"crm_product_execution":                    "windows_builder.crm_product.v1",
		"canopy_account_connection":                "windows_builder.canopy_accounts.v1",
		"canopy_desktop_experience":                "windows_builder.canopy_desktop.v1",
		"telnyx_messaging":                         "windows_builder.telnyx.v1",
		"platform_recovery_monitoring_reliability": "windows_builder.platform_recovery.v1",
		"release_runtime_reliability":              "windows_builder.platform_recovery.v1",
	}
	executorConfiguration := executorConfigurations[route.AccountableCapability]
	if route.AccountableCapability == "phonefarm_operation" || slices.Contains(route.AllowedHosts, "mac_studio_only") {
		primaryRole = "phonefarm_application_specialist"
		primaryHost = "mac_studio_only"
		executorConfiguration = "phonefarm.application.mac.v1"
	}
	if route.Mode == "execute" {
		if plan.AccountableActor == nil || executorConfiguration == "" {
			plan.Unfilled = append(plan.Unfilled, "primary_executor_configuration")
		} else {
			status := statusFor(executorConfiguration)
			plan.Assignments = append(plan.Assignments, ExecutionAssignment{Stage: "implementation", RoleID: primaryRole, ConfigurationID: executorConfiguration, QualificationStatus: status, Host: primaryHost})
			if status != "qualified_bounded" {
				plan.Unfilled = append(plan.Unfilled, "primary_executor_configuration")
			}
		}
	}
	type configuredRole struct{ roleID, configurationID string }
	functionRoles := map[string]configuredRole{
		"bounded_first_divergence_triage": {"data_sync_integrity", "data_sync.sol.high.v1"},
		"data_sync_integrity":             {"data_sync_integrity", "data_sync.sol.high.v1"},
		"architecture_assurance":          {"architecture_assurance", "architecture_assurance.sol.high.v1"},
		"security_privacy_review":         {"security_privacy_review", "security_privacy.sol.high.v1"},
		"release_runtime_assurance":       {"release_runtime_assurance", "release_runtime.assurance.v1"},
		"independent_outcome_assurance":   {"quality_checker", "quality_checker.terra.xhigh.v1"},
		"product_assurance":               {"product_ui", "product_ui.sol.high.v1"},
		"phone_fleet_reliability":         {"phone_fleet_reliability", "phonefleet.reliability.mac.v1"},
	}
	for _, function := range route.RequiredFunctions {
		if function == "primary_executor" {
			continue
		}
		configured, ok := functionRoles[function]
		if !ok {
			plan.Unfilled = append(plan.Unfilled, function)
			continue
		}
		roleID := configured.roleID
		if _, ok := c.Role(roleID); !ok {
			plan.Unfilled = append(plan.Unfilled, function+"_configuration")
			continue
		}
		status := statusFor(configured.configurationID)
		host := ""
		if roleID == "release_runtime_assurance" {
			host = primaryHost
			if primaryHost == "mac_studio_only" {
				// The dedicated Mac application specialist performs the action;
				// release assurance remains a receipt-verification function.
				host = "mac_studio_only_via_dedicated_role"
			}
		}
		plan.Assignments = append(plan.Assignments, ExecutionAssignment{Stage: function, RoleID: roleID, ConfigurationID: configured.configurationID, QualificationStatus: status, Host: host})
		if status != "qualified_bounded" {
			plan.Unfilled = append(plan.Unfilled, function+"_configuration")
		}
	}
	plan.Unfilled = uniqueSorted(plan.Unfilled)
	if len(plan.Unfilled) == 0 && route.Admission == "eligible_for_typed_authority_receipt" {
		plan.Admission = "eligible_for_typed_authority_receipt"
	}
	return plan
}

// CapabilityReceipt is issued by the deterministic controller and checked at
// the execution gateway. A prompt or graph match cannot expand it.
type CapabilityReceipt struct {
	ID                          string    `json:"id"`
	ExecutorAgentID             string    `json:"executor_agent_id"`
	IssueID                     string    `json:"issue_id"`
	AuthorityRevision           int64     `json:"authority_revision"`
	RouteVersion                string    `json:"route_version"`
	RoleID                      string    `json:"role_id"`
	ConfigurationID             string    `json:"configuration_id"`
	Action                      string    `json:"action"`
	TargetNodeIDs               []string  `json:"target_node_ids"`
	HostAttestation             string    `json:"host_attestation"`
	AllowedTools                []string  `json:"allowed_tools"`
	CredentialPurposes          []string  `json:"credential_purposes,omitempty"`
	IssuedAt                    time.Time `json:"issued_at"`
	ExpiresAt                   time.Time `json:"expires_at"`
	Revoked                     bool      `json:"revoked"`
	CoverageReceiptID           string    `json:"coverage_receipt_id,omitempty"`
	OwnerAuthorizationReceiptID string    `json:"owner_authorization_receipt_id,omitempty"`
	CandidateID                 string    `json:"candidate_id,omitempty"`
	AuthorizedScope             string    `json:"authorized_scope,omitempty"`
}

type ExpectedCapabilityCall struct {
	ExecutorAgentID          string   `json:"executor_agent_id"`
	IssueID                  string   `json:"issue_id"`
	AuthorityRevision        int64    `json:"authority_revision"`
	RouteVersion             string   `json:"route_version"`
	RoleID                   string   `json:"role_id"`
	ConfigurationID          string   `json:"configuration_id"`
	Action                   string   `json:"action"`
	TargetNodeIDs            []string `json:"target_node_ids"`
	HostAttestation          string   `json:"host_attestation"`
	AttemptedTool            string   `json:"attempted_tool"`
	CredentialPurposes       []string `json:"credential_purposes,omitempty"`
	CandidateID              string   `json:"candidate_id,omitempty"`
	AuthorizedScope          string   `json:"authorized_scope,omitempty"`
	AssuranceConfigurationID string   `json:"assurance_configuration_id,omitempty"`
}

type ArchitectureAssuranceReceipt struct {
	ID                    string    `json:"id"`
	IssueID               string    `json:"issue_id"`
	RouteVersion          string    `json:"route_version"`
	CoverageGate          string    `json:"coverage_gate"`
	IssuerAgentID         string    `json:"issuer_agent_id"`
	IssuerRoleID          string    `json:"issuer_role_id"`
	IssuerConfigurationID string    `json:"issuer_configuration_id"`
	Source                string    `json:"source"`
	Passed                bool      `json:"passed"`
	Immutable             bool      `json:"immutable"`
	IssuedAt              time.Time `json:"issued_at"`
	ExpiresAt             time.Time `json:"expires_at"`
}

// OwnerAuthorizationReceipt is durable controller state. It is reusable for
// the exact candidate and scope until revoked; an agent cannot consume it,
// broaden it, silently switch release paths, or ask the owner to repeat it.
type OwnerAuthorizationReceipt struct {
	ID                string    `json:"id"`
	IssueID           string    `json:"issue_id"`
	AuthorityRevision int64     `json:"authority_revision"`
	CandidateID       string    `json:"candidate_id"`
	Action            string    `json:"action"`
	TargetNodeIDs     []string  `json:"target_node_ids"`
	AuthorizedScope   string    `json:"authorized_scope"`
	IssuerRoleID      string    `json:"issuer_role_id"`
	Source            string    `json:"source"`
	Immutable         bool      `json:"immutable"`
	Revoked           bool      `json:"revoked"`
	RecordedAt        time.Time `json:"recorded_at"`
}

func (c RoleCatalog) ValidateCapabilityReceipt(receipt CapabilityReceipt, route RoutePlan, expected ExpectedCapabilityCall, assurance *ArchitectureAssuranceReceipt, ownerAuthorization *OwnerAuthorizationReceipt, now time.Time) error {
	if receipt.ID == "" || receipt.ExecutorAgentID == "" || receipt.IssueID == "" || receipt.AuthorityRevision < 1 || receipt.RouteVersion == "" || receipt.ConfigurationID == "" || receipt.HostAttestation == "" || len(receipt.TargetNodeIDs) == 0 || len(receipt.AllowedTools) == 0 {
		return fmt.Errorf("%w: incomplete capability receipt", ErrRoleActionDenied)
	}
	if expected.ExecutorAgentID == "" || expected.IssueID == "" || expected.AuthorityRevision < 1 || expected.RouteVersion == "" || expected.RoleID == "" || expected.ConfigurationID == "" || expected.Action == "" || expected.HostAttestation == "" || expected.AttemptedTool == "" || len(expected.TargetNodeIDs) == 0 {
		return fmt.Errorf("%w: incomplete controller expected-call context", ErrRoleActionDenied)
	}
	if receipt.Revoked || !now.Before(receipt.ExpiresAt) || now.Before(receipt.IssuedAt) {
		return fmt.Errorf("%w: expired, future, or revoked receipt", ErrRoleActionDenied)
	}
	if receipt.ExecutorAgentID != expected.ExecutorAgentID || receipt.IssueID != expected.IssueID || receipt.AuthorityRevision != expected.AuthorityRevision || receipt.RouteVersion != expected.RouteVersion || receipt.RoleID != expected.RoleID || receipt.ConfigurationID != expected.ConfigurationID || receipt.Action != expected.Action || receipt.HostAttestation != expected.HostAttestation || receipt.CandidateID != expected.CandidateID || receipt.AuthorizedScope != expected.AuthorizedScope {
		return fmt.Errorf("%w: receipt does not match controller authority context", ErrRoleActionDenied)
	}
	if route.RequestedAction != "" && route.RequestedAction != expected.Action {
		return fmt.Errorf("%w: action does not match admitted route", ErrRoleActionDenied)
	}
	if !slices.Contains(route.AllowedHosts, expected.HostAttestation) || slices.Contains(route.ProhibitedHosts, expected.HostAttestation) {
		return fmt.Errorf("%w: host is not admitted by route", ErrRoleActionDenied)
	}
	if !slices.Contains(receipt.AllowedTools, expected.AttemptedTool) {
		return fmt.Errorf("%w: attempted tool is not allowed", ErrRoleActionDenied)
	}
	if !sameStrings(receipt.CredentialPurposes, expected.CredentialPurposes) {
		return fmt.Errorf("%w: credential purpose mismatch", ErrRoleActionDenied)
	}
	if err := c.Authorize(receipt.RoleID, receipt.Action, receipt.HostAttestation); err != nil {
		return err
	}
	if !sameStrings(receipt.TargetNodeIDs, expected.TargetNodeIDs) {
		return fmt.Errorf("%w: attempted targets do not match capability receipt", ErrRoleActionDenied)
	}
	for _, target := range expected.TargetNodeIDs {
		if !slices.Contains(route.ScopedNodeIDs, target) {
			return fmt.Errorf("%w: target %s is outside route scope", ErrRoleActionDenied, target)
		}
	}
	if route.RiskClass == "consequential" || route.CoverageGate != "open" {
		if assurance == nil || receipt.CoverageReceiptID == "" || assurance.ID != receipt.CoverageReceiptID || assurance.IssueID != expected.IssueID || assurance.RouteVersion != expected.RouteVersion || assurance.CoverageGate != route.CoverageGate || assurance.IssuerAgentID == "" || assurance.IssuerAgentID == expected.ExecutorAgentID || assurance.IssuerRoleID != "architecture_assurance" || assurance.IssuerConfigurationID == "" || assurance.IssuerConfigurationID != expected.AssuranceConfigurationID || assurance.Source != "controller_record" || !assurance.Passed || !assurance.Immutable || now.Before(assurance.IssuedAt) || !now.Before(assurance.ExpiresAt) {
			return fmt.Errorf("%w: consequential or incomplete coverage requires valid independent architecture assurance", ErrRoleActionDenied)
		}
		if ownerAuthorization == nil || receipt.OwnerAuthorizationReceiptID == "" || ownerAuthorization.ID != receipt.OwnerAuthorizationReceiptID || ownerAuthorization.IssueID != expected.IssueID || ownerAuthorization.AuthorityRevision != expected.AuthorityRevision || ownerAuthorization.CandidateID != expected.CandidateID || ownerAuthorization.Action != expected.Action || !sameStrings(ownerAuthorization.TargetNodeIDs, expected.TargetNodeIDs) || ownerAuthorization.AuthorizedScope != expected.AuthorizedScope || ownerAuthorization.IssuerRoleID != "owner" || ownerAuthorization.Source != "controller_record" || !ownerAuthorization.Immutable || ownerAuthorization.Revoked || now.Before(ownerAuthorization.RecordedAt) {
			return fmt.Errorf("%w: consequential action requires matching durable owner authorization", ErrRoleActionDenied)
		}
	}
	return nil
}

type ExecutionReceipt struct {
	CapabilityReceiptID string    `json:"capability_receipt_id"`
	IssueID             string    `json:"issue_id"`
	Action              string    `json:"action"`
	TargetNodeIDs       []string  `json:"target_node_ids"`
	HostAttestation     string    `json:"host_attestation"`
	CompletedAt         time.Time `json:"completed_at"`
	EvidenceRefs        []string  `json:"evidence_refs"`
}

type WorkProvenance struct {
	AgentID         string `json:"agent_id"`
	ConfigurationID string `json:"configuration_id"`
	ModelFamily     string `json:"model_family"`
	MutableChannel  bool   `json:"mutable_channel"`
	Source          string `json:"source"`
	Immutable       bool   `json:"immutable"`
}

type DeterministicSubstituteReceipt struct {
	ID                 string    `json:"id"`
	CandidateReceiptID string    `json:"candidate_receipt_id"`
	AcceptanceRevision int64     `json:"acceptance_revision"`
	RunID              string    `json:"run_id"`
	Passed             bool      `json:"passed"`
	Immutable          bool      `json:"immutable"`
	IssuedAt           time.Time `json:"issued_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

func ValidateReviewerIndependence(implementation, reviewer WorkProvenance, highRisk bool, candidateReceiptID string, acceptanceRevision int64, substitute *DeterministicSubstituteReceipt, now time.Time) error {
	if implementation.AgentID == "" || reviewer.AgentID == "" || implementation.ConfigurationID == "" || reviewer.ConfigurationID == "" {
		return fmt.Errorf("review provenance is incomplete")
	}
	if implementation.Source != "controller_record" || reviewer.Source != "controller_record" || !implementation.Immutable || !reviewer.Immutable {
		return fmt.Errorf("review provenance is not immutable controller evidence")
	}
	if implementation.AgentID == reviewer.AgentID || implementation.ConfigurationID == reviewer.ConfigurationID {
		return fmt.Errorf("reviewer shares implementation lineage")
	}
	if reviewer.MutableChannel {
		return fmt.Errorf("reviewer has mutable implementation channel")
	}
	if highRisk && implementation.ModelFamily == reviewer.ModelFamily {
		if substitute == nil || substitute.ID == "" || substitute.CandidateReceiptID != candidateReceiptID || substitute.AcceptanceRevision != acceptanceRevision || substitute.RunID == "" || !substitute.Passed || !substitute.Immutable || now.Before(substitute.IssuedAt) || !now.Before(substitute.ExpiresAt) {
			return fmt.Errorf("high-risk review requires a valid immutable deterministic substitute")
		}
	}
	return nil
}

func sameStrings(left, right []string) bool {
	a := uniqueSorted(left)
	b := uniqueSorted(right)
	return slices.Equal(a, b)
}
