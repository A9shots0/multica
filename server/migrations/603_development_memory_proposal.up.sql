-- Human-reviewed development memory candidates. No foreign keys: task and
-- issue deletion must not erase the audit trail or an approved delivery intent.
CREATE TABLE development_memory_proposal (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    task_id UUID NOT NULL,
    workstream_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    task_started_at TIMESTAMPTZ NOT NULL,
    task_completed_at TIMESTAMPTZ NOT NULL,
    task_status TEXT NOT NULL CHECK (task_status IN ('complete', 'failed', 'cancelled')),
    binding JSONB NOT NULL,
    candidate JSONB NOT NULL,
    approved_payload JSONB,
    source_output_sha256 TEXT NOT NULL,
    review_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (review_status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID,
    reviewed_at TIMESTAMPTZ,
    review_note TEXT,
    delivered_at TIMESTAMPTZ,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_token UUID,
    last_error TEXT,
    dead_lettered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((review_status = 'approved') = (approved_payload IS NOT NULL)),
    CHECK ((review_status = 'pending') OR reviewed_at IS NOT NULL)
);
