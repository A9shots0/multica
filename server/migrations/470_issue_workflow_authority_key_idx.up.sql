CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_authority_key ON issue_workflow_authority (workspace_id, issue_id, id);
