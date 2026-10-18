package agentqualification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	permissionControllerID = "00000000-0000-0000-0000-000000000001"
	permissionWorkspaceID  = "00000000-0000-0000-0000-000000000002"
	permissionIssueID      = "00000000-0000-0000-0000-000000000003"
	permissionAgentID      = "00000000-0000-0000-0000-000000000004"
	permissionRuntimeID    = "00000000-0000-0000-0000-000000000005"
	permissionTaskID       = "00000000-0000-0000-0000-000000000006"
	permissionDaemonID     = "fake-daemon-authority-test"
)

func TestPermissionReceiptStoreIssuesCompleteAuthorityAndExactRetryConverges(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)
	request := GuardedExecutionIssueV1{
		TaskID: permissionTaskID, Receipt: receipt, Envelope: envelope,
		CapabilityScope: scope, PluginToolTargets: targets,
	}
	first, err := store.IssueGuardedExecutionAuthority(context.Background(), permissionControllerID, request, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.IssueGuardedExecutionAuthority(context.Background(), permissionControllerID, request, now)
	if err != nil {
		t.Fatalf("exact issuance retry: %v", err)
	}
	_, firstDigest, _ := permissionDigest(first)
	_, secondDigest, _ := permissionDigest(second)
	if firstDigest != secondDigest {
		t.Fatalf("exact issuance retry changed package: %s != %s", firstDigest, secondDigest)
	}
	queries.mu.Lock()
	if len(queries.runtimeReceipts) != 1 || len(queries.capabilityGrants) != 1 {
		t.Fatalf("durable authority rows = %d/%d, want 1/1", len(queries.runtimeReceipts), len(queries.capabilityGrants))
	}
	queries.mu.Unlock()

	altered := request
	altered.Envelope.Model = "altered-after-approval"
	if _, err := store.IssueGuardedExecutionAuthority(context.Background(), permissionControllerID, altered, now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("altered issuance error = %v, want configuration unbound", err)
	}
}

func TestPermissionReceiptStoreConsumesExactAuthorityOnce(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)
	pkg, err := store.IssueGuardedExecutionAuthority(context.Background(), permissionControllerID, GuardedExecutionIssueV1{
		TaskID: permissionTaskID, Receipt: receipt, Envelope: envelope,
		CapabilityScope: scope, PluginToolTargets: targets,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := GuardedExecutionPackageSHA256V1(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(context.Background(), permissionLoadIdentity(), envelope, strings.Repeat("a", 64), permissionDaemonID, now); !errors.Is(err, ErrGuardedAuthorityUnavailable) {
		t.Fatalf("wrong-digest consumption error = %v", err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(context.Background(), permissionLoadIdentity(), envelope, digest, "wrong-daemon", now); !errors.Is(err, ErrGuardedAuthorityUnavailable) {
		t.Fatalf("wrong-daemon consumption error = %v", err)
	}
	first, err := store.ConsumeGuardedExecutionAuthority(context.Background(), permissionLoadIdentity(), envelope, digest, permissionDaemonID, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.RuntimeBinding.ID != receipt.ID || first.CapabilityScope.Receipt.ID != scope.Receipt.ID {
		t.Fatalf("consumed package drifted: %+v", first)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(context.Background(), permissionLoadIdentity(), envelope, digest, permissionDaemonID, now); !errors.Is(err, ErrGuardedAuthorityUnavailable) {
		t.Fatalf("replay consumption error = %v", err)
	}
	queries.mu.Lock()
	defer queries.mu.Unlock()
	if len(queries.consumptions) != 1 {
		t.Fatalf("durable launch consumptions = %d, want 1", len(queries.consumptions))
	}
}

func TestPermissionReceiptStoreDeniesExpiredRevokedAndAlteredConsumption(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		mutate func(*PermissionReceiptStore, GuardedDispatchEnvelopeV1, TaskCapabilityScopeV1) GuardedDispatchEnvelopeV1
		at     time.Time
		want   error
	}{
		{name: "expired", at: now.Add(2 * time.Hour), want: ErrGuardedConfigurationUnbound},
		{name: "altered", at: now, want: ErrGuardedConfigurationUnbound, mutate: func(_ *PermissionReceiptStore, envelope GuardedDispatchEnvelopeV1, _ TaskCapabilityScopeV1) GuardedDispatchEnvelopeV1 {
			envelope.Model = "altered-after-claim"
			return envelope
		}},
		{name: "revoked", at: now, want: ErrTaskCapabilityDenied, mutate: func(store *PermissionReceiptStore, envelope GuardedDispatchEnvelopeV1, scope TaskCapabilityScopeV1) GuardedDispatchEnvelopeV1 {
			if err := store.RevokeTaskCapabilityGrant(context.Background(), permissionControllerID, permissionWorkspaceID, scope.Receipt.ID, "withdrawn before launch", now); err != nil {
				t.Fatal(err)
			}
			return envelope
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queries := newMemoryPermissionReceiptQueries()
			store := NewPermissionReceiptStore(queries)
			receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)
			pkg, err := store.IssueGuardedExecutionAuthority(context.Background(), permissionControllerID, GuardedExecutionIssueV1{
				TaskID: permissionTaskID, Receipt: receipt, Envelope: envelope,
				CapabilityScope: scope, PluginToolTargets: targets,
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			digest, err := GuardedExecutionPackageSHA256V1(pkg)
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				envelope = tc.mutate(store, envelope, scope)
			}
			if _, err := store.ConsumeGuardedExecutionAuthority(context.Background(), permissionLoadIdentity(), envelope, digest, permissionDaemonID, tc.at); !errors.Is(err, tc.want) {
				t.Fatalf("consumption error = %v, want %v", err, tc.want)
			}
			queries.mu.Lock()
			defer queries.mu.Unlock()
			if len(queries.consumptions) != 0 {
				t.Fatalf("denied authority created %d consumption rows", len(queries.consumptions))
			}
		})
	}
}

func TestPermissionReceiptStorePersistsReloadsAndIdempotentlyReplaysExactBundle(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 123456000, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)

	storedReceipt, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now)
	if err != nil {
		t.Fatalf("store runtime binding: %v", err)
	}
	storedScope, err := store.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, scope, targets, now)
	if err != nil {
		t.Fatalf("store capability grant: %v", err)
	}
	if storedReceipt.ID != receipt.ID || storedScope.Receipt.ID != scope.Receipt.ID {
		t.Fatalf("stored identities = %q/%q", storedReceipt.ID, storedScope.Receipt.ID)
	}

	// A new store instance proves the result comes from durable-query state, not
	// an in-process cache. Exact controller retries are idempotent.
	restarted := NewPermissionReceiptStore(queries)
	if _, err := restarted.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatalf("idempotent runtime replay: %v", err)
	}
	if _, err := restarted.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, scope, targets, now); err != nil {
		t.Fatalf("idempotent capability replay: %v", err)
	}
	pkg, err := restarted.LoadGuardedExecutionPackage(context.Background(), permissionLoadIdentity(), envelope, now)
	if err != nil {
		t.Fatalf("load guarded package: %v", err)
	}
	if pkg.RuntimeBinding.ID != receipt.ID || pkg.CapabilityScope.Receipt.ID != scope.Receipt.ID || pkg.PluginToolTargets["plugin__read"][0] != "crm_product" {
		t.Fatalf("loaded package drifted: %+v", pkg)
	}
}

func TestPermissionReceiptStoreRejectsStoredLegacyV1BeforeChildBoundaryDecode(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, scope, targets, now); err != nil {
		t.Fatal(err)
	}

	queries.mu.Lock()
	row := queries.runtimeReceipts[receipt.ID]
	var legacy map[string]any
	if err := json.Unmarshal(row.Receipt, &legacy); err != nil {
		queries.mu.Unlock()
		t.Fatal(err)
	}
	legacy["version"] = QualifiedRuntimeBindingVersionV1
	delete(legacy, "child_execution")
	row.Receipt, _ = json.Marshal(legacy)
	queries.runtimeReceipts[receipt.ID] = row
	queries.mu.Unlock()

	if _, err := store.LoadGuardedExecutionPackage(context.Background(), permissionLoadIdentity(), envelope, now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("stored legacy v1 receipt error = %v, want configuration unbound", err)
	}
}

func TestPermissionReceiptStoreRejectsConflictingDuplicateAndTaskGrant(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, _, scope, targets := permissionReceiptFixtures(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, scope, targets, now); err != nil {
		t.Fatal(err)
	}

	changed := receipt
	changed.Model = "substituted-model"
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, changed, now); !errors.Is(err, ErrGuardedDispatchInvalid) {
		t.Fatalf("conflicting runtime error = %v", err)
	}
	changedScope := scope
	changedScope.Receipt.ID = "capability-other"
	if _, err := store.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, changedScope, targets, now); err == nil {
		t.Fatal("second capability grant for one task unexpectedly stored")
	}
}

func TestPermissionReceiptStoreRejectsTamperedRowsAndCrossIdentity(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, scope, targets, now); err != nil {
		t.Fatal(err)
	}

	wrongIdentity := permissionLoadIdentity()
	wrongIdentity.IssueID = "00000000-0000-0000-0000-000000000099"
	if _, err := store.LoadGuardedExecutionPackage(context.Background(), wrongIdentity, envelope, now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("cross-issue load error = %v", err)
	}

	queries.mu.Lock()
	tampered := queries.runtimeReceipts[receipt.ID]
	tampered.Model = "database-column-substitution"
	queries.runtimeReceipts[receipt.ID] = tampered
	queries.mu.Unlock()
	if _, err := store.LoadGuardedExecutionPackage(context.Background(), permissionLoadIdentity(), envelope, now); !errors.Is(err, ErrGuardedDispatchInvalid) {
		t.Fatalf("tampered runtime row error = %v", err)
	}

	queries.mu.Lock()
	tampered.Model = receipt.Model
	queries.runtimeReceipts[receipt.ID] = tampered
	grant := queries.capabilityGrants[scope.Receipt.ID]
	grant.PayloadSha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	queries.capabilityGrants[scope.Receipt.ID] = grant
	queries.mu.Unlock()
	if _, err := store.LoadGuardedExecutionPackage(context.Background(), permissionLoadIdentity(), envelope, now); !errors.Is(err, ErrTaskCapabilityDenied) {
		t.Fatalf("tampered capability row error = %v", err)
	}
}

func TestPermissionReceiptStoreRevocationIsOneWayAndFailsClosed(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, envelope, scope, targets := permissionReceiptFixtures(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, scope, targets, now); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeTaskCapabilityGrant(context.Background(), permissionControllerID, permissionWorkspaceID, scope.Receipt.ID, "scope superseded", now); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeTaskCapabilityGrant(context.Background(), permissionControllerID, permissionWorkspaceID, scope.Receipt.ID, "scope superseded", now); err != nil {
		t.Fatalf("repeat capability revocation: %v", err)
	}
	if err := store.RevokeTaskCapabilityGrant(context.Background(), permissionControllerID, permissionWorkspaceID, scope.Receipt.ID, "different replay", now); !errors.Is(err, ErrTaskCapabilityDenied) {
		t.Fatalf("conflicting capability replay error = %v", err)
	}
	if _, err := store.LoadGuardedExecutionPackage(context.Background(), permissionLoadIdentity(), envelope, now); !errors.Is(err, ErrTaskCapabilityDenied) {
		t.Fatalf("revoked grant load error = %v", err)
	}

	// A separately revoked runtime receipt is denied before capability loading.
	queries.capabilityRevocations = map[string]db.GuardedTaskCapabilityRevocation{}
	if err := store.RevokeQualifiedRuntimeBinding(context.Background(), permissionControllerID, permissionWorkspaceID, receipt.ID, "qualification withdrawn", now); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeQualifiedRuntimeBinding(context.Background(), permissionControllerID, permissionWorkspaceID, receipt.ID, "qualification withdrawn", now); err != nil {
		t.Fatalf("repeat runtime revocation: %v", err)
	}
	if err := store.RevokeQualifiedRuntimeBinding(context.Background(), permissionControllerID, permissionWorkspaceID, receipt.ID, "different replay", now); !errors.Is(err, ErrGuardedDispatchInvalid) {
		t.Fatalf("conflicting runtime replay error = %v", err)
	}
	if _, err := store.LoadGuardedExecutionPackage(context.Background(), permissionLoadIdentity(), envelope, now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("revoked runtime load error = %v", err)
	}
}

func TestPermissionReceiptStoreRejectsUnknownAndWrongWorkspaceRevocation(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, _, scope, targets := permissionReceiptFixtures(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StoreTaskCapabilityGrant(context.Background(), permissionControllerID, permissionTaskID, scope, targets, now); err != nil {
		t.Fatal(err)
	}
	wrongWorkspace := "00000000-0000-0000-0000-000000000099"
	if err := store.RevokeQualifiedRuntimeBinding(context.Background(), permissionControllerID, wrongWorkspace, receipt.ID, "wrong scope", now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("wrong-workspace runtime revocation error = %v", err)
	}
	if err := store.RevokeTaskCapabilityGrant(context.Background(), permissionControllerID, wrongWorkspace, scope.Receipt.ID, "wrong scope", now); !errors.Is(err, ErrTaskCapabilityDenied) {
		t.Fatalf("wrong-workspace capability revocation error = %v", err)
	}
	if err := store.RevokeQualifiedRuntimeBinding(context.Background(), permissionControllerID, permissionWorkspaceID, "missing-runtime", "unknown", now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("unknown runtime revocation error = %v", err)
	}
	if err := store.RevokeTaskCapabilityGrant(context.Background(), permissionControllerID, permissionWorkspaceID, "missing-grant", "unknown", now); !errors.Is(err, ErrTaskCapabilityDenied) {
		t.Fatalf("unknown capability revocation error = %v", err)
	}
	if len(queries.runtimeRevocations) != 0 || len(queries.capabilityRevocations) != 0 {
		t.Fatal("invalid revocation created a tombstone")
	}
}

func TestPermissionReceiptStoreConcurrentExactRevocationHasOneDurableRecord(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, _, _, _ := permissionReceiptFixtures(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	const workers = 8
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.RevokeQualifiedRuntimeBinding(context.Background(), permissionControllerID, permissionWorkspaceID, receipt.ID, "qualification withdrawn", now)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent exact revocation: %v", err)
		}
	}
	queries.mu.Lock()
	defer queries.mu.Unlock()
	if len(queries.runtimeRevocations) != 1 {
		t.Fatalf("durable revocations = %d, want 1", len(queries.runtimeRevocations))
	}
}

func TestPermissionReceiptStoreConcurrentExactIssuanceHasOneDurableRecord(t *testing.T) {
	now := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt, _, _, _ := permissionReceiptFixtures(t, now)
	const workers = 8
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent exact issuance: %v", err)
		}
	}
	queries.mu.Lock()
	defer queries.mu.Unlock()
	if len(queries.runtimeReceipts) != 1 {
		t.Fatalf("durable runtime rows = %d, want 1", len(queries.runtimeReceipts))
	}
}

func permissionReceiptFixtures(t *testing.T, now time.Time) (QualifiedRuntimeBindingReceiptV2, GuardedDispatchEnvelopeV1, TaskCapabilityScopeV1, map[string][]string) {
	t.Helper()
	artifact, err := ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		t.Fatal(err)
	}
	artifactHash, err := roleArtifactBindingSHA256V2(artifact)
	if err != nil {
		t.Fatal(err)
	}
	receipt := QualifiedRuntimeBindingReceiptV2{
		Version: QualifiedRuntimeBindingVersionV2, ID: "runtime-binding-1",
		WorkspaceID: permissionWorkspaceID, IssueID: permissionIssueID,
		ExecutorAgentID: permissionAgentID, RuntimeID: permissionRuntimeID,
		ConfigurationID: "future.data-sync.runtime.v1", RoleID: "data_sync_integrity",
		RuntimeProvider: "claude", Model: "future-model", ReasoningEffort: "future-effort",
		ChildExecution:        guardedChildExecutionFixture(t, "claude", "windows"),
		ArtifactBindingSHA256: artifactHash, QualificationStatus: StatusQualifiedBounded,
		EvidenceReceiptIDs: []string{"qualification-evidence-1"}, Source: qualifiedRuntimeBindingSourceV2,
		Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	envelope := GuardedDispatchEnvelopeV1{
		Version: GuardedDispatchEnvelopeVersionV2, WorkspaceID: permissionWorkspaceID,
		IssueID: permissionIssueID, ExecutorAgentID: permissionAgentID, RuntimeID: permissionRuntimeID,
		ConfigurationID: receipt.ConfigurationID, RoleID: receipt.RoleID,
		RuntimeProvider: receipt.RuntimeProvider, Model: receipt.Model, ReasoningEffort: receipt.ReasoningEffort,
		ArtifactBindingID: artifact.BindingID, ArtifactBinding: artifact, RuntimeBindingReceiptID: receipt.ID,
	}
	route := systemgraph.RoutePlan{
		Mode: "contribute", AllowedHosts: []string{"alienware_windows"}, ScopedNodeIDs: []string{"crm_product"},
		CoverageGate: "expand", RiskClass: "bounded_read", RequestedAction: "read_boundary_state",
	}
	capability := systemgraph.CapabilityReceipt{
		ID: "capability-1", ExecutorAgentID: permissionAgentID, IssueID: permissionIssueID,
		AuthorityRevision: 2, RouteVersion: "route-1", RoleID: receipt.RoleID,
		ConfigurationID: receipt.ConfigurationID, Action: "read_boundary_state",
		TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows",
		AllowedTools: []string{"plugin__read"}, CredentialPurposes: []string{"crm_read"},
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour), CoverageReceiptID: "assurance-1",
		OwnerAuthorizationReceiptID: "owner-1", CandidateID: "candidate-1", AuthorizedScope: "crm-read-only",
	}
	assurance := &systemgraph.ArchitectureAssuranceReceipt{
		ID: "assurance-1", IssueID: permissionIssueID, RouteVersion: "route-1", CoverageGate: "expand",
		IssuerAgentID: "architecture-agent", IssuerRoleID: "architecture_assurance",
		IssuerConfigurationID: "future.architecture.runtime.v1", Source: "controller_record",
		Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	owner := &systemgraph.OwnerAuthorizationReceipt{
		ID: "owner-1", IssueID: permissionIssueID, AuthorityRevision: 2, CandidateID: "candidate-1",
		Action: "read_boundary_state", TargetNodeIDs: []string{"crm_product"}, AuthorizedScope: "crm-read-only",
		IssuerRoleID: "owner", Source: "controller_record", Immutable: true, RecordedAt: now.Add(-time.Minute),
	}
	scope := TaskCapabilityScopeV1{
		Version: TaskCapabilityScopeVersionV1, WorkspaceID: permissionWorkspaceID, IssueID: permissionIssueID,
		ExecutorAgentID: permissionAgentID, RuntimeID: permissionRuntimeID, RoleID: receipt.RoleID,
		ConfigurationID: receipt.ConfigurationID, RuntimeBindingReceiptID: receipt.ID, RouteVersion: "route-1",
		HostAttestation: "alienware_windows", Action: "read_boundary_state", Route: route,
		Receipt: capability, Assurance: assurance, OwnerAuthorization: owner,
	}
	return receipt, envelope, scope, map[string][]string{"plugin__read": {"crm_product"}}
}

func permissionLoadIdentity() GuardedExecutionLoadIdentityV1 {
	return GuardedExecutionLoadIdentityV1{
		TaskID: permissionTaskID, WorkspaceID: permissionWorkspaceID, IssueID: permissionIssueID,
		ExecutorAgentID: permissionAgentID, RuntimeID: permissionRuntimeID,
		ExpectedControllerID: permissionControllerID,
	}
}

type memoryPermissionReceiptQueries struct {
	mu                    sync.Mutex
	runtimeReceipts       map[string]db.QualifiedRuntimeBindingReceipt
	runtimeRevocations    map[string]db.QualifiedRuntimeBindingRevocation
	capabilityGrants      map[string]db.GuardedTaskCapabilityGrant
	capabilityTaskIDs     map[string]string
	capabilityRevocations map[string]db.GuardedTaskCapabilityRevocation
	consumptions          map[string]db.GuardedExecutionLaunchConsumption
	daemonIDs             map[string]string
}

func newMemoryPermissionReceiptQueries() *memoryPermissionReceiptQueries {
	return &memoryPermissionReceiptQueries{
		runtimeReceipts:    map[string]db.QualifiedRuntimeBindingReceipt{},
		runtimeRevocations: map[string]db.QualifiedRuntimeBindingRevocation{},
		capabilityGrants:   map[string]db.GuardedTaskCapabilityGrant{}, capabilityTaskIDs: map[string]string{},
		capabilityRevocations: map[string]db.GuardedTaskCapabilityRevocation{},
		consumptions:          map[string]db.GuardedExecutionLaunchConsumption{},
		daemonIDs:             map[string]string{permissionRuntimeID: permissionDaemonID},
	}
}

func (m *memoryPermissionReceiptQueries) CreateQualifiedRuntimeBindingReceipt(_ context.Context, p db.CreateQualifiedRuntimeBindingReceiptParams) (db.QualifiedRuntimeBindingReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.runtimeReceipts[p.ID]; exists {
		return db.QualifiedRuntimeBindingReceipt{}, pgx.ErrNoRows
	}
	row := db.QualifiedRuntimeBindingReceipt{
		ID: p.ID, WorkspaceID: p.WorkspaceID, IssueID: p.IssueID, ExecutorAgentID: p.ExecutorAgentID, RuntimeID: p.RuntimeID,
		ConfigurationID: p.ConfigurationID, RoleID: p.RoleID, RuntimeProvider: p.RuntimeProvider, Model: p.Model,
		ReasoningEffort: p.ReasoningEffort, ArtifactBindingSha256: p.ArtifactBindingSha256,
		QualificationStatus: p.QualificationStatus, EvidenceReceiptIds: append([]byte(nil), p.EvidenceReceiptIds...),
		Source: p.Source, Immutable: p.Immutable, IssuedByControllerID: p.IssuedByControllerID,
		IssuedAt: p.IssuedAt, ExpiresAt: p.ExpiresAt, Receipt: append([]byte(nil), p.Receipt...), ReceiptSha256: p.ReceiptSha256,
	}
	m.runtimeReceipts[p.ID] = row
	return row, nil
}

func (m *memoryPermissionReceiptQueries) GetQualifiedRuntimeBindingReceiptByID(_ context.Context, id string) (db.QualifiedRuntimeBindingReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runtimeReceipts[id]
	if !ok {
		return row, pgx.ErrNoRows
	}
	return row, nil
}

func (m *memoryPermissionReceiptQueries) GetActiveQualifiedRuntimeBindingReceiptForExecution(_ context.Context, p db.GetActiveQualifiedRuntimeBindingReceiptForExecutionParams) (db.QualifiedRuntimeBindingReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runtimeReceipts[p.ID]
	if !ok || m.runtimeRevocations[runtimeRevocationKey(util.UUIDToString(p.WorkspaceID), p.ID)].ReceiptID != "" ||
		util.UUIDToString(row.WorkspaceID) != util.UUIDToString(p.WorkspaceID) || util.UUIDToString(row.IssueID) != util.UUIDToString(p.IssueID) ||
		util.UUIDToString(row.ExecutorAgentID) != util.UUIDToString(p.ExecutorAgentID) || util.UUIDToString(row.RuntimeID) != util.UUIDToString(p.RuntimeID) ||
		!row.ExpiresAt.Time.After(p.Now.Time) {
		return db.QualifiedRuntimeBindingReceipt{}, pgx.ErrNoRows
	}
	return row, nil
}

func (m *memoryPermissionReceiptQueries) RevokeQualifiedRuntimeBindingReceipt(_ context.Context, p db.RevokeQualifiedRuntimeBindingReceiptParams) (db.QualifiedRuntimeBindingRevocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target, exists := m.runtimeReceipts[p.ReceiptID]
	if !exists || util.UUIDToString(target.WorkspaceID) != util.UUIDToString(p.WorkspaceID) || util.UUIDToString(target.IssuedByControllerID) != util.UUIDToString(p.RevokedByControllerID) {
		return db.QualifiedRuntimeBindingRevocation{}, pgx.ErrNoRows
	}
	key := runtimeRevocationKey(util.UUIDToString(p.WorkspaceID), p.ReceiptID)
	if _, exists = m.runtimeRevocations[key]; exists {
		return db.QualifiedRuntimeBindingRevocation{}, pgx.ErrNoRows
	}
	row := db.QualifiedRuntimeBindingRevocation{ReceiptID: p.ReceiptID, WorkspaceID: p.WorkspaceID, RevokedByControllerID: p.RevokedByControllerID, Reason: p.Reason, RevokedAt: p.RevokedAt}
	m.runtimeRevocations[key] = row
	return row, nil
}

func (m *memoryPermissionReceiptQueries) GetQualifiedRuntimeBindingRevocation(_ context.Context, p db.GetQualifiedRuntimeBindingRevocationParams) (db.QualifiedRuntimeBindingRevocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runtimeRevocations[runtimeRevocationKey(util.UUIDToString(p.WorkspaceID), p.ReceiptID)]
	if !ok {
		return row, pgx.ErrNoRows
	}
	return row, nil
}

func (m *memoryPermissionReceiptQueries) CreateGuardedTaskCapabilityGrant(_ context.Context, p db.CreateGuardedTaskCapabilityGrantParams) (db.GuardedTaskCapabilityGrant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	taskKey := util.UUIDToString(p.WorkspaceID) + "/" + util.UUIDToString(p.TaskID)
	if _, exists := m.capabilityGrants[p.ID]; exists || m.capabilityTaskIDs[taskKey] != "" {
		return db.GuardedTaskCapabilityGrant{}, pgx.ErrNoRows
	}
	row := db.GuardedTaskCapabilityGrant{
		ID: p.ID, TaskID: p.TaskID, WorkspaceID: p.WorkspaceID, IssueID: p.IssueID, ExecutorAgentID: p.ExecutorAgentID,
		RuntimeID: p.RuntimeID, RuntimeBindingReceiptID: p.RuntimeBindingReceiptID, ConfigurationID: p.ConfigurationID,
		RoleID: p.RoleID, RouteVersion: p.RouteVersion, HostAttestation: p.HostAttestation, Action: p.Action,
		IssuedByControllerID: p.IssuedByControllerID, IssuedAt: p.IssuedAt, ExpiresAt: p.ExpiresAt,
		CapabilityScope: append([]byte(nil), p.CapabilityScope...), PluginToolTargets: append([]byte(nil), p.PluginToolTargets...), PayloadSha256: p.PayloadSha256,
	}
	m.capabilityGrants[p.ID] = row
	m.capabilityTaskIDs[taskKey] = p.ID
	return row, nil
}

func (m *memoryPermissionReceiptQueries) GetGuardedTaskCapabilityGrantByID(_ context.Context, id string) (db.GuardedTaskCapabilityGrant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.capabilityGrants[id]
	if !ok {
		return row, pgx.ErrNoRows
	}
	return row, nil
}

func (m *memoryPermissionReceiptQueries) GetActiveGuardedTaskCapabilityGrantForExecution(_ context.Context, p db.GetActiveGuardedTaskCapabilityGrantForExecutionParams) (db.GuardedTaskCapabilityGrant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	taskKey := util.UUIDToString(p.WorkspaceID) + "/" + util.UUIDToString(p.TaskID)
	id := m.capabilityTaskIDs[taskKey]
	row, ok := m.capabilityGrants[id]
	if !ok || m.capabilityRevocations[capabilityRevocationKey(util.UUIDToString(p.WorkspaceID), id)].GrantID != "" ||
		util.UUIDToString(row.IssueID) != util.UUIDToString(p.IssueID) || util.UUIDToString(row.ExecutorAgentID) != util.UUIDToString(p.ExecutorAgentID) ||
		util.UUIDToString(row.RuntimeID) != util.UUIDToString(p.RuntimeID) || row.RuntimeBindingReceiptID != p.RuntimeBindingReceiptID || !row.ExpiresAt.Time.After(p.Now.Time) {
		return db.GuardedTaskCapabilityGrant{}, pgx.ErrNoRows
	}
	return row, nil
}

func (m *memoryPermissionReceiptQueries) RevokeGuardedTaskCapabilityGrant(_ context.Context, p db.RevokeGuardedTaskCapabilityGrantParams) (db.GuardedTaskCapabilityRevocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target, exists := m.capabilityGrants[p.GrantID]
	if !exists || util.UUIDToString(target.WorkspaceID) != util.UUIDToString(p.WorkspaceID) || util.UUIDToString(target.IssuedByControllerID) != util.UUIDToString(p.RevokedByControllerID) {
		return db.GuardedTaskCapabilityRevocation{}, pgx.ErrNoRows
	}
	key := capabilityRevocationKey(util.UUIDToString(p.WorkspaceID), p.GrantID)
	if _, exists = m.capabilityRevocations[key]; exists {
		return db.GuardedTaskCapabilityRevocation{}, pgx.ErrNoRows
	}
	row := db.GuardedTaskCapabilityRevocation{GrantID: p.GrantID, WorkspaceID: p.WorkspaceID, RevokedByControllerID: p.RevokedByControllerID, Reason: p.Reason, RevokedAt: p.RevokedAt}
	m.capabilityRevocations[key] = row
	return row, nil
}

func (m *memoryPermissionReceiptQueries) GetGuardedTaskCapabilityRevocation(_ context.Context, p db.GetGuardedTaskCapabilityRevocationParams) (db.GuardedTaskCapabilityRevocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.capabilityRevocations[capabilityRevocationKey(util.UUIDToString(p.WorkspaceID), p.GrantID)]
	if !ok {
		return row, pgx.ErrNoRows
	}
	return row, nil
}

func (m *memoryPermissionReceiptQueries) ConsumeGuardedExecutionAuthority(_ context.Context, p db.ConsumeGuardedExecutionAuthorityParams) (db.GuardedExecutionLaunchConsumption, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	taskKey := util.UUIDToString(p.WorkspaceID) + "/" + util.UUIDToString(p.TaskID)
	grant, grantOK := m.capabilityGrants[p.CapabilityGrantID]
	binding, bindingOK := m.runtimeReceipts[p.RuntimeBindingReceiptID]
	_, alreadyConsumed := m.consumptions[taskKey]
	grantRevoked := m.capabilityRevocations[capabilityRevocationKey(util.UUIDToString(p.WorkspaceID), p.CapabilityGrantID)].GrantID != ""
	bindingRevoked := m.runtimeRevocations[runtimeRevocationKey(util.UUIDToString(p.WorkspaceID), p.RuntimeBindingReceiptID)].ReceiptID != ""
	if !grantOK || !bindingOK || alreadyConsumed || grantRevoked || bindingRevoked ||
		util.UUIDToString(grant.TaskID) != util.UUIDToString(p.TaskID) ||
		util.UUIDToString(grant.WorkspaceID) != util.UUIDToString(p.WorkspaceID) ||
		util.UUIDToString(grant.IssueID) != util.UUIDToString(p.IssueID) ||
		util.UUIDToString(grant.ExecutorAgentID) != util.UUIDToString(p.ExecutorAgentID) ||
		util.UUIDToString(grant.RuntimeID) != util.UUIDToString(p.RuntimeID) ||
		grant.RuntimeBindingReceiptID != p.RuntimeBindingReceiptID ||
		util.UUIDToString(grant.IssuedByControllerID) != util.UUIDToString(p.ExpectedControllerID) ||
		util.UUIDToString(binding.IssuedByControllerID) != util.UUIDToString(p.ExpectedControllerID) ||
		!grant.ExpiresAt.Time.After(p.ConsumedAt.Time) || !binding.ExpiresAt.Time.After(p.ConsumedAt.Time) ||
		m.daemonIDs[util.UUIDToString(p.RuntimeID)] != p.ConsumedByDaemonID {
		return db.GuardedExecutionLaunchConsumption{}, pgx.ErrNoRows
	}
	for _, existing := range m.consumptions {
		if existing.CapabilityGrantID == p.CapabilityGrantID {
			return db.GuardedExecutionLaunchConsumption{}, pgx.ErrNoRows
		}
	}
	row := db.GuardedExecutionLaunchConsumption{
		TaskID: p.TaskID, WorkspaceID: p.WorkspaceID, IssueID: p.IssueID,
		ExecutorAgentID: p.ExecutorAgentID, RuntimeID: p.RuntimeID,
		RuntimeBindingReceiptID: p.RuntimeBindingReceiptID, CapabilityGrantID: p.CapabilityGrantID,
		ConsumedByDaemonID: p.ConsumedByDaemonID, PackageSha256: p.PackageSha256,
		ConsumedAt: p.ConsumedAt,
	}
	m.consumptions[taskKey] = row
	return row, nil
}

func runtimeRevocationKey(workspaceID, receiptID string) string  { return workspaceID + "/" + receiptID }
func capabilityRevocationKey(workspaceID, grantID string) string { return workspaceID + "/" + grantID }
