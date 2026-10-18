CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_external_work_receipt_revision ON issue_external_work_receipt (workspace_id, issue_id, provider, work_id, receipt_revision);
