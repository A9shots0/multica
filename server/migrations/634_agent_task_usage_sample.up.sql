CREATE TABLE agent_task_usage_sample (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID,
    task_id UUID NOT NULL,
    task_attempt INTEGER NOT NULL CHECK (task_attempt > 0),
    provider_attempt INTEGER NOT NULL CHECK (provider_attempt > 0),
    backend_attempt INTEGER NOT NULL CHECK (backend_attempt > 0),
    sequence BIGINT NOT NULL CHECK (sequence > 0),
    provider TEXT NOT NULL CHECK (length(provider) BETWEEN 1 AND 32),
    model TEXT NOT NULL CHECK (length(model) BETWEEN 1 AND 256),
    input_tokens BIGINT NOT NULL CHECK (input_tokens >= 0),
    output_tokens BIGINT NOT NULL CHECK (output_tokens >= 0),
    cache_read_tokens BIGINT NOT NULL CHECK (cache_read_tokens >= 0),
    cache_write_tokens BIGINT NOT NULL CHECK (cache_write_tokens >= 0),
    provider_calls BIGINT NOT NULL CHECK (provider_calls > 0),
    source_event TEXT NOT NULL CHECK (length(source_event) BETWEEN 1 AND 128),
    model_context_window BIGINT,
    model_context_window_kind TEXT NOT NULL CHECK (model_context_window_kind IN ('provider_exact', 'unknown')),
    context_tokens BIGINT,
    context_tokens_measurement_kind TEXT NOT NULL CHECK (context_tokens_measurement_kind IN ('provider_exact', 'provider_proxy', 'unknown')),
    context_ratio_ppm BIGINT CHECK (context_ratio_ppm >= 0),
    shadow_decision TEXT NOT NULL CHECK (shadow_decision IN ('unknown', 'continue', 'would_prepare', 'would_request_checkpoint', 'would_stop')),
    sample_sha256 TEXT NOT NULL CHECK (length(sample_sha256) = 64),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (model_context_window_kind = 'unknown' AND model_context_window IS NULL)
        OR (model_context_window_kind = 'provider_exact' AND model_context_window > 0)
    ),
    CHECK (
        (context_tokens_measurement_kind = 'unknown' AND context_tokens IS NULL)
        OR (context_tokens_measurement_kind IN ('provider_exact', 'provider_proxy') AND context_tokens > 0)
    ),
    CHECK (
        (shadow_decision = 'unknown' AND context_ratio_ppm IS NULL)
        OR (shadow_decision <> 'unknown' AND context_ratio_ppm IS NOT NULL)
    )
);
