CREATE TABLE issue_workflow_concern_head (
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    concern_type TEXT NOT NULL CHECK (concern_type IN (
        'outcome', 'scope', 'authority', 'execution', 'review',
        'release', 'attention', 'evidence', 'budget'
    )),
    revision BIGINT NOT NULL CHECK (revision > 0),
    source_record_id UUID NOT NULL,
    source_record_revision BIGINT NOT NULL CHECK (source_record_revision > 0),
    state TEXT NOT NULL CHECK (state <> '' AND state = btrim(state)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (concern_type = 'outcome' AND state IN ('open', 'satisfied', 'cancelled', 'superseded')) OR
        (concern_type = 'scope' AND state IN ('accepted', 'superseded')) OR
        (concern_type = 'authority' AND state IN ('missing', 'active', 'paused', 'revoked', 'expired', 'superseded')) OR
        (concern_type = 'execution' AND state IN ('received', 'queued', 'working', 'waiting_automatic', 'in_review', 'correction', 'blocked_external', 'technically_stopped', 'paused', 'completed', 'failed', 'cancelled', 'superseded')) OR
        (concern_type = 'review' AND state IN ('not_required', 'pending', 'in_progress', 'passed', 'failed', 'inconclusive', 'blocked_by_evidence')) OR
        (concern_type = 'release' AND state IN ('not_required', 'pending', 'built', 'reviewed', 'authorized', 'deploying', 'deployed', 'exposed', 'production_verifying', 'live', 'degraded', 'rolling_back', 'rolled_back', 'restored', 'failed')) OR
        (concern_type = 'attention' AND state IN ('open', 'resuming', 'resolved', 'snoozed')) OR
        (concern_type = 'evidence' AND state IN ('unknown', 'current', 'stale', 'contradicted')) OR
        (concern_type = 'budget' AND state IN ('active', 'exhausted'))
    )
);
