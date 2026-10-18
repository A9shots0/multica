CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_command_idempotency ON issue_workflow_command (workspace_id, command_key);
