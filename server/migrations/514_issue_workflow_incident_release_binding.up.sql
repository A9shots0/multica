ALTER TABLE issue_workflow_incident
    ADD COLUMN release_revision BIGINT,
    ADD COLUMN candidate_id UUID,
    ADD COLUMN candidate_revision BIGINT,
    ADD COLUMN target_generation BIGINT,
    ADD COLUMN detection_evidence_id UUID,
    ADD COLUMN recovery_action_id UUID,
    ADD COLUMN compatibility_state TEXT,
    ADD CONSTRAINT issue_workflow_incident_release_binding_positive CHECK (
        (release_revision IS NULL OR release_revision > 0)
        AND (candidate_revision IS NULL OR candidate_revision > 0)
        AND (target_generation IS NULL OR target_generation >= 0)
    ),
    ADD CONSTRAINT issue_workflow_incident_compatibility_state_valid CHECK (
        compatibility_state IS NULL OR compatibility_state IN ('unknown', 'rollback_safe', 'containment_required', 'irreversible')
    ),
    ADD CONSTRAINT issue_workflow_incident_exact_binding_coherent CHECK (
        (
            release_revision IS NULL AND candidate_id IS NULL
            AND candidate_revision IS NULL AND target_generation IS NULL
            AND detection_evidence_id IS NULL
        )
        OR (
            release_id IS NOT NULL AND release_revision IS NOT NULL
            AND candidate_id IS NOT NULL AND candidate_revision IS NOT NULL
            AND target_key IS NOT NULL AND btrim(target_key) <> ''
            AND target_generation IS NOT NULL AND target_generation > 0
            AND detection_evidence_id IS NOT NULL
        )
    );
