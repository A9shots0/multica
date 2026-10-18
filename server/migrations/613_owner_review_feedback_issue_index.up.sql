CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_owner_review_feedback_issue_created
    ON owner_review_feedback (workspace_id, issue_id, created_at DESC);
