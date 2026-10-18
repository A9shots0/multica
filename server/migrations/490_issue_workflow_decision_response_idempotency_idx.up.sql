CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_workflow_decision_response_idempotency ON issue_workflow_decision_response (workspace_id, decision_id, idempotency_key);
