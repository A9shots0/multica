CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_workflow_active_decision ON issue_workflow_decision (workspace_id, issue_id, created_at) WHERE state = 'unanswered' OR resolution_state = 'resuming';
