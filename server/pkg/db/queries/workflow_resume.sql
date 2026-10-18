-- Fresh-session continuity is selected only from the claimed task's exact
-- lineage. It never falls back to "latest task for this agent/issue", which
-- could cross parallel work. Two rows are enough for the caller to detect and
-- fail closed on ambiguity.
-- name: ListFreshWorkflowResumeSnapshotCandidates :many
WITH target AS (
    SELECT task.*
    FROM agent_task_queue AS task
    WHERE task.id = @target_task_id
      AND task.issue_id IS NOT NULL
      AND task.force_fresh_session
      AND task.workflow_action_id IS NULL
      AND task.workflow_operation_id IS NULL
      AND task.workflow_branch_key IS NULL
), lineage AS (
    SELECT DISTINCT source_task_id
    FROM target,
    LATERAL unnest(ARRAY[
        target.id,
        target.parent_task_id,
        target.retry_of_task_id,
        target.rerun_of_task_id,
        target.continuation_of_task_id
    ]::uuid[]) AS lineage_value(source_task_id)
    WHERE source_task_id IS NOT NULL
)
SELECT snapshot.*, picture.revision AS current_picture_revision,
       receipt.checkpoint AS continuation_checkpoint,
       receipt.source_step AS continuation_source_step,
       receipt.checkpoint_sha256 AS continuation_checkpoint_sha256
FROM target
JOIN issue ON issue.id = target.issue_id
JOIN lineage ON TRUE
JOIN issue_workflow_execution_checkpoint_snapshot AS snapshot
  ON snapshot.workspace_id = issue.workspace_id
 AND snapshot.issue_id = issue.id
 AND snapshot.task_id = lineage.source_task_id
 AND snapshot.actor_id = target.agent_id
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = snapshot.workspace_id
 AND execution.issue_id = snapshot.issue_id
 AND execution.id = snapshot.execution_id
 AND execution.revision = snapshot.execution_revision
 AND execution.superseded_at IS NULL
JOIN agent_task_continuation_receipt AS receipt
  ON receipt.workspace_id = snapshot.workspace_id
 AND receipt.issue_id = snapshot.issue_id
 AND receipt.source_task_id = snapshot.task_id
 AND receipt.execution_id = snapshot.execution_id
 AND receipt.execution_revision = snapshot.execution_revision
JOIN issue_current_picture AS picture
  ON picture.workspace_id = snapshot.workspace_id
 AND picture.issue_id = snapshot.issue_id
 AND picture.scope_revision = snapshot.scope_revision
WHERE octet_length(receipt.checkpoint::text) <= 4096
ORDER BY snapshot.execution_revision DESC, snapshot.execution_id ASC
LIMIT 2;
