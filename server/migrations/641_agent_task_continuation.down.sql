DROP TABLE IF EXISTS agent_task_continuation_receipt;

ALTER TABLE agent_task_queue
    DROP CONSTRAINT IF EXISTS ck_agent_task_continuation_checkpoint_shape,
    DROP CONSTRAINT IF EXISTS ck_agent_task_continuation_shape,
    DROP COLUMN IF EXISTS continuity_observation_enabled,
    DROP COLUMN IF EXISTS continuation_execution_revision,
    DROP COLUMN IF EXISTS continuation_execution_id,
    DROP COLUMN IF EXISTS continuation_checkpoint_sha256,
    DROP COLUMN IF EXISTS continuation_max_packages,
    DROP COLUMN IF EXISTS continuation_step,
    DROP COLUMN IF EXISTS continuation_of_task_id,
    DROP COLUMN IF EXISTS continuation_root_task_id;

