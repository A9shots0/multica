CREATE TABLE chat_library (
    workspace_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 0,
    preferences jsonb NOT NULL DEFAULT '{"grouping":"recent","filter":"all","groups":[]}',
    PRIMARY KEY (workspace_id, owner_id)
);
