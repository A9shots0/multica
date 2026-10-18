CREATE TABLE squad_workflow_template (
    squad_id UUID PRIMARY KEY REFERENCES squad(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    implementer_id UUID NOT NULL REFERENCES agent(id),
    reviewer_id UUID NOT NULL REFERENCES agent(id),
    release_agent_id UUID REFERENCES agent(id),
    verifier_id UUID REFERENCES agent(id),
    requires_owner BOOLEAN NOT NULL DEFAULT FALSE,
    requires_deployment BOOLEAN NOT NULL DEFAULT FALSE,
    max_corrections INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (implementer_id <> reviewer_id),
    CHECK (verifier_id IS NULL OR verifier_id <> release_agent_id),
    CHECK (max_corrections BETWEEN 1 AND 3)
);

CREATE INDEX squad_workflow_template_workspace_idx
    ON squad_workflow_template(workspace_id);
