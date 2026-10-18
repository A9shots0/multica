CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_attention_key ON issue_workflow_attention (workspace_id, issue_id, id);
