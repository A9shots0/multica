CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_command_issue_idempotency ON issue_workflow_command (workspace_id, issue_id, command_key);
