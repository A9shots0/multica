package agentqualification

import (
	"errors"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

func TestTaskCapabilityNilScopePreservesLegacyGateway(t *testing.T) {
	admission, err := ValidateTaskCapabilityGatewayV1(nil, TaskCapabilityGatewayCallV1{}, time.Now())
	if err != nil || admission.Mode != GuardedDispatchModeLegacy {
		t.Fatalf("legacy gateway admission=%+v error=%v", admission, err)
	}
}

func TestTaskCapabilityGuardedBrokerCallValidatesExactReceipt(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	scope, call := taskCapabilityFixture(now)
	admission, err := ValidateTaskCapabilityGatewayV1(&scope, call, now)
	if err != nil {
		t.Fatalf("exact guarded call rejected: %v", err)
	}
	if admission.Mode != GuardedDispatchModeGuarded || admission.CapabilityReceiptID != scope.Receipt.ID {
		t.Fatalf("admission = %+v", admission)
	}
}

func TestTaskCapabilityRejectsConsequentialRawShell(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	scope, call := taskCapabilityFixture(now)
	call.GatewayKind = CapabilityGatewayRawShell
	_, err := ValidateTaskCapabilityGatewayV1(&scope, call, now)
	if !errors.Is(err, ErrConsequentialRawShellDenied) {
		t.Fatalf("raw-shell error = %v", err)
	}
}

func TestTaskCapabilityRejectsIdentityToolAndCredentialSubstitution(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	scope, base := taskCapabilityFixture(now)
	mutations := []func(*TaskCapabilityGatewayCallV1){
		func(c *TaskCapabilityGatewayCallV1) { c.WorkspaceID = "other-workspace" },
		func(c *TaskCapabilityGatewayCallV1) { c.IssueID = "other-issue" },
		func(c *TaskCapabilityGatewayCallV1) { c.ExecutorAgentID = "other-agent" },
		func(c *TaskCapabilityGatewayCallV1) { c.RuntimeID = "other-runtime" },
		func(c *TaskCapabilityGatewayCallV1) { c.RoleID = "quality_checker" },
		func(c *TaskCapabilityGatewayCallV1) { c.ConfigurationID = "other-config" },
		func(c *TaskCapabilityGatewayCallV1) { c.RouteVersion = "other-route" },
		func(c *TaskCapabilityGatewayCallV1) { c.HostAttestation = "mac_studio_only" },
		func(c *TaskCapabilityGatewayCallV1) { c.Action = "deploy_with_valid_authority" },
		func(c *TaskCapabilityGatewayCallV1) { c.TargetNodeIDs = []string{"linkhub_customer"} },
		func(c *TaskCapabilityGatewayCallV1) { c.AttemptedTool = "shell" },
		func(c *TaskCapabilityGatewayCallV1) { c.CredentialPurposes = []string{"production_deploy"} },
		func(c *TaskCapabilityGatewayCallV1) { c.CandidateID = "other-candidate" },
		func(c *TaskCapabilityGatewayCallV1) { c.AuthorizedScope = "expanded" },
		func(c *TaskCapabilityGatewayCallV1) { c.GatewayKind = "unknown" },
	}
	for i, mutate := range mutations {
		call := base
		mutate(&call)
		if _, err := ValidateTaskCapabilityGatewayV1(&scope, call, now); err == nil {
			t.Errorf("substitution %d unexpectedly admitted", i)
		}
	}
}

func TestTaskCapabilityRejectsReceiptScopeAndLifetimeDrift(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	base, call := taskCapabilityFixture(now)
	mutations := []func(*TaskCapabilityScopeV1){
		func(s *TaskCapabilityScopeV1) { s.Version = "future" },
		func(s *TaskCapabilityScopeV1) { s.WorkspaceID = "other-workspace" },
		func(s *TaskCapabilityScopeV1) { s.Receipt.ExecutorAgentID = "other-agent" },
		func(s *TaskCapabilityScopeV1) { s.Receipt.Revoked = true },
		func(s *TaskCapabilityScopeV1) { s.Receipt.ExpiresAt = now },
		func(s *TaskCapabilityScopeV1) { s.Assurance.IssuerAgentID = s.ExecutorAgentID },
		func(s *TaskCapabilityScopeV1) { s.OwnerAuthorization.Revoked = true },
	}
	for i, mutate := range mutations {
		scope := base
		assurance := *base.Assurance
		owner := *base.OwnerAuthorization
		scope.Assurance, scope.OwnerAuthorization = &assurance, &owner
		mutate(&scope)
		if _, err := ValidateTaskCapabilityGatewayV1(&scope, call, now); err == nil {
			t.Errorf("scope mutation %d unexpectedly admitted", i)
		}
	}
}

func taskCapabilityFixture(now time.Time) (TaskCapabilityScopeV1, TaskCapabilityGatewayCallV1) {
	route := systemgraph.RoutePlan{
		Mode: "execute", AllowedHosts: []string{"alienware_windows"}, ScopedNodeIDs: []string{"crm_product"},
		CoverageGate: "expand", RiskClass: "consequential", RequestedAction: "edit_source",
	}
	receipt := systemgraph.CapabilityReceipt{
		ID: "cap-1", ExecutorAgentID: "agent-1", IssueID: "issue-1", AuthorityRevision: 2,
		RouteVersion: "route-1", RoleID: "shared_windows_executor", ConfigurationID: "future.windows.runtime.v1",
		Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows",
		AllowedTools: []string{"apply_patch"}, CredentialPurposes: []string{"crm_source"},
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), CoverageReceiptID: "assure-1",
		OwnerAuthorizationReceiptID: "owner-1", CandidateID: "candidate-1", AuthorizedScope: "crm-only",
	}
	assurance := &systemgraph.ArchitectureAssuranceReceipt{
		ID: "assure-1", IssueID: "issue-1", RouteVersion: "route-1", CoverageGate: "expand",
		IssuerAgentID: "architect-1", IssuerRoleID: "architecture_assurance",
		IssuerConfigurationID: "future.architecture.runtime.v1", Source: "controller_record",
		Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
	}
	owner := &systemgraph.OwnerAuthorizationReceipt{
		ID: "owner-1", IssueID: "issue-1", AuthorityRevision: 2, CandidateID: "candidate-1",
		Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, AuthorizedScope: "crm-only",
		IssuerRoleID: "owner", Source: "controller_record", Immutable: true, RecordedAt: now.Add(-time.Minute),
	}
	scope := TaskCapabilityScopeV1{
		Version: TaskCapabilityScopeVersionV1, WorkspaceID: "workspace-1", IssueID: "issue-1",
		ExecutorAgentID: "agent-1", RuntimeID: "runtime-1", RoleID: "shared_windows_executor",
		ConfigurationID: "future.windows.runtime.v1", RuntimeBindingReceiptID: "runtime-binding-1", RouteVersion: "route-1",
		HostAttestation: "alienware_windows", Action: "edit_source", Route: route,
		Receipt: receipt, Assurance: assurance, OwnerAuthorization: owner,
	}
	call := TaskCapabilityGatewayCallV1{
		WorkspaceID: "workspace-1", IssueID: "issue-1", ExecutorAgentID: "agent-1", RuntimeID: "runtime-1",
		RoleID: "shared_windows_executor", ConfigurationID: "future.windows.runtime.v1", RouteVersion: "route-1",
		HostAttestation: "alienware_windows", GatewayKind: CapabilityGatewayRemoteMCP,
		Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, AttemptedTool: "apply_patch",
		CredentialPurposes: []string{"crm_source"}, CandidateID: "candidate-1", AuthorizedScope: "crm-only",
		AssuranceConfigurationID: "future.architecture.runtime.v1",
	}
	return scope, call
}
