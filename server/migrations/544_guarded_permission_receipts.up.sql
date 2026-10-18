-- Controller-owned authority for guarded execution.
--
-- Receipt bodies are immutable snapshots. Revocation is a separate append-only
-- fact so a grant can never be silently edited or "unrevoked". Relationships
-- are application-owned: repository policy forbids foreign keys and cascades.

CREATE TABLE qualified_runtime_binding_receipt (
    id TEXT NOT NULL CHECK (length(btrim(id)) BETWEEN 1 AND 256),
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    executor_agent_id UUID NOT NULL,
    runtime_id UUID NOT NULL,
    configuration_id TEXT NOT NULL CHECK (length(btrim(configuration_id)) BETWEEN 1 AND 256),
    role_id TEXT NOT NULL CHECK (length(btrim(role_id)) BETWEEN 1 AND 256),
    runtime_provider TEXT NOT NULL CHECK (length(btrim(runtime_provider)) BETWEEN 1 AND 128),
    model TEXT NOT NULL CHECK (length(btrim(model)) BETWEEN 1 AND 256),
    reasoning_effort TEXT NOT NULL CHECK (length(btrim(reasoning_effort)) BETWEEN 1 AND 64),
    artifact_binding_sha256 TEXT NOT NULL CHECK (artifact_binding_sha256 ~ '^[0-9a-f]{64}$'),
    qualification_status TEXT NOT NULL CHECK (qualification_status = 'qualified_bounded'),
    evidence_receipt_ids JSONB NOT NULL CHECK (
        jsonb_typeof(evidence_receipt_ids) = 'array'
        AND jsonb_array_length(evidence_receipt_ids) > 0
    ),
    source TEXT NOT NULL CHECK (source = 'controller_record'),
    immutable BOOLEAN NOT NULL CHECK (immutable = TRUE),
    issued_by_controller_id UUID NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > issued_at),
    receipt JSONB NOT NULL CHECK (jsonb_typeof(receipt) = 'object'),
    receipt_sha256 TEXT NOT NULL CHECK (receipt_sha256 ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE qualified_runtime_binding_revocation (
    receipt_id TEXT NOT NULL CHECK (length(btrim(receipt_id)) BETWEEN 1 AND 256),
    workspace_id UUID NOT NULL,
    revoked_by_controller_id UUID NOT NULL,
    reason TEXT NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 1000),
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE guarded_task_capability_grant (
    id TEXT NOT NULL CHECK (length(btrim(id)) BETWEEN 1 AND 256),
    task_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    executor_agent_id UUID NOT NULL,
    runtime_id UUID NOT NULL,
    runtime_binding_receipt_id TEXT NOT NULL CHECK (length(btrim(runtime_binding_receipt_id)) BETWEEN 1 AND 256),
    configuration_id TEXT NOT NULL CHECK (length(btrim(configuration_id)) BETWEEN 1 AND 256),
    role_id TEXT NOT NULL CHECK (length(btrim(role_id)) BETWEEN 1 AND 256),
    route_version TEXT NOT NULL CHECK (length(btrim(route_version)) BETWEEN 1 AND 256),
    host_attestation TEXT NOT NULL CHECK (length(btrim(host_attestation)) BETWEEN 1 AND 256),
    action TEXT NOT NULL CHECK (length(btrim(action)) BETWEEN 1 AND 256),
    issued_by_controller_id UUID NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > issued_at),
    capability_scope JSONB NOT NULL CHECK (jsonb_typeof(capability_scope) = 'object'),
    plugin_tool_targets JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(plugin_tool_targets) = 'object'),
    payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE guarded_task_capability_revocation (
    grant_id TEXT NOT NULL CHECK (length(btrim(grant_id)) BETWEEN 1 AND 256),
    workspace_id UUID NOT NULL,
    revoked_by_controller_id UUID NOT NULL,
    reason TEXT NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 1000),
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION reject_guarded_permission_receipt_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% rows are append-only', TG_TABLE_NAME;
END;
$$;

CREATE TRIGGER trg_qualified_runtime_binding_receipt_append_only
BEFORE UPDATE OR DELETE ON qualified_runtime_binding_receipt
FOR EACH ROW EXECUTE FUNCTION reject_guarded_permission_receipt_update();

CREATE TRIGGER trg_qualified_runtime_binding_revocation_append_only
BEFORE UPDATE OR DELETE ON qualified_runtime_binding_revocation
FOR EACH ROW EXECUTE FUNCTION reject_guarded_permission_receipt_update();

CREATE TRIGGER trg_guarded_task_capability_grant_append_only
BEFORE UPDATE OR DELETE ON guarded_task_capability_grant
FOR EACH ROW EXECUTE FUNCTION reject_guarded_permission_receipt_update();

CREATE TRIGGER trg_guarded_task_capability_revocation_append_only
BEFORE UPDATE OR DELETE ON guarded_task_capability_revocation
FOR EACH ROW EXECUTE FUNCTION reject_guarded_permission_receipt_update();
