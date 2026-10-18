-- name: LockCompanyServiceKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key, 0));

-- name: GetCompanyService :one
SELECT * FROM company_service
WHERE workspace_id = @workspace_id AND service_key = @service_key;

-- name: UpsertCompanyService :one
INSERT INTO company_service (
    workspace_id, service_key, display_name, area_key, environment,
    source_system, source_native_id, project_id, authority,
    freshness_state, privacy_class, observed_at, expires_at, metadata,
    system_node_ids, journey_ids
) VALUES (
    @workspace_id, @service_key, @display_name, @area_key, @environment,
    @source_system, @source_native_id, @project_id, @authority,
    @freshness_state, @privacy_class, @observed_at, @expires_at, @metadata,
    @system_node_ids, @journey_ids
)
ON CONFLICT (workspace_id, service_key) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    area_key = EXCLUDED.area_key,
    environment = EXCLUDED.environment,
    source_system = EXCLUDED.source_system,
    source_native_id = EXCLUDED.source_native_id,
    project_id = EXCLUDED.project_id,
    authority = EXCLUDED.authority,
    freshness_state = EXCLUDED.freshness_state,
    privacy_class = EXCLUDED.privacy_class,
    observed_at = EXCLUDED.observed_at,
    expires_at = EXCLUDED.expires_at,
    metadata = EXCLUDED.metadata,
    system_node_ids = EXCLUDED.system_node_ids,
    journey_ids = EXCLUDED.journey_ids,
    revision = company_service.revision + 1,
    updated_at = now()
RETURNING *;

-- name: ListCompanyServices :many
SELECT * FROM company_service
WHERE workspace_id = @workspace_id
ORDER BY area_key, display_name, environment;

-- name: AdmitCompanySignalEvent :one
INSERT INTO company_signal_event (
    workspace_id, source_key, event_id, request_hash, contract_version,
    source_system, source_instance, source_native_id, source_revision,
    event_kind, source_lane, lifecycle_state, severity, action_required,
    summary, next_step, affected_refs, service_key, environment,
    project_ref, outcome_ref, evidence_refs, predecessor_event_id,
    privacy_class, raw_payload_hash, correlation_key, incident_generation,
    incident_id, observed_at
) VALUES (
    @workspace_id, @source_key, @event_id, @request_hash, @contract_version,
    @source_system, @source_instance, @source_native_id, @source_revision,
    @event_kind, @source_lane, @lifecycle_state, @severity, @action_required,
    @summary, @next_step, @affected_refs, @service_key, @environment,
    @project_ref, @outcome_ref, @evidence_refs, @predecessor_event_id,
    @privacy_class, @raw_payload_hash, @correlation_key, @incident_generation,
    @incident_id, @observed_at
)
ON CONFLICT (workspace_id, source_key, event_id) DO UPDATE SET
    request_hash = company_signal_event.request_hash
WHERE company_signal_event.request_hash = EXCLUDED.request_hash
RETURNING *;

-- name: GetCompanySignalEvent :one
SELECT * FROM company_signal_event
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_id = @event_id;

-- name: GetCompanySignalPredecessor :one
SELECT * FROM company_signal_event
WHERE workspace_id = @workspace_id AND source_key = @source_key AND event_id = @event_id;

-- name: GetLatestCompanySignalStreamEvent :one
SELECT * FROM company_signal_event
WHERE workspace_id = @workspace_id
  AND source_key = @source_key
  AND source_instance = @source_instance
  AND source_native_id = @source_native_id
  AND source_lane = @source_lane
  AND state = 'applied'
ORDER BY observed_at DESC, received_at DESC
LIMIT 1;

-- name: LockCompanyIncidentKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key, 0));

-- name: GetCompanyIncident :one
SELECT * FROM company_incident
WHERE workspace_id = @workspace_id
  AND correlation_key = @correlation_key
  AND generation = @generation;

-- name: CreateCompanyIncident :one
INSERT INTO company_incident (
    id, workspace_id, correlation_key, generation, service_key, environment,
    title, lifecycle_state, severity, action_required, next_step,
    verification_state, project_id, opened_at, last_observed_at
) VALUES (
    @id, @workspace_id, @correlation_key, @generation, @service_key, @environment,
    @title, @lifecycle_state, @severity, @action_required, @next_step,
    @verification_state, @project_id, @opened_at, @last_observed_at
)
RETURNING *;

-- name: ListLatestCompanyIncidentSignals :many
SELECT DISTINCT ON (source_lane, source_system, source_instance, source_native_id)
    *
FROM company_signal_event
WHERE workspace_id = @workspace_id
  AND incident_id = @incident_id
  AND state IN ('received', 'applied')
ORDER BY source_lane, source_system, source_instance, source_native_id,
         observed_at DESC, received_at DESC;

-- name: UpdateCompanyIncident :one
UPDATE company_incident
SET title = @title,
    lifecycle_state = @lifecycle_state,
    severity = @severity,
    action_required = @action_required,
    next_step = @next_step,
    verification_state = @verification_state,
    project_id = @project_id,
    last_observed_at = @last_observed_at,
    resolved_at = @resolved_at,
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND revision = @expected_revision
RETURNING *;

-- name: MarkCompanySignalApplied :one
UPDATE company_signal_event
SET state = 'applied', result = @result, error_code = NULL,
    applied_at = now(), updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key
  AND event_id = @event_id AND request_hash = @request_hash
RETURNING *;

-- name: MarkCompanySignalRejected :one
UPDATE company_signal_event
SET state = 'rejected', result = @result, error_code = @error_code,
    updated_at = now()
WHERE workspace_id = @workspace_id AND source_key = @source_key
  AND event_id = @event_id AND request_hash = @request_hash
RETURNING *;

-- name: QuarantineCompanySignalConflict :one
INSERT INTO company_signal_quarantine (
    id, workspace_id, source_key, event_id, stored_request_hash,
    received_request_hash, reason_code, received_payload
) VALUES (
    @id, @workspace_id, @source_key, @event_id, @stored_request_hash,
    @received_request_hash, @reason_code, @received_payload
)
ON CONFLICT (workspace_id, source_key, event_id, received_request_hash)
DO UPDATE SET received_request_hash = company_signal_quarantine.received_request_hash
RETURNING *;

-- name: ListCompanyIncidents :many
SELECT * FROM company_incident
WHERE workspace_id = @workspace_id
ORDER BY
    CASE lifecycle_state WHEN 'needs_human' THEN 0 WHEN 'open' THEN 1 WHEN 'recovering' THEN 2 ELSE 3 END,
    last_observed_at DESC
LIMIT @result_limit;

-- name: ListCompanySignalsForIncidents :many
SELECT * FROM company_signal_event
WHERE workspace_id = @workspace_id
  AND incident_id = ANY(@incident_ids::uuid[])
  AND state = 'applied'
ORDER BY observed_at ASC, received_at ASC
LIMIT @result_limit;

-- name: ListCompanyActiveAgentTasks :many
-- Current effort is intentionally narrower than task history. Terminal tasks
-- stay in History; the company operating picture receives only work that can
-- still consume capacity or is waiting to start/resume.
SELECT
    task.id,
    task.status,
    task.issue_id,
    issue.title AS issue_title,
    issue.project_id,
    agent.id AS agent_id,
    agent.name AS agent_name,
    task.trigger_summary,
    CASE
      WHEN task.chat_session_id IS NOT NULL THEN 'chat'
      WHEN task.autopilot_run_id IS NOT NULL THEN 'autopilot'
      WHEN task.issue_id IS NOT NULL THEN 'issue'
      ELSE 'quick_create'
    END::text AS work_source,
    task.created_at,
    task.dispatched_at,
    task.started_at
FROM agent_task_queue task
JOIN agent ON agent.id = task.agent_id
LEFT JOIN issue ON issue.id = task.issue_id AND issue.workspace_id = agent.workspace_id
WHERE agent.workspace_id = @workspace_id
  AND task.status IN ('queued', 'dispatched', 'running', 'waiting_local_directory')
ORDER BY COALESCE(task.started_at, task.dispatched_at, task.created_at) DESC, task.id DESC
LIMIT @result_limit;

-- name: ListCompanyLatestExternalWorkReceipts :many
-- One current observation per provider/work stream. Older receipts remain in
-- issue history and must not inflate the company-wide count.
SELECT latest.*
FROM (
    SELECT DISTINCT ON (receipt.issue_id, receipt.provider, receipt.work_id)
        receipt.id,
        receipt.issue_id,
        issue.title AS issue_title,
        issue.project_id,
        picture.outcome_state,
        picture.outcome_open,
        receipt.provider,
        receipt.work_id,
        receipt.run_id,
        receipt.receipt_revision,
        receipt.raw_receipt,
        receipt.admitted_at,
        receipt.updated_at
    FROM issue_external_work_receipt receipt
    JOIN issue ON issue.id = receipt.issue_id AND issue.workspace_id = receipt.workspace_id
    LEFT JOIN issue_current_picture picture
      ON picture.issue_id = receipt.issue_id AND picture.workspace_id = receipt.workspace_id
    WHERE receipt.workspace_id = @workspace_id
      AND receipt.evidence_id IS NOT NULL
      AND receipt.execution_id IS NOT NULL
      AND (
        COALESCE(receipt.raw_receipt->>'state', '') NOT IN ('complete', 'stopped')
        OR COALESCE(picture.outcome_open, TRUE)
      )
    ORDER BY receipt.issue_id, receipt.provider, receipt.work_id,
             receipt.receipt_revision DESC, receipt.id DESC
) latest
ORDER BY latest.updated_at DESC, latest.id DESC
LIMIT @result_limit;

-- name: UpsertCompanyWorkTopologyBinding :one
INSERT INTO company_work_topology_binding (
    workspace_id, issue_id, graph_version, system_node_ids, journey_ids,
    authority, evidence_refs, observed_at, expires_at
) VALUES (
    @workspace_id, @issue_id, @graph_version, @system_node_ids, @journey_ids,
    @authority, @evidence_refs, @observed_at, @expires_at
)
ON CONFLICT (workspace_id, issue_id) DO UPDATE SET
    graph_version = EXCLUDED.graph_version,
    system_node_ids = EXCLUDED.system_node_ids,
    journey_ids = EXCLUDED.journey_ids,
    authority = EXCLUDED.authority,
    evidence_refs = EXCLUDED.evidence_refs,
    observed_at = EXCLUDED.observed_at,
    expires_at = EXCLUDED.expires_at,
    revision = company_work_topology_binding.revision + 1,
    updated_at = now()
RETURNING *;

-- name: ListCompanyWorkTopologyBindings :many
SELECT * FROM company_work_topology_binding
WHERE workspace_id = @workspace_id
ORDER BY updated_at DESC, issue_id
LIMIT @result_limit;
