CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_decision_response_key ON issue_workflow_decision_response (workspace_id, issue_id, id);
