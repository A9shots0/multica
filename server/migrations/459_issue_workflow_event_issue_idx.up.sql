CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_workflow_event_issue
    ON issue_workflow_event (issue_id, created_at DESC);
