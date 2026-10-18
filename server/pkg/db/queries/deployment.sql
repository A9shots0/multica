-- name: CreateManualDeploymentRecord :one
-- Records "this was already deployed, outside Multica" without dispatching
-- any agent work — inserted straight into 'completed', so the daemon claim
-- path never sees it and nothing actually runs. Same [MULTICA_DEPLOY_TASK]
-- handoff_note prefix as a real dispatched deploy so it's picked up by
-- ListRecentDeployTasks identically; the note text itself says "recorded
-- manually" so it reads honestly in history. See MarkIssueDeployed in
-- issue_deploy.go for why this exists: an owner clicking "Approve & Deploy"
-- on something an outside agent already shipped would otherwise dispatch a
-- real, unnecessary redeploy with no way to tell Multica it wasn't needed.
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, status, handoff_note, completed_at, initiator_user_id
) VALUES (
    $1, $2, $3, 'completed', $4, now(), $5
)
RETURNING id;

-- name: ListRecentDeployTasks :many
-- Real deploy history for the Deployments page's "Release history" and
-- "currently live per project" sections. A "deploy task" is identified by
-- the [MULTICA_DEPLOY_TASK] sentinel TriggerIssueDeploy stamps on every
-- handoff note it writes (see issue_deploy.go) — cheaper and just as
-- reliable as a dedicated column, and needs no migration. Only terminal
-- tasks (completed/failed/cancelled) are meaningful history; an in-flight
-- one is still shown live via GetIssueDeployTaskStatus polling.
SELECT
    t.id AS task_id,
    t.issue_id,
    t.status,
    t.started_at,
    t.completed_at,
    t.error,
    t.failure_reason,
    i.number AS issue_number,
    i.title AS issue_title,
    i.project_id,
    p.title AS project_title
FROM agent_task_queue t
JOIN issue i ON i.id = t.issue_id
LEFT JOIN project p ON p.id = i.project_id
WHERE i.workspace_id = $1
  AND t.handoff_note LIKE '[MULTICA_DEPLOY_TASK]%'
  AND t.status IN ('completed', 'failed', 'cancelled')
ORDER BY t.completed_at DESC NULLS LAST, t.created_at DESC
LIMIT $2;


-- name: FindActiveProjectDeploy :one
-- Project-level deploy lock: is there ALREADY a deploy in flight for this
-- project, on ANY issue, not just this one? TriggerIssueDeploy's existing
-- idx_one_pending_task_per_issue_agent_thread guard only stops the SAME
-- issue+agent from double-deploying — it does nothing to stop two different
-- issues on the same project (e.g. two "Multica Platform" issues) from
-- redeploying the same running service at the same time, which is the real
-- collision risk. sqlc.narg keeps a NULL project_id (an issue with no
-- project) from matching every other unassigned issue via `= NULL` — it
-- must never lock across unrelated issues that merely share "no project."
SELECT t.id AS task_id, i.id AS issue_id, i.number AS issue_number
FROM agent_task_queue t
JOIN issue i ON i.id = t.issue_id
WHERE i.workspace_id = $1
  AND i.project_id = sqlc.narg(project_id)::uuid
  AND t.handoff_note LIKE '[MULTICA_DEPLOY_TASK]%'
  AND t.status IN ('queued', 'dispatched', 'running')
ORDER BY t.created_at ASC
LIMIT 1;
