DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM issue_workflow_concern_head LIMIT 1) THEN
        RAISE EXCEPTION 'workflow concern heads contain durable records; migrate or explicitly remove them before rollback';
    END IF;
END $$;

DROP TABLE IF EXISTS issue_workflow_concern_head;
