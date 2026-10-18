-- name: PutAgentTaskContextManifest :one
INSERT INTO agent_task_context_manifest (
    id,
    workspace_id,
    issue_id,
    task_id,
    task_attempt,
    provider_attempt,
    schema_version,
    measurement_state,
    aggregate_limit_bytes,
    aggregate_bytes,
    aggregate_token_estimate,
    blocks,
    duplicates_rejected,
    manifest_sha256
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
ON CONFLICT (workspace_id, task_id, provider_attempt) DO UPDATE
SET manifest_sha256 = agent_task_context_manifest.manifest_sha256
WHERE agent_task_context_manifest.manifest_sha256 = EXCLUDED.manifest_sha256
RETURNING *;

-- name: GetAgentTaskContextManifest :one
SELECT *
FROM agent_task_context_manifest
WHERE workspace_id = $1 AND task_id = $2 AND provider_attempt = $3;

-- name: ListAgentTaskContextManifests :many
SELECT *
FROM agent_task_context_manifest
WHERE workspace_id = $1 AND task_id = $2
ORDER BY provider_attempt;
