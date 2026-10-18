CREATE INDEX CONCURRENTLY issue_workflow_consequence_affected_idx ON issue_workflow_consequence_receipt (workspace_id, affected_issue_id, committed_at, id);
