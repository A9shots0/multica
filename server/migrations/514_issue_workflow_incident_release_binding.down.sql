DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM issue_workflow_incident
        WHERE release_revision IS NOT NULL
           OR candidate_id IS NOT NULL
           OR candidate_revision IS NOT NULL
           OR target_generation IS NOT NULL
           OR detection_evidence_id IS NOT NULL
           OR recovery_action_id IS NOT NULL
           OR compatibility_state IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'workflow incident contains durable release binding metadata; refusing destructive rollback';
    END IF;
    ALTER TABLE issue_workflow_incident
        DROP CONSTRAINT IF EXISTS issue_workflow_incident_exact_binding_coherent,
        DROP CONSTRAINT IF EXISTS issue_workflow_incident_compatibility_state_valid,
        DROP CONSTRAINT IF EXISTS issue_workflow_incident_release_binding_positive,
        DROP COLUMN IF EXISTS compatibility_state,
        DROP COLUMN IF EXISTS recovery_action_id,
        DROP COLUMN IF EXISTS detection_evidence_id,
        DROP COLUMN IF EXISTS target_generation,
        DROP COLUMN IF EXISTS candidate_revision,
        DROP COLUMN IF EXISTS candidate_id,
        DROP COLUMN IF EXISTS release_revision;
END $$;
