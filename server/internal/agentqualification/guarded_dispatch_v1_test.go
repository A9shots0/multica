package agentqualification

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGuardedDispatchAbsentEnvelopePreservesLegacy(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, {}, []byte(" \n\t")} {
		admission, err := ValidateGuardedDispatchV1(raw, nil, time.Now())
		if err != nil {
			t.Fatalf("legacy envelope %q: %v", raw, err)
		}
		if admission.Mode != GuardedDispatchModeLegacy {
			t.Fatalf("mode = %q, want legacy", admission.Mode)
		}
	}
}

func TestGuardedTaskContextPreservesLegacyWithoutReservedKey(t *testing.T) {
	for _, raw := range []json.RawMessage{
		nil,
		[]byte(`{"workflow":"legacy"}`),
		[]byte(`not-json legacy context`),
		[]byte(`["legacy","context"]`),
		[]byte(`{"legacy":`),
	} {
		admission, err := ValidateGuardedTaskContextV1(raw, nil, time.Now())
		if err != nil || admission.Mode != GuardedDispatchModeLegacy {
			t.Errorf("legacy context %q admission=%+v error=%v", raw, admission, err)
		}
	}
}

func TestGuardedTaskContextExtractsReservedEnvelopeAndRejectsUnbound(t *testing.T) {
	envelope := guardedDispatchFixture(t)
	rawEnvelope := mustGuardedDispatchJSON(t, envelope)
	contextRaw, err := json.Marshal(map[string]json.RawMessage{
		"unrelated":                   json.RawMessage(`{"preserved":true}`),
		GuardedDispatchTaskContextKey: rawEnvelope,
	})
	if err != nil {
		t.Fatal(err)
	}
	admission, err := ValidateGuardedTaskContextV1(contextRaw, nil, time.Now())
	if !errors.Is(err, ErrGuardedConfigurationUnbound) || admission.Mode != GuardedDispatchModeGuarded {
		t.Fatalf("guarded context admission=%+v error=%v", admission, err)
	}
}

func TestGuardedTaskContextReservedKeyFailsClosed(t *testing.T) {
	for _, raw := range []json.RawMessage{
		[]byte(`{"guarded_dispatch":null}`),
		[]byte(`{"guarded_dispatch":{},"guarded_dispatch":{}}`),
		[]byte(`{"guarded_dispatch":{},"later":`),
		[]byte(`{"guarded_dispatch":{}} trailing`),
	} {
		admission, err := ValidateGuardedTaskContextV1(raw, nil, time.Now())
		if !errors.Is(err, ErrGuardedDispatchInvalid) || admission.Mode != GuardedDispatchModeGuarded {
			t.Errorf("reserved context %q admission=%+v error=%v", raw, admission, err)
		}
	}
}

func TestGuardedDispatchV2RejectsConfigurationWithoutRuntimeBinding(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	envelope := guardedDispatchFixture(t)
	raw := mustGuardedDispatchJSON(t, envelope)

	admission, err := ValidateGuardedDispatchV1(raw, nil, now)
	if !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("error = %v, want %s", err, GuardedConfigurationUnbound)
	}
	if admission.Mode != GuardedDispatchModeGuarded || admission.Artifact.BindingID != envelope.ArtifactBindingID {
		t.Fatalf("rejection lost guarded artifact context: %+v", admission)
	}
}

func TestGuardedDispatchV2RequiresExactArtifactBindingBeforeRuntimeQualification(t *testing.T) {
	envelope := guardedDispatchFixture(t)
	envelope.ArtifactBinding.Instruction.SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err := ValidateGuardedDispatchV1(mustGuardedDispatchJSON(t, envelope), nil, time.Now())
	if !errors.Is(err, ErrGuardedDispatchInvalid) {
		t.Fatalf("artifact drift error = %v, want invalid guarded dispatch", err)
	}
}

func TestGuardedDispatchPresentMalformedOrUnknownEnvelopeFailsClosed(t *testing.T) {
	for _, raw := range []json.RawMessage{
		[]byte(`{}`),
		[]byte(`{"version":"silver-mediums.guarded-dispatch.v3"}`),
		[]byte(`{"version":"silver-mediums.guarded-dispatch.v2","unknown":true}`),
		[]byte(`{"version":"silver-mediums.guarded-dispatch.v2"} {}`),
	} {
		admission, err := ValidateGuardedDispatchV1(raw, nil, time.Now())
		if !errors.Is(err, ErrGuardedDispatchInvalid) {
			t.Errorf("raw %s error = %v, want invalid", raw, err)
		}
		if admission.Mode != GuardedDispatchModeGuarded {
			t.Errorf("raw %s mode = %q, want guarded", raw, admission.Mode)
		}
	}
}

func TestGuardedDispatchRuntimeReceiptCannotSubstituteRuntimeOrArtifacts(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	envelope := guardedDispatchFixture(t)
	receipt := guardedRuntimeBindingFixture(t, envelope, now)

	if _, err := ValidateGuardedDispatchV1(mustGuardedDispatchJSON(t, envelope), &receipt, now); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}

	mutations := []func(*QualifiedRuntimeBindingReceiptV2){
		func(r *QualifiedRuntimeBindingReceiptV2) { r.WorkspaceID = "other-workspace" },
		func(r *QualifiedRuntimeBindingReceiptV2) { r.IssueID = "other-issue" },
		func(r *QualifiedRuntimeBindingReceiptV2) { r.ExecutorAgentID = "other-agent" },
		func(r *QualifiedRuntimeBindingReceiptV2) { r.RuntimeID = "other-runtime" },
		func(r *QualifiedRuntimeBindingReceiptV2) { r.Model = "other-model" },
		func(r *QualifiedRuntimeBindingReceiptV2) {
			r.ArtifactBindingSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		func(r *QualifiedRuntimeBindingReceiptV2) { r.QualificationStatus = StatusProvisionalShadow },
		func(r *QualifiedRuntimeBindingReceiptV2) { r.Revoked = true },
		func(r *QualifiedRuntimeBindingReceiptV2) { r.ExpiresAt = now },
	}
	for i, mutate := range mutations {
		changed := receipt
		mutate(&changed)
		if _, err := ValidateGuardedDispatchV1(mustGuardedDispatchJSON(t, envelope), &changed, now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
			t.Errorf("mutation %d error = %v, want configuration unbound", i, err)
		}
	}
}

func TestGuardedDispatchLegacyV1RuntimeReceiptCannotMasqueradeAsChildBoundV2(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	envelope := guardedDispatchFixture(t)
	receipt := guardedRuntimeBindingFixture(t, envelope, now)
	receipt.Version = QualifiedRuntimeBindingVersionV1
	receipt.ChildExecution = GuardedChildExecutionBoundaryV1{}

	if _, err := ValidateGuardedDispatchV1(mustGuardedDispatchJSON(t, envelope), &receipt, now); !errors.Is(err, ErrGuardedConfigurationUnbound) {
		t.Fatalf("legacy v1 runtime receipt error = %v, want configuration unbound", err)
	}
}

func TestGuardedDispatchExecutionIdentityRejectsEveryTaskSubstitution(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	envelope := guardedDispatchFixture(t)
	receipt := guardedRuntimeBindingFixture(t, envelope, now)
	rawEnvelope := mustGuardedDispatchJSON(t, envelope)
	rawContext, err := json.Marshal(map[string]json.RawMessage{GuardedDispatchTaskContextKey: rawEnvelope})
	if err != nil {
		t.Fatal(err)
	}
	expected := GuardedDispatchExpectedIdentityV1{
		WorkspaceID: envelope.WorkspaceID, IssueID: envelope.IssueID,
		ExecutorAgentID: envelope.ExecutorAgentID, RuntimeID: envelope.RuntimeID,
	}
	if _, err := ValidateGuardedTaskContextForExecutionV1(rawContext, &receipt, expected, now); err != nil {
		t.Fatalf("exact execution identity rejected: %v", err)
	}

	mutations := []func(*GuardedDispatchExpectedIdentityV1){
		func(v *GuardedDispatchExpectedIdentityV1) { v.WorkspaceID = "other-workspace" },
		func(v *GuardedDispatchExpectedIdentityV1) { v.IssueID = "other-issue" },
		func(v *GuardedDispatchExpectedIdentityV1) { v.ExecutorAgentID = "other-agent" },
		func(v *GuardedDispatchExpectedIdentityV1) { v.RuntimeID = "other-runtime" },
		func(v *GuardedDispatchExpectedIdentityV1) { v.RuntimeID = "" },
	}
	for i, mutate := range mutations {
		changed := expected
		mutate(&changed)
		if _, err := ValidateGuardedTaskContextForExecutionV1(rawContext, &receipt, changed, now); !errors.Is(err, ErrGuardedDispatchInvalid) {
			t.Errorf("identity mutation %d error = %v, want invalid guarded dispatch", i, err)
		}
	}
}

func TestGuardedDispatchExecutionIdentityChecksBeforeUnboundReceipt(t *testing.T) {
	envelope := guardedDispatchFixture(t)
	rawEnvelope := mustGuardedDispatchJSON(t, envelope)
	rawContext, err := json.Marshal(map[string]json.RawMessage{GuardedDispatchTaskContextKey: rawEnvelope})
	if err != nil {
		t.Fatal(err)
	}
	expected := GuardedDispatchExpectedIdentityV1{
		WorkspaceID: envelope.WorkspaceID, IssueID: "substituted-issue",
		ExecutorAgentID: envelope.ExecutorAgentID, RuntimeID: envelope.RuntimeID,
	}
	_, err = ValidateGuardedTaskContextForExecutionV1(rawContext, nil, expected, time.Now())
	if !errors.Is(err, ErrGuardedDispatchInvalid) {
		t.Fatalf("error = %v, want identity rejection before unbound receipt", err)
	}
}

func guardedDispatchFixture(t *testing.T) GuardedDispatchEnvelopeV1 {
	t.Helper()
	const roleID = "data_sync_integrity"
	const artifactBindingID = "silver-mediums.role-artifact.v2.data_sync_integrity"
	artifact, err := ValidatedRoleArtifactBindingV2(artifactBindingID)
	if err != nil {
		t.Fatal(err)
	}
	return GuardedDispatchEnvelopeV1{
		Version: GuardedDispatchEnvelopeVersionV2, ConfigurationID: "future.data-sync.runtime.v1",
		WorkspaceID: "workspace-1", IssueID: "issue-1", RoleID: roleID,
		ExecutorAgentID: "agent-1", RuntimeID: "runtime-1",
		RuntimeProvider: "claude", Model: "future-model",
		ReasoningEffort: "future-effort", ArtifactBindingID: artifactBindingID, ArtifactBinding: artifact,
		RuntimeBindingReceiptID: "runtime-binding-1",
	}
}

func guardedRuntimeBindingFixture(t *testing.T, envelope GuardedDispatchEnvelopeV1, now time.Time) QualifiedRuntimeBindingReceiptV2 {
	t.Helper()
	hash, err := roleArtifactBindingSHA256V2(envelope.ArtifactBinding)
	if err != nil {
		t.Fatal(err)
	}
	return QualifiedRuntimeBindingReceiptV2{
		Version: QualifiedRuntimeBindingVersionV2, ID: envelope.RuntimeBindingReceiptID,
		WorkspaceID: envelope.WorkspaceID, IssueID: envelope.IssueID, ExecutorAgentID: envelope.ExecutorAgentID,
		ConfigurationID: envelope.ConfigurationID, RoleID: envelope.RoleID,
		RuntimeID: envelope.RuntimeID, RuntimeProvider: envelope.RuntimeProvider,
		Model: envelope.Model, ReasoningEffort: envelope.ReasoningEffort,
		ChildExecution:        guardedChildExecutionFixture(t, envelope.RuntimeProvider, "windows"),
		ArtifactBindingSHA256: hash, QualificationStatus: StatusQualifiedBounded,
		EvidenceReceiptIDs: []string{"qualification-evidence-1"}, Source: qualifiedRuntimeBindingSourceV2,
		Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
	}
}

func guardedChildExecutionFixture(t *testing.T, provider, hostOS string) GuardedChildExecutionBoundaryV1 {
	t.Helper()
	keys, err := GuardedChildEnvironmentKeysV1(provider, hostOS)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := GuardedEnvironmentAllowlistSHA256V1(keys)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := GuardedAdapterV1(provider)
	if err != nil {
		t.Fatal(err)
	}
	return GuardedChildExecutionBoundaryV1{
		Version: GuardedChildExecutionBoundaryVersionV1, HostOS: hostOS, Adapter: adapter,
		EnvironmentPolicy: GuardedEnvironmentPolicyExactV1, EnvironmentAllowlistSHA256: digest,
		ToolPolicy: GuardedToolPolicyNoneV1, HostAttestationCustody: GuardedHostAttestationParentOnlyV1,
		ExecutableSHA256: strings.Repeat("a", 64),
	}
}

func mustGuardedDispatchJSON(t *testing.T, envelope GuardedDispatchEnvelopeV1) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
