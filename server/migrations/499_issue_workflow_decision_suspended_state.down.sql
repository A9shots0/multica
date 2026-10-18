DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM issue_workflow_decision LIMIT 1) THEN
        RAISE EXCEPTION 'workflow decisions contain suspended-state authority; migrate or explicitly remove them before rollback';
    END IF;
END $$;

ALTER TABLE issue_workflow_decision DROP COLUMN IF EXISTS suspended_execution_state;
