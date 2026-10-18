package agentqualification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// PermissionReceiptQuerier is the controller-only durable boundary used by
// PermissionReceiptStore. No HTTP, task-context, or agent-facing writer owns
// this interface.
type PermissionReceiptQuerier interface {
	CreateQualifiedRuntimeBindingReceipt(context.Context, db.CreateQualifiedRuntimeBindingReceiptParams) (db.QualifiedRuntimeBindingReceipt, error)
	GetQualifiedRuntimeBindingReceiptByID(context.Context, string) (db.QualifiedRuntimeBindingReceipt, error)
	GetActiveQualifiedRuntimeBindingReceiptForExecution(context.Context, db.GetActiveQualifiedRuntimeBindingReceiptForExecutionParams) (db.QualifiedRuntimeBindingReceipt, error)
	RevokeQualifiedRuntimeBindingReceipt(context.Context, db.RevokeQualifiedRuntimeBindingReceiptParams) (db.QualifiedRuntimeBindingRevocation, error)
	GetQualifiedRuntimeBindingRevocation(context.Context, db.GetQualifiedRuntimeBindingRevocationParams) (db.QualifiedRuntimeBindingRevocation, error)

	CreateGuardedTaskCapabilityGrant(context.Context, db.CreateGuardedTaskCapabilityGrantParams) (db.GuardedTaskCapabilityGrant, error)
	GetGuardedTaskCapabilityGrantByID(context.Context, string) (db.GuardedTaskCapabilityGrant, error)
	GetActiveGuardedTaskCapabilityGrantForExecution(context.Context, db.GetActiveGuardedTaskCapabilityGrantForExecutionParams) (db.GuardedTaskCapabilityGrant, error)
	RevokeGuardedTaskCapabilityGrant(context.Context, db.RevokeGuardedTaskCapabilityGrantParams) (db.GuardedTaskCapabilityRevocation, error)
	GetGuardedTaskCapabilityRevocation(context.Context, db.GetGuardedTaskCapabilityRevocationParams) (db.GuardedTaskCapabilityRevocation, error)

	ConsumeGuardedExecutionAuthority(context.Context, db.ConsumeGuardedExecutionAuthorityParams) (db.GuardedExecutionLaunchConsumption, error)
}

type PermissionReceiptStore struct {
	queries PermissionReceiptQuerier
}

func NewPermissionReceiptStore(queries PermissionReceiptQuerier) *PermissionReceiptStore {
	return &PermissionReceiptStore{queries: queries}
}

// GuardedExecutionLoadIdentityV1 contains only facts independently resolved by
// the server. The task context may select a runtime receipt ID, but it cannot
// supply any of these scope facts or the controller identity.
type GuardedExecutionLoadIdentityV1 struct {
	TaskID               string
	WorkspaceID          string
	IssueID              string
	ExecutorAgentID      string
	RuntimeID            string
	ExpectedControllerID string
}

// GuardedExecutionIssueV1 is the complete controller input for one exact
// guarded task. Issuance is retry-safe: exact retries converge on the same two
// immutable rows, while conflicts never produce an executable package.
type GuardedExecutionIssueV1 struct {
	TaskID            string
	Receipt           QualifiedRuntimeBindingReceiptV2
	Envelope          GuardedDispatchEnvelopeV1
	CapabilityScope   TaskCapabilityScopeV1
	PluginToolTargets map[string][]string
}

type storedCapabilityPayloadV1 struct {
	Scope             TaskCapabilityScopeV1 `json:"capability_scope"`
	PluginToolTargets map[string][]string   `json:"plugin_tool_targets"`
}

func canonicalPermissionTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func permissionTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: canonicalPermissionTime(value), Valid: true}
}

func permissionDigest(value any) ([]byte, string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:]), nil
}

func parsePermissionUUID(value, field string) (pgtype.UUID, error) {
	parsed, err := util.ParseUUID(value)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("%w: invalid %s", ErrGuardedDispatchInvalid, field)
	}
	return parsed, nil
}

// IssueGuardedExecutionAuthority stores and reloads the exact runtime binding
// and per-task grant as one controller-owned operation. The runtime receipt is
// intentionally safe to store first: without the task grant it cannot produce
// a positive execution package, and an exact retry completes a partial write.
func (s *PermissionReceiptStore) IssueGuardedExecutionAuthority(
	ctx context.Context,
	controllerID string,
	issue GuardedExecutionIssueV1,
	now time.Time,
) (*GuardedExecutionPackageV1, error) {
	identity := GuardedExecutionLoadIdentityV1{
		TaskID: issue.TaskID, WorkspaceID: issue.Receipt.WorkspaceID,
		IssueID: issue.Receipt.IssueID, ExecutorAgentID: issue.Receipt.ExecutorAgentID,
		RuntimeID: issue.Receipt.RuntimeID, ExpectedControllerID: controllerID,
	}
	if _, _, _, _, _, err := parseLoadIdentity(identity); err != nil {
		return nil, err
	}
	preview := &GuardedExecutionPackageV1{
		Version: GuardedExecutionPackageVersionV1, Envelope: issue.Envelope,
		RuntimeBinding: issue.Receipt, CapabilityScope: issue.CapabilityScope,
		PluginToolTargets: clonePluginTargets(issue.PluginToolTargets),
	}
	expected := GuardedExecutionExpectedV1{
		WorkspaceID: identity.WorkspaceID, IssueID: identity.IssueID,
		ExecutorAgentID: identity.ExecutorAgentID, RuntimeID: identity.RuntimeID,
		RuntimeProvider: issue.Receipt.RuntimeProvider, Model: issue.Receipt.Model,
		ReasoningEffort: issue.Receipt.ReasoningEffort,
		HostAttestation: issue.CapabilityScope.HostAttestation,
	}
	if issue.CapabilityScope.WorkflowBroker != nil {
		expected.AgentName = issue.CapabilityScope.WorkflowBroker.AgentName
	}
	if _, err := ValidateGuardedExecutionPackageV1(preview, expected, canonicalPermissionTime(now)); err != nil {
		return nil, err
	}
	if _, err := s.StoreQualifiedRuntimeBinding(ctx, controllerID, issue.Receipt, now); err != nil {
		return nil, err
	}
	if _, err := s.StoreTaskCapabilityGrant(ctx, controllerID, issue.TaskID, issue.CapabilityScope, issue.PluginToolTargets, now); err != nil {
		return nil, err
	}
	return s.LoadGuardedExecutionPackage(ctx, identity, issue.Envelope, now)
}

func (s *PermissionReceiptStore) StoreQualifiedRuntimeBinding(
	ctx context.Context,
	controllerID string,
	receipt QualifiedRuntimeBindingReceiptV2,
	now time.Time,
) (QualifiedRuntimeBindingReceiptV2, error) {
	if s == nil || s.queries == nil {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("permission receipt store is unavailable")
	}
	controllerUUID, err := parsePermissionUUID(controllerID, "controller id")
	if err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, err
	}
	receipt.IssuedAt = canonicalPermissionTime(receipt.IssuedAt)
	receipt.ExpiresAt = canonicalPermissionTime(receipt.ExpiresAt)
	now = canonicalPermissionTime(now)
	if err := validateRuntimeBindingForStorage(receipt, now); err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, err
	}
	workspaceID, issueID, agentID, runtimeID, err := parseRuntimeBindingIdentity(receipt)
	if err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, err
	}
	evidenceJSON, _, err := permissionDigest(receipt.EvidenceReceiptIDs)
	if err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("encode runtime evidence receipts: %w", err)
	}
	receiptJSON, digest, err := permissionDigest(receipt)
	if err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("encode runtime binding receipt: %w", err)
	}
	row, err := s.queries.CreateQualifiedRuntimeBindingReceipt(ctx, db.CreateQualifiedRuntimeBindingReceiptParams{
		ID: receipt.ID, WorkspaceID: workspaceID, IssueID: issueID,
		ExecutorAgentID: agentID, RuntimeID: runtimeID,
		ConfigurationID: receipt.ConfigurationID, RoleID: receipt.RoleID,
		RuntimeProvider: receipt.RuntimeProvider, Model: receipt.Model,
		ReasoningEffort: receipt.ReasoningEffort, ArtifactBindingSha256: receipt.ArtifactBindingSHA256,
		QualificationStatus: receipt.QualificationStatus, EvidenceReceiptIds: evidenceJSON,
		Source: receipt.Source, Immutable: receipt.Immutable, IssuedByControllerID: controllerUUID,
		IssuedAt: permissionTimestamp(receipt.IssuedAt), ExpiresAt: permissionTimestamp(receipt.ExpiresAt),
		Receipt: receiptJSON, ReceiptSha256: digest,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.queries.GetQualifiedRuntimeBindingReceiptByID(ctx, receipt.ID)
	}
	if err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("store runtime binding receipt: %w", err)
	}
	stored, err := decodeRuntimeBindingRow(row, controllerID)
	if err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, err
	}
	if !reflect.DeepEqual(stored, receipt) {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("%w: conflicting runtime binding receipt id", ErrGuardedDispatchInvalid)
	}
	return stored, nil
}

func (s *PermissionReceiptStore) StoreTaskCapabilityGrant(
	ctx context.Context,
	controllerID, taskID string,
	scope TaskCapabilityScopeV1,
	pluginToolTargets map[string][]string,
	now time.Time,
) (TaskCapabilityScopeV1, error) {
	if s == nil || s.queries == nil {
		return TaskCapabilityScopeV1{}, fmt.Errorf("permission receipt store is unavailable")
	}
	controllerUUID, err := parsePermissionUUID(controllerID, "controller id")
	if err != nil {
		return TaskCapabilityScopeV1{}, err
	}
	taskUUID, err := parsePermissionUUID(taskID, "task id")
	if err != nil {
		return TaskCapabilityScopeV1{}, err
	}
	canonicalizeTaskCapabilityTimes(&scope)
	now = canonicalPermissionTime(now)
	if err := validateCapabilityScopeForStorage(scope, pluginToolTargets, now); err != nil {
		return TaskCapabilityScopeV1{}, err
	}
	workspaceID, issueID, agentID, runtimeID, err := parseCapabilityScopeIdentity(scope)
	if err != nil {
		return TaskCapabilityScopeV1{}, err
	}
	scopeJSON, _, err := permissionDigest(scope)
	if err != nil {
		return TaskCapabilityScopeV1{}, fmt.Errorf("encode capability scope: %w", err)
	}
	targets := clonePluginTargets(pluginToolTargets)
	targetsJSON, _, err := permissionDigest(targets)
	if err != nil {
		return TaskCapabilityScopeV1{}, fmt.Errorf("encode plugin targets: %w", err)
	}
	_, payloadDigest, err := permissionDigest(storedCapabilityPayloadV1{Scope: scope, PluginToolTargets: targets})
	if err != nil {
		return TaskCapabilityScopeV1{}, fmt.Errorf("hash capability grant: %w", err)
	}
	row, err := s.queries.CreateGuardedTaskCapabilityGrant(ctx, db.CreateGuardedTaskCapabilityGrantParams{
		ID: scope.Receipt.ID, TaskID: taskUUID, WorkspaceID: workspaceID, IssueID: issueID,
		ExecutorAgentID: agentID, RuntimeID: runtimeID,
		RuntimeBindingReceiptID: scope.RuntimeBindingReceiptID,
		ConfigurationID:         scope.ConfigurationID, RoleID: scope.RoleID,
		RouteVersion: scope.RouteVersion, HostAttestation: scope.HostAttestation, Action: scope.Action,
		IssuedByControllerID: controllerUUID, IssuedAt: permissionTimestamp(scope.Receipt.IssuedAt),
		ExpiresAt: permissionTimestamp(scope.Receipt.ExpiresAt), CapabilityScope: scopeJSON,
		PluginToolTargets: targetsJSON, PayloadSha256: payloadDigest,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.queries.GetGuardedTaskCapabilityGrantByID(ctx, scope.Receipt.ID)
	}
	if err != nil {
		return TaskCapabilityScopeV1{}, fmt.Errorf("store task capability grant: %w", err)
	}
	storedScope, storedTargets, err := decodeCapabilityGrantRow(row, controllerID, taskID)
	if err != nil {
		return TaskCapabilityScopeV1{}, err
	}
	if !reflect.DeepEqual(storedScope, scope) || !reflect.DeepEqual(storedTargets, targets) {
		return TaskCapabilityScopeV1{}, fmt.Errorf("%w: conflicting task capability grant", ErrTaskCapabilityDenied)
	}
	return storedScope, nil
}

func (s *PermissionReceiptStore) RevokeQualifiedRuntimeBinding(
	ctx context.Context,
	controllerID, workspaceID, receiptID, reason string,
	now time.Time,
) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("permission receipt store is unavailable")
	}
	controllerUUID, err := parsePermissionUUID(controllerID, "controller id")
	if err != nil {
		return err
	}
	workspaceUUID, err := parsePermissionUUID(workspaceID, "workspace id")
	if err != nil {
		return err
	}
	if strings.TrimSpace(receiptID) == "" || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: incomplete runtime binding revocation", ErrGuardedDispatchInvalid)
	}
	target, err := s.queries.GetQualifiedRuntimeBindingReceiptByID(ctx, receiptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: runtime binding receipt does not exist", ErrGuardedConfigurationUnbound)
	}
	if err != nil {
		return fmt.Errorf("load runtime binding receipt for revocation: %w", err)
	}
	decodedTarget, err := decodeRuntimeBindingRow(target, controllerID)
	if err != nil {
		return err
	}
	if decodedTarget.WorkspaceID != workspaceID {
		return fmt.Errorf("%w: runtime binding revocation workspace mismatch", ErrGuardedConfigurationUnbound)
	}
	reason = strings.TrimSpace(reason)
	now = canonicalPermissionTime(now)
	params := db.RevokeQualifiedRuntimeBindingReceiptParams{
		ReceiptID: receiptID, WorkspaceID: workspaceUUID, RevokedByControllerID: controllerUUID,
		Reason: reason, RevokedAt: permissionTimestamp(now),
	}
	stored, err := s.queries.RevokeQualifiedRuntimeBindingReceipt(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		stored, err = s.queries.GetQualifiedRuntimeBindingRevocation(ctx, db.GetQualifiedRuntimeBindingRevocationParams{WorkspaceID: workspaceUUID, ReceiptID: receiptID})
	}
	if err != nil {
		return fmt.Errorf("revoke runtime binding receipt: %w", err)
	}
	if !runtimeRevocationMatches(stored, params) {
		return fmt.Errorf("%w: conflicting runtime binding revocation", ErrGuardedDispatchInvalid)
	}
	return nil
}

func (s *PermissionReceiptStore) RevokeTaskCapabilityGrant(
	ctx context.Context,
	controllerID, workspaceID, grantID, reason string,
	now time.Time,
) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("permission receipt store is unavailable")
	}
	controllerUUID, err := parsePermissionUUID(controllerID, "controller id")
	if err != nil {
		return err
	}
	workspaceUUID, err := parsePermissionUUID(workspaceID, "workspace id")
	if err != nil {
		return err
	}
	if strings.TrimSpace(grantID) == "" || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: incomplete capability grant revocation", ErrTaskCapabilityDenied)
	}
	target, err := s.queries.GetGuardedTaskCapabilityGrantByID(ctx, grantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: capability grant does not exist", ErrTaskCapabilityDenied)
	}
	if err != nil {
		return fmt.Errorf("load capability grant for revocation: %w", err)
	}
	decodedTarget, _, err := decodeCapabilityGrantRow(target, controllerID, util.UUIDToString(target.TaskID))
	if err != nil {
		return err
	}
	if decodedTarget.WorkspaceID != workspaceID {
		return fmt.Errorf("%w: capability grant revocation workspace mismatch", ErrTaskCapabilityDenied)
	}
	reason = strings.TrimSpace(reason)
	now = canonicalPermissionTime(now)
	params := db.RevokeGuardedTaskCapabilityGrantParams{
		GrantID: grantID, WorkspaceID: workspaceUUID, RevokedByControllerID: controllerUUID,
		Reason: reason, RevokedAt: permissionTimestamp(now),
	}
	stored, err := s.queries.RevokeGuardedTaskCapabilityGrant(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		stored, err = s.queries.GetGuardedTaskCapabilityRevocation(ctx, db.GetGuardedTaskCapabilityRevocationParams{WorkspaceID: workspaceUUID, GrantID: grantID})
	}
	if err != nil {
		return fmt.Errorf("revoke task capability grant: %w", err)
	}
	if !capabilityRevocationMatches(stored, params) {
		return fmt.Errorf("%w: conflicting capability grant revocation", ErrTaskCapabilityDenied)
	}
	return nil
}

func runtimeRevocationMatches(row db.QualifiedRuntimeBindingRevocation, params db.RevokeQualifiedRuntimeBindingReceiptParams) bool {
	return row.ReceiptID == params.ReceiptID &&
		util.UUIDToString(row.WorkspaceID) == util.UUIDToString(params.WorkspaceID) &&
		util.UUIDToString(row.RevokedByControllerID) == util.UUIDToString(params.RevokedByControllerID) &&
		row.Reason == params.Reason && row.RevokedAt.Valid && params.RevokedAt.Valid &&
		row.RevokedAt.Time.Equal(params.RevokedAt.Time)
}

func capabilityRevocationMatches(row db.GuardedTaskCapabilityRevocation, params db.RevokeGuardedTaskCapabilityGrantParams) bool {
	return row.GrantID == params.GrantID &&
		util.UUIDToString(row.WorkspaceID) == util.UUIDToString(params.WorkspaceID) &&
		util.UUIDToString(row.RevokedByControllerID) == util.UUIDToString(params.RevokedByControllerID) &&
		row.Reason == params.Reason && row.RevokedAt.Valid && params.RevokedAt.Valid &&
		row.RevokedAt.Time.Equal(params.RevokedAt.Time)
}

func (s *PermissionReceiptStore) LoadGuardedExecutionPackage(
	ctx context.Context,
	identity GuardedExecutionLoadIdentityV1,
	envelope GuardedDispatchEnvelopeV1,
	now time.Time,
) (*GuardedExecutionPackageV1, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("permission receipt store is unavailable")
	}
	taskID, workspaceID, issueID, agentID, runtimeID, err := parseLoadIdentity(identity)
	if err != nil {
		return nil, err
	}
	now = canonicalPermissionTime(now)
	runtimeRow, err := s.queries.GetActiveQualifiedRuntimeBindingReceiptForExecution(ctx, db.GetActiveQualifiedRuntimeBindingReceiptForExecutionParams{
		ID: envelope.RuntimeBindingReceiptID, WorkspaceID: workspaceID, IssueID: issueID,
		ExecutorAgentID: agentID, RuntimeID: runtimeID, Now: permissionTimestamp(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrGuardedConfigurationUnbound
	}
	if err != nil {
		return nil, fmt.Errorf("load runtime binding receipt: %w", err)
	}
	runtimeReceipt, err := decodeRuntimeBindingRow(runtimeRow, identity.ExpectedControllerID)
	if err != nil {
		return nil, err
	}
	grantRow, err := s.queries.GetActiveGuardedTaskCapabilityGrantForExecution(ctx, db.GetActiveGuardedTaskCapabilityGrantForExecutionParams{
		TaskID: taskID, WorkspaceID: workspaceID, IssueID: issueID,
		ExecutorAgentID: agentID, RuntimeID: runtimeID,
		RuntimeBindingReceiptID: runtimeReceipt.ID, Now: permissionTimestamp(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTaskCapabilityDenied
	}
	if err != nil {
		return nil, fmt.Errorf("load task capability grant: %w", err)
	}
	scope, targets, err := decodeCapabilityGrantRow(grantRow, identity.ExpectedControllerID, identity.TaskID)
	if err != nil {
		return nil, err
	}
	pkg := &GuardedExecutionPackageV1{
		Version: GuardedExecutionPackageVersionV1, Envelope: envelope,
		RuntimeBinding: runtimeReceipt, CapabilityScope: scope, PluginToolTargets: targets,
	}
	expected := GuardedExecutionExpectedV1{
		WorkspaceID: identity.WorkspaceID, IssueID: identity.IssueID,
		ExecutorAgentID: identity.ExecutorAgentID, RuntimeID: identity.RuntimeID,
		RuntimeProvider: envelope.RuntimeProvider, Model: envelope.Model,
		ReasoningEffort: envelope.ReasoningEffort, HostAttestation: scope.HostAttestation,
	}
	if scope.WorkflowBroker != nil {
		expected.AgentName = scope.WorkflowBroker.AgentName
	}
	if _, err := ValidateGuardedExecutionPackageV1(pkg, expected, now); err != nil {
		return nil, err
	}
	return pkg, nil
}

// ConsumeGuardedExecutionAuthority reloads current controller state and then
// appends the single allowed launch. Callers must invoke it inside the same
// database transaction that moves the task to running. The package returned
// here is evidence of the exact bytes consumed; it is not reusable authority.
func (s *PermissionReceiptStore) ConsumeGuardedExecutionAuthority(
	ctx context.Context,
	identity GuardedExecutionLoadIdentityV1,
	envelope GuardedDispatchEnvelopeV1,
	presentedPackageSHA256 string,
	daemonID string,
	now time.Time,
) (*GuardedExecutionPackageV1, error) {
	daemonID = strings.TrimSpace(daemonID)
	presentedPackageSHA256 = strings.TrimSpace(presentedPackageSHA256)
	if daemonID == "" || presentedPackageSHA256 == "" {
		return nil, fmt.Errorf("%w: authenticated daemon id and package digest are required", ErrGuardedAuthorityUnavailable)
	}
	pkg, err := s.LoadGuardedExecutionPackage(ctx, identity, envelope, now)
	if err != nil {
		return nil, err
	}
	taskID, workspaceID, issueID, agentID, runtimeID, err := parseLoadIdentity(identity)
	if err != nil {
		return nil, err
	}
	controllerID, err := parsePermissionUUID(identity.ExpectedControllerID, "controller id")
	if err != nil {
		return nil, err
	}
	packageSHA256, err := GuardedExecutionPackageSHA256V1(pkg)
	if err != nil {
		return nil, fmt.Errorf("hash guarded execution package: %w", err)
	}
	if presentedPackageSHA256 != packageSHA256 {
		return nil, fmt.Errorf("%w: presented package digest does not match controller authority", ErrGuardedAuthorityUnavailable)
	}
	now = canonicalPermissionTime(now)
	row, err := s.queries.ConsumeGuardedExecutionAuthority(ctx, db.ConsumeGuardedExecutionAuthorityParams{
		TaskID: taskID, WorkspaceID: workspaceID, IssueID: issueID,
		ExecutorAgentID: agentID, RuntimeID: runtimeID,
		RuntimeBindingReceiptID: pkg.RuntimeBinding.ID,
		CapabilityGrantID:       pkg.CapabilityScope.Receipt.ID,
		ConsumedByDaemonID:      daemonID, PackageSha256: packageSHA256,
		ConsumedAt: permissionTimestamp(now), ExpectedControllerID: controllerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrGuardedAuthorityUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("consume guarded execution authority: %w", err)
	}
	if util.UUIDToString(row.TaskID) != identity.TaskID ||
		util.UUIDToString(row.WorkspaceID) != identity.WorkspaceID ||
		util.UUIDToString(row.IssueID) != identity.IssueID ||
		util.UUIDToString(row.ExecutorAgentID) != identity.ExecutorAgentID ||
		util.UUIDToString(row.RuntimeID) != identity.RuntimeID ||
		row.RuntimeBindingReceiptID != pkg.RuntimeBinding.ID ||
		row.CapabilityGrantID != pkg.CapabilityScope.Receipt.ID ||
		row.ConsumedByDaemonID != daemonID || row.PackageSha256 != packageSHA256 ||
		!row.ConsumedAt.Valid || !row.ConsumedAt.Time.Equal(now) {
		return nil, fmt.Errorf("%w: stored launch consumption mismatch", ErrGuardedAuthorityUnavailable)
	}
	return pkg, nil
}

func validateRuntimeBindingForStorage(receipt QualifiedRuntimeBindingReceiptV2, now time.Time) error {
	if receipt.Revoked || len(receipt.EvidenceReceiptIDs) == 0 {
		return ErrGuardedConfigurationUnbound
	}
	seenEvidence := make(map[string]struct{}, len(receipt.EvidenceReceiptIDs))
	for _, evidenceID := range receipt.EvidenceReceiptIDs {
		if strings.TrimSpace(evidenceID) == "" {
			return fmt.Errorf("%w: empty qualification evidence receipt", ErrGuardedDispatchInvalid)
		}
		if _, duplicate := seenEvidence[evidenceID]; duplicate {
			return fmt.Errorf("%w: duplicate qualification evidence receipt", ErrGuardedDispatchInvalid)
		}
		seenEvidence[evidenceID] = struct{}{}
	}
	artifact, err := ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2." + receipt.RoleID)
	if err != nil {
		return fmt.Errorf("%w: runtime role artifact is not current", ErrGuardedConfigurationUnbound)
	}
	artifactHash, err := roleArtifactBindingSHA256V2(artifact)
	if err != nil {
		return fmt.Errorf("hash current role artifact: %w", err)
	}
	envelope := GuardedDispatchEnvelopeV1{
		Version: GuardedDispatchEnvelopeVersionV2, WorkspaceID: receipt.WorkspaceID,
		IssueID: receipt.IssueID, ConfigurationID: receipt.ConfigurationID, RoleID: receipt.RoleID,
		ExecutorAgentID: receipt.ExecutorAgentID, RuntimeID: receipt.RuntimeID,
		RuntimeProvider: receipt.RuntimeProvider, Model: receipt.Model, ReasoningEffort: receipt.ReasoningEffort,
		ArtifactBindingID: artifact.BindingID, ArtifactBinding: artifact,
		RuntimeBindingReceiptID: receipt.ID,
	}
	if err := validateQualifiedRuntimeBindingV2(envelope, receipt, artifactHash, now); err != nil {
		return err
	}
	if receipt.WorkflowBroker != nil {
		if err := validateGuardedWorkflowBrokerTemplateV1(receipt.WorkflowBroker, receipt.ChildExecution); err != nil {
			return err
		}
	} else if receipt.ChildExecution.Adapter == GuardedClaudeWorkflowBrokerAdapterV1 {
		return ErrGuardedConfigurationUnbound
	}
	return nil
}

func validateCapabilityScopeForStorage(scope TaskCapabilityScopeV1, targets map[string][]string, now time.Time) error {
	if scope.Version == TaskCapabilityScopeVersionV1 && (scope.ModelOnlyShadow != nil || scope.WorkflowBroker != nil) {
		return fmt.Errorf("%w: v1 scope cannot contain specialized authority", ErrTaskCapabilityDenied)
	}
	if scope.Version == TaskCapabilityScopeVersionV2 && (scope.ModelOnlyShadow == nil || scope.WorkflowBroker != nil) {
		return fmt.Errorf("%w: v2 scope requires model-only authority", ErrTaskCapabilityDenied)
	}
	if scope.Version == TaskCapabilityScopeVersionV3 && (scope.ModelOnlyShadow != nil || scope.WorkflowBroker == nil) {
		return fmt.Errorf("%w: v3 scope requires workflow-broker authority", ErrTaskCapabilityDenied)
	}
	if scope.Version != TaskCapabilityScopeVersionV1 && scope.Version != TaskCapabilityScopeVersionV2 && scope.Version != TaskCapabilityScopeVersionV3 {
		return fmt.Errorf("%w: unsupported capability scope", ErrTaskCapabilityDenied)
	}
	if scope.Receipt.Revoked || !scope.Receipt.IssuedAt.Before(scope.Receipt.ExpiresAt) || now.Before(scope.Receipt.IssuedAt) || !now.Before(scope.Receipt.ExpiresAt) {
		return ErrTaskCapabilityDenied
	}
	if scope.Receipt.ID == "" || len(scope.Receipt.AllowedTools) == 0 || len(scope.Receipt.TargetNodeIDs) == 0 {
		return fmt.Errorf("%w: incomplete capability receipt", ErrTaskCapabilityDenied)
	}
	if err := validatePluginToolTargetsV1(targets, scope.Receipt.TargetNodeIDs); err != nil {
		return err
	}
	assuranceConfig := ""
	if scope.Assurance != nil {
		assuranceConfig = scope.Assurance.IssuerConfigurationID
	}
	for _, tool := range scope.Receipt.AllowedTools {
		if strings.TrimSpace(tool) == "" {
			return fmt.Errorf("%w: empty allowed tool", ErrTaskCapabilityDenied)
		}
		actual := TaskCapabilityGatewayCallV1{
			WorkspaceID: scope.WorkspaceID, IssueID: scope.IssueID,
			ExecutorAgentID: scope.ExecutorAgentID, RuntimeID: scope.RuntimeID,
			RoleID: scope.RoleID, ConfigurationID: scope.ConfigurationID,
			RouteVersion: scope.RouteVersion, HostAttestation: scope.HostAttestation,
			GatewayKind: CapabilityGatewayPluginHook, Action: scope.Action,
			TargetNodeIDs: append([]string(nil), scope.Receipt.TargetNodeIDs...), AttemptedTool: tool,
			CredentialPurposes: append([]string(nil), scope.Receipt.CredentialPurposes...),
			CandidateID:        scope.Receipt.CandidateID, AuthorizedScope: scope.Receipt.AuthorizedScope,
			AssuranceConfigurationID: assuranceConfig,
		}
		if _, err := ValidateTaskCapabilityGatewayV1(&scope, actual, now); err != nil {
			return err
		}
	}
	return nil
}

func canonicalizeTaskCapabilityTimes(scope *TaskCapabilityScopeV1) {
	scope.Receipt.IssuedAt = canonicalPermissionTime(scope.Receipt.IssuedAt)
	scope.Receipt.ExpiresAt = canonicalPermissionTime(scope.Receipt.ExpiresAt)
	if scope.Assurance != nil {
		scope.Assurance.IssuedAt = canonicalPermissionTime(scope.Assurance.IssuedAt)
		scope.Assurance.ExpiresAt = canonicalPermissionTime(scope.Assurance.ExpiresAt)
	}
	if scope.OwnerAuthorization != nil {
		scope.OwnerAuthorization.RecordedAt = canonicalPermissionTime(scope.OwnerAuthorization.RecordedAt)
	}
}

func parseRuntimeBindingIdentity(receipt QualifiedRuntimeBindingReceiptV2) (pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	workspaceID, err := parsePermissionUUID(receipt.WorkspaceID, "workspace id")
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	issueID, err := parsePermissionUUID(receipt.IssueID, "issue id")
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	agentID, err := parsePermissionUUID(receipt.ExecutorAgentID, "executor agent id")
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	runtimeID, err := parsePermissionUUID(receipt.RuntimeID, "runtime id")
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	return workspaceID, issueID, agentID, runtimeID, nil
}

func parseCapabilityScopeIdentity(scope TaskCapabilityScopeV1) (pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	receipt := QualifiedRuntimeBindingReceiptV2{WorkspaceID: scope.WorkspaceID, IssueID: scope.IssueID, ExecutorAgentID: scope.ExecutorAgentID, RuntimeID: scope.RuntimeID}
	return parseRuntimeBindingIdentity(receipt)
}

func parseLoadIdentity(identity GuardedExecutionLoadIdentityV1) (pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	taskID, err := parsePermissionUUID(identity.TaskID, "task id")
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	workspaceID, issueID, agentID, runtimeID, err := parseRuntimeBindingIdentity(QualifiedRuntimeBindingReceiptV2{
		WorkspaceID: identity.WorkspaceID, IssueID: identity.IssueID,
		ExecutorAgentID: identity.ExecutorAgentID, RuntimeID: identity.RuntimeID,
	})
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	if _, err := parsePermissionUUID(identity.ExpectedControllerID, "expected controller id"); err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	return taskID, workspaceID, issueID, agentID, runtimeID, nil
}

func decodeRuntimeBindingRow(row db.QualifiedRuntimeBindingReceipt, expectedControllerID string) (QualifiedRuntimeBindingReceiptV2, error) {
	if util.UUIDToString(row.IssuedByControllerID) != expectedControllerID {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("%w: runtime binding issuer mismatch", ErrGuardedConfigurationUnbound)
	}
	var header struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(row.Receipt, &header); err != nil {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("%w: decode stored runtime binding version: %v", ErrGuardedDispatchInvalid, err)
	}
	if header.Version != QualifiedRuntimeBindingVersionV2 {
		return QualifiedRuntimeBindingReceiptV2{}, fmt.Errorf("%w: stored runtime binding is not child-bound v2 evidence", ErrGuardedConfigurationUnbound)
	}
	var receipt QualifiedRuntimeBindingReceiptV2
	if err := strictJSON(row.Receipt, &receipt); err != nil {
		return receipt, fmt.Errorf("%w: decode stored runtime binding: %v", ErrGuardedDispatchInvalid, err)
	}
	_, digest, err := permissionDigest(receipt)
	if err != nil || digest != row.ReceiptSha256 {
		return receipt, fmt.Errorf("%w: stored runtime binding digest mismatch", ErrGuardedDispatchInvalid)
	}
	var evidenceIDs []string
	if err := strictJSON(row.EvidenceReceiptIds, &evidenceIDs); err != nil || !reflect.DeepEqual(evidenceIDs, receipt.EvidenceReceiptIDs) {
		return receipt, fmt.Errorf("%w: stored runtime evidence mismatch", ErrGuardedDispatchInvalid)
	}
	if row.ID != receipt.ID || util.UUIDToString(row.WorkspaceID) != receipt.WorkspaceID ||
		util.UUIDToString(row.IssueID) != receipt.IssueID || util.UUIDToString(row.ExecutorAgentID) != receipt.ExecutorAgentID ||
		util.UUIDToString(row.RuntimeID) != receipt.RuntimeID || row.ConfigurationID != receipt.ConfigurationID ||
		row.RoleID != receipt.RoleID || row.RuntimeProvider != receipt.RuntimeProvider || row.Model != receipt.Model ||
		row.ReasoningEffort != receipt.ReasoningEffort || row.ArtifactBindingSha256 != receipt.ArtifactBindingSHA256 ||
		row.QualificationStatus != receipt.QualificationStatus || row.Source != receipt.Source || row.Immutable != receipt.Immutable ||
		!row.IssuedAt.Valid || !row.ExpiresAt.Valid || !row.IssuedAt.Time.Equal(receipt.IssuedAt) || !row.ExpiresAt.Time.Equal(receipt.ExpiresAt) {
		return receipt, fmt.Errorf("%w: stored runtime binding columns disagree with receipt", ErrGuardedDispatchInvalid)
	}
	return receipt, nil
}

func decodeCapabilityGrantRow(row db.GuardedTaskCapabilityGrant, expectedControllerID, expectedTaskID string) (TaskCapabilityScopeV1, map[string][]string, error) {
	var scope TaskCapabilityScopeV1
	if util.UUIDToString(row.IssuedByControllerID) != expectedControllerID || util.UUIDToString(row.TaskID) != expectedTaskID {
		return scope, nil, fmt.Errorf("%w: capability grant issuer or task mismatch", ErrTaskCapabilityDenied)
	}
	if err := strictJSON(row.CapabilityScope, &scope); err != nil {
		return scope, nil, fmt.Errorf("%w: decode stored capability scope: %v", ErrTaskCapabilityDenied, err)
	}
	var targets map[string][]string
	if err := strictJSON(row.PluginToolTargets, &targets); err != nil {
		return scope, nil, fmt.Errorf("%w: decode stored plugin targets: %v", ErrTaskCapabilityDenied, err)
	}
	if targets == nil {
		targets = map[string][]string{}
	}
	_, digest, err := permissionDigest(storedCapabilityPayloadV1{Scope: scope, PluginToolTargets: targets})
	if err != nil || digest != row.PayloadSha256 {
		return scope, nil, fmt.Errorf("%w: stored capability grant digest mismatch", ErrTaskCapabilityDenied)
	}
	if row.ID != scope.Receipt.ID || util.UUIDToString(row.WorkspaceID) != scope.WorkspaceID ||
		util.UUIDToString(row.IssueID) != scope.IssueID || util.UUIDToString(row.ExecutorAgentID) != scope.ExecutorAgentID ||
		util.UUIDToString(row.RuntimeID) != scope.RuntimeID || row.RuntimeBindingReceiptID != scope.RuntimeBindingReceiptID ||
		row.ConfigurationID != scope.ConfigurationID || row.RoleID != scope.RoleID || row.RouteVersion != scope.RouteVersion ||
		row.HostAttestation != scope.HostAttestation || row.Action != scope.Action || !row.IssuedAt.Valid || !row.ExpiresAt.Valid ||
		!row.IssuedAt.Time.Equal(scope.Receipt.IssuedAt) || !row.ExpiresAt.Time.Equal(scope.Receipt.ExpiresAt) {
		return scope, nil, fmt.Errorf("%w: stored capability columns disagree with receipt", ErrTaskCapabilityDenied)
	}
	return scope, targets, nil
}

func clonePluginTargets(source map[string][]string) map[string][]string {
	if len(source) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(source))
	for tool, targets := range source {
		out[tool] = append([]string(nil), targets...)
	}
	return out
}
