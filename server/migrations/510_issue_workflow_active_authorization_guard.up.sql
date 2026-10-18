DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM issue_workflow_decision
        WHERE decision_type = 'authorization'
          AND superseded_at IS NULL
          AND (state = 'unanswered' OR resolution_state = 'resuming')
        GROUP BY workspace_id, issue_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot enforce one active authorization decision: duplicate active decisions exist';
    END IF;
END $$;
