CREATE TABLE issue_preview_publication (
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    candidate_id UUID NOT NULL,
    candidate_revision BIGINT NOT NULL CHECK (candidate_revision = 1),
    agent_id UUID NOT NULL,
    agent_run_id UUID NOT NULL,
    source_revision TEXT NOT NULL,
    daemon_id TEXT NOT NULL,
    runtime_id UUID NOT NULL,
    worktree_path TEXT NOT NULL,
    branch TEXT NOT NULL,
    preview_url TEXT NOT NULL,
    identity_url TEXT NOT NULL,
    data_policy JSONB NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    publication JSONB NOT NULL
);
