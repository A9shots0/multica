ALTER TABLE issue_workflow_plan
    ADD COLUMN correction_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN max_corrections INTEGER NOT NULL DEFAULT 1,
    ADD CONSTRAINT issue_workflow_plan_correction_count_nonnegative CHECK (correction_count >= 0),
    ADD CONSTRAINT issue_workflow_plan_max_corrections_bounded CHECK (max_corrections BETWEEN 1 AND 3);

ALTER TABLE issue_workflow_event
    ADD COLUMN payload JSONB NOT NULL DEFAULT '{}'::jsonb;
