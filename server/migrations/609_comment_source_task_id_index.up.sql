CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_comment_source_task_id
    ON comment(source_task_id)
    WHERE source_task_id IS NOT NULL;
