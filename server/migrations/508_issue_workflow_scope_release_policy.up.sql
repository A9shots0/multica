ALTER TABLE issue_workflow_scope
    ADD COLUMN requires_deployment BOOLEAN,
    ADD COLUMN source_plan_revision BIGINT,
    ADD CONSTRAINT issue_workflow_scope_release_policy_coherent CHECK (
        (requires_deployment IS NULL AND source_plan_revision IS NULL)
        OR (requires_deployment IS NOT NULL AND source_plan_revision > 0)
    );

-- Existing scopes intentionally remain NULL/unknown. The compatibility plan
-- is mutable and has no scope-time provenance, so retroactively copying its
-- current default could silently authorize a non-live completion.
