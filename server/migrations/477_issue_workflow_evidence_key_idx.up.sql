CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_evidence_key ON issue_workflow_evidence (workspace_id, issue_id, id);
