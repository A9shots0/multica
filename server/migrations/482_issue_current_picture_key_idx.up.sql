CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_issue_current_picture_key ON issue_current_picture (workspace_id, issue_id);
