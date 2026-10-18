ALTER TABLE issue_workflow_release
    ADD COLUMN attempt_contract_version SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN acceptance_version BIGINT,
    ADD COLUMN operation_id UUID,
    ADD COLUMN branch_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN expected_target_release_id UUID,
    ADD COLUMN expected_target_generation BIGINT,
    ADD COLUMN release_kind TEXT NOT NULL DEFAULT 'forward',
    ADD COLUMN recovery_of_release_id UUID,
    ADD COLUMN head_observed_at TIMESTAMPTZ,
    ADD COLUMN release_operator_id UUID,
    ADD COLUMN planned_verifier_id UUID,
    ADD COLUMN build_manifest_digest TEXT,
    ADD COLUMN artifact_digest TEXT,
    ADD COLUMN runtime_contract_digest TEXT,
    ADD COLUMN authority_capability TEXT,
    ADD COLUMN authority_evidence_id UUID,
    ADD COLUMN recovery_policy_revision BIGINT,
    ADD CONSTRAINT issue_workflow_release_attempt_generation_valid CHECK (
        expected_target_generation IS NULL
        OR (
            expected_target_generation >= 0
            AND target_generation > expected_target_generation
            AND (
                (expected_target_generation = 0 AND expected_target_release_id IS NULL)
                OR (expected_target_generation > 0 AND expected_target_release_id IS NOT NULL)
            )
        )
    ),
    ADD CONSTRAINT issue_workflow_release_attempt_kind_valid CHECK (
        release_kind IN ('forward', 'recovery')
        AND (
            (release_kind = 'forward' AND recovery_of_release_id IS NULL)
            OR (release_kind = 'recovery' AND recovery_of_release_id IS NOT NULL AND expected_target_generation IS NOT NULL)
        )
    ),
    ADD CONSTRAINT issue_workflow_release_attempt_text_valid CHECK (
        branch_key = btrim(branch_key)
        AND (build_manifest_digest IS NULL OR build_manifest_digest <> '' AND build_manifest_digest = btrim(build_manifest_digest))
        AND (artifact_digest IS NULL OR artifact_digest <> '' AND artifact_digest = btrim(artifact_digest))
        AND (runtime_contract_digest IS NULL OR runtime_contract_digest <> '' AND runtime_contract_digest = btrim(runtime_contract_digest))
        AND (authority_capability IS NULL OR authority_capability <> '' AND authority_capability = btrim(authority_capability))
    ),
    ADD CONSTRAINT issue_workflow_release_attempt_positive_revisions CHECK (
        (acceptance_version IS NULL OR acceptance_version > 0)
        AND (recovery_policy_revision IS NULL OR recovery_policy_revision > 0)
    ),
    ADD CONSTRAINT issue_workflow_release_attempt_contract_complete CHECK (
        attempt_contract_version IN (1, 2)
        AND (
            attempt_contract_version = 1
            OR (
                acceptance_version IS NOT NULL AND operation_id IS NOT NULL
                AND branch_key <> '' AND expected_target_generation IS NOT NULL
                AND release_operator_id IS NOT NULL AND planned_verifier_id IS NOT NULL
                AND release_operator_id <> planned_verifier_id
                AND build_manifest_digest IS NOT NULL AND artifact_digest IS NOT NULL
                AND runtime_contract_digest IS NOT NULL AND authority_capability IS NOT NULL
                AND authority_evidence_id IS NOT NULL AND recovery_policy_revision IS NOT NULL
            )
        )
    );

UPDATE issue_workflow_release AS release
SET acceptance_version = scope.acceptance_version,
    release_operator_id = CASE
        WHEN release.facets ? 'release_operator_id' AND release.facets->>'release_operator_id' <> ''
        THEN (release.facets->>'release_operator_id')::uuid
        ELSE NULL
    END,
    planned_verifier_id = CASE
        WHEN release.facets ? 'planned_verifier_id' AND release.facets->>'planned_verifier_id' <> ''
        THEN (release.facets->>'planned_verifier_id')::uuid
        ELSE NULL
    END
FROM issue_workflow_scope AS scope
WHERE scope.workspace_id = release.workspace_id
  AND scope.issue_id = release.issue_id
  AND scope.id = release.scope_id
  AND scope.revision = release.scope_revision;

UPDATE issue_workflow_release AS release
SET head_observed_at = head.updated_at
FROM issue_workflow_target_head AS head
WHERE head.workspace_id = release.workspace_id
  AND head.target_key = release.target_key
  AND head.release_id = release.id
  AND head.issue_id = release.issue_id
  AND head.candidate_id = release.candidate_id
  AND head.generation = release.target_generation;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM issue_workflow_release
        WHERE acceptance_version IS NULL
           OR (target_key <> '' AND state IN ('deployed', 'exposed', 'production_verifying', 'live', 'degraded', 'rolling_back', 'rolled_back', 'restored') AND head_observed_at IS NULL)
    ) THEN
        RAISE EXCEPTION 'workflow release metadata cannot be backfilled from exact authoritative scope and target head';
    END IF;
END $$;
