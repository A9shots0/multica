CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_target_head_key ON issue_workflow_target_head (workspace_id, target_key);
