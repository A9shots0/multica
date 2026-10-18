CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_development_memory_proposal_due
    ON development_memory_proposal (next_attempt_at, created_at)
    WHERE review_status = 'approved' AND delivered_at IS NULL AND dead_lettered_at IS NULL;
