CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_decision_key ON issue_workflow_decision (workspace_id, issue_id, id);
