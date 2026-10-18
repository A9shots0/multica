CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_preview_publication_identity ON issue_preview_publication (workspace_id, issue_id, candidate_id, candidate_revision);
