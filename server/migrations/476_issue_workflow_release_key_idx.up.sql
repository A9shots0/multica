CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_release_key ON issue_workflow_release (workspace_id, issue_id, id);
