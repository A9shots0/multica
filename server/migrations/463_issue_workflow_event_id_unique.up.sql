CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_event_id
    ON issue_workflow_event (id);
