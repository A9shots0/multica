CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_candidate_key ON issue_workflow_candidate (workspace_id, issue_id, id);
