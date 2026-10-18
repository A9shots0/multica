CREATE TABLE issue_owner_directive (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    directive_type TEXT NOT NULL CHECK (directive_type IN ('correct', 'request_changes', 'partial_acceptance', 'change_scope', 'cancel')),
    expected_issue_revision BIGINT NOT NULL CHECK (expected_issue_revision > 0),
    content TEXT NOT NULL CHECK (length(btrim(content)) BETWEEN 1 AND 4000),
    state TEXT NOT NULL DEFAULT 'recorded' CHECK (state IN ('recorded', 'applied', 'cancelled', 'failed')),
    actor_id UUID NOT NULL REFERENCES "user"(id),
    comment_id UUID REFERENCES comment(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, issue_id, id)
);

CREATE INDEX issue_owner_directive_issue_created_idx
    ON issue_owner_directive (workspace_id, issue_id, created_at DESC);
