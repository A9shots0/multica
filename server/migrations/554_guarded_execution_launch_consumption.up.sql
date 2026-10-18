-- One-shot controller authority consumption for guarded execution.
--
-- The row is appended in the same transaction that moves a task to running.
-- It is deliberately relationship-free and immutable; repository policy keeps
-- referential checks in the application/query boundary and forbids cascades.

CREATE TABLE guarded_execution_launch_consumption (
    task_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    executor_agent_id UUID NOT NULL,
    runtime_id UUID NOT NULL,
    runtime_binding_receipt_id TEXT NOT NULL CHECK (length(btrim(runtime_binding_receipt_id)) BETWEEN 1 AND 256),
    capability_grant_id TEXT NOT NULL CHECK (length(btrim(capability_grant_id)) BETWEEN 1 AND 256),
    consumed_by_daemon_id TEXT NOT NULL CHECK (length(btrim(consumed_by_daemon_id)) BETWEEN 1 AND 256),
    package_sha256 TEXT NOT NULL CHECK (package_sha256 ~ '^[0-9a-f]{64}$'),
    consumed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_guarded_execution_launch_consumption_append_only
BEFORE UPDATE OR DELETE ON guarded_execution_launch_consumption
FOR EACH ROW EXECUTE FUNCTION reject_guarded_permission_receipt_update();
