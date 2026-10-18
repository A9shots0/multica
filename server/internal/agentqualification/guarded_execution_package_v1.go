package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const GuardedExecutionPackageVersionV1 = "silver-mediums.guarded-execution-package.v1"

// GuardedExecutionPackageV1 is the typed, server-authenticated claim-wire
// payload for a future positively qualified guarded task. It is transport, not
// authority: the server must load every receipt from controller-owned durable
// storage before constructing it. Task context must never supply this package.
type GuardedExecutionPackageV1 struct {
	Version           string                           `json:"version"`
	Envelope          GuardedDispatchEnvelopeV1        `json:"dispatch_envelope"`
	RuntimeBinding    QualifiedRuntimeBindingReceiptV2 `json:"runtime_binding"`
	CapabilityScope   TaskCapabilityScopeV1            `json:"capability_scope"`
	PluginToolTargets map[string][]string              `json:"plugin_tool_targets,omitempty"`
}

// GuardedExecutionExpectedV1 contains facts independently derived at the
// daemon from the claimed task and resolved runtime. It deliberately does not
// accept values copied from the package being validated.
type GuardedExecutionExpectedV1 struct {
	WorkspaceID     string
	IssueID         string
	ExecutorAgentID string
	RuntimeID       string
	RuntimeProvider string
	Model           string
	ReasoningEffort string
	HostAttestation string
	AgentName       string
}

// GuardedExecutionPackageSHA256V1 returns the canonical digest presented by a
// daemon when it asks the controller to consume launch authority. Both sides
// hash the typed package with encoding/json so the start gate binds the exact
// in-memory package to the immutable controller-owned rows.
func GuardedExecutionPackageSHA256V1(pkg *GuardedExecutionPackageV1) (string, error) {
	if pkg == nil {
		return "", fmt.Errorf("%w: guarded execution package is required", ErrGuardedDispatchInvalid)
	}
	raw, err := json.Marshal(pkg)
	if err != nil {
		return "", fmt.Errorf("%w: encode guarded execution package: %v", ErrGuardedDispatchInvalid, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ValidateGuardedExecutionPackageV1 validates the strict positive wire
// package. A nil package is an ordinary historical task and remains legacy.
func ValidateGuardedExecutionPackageV1(pkg *GuardedExecutionPackageV1, expected GuardedExecutionExpectedV1, now time.Time) (GuardedDispatchAdmissionV1, error) {
	if pkg == nil {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeLegacy}, nil
	}
	if pkg.Version != GuardedExecutionPackageVersionV1 {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: unsupported guarded execution package", ErrGuardedDispatchInvalid)
	}
	if strings.TrimSpace(expected.WorkspaceID) == "" || strings.TrimSpace(expected.IssueID) == "" || strings.TrimSpace(expected.ExecutorAgentID) == "" || strings.TrimSpace(expected.RuntimeID) == "" || strings.TrimSpace(expected.RuntimeProvider) == "" || strings.TrimSpace(expected.Model) == "" || strings.TrimSpace(expected.ReasoningEffort) == "" || strings.TrimSpace(expected.HostAttestation) == "" {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: incomplete guarded execution facts", ErrGuardedDispatchInvalid)
	}
	rawEnvelope, err := json.Marshal(pkg.Envelope)
	if err != nil {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: encode dispatch envelope: %v", ErrGuardedDispatchInvalid, err)
	}
	admission, err := ValidateGuardedDispatchV1(rawEnvelope, &pkg.RuntimeBinding, now)
	if err != nil {
		return admission, err
	}
	if pkg.Envelope.WorkspaceID != expected.WorkspaceID || pkg.Envelope.IssueID != expected.IssueID || pkg.Envelope.ExecutorAgentID != expected.ExecutorAgentID || pkg.Envelope.RuntimeID != expected.RuntimeID || pkg.Envelope.RuntimeProvider != expected.RuntimeProvider || pkg.Envelope.Model != expected.Model || pkg.Envelope.ReasoningEffort != expected.ReasoningEffort {
		return admission, fmt.Errorf("%w: guarded execution facts do not match dispatch", ErrGuardedDispatchInvalid)
	}
	scope := pkg.CapabilityScope
	if (scope.Version != TaskCapabilityScopeVersionV1 && scope.Version != TaskCapabilityScopeVersionV2 && scope.Version != TaskCapabilityScopeVersionV3) || scope.WorkspaceID != expected.WorkspaceID || scope.IssueID != expected.IssueID || scope.ExecutorAgentID != expected.ExecutorAgentID || scope.RuntimeID != expected.RuntimeID || scope.RoleID != admission.Envelope.RoleID || scope.ConfigurationID != admission.Envelope.ConfigurationID || scope.RuntimeBindingReceiptID != admission.RuntimeBinding.ID || scope.HostAttestation != expected.HostAttestation {
		return admission, fmt.Errorf("%w: capability scope does not match guarded execution", ErrGuardedDispatchInvalid)
	}
	switch scope.Version {
	case TaskCapabilityScopeVersionV1:
		if scope.ModelOnlyShadow != nil || scope.WorkflowBroker != nil {
			return admission, fmt.Errorf("%w: v1 capability scope cannot grant model-only execution", ErrGuardedDispatchInvalid)
		}
	case TaskCapabilityScopeVersionV2:
		if scope.WorkflowBroker != nil {
			return admission, fmt.Errorf("%w: v2 capability scope cannot grant a workflow broker", ErrGuardedDispatchInvalid)
		}
		if err := validateGuardedModelOnlyShadowV1(scope.ModelOnlyShadow, admission.Envelope.ArtifactBinding, admission.Envelope.RuntimeProvider, scope.Receipt.CredentialPurposes); err != nil {
			return admission, err
		}
	case TaskCapabilityScopeVersionV3:
		if scope.ModelOnlyShadow != nil || admission.RuntimeBinding.WorkflowBroker == nil {
			return admission, fmt.Errorf("%w: v3 capability scope requires a workflow broker", ErrGuardedDispatchInvalid)
		}
		if err := validateGuardedWorkflowBrokerV1(scope.WorkflowBroker, admission.RuntimeBinding.WorkflowBroker, admission.RuntimeBinding.ChildExecution, expected.AgentName); err != nil {
			return admission, err
		}
	}
	if err := validatePluginToolTargetsV1(pkg.PluginToolTargets, scope.Receipt.TargetNodeIDs); err != nil {
		return admission, err
	}
	return admission, nil
}

func validatePluginToolTargetsV1(bindings map[string][]string, authorizedTargets []string) error {
	allowedTargets := make(map[string]struct{}, len(authorizedTargets))
	for _, target := range authorizedTargets {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("%w: empty authorized target", ErrGuardedDispatchInvalid)
		}
		allowedTargets[target] = struct{}{}
	}
	for tool, targets := range bindings {
		if strings.TrimSpace(tool) == "" || len(targets) == 0 {
			return fmt.Errorf("%w: incomplete plugin target binding", ErrGuardedDispatchInvalid)
		}
		seen := map[string]struct{}{}
		for _, target := range targets {
			if _, ok := allowedTargets[target]; !ok {
				return fmt.Errorf("%w: plugin target is outside capability scope", ErrGuardedDispatchInvalid)
			}
			if _, duplicate := seen[target]; duplicate {
				return fmt.Errorf("%w: duplicate plugin target", ErrGuardedDispatchInvalid)
			}
			seen[target] = struct{}{}
		}
	}
	return nil
}

// DecodeGuardedExecutionPackageV1 is strict so future/unknown fields cannot
// silently become authority when old daemons decode a newer controller wire.
func DecodeGuardedExecutionPackageV1(raw json.RawMessage) (*GuardedExecutionPackageV1, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var pkg GuardedExecutionPackageV1
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&pkg); err != nil {
		return nil, fmt.Errorf("%w: decode guarded execution package: %v", ErrGuardedDispatchInvalid, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing guarded execution package", ErrGuardedDispatchInvalid)
	}
	return &pkg, nil
}
