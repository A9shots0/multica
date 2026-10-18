-- name: PutAgentTaskLoopObservation :one
INSERT INTO agent_task_loop_observation (
    id, workspace_id, issue_id, task_id, task_attempt, provider_attempt,
    schema_version, provider, tool, input_sha256, output_sha256,
    result_status, repeated_cycles, shadow_decision, fingerprint_sha256,
    observation_sha256
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15,
    $16
)
ON CONFLICT (workspace_id, task_id, task_attempt, provider_attempt, fingerprint_sha256) DO UPDATE
SET observation_sha256 = agent_task_loop_observation.observation_sha256
WHERE agent_task_loop_observation.observation_sha256 = EXCLUDED.observation_sha256
RETURNING *;

-- name: ListAgentTaskLoopObservations :many
SELECT *
FROM agent_task_loop_observation
WHERE workspace_id = $1 AND task_id = $2
ORDER BY task_attempt, provider_attempt, observed_at;
