CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_workflow_concern_head_source ON issue_workflow_concern_head (workspace_id, issue_id, source_record_id, source_record_revision);
