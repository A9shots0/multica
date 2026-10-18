CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_event_source_type
    ON issue_workflow_event (source_task_id, event_type);
