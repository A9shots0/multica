CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_workflow_plan_workspace
    ON issue_workflow_plan (workspace_id, updated_at DESC);
