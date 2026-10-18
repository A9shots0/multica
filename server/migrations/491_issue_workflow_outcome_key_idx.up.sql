CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_outcome_key ON issue_workflow_outcome (workspace_id, issue_id, id);
