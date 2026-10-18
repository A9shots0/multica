-- name: IsOrdinaryChatIssue :one
-- A generic controller must not adopt ordinary conversation execution later.
-- Origin covers the opening before bind; a committed intake covers existing work.
SELECT EXISTS (
  SELECT 1 FROM issue i JOIN agent_task_queue t ON t.id=i.origin_id
  WHERE i.id=sqlc.arg(issue_id) AND i.workspace_id=sqlc.arg(workspace_id)
    AND i.origin_type='agent_create' AND t.chat_session_id IS NOT NULL AND t.squad_id IS NULL
) OR EXISTS (
  SELECT 1 FROM inbox_item b WHERE b.issue_id=sqlc.arg(issue_id) AND b.workspace_id=sqlc.arg(workspace_id)
    AND b.type='chat_intake' AND b.details->>'intake_state'='continued'
) AS ordinary;

-- name: ListChatWork :many
SELECT i.id, i.number, i.title, i.status, i.revision, i.project_id,
       l.source, l.created_at AS linked_at,
       COALESCE(p.document, '{}'::jsonb)::jsonb AS picture,
       COALESCE(p.revision, 0)::bigint AS picture_revision,
       COALESCE(p.generated_at, i.updated_at) AS observed_at,
       COALESCE(t.status, '')::text AS task_status,
       t.id AS task_id, t.agent_id, t.created_at AS task_created_at,
       COALESCE(t.wait_reason, '')::text AS wait_reason,
       COALESCE(rt.status = 'online' AND rt.last_seen_at > now() - sqlc.arg(freshness_seconds)::double precision * interval '1 second', false)::boolean AS runtime_fresh
FROM chat_issue_link l
JOIN issue i ON i.id = l.issue_id AND i.workspace_id = l.workspace_id
LEFT JOIN issue_current_picture p ON p.issue_id = i.id AND p.workspace_id = i.workspace_id
LEFT JOIN LATERAL (
  SELECT q.* FROM agent_task_queue q WHERE q.issue_id = i.id
  ORDER BY (q.status IN ('running', 'dispatched', 'queued', 'waiting_local_directory')) DESC, q.created_at DESC, q.id DESC LIMIT 1
) t ON true
LEFT JOIN agent_runtime rt ON rt.id = t.runtime_id AND rt.workspace_id = i.workspace_id
WHERE l.workspace_id = sqlc.arg(workspace_id) AND l.chat_session_id = sqlc.arg(chat_session_id)
ORDER BY l.created_at, i.id;

-- name: ListChatWorkActivity :many
-- Existing immutable workflow events are the durable source. No browser or
-- model must be online to deliver them. Cursor is a strict (time, id) tuple.
SELECT e.id, e.issue_id, e.event_type, e.reason, e.created_at
FROM issue_workflow_event e
JOIN chat_issue_link l ON l.workspace_id = e.workspace_id AND l.issue_id = e.issue_id
WHERE l.workspace_id = sqlc.arg(workspace_id) AND l.chat_session_id = sqlc.arg(chat_session_id)
  AND (e.created_at, e.id) < (sqlc.arg(before_time)::timestamptz, sqlc.arg(before_id)::uuid)
ORDER BY e.created_at DESC, e.id DESC LIMIT 51;

-- name: InheritChatIssueLinks :exec
INSERT INTO chat_issue_link (workspace_id, chat_session_id, issue_id, source)
SELECT parent.workspace_id, parent.chat_session_id, sqlc.arg(child_id), 'origin'
FROM chat_issue_link parent WHERE parent.workspace_id = sqlc.arg(workspace_id) AND parent.issue_id = sqlc.arg(parent_id)
ON CONFLICT (workspace_id, chat_session_id, issue_id) DO NOTHING;

-- name: SetChatWorkTarget :exec
UPDATE agent_task_queue SET context = jsonb_set(COALESCE(context, '{}'::jsonb), '{chat_work_target}', sqlc.arg(target)::jsonb)
WHERE id = sqlc.arg(task_id) AND chat_session_id = sqlc.arg(chat_session_id);

-- name: SetChatWorkResponses :exec
UPDATE agent_task_queue SET context = jsonb_set(COALESCE(context, '{}'::jsonb), '{chat_work_responses}', sqlc.arg(responses)::jsonb)
WHERE id = sqlc.arg(task_id) AND chat_session_id = sqlc.arg(chat_session_id);

-- name: SetChatOpeningTasks :exec
-- Frozen while sending into a conversation with no issue yet. Only these
-- already-present owner turns can create a later correction target.
UPDATE agent_task_queue SET context=jsonb_set(COALESCE(context,'{}'::jsonb),'{chat_opening_tasks}',sqlc.arg(opening_tasks)::jsonb)
WHERE id=sqlc.arg(task_id) AND chat_session_id=sqlc.arg(chat_session_id);

-- name: FreezeOpeningChatWorkTarget :one
-- Never replace an explicit/captured target or race a second successful freeze.
UPDATE agent_task_queue SET context=jsonb_set(context,'{chat_work_target}',sqlc.arg(target)::jsonb)
WHERE id=sqlc.arg(task_id) AND chat_session_id=sqlc.arg(chat_session_id)
  AND chat_input_task_id IS NOT NULL
  AND context->'chat_opening_tasks'=sqlc.arg(opening_tasks)::jsonb
  AND NOT context ? 'chat_work_target' AND NOT context ? 'chat_work_responses'
RETURNING *;

-- name: ListChatWorkIndex :many
-- Compact library states in one query; do not fetch every transcript's evidence.
SELECT c.id AS session_id, i.id AS issue_id,
       COALESCE(p.document, '{}'::jsonb)::jsonb AS picture,
       COALESCE(p.revision, 0)::bigint AS picture_revision,
       COALESCE(p.generated_at, i.updated_at) AS observed_at,
       COALESCE(t.status, '')::text AS task_status,
       COALESCE(rt.status = 'online' AND rt.last_seen_at > now() - sqlc.arg(freshness_seconds)::double precision * interval '1 second', false)::boolean AS runtime_fresh,
       EXISTS (SELECT 1 FROM inbox_item b WHERE b.workspace_id=c.workspace_id AND b.issue_id=i.id
         AND b.recipient_type='member' AND b.recipient_id=c.creator_id AND NOT b.archived
         AND b.details->>'intake_state' IS DISTINCT FROM 'continued'
         AND (b.type='workflow_owner_decision' OR (b.type='chat_intake' AND b.details->>'question_scope'='ordinary_task'))) AS needs_owner
FROM chat_session c
JOIN chat_issue_link l ON l.workspace_id=c.workspace_id AND l.chat_session_id=c.id
JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
LEFT JOIN issue_current_picture p ON p.workspace_id=i.workspace_id AND p.issue_id=i.id
LEFT JOIN LATERAL (SELECT q.status,q.runtime_id FROM agent_task_queue q WHERE q.issue_id=i.id
 ORDER BY (q.status IN ('running','dispatched','queued','waiting_local_directory')) DESC,q.created_at DESC,q.id DESC LIMIT 1) t ON true
LEFT JOIN agent_runtime rt ON rt.id=t.runtime_id AND rt.workspace_id=i.workspace_id
WHERE c.workspace_id=sqlc.arg(workspace_id) AND c.creator_id=sqlc.arg(owner_id) AND c.status='active';

-- name: SetChatResponseFormat :exec
UPDATE agent_task_queue SET context = jsonb_set(COALESCE(context, '{}'::jsonb), '{chat_response_format}', to_jsonb(sqlc.arg(format)::text))
WHERE id = sqlc.arg(task_id) AND chat_session_id = sqlc.arg(chat_session_id);
