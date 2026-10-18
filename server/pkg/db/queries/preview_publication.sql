-- name: LockPreviewPublicationAuthority :one
SELECT task.id AS agent_run_id,
       task.agent_id,
       receipt.workspace_id,
       receipt.issue_id,
       (task.context->'preview'->>'candidate_id')::uuid AS candidate_id,
       (task.context->'preview'->>'candidate_revision')::bigint AS candidate_revision,
       (task.context->'preview'->>'source_revision')::text AS source_revision,
       runtime.daemon_id,
       task.runtime_id,
       COALESCE(NULLIF(task.durable_work_dir, ''), task.work_dir) AS worktree_path,
       task.branch_name AS branch
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id
 AND receipt.bound_task_id = task.id
 AND receipt.issue_id = task.issue_id
JOIN issue_workflow_candidate AS rejected
  ON rejected.workspace_id = receipt.workspace_id
 AND rejected.issue_id = receipt.issue_id
 AND rejected.id::text = task.context->>'candidate_id'
 AND rejected.revision = (task.context->>'candidate_revision')::bigint
JOIN issue_workflow_review AS failed_review
  ON failed_review.workspace_id = receipt.workspace_id
 AND failed_review.issue_id = receipt.issue_id
 AND failed_review.id::text = task.context->>'review_id'
 AND failed_review.candidate_id = rejected.id
 AND failed_review.candidate_revision = rejected.revision
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = receipt.workspace_id
 AND execution.issue_id = receipt.issue_id
 AND execution.id = receipt.operation_id
 AND execution.branch_key = task.workflow_branch_key
JOIN agent_runtime AS runtime
  ON runtime.id = task.runtime_id
 AND runtime.workspace_id = receipt.workspace_id
JOIN agent AS worker
  ON worker.id = task.agent_id
 AND worker.workspace_id = receipt.workspace_id
 AND worker.runtime_id = runtime.id
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = execution.workspace_id
 AND execution_head.issue_id = execution.issue_id
 AND execution_head.concern_type = 'execution'
 AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision
 AND execution_head.state = 'correction'
JOIN issue_workflow_concern_head AS review_head
  ON review_head.workspace_id = failed_review.workspace_id
 AND review_head.issue_id = failed_review.issue_id
 AND review_head.concern_type = 'review'
 AND review_head.source_record_id = failed_review.id
 AND review_head.source_record_revision = 1
 AND review_head.state = 'failed'
WHERE task.id = @task_id
  AND receipt.action_type = 'review_correction'
  AND receipt.state = 'settled'
  AND task.status IN ('dispatched', 'running', 'completed')
  AND runtime.status = 'online'
  AND runtime.daemon_id = @daemon_id
  AND worker.archived_at IS NULL
  AND failed_review.outcome = 'failed'
  AND failed_review.superseded_at IS NULL
  AND rejected.superseded_at IS NULL
  AND execution.state = 'correction'
  AND receipt.payload->>'branch_key' = task.workflow_branch_key
  AND receipt.payload->>'correction_worker_id' = task.agent_id::text
  AND receipt.payload->>'correction_candidate_revision' = '1'
  AND task.context->'preview'->>'kind' = 'static'
  AND task.context->'preview'->>'artifact_root' = '.multica/preview'
  AND task.context->'preview'->>'policy_id' = 'synthetic-or-redacted-read-only-v1'
  AND task.context->'preview'->>'candidate_revision' = '1'
  AND NULLIF(task.context->'preview'->>'source_revision', '') IS NOT NULL
  AND COALESCE(NULLIF(task.durable_work_dir, ''), NULLIF(task.work_dir, '')) IS NOT NULL
  AND NULLIF(task.branch_name, '') IS NOT NULL
FOR UPDATE OF task, receipt, rejected, failed_review, execution, runtime, worker, execution_head, review_head;

-- name: InsertPreviewPublication :execrows
INSERT INTO issue_preview_publication (
    workspace_id, issue_id, candidate_id, candidate_revision, agent_id,
    agent_run_id, source_revision, daemon_id, runtime_id, worktree_path,
    branch, preview_url, identity_url, data_policy, published_at, expires_at,
    publication
) VALUES (
    @workspace_id, @issue_id, @candidate_id, @candidate_revision, @agent_id,
    @agent_run_id, @source_revision, @daemon_id, @runtime_id, @worktree_path,
    @branch, @preview_url, @identity_url, @data_policy, @published_at,
    @expires_at, @publication
)
ON CONFLICT (workspace_id, issue_id, candidate_id, candidate_revision) DO NOTHING;

-- name: GetPreviewPublicationByIdentity :one
SELECT *
FROM issue_preview_publication
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND candidate_id = @candidate_id
  AND candidate_revision = @candidate_revision;

-- name: GetLatestPreviewPublicationForIssue :one
SELECT *
FROM issue_preview_publication
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY published_at DESC, candidate_id DESC
LIMIT 1;

-- name: PreviewPublicationIsEligible :one
SELECT EXISTS (
    SELECT 1
    FROM issue_preview_publication AS publication
    JOIN agent_task_queue AS task
      ON task.id = publication.agent_run_id
     AND task.agent_id = publication.agent_id
     AND task.issue_id = publication.issue_id
     AND task.runtime_id = publication.runtime_id
    JOIN issue_workflow_action_receipt AS receipt
      ON receipt.id = task.workflow_action_id
     AND receipt.bound_task_id = task.id
     AND receipt.workspace_id = publication.workspace_id
     AND receipt.issue_id = publication.issue_id
    JOIN agent_runtime AS runtime
      ON runtime.id = publication.runtime_id
     AND runtime.workspace_id = publication.workspace_id
     AND runtime.daemon_id = publication.daemon_id
    LEFT JOIN issue_workflow_candidate AS candidate
      ON candidate.workspace_id = publication.workspace_id
     AND candidate.issue_id = publication.issue_id
     AND candidate.id = publication.candidate_id
     AND candidate.revision = publication.candidate_revision
    LEFT JOIN issue_workflow_candidate AS rejected
      ON rejected.workspace_id = publication.workspace_id
     AND rejected.issue_id = publication.issue_id
     AND rejected.id::text = task.context->>'candidate_id'
     AND rejected.revision = (task.context->>'candidate_revision')::bigint
    WHERE publication.workspace_id = @workspace_id
      AND publication.issue_id = @issue_id
      AND publication.candidate_id = @candidate_id
      AND publication.candidate_revision = @candidate_revision
      AND receipt.action_type = 'review_correction'
      AND receipt.state = 'settled'
      AND task.context->'preview'->>'candidate_id' = publication.candidate_id::text
      AND task.context->'preview'->>'candidate_revision' = publication.candidate_revision::text
      AND task.context->'preview'->>'source_revision' = publication.source_revision
      AND task.context->'preview'->>'policy_id' = 'synthetic-or-redacted-read-only-v1'
      AND task.status IN ('dispatched', 'running', 'completed')
      AND runtime.status = 'online'
      AND (
        (candidate.id IS NULL AND rejected.superseded_at IS NULL)
        OR
        (candidate.id IS NOT NULL AND candidate.superseded_at IS NULL
         AND candidate.created_by = publication.agent_id)
      )
);
