-- name: ListUndeliveredChatWorkUpdates :many
-- Reconcile durable sources, not ephemeral websocket events. A stable message
-- id per link/source/event makes retries and multiple server instances safe.
-- No time watermark: a late commit can never fall behind the cursor. Batches
-- are bounded; source and chat-message primary keys support the anti-join.
WITH candidates AS (
  SELECT l.workspace_id, l.chat_session_id, l.issue_id, i.title, s.creator_id,
         e.kind, e.source_id, e.occurred_at, e.body, e.request_id, e.artifacts,
         md5('chat-work:' || l.chat_session_id::text || ':' || e.identity_kind || ':' || e.source_id::text || ':' || e.identity_suffix)::uuid AS message_id
  FROM chat_issue_link l
  JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
  JOIN chat_session s ON s.id=l.chat_session_id AND s.workspace_id=l.workspace_id
  JOIN workspace w ON w.id=l.workspace_id
  JOIN member m ON m.workspace_id=s.workspace_id AND m.user_id=s.creator_id
  CROSS JOIN LATERAL (
    SELECT 'started'::text AS kind, t.id AS source_id, t.started_at AS occurred_at,
           'The worker started.'::text AS body, NULL::uuid AS request_id,
           ''::text AS identity_suffix, '[]'::jsonb AS artifacts, 'started'::text AS identity_kind
    FROM agent_task_queue t WHERE t.issue_id=i.id AND t.chat_session_id IS NULL
      AND t.started_at >= l.created_at AND t.status='running'
    UNION ALL
    SELECT CASE WHEN COALESCE((SELECT st.category FROM issue_status st WHERE st.workspace_id=i.workspace_id AND st.key=i.status),i.status)='blocked'
                  AND NOT EXISTS (SELECT 1 FROM agent_task_queue newer WHERE newer.issue_id=i.id AND (newer.created_at,newer.id)>(t.created_at,t.id)) THEN 'stopped'
                WHEN delivered.content IS NOT NULL THEN 'result'
                WHEN t.status='completed' THEN 'result' ELSE 'stopped' END,
           t.id, t.completed_at,
           CASE WHEN delivered.content IS NOT NULL THEN
             CASE t.status
               WHEN 'completed' THEN 'The worker left this report.'
               WHEN 'cancelled' THEN 'The worker left this report before the run was stopped.'
               ELSE 'The worker left this report before the run ended with an error.'
             END || E'\n\n' || delivered.content ||
             CASE WHEN COALESCE(delivered.attachments,'') <> '' THEN E'\n\nAttachments:\n' || delivered.attachments ELSE '' END
           WHEN t.status='completed' THEN 'The worker finished this run. The issue is not automatically approved or complete.' || CASE WHEN COALESCE(t.result->>'output','') <> '' THEN E'\n\n' || (t.result->>'output') ELSE '' END
           WHEN t.status='cancelled' THEN 'This run was stopped.' ELSE 'The worker could not finish this run.' END,
           NULL::uuid,
           CASE WHEN delivered.content IS NOT NULL THEN 'delivery-preview-v2' ELSE '' END,
           COALESCE(delivered.artifacts, '[]'::jsonb),
           -- Preserve delivery identity when a blocked report changes its display
           -- kind. A replay must not create a second historical update.
           CASE WHEN delivered.content IS NOT NULL OR t.status='completed' THEN 'result' ELSE 'stopped' END
    FROM agent_task_queue t
    LEFT JOIN LATERAL (
      SELECT c.content,
             string_agg(
               CASE
                 WHEN lower(split_part(COALESCE(a.content_type,''),';',1))='text/html'
                   OR lower(COALESCE(a.filename,'')) ~ '\.html?$'
                 THEN format('- [Open delivered preview](/%s/attachments/%s/preview)',w.slug,a.id)
                 WHEN lower(split_part(COALESCE(a.content_type,''),';',1)) LIKE 'image/%'
                 THEN format('- [Open delivered image](/api/attachments/%s/download)',a.id)
                 ELSE format('- [Download delivered attachment](/api/attachments/%s/download)',a.id)
               END,
               E'\n' ORDER BY a.created_at,a.id)
               FILTER (WHERE a.id IS NOT NULL) AS attachments,
             jsonb_agg(
               jsonb_build_object(
                 'id', a.id::text,
                 'label', COALESCE(NULLIF(a.filename,''),'Delivered file'),
                 'kind', CASE
                   WHEN lower(split_part(COALESCE(a.content_type,''),';',1))='text/html'
                     OR lower(COALESCE(a.filename,'')) ~ '\.html?$' THEN 'page'
                   WHEN lower(split_part(COALESCE(a.content_type,''),';',1)) LIKE 'image/%' THEN 'image'
                   ELSE 'file'
                 END,
                 'url', CASE
                   WHEN lower(split_part(COALESCE(a.content_type,''),';',1))='text/html'
                     OR lower(COALESCE(a.filename,'')) ~ '\.html?$'
                     THEN format('/%s/attachments/%s/preview',w.slug,a.id)
                   ELSE format('/api/attachments/%s/download',a.id)
                 END,
                 'original', true
               ) ORDER BY a.created_at,a.id
             ) FILTER (WHERE a.id IS NOT NULL) AS artifacts
      FROM comment c
      LEFT JOIN attachment a ON a.comment_id=c.id AND a.workspace_id=l.workspace_id
      WHERE c.source_task_id=t.id AND c.issue_id=i.id AND c.workspace_id=l.workspace_id
        AND c.author_type='agent' AND c.type='comment'
      GROUP BY c.id,c.content,c.created_at
      ORDER BY c.created_at DESC,c.id DESC
      LIMIT 1
    ) delivered ON true
    WHERE t.issue_id=i.id AND t.chat_session_id IS NULL
      AND t.completed_at >= l.created_at AND t.status IN ('completed','failed','cancelled')
      -- A question is the meaningful result of that run; do not emit a second
      -- misleading 'result ready' entry for the same question-producing task.
      AND NOT EXISTS (SELECT 1 FROM inbox_item qb WHERE qb.workspace_id=l.workspace_id
        AND qb.issue_id=i.id AND qb.recipient_type='member' AND qb.recipient_id=s.creator_id
        AND qb.details->>'source_task_id'=t.id::text
        AND qb.details->>'question_scope'='ordinary_task')
    UNION ALL
    SELECT 'question', b.id, b.created_at,
           COALESCE(NULLIF(b.details->>'question',''), b.title), b.id, ''::text, '[]'::jsonb, 'question'::text
    FROM inbox_item b WHERE b.workspace_id=l.workspace_id AND b.issue_id=i.id
      AND b.recipient_type='member' AND b.recipient_id=s.creator_id
      AND NOT b.archived AND b.details->>'intake_state' IS DISTINCT FROM 'continued'
      AND (b.type='workflow_owner_decision' OR (b.type='chat_intake' AND b.details->>'question_scope'='ordinary_task'))
      -- Same current-attempt eligibility as ListOpenOrdinaryTaskQuestions.
      AND (b.details->>'question_scope' IS DISTINCT FROM 'ordinary_task' OR EXISTS (
        SELECT 1 FROM agent_task_queue source JOIN agent a ON a.id=source.agent_id AND a.workspace_id=i.workspace_id
        WHERE source.id::text=b.details->>'source_task_id' AND source.issue_id=i.id
          AND source.status IN ('running','completed') AND i.assignee_type='agent' AND i.assignee_id=source.agent_id
          AND source.accountable_user_id=b.recipient_id AND b.details->>'intake_state'='pending'
          AND NOT EXISTS (SELECT 1 FROM agent_task_queue newer WHERE newer.issue_id=i.id AND (newer.created_at,newer.id)>(source.created_at,source.id))
          AND NOT EXISTS (SELECT 1 FROM issue_workflow_plan plan WHERE plan.issue_id=i.id AND plan.workspace_id=i.workspace_id)
          AND NOT EXISTS (SELECT 1 FROM issue_current_picture picture WHERE picture.issue_id=i.id AND picture.workspace_id=i.workspace_id)
      ))
  ) e
  WHERE s.status='active'
    AND NOT EXISTS (SELECT 1 FROM channel_chat_session_binding b WHERE b.chat_session_id=s.id)
)
SELECT c.* FROM candidates c
WHERE NOT EXISTS (SELECT 1 FROM chat_message m WHERE m.id=c.message_id)
ORDER BY c.occurred_at,c.message_id LIMIT 100;

-- name: DeliverChatWorkUpdate :one
-- Revalidate the relationship at write time. Removed links, deleted issues,
-- removed members and archived conversations cannot receive queued updates.
INSERT INTO chat_message (id,chat_session_id,role,content,message_kind,work_issue_id,work_update)
SELECT sqlc.arg(id),s.id,'assistant',sqlc.arg(content),'work_update',l.issue_id,sqlc.arg(work_update)::jsonb
FROM chat_session s
JOIN chat_issue_link l ON l.chat_session_id=s.id AND l.workspace_id=s.workspace_id
JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
JOIN member m ON m.workspace_id=s.workspace_id AND m.user_id=s.creator_id
WHERE s.id=sqlc.arg(chat_session_id) AND s.workspace_id=sqlc.arg(workspace_id)
  AND s.creator_id=sqlc.arg(owner_id)
  AND l.issue_id=sqlc.arg(issue_id) AND s.status='active'
  AND NOT EXISTS (SELECT 1 FROM channel_chat_session_binding b WHERE b.chat_session_id=s.id)
ON CONFLICT (id) DO NOTHING RETURNING *;

-- name: ListRecentChatWorkUpdates :many
SELECT m.* FROM chat_message m
JOIN chat_issue_link l ON l.chat_session_id=m.chat_session_id AND l.issue_id=m.work_issue_id
JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
WHERE l.workspace_id=sqlc.arg(workspace_id) AND m.chat_session_id=sqlc.arg(chat_session_id)
  AND m.message_kind='work_update'
ORDER BY m.created_at DESC,m.id DESC LIMIT 30;

-- name: ListUnseenChatWorkUpdates :many
-- Read state belongs to delivered messages, never to the source event's clock.
-- The extra row exposes truncation; late deliveries cannot fall behind a cursor.
SELECT m.* FROM chat_message m
JOIN chat_session s ON s.id=m.chat_session_id
JOIN member owner ON owner.workspace_id=s.workspace_id AND owner.user_id=s.creator_id
JOIN chat_issue_link l ON l.chat_session_id=s.id AND l.issue_id=m.work_issue_id AND l.workspace_id=s.workspace_id
JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
WHERE s.workspace_id=sqlc.arg(workspace_id) AND s.id=sqlc.arg(chat_session_id)
  AND m.message_kind='work_update' AND m.work_seen_at IS NULL
ORDER BY m.created_at,m.id LIMIT 101;

-- name: AcknowledgeChatWorkUpdates :exec
-- Only acknowledge the exact set the owner saw. Never advances past a new row
-- arriving while the client was reviewing an older snapshot.
UPDATE chat_message m SET work_seen_at=now()
FROM chat_session s
JOIN member owner ON owner.workspace_id=s.workspace_id AND owner.user_id=s.creator_id
WHERE m.chat_session_id=s.id AND s.id=sqlc.arg(chat_session_id)
  AND s.workspace_id=sqlc.arg(workspace_id) AND s.creator_id=sqlc.arg(owner_id)
  AND m.message_kind='work_update' AND m.work_seen_at IS NULL
  AND m.id=ANY(sqlc.arg(message_ids)::uuid[])
  AND EXISTS (SELECT 1 FROM chat_issue_link l JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
    WHERE l.chat_session_id=s.id AND l.workspace_id=s.workspace_id AND l.issue_id=m.work_issue_id);
