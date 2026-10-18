-- name: PutAgentTaskContinuationReceipt :one
INSERT INTO agent_task_continuation_receipt (
    id, workspace_id, issue_id, root_task_id, source_task_id,
    successor_task_id, source_step, next_step, max_packages,
    checkpoint, checkpoint_sha256, execution_id, execution_revision
) VALUES (
    @id, @workspace_id, @issue_id, @root_task_id, @source_task_id,
    @successor_task_id, @source_step, @next_step, @max_packages,
    @checkpoint, @checkpoint_sha256, @execution_id, @execution_revision
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetAgentTaskContinuationReceiptBySource :one
SELECT * FROM agent_task_continuation_receipt
WHERE source_task_id = @source_task_id;

