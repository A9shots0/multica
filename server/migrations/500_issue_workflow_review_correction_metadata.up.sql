ALTER TABLE issue_workflow_review
    ADD COLUMN correction_count INTEGER NOT NULL DEFAULT 0 CHECK (correction_count >= 0),
    ADD COLUMN max_corrections INTEGER NOT NULL DEFAULT 0 CHECK (max_corrections >= 0),
    ADD CONSTRAINT issue_workflow_review_correction_budget_check
        CHECK (correction_count <= max_corrections OR max_corrections = 0);
