package agentqualification

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGuardedExecutionPackageNilPreservesLegacy(t *testing.T) {
	admission, err := ValidateGuardedExecutionPackageV1(nil, GuardedExecutionExpectedV1{}, time.Now())
	if err != nil || admission.Mode != GuardedDispatchModeLegacy {
		t.Fatalf("admission=%+v err=%v", admission, err)
	}
}

func TestGuardedExecutionPackageExactFactsPass(t *testing.T) {
	pkg, expected, now := guardedExecutionPackageFixture(t)
	admission, err := ValidateGuardedExecutionPackageV1(&pkg, expected, now)
	if err != nil || admission.Mode != GuardedDispatchModeGuarded {
		t.Fatalf("admission=%+v err=%v", admission, err)
	}
}

func TestGuardedExecutionPackageRejectsFactAndScopeSubstitution(t *testing.T) {
	mutations := map[string]func(*GuardedExecutionPackageV1, *GuardedExecutionExpectedV1){
		"workspace": func(_ *GuardedExecutionPackageV1, e *GuardedExecutionExpectedV1) { e.WorkspaceID = "other" },
		"provider":  func(_ *GuardedExecutionPackageV1, e *GuardedExecutionExpectedV1) { e.RuntimeProvider = "other" },
		"model":     func(_ *GuardedExecutionPackageV1, e *GuardedExecutionExpectedV1) { e.Model = "other" },
		"effort":    func(_ *GuardedExecutionPackageV1, e *GuardedExecutionExpectedV1) { e.ReasoningEffort = "other" },
		"host":      func(_ *GuardedExecutionPackageV1, e *GuardedExecutionExpectedV1) { e.HostAttestation = "mac_studio" },
		"role":      func(p *GuardedExecutionPackageV1, _ *GuardedExecutionExpectedV1) { p.CapabilityScope.RoleID = "other" },
		"config": func(p *GuardedExecutionPackageV1, _ *GuardedExecutionExpectedV1) {
			p.CapabilityScope.ConfigurationID = "other"
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			pkg, expected, now := guardedExecutionPackageFixture(t)
			mutate(&pkg, &expected)
			if _, err := ValidateGuardedExecutionPackageV1(&pkg, expected, now); err == nil {
				t.Fatal("substitution was accepted")
			}
		})
	}
}

func TestGuardedExecutionPackageRejectsPluginTargetDrift(t *testing.T) {
	pkg, expected, now := guardedExecutionPackageFixture(t)
	pkg.PluginToolTargets["plugin__read"] = []string{"linkhub_customer"}
	if _, err := ValidateGuardedExecutionPackageV1(&pkg, expected, now); err == nil {
		t.Fatal("out-of-scope plugin target was accepted")
	}
	pkg, expected, now = guardedExecutionPackageFixture(t)
	pkg.PluginToolTargets["plugin__read"] = []string{"crm_product", "crm_product"}
	if _, err := ValidateGuardedExecutionPackageV1(&pkg, expected, now); err == nil {
		t.Fatal("duplicate plugin target was accepted")
	}
}

func TestDecodeGuardedExecutionPackageRejectsUnknownAndTrailingData(t *testing.T) {
	for _, raw := range []json.RawMessage{
		[]byte(`{"version":"silver-mediums.guarded-execution-package.v1","unknown":true}`),
		[]byte(`{"version":"silver-mediums.guarded-execution-package.v1"}{}`),
	} {
		if _, err := DecodeGuardedExecutionPackageV1(raw); err == nil {
			t.Fatalf("accepted invalid wire: %s", raw)
		}
	}
}

func guardedExecutionPackageFixture(t *testing.T) (GuardedExecutionPackageV1, GuardedExecutionExpectedV1, time.Time) {
	t.Helper()
	now := time.Now().UTC()
	envelope := guardedDispatchFixture(t)
	receipt := guardedRuntimeBindingFixture(t, envelope, now)
	scope, _ := taskCapabilityFixture(now)
	scope.WorkspaceID = envelope.WorkspaceID
	scope.IssueID = envelope.IssueID
	scope.ExecutorAgentID = envelope.ExecutorAgentID
	scope.RuntimeID = envelope.RuntimeID
	scope.RoleID = envelope.RoleID
	scope.ConfigurationID = envelope.ConfigurationID
	scope.RuntimeBindingReceiptID = receipt.ID
	scope.HostAttestation = "alienware_windows"
	scope.Receipt.ExecutorAgentID = envelope.ExecutorAgentID
	scope.Receipt.IssueID = envelope.IssueID
	scope.Receipt.RoleID = envelope.RoleID
	scope.Receipt.ConfigurationID = envelope.ConfigurationID
	scope.Receipt.HostAttestation = "alienware_windows"
	pkg := GuardedExecutionPackageV1{
		Version: GuardedExecutionPackageVersionV1, Envelope: envelope, RuntimeBinding: receipt,
		CapabilityScope: scope, PluginToolTargets: map[string][]string{"plugin__read": {"crm_product"}},
	}
	expected := GuardedExecutionExpectedV1{
		WorkspaceID: envelope.WorkspaceID, IssueID: envelope.IssueID, ExecutorAgentID: envelope.ExecutorAgentID,
		RuntimeID: envelope.RuntimeID, RuntimeProvider: envelope.RuntimeProvider, Model: envelope.Model,
		ReasoningEffort: envelope.ReasoningEffort, HostAttestation: "alienware_windows",
	}
	return pkg, expected, now
}
