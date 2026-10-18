-- name: GetExternalIncidentSourceBinding :one
SELECT * FROM external_incident_source_binding
WHERE workspace_id = @workspace_id AND source_key = @source_key;

-- name: LockExternalIncidentSourceKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key, 0));

-- name: UpsertExternalIncidentSourceBinding :one
INSERT INTO external_incident_source_binding (
    workspace_id, source_key, creator_agent_id, project_id,
    credential_hash, credential_hint, enabled
) VALUES (
    @workspace_id, @source_key, @creator_agent_id, @project_id,
    @credential_hash, @credential_hint, @enabled
)
ON CONFLICT (workspace_id, source_key) DO UPDATE SET
    creator_agent_id = EXCLUDED.creator_agent_id,
    project_id = EXCLUDED.project_id,
    credential_hash = EXCLUDED.credential_hash,
    credential_hint = EXCLUDED.credential_hint,
    enabled = EXCLUDED.enabled,
    revision = external_incident_source_binding.revision + 1,
    updated_at = now()
RETURNING *;

-- name: AdmitExternalIncidentOccurrence :one
INSERT INTO external_incident_occurrence (
    workspace_id, source_key, event_key, request_hash, finding_key, incident_key,
    source_sequence, predecessor_event_key,
    episode_generation, lifecycle_state, severity, action_required, summary, next_step,
    observed_at, payload, binding_revision, creator_agent_id, project_id
) VALUES (
    @workspace_id, @source_key, @event_key, @request_hash, @finding_key, @incident_key,
    @source_sequence, @predecessor_event_key,
    @episode_generation, @lifecycle_state, @severity, @action_required, @summary, @next_step,
    @observed_at, @payload, @binding_revision, @creator_agent_id, @project_id
)
ON CONFLICT (workspace_id, source_key, event_key) DO UPDATE SET
    request_hash = external_incident_occurrence.request_hash
WHERE external_incident_occurrence.request_hash = EXCLUDED.request_hash
RETURNING *;

-- name: GetExternalIncidentOccurrence :one
SELECT * FROM external_incident_occurrence
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_key = @event_key;

-- name: GetExternalIncidentPredecessor :one
SELECT * FROM external_incident_occurrence
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_key = @event_key;

-- name: GetLatestAppliedExternalIncidentOccurrence :one
SELECT * FROM external_incident_occurrence
WHERE workspace_id = @workspace_id AND source_key = @source_key
  AND finding_key = @finding_key AND state = 'applied'
ORDER BY source_sequence DESC
LIMIT 1;

-- name: ClaimExternalIncidentOccurrence :one
UPDATE external_incident_occurrence
SET state = 'processing', lease_token = @lease_token,
    lease_expires_at = @lease_expires_at, error_code = NULL, updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_key = @event_key
  AND request_hash = @request_hash
  AND (
      state IN ('received', 'pending_predecessor', 'failed')
      OR (state = 'processing' AND (lease_expires_at IS NULL OR lease_expires_at <= @now))
  )
RETURNING *;

-- name: LockExternalIncidentEpisodeKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key, 0));

-- name: GetExternalIncidentEpisode :one
SELECT * FROM external_incident_episode
WHERE workspace_id = @workspace_id AND source_key = @source_key
  AND finding_key = @finding_key AND generation = @generation;

-- name: GetLatestExternalIncidentEpisode :one
SELECT * FROM external_incident_episode
WHERE workspace_id = @workspace_id AND source_key = @source_key
  AND finding_key = @finding_key
ORDER BY generation DESC
LIMIT 1;

-- name: CreateExternalIncidentEpisode :one
INSERT INTO external_incident_episode (
    workspace_id, source_key, finding_key, incident_key, generation, issue_id,
    current_state, severity, current_event_key, current_source_sequence,
    machine_title, machine_description, machine_status, machine_priority,
    title_human_owned, description_human_owned, status_human_owned, priority_human_owned,
    opened_at, last_observed_at, resolved_at
) VALUES (
    @workspace_id, @source_key, @finding_key, @incident_key, @generation, @issue_id,
    @current_state, @severity, @current_event_key, @current_source_sequence,
    @machine_title, @machine_description, @machine_status, @machine_priority,
    @title_human_owned, @description_human_owned, @status_human_owned, @priority_human_owned,
    @opened_at, @last_observed_at, @resolved_at
)
RETURNING *;

-- name: AdvanceExternalIncidentEpisode :one
UPDATE external_incident_episode
SET current_state = @current_state, severity = @severity,
    current_event_key = @current_event_key, current_source_sequence = @current_source_sequence,
    machine_title = @machine_title, machine_description = @machine_description,
    machine_status = @machine_status, machine_priority = @machine_priority,
    title_human_owned = @title_human_owned,
    description_human_owned = @description_human_owned,
    status_human_owned = @status_human_owned,
    priority_human_owned = @priority_human_owned,
    last_observed_at = @last_observed_at,
    resolved_at = @resolved_at, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key
  AND finding_key = @finding_key AND generation = @generation
  AND revision = @expected_revision
RETURNING *;

-- name: GetExternalIncidentIssueForUpdate :one
SELECT * FROM issue
WHERE id = @id AND workspace_id = @workspace_id
  AND origin_type = 'external_incident' AND origin_id = @origin_id
FOR UPDATE;

-- name: UpdateExternalIncidentIssue :one
UPDATE issue AS current_issue
SET title = @title,
    description = @description,
    position = CASE WHEN status IS DISTINCT FROM @status THEN (
        SELECT COALESCE(MIN(target.position), 0) - 1
        FROM issue AS target
        WHERE target.workspace_id = current_issue.workspace_id AND target.status = @status
    ) ELSE position END,
    status = @status,
    priority = @priority,
    metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('external_incident', @incident_metadata::jsonb),
    revision = revision + 1,
    last_activity_at = GREATEST(COALESCE(last_activity_at, updated_at), now()),
    updated_at = now()
WHERE current_issue.id = @id AND current_issue.workspace_id = @workspace_id
  AND current_issue.origin_type = 'external_incident' AND current_issue.origin_id = @origin_id
RETURNING *;

-- name: MarkExternalIncidentOccurrenceApplied :one
UPDATE external_incident_occurrence
SET state = 'applied', issue_id = @issue_id, episode_revision = @episode_revision,
    result = @result, error_code = NULL, lease_token = NULL,
    lease_expires_at = NULL, processed_at = now(), updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_key = @event_key
  AND request_hash = @request_hash AND state = 'processing' AND lease_token = @lease_token
RETURNING *;

-- name: MarkExternalIncidentOccurrenceRejected :one
UPDATE external_incident_occurrence
SET state = 'rejected', error_code = @error_code, result = @result,
    lease_token = NULL, lease_expires_at = NULL, processed_at = now(), updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_key = @event_key
  AND request_hash = @request_hash AND state = 'processing' AND lease_token = @lease_token
RETURNING *;

-- name: MarkExternalIncidentOccurrencePending :one
UPDATE external_incident_occurrence
SET state = 'pending_predecessor', result = @result, error_code = 'predecessor_pending',
    lease_token = NULL, lease_expires_at = NULL, updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_key = @event_key
  AND request_hash = @request_hash AND state = 'processing' AND lease_token = @lease_token
RETURNING *;

-- name: MarkExternalIncidentOccurrenceFailed :one
UPDATE external_incident_occurrence
SET state = 'failed', error_code = @error_code, lease_token = NULL,
    lease_expires_at = NULL, updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_key = @event_key
  AND request_hash = @request_hash AND state = 'processing' AND lease_token = @lease_token
RETURNING *;

-- name: QuarantineExternalIncidentConflict :one
INSERT INTO external_incident_quarantine (
    id, workspace_id, source_key, event_key, stored_request_hash,
    received_request_hash, reason_code, received_payload
) VALUES (
    @id, @workspace_id, @source_key, @event_key, @stored_request_hash,
    @received_request_hash, @reason_code, @received_payload
)
ON CONFLICT (workspace_id, source_key, event_key, received_request_hash)
DO UPDATE SET received_request_hash = external_incident_quarantine.received_request_hash
RETURNING *;
