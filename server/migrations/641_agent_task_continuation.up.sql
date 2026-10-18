ALTER TABLE agent_task_queue
    ADD COLUMN continuation_root_task_id UUID,
    ADD COLUMN continuation_of_task_id UUID,
    ADD COLUMN continuation_step INTEGER,
    ADD COLUMN continuation_max_packages INTEGER,
    ADD COLUMN continuation_checkpoint_sha256 TEXT,
    ADD COLUMN continuation_execution_id UUID,
    ADD COLUMN continuation_execution_revision BIGINT,
    ADD COLUMN continuity_observation_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE agent_task_queue
    ADD CONSTRAINT ck_agent_task_continuation_shape CHECK (
        (
            continuation_root_task_id IS NULL
            AND continuation_of_task_id IS NULL
            AND continuation_step IS NULL
            AND continuation_max_packages IS NULL
            AND continuation_checkpoint_sha256 IS NULL
            AND continuation_execution_id IS NULL
            AND continuation_execution_revision IS NULL
        )
        OR
        (
            continuation_root_task_id IS NOT NULL
            AND continuation_step BETWEEN 1 AND 3
            AND continuation_max_packages BETWEEN 2 AND 3
            AND continuation_step <= continuation_max_packages
            AND (continuation_step = 1) = (continuation_of_task_id IS NULL)
            AND (continuation_step > 1) = (continuation_of_task_id IS NOT NULL)
        )
    ),
    ADD CONSTRAINT ck_agent_task_continuation_checkpoint_shape CHECK (
        (continuation_checkpoint_sha256 IS NULL
         AND continuation_execution_id IS NULL
         AND continuation_execution_revision IS NULL)
        OR
        (continuation_checkpoint_sha256 ~ '^[0-9a-f]{64}$'
         AND continuation_execution_id IS NOT NULL
         AND continuation_execution_revision > 0)
    );

CREATE TABLE agent_task_continuation_receipt (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    root_task_id UUID NOT NULL,
    source_task_id UUID NOT NULL,
    successor_task_id UUID NOT NULL,
    source_step INTEGER NOT NULL CHECK (source_step BETWEEN 1 AND 2),
    next_step INTEGER NOT NULL CHECK (next_step = source_step + 1 AND next_step BETWEEN 2 AND 3),
    max_packages INTEGER NOT NULL CHECK (max_packages BETWEEN 2 AND 3 AND next_step <= max_packages),
    -- Go enforces the exact 2 KiB canonical wire limit. JSONB's text renderer
    -- adds formatting whitespace, so this larger structural ceiling avoids
    -- rejecting a valid near-limit canonical receipt during persistence.
    checkpoint JSONB NOT NULL CHECK (jsonb_typeof(checkpoint) = 'object' AND octet_length(checkpoint::text) <= 4096),
    checkpoint_sha256 TEXT NOT NULL CHECK (checkpoint_sha256 ~ '^[0-9a-f]{64}$'),
    execution_id UUID NOT NULL,
    execution_revision BIGINT NOT NULL CHECK (execution_revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_agent_task_continuation_receipt_source UNIQUE (source_task_id)
);

