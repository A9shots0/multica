CREATE INDEX CONCURRENTLY issue_workflow_regression_issue_idx ON issue_workflow_regression_change_record (workspace_id, source_issue_id, applied_at, id);
