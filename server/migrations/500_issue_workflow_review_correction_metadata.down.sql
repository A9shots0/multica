ALTER TABLE issue_workflow_review
    DROP CONSTRAINT IF EXISTS issue_workflow_review_correction_budget_check,
    DROP COLUMN IF EXISTS max_corrections,
    DROP COLUMN IF EXISTS correction_count;
