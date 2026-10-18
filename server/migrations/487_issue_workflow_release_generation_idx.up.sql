CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_release_generation ON issue_workflow_release (workspace_id, target_key, target_generation);
