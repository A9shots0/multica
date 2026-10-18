-- name: CreateAIFailureInvestigation :one
INSERT INTO ai_failure_investigation (
    id, workspace_id, issue_id, trigger_kind, trigger_id,
    source_task_id, source_agent_id, accountable_user_id, summary, research_scope
) VALUES (
    @id, @workspace_id, @issue_id, @trigger_kind, @trigger_id,
    sqlc.narg(source_task_id), sqlc.narg(source_agent_id), sqlc.narg(accountable_user_id), @summary, @research_scope
)
ON CONFLICT (workspace_id, trigger_kind, trigger_id) DO UPDATE
SET updated_at = ai_failure_investigation.updated_at
RETURNING *;

-- name: SetAIFailureInvestigationComment :one
UPDATE ai_failure_investigation
SET trigger_comment_id = @trigger_comment_id, updated_at = now()
WHERE id = @id AND trigger_comment_id IS NULL
RETURNING *;

-- name: SetAIFailureInvestigationDispatch :one
UPDATE ai_failure_investigation
SET investigator_agent_id = @investigator_agent_id,
    investigator_task_id = @investigator_task_id,
    status = 'running', dispatched_at = now(), updated_at = now()
WHERE id = @id AND status IN ('queued', 'awaiting_investigator')
RETURNING *;

-- name: SetAIFailureInvestigationAwaiting :exec
UPDATE ai_failure_investigation
SET status = 'awaiting_investigator', updated_at = now()
WHERE id = @id AND status = 'queued';

-- name: CompleteAIFailureInvestigationByTask :execrows
UPDATE ai_failure_investigation
SET status = 'completed', findings = @findings, completed_at = now(), updated_at = now()
WHERE investigator_task_id = @investigator_task_id AND status = 'running';

-- name: FailAIFailureInvestigationByTask :execrows
UPDATE ai_failure_investigation
SET status = 'investigator_failed', findings = NULL, completed_at = now(), updated_at = now()
WHERE investigator_task_id = @investigator_task_id AND status = 'running';

-- name: GetAIFailureInvestigationByTask :one
SELECT * FROM ai_failure_investigation WHERE investigator_task_id = @investigator_task_id;

-- name: ListDispatchableAIFailureInvestigationsForIssue :many
SELECT * FROM ai_failure_investigation
WHERE issue_id = @issue_id
  AND status IN ('queued', 'awaiting_investigator')
ORDER BY created_at, id;

-- name: ListAIFailureInvestigationsByIssue :many
SELECT * FROM ai_failure_investigation
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY created_at, id;
