CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_incident_key ON issue_workflow_incident (workspace_id, issue_id, id);
