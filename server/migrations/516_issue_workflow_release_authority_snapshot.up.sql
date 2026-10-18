ALTER TABLE issue_workflow_release
    ADD COLUMN authority_envelope_digest TEXT,
    ADD COLUMN owner_evidence_revision BIGINT,
    ADD COLUMN exposure_policy_digest TEXT,
    ADD COLUMN rollback_target_release_id UUID,
    ADD COLUMN irreversible_effects_digest TEXT,
    ADD COLUMN reviewed_review_id UUID,
    ADD COLUMN reviewed_review_revision BIGINT,
    DROP CONSTRAINT issue_workflow_release_attempt_contract_complete,
    ADD CONSTRAINT issue_workflow_release_admission_text_valid CHECK (
        (authority_envelope_digest IS NULL OR authority_envelope_digest <> '' AND authority_envelope_digest = btrim(authority_envelope_digest))
        AND (exposure_policy_digest IS NULL OR exposure_policy_digest <> '' AND exposure_policy_digest = btrim(exposure_policy_digest))
        AND (irreversible_effects_digest IS NULL OR irreversible_effects_digest <> '' AND irreversible_effects_digest = btrim(irreversible_effects_digest))
    ),
    ADD CONSTRAINT issue_workflow_release_admission_revision_valid CHECK (
        (owner_evidence_revision IS NULL OR owner_evidence_revision > 0)
        AND (reviewed_review_revision IS NULL OR reviewed_review_revision > 0)
    ),
    ADD CONSTRAINT issue_workflow_release_attempt_contract_complete CHECK (
        attempt_contract_version IN (1, 2, 3)
        AND (attempt_contract_version = 1 OR (
            acceptance_version IS NOT NULL AND operation_id IS NOT NULL
            AND branch_key <> '' AND expected_target_generation IS NOT NULL
            AND release_operator_id IS NOT NULL AND planned_verifier_id IS NOT NULL
            AND release_operator_id <> planned_verifier_id
            AND build_manifest_digest IS NOT NULL AND artifact_digest IS NOT NULL
            AND runtime_contract_digest IS NOT NULL AND authority_capability IS NOT NULL
            AND authority_evidence_id IS NOT NULL AND recovery_policy_revision IS NOT NULL
        ))
        AND (attempt_contract_version < 3 OR (
            authority_envelope_digest IS NOT NULL AND owner_evidence_revision IS NOT NULL
            AND exposure_policy_digest IS NOT NULL AND irreversible_effects_digest IS NOT NULL
            AND reviewed_review_id IS NOT NULL AND reviewed_review_revision IS NOT NULL
        ))
    );
