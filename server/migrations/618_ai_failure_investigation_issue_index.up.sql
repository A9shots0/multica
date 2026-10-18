CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_failure_investigation_issue_status
    ON ai_failure_investigation (workspace_id, issue_id, status, created_at);
