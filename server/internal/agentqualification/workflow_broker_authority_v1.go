package agentqualification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	GuardedWorkflowBrokerTemplateVersionV1 = "silver-mediums.workflow-broker-template.v1"
	GuardedWorkflowBrokerVersionV1         = "silver-mediums.workflow-broker.v1"
	GuardedWorkflowBrokerProfileFixtureV1  = "task_local_fixture"
	GuardedWorkflowBrokerProfileClaudeV1   = "claude_code_subscription"
)

type GuardedWorkflowBrokerTemplateV1 struct {
	Version                  string   `json:"version"`
	AgentName                string   `json:"agent_name"`
	BrokerExecutablePath     string   `json:"broker_executable_path"`
	ProviderExecutablePath   string   `json:"provider_executable_path"`
	ProviderExecutableSHA256 string   `json:"provider_executable_sha256"`
	ProviderProfileKind      string   `json:"provider_profile_kind"`
	ProviderProfilePath      string   `json:"provider_profile_path"`
	HostAttestation          string   `json:"host_attestation"`
	AllowedTaskKinds         []string `json:"allowed_task_kinds"`
}

type GuardedWorkflowBrokerV1 struct {
	Version                  string `json:"version"`
	AgentName                string `json:"agent_name"`
	TaskKind                 string `json:"task_kind"`
	BrokerExecutablePath     string `json:"broker_executable_path"`
	ProviderExecutablePath   string `json:"provider_executable_path"`
	ProviderExecutableSHA256 string `json:"provider_executable_sha256"`
	ProviderProfileKind      string `json:"provider_profile_kind"`
	ProviderProfilePath      string `json:"provider_profile_path"`
	ProviderCallLimit        int    `json:"provider_call_limit"`
	RetryLimit               int    `json:"retry_limit"`
	MaxTurns                 int    `json:"max_turns"`
}

func WorkflowBrokerRuntimeReceiptIDV1(issueID, agentID string) string {
	return "item14-workflow-broker-runtime-v1." + issueID + "." + agentID
}

func validateExactAuthorityPathForHostV1(value, hostOS string) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.IndexByte(value, 0) >= 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(hostOS)) {
	case "windows":
		if len(value) < 3 || !isASCIIAlpha(value[0]) || value[1] != ':' || value[2] != '\\' || strings.Contains(value, "/") {
			return false
		}
		if len(value) == 3 {
			return true
		}
		if strings.HasSuffix(value, "\\") {
			return false
		}
		for _, segment := range strings.Split(value[3:], "\\") {
			if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `<>:"|?*`) {
				return false
			}
			for _, char := range segment {
				if char < 0x20 {
					return false
				}
			}
		}
		return true
	case "linux", "darwin":
		return path.IsAbs(value) && path.Clean(value) == value
	default:
		return false
	}
}

func isASCIIAlpha(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func validateGuardedWorkflowBrokerTemplateV1(template *GuardedWorkflowBrokerTemplateV1, boundary GuardedChildExecutionBoundaryV1) error {
	if template == nil || template.Version != GuardedWorkflowBrokerTemplateVersionV1 ||
		strings.TrimSpace(template.AgentName) == "" || strings.TrimSpace(template.HostAttestation) == "" ||
		!validateExactAuthorityPathForHostV1(template.BrokerExecutablePath, boundary.HostOS) ||
		!validateExactAuthorityPathForHostV1(template.ProviderExecutablePath, boundary.HostOS) ||
		!validSHA256(template.ProviderExecutableSHA256) || boundary.Adapter != GuardedClaudeWorkflowBrokerAdapterV1 ||
		boundary.ProviderExecutableSHA256 != template.ProviderExecutableSHA256 {
		return ErrGuardedConfigurationUnbound
	}
	switch template.ProviderProfileKind {
	case GuardedWorkflowBrokerProfileFixtureV1:
		if template.ProviderProfilePath != "" {
			return ErrGuardedConfigurationUnbound
		}
	case GuardedWorkflowBrokerProfileClaudeV1:
		if !validateExactAuthorityPathForHostV1(template.ProviderProfilePath, boundary.HostOS) {
			return ErrGuardedConfigurationUnbound
		}
	default:
		return ErrGuardedConfigurationUnbound
	}
	if len(template.AllowedTaskKinds) == 0 || len(template.AllowedTaskKinds) > 4 {
		return ErrGuardedConfigurationUnbound
	}
	seen := map[string]struct{}{}
	for _, kind := range template.AllowedTaskKinds {
		switch kind {
		case "leader", "implementation", "review", "correction":
		default:
			return ErrGuardedConfigurationUnbound
		}
		if _, duplicate := seen[kind]; duplicate {
			return ErrGuardedConfigurationUnbound
		}
		seen[kind] = struct{}{}
	}
	return nil
}

func validateGuardedWorkflowBrokerV1(binding *GuardedWorkflowBrokerV1, template *GuardedWorkflowBrokerTemplateV1, boundary GuardedChildExecutionBoundaryV1, expectedAgentName string) error {
	if err := validateGuardedWorkflowBrokerTemplateV1(template, boundary); err != nil || binding == nil {
		return fmt.Errorf("%w: workflow broker template is not qualified", ErrGuardedDispatchInvalid)
	}
	if binding.Version != GuardedWorkflowBrokerVersionV1 || binding.ProviderCallLimit != 1 || binding.RetryLimit != 0 || binding.MaxTurns != 1 ||
		binding.AgentName != template.AgentName || binding.AgentName != expectedAgentName ||
		binding.BrokerExecutablePath != template.BrokerExecutablePath || binding.ProviderExecutablePath != template.ProviderExecutablePath ||
		binding.ProviderExecutableSHA256 != template.ProviderExecutableSHA256 || binding.ProviderProfileKind != template.ProviderProfileKind ||
		binding.ProviderProfilePath != template.ProviderProfilePath {
		return fmt.Errorf("%w: workflow broker binding mismatch", ErrGuardedDispatchInvalid)
	}
	for _, kind := range template.AllowedTaskKinds {
		if binding.TaskKind == kind {
			return nil
		}
	}
	return fmt.Errorf("%w: workflow task kind is outside controller authority", ErrGuardedDispatchInvalid)
}

func mergeGuardedDispatchContextV1(base []byte, envelope GuardedDispatchEnvelopeV1) ([]byte, error) {
	object := map[string]json.RawMessage{}
	trimmed := bytes.TrimSpace(base)
	if len(trimmed) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&object); err != nil {
			return nil, fmt.Errorf("%w: task context is not a strict object", ErrGuardedDispatchInvalid)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return nil, fmt.Errorf("%w: task context has trailing data", ErrGuardedDispatchInvalid)
		}
	}
	if _, exists := object[GuardedDispatchTaskContextKey]; exists {
		return nil, fmt.Errorf("%w: guarded dispatch context already exists", ErrGuardedDispatchInvalid)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	object[GuardedDispatchTaskContextKey] = raw
	return json.Marshal(object)
}

// PrepareWorkflowBrokerTaskAuthority converts a pre-issued, deterministic
// qualified-runtime-binding.v2 receipt into one exact per-task grant. A missing
// receipt means the agent is ordinary. Once the deterministic receipt exists,
// expiry, revocation, conflict, or task-kind drift fails the enqueue closed.
// Callers run this before inserting the task in the same database transaction.
func (s *PermissionReceiptStore) PrepareWorkflowBrokerTaskAuthority(
	ctx context.Context, controllerID, taskID, workspaceID, issueID, agentID, runtimeID, taskKind string,
	baseContext []byte, now time.Time,
) ([]byte, bool, error) {
	receiptID := WorkflowBrokerRuntimeReceiptIDV1(issueID, agentID)
	row, err := s.queries.GetQualifiedRuntimeBindingReceiptByID(ctx, receiptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return baseContext, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("load workflow broker receipt marker: %w", err)
	}
	receipt, err := decodeRuntimeBindingRow(row, controllerID)
	if err != nil {
		return nil, true, err
	}
	if receipt.WorkspaceID != workspaceID || receipt.IssueID != issueID || receipt.ExecutorAgentID != agentID || receipt.RuntimeID != runtimeID || receipt.RuntimeProvider != "claude" {
		return nil, true, ErrGuardedConfigurationUnbound
	}
	workspaceUUID, issueUUID, agentUUID, runtimeUUID, err := parseRuntimeBindingIdentity(receipt)
	if err != nil {
		return nil, true, err
	}
	active, err := s.queries.GetActiveQualifiedRuntimeBindingReceiptForExecution(ctx, db.GetActiveQualifiedRuntimeBindingReceiptForExecutionParams{
		ID: receipt.ID, WorkspaceID: workspaceUUID, IssueID: issueUUID, ExecutorAgentID: agentUUID,
		RuntimeID: runtimeUUID, Now: permissionTimestamp(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, true, ErrGuardedConfigurationUnbound
	}
	if err != nil {
		return nil, true, err
	}
	receipt, err = decodeRuntimeBindingRow(active, controllerID)
	if err != nil {
		return nil, true, err
	}
	if err := validateGuardedWorkflowBrokerTemplateV1(receipt.WorkflowBroker, receipt.ChildExecution); err != nil {
		return nil, true, err
	}
	artifact, err := ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2." + receipt.RoleID)
	if err != nil {
		return nil, true, err
	}
	envelope := GuardedDispatchEnvelopeV1{
		Version: GuardedDispatchEnvelopeVersionV2, WorkspaceID: workspaceID, IssueID: issueID,
		ConfigurationID: receipt.ConfigurationID, RoleID: receipt.RoleID, ExecutorAgentID: agentID,
		RuntimeID: runtimeID, RuntimeProvider: receipt.RuntimeProvider, Model: receipt.Model,
		ReasoningEffort: receipt.ReasoningEffort, ArtifactBindingID: artifact.BindingID,
		ArtifactBinding: artifact, RuntimeBindingReceiptID: receipt.ID,
	}
	template := receipt.WorkflowBroker
	if taskKind == "" && len(template.AllowedTaskKinds) == 1 {
		taskKind = template.AllowedTaskKinds[0]
	}
	binding := &GuardedWorkflowBrokerV1{
		Version: GuardedWorkflowBrokerVersionV1, AgentName: template.AgentName, TaskKind: taskKind,
		BrokerExecutablePath: template.BrokerExecutablePath, ProviderExecutablePath: template.ProviderExecutablePath,
		ProviderExecutableSHA256: template.ProviderExecutableSHA256, ProviderProfileKind: template.ProviderProfileKind,
		ProviderProfilePath: template.ProviderProfilePath, ProviderCallLimit: 1, RetryLimit: 0, MaxTurns: 1,
	}
	grantID := "item14-workflow-broker-task-v1." + taskID
	issuedAt := canonicalPermissionTime(now)
	expiresAt := receipt.ExpiresAt
	routeVersion := "item14-workflow-broker-route-v1"
	route := systemgraph.RoutePlan{Mode: "contribute", AllowedHosts: []string{template.HostAttestation}, ScopedNodeIDs: []string{"crm_product"}, CoverageGate: "expand", RiskClass: "bounded_read", RequestedAction: "read_boundary_state"}
	capability := systemgraph.CapabilityReceipt{
		ID: grantID, ExecutorAgentID: agentID, IssueID: issueID, AuthorityRevision: 2,
		RouteVersion: routeVersion, RoleID: receipt.RoleID, ConfigurationID: receipt.ConfigurationID,
		Action: "read_boundary_state", TargetNodeIDs: []string{"crm_product"}, HostAttestation: template.HostAttestation,
		AllowedTools: []string{"provider_free_result_fixture"}, CredentialPurposes: []string{GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1},
		IssuedAt: issuedAt, ExpiresAt: expiresAt, CoverageReceiptID: grantID + ".assurance",
		OwnerAuthorizationReceiptID: grantID + ".owner", CandidateID: grantID + ".candidate",
		AuthorizedScope: "item14-bounded-workflow-broker-only",
	}
	scope := TaskCapabilityScopeV1{
		Version: TaskCapabilityScopeVersionV3, WorkspaceID: workspaceID, IssueID: issueID,
		ExecutorAgentID: agentID, RuntimeID: runtimeID, RoleID: receipt.RoleID,
		ConfigurationID: receipt.ConfigurationID, RuntimeBindingReceiptID: receipt.ID,
		RouteVersion: routeVersion, HostAttestation: template.HostAttestation, Action: capability.Action,
		Route: route, Receipt: capability,
		Assurance: &systemgraph.ArchitectureAssuranceReceipt{
			ID: capability.CoverageReceiptID, IssueID: issueID, RouteVersion: routeVersion, CoverageGate: route.CoverageGate,
			IssuerAgentID: "item14-controller", IssuerRoleID: "architecture_assurance",
			IssuerConfigurationID: "item14.workflow-broker.assurance.v1", Source: "controller_record",
			Passed: true, Immutable: true, IssuedAt: issuedAt, ExpiresAt: expiresAt,
		},
		OwnerAuthorization: &systemgraph.OwnerAuthorizationReceipt{
			ID: capability.OwnerAuthorizationReceiptID, IssueID: issueID, AuthorityRevision: capability.AuthorityRevision,
			CandidateID: capability.CandidateID, Action: capability.Action, TargetNodeIDs: capability.TargetNodeIDs,
			AuthorizedScope: capability.AuthorizedScope, IssuerRoleID: "owner", Source: "controller_record",
			Immutable: true, RecordedAt: issuedAt,
		},
		WorkflowBroker: binding,
	}
	preview := &GuardedExecutionPackageV1{Version: GuardedExecutionPackageVersionV1, Envelope: envelope, RuntimeBinding: receipt, CapabilityScope: scope, PluginToolTargets: map[string][]string{}}
	if _, err := ValidateGuardedExecutionPackageV1(preview, GuardedExecutionExpectedV1{
		WorkspaceID: workspaceID, IssueID: issueID, ExecutorAgentID: agentID, RuntimeID: runtimeID,
		RuntimeProvider: receipt.RuntimeProvider, Model: receipt.Model, ReasoningEffort: receipt.ReasoningEffort,
		HostAttestation: template.HostAttestation, AgentName: template.AgentName,
	}, now); err != nil {
		return nil, true, err
	}
	if _, err := s.StoreTaskCapabilityGrant(ctx, controllerID, taskID, scope, map[string][]string{}, now); err != nil {
		return nil, true, err
	}
	merged, err := mergeGuardedDispatchContextV1(baseContext, envelope)
	return merged, true, err
}
