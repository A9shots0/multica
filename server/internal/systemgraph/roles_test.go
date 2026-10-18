package systemgraph

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestDefaultRolesValidateAgainstEveryJourney(t *testing.T) {
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	graph := defaultForTest(t)
	for _, journey := range graph.Journeys {
		if len(roles.EligibleFor(journey.AccountableCapability)) == 0 {
			t.Errorf("journey %s has no eligible role", journey.ID)
		}
	}
}

func TestEveryJourneyHasExactlyOneConcreteAccountableActorBinding(t *testing.T) {
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	graph := defaultForTest(t)
	for _, journey := range graph.Journeys {
		count := 0
		for _, binding := range roles.AccountableActorBindings {
			if binding.Capability == journey.AccountableCapability {
				count++
			}
		}
		if count != 1 {
			t.Errorf("journey %s binding count=%d", journey.ID, count)
		}
	}
}

func TestMechanicalPermissionsCannotBeExpandedByPrompt(t *testing.T) {
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Authorize("quality_checker", "deploy", "alienware_windows"); !errors.Is(err, ErrRoleActionDenied) {
		t.Fatalf("reviewer deployment = %v, want denied", err)
	}
	if err := roles.Authorize("phonefarm_application_specialist", "edit_phonefarm_source", "windows"); !errors.Is(err, ErrRoleActionDenied) {
		t.Fatalf("Windows Phone Farm work = %v, want denied", err)
	}
	if err := roles.Authorize("phonefarm_application_specialist", "edit_phonefarm_source", "mac_studio_only"); err != nil {
		t.Fatalf("Mac Phone Farm work denied: %v", err)
	}
}

func TestExecutionPlanKeepsImplementationAndReviewIndependent(t *testing.T) {
	graph := defaultForTest(t)
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	route, err := graph.RouteIssue(RouteRequest{ObservedNodeIDs: []string{"linkhub", "customer_link_journey"}, RiskFlags: []string{"production"}})
	if err != nil {
		t.Fatal(err)
	}
	plan := roles.BuildExecutionPlan(route)
	var builder, reviewer string
	for _, assignment := range plan.Assignments {
		if assignment.Stage == "implementation" {
			builder = assignment.RoleID
		}
		if assignment.Stage == "independent_outcome_assurance" {
			reviewer = assignment.RoleID
		}
	}
	if builder == "" || reviewer == "" || builder == reviewer {
		t.Fatalf("execution/review separation missing: %+v", plan)
	}
}

func TestAccountableOutcomeOwnerIsNotTheRepositoryExecutor(t *testing.T) {
	graph := defaultForTest(t)
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	route, err := graph.RouteIssue(RouteRequest{ObservedNodeIDs: []string{"linkhub", "customer_link_journey"}, RequestedAction: "edit_source"})
	if err != nil {
		t.Fatal(err)
	}
	plan := roles.BuildExecutionPlan(route, func(string) string { return "qualified_bounded" })
	if plan.AccountableActor == nil || plan.AccountableActor.RoleID != "linkhub_domains_outcome" {
		t.Fatalf("wrong accountable outcome owner: %+v", plan.AccountableActor)
	}
	for _, assignment := range plan.Assignments {
		if assignment.Stage == "implementation" {
			if assignment.RoleID != "shared_windows_executor" || assignment.ConfigurationID != "windows_builder.linkhub_customer.v1" {
				t.Fatalf("wrong repository executor: %+v", assignment)
			}
			return
		}
	}
	t.Fatal("implementation assignment missing")
}

func TestQualityCheckerCannotBuildOrRelease(t *testing.T) {
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	var checker RoleProfile
	for _, role := range roles.Roles {
		if role.ID == "quality_checker" {
			checker = role
		}
	}
	for _, forbidden := range []string{"edit_source", "deploy", "approve_own_work", "change_acceptance"} {
		if slices.Contains(checker.AllowedActions, forbidden) || !slices.Contains(checker.ProhibitedActions, forbidden) {
			t.Errorf("checker separation missing for %s: %+v", forbidden, checker)
		}
	}
}

func TestContentSupplyAndTelnyxUseSeparateAccountabilityAndCapabilities(t *testing.T) {
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]string{}
	profiles := map[string]RoleProfile{}
	for _, binding := range roles.AccountableActorBindings {
		if binding.Capability == "content_supply" || binding.Capability == "telnyx_messaging" {
			bindings[binding.Capability] = binding.RoleID
		}
	}
	for _, role := range roles.Roles {
		profiles[role.ID] = role
	}
	if bindings["content_supply"] != "content_inspiration_outcome" || bindings["telnyx_messaging"] != "telnyx_messaging_outcome" {
		t.Fatalf("accountability identities are not isolated: %+v", bindings)
	}
	if bindings["content_supply"] == bindings["telnyx_messaging"] {
		t.Fatal("content and messaging share one accountability identity")
	}
	if slices.Contains(profiles["content_inspiration_outcome"].OutcomeCapabilities, "telnyx_messaging") || slices.Contains(profiles["telnyx_messaging_outcome"].OutcomeCapabilities, "content_supply") {
		t.Fatal("content and messaging capability contexts overlap")
	}
}

func TestSquadsAreBoundedAndControllerOwned(t *testing.T) {
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	for _, squad := range roles.Squads {
		if !squad.ControllerOwnsMoves || squad.MaxConcurrentWorkers > 2 || squad.MaxCorrectionCycles > 2 {
			t.Errorf("unbounded squad: %+v", squad)
		}
	}
}

func TestCapabilityReceiptFailsClosedWithoutCoverageAssurance(t *testing.T) {
	roles, err := DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	route := RoutePlan{RiskClass: "consequential", CoverageGate: "expand_before_consequential_action", ScopedNodeIDs: []string{"crm_product"}, AllowedHosts: []string{"alienware_windows"}, RequestedAction: "edit_source"}
	receipt := CapabilityReceipt{
		ID: "cap-1", ExecutorAgentID: "builder-1", IssueID: "issue-1", AuthorityRevision: 1, RouteVersion: "route-v1", RoleID: "shared_windows_executor",
		ConfigurationID: "windows_builder.crm_product.v1", Action: "edit_source",
		TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows",
		AllowedTools: []string{"apply_patch"}, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), CandidateID: "candidate-1", AuthorizedScope: "crm_product_only", OwnerAuthorizationReceiptID: "owner-1",
	}
	expected := ExpectedCapabilityCall{ExecutorAgentID: "builder-1", IssueID: "issue-1", AuthorityRevision: 1, RouteVersion: "route-v1", RoleID: "shared_windows_executor", ConfigurationID: "windows_builder.crm_product.v1", Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows", AttemptedTool: "apply_patch", CandidateID: "candidate-1", AuthorizedScope: "crm_product_only", AssuranceConfigurationID: "architecture_assurance.sol.high.v1"}
	if err := roles.ValidateCapabilityReceipt(receipt, route, expected, nil, nil, now); !errors.Is(err, ErrRoleActionDenied) {
		t.Fatalf("missing architecture assurance = %v, want denial", err)
	}
	receipt.CoverageReceiptID = "architecture-assurance-1"
	assurance := &ArchitectureAssuranceReceipt{ID: "architecture-assurance-1", IssueID: "issue-1", RouteVersion: "route-v1", CoverageGate: "expand_before_consequential_action", IssuerAgentID: "architect-1", IssuerRoleID: "architecture_assurance", IssuerConfigurationID: "architecture_assurance.sol.high.v1", Source: "controller_record", Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)}
	owner := &OwnerAuthorizationReceipt{ID: "owner-1", IssueID: "issue-1", AuthorityRevision: 1, CandidateID: "candidate-1", Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, AuthorizedScope: "crm_product_only", IssuerRoleID: "owner", Source: "controller_record", Immutable: true, RecordedAt: now.Add(-time.Minute)}
	if err := roles.ValidateCapabilityReceipt(receipt, route, expected, assurance, owner, now); err != nil {
		t.Fatalf("valid scoped receipt denied: %v", err)
	}
	receipt.HostAttestation = "mac_studio_only"
	if err := roles.ValidateCapabilityReceipt(receipt, route, expected, assurance, owner, now); !errors.Is(err, ErrRoleActionDenied) {
		t.Fatalf("host mismatch = %v, want denial", err)
	}
}

func TestCapabilityReceiptRejectsEveryAuthorityBypass(t *testing.T) {
	roles, _ := DefaultRoles()
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	route := RoutePlan{RiskClass: "consequential", CoverageGate: "expand", ScopedNodeIDs: []string{"crm_product"}, AllowedHosts: []string{"alienware_windows"}, RequestedAction: "edit_source"}
	receipt := CapabilityReceipt{ID: "cap", ExecutorAgentID: "builder", IssueID: "issue", AuthorityRevision: 2, RouteVersion: "route", RoleID: "shared_windows_executor", ConfigurationID: "windows_builder.crm_product.v1", Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows", AllowedTools: []string{"apply_patch"}, CredentialPurposes: []string{"crm_source"}, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), CoverageReceiptID: "assure", OwnerAuthorizationReceiptID: "owner", CandidateID: "candidate", AuthorizedScope: "crm_only"}
	expected := ExpectedCapabilityCall{ExecutorAgentID: "builder", IssueID: "issue", AuthorityRevision: 2, RouteVersion: "route", RoleID: "shared_windows_executor", ConfigurationID: "windows_builder.crm_product.v1", Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows", AttemptedTool: "apply_patch", CredentialPurposes: []string{"crm_source"}, CandidateID: "candidate", AuthorizedScope: "crm_only", AssuranceConfigurationID: "architecture_assurance.sol.high.v1"}
	assurance := &ArchitectureAssuranceReceipt{ID: "assure", IssueID: "issue", RouteVersion: "route", CoverageGate: "expand", IssuerAgentID: "architect", IssuerRoleID: "architecture_assurance", IssuerConfigurationID: "architecture_assurance.sol.high.v1", Source: "controller_record", Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)}
	owner := &OwnerAuthorizationReceipt{ID: "owner", IssueID: "issue", AuthorityRevision: 2, CandidateID: "candidate", Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, AuthorizedScope: "crm_only", IssuerRoleID: "owner", Source: "controller_record", Immutable: true, RecordedAt: now.Add(-time.Minute)}
	if err := roles.ValidateCapabilityReceipt(receipt, route, expected, assurance, owner, now); err != nil {
		t.Fatal(err)
	}
	mutations := []func(*CapabilityReceipt, *ExpectedCapabilityCall, *ArchitectureAssuranceReceipt){
		func(r *CapabilityReceipt, _ *ExpectedCapabilityCall, _ *ArchitectureAssuranceReceipt) {
			r.HostAttestation = ""
		},
		func(_ *CapabilityReceipt, e *ExpectedCapabilityCall, _ *ArchitectureAssuranceReceipt) {
			e.AttemptedTool = "exec_command"
		},
		func(_ *CapabilityReceipt, e *ExpectedCapabilityCall, _ *ArchitectureAssuranceReceipt) {
			e.TargetNodeIDs = []string{"linkhub"}
		},
		func(_ *CapabilityReceipt, e *ExpectedCapabilityCall, _ *ArchitectureAssuranceReceipt) {
			e.CredentialPurposes = []string{"other"}
		},
		func(r *CapabilityReceipt, _ *ExpectedCapabilityCall, _ *ArchitectureAssuranceReceipt) {
			r.AuthorityRevision = 3
		},
		func(_ *CapabilityReceipt, _ *ExpectedCapabilityCall, a *ArchitectureAssuranceReceipt) {
			a.Passed = false
		},
		func(_ *CapabilityReceipt, e *ExpectedCapabilityCall, a *ArchitectureAssuranceReceipt) {
			a.IssuerAgentID = e.ExecutorAgentID
		},
		func(_ *CapabilityReceipt, _ *ExpectedCapabilityCall, a *ArchitectureAssuranceReceipt) {
			a.IssuerConfigurationID = "caller_supplied"
		},
	}
	for i, mutate := range mutations {
		r, e, a := receipt, expected, *assurance
		mutate(&r, &e, &a)
		if err := roles.ValidateCapabilityReceipt(r, route, e, &a, owner, now); !errors.Is(err, ErrRoleActionDenied) {
			t.Errorf("bypass %d accepted: %v", i, err)
		}
	}
}

func TestDurableOwnerAuthorizationCannotRepeatPermissionOrReroute(t *testing.T) {
	roles, _ := DefaultRoles()
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	route := RoutePlan{RiskClass: "consequential", CoverageGate: "expand", ScopedNodeIDs: []string{"crm_product"}, AllowedHosts: []string{"alienware_windows"}, RequestedAction: "deploy_with_valid_authority"}
	receipt := CapabilityReceipt{ID: "cap", ExecutorAgentID: "release-agent", IssueID: "issue", AuthorityRevision: 4, RouteVersion: "route", RoleID: "release_runtime_assurance", ConfigurationID: "release_runtime.assurance.v1", Action: "deploy_with_valid_authority", TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows", AllowedTools: []string{"git_push"}, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), CoverageReceiptID: "assure", OwnerAuthorizationReceiptID: "owner", CandidateID: "commit-b177563", AuthorizedScope: "push reviewed commit to configured origin"}
	expected := ExpectedCapabilityCall{ExecutorAgentID: "release-agent", IssueID: "issue", AuthorityRevision: 4, RouteVersion: "route", RoleID: "release_runtime_assurance", ConfigurationID: "release_runtime.assurance.v1", Action: "deploy_with_valid_authority", TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows", AttemptedTool: "git_push", CandidateID: "commit-b177563", AuthorizedScope: "push reviewed commit to configured origin", AssuranceConfigurationID: "architecture_assurance.sol.high.v1"}
	assurance := &ArchitectureAssuranceReceipt{ID: "assure", IssueID: "issue", RouteVersion: "route", CoverageGate: "expand", IssuerAgentID: "architect", IssuerRoleID: "architecture_assurance", IssuerConfigurationID: "architecture_assurance.sol.high.v1", Source: "controller_record", Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)}
	owner := &OwnerAuthorizationReceipt{ID: "owner", IssueID: "issue", AuthorityRevision: 4, CandidateID: "commit-b177563", Action: "deploy_with_valid_authority", TargetNodeIDs: []string{"crm_product"}, AuthorizedScope: "push reviewed commit to configured origin", IssuerRoleID: "owner", Source: "controller_record", Immutable: true, RecordedAt: now.Add(-time.Minute)}

	for i := 0; i < 2; i++ {
		if err := roles.ValidateCapabilityReceipt(receipt, route, expected, assurance, owner, now); err != nil {
			t.Fatalf("durable authorization rejected on use %d: %v", i+1, err)
		}
	}
	rerouted := expected
	rerouted.AuthorizedScope = "direct Railway deployment"
	if err := roles.ValidateCapabilityReceipt(receipt, route, rerouted, assurance, owner, now); !errors.Is(err, ErrRoleActionDenied) {
		t.Fatalf("silent reroute accepted: %v", err)
	}
	changedCandidate := expected
	changedCandidate.CandidateID = "different-commit"
	if err := roles.ValidateCapabilityReceipt(receipt, route, changedCandidate, assurance, owner, now); !errors.Is(err, ErrRoleActionDenied) {
		t.Fatalf("different candidate accepted: %v", err)
	}
}

func TestReviewerProvenanceRejectsSameLineageAndCorrelatedHighRisk(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	impl := WorkProvenance{AgentID: "builder-1", ConfigurationID: "builder.sol.high.v1", ModelFamily: "sol", Source: "controller_record", Immutable: true}
	same := WorkProvenance{AgentID: "reviewer-1", ConfigurationID: "builder.sol.high.v1", ModelFamily: "sol", Source: "controller_record", Immutable: true}
	if err := ValidateReviewerIndependence(impl, same, false, "candidate", 1, nil, now); err == nil {
		t.Fatal("same configuration lineage was accepted")
	}
	correlated := WorkProvenance{AgentID: "reviewer-2", ConfigurationID: "reviewer.sol.high.v1", ModelFamily: "sol", Source: "controller_record", Immutable: true}
	if err := ValidateReviewerIndependence(impl, correlated, true, "candidate", 1, nil, now); err == nil {
		t.Fatal("correlated high-risk reviewer was accepted without substitute")
	}
	fake := &DeterministicSubstituteReceipt{ID: "x", CandidateReceiptID: "wrong", AcceptanceRevision: 1, RunID: "run", Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)}
	if err := ValidateReviewerIndependence(impl, correlated, true, "candidate", 1, fake, now); err == nil {
		t.Fatal("wrong-candidate substitute accepted")
	}
	independent := WorkProvenance{AgentID: "reviewer-3", ConfigurationID: "reviewer.terra.high.v1", ModelFamily: "terra", Source: "controller_record", Immutable: true}
	if err := ValidateReviewerIndependence(impl, independent, true, "candidate", 1, nil, now); err != nil {
		t.Fatalf("independent reviewer denied: %v", err)
	}
}
