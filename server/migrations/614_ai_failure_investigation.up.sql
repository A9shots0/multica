-- One durable investigation per declared AI failure. This is operational
-- evidence, so it survives issue/task cleanup and deliberately has no FKs.
CREATE TABLE ai_failure_investigation (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    trigger_kind TEXT NOT NULL CHECK (trigger_kind IN ('task_failed', 'owner_rejection', 'qa_rejection')),
    trigger_id UUID NOT NULL,
    source_task_id UUID,
    source_agent_id UUID,
    accountable_user_id UUID,
    summary TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'awaiting_investigator', 'investigator_failed')),
    research_scope JSONB NOT NULL,
    investigator_agent_id UUID,
    investigator_task_id UUID,
    trigger_comment_id UUID,
    findings JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    dispatched_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (jsonb_typeof(research_scope) = 'array'),
    CHECK ((status = 'completed') = (findings IS NOT NULL))
);
