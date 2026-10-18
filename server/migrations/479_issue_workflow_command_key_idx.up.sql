CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_command_key ON issue_workflow_command (workspace_id, issue_id, id);
