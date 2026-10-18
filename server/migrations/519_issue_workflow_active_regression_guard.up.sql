DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM issue_workflow_incident
        WHERE regression_fingerprint IS NOT NULL
          AND superseded_at IS NULL
          AND state NOT IN ('resolved', 'cancelled', 'superseded')
        GROUP BY workspace_id, issue_id, regression_fingerprint
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate active workflow regression fingerprints exist; refusing unique-index admission';
    END IF;
END $$;
