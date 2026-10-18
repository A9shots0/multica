package agentqualification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWorkflowBrokerAuthorityUsesDurableLifecycleAndBindsExactTask(t *testing.T) {
	now := time.Date(2026, 9, 15, 4, 45, 0, 0, time.UTC)
	queries := newMemoryPermissionReceiptQueries()
	store := NewPermissionReceiptStore(queries)
	receipt := workflowBrokerRuntimeReceiptFixture(t, now)
	if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
		t.Fatal(err)
	}
	base := json.RawMessage(`{"workflow_assignment_v1":{"opaque":"preserved"}}`)
	prepared, guarded, err := store.PrepareWorkflowBrokerTaskAuthority(
		context.Background(), permissionControllerID, permissionTaskID, permissionWorkspaceID,
		permissionIssueID, permissionAgentID, permissionRuntimeID, "review", base, now,
	)
	if err != nil || !guarded {
		t.Fatalf("prepare guarded broker authority = guarded %v err %v", guarded, err)
	}
	var contextObject map[string]json.RawMessage
	if json.Unmarshal(prepared, &contextObject) != nil || contextObject[GuardedDispatchTaskContextKey] == nil || contextObject["workflow_assignment_v1"] == nil {
		t.Fatalf("prepared context did not preserve assignment and add guarded dispatch: %s", prepared)
	}
	admission, err := ValidateGuardedTaskContextV1(prepared, nil, now)
	if !errors.Is(err, ErrGuardedConfigurationUnbound) || admission.Mode != GuardedDispatchModeGuarded {
		t.Fatalf("guarded context selection = %+v, %v", admission, err)
	}
	pkg, err := store.LoadGuardedExecutionPackage(context.Background(), permissionLoadIdentity(), admission.Envelope, now)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.CapabilityScope.WorkflowBroker == nil || pkg.CapabilityScope.WorkflowBroker.TaskKind != "review" ||
		pkg.RuntimeBinding.ChildExecution.ExecutableSHA256 != strings.Repeat("a", 64) ||
		pkg.RuntimeBinding.ChildExecution.ProviderExecutableSHA256 != strings.Repeat("b", 64) {
		t.Fatalf("loaded broker package drifted: %+v", pkg)
	}
	digest, err := GuardedExecutionPackageSHA256V1(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(context.Background(), permissionLoadIdentity(), admission.Envelope, digest, permissionDaemonID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeGuardedExecutionAuthority(context.Background(), permissionLoadIdentity(), admission.Envelope, digest, permissionDaemonID, now); !errors.Is(err, ErrGuardedAuthorityUnavailable) {
		t.Fatalf("replayed broker authority error = %v", err)
	}
}

func TestWorkflowBrokerAuthorityValidatesPathsForAttestedHostOS(t *testing.T) {
	for _, tc := range []struct {
		name, value, hostOS string
		want                bool
	}{
		{name: "windows drive path", value: `A:\item14\broker.exe`, hostOS: "windows", want: true},
		{name: "windows provider profile", value: `C:\Users\fixture\.claude`, hostOS: "windows", want: true},
		{name: "windows slash drift", value: `A:/item14/broker.exe`, hostOS: "windows"},
		{name: "windows traversal", value: `A:\item14\..\broker.exe`, hostOS: "windows"},
		{name: "windows duplicate separator", value: `A:\item14\\broker.exe`, hostOS: "windows"},
		{name: "linux path", value: "/opt/item14/broker", hostOS: "linux", want: true},
		{name: "linux traversal", value: "/opt/item14/../broker", hostOS: "linux"},
		{name: "foreign path", value: `A:\item14\broker.exe`, hostOS: "linux"},
		{name: "unknown host", value: "/opt/item14/broker", hostOS: "plan9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateExactAuthorityPathForHostV1(tc.value, tc.hostOS); got != tc.want {
				t.Fatalf("validate path = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWorkflowBrokerAuthorityFailsClosedForMissingAlteredExpiredAndRevokedTemplate(t *testing.T) {
	now := time.Date(2026, 9, 15, 4, 45, 0, 0, time.UTC)
	missing := NewPermissionReceiptStore(newMemoryPermissionReceiptQueries())
	if got, guarded, err := missing.PrepareWorkflowBrokerTaskAuthority(context.Background(), permissionControllerID, permissionTaskID, permissionWorkspaceID, permissionIssueID, permissionAgentID, permissionRuntimeID, "review", []byte(`{}`), now); err != nil || guarded || string(got) != `{}` {
		t.Fatalf("missing template changed legacy task: guarded %v context %s err %v", guarded, got, err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*PermissionReceiptStore, *QualifiedRuntimeBindingReceiptV2)
		at     time.Time
		kind   string
	}{
		{name: "altered task kind", kind: "implementation", at: now},
		{name: "expired", kind: "review", at: now.Add(2 * time.Hour)},
		{name: "revoked", kind: "review", at: now, mutate: func(store *PermissionReceiptStore, receipt *QualifiedRuntimeBindingReceiptV2) {
			if err := store.RevokeQualifiedRuntimeBinding(context.Background(), permissionControllerID, permissionWorkspaceID, receipt.ID, "withdrawn", now); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queries := newMemoryPermissionReceiptQueries()
			store := NewPermissionReceiptStore(queries)
			receipt := workflowBrokerRuntimeReceiptFixture(t, now)
			if _, err := store.StoreQualifiedRuntimeBinding(context.Background(), permissionControllerID, receipt, now); err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(store, &receipt)
			}
			if _, guarded, err := store.PrepareWorkflowBrokerTaskAuthority(context.Background(), permissionControllerID, permissionTaskID, permissionWorkspaceID, permissionIssueID, permissionAgentID, permissionRuntimeID, tc.kind, []byte(`{}`), tc.at); err == nil || !guarded {
				t.Fatalf("denied template = guarded %v err %v", guarded, err)
			}
		})
	}
}

func workflowBrokerRuntimeReceiptFixture(t *testing.T, now time.Time) QualifiedRuntimeBindingReceiptV2 {
	t.Helper()
	artifact, err := ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		t.Fatal(err)
	}
	artifactHash, err := roleArtifactBindingSHA256V2(artifact)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := GuardedWorkflowBrokerEnvironmentKeysV1("windows")
	if err != nil {
		t.Fatal(err)
	}
	environmentHash, err := GuardedEnvironmentAllowlistSHA256V1(keys)
	if err != nil {
		t.Fatal(err)
	}
	return QualifiedRuntimeBindingReceiptV2{
		Version:     QualifiedRuntimeBindingVersionV2,
		ID:          WorkflowBrokerRuntimeReceiptIDV1(permissionIssueID, permissionAgentID),
		WorkspaceID: permissionWorkspaceID, IssueID: permissionIssueID, ExecutorAgentID: permissionAgentID,
		RuntimeID: permissionRuntimeID, ConfigurationID: "item14.workflow-broker.fixture.v1", RoleID: artifact.RoleID,
		RuntimeProvider: "claude", Model: "claude-sonnet-4-6", ReasoningEffort: "medium",
		ChildExecution: GuardedChildExecutionBoundaryV1{
			Version: GuardedChildExecutionBoundaryVersionV1, HostOS: "windows", Adapter: GuardedClaudeWorkflowBrokerAdapterV1,
			EnvironmentPolicy: GuardedEnvironmentPolicyExactV1, EnvironmentAllowlistSHA256: environmentHash,
			ToolPolicy: GuardedToolPolicyNoneV1, HostAttestationCustody: GuardedHostAttestationParentOnlyV1,
			ExecutableSHA256: strings.Repeat("a", 64), ProviderExecutableSHA256: strings.Repeat("b", 64),
		},
		WorkflowBroker: &GuardedWorkflowBrokerTemplateV1{
			Version: GuardedWorkflowBrokerTemplateVersionV1, AgentName: "Item 14 Replay Reviewer",
			BrokerExecutablePath: `C:\item14-fixture\broker.exe`, ProviderExecutablePath: `C:\item14-fixture\provider.exe`,
			ProviderExecutableSHA256: strings.Repeat("b", 64), ProviderProfileKind: GuardedWorkflowBrokerProfileFixtureV1,
			HostAttestation: "alienware_windows", AllowedTaskKinds: []string{"review"},
		},
		ArtifactBindingSHA256: artifactHash, QualificationStatus: StatusQualifiedBounded,
		EvidenceReceiptIDs: []string{"item14-workflow-broker-review"}, Source: qualifiedRuntimeBindingSourceV2,
		Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
}
