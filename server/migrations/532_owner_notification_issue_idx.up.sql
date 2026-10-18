CREATE INDEX CONCURRENTLY owner_notification_issue_idx ON owner_notification (workspace_id, issue_id, admitted_at DESC);
