CREATE TABLE agent_task_context_manifest (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID,
    task_id UUID NOT NULL,
    task_attempt INTEGER NOT NULL CHECK (task_attempt > 0),
    provider_attempt INTEGER NOT NULL CHECK (provider_attempt > 0),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    measurement_state TEXT NOT NULL CHECK (measurement_state IN ('exact', 'partial', 'estimated')),
    aggregate_limit_bytes BIGINT NOT NULL CHECK (aggregate_limit_bytes > 0),
    aggregate_bytes BIGINT NOT NULL CHECK (aggregate_bytes >= 0 AND aggregate_bytes <= aggregate_limit_bytes),
    aggregate_token_estimate BIGINT CHECK (aggregate_token_estimate >= 0),
    blocks JSONB NOT NULL CHECK (jsonb_typeof(blocks) = 'array'),
    duplicates_rejected JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(duplicates_rejected) = 'array'),
    manifest_sha256 TEXT NOT NULL CHECK (length(manifest_sha256) = 64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
