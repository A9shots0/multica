CREATE TABLE issue_workflow_execution_checkpoint_snapshot (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    execution_id UUID NOT NULL,
    execution_revision BIGINT NOT NULL CHECK (execution_revision > 0),
    task_id UUID,
    actor_id UUID,
    scope_id UUID NOT NULL,
    scope_revision BIGINT NOT NULL CHECK (scope_revision > 0),
    authority_id UUID,
    authority_revision BIGINT,
    branch_key TEXT NOT NULL CHECK (length(branch_key) BETWEEN 1 AND 128),
    execution_state TEXT NOT NULL CHECK (length(execution_state) BETWEEN 1 AND 64),
    checkpoint JSONB NOT NULL CHECK (jsonb_typeof(checkpoint) = 'object'),
    snapshot_document JSONB NOT NULL CHECK (jsonb_typeof(snapshot_document) = 'object'),
    previous_revision BIGINT,
    previous_snapshot_sha256 TEXT,
    chain_anchor BOOLEAN NOT NULL,
    snapshot_sha256 TEXT NOT NULL CHECK (snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((authority_id IS NULL) = (authority_revision IS NULL)),
    CHECK (authority_revision IS NULL OR authority_revision > 0),
    CHECK (
        (chain_anchor AND previous_revision IS NULL AND previous_snapshot_sha256 IS NULL)
        OR
        (NOT chain_anchor AND previous_revision = execution_revision - 1 AND previous_snapshot_sha256 ~ '^[0-9a-f]{64}$')
    )
);
