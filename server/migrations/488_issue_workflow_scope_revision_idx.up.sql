CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_scope_revision ON issue_workflow_scope (workspace_id, issue_id, revision);
