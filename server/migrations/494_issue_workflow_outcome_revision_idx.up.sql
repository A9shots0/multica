CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_outcome_revision ON issue_workflow_outcome (workspace_id, issue_id, revision);
