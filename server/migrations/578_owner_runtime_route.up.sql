-- Explicit, per-issue human authority for ordinary runtime workflow stages.
-- NULL preserves the existing guarded workflow path.
ALTER TABLE issue_workflow_plan ADD COLUMN owner_runtime_route jsonb;
