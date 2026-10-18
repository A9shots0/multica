CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_concern_head_key ON issue_workflow_concern_head (workspace_id, issue_id, concern_type);
