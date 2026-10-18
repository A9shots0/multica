CREATE TABLE agent_task_loop_observation (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID,
    task_id UUID NOT NULL,
    task_attempt INTEGER NOT NULL CHECK (task_attempt > 0),
    provider_attempt INTEGER NOT NULL CHECK (provider_attempt > 0),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    provider TEXT NOT NULL CHECK (length(provider) BETWEEN 1 AND 64),
    tool TEXT NOT NULL CHECK (length(tool) BETWEEN 1 AND 256),
    input_sha256 TEXT NOT NULL CHECK (length(input_sha256) = 64),
    output_sha256 TEXT NOT NULL CHECK (length(output_sha256) = 64),
    result_status TEXT NOT NULL CHECK (length(result_status) BETWEEN 1 AND 64),
    repeated_cycles INTEGER NOT NULL CHECK (repeated_cycles = 3),
    shadow_decision TEXT NOT NULL CHECK (shadow_decision = 'would_checkpoint'),
    fingerprint_sha256 TEXT NOT NULL CHECK (length(fingerprint_sha256) = 64),
    observation_sha256 TEXT NOT NULL CHECK (length(observation_sha256) = 64),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
