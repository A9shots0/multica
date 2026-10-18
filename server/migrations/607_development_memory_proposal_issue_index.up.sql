CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_development_memory_proposal_issue
    ON development_memory_proposal (workspace_id, issue_id, created_at DESC);
