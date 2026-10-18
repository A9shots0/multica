ALTER TABLE issue_workflow_decision
    ADD COLUMN suspended_execution_state TEXT NOT NULL
    CHECK (suspended_execution_state IN (
        'received', 'queued', 'working', 'waiting_automatic', 'in_review',
        'correction', 'blocked_external', 'technically_stopped', 'paused',
        'completed', 'failed', 'cancelled', 'superseded'
    ));
