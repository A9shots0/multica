-- name: GetIssueWorkflowPlan :one
SELECT * FROM issue_workflow_plan
WHERE issue_id = $1 AND workspace_id = $2;

-- name: CreateOwnerRuntimeInboxNotice :one
INSERT INTO inbox_item (id,workspace_id,recipient_type,recipient_id,type,severity,issue_id,title,body,actor_type,details)
VALUES (@id,@workspace_id,'member',@recipient_id,@type,@severity,@issue_id,@title,@body,'system',@details)
ON CONFLICT (id) DO NOTHING RETURNING *;

-- name: ListOwnerRuntimeCompletionRecoveryTasks :many
-- Only the latest completed route carrier can need post-commit repair. A newer
-- carrier, live worker, owner wait or terminal plan must never be superseded.
SELECT task.* FROM issue_workflow_plan plan
JOIN LATERAL (
 SELECT candidate.* FROM agent_task_queue candidate WHERE candidate.issue_id = plan.issue_id
 ORDER BY candidate.created_at DESC, candidate.id DESC LIMIT 1
) task ON true
WHERE plan.owner_runtime_route IS NOT NULL
 AND plan.current_state IN ('queued','working','correction','technical_review','deploying','production_verifying','rolling_back')
 AND task.status = 'completed' AND task.squad_id = plan.squad_id
 AND task.context->'owner_runtime_route'->>'directive_id' = plan.owner_runtime_route->>'directive_id'
 AND (sqlc.narg(task_id)::uuid IS NULL OR task.id = sqlc.narg(task_id)::uuid)
 AND NOT EXISTS (SELECT 1 FROM agent_task_queue active WHERE active.issue_id = plan.issue_id AND active.status IN ('queued','dispatched','running'))
ORDER BY task.completed_at, task.id LIMIT 100;

-- name: InsertOwnerRuntimeWorkflowPlan :one
INSERT INTO issue_workflow_plan (
 issue_id, workspace_id, squad_id, implementer_id, reviewer_id,
 release_agent_id, verifier_id, requires_owner, requires_deployment,
 current_state, correction_count, max_corrections, owner_runtime_route
) VALUES (
 @issue_id, @workspace_id, @squad_id, @implementer_id, @reviewer_id,
 @release_agent_id, @verifier_id, false, true, 'queued', 0, 1, @owner_runtime_route
) RETURNING *;

-- name: LockOwnerRouteAgent :one
SELECT agent.* FROM agent JOIN squad_member member
 ON member.member_id = agent.id AND member.member_type = 'agent'
WHERE agent.id = @agent_id AND agent.workspace_id = @workspace_id
 AND member.squad_id = @squad_id AND agent.archived_at IS NULL
FOR SHARE OF agent, member;

-- name: LockOwnerRouteRuntime :one
SELECT runtime.* FROM agent_runtime runtime
WHERE runtime.id = @runtime_id AND runtime.workspace_id = @workspace_id
FOR SHARE;

-- name: HasOtherRunningOwnerWorkflowTask :one
SELECT EXISTS (SELECT 1 FROM agent_task_queue
 WHERE issue_id = @issue_id AND id <> @task_id AND status = 'running');

-- name: GetWorkflowEventCarrier :one
SELECT * FROM agent_task_queue WHERE issue_id = @issue_id
 AND trigger_evidence_kind = 'workflow_event' AND trigger_evidence_ref_id = @event_id
ORDER BY created_at, id LIMIT 1;

-- name: GetLatestWorkflowManifest :one
SELECT * FROM issue_workflow_event WHERE issue_id = @issue_id
 AND workspace_id = @workspace_id AND event_type = 'implementation_ready'
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: LockOwnerRuntimeSourceTask :one
SELECT * FROM agent_task_queue WHERE id = @task_id AND issue_id = @issue_id
 AND status='running' FOR SHARE;

-- name: RetireStoppedOwnerRuntimeQuestions :exec
UPDATE inbox_item SET archived=true, details=details || '{"owner_category":"history","required_action":"inspect_evidence"}'::jsonb
WHERE issue_id = @issue_id AND workspace_id = @workspace_id
 AND type='workflow_owner_decision' AND details ? 'workflow_event_id';

-- name: ListStalledWork :many
-- Detection only: this query must never lock, advance, enqueue, or complete
-- work. `stalled_since` is the latest durable state change the database can
-- observe for the issue, its controller projection, or one of its runs.
SELECT
  issue.id AS issue_id,
  issue.workspace_id,
  issue.number AS issue_number,
  issue.status AS issue_status,
  issue.parent_issue_id,
  CASE WHEN plan.issue_id IS NULL THEN false ELSE true END AS has_workflow_plan,
  plan.current_state AS workflow_state,
  GREATEST(
    issue.updated_at,
    COALESCE(plan.updated_at, '-infinity'::timestamptz),
    COALESCE(picture.updated_at, '-infinity'::timestamptz),
    COALESCE(latest_task.observed_at, '-infinity'::timestamptz)
  )::timestamptz AS stalled_since,
  count(*) OVER () AS total_count
FROM issue
LEFT JOIN issue_status status_entry
  ON status_entry.workspace_id = issue.workspace_id
 AND status_entry.key = issue.status
 AND status_entry.archived_at IS NULL
LEFT JOIN issue_workflow_plan plan
  ON plan.issue_id = issue.id
 AND plan.workspace_id = issue.workspace_id
LEFT JOIN issue_current_picture picture
  ON picture.issue_id = issue.id
 AND picture.workspace_id = issue.workspace_id
LEFT JOIN LATERAL (
  SELECT max(COALESCE(task.completed_at, task.started_at, task.dispatched_at, task.created_at)) AS observed_at
  FROM agent_task_queue task
  WHERE task.issue_id = issue.id
) latest_task ON true
WHERE COALESCE(status_entry.category, issue.status) NOT IN ('done', 'cancelled')
  AND NOT EXISTS (
    SELECT 1
    FROM agent_task_queue running_task
    WHERE running_task.issue_id = issue.id
      AND running_task.status = 'running'
  )
  AND NOT (
    COALESCE(plan.current_state IN (
      'received', 'queued', 'working', 'technical_review', 'correction',
      'deploying', 'production_verifying', 'rolling_back'
    ), false)
    OR COALESCE((
      jsonb_typeof(picture.next_automatic_action) = 'object'
      AND picture.next_automatic_action <> '{}'::jsonb
    ), false)
    OR EXISTS (
      SELECT 1
      FROM issue_workflow_action_receipt action
      WHERE action.issue_id = issue.id
        AND action.workspace_id = issue.workspace_id
        AND action.state IN ('admitted', 'retry_wait', 'reconciling', 'leased')
    )
  )
  AND GREATEST(
    issue.updated_at,
    COALESCE(plan.updated_at, '-infinity'::timestamptz),
    COALESCE(picture.updated_at, '-infinity'::timestamptz),
    COALESCE(latest_task.observed_at, '-infinity'::timestamptz)
  ) <= @cutoff
ORDER BY stalled_since ASC, issue.id ASC
LIMIT @row_limit;

-- Every non-terminal squad issue must be checked by the controller reconciler.
-- Keep rows whose template is missing visible even when an older plan row
-- exists: deleting the template must not silently disable recovery. A plan in
-- an automatic state with no active task is also unfinished controller work;
-- revisiting it repairs the exact gap where a plan row existed but every
-- carrier had already exited.
-- name: ListIssuesMissingWorkflowPlan :many
SELECT issue.*
FROM issue
JOIN issue_status status_entry
  ON status_entry.workspace_id = issue.workspace_id
 AND status_entry.key = issue.status
 AND status_entry.archived_at IS NULL
LEFT JOIN issue_workflow_plan plan
  ON plan.issue_id = issue.id
 AND plan.workspace_id = issue.workspace_id
LEFT JOIN squad_workflow_template template
  ON template.squad_id = issue.assignee_id
 AND template.workspace_id = issue.workspace_id
LEFT JOIN inbox_item recovery
  ON recovery.issue_id = issue.id
 AND recovery.workspace_id = issue.workspace_id
 AND recovery.type = 'workflow_controller_unavailable'
 AND recovery.archived = false
WHERE issue.assignee_type = 'squad'
  AND plan.owner_runtime_route IS NULL
  AND status_entry.category NOT IN ('done', 'cancelled')
  AND (
    plan.issue_id IS NULL
    OR template.squad_id IS NULL
    OR recovery.issue_id IS NOT NULL
    OR (
      plan.current_state IN ('working', 'queued', 'correction')
      AND NOT EXISTS (
        SELECT 1 FROM agent_task_queue task
        WHERE task.issue_id = issue.id
          AND task.status IN ('queued', 'dispatched', 'running', 'waiting_local_directory')
      )
    )
  )
ORDER BY issue.created_at ASC
LIMIT @row_limit;

-- The recovery notice has a stable type and issue scope. Reading it before
-- writing keeps retries from creating duplicate Inbox rows.
-- name: GetActiveWorkflowControllerInbox :one
  SELECT * FROM inbox_item
  WHERE workspace_id = @workspace_id
    AND issue_id = @issue_id
    AND type = 'workflow_controller_unavailable'
  AND archived = false
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ArchiveWorkflowControllerInbox :execrows
UPDATE inbox_item
SET archived = true
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND type = 'workflow_controller_unavailable'
  AND archived = false;

-- name: GetSquadWorkflowTemplate :one
SELECT * FROM squad_workflow_template
WHERE squad_id = $1 AND workspace_id = $2;

-- name: LockWorkflowOwnerWorkspace :one
SELECT id FROM workspace WHERE id = $1 FOR KEY SHARE;

-- name: LockSquadWorkflowTemplateSquad :one
SELECT id FROM squad
WHERE id = @squad_id AND workspace_id = @workspace_id AND archived_at IS NULL
FOR KEY SHARE;

-- name: LockSquadWorkflowTemplateAgents :many
SELECT agent.id
FROM agent
JOIN squad_member
  ON squad_member.member_id = agent.id
 AND squad_member.member_type = 'agent'
WHERE agent.workspace_id = @workspace_id
  AND agent.archived_at IS NULL
  AND squad_member.squad_id = @squad_id
  AND agent.id = ANY(@agent_ids::uuid[])
ORDER BY agent.id
FOR KEY SHARE OF agent, squad_member;

-- name: UpsertSquadWorkflowTemplate :one
INSERT INTO squad_workflow_template (
    squad_id, workspace_id, implementer_id, reviewer_id,
    release_agent_id, verifier_id, requires_owner,
    requires_deployment, max_corrections
) VALUES (
    @squad_id, @workspace_id, @implementer_id, @reviewer_id,
    sqlc.narg('release_agent_id'), sqlc.narg('verifier_id'),
    @requires_owner, @requires_deployment, @max_corrections
)
ON CONFLICT (squad_id) DO UPDATE SET
    implementer_id = EXCLUDED.implementer_id,
    reviewer_id = EXCLUDED.reviewer_id,
    release_agent_id = EXCLUDED.release_agent_id,
    verifier_id = EXCLUDED.verifier_id,
    requires_owner = EXCLUDED.requires_owner,
    requires_deployment = EXCLUDED.requires_deployment,
    max_corrections = EXCLUDED.max_corrections,
    updated_at = now()
WHERE squad_workflow_template.workspace_id = EXCLUDED.workspace_id
RETURNING *;

-- name: InstallIssueWorkflowPlanFromSquadTemplate :one
INSERT INTO issue_workflow_plan (
    issue_id, workspace_id, squad_id, implementer_id, reviewer_id,
    release_agent_id, verifier_id, requires_owner, requires_deployment,
    current_state, correction_count, max_corrections
)
SELECT
    @issue_id, template.workspace_id, template.squad_id,
    template.implementer_id, template.reviewer_id,
    template.release_agent_id, template.verifier_id,
    template.requires_owner, template.requires_deployment,
    'working', 0, template.max_corrections
FROM squad_workflow_template template
WHERE template.squad_id = @squad_id
  AND template.workspace_id = @workspace_id
  AND EXISTS (
      SELECT 1 FROM squad_member member
      JOIN agent agent_row ON agent_row.id = member.member_id
      WHERE member.squad_id = template.squad_id
        AND member.member_type = 'agent'
        AND member.member_id = template.implementer_id
        AND agent_row.archived_at IS NULL
  )
  AND EXISTS (
      SELECT 1 FROM squad_member member
      JOIN agent agent_row ON agent_row.id = member.member_id
      WHERE member.squad_id = template.squad_id
        AND member.member_type = 'agent'
        AND member.member_id = template.reviewer_id
        AND agent_row.archived_at IS NULL
  )
  AND (template.release_agent_id IS NULL OR EXISTS (
      SELECT 1 FROM squad_member member
      JOIN agent agent_row ON agent_row.id = member.member_id
      WHERE member.squad_id = template.squad_id
        AND member.member_type = 'agent'
        AND member.member_id = template.release_agent_id
        AND agent_row.archived_at IS NULL
  ))
  AND (template.verifier_id IS NULL OR EXISTS (
      SELECT 1 FROM squad_member member
      JOIN agent agent_row ON agent_row.id = member.member_id
      WHERE member.squad_id = template.squad_id
        AND member.member_type = 'agent'
        AND member.member_id = template.verifier_id
        AND agent_row.archived_at IS NULL
  ))
ON CONFLICT (issue_id) DO NOTHING
RETURNING *;

-- name: GetIssueWorkflowPlanForUpdate :one
SELECT * FROM issue_workflow_plan
WHERE issue_id = $1 AND workspace_id = $2
FOR UPDATE;

-- name: GetIssueWorkflowPlanForCompletion :one
-- Completion already owns the issue fence. Never wait on a legacy writer
-- that owns the plan and may next need that issue lock.
SELECT * FROM issue_workflow_plan
WHERE issue_id = $1 AND workspace_id = $2
FOR UPDATE NOWAIT;

-- name: HasUnansweredLegacyWorkflowDecision :one
SELECT EXISTS (
    SELECT 1 FROM issue_workflow_event request
    WHERE request.workspace_id = @workspace_id
      AND request.issue_id = @issue_id
      AND request.decision->>'notify_owner' = 'true'
      AND NOT EXISTS (
          SELECT 1 FROM issue_workflow_event answer
          WHERE answer.workspace_id = request.workspace_id
            AND answer.issue_id = request.issue_id
            AND answer.event_type = 'owner_decision_recorded'
            AND answer.payload->>'request_event_id' = request.id::text
      )
);

-- name: UpsertIssueWorkflowPlan :one
INSERT INTO issue_workflow_plan (
    issue_id, workspace_id, squad_id, implementer_id, reviewer_id,
    release_agent_id, verifier_id, requires_owner, requires_deployment,
    current_state, suspended_state, suspended_agent_id,
    correction_count, max_corrections
) VALUES (
    @issue_id, @workspace_id, @squad_id, @implementer_id, @reviewer_id,
    sqlc.narg('release_agent_id'), sqlc.narg('verifier_id'),
    @requires_owner, @requires_deployment, @current_state,
    sqlc.narg('suspended_state'), sqlc.narg('suspended_agent_id'), 0, 1
)
ON CONFLICT (issue_id) DO UPDATE SET
    squad_id = EXCLUDED.squad_id,
    implementer_id = EXCLUDED.implementer_id,
    reviewer_id = EXCLUDED.reviewer_id,
    release_agent_id = EXCLUDED.release_agent_id,
    verifier_id = EXCLUDED.verifier_id,
    requires_owner = EXCLUDED.requires_owner,
    requires_deployment = EXCLUDED.requires_deployment,
    current_state = EXCLUDED.current_state,
    suspended_state = EXCLUDED.suspended_state,
    suspended_agent_id = EXCLUDED.suspended_agent_id,
    correction_count = 0,
    max_corrections = 1,
    revision = issue_workflow_plan.revision + 1,
    updated_at = now()
WHERE issue_workflow_plan.workspace_id = EXCLUDED.workspace_id
  AND issue_workflow_plan.owner_runtime_route IS NULL
RETURNING *;

-- name: UpdateIssueWorkflowPlanState :one
UPDATE issue_workflow_plan SET
    current_state = @current_state,
    suspended_state = sqlc.narg('suspended_state'),
    suspended_agent_id = sqlc.narg('suspended_agent_id'),
    correction_count = @correction_count,
    revision = revision + 1,
    updated_at = now()
WHERE issue_id = @issue_id
  AND workspace_id = @workspace_id
  AND revision = @expected_revision
RETURNING *;

-- name: GetIssueWorkflowEventBySource :one
SELECT * FROM issue_workflow_event
WHERE source_task_id = @source_task_id AND event_type = @event_type;

-- name: GetIssueWorkflowEvent :one
SELECT * FROM issue_workflow_event
WHERE id = @id AND issue_id = @issue_id AND workspace_id = @workspace_id;

-- name: CreateIssueWorkflowEvent :one
INSERT INTO issue_workflow_event (
    id, workspace_id, issue_id, source_task_id, actor_id,
    event_type, reason, payload, decision
) VALUES (
    @id, @workspace_id, @issue_id, @source_task_id, @actor_id,
    @event_type, @reason, @payload, @decision
)
RETURNING *;

-- name: ListIssueWorkflowEvents :many
SELECT * FROM issue_workflow_event
WHERE issue_id = @issue_id AND workspace_id = @workspace_id
ORDER BY created_at ASC, id ASC;

-- name: HasAdvancingIssueWorkflowEventForTask :one
SELECT EXISTS (
    SELECT 1 FROM issue_workflow_event
    WHERE source_task_id = @source_task_id
      -- Event admission already rejects ignored transitions. Once accepted,
      -- its decision is authoritative even if successor enqueue fails after
      -- commit; replaying the same event reconciles that dispatch idempotently.
      AND COALESCE(decision->>'next', '') <> ''
) AS exists;

-- name: ListUndispatchedIssueWorkflowEventsForTask :many
-- Accepted events of a source task that name a successor agent but never got
-- their dispatched task. The event admission commits before the successor is
-- enqueued, so an enqueue failure after commit leaves exactly this shape.
-- review_failed is deliberately held for an AI failure investigation before
-- another implementation attempt, so it is not a missing dispatch.
SELECT event.*
FROM issue_workflow_event event
WHERE event.source_task_id = @source_task_id
	AND event.event_type <> 'review_failed'
  AND COALESCE(event.decision->>'enqueue_agent_id', '') <> ''
  AND NOT EXISTS (
    SELECT 1 FROM agent_task_queue task
    WHERE task.trigger_evidence_kind = 'workflow_event'
      AND task.trigger_evidence_ref_id = event.id
  )
ORDER BY event.created_at, event.id;

-- name: GetIssueWorkflowPlanByTask :one
SELECT plan.*
FROM issue_workflow_plan plan
JOIN agent_task_queue task
  ON task.issue_id = plan.issue_id
 AND task.squad_id = plan.squad_id
WHERE task.id = @task_id;

-- name: LockIssueWorkflowPlanForTaskStart :one
-- The plan row is the workflow-wide provider-start mutex. Every start path,
-- including generic recovery rows, converges here before a provider process is
-- allowed to launch.
SELECT plan.*
FROM issue_workflow_plan plan
JOIN agent_task_queue task
  ON task.issue_id = plan.issue_id
 AND task.squad_id = plan.squad_id
WHERE task.id = @task_id
  AND task.status IN ('dispatched', 'waiting_local_directory')
FOR UPDATE OF plan;

-- name: CountStartedIssueWorkflowTasks :one
SELECT count(*)::integer AS count
FROM agent_task_queue
WHERE issue_id = @issue_id
  AND squad_id = @squad_id
  AND started_at IS NOT NULL;
