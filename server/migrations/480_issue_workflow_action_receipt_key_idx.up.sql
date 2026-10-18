CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_action_receipt_key ON issue_workflow_action_receipt (workspace_id, issue_id, id);
