CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_failure_investigation_trigger
    ON ai_failure_investigation (workspace_id, trigger_kind, trigger_id);
