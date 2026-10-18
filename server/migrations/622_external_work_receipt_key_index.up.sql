CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_external_work_receipt_key ON issue_external_work_receipt (workspace_id, issue_id, id);
