CREATE INDEX CONCURRENTLY issue_workflow_consequence_source_idx ON issue_workflow_consequence_receipt (workspace_id, source_issue_id, committed_at, id);
