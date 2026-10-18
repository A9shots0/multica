CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_workflow_action_vertical_due
    ON issue_workflow_action_receipt (not_before, lease_expires_at, created_at, id)
    WHERE action_type IN ('review_correction', 'review_candidate', 'finalize_non_live')
      AND state IN ('admitted', 'retry_wait', 'reconciling', 'leased');
