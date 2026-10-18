CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_plan_issue
    ON issue_workflow_plan (issue_id);
