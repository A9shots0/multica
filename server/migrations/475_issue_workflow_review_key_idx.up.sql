CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_review_key ON issue_workflow_review (workspace_id, issue_id, id);
