CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_action_idempotency ON issue_workflow_action_receipt (workspace_id, action_key);
