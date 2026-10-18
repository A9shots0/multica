CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_development_memory_proposal_task
    ON development_memory_proposal (task_id);
