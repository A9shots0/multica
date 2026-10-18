-- Durable controller evidence for Item 14 system-graph responsibilities.
--
-- These records are append-only and relationship-free. The controller store
-- validates workflow identities and evidence references inside one database
-- transaction; repository policy forbids foreign keys and cascades.

CREATE TABLE issue_workflow_consequence_receipt (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    source_issue_id UUID NOT NULL,
    affected_issue_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 256),
    receipt_id TEXT NOT NULL CHECK (length(btrim(receipt_id)) BETWEEN 1 AND 256),
    controller_authority_receipt_id TEXT NOT NULL CHECK (length(btrim(controller_authority_receipt_id)) BETWEEN 1 AND 256),
    controller_id TEXT NOT NULL CHECK (length(btrim(controller_id)) BETWEEN 1 AND 256),
    graph_version TEXT NOT NULL CHECK (length(btrim(graph_version)) BETWEEN 1 AND 256),
    source_outcome_id UUID NOT NULL,
    affected_outcome_id UUID NOT NULL,
    source_scope_revision BIGINT NOT NULL CHECK (source_scope_revision >= 1),
    affected_scope_revision BIGINT NOT NULL CHECK (affected_scope_revision >= 1),
    source_authority_revision BIGINT NOT NULL CHECK (source_authority_revision >= 1),
    affected_authority_revision BIGINT NOT NULL CHECK (affected_authority_revision >= 1),
    relationship_kind TEXT NOT NULL CHECK (length(btrim(relationship_kind)) BETWEEN 1 AND 256),
    payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    receipt JSONB NOT NULL CHECK (jsonb_typeof(receipt) = 'object'),
    committed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (source_issue_id <> affected_issue_id),
    CHECK (source_outcome_id <> affected_outcome_id)
);

CREATE TABLE issue_workflow_regression_change_record (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    source_issue_id UUID NOT NULL,
    registry_version BIGINT NOT NULL CHECK (registry_version >= 1),
    proposal_id TEXT NOT NULL CHECK (length(btrim(proposal_id)) BETWEEN 1 AND 256),
    obligation_id TEXT NOT NULL CHECK (length(btrim(obligation_id)) BETWEEN 1 AND 256),
    record_hash TEXT NOT NULL CHECK (record_hash ~ '^sha256:[0-9a-f]{64}$'),
    previous_record_hash TEXT NOT NULL DEFAULT '' CHECK (previous_record_hash = '' OR previous_record_hash ~ '^sha256:[0-9a-f]{64}$'),
    record JSONB NOT NULL CHECK (jsonb_typeof(record) = 'object'),
    applied_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE issue_workflow_regression_rejected_attempt (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    source_issue_id UUID NOT NULL,
    sequence BIGINT NOT NULL CHECK (sequence >= 1),
    registry_version BIGINT NOT NULL CHECK (registry_version >= 0),
    proposal_id TEXT NOT NULL CHECK (length(btrim(proposal_id)) BETWEEN 1 AND 256),
    obligation_id TEXT NOT NULL CHECK (length(btrim(obligation_id)) BETWEEN 1 AND 256),
    submission_hash TEXT NOT NULL CHECK (submission_hash ~ '^sha256:[0-9a-f]{64}$'),
    attempt_hash TEXT NOT NULL CHECK (attempt_hash ~ '^sha256:[0-9a-f]{64}$'),
    previous_attempt_hash TEXT NOT NULL DEFAULT '' CHECK (previous_attempt_hash = '' OR previous_attempt_hash ~ '^sha256:[0-9a-f]{64}$'),
    attempt JSONB NOT NULL CHECK (jsonb_typeof(attempt) = 'object'),
    attempted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION reject_system_graph_workflow_evidence_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% rows are append-only', TG_TABLE_NAME;
END;
$$;

CREATE TRIGGER trg_issue_workflow_consequence_receipt_append_only
BEFORE UPDATE OR DELETE ON issue_workflow_consequence_receipt
FOR EACH ROW EXECUTE FUNCTION reject_system_graph_workflow_evidence_update();

CREATE TRIGGER trg_issue_workflow_regression_change_record_append_only
BEFORE UPDATE OR DELETE ON issue_workflow_regression_change_record
FOR EACH ROW EXECUTE FUNCTION reject_system_graph_workflow_evidence_update();

CREATE TRIGGER trg_issue_workflow_regression_rejected_attempt_append_only
BEFORE UPDATE OR DELETE ON issue_workflow_regression_rejected_attempt
FOR EACH ROW EXECUTE FUNCTION reject_system_graph_workflow_evidence_update();
