-- name: CreateQualifiedRuntimeBindingReceipt :one
INSERT INTO qualified_runtime_binding_receipt (
    id, workspace_id, issue_id, executor_agent_id, runtime_id,
    configuration_id, role_id, runtime_provider, model, reasoning_effort,
    artifact_binding_sha256, qualification_status, evidence_receipt_ids,
    source, immutable, issued_by_controller_id, issued_at, expires_at,
    receipt, receipt_sha256
) VALUES (
    @id, @workspace_id, @issue_id, @executor_agent_id, @runtime_id,
    @configuration_id, @role_id, @runtime_provider, @model, @reasoning_effort,
    @artifact_binding_sha256, @qualification_status, @evidence_receipt_ids,
    @source, @immutable, @issued_by_controller_id, @issued_at, @expires_at,
    @receipt, @receipt_sha256
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetQualifiedRuntimeBindingReceiptByID :one
SELECT * FROM qualified_runtime_binding_receipt
WHERE id = @id;

-- name: GetActiveQualifiedRuntimeBindingReceiptForExecution :one
SELECT binding.*
FROM qualified_runtime_binding_receipt AS binding
WHERE binding.id = @id
  AND binding.workspace_id = @workspace_id
  AND binding.issue_id = @issue_id
  AND binding.executor_agent_id = @executor_agent_id
  AND binding.runtime_id = @runtime_id
  AND binding.expires_at > @now
  AND NOT EXISTS (
      SELECT 1
      FROM qualified_runtime_binding_revocation AS revocation
      WHERE revocation.workspace_id = binding.workspace_id
        AND revocation.receipt_id = binding.id
  );

-- name: RevokeQualifiedRuntimeBindingReceipt :one
INSERT INTO qualified_runtime_binding_revocation (
    receipt_id, workspace_id, revoked_by_controller_id, reason, revoked_at
) SELECT
    @receipt_id, @workspace_id, @revoked_by_controller_id, @reason, @revoked_at
WHERE EXISTS (
    SELECT 1
    FROM qualified_runtime_binding_receipt
    WHERE id = @receipt_id
      AND workspace_id = @workspace_id
      AND issued_by_controller_id = @revoked_by_controller_id
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetQualifiedRuntimeBindingRevocation :one
SELECT * FROM qualified_runtime_binding_revocation
WHERE workspace_id = @workspace_id AND receipt_id = @receipt_id;

-- name: CreateGuardedTaskCapabilityGrant :one
INSERT INTO guarded_task_capability_grant (
    id, task_id, workspace_id, issue_id, executor_agent_id, runtime_id,
    runtime_binding_receipt_id, configuration_id, role_id, route_version,
    host_attestation, action, issued_by_controller_id, issued_at, expires_at,
    capability_scope, plugin_tool_targets, payload_sha256
) VALUES (
    @id, @task_id, @workspace_id, @issue_id, @executor_agent_id, @runtime_id,
    @runtime_binding_receipt_id, @configuration_id, @role_id, @route_version,
    @host_attestation, @action, @issued_by_controller_id, @issued_at, @expires_at,
    @capability_scope, @plugin_tool_targets, @payload_sha256
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetGuardedTaskCapabilityGrantByID :one
SELECT * FROM guarded_task_capability_grant
WHERE id = @id;

-- name: GetActiveGuardedTaskCapabilityGrantForExecution :one
SELECT grant_record.*
FROM guarded_task_capability_grant AS grant_record
WHERE grant_record.task_id = @task_id
  AND grant_record.workspace_id = @workspace_id
  AND grant_record.issue_id = @issue_id
  AND grant_record.executor_agent_id = @executor_agent_id
  AND grant_record.runtime_id = @runtime_id
  AND grant_record.runtime_binding_receipt_id = @runtime_binding_receipt_id
  AND grant_record.expires_at > @now
  AND NOT EXISTS (
      SELECT 1
      FROM guarded_task_capability_revocation AS revocation
      WHERE revocation.workspace_id = grant_record.workspace_id
        AND revocation.grant_id = grant_record.id
  );

-- name: RevokeGuardedTaskCapabilityGrant :one
INSERT INTO guarded_task_capability_revocation (
    grant_id, workspace_id, revoked_by_controller_id, reason, revoked_at
) SELECT
    @grant_id, @workspace_id, @revoked_by_controller_id, @reason, @revoked_at
WHERE EXISTS (
    SELECT 1
    FROM guarded_task_capability_grant
    WHERE id = @grant_id
      AND workspace_id = @workspace_id
      AND issued_by_controller_id = @revoked_by_controller_id
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetGuardedTaskCapabilityRevocation :one
SELECT * FROM guarded_task_capability_revocation
WHERE workspace_id = @workspace_id AND grant_id = @grant_id;

-- name: ConsumeGuardedExecutionAuthority :one
-- Appends the one allowed launch only while every controller-owned authority
-- fact is still active and the authenticated daemon still owns this runtime.
-- ON CONFLICT is intentionally not idempotent at the application boundary: a
-- duplicate task or grant returns no row and therefore cannot launch twice.
INSERT INTO guarded_execution_launch_consumption (
    task_id, workspace_id, issue_id, executor_agent_id, runtime_id,
    runtime_binding_receipt_id, capability_grant_id,
    consumed_by_daemon_id, package_sha256, consumed_at
)
SELECT
    @task_id, @workspace_id, @issue_id, @executor_agent_id, @runtime_id,
    @runtime_binding_receipt_id, @capability_grant_id,
    @consumed_by_daemon_id, @package_sha256, @consumed_at
FROM guarded_task_capability_grant AS grant_record
JOIN qualified_runtime_binding_receipt AS binding
  ON binding.id = grant_record.runtime_binding_receipt_id
 AND binding.workspace_id = grant_record.workspace_id
 AND binding.issue_id = grant_record.issue_id
 AND binding.executor_agent_id = grant_record.executor_agent_id
 AND binding.runtime_id = grant_record.runtime_id
JOIN agent_runtime AS runtime
  ON runtime.id = grant_record.runtime_id
 AND runtime.workspace_id = grant_record.workspace_id
WHERE grant_record.id = @capability_grant_id
  AND grant_record.task_id = @task_id
  AND grant_record.workspace_id = @workspace_id
  AND grant_record.issue_id = @issue_id
  AND grant_record.executor_agent_id = @executor_agent_id
  AND grant_record.runtime_id = @runtime_id
  AND grant_record.runtime_binding_receipt_id = @runtime_binding_receipt_id
  AND grant_record.issued_by_controller_id = @expected_controller_id
  AND binding.issued_by_controller_id = @expected_controller_id
  AND grant_record.expires_at > @consumed_at
  AND binding.expires_at > @consumed_at
  AND runtime.daemon_id = @consumed_by_daemon_id
  AND NOT EXISTS (
      SELECT 1
      FROM guarded_task_capability_revocation AS grant_revocation
      WHERE grant_revocation.workspace_id = grant_record.workspace_id
        AND grant_revocation.grant_id = grant_record.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM qualified_runtime_binding_revocation AS binding_revocation
      WHERE binding_revocation.workspace_id = binding.workspace_id
        AND binding_revocation.receipt_id = binding.id
  )
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetGuardedExecutionLaunchConsumption :one
SELECT * FROM guarded_execution_launch_consumption
WHERE workspace_id = @workspace_id AND task_id = @task_id;
