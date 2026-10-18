DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM issue_workflow_incident
        WHERE regression_fingerprint IS NOT NULL OR recovery_release_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'workflow incident contains durable recovery identity; refusing destructive rollback';
    END IF;
    ALTER TABLE issue_workflow_incident
        DROP CONSTRAINT IF EXISTS issue_workflow_incident_recovery_identity_coherent,
        DROP COLUMN IF EXISTS recovery_release_id,
        DROP COLUMN IF EXISTS regression_fingerprint;
END $$;
