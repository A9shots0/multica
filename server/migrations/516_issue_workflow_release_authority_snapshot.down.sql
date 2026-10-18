DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM issue_workflow_release
        WHERE attempt_contract_version = 3
           OR authority_envelope_digest IS NOT NULL
           OR owner_evidence_revision IS NOT NULL
           OR exposure_policy_digest IS NOT NULL
           OR rollback_target_release_id IS NOT NULL
           OR irreversible_effects_digest IS NOT NULL
           OR reviewed_review_id IS NOT NULL
           OR reviewed_review_revision IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'workflow release contains durable authority snapshots; refusing destructive rollback';
    END IF;
    ALTER TABLE issue_workflow_release
        DROP CONSTRAINT IF EXISTS issue_workflow_release_attempt_contract_complete,
        DROP CONSTRAINT IF EXISTS issue_workflow_release_admission_revision_valid,
        DROP CONSTRAINT IF EXISTS issue_workflow_release_admission_text_valid,
        DROP COLUMN IF EXISTS reviewed_review_revision,
        DROP COLUMN IF EXISTS reviewed_review_id,
        DROP COLUMN IF EXISTS irreversible_effects_digest,
        DROP COLUMN IF EXISTS rollback_target_release_id,
        DROP COLUMN IF EXISTS exposure_policy_digest,
        DROP COLUMN IF EXISTS owner_evidence_revision,
        DROP COLUMN IF EXISTS authority_envelope_digest,
        ADD CONSTRAINT issue_workflow_release_attempt_contract_complete CHECK (
            attempt_contract_version IN (1, 2)
            AND (attempt_contract_version = 1 OR (
                acceptance_version IS NOT NULL AND operation_id IS NOT NULL
                AND branch_key <> '' AND expected_target_generation IS NOT NULL
                AND release_operator_id IS NOT NULL AND planned_verifier_id IS NOT NULL
                AND release_operator_id <> planned_verifier_id
                AND build_manifest_digest IS NOT NULL AND artifact_digest IS NOT NULL
                AND runtime_contract_digest IS NOT NULL AND authority_capability IS NOT NULL
                AND authority_evidence_id IS NOT NULL AND recovery_policy_revision IS NOT NULL
            ))
        );
END $$;
