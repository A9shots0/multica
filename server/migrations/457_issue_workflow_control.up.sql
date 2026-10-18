CREATE TABLE issue_workflow_plan (
    issue_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    squad_id UUID NOT NULL,
    implementer_id UUID NOT NULL,
    reviewer_id UUID NOT NULL,
    release_agent_id UUID,
    verifier_id UUID,
    requires_owner BOOLEAN NOT NULL DEFAULT FALSE,
    requires_deployment BOOLEAN NOT NULL DEFAULT FALSE,
    current_state TEXT NOT NULL DEFAULT 'working',
    suspended_state TEXT,
    suspended_agent_id UUID,
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (implementer_id <> reviewer_id),
    CHECK (verifier_id IS NULL OR verifier_id <> release_agent_id)
);

CREATE TABLE issue_workflow_event (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    source_task_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    decision JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
