CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_external_work_receipt_idempotency ON issue_external_work_receipt (workspace_id, issue_id, idempotency_key);
