DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM issue_workflow_release
        WHERE attempt_contract_version <> 1
           OR operation_id IS NOT NULL
           OR branch_key <> ''
           OR expected_target_release_id IS NOT NULL
           OR expected_target_generation IS NOT NULL
           OR release_kind <> 'forward'
           OR recovery_of_release_id IS NOT NULL
           OR head_observed_at IS NOT NULL
           OR release_operator_id IS NOT NULL
           OR planned_verifier_id IS NOT NULL
           OR build_manifest_digest IS NOT NULL
           OR artifact_digest IS NOT NULL
           OR runtime_contract_digest IS NOT NULL
           OR authority_capability IS NOT NULL
           OR authority_evidence_id IS NOT NULL
           OR recovery_policy_revision IS NOT NULL
           OR acceptance_version IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'workflow release contains durable attempt metadata; refusing destructive rollback';
    END IF;
    ALTER TABLE issue_workflow_release
        DROP CONSTRAINT IF EXISTS issue_workflow_release_attempt_contract_complete,
        DROP CONSTRAINT IF EXISTS issue_workflow_release_attempt_positive_revisions,
        DROP CONSTRAINT IF EXISTS issue_workflow_release_attempt_text_valid,
        DROP CONSTRAINT IF EXISTS issue_workflow_release_attempt_kind_valid,
        DROP CONSTRAINT IF EXISTS issue_workflow_release_attempt_generation_valid,
        DROP COLUMN IF EXISTS recovery_policy_revision,
        DROP COLUMN IF EXISTS authority_evidence_id,
        DROP COLUMN IF EXISTS authority_capability,
        DROP COLUMN IF EXISTS runtime_contract_digest,
        DROP COLUMN IF EXISTS artifact_digest,
        DROP COLUMN IF EXISTS build_manifest_digest,
        DROP COLUMN IF EXISTS planned_verifier_id,
        DROP COLUMN IF EXISTS release_operator_id,
        DROP COLUMN IF EXISTS head_observed_at,
        DROP COLUMN IF EXISTS recovery_of_release_id,
        DROP COLUMN IF EXISTS release_kind,
        DROP COLUMN IF EXISTS expected_target_generation,
        DROP COLUMN IF EXISTS expected_target_release_id,
        DROP COLUMN IF EXISTS branch_key,
        DROP COLUMN IF EXISTS operation_id,
        DROP COLUMN IF EXISTS acceptance_version,
        DROP COLUMN IF EXISTS attempt_contract_version;
END $$;
