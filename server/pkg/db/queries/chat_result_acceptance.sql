-- name: GetChatAcceptableResult :one
-- Ordinary owner-reviewed work only. Workflow-controlled acceptance has its
-- own exact decision/evidence gates and must never fall through this route.
SELECT t.* FROM agent_task_queue t
JOIN issue i ON i.id=t.issue_id
WHERE i.id=sqlc.arg(issue_id) AND i.workspace_id=sqlc.arg(workspace_id)
 AND i.status='in_review' AND i.assignee_type='agent' AND i.assignee_id=t.agent_id
 AND t.accountable_user_id=sqlc.arg(owner_id) AND t.status='completed'
 AND COALESCE(t.result->>'output','')<>'' AND t.chat_session_id IS NULL
 AND NOT EXISTS (SELECT 1 FROM agent_task_queue n WHERE n.issue_id=i.id AND (n.created_at,n.id)>(t.created_at,t.id))
 AND NOT EXISTS (SELECT 1 FROM agent_task_queue n WHERE n.issue_id=i.id AND n.status IN ('queued','deferred','dispatched','running','waiting_local_directory'))
 AND NOT EXISTS (SELECT 1 FROM issue_workflow_plan p WHERE p.issue_id=i.id)
 AND NOT EXISTS (SELECT 1 FROM issue_current_picture p WHERE p.issue_id=i.id)
 AND NOT EXISTS (SELECT 1 FROM issue_workflow_concern_head h WHERE h.issue_id=i.id)
 AND NOT EXISTS (SELECT 1 FROM inbox_item b WHERE b.issue_id=i.id
   AND (b.type='workflow_owner_decision' OR (b.type='chat_intake' AND b.details->>'question_scope'='ordinary_task'))
   AND NOT b.archived AND b.details->>'intake_state' IS DISTINCT FROM 'continued')
ORDER BY t.created_at DESC,t.id DESC LIMIT 1;
