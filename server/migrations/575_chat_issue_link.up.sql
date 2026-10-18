-- Durable many-to-many links between web/channel chats and work issues.
-- A chat can discuss several issues and an issue can be discussed in several
-- chats; the existing agent_task_queue foreign keys are per-run provenance,
-- not a durable conversation relationship.
CREATE TABLE chat_issue_link (
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    chat_session_id UUID NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'binding_receipt', 'origin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, chat_session_id, issue_id)
);

CREATE INDEX idx_chat_issue_link_issue ON chat_issue_link(workspace_id, issue_id, created_at DESC);
CREATE INDEX idx_chat_issue_link_chat ON chat_issue_link(workspace_id, chat_session_id, created_at DESC);
