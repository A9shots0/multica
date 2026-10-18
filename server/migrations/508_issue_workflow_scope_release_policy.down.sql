DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM issue_workflow_scope
        WHERE requires_deployment IS NOT NULL OR source_plan_revision IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'workflow scope contains durable release policy; refusing destructive rollback';
    END IF;
END $$;

ALTER TABLE issue_workflow_scope
    DROP CONSTRAINT IF EXISTS issue_workflow_scope_release_policy_coherent,
    DROP COLUMN IF EXISTS source_plan_revision,
    DROP COLUMN IF EXISTS requires_deployment;
