CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_authority_revision ON issue_workflow_authority (workspace_id, issue_id, revision);
