ALTER TABLE issue_workflow_event DROP COLUMN IF EXISTS payload;

ALTER TABLE issue_workflow_plan
    DROP CONSTRAINT IF EXISTS issue_workflow_plan_max_corrections_bounded,
    DROP CONSTRAINT IF EXISTS issue_workflow_plan_correction_count_nonnegative,
    DROP COLUMN IF EXISTS max_corrections,
    DROP COLUMN IF EXISTS correction_count;
