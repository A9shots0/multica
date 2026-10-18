package agentqualification

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

const (
	TaskCapabilityScopeVersionV1 = "silver-mediums.task-capability-scope.v1"
	TaskCapabilityScopeVersionV2 = "silver-mediums.task-capability-scope.v2"
	TaskCapabilityScopeVersionV3 = "silver-mediums.task-capability-scope.v3"
	CapabilityGatewayRemoteMCP   = "brokered_remote_mcp"
	CapabilityGatewayPluginHook  = "brokered_plugin_hook"
	CapabilityGatewayRawShell    = "raw_shell"
)

var (
	ErrTaskCapabilityDenied        = errors.New("guarded_capability_denied")
	ErrConsequentialRawShellDenied = errors.New("guarded_consequential_raw_shell_denied")
)

// TaskCapabilityScopeV1 is immutable controller input for a guarded task. It
// adds workspace and runtime binding around the model-neutral system-graph
// capability receipt, whose own validation remains authoritative for action,
// route, host, targets, tools, credential purposes, and owner/assurance gates.
type TaskCapabilityScopeV1 struct {
	Version                 string                                    `json:"version"`
	WorkspaceID             string                                    `json:"workspace_id"`
	IssueID                 string                                    `json:"issue_id"`
	ExecutorAgentID         string                                    `json:"executor_agent_id"`
	RuntimeID               string                                    `json:"runtime_id"`
	RoleID                  string                                    `json:"role_id"`
	ConfigurationID         string                                    `json:"configuration_id"`
	RuntimeBindingReceiptID string                                    `json:"runtime_binding_receipt_id"`
	RouteVersion            string                                    `json:"route_version"`
	HostAttestation         string                                    `json:"host_attestation"`
	Action                  string                                    `json:"action"`
	Route                   systemgraph.RoutePlan                     `json:"route"`
	Receipt                 systemgraph.CapabilityReceipt             `json:"capability_receipt"`
	Assurance               *systemgraph.ArchitectureAssuranceReceipt `json:"architecture_assurance,omitempty"`
	OwnerAuthorization      *systemgraph.OwnerAuthorizationReceipt    `json:"owner_authorization,omitempty"`
	ModelOnlyShadow         *GuardedModelOnlyShadowV1                 `json:"model_only_shadow,omitempty"`
	WorkflowBroker          *GuardedWorkflowBrokerV1                  `json:"workflow_broker,omitempty"`
}

// TaskCapabilityGatewayCallV1 is derived at the gateway from the real claimed
// task, runtime, tool request, and credential resolver request.
type TaskCapabilityGatewayCallV1 struct {
	WorkspaceID              string   `json:"workspace_id"`
	IssueID                  string   `json:"issue_id"`
	ExecutorAgentID          string   `json:"executor_agent_id"`
	RuntimeID                string   `json:"runtime_id"`
	RoleID                   string   `json:"role_id"`
	ConfigurationID          string   `json:"configuration_id"`
	RouteVersion             string   `json:"route_version"`
	HostAttestation          string   `json:"host_attestation"`
	GatewayKind              string   `json:"gateway_kind"`
	Action                   string   `json:"action"`
	TargetNodeIDs            []string `json:"target_node_ids"`
	AttemptedTool            string   `json:"attempted_tool"`
	CredentialPurposes       []string `json:"credential_purposes,omitempty"`
	CandidateID              string   `json:"candidate_id,omitempty"`
	AuthorizedScope          string   `json:"authorized_scope,omitempty"`
	AssuranceConfigurationID string   `json:"assurance_configuration_id,omitempty"`
}

type TaskCapabilityAdmissionV1 struct {
	Mode                string
	CapabilityReceiptID string
}

// ValidateTaskCapabilityGatewayV1 is the shared last-mile authorization gate.
// A nil scope is an unguarded historical task and retains legacy behavior. Any
// present scope is fail closed. Consequential raw-shell execution is denied
// even when its receipt otherwise validates because Multica cannot yet
// intercept provider-native shell actions.
func ValidateTaskCapabilityGatewayV1(scope *TaskCapabilityScopeV1, actual TaskCapabilityGatewayCallV1, now time.Time) (TaskCapabilityAdmissionV1, error) {
	if scope == nil {
		return TaskCapabilityAdmissionV1{Mode: GuardedDispatchModeLegacy}, nil
	}
	admission := TaskCapabilityAdmissionV1{Mode: GuardedDispatchModeGuarded, CapabilityReceiptID: scope.Receipt.ID}
	if err := validateTaskCapabilityScopeIdentityV1(*scope, actual); err != nil {
		return admission, err
	}
	if actual.GatewayKind == CapabilityGatewayRawShell && (scope.Route.RiskClass == "consequential" || scope.Route.CoverageGate != "open") {
		return admission, ErrConsequentialRawShellDenied
	}
	roles, err := systemgraph.DefaultRoles()
	if err != nil {
		return admission, fmt.Errorf("%w: load role catalog: %v", ErrTaskCapabilityDenied, err)
	}
	expected := systemgraph.ExpectedCapabilityCall{
		ExecutorAgentID: actual.ExecutorAgentID, IssueID: actual.IssueID,
		AuthorityRevision: scope.Receipt.AuthorityRevision, RouteVersion: actual.RouteVersion,
		RoleID: actual.RoleID, ConfigurationID: actual.ConfigurationID, Action: actual.Action,
		TargetNodeIDs: actual.TargetNodeIDs, HostAttestation: actual.HostAttestation,
		AttemptedTool: actual.AttemptedTool, CredentialPurposes: actual.CredentialPurposes,
		CandidateID: actual.CandidateID, AuthorizedScope: actual.AuthorizedScope,
		AssuranceConfigurationID: actual.AssuranceConfigurationID,
	}
	if err := roles.ValidateCapabilityReceipt(scope.Receipt, scope.Route, expected, scope.Assurance, scope.OwnerAuthorization, now); err != nil {
		return admission, fmt.Errorf("%w: %v", ErrTaskCapabilityDenied, err)
	}
	return admission, nil
}

func validateTaskCapabilityScopeIdentityV1(scope TaskCapabilityScopeV1, actual TaskCapabilityGatewayCallV1) error {
	if (scope.Version != TaskCapabilityScopeVersionV1 && scope.Version != TaskCapabilityScopeVersionV2 && scope.Version != TaskCapabilityScopeVersionV3) || strings.TrimSpace(scope.WorkspaceID) == "" || strings.TrimSpace(scope.IssueID) == "" || strings.TrimSpace(scope.ExecutorAgentID) == "" || strings.TrimSpace(scope.RuntimeID) == "" || strings.TrimSpace(scope.RoleID) == "" || strings.TrimSpace(scope.ConfigurationID) == "" || strings.TrimSpace(scope.RuntimeBindingReceiptID) == "" || strings.TrimSpace(scope.RouteVersion) == "" || strings.TrimSpace(scope.HostAttestation) == "" || strings.TrimSpace(scope.Action) == "" {
		return fmt.Errorf("%w: incomplete task capability scope", ErrTaskCapabilityDenied)
	}
	if actual.GatewayKind != CapabilityGatewayRemoteMCP && actual.GatewayKind != CapabilityGatewayPluginHook && actual.GatewayKind != CapabilityGatewayRawShell {
		return fmt.Errorf("%w: unknown capability gateway", ErrTaskCapabilityDenied)
	}
	if strings.TrimSpace(actual.AttemptedTool) == "" || len(actual.TargetNodeIDs) == 0 {
		return fmt.Errorf("%w: incomplete gateway call", ErrTaskCapabilityDenied)
	}
	if scope.WorkspaceID != actual.WorkspaceID || scope.IssueID != actual.IssueID || scope.ExecutorAgentID != actual.ExecutorAgentID || scope.RuntimeID != actual.RuntimeID || scope.RoleID != actual.RoleID || scope.ConfigurationID != actual.ConfigurationID || scope.RouteVersion != actual.RouteVersion || scope.HostAttestation != actual.HostAttestation || scope.Action != actual.Action {
		return fmt.Errorf("%w: gateway identity does not match guarded scope", ErrTaskCapabilityDenied)
	}
	receipt := scope.Receipt
	if receipt.ExecutorAgentID != scope.ExecutorAgentID || receipt.IssueID != scope.IssueID || receipt.RoleID != scope.RoleID || receipt.ConfigurationID != scope.ConfigurationID || receipt.RouteVersion != scope.RouteVersion || receipt.HostAttestation != scope.HostAttestation || receipt.Action != scope.Action {
		return fmt.Errorf("%w: capability receipt does not match guarded scope", ErrTaskCapabilityDenied)
	}
	return nil
}
