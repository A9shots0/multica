ALTER TABLE issue_workflow_incident
    ADD COLUMN regression_fingerprint TEXT,
    ADD COLUMN recovery_release_id UUID,
    ADD CONSTRAINT issue_workflow_incident_recovery_identity_coherent CHECK (
        (regression_fingerprint IS NULL AND recovery_release_id IS NULL)
        OR (
            regression_fingerprint IS NOT NULL
            AND btrim(regression_fingerprint) <> ''
            AND recovery_release_id IS NOT NULL
            AND release_id IS NOT NULL
            AND recovery_release_id <> release_id
            AND release_revision IS NOT NULL
            AND candidate_id IS NOT NULL
            AND candidate_revision IS NOT NULL
            AND target_key IS NOT NULL
            AND target_generation IS NOT NULL
            AND detection_evidence_id IS NOT NULL
            AND compatibility_state = 'rollback_safe'
        )
        OR (
            regression_fingerprint IS NOT NULL
            AND btrim(regression_fingerprint) <> ''
            AND recovery_release_id IS NULL
            AND release_id IS NOT NULL
            AND release_revision IS NOT NULL
            AND candidate_id IS NOT NULL
            AND candidate_revision IS NOT NULL
            AND target_key IS NOT NULL
            AND target_generation IS NOT NULL
            AND detection_evidence_id IS NOT NULL
            AND compatibility_state IN ('unknown', 'containment_required', 'irreversible')
        )
    );
