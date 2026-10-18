-- name: PutAgentTaskUsageSample :one
INSERT INTO agent_task_usage_sample (
    id, workspace_id, issue_id, task_id, task_attempt, provider_attempt,
    backend_attempt, sequence, provider, model, input_tokens, output_tokens, cache_read_tokens,
    cache_write_tokens, provider_calls, source_event, model_context_window,
    model_context_window_kind, context_tokens, context_tokens_measurement_kind,
    context_ratio_ppm, shadow_decision, sample_sha256
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12, $13,
    $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23
)
ON CONFLICT (workspace_id, task_id, provider_attempt, backend_attempt, provider, model, sequence) DO UPDATE
SET sample_sha256 = agent_task_usage_sample.sample_sha256
WHERE agent_task_usage_sample.sample_sha256 = EXCLUDED.sample_sha256
RETURNING *;

-- name: ListAgentTaskUsageSamples :many
SELECT *
FROM agent_task_usage_sample
WHERE workspace_id = $1 AND task_id = $2
ORDER BY provider_attempt, backend_attempt, sequence, observed_at;
