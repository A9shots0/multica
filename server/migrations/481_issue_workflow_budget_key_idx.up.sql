CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_budget_key ON issue_workflow_budget (workspace_id, issue_id, id);
