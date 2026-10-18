-- Append-only owner feedback bound to the exact screenshot that was reviewed.
-- No foreign keys: issue and attachment cleanup must not erase the audit trail
-- describing what the owner saw and marked.
CREATE TABLE owner_review_feedback (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    user_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    inbox_item_id UUID,
    source_attachment_id UUID NOT NULL,
    source_filename TEXT NOT NULL,
    source_content_type TEXT NOT NULL,
    marked_screenshot_attachment_id UUID NOT NULL,
    marks JSONB NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (jsonb_typeof(marks) = 'array')
);
