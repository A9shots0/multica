-- name: BindBrainTask :one
-- Freeze the routing for an attempt only after workspace authorization. A
-- replay cannot silently move its handoff into a different brain project.
INSERT INTO brain_task_link (workspace_id,task_id,attempt,project_id,target_revision)
SELECT sqlc.arg(workspace_id),t.id,t.attempt,sqlc.arg(project_id),sqlc.arg(target_revision)
FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id
WHERE t.id=sqlc.arg(task_id) AND t.attempt=sqlc.arg(attempt)
  AND a.workspace_id=sqlc.arg(workspace_id)
ON CONFLICT (workspace_id,task_id,attempt) DO UPDATE SET task_id=EXCLUDED.task_id
RETURNING *;

-- name: CaptureBrainEvents :execrows
-- No high-water mark: a late source commit remains eligible. Freeze the exact
-- payload before HTTP delivery so crash retries preserve key AND body.
INSERT INTO brain_event_outbox (event_id,workspace_id,project_id,payload)
SELECT e.event_id,l.workspace_id,l.project_id,
  jsonb_build_object('project',l.project_id,'taskId',t.id::text,
    'kind',e.kind,'summary',e.summary,'observedAt',e.observed_at,
    'evidenceRefs',jsonb_build_array('multica://workspace/'||l.workspace_id::text||'/task/'||t.id::text),
    'targetRevision',l.target_revision,'sourceEventId',e.event_id,
    'ownerSystem','multica','idempotencyKey',e.event_id)
FROM brain_task_link l
JOIN agent_task_queue t ON t.id=l.task_id AND t.attempt=l.attempt
JOIN agent a ON a.id=t.agent_id AND a.workspace_id=l.workspace_id
CROSS JOIN LATERAL (
 SELECT 'multica:'||l.workspace_id::text||':'||t.id::text||':'||t.attempt::text||':started' AS event_id,
 'started'::text AS kind, t.started_at AS observed_at,
 'Multica saved a run start. Source evidence is the canonical run record; task acceptance is not established.'::text AS summary
 WHERE t.started_at IS NOT NULL
 UNION ALL
 SELECT 'multica:'||l.workspace_id::text||':'||t.id::text||':'||t.attempt::text||':'||t.status,
 t.status,t.completed_at,
 'Multica saved run status '||t.status||'. Result availability: '||CASE WHEN t.result IS NULL THEN 'missing' ELSE 'recorded' END||'. This observation does not accept the issue or certify a deployment. Read the canonical run for outcome details.'
 WHERE t.completed_at IS NOT NULL AND t.status IN ('completed','failed','cancelled')
) e
WHERE NOT EXISTS (SELECT 1 FROM brain_event_outbox o WHERE o.event_id=e.event_id)
ORDER BY e.observed_at,e.event_id LIMIT 100
ON CONFLICT (event_id) DO NOTHING;

-- name: GetBrainTaskLink :one
SELECT * FROM brain_task_link WHERE workspace_id=$1 AND task_id=$2 AND attempt=$3;

-- name: EnqueueBrainHandoff :one
INSERT INTO brain_event_outbox (event_id,workspace_id,project_id,payload)
SELECT $1,$2,$3,$4
WHERE EXISTS (SELECT 1 FROM workspace WHERE id=$2)
ON CONFLICT (event_id) DO UPDATE SET event_id=EXCLUDED.event_id
RETURNING *;

-- name: ListPendingBrainEvents :many
SELECT * FROM brain_event_outbox
WHERE delivered_at IS NULL AND NOT is_blocked AND next_attempt_at<=now()
  AND EXISTS (SELECT 1 FROM workspace w WHERE w.id=brain_event_outbox.workspace_id)
ORDER BY next_attempt_at,created_at,event_id LIMIT 50;

-- name: MarkBrainEventDelivered :exec
UPDATE brain_event_outbox SET delivered_at=now(),last_error=NULL
WHERE event_id=$1 AND delivered_at IS NULL;

-- name: MarkBrainEventFailure :exec
UPDATE brain_event_outbox SET attempts=attempts+1,last_error=sqlc.arg(last_error),
 is_blocked=sqlc.arg(is_blocked),next_attempt_at=now()+least(3600,5*power(2,least(attempts,9))) * interval '1 second'
WHERE event_id=sqlc.arg(event_id) AND delivered_at IS NULL AND is_blocked = false;
