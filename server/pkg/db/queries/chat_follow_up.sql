-- name: ListChatFollowUpSessions :many
SELECT DISTINCT s.* FROM chat_session s
JOIN agent a ON a.id=s.agent_id AND a.workspace_id=s.workspace_id AND a.archived_at IS NULL
JOIN agent_runtime rt ON rt.id=a.runtime_id AND rt.workspace_id=s.workspace_id
JOIN member owner ON owner.workspace_id=s.workspace_id AND owner.user_id=s.creator_id
WHERE s.status='active' AND rt.provider='codex'
 AND rt.metadata->'capabilities' ? 'chat-follow-up-v1'
 AND NOT EXISTS (SELECT 1 FROM channel_chat_session_binding b WHERE b.chat_session_id=s.id)
 AND EXISTS (SELECT 1 FROM chat_message m WHERE m.chat_session_id=s.id AND m.message_kind='work_update'
   AND m.work_follow_up_eligible AND m.work_follow_up_task_id IS NULL AND m.work_update->>'kind' IN ('result','stopped','question')
   AND m.created_at < now()-interval '10 seconds'
   -- Historical updates remain visible, but an old cancellation/result must
   -- not generate an approval request after correction/review work replaced it.
   AND NOT EXISTS (SELECT 1 FROM agent_task_queue prior
     JOIN agent_task_queue newer ON newer.issue_id=prior.issue_id
       AND (newer.created_at,newer.id) > (prior.created_at,prior.id)
     WHERE prior.issue_id=m.work_issue_id AND prior.id::text=m.work_update->>'source_id'
       AND m.work_update->>'kind' IN ('result','stopped'))
   AND EXISTS (SELECT 1 FROM chat_issue_link l JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
     WHERE l.chat_session_id=s.id AND l.workspace_id=s.workspace_id AND l.issue_id=m.work_issue_id))
 AND NOT EXISTS (SELECT 1 FROM agent_task_queue t WHERE t.chat_session_id=s.id AND t.status IN ('queued','deferred','dispatched','running','waiting_local_directory'))
LIMIT 20;

-- name: ListPendingChatFollowUps :many
SELECT m.* FROM chat_message m
JOIN chat_session s ON s.id=m.chat_session_id AND s.status='active'
JOIN member owner ON owner.workspace_id=s.workspace_id AND owner.user_id=s.creator_id
JOIN chat_issue_link l ON l.chat_session_id=s.id AND l.workspace_id=s.workspace_id AND l.issue_id=m.work_issue_id
JOIN issue i ON i.id=l.issue_id AND i.workspace_id=l.workspace_id
WHERE s.id=sqlc.arg(chat_session_id) AND m.message_kind='work_update' AND m.work_follow_up_eligible
 AND m.work_follow_up_task_id IS NULL AND m.work_update->>'kind' IN ('result','stopped','question')
 AND m.created_at < now()-interval '10 seconds'
 AND NOT EXISTS (SELECT 1 FROM agent_task_queue prior
   JOIN agent_task_queue newer ON newer.issue_id=prior.issue_id
     AND (newer.created_at,newer.id) > (prior.created_at,prior.id)
   WHERE prior.issue_id=m.work_issue_id AND prior.id::text=m.work_update->>'source_id'
     AND m.work_update->>'kind' IN ('result','stopped'))
 AND NOT EXISTS (SELECT 1 FROM agent_task_queue t WHERE t.chat_session_id=s.id AND t.status IN ('queued','deferred','dispatched','running','waiting_local_directory'))
ORDER BY m.created_at,m.id LIMIT 30;

-- name: SetChatFollowUpContext :one
UPDATE agent_task_queue SET context=jsonb_build_object('chat_follow_up',sqlc.arg(evidence)::jsonb),max_attempts=1
WHERE id=sqlc.arg(task_id) RETURNING *;

-- name: SealChatFollowUps :exec
UPDATE chat_message SET work_follow_up_task_id=sqlc.arg(task_id)
WHERE id=ANY(sqlc.arg(message_ids)::uuid[]) AND chat_session_id=sqlc.arg(chat_session_id) AND work_follow_up_task_id IS NULL;
