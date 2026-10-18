CREATE TABLE external_incident_source_binding (
    workspace_id UUID NOT NULL,
    source_key TEXT NOT NULL CHECK (length(source_key) BETWEEN 1 AND 100),
    creator_agent_id UUID NOT NULL,
    project_id UUID NOT NULL,
    credential_hash TEXT NOT NULL CHECK (length(credential_hash) = 64),
    credential_hint TEXT NOT NULL CHECK (length(credential_hint) BETWEEN 4 AND 20),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE external_incident_occurrence (
    workspace_id UUID NOT NULL,
    source_key TEXT NOT NULL CHECK (length(source_key) BETWEEN 1 AND 100),
    event_key TEXT NOT NULL CHECK (length(event_key) BETWEEN 1 AND 240),
    request_hash TEXT NOT NULL CHECK (length(request_hash) = 64),
    finding_key TEXT NOT NULL CHECK (length(finding_key) BETWEEN 1 AND 200),
    incident_key TEXT NOT NULL CHECK (length(incident_key) BETWEEN 1 AND 200),
    source_sequence BIGINT NOT NULL CHECK (source_sequence > 0),
    predecessor_event_key TEXT CHECK (
        predecessor_event_key IS NULL OR length(predecessor_event_key) BETWEEN 1 AND 240
    ),
    episode_generation BIGINT NOT NULL CHECK (episode_generation > 0),
    lifecycle_state TEXT NOT NULL CHECK (lifecycle_state IN ('new', 'worsened', 'needs_human', 'recovering', 'recovered', 'suppressed', 'retired')),
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    action_required BOOLEAN NOT NULL,
    summary TEXT NOT NULL CHECK (length(btrim(summary)) BETWEEN 1 AND 1000),
    next_step TEXT NOT NULL CHECK (length(btrim(next_step)) BETWEEN 1 AND 500),
    observed_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    binding_revision BIGINT NOT NULL CHECK (binding_revision > 0),
    creator_agent_id UUID NOT NULL,
    project_id UUID NOT NULL,
    state TEXT NOT NULL DEFAULT 'received' CHECK (state IN ('received', 'pending_predecessor', 'processing', 'applied', 'rejected', 'failed')),
    issue_id UUID,
    episode_revision BIGINT,
    result JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(result) = 'object'),
    error_code TEXT,
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE external_incident_episode (
    workspace_id UUID NOT NULL,
    source_key TEXT NOT NULL CHECK (length(source_key) BETWEEN 1 AND 100),
    finding_key TEXT NOT NULL CHECK (length(finding_key) BETWEEN 1 AND 200),
    incident_key TEXT NOT NULL CHECK (length(incident_key) BETWEEN 1 AND 200),
    generation BIGINT NOT NULL CHECK (generation > 0),
    issue_id UUID NOT NULL,
    current_state TEXT NOT NULL CHECK (current_state IN ('new', 'worsened', 'needs_human', 'recovering', 'recovered', 'suppressed', 'retired')),
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    current_event_key TEXT NOT NULL CHECK (length(current_event_key) BETWEEN 1 AND 240),
    current_source_sequence BIGINT NOT NULL CHECK (current_source_sequence > 0),
    machine_title TEXT NOT NULL,
    machine_description TEXT NOT NULL,
    machine_status TEXT NOT NULL,
    machine_priority TEXT NOT NULL,
    title_human_owned BOOLEAN NOT NULL DEFAULT FALSE,
    description_human_owned BOOLEAN NOT NULL DEFAULT FALSE,
    status_human_owned BOOLEAN NOT NULL DEFAULT FALSE,
    priority_human_owned BOOLEAN NOT NULL DEFAULT FALSE,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    opened_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE external_incident_quarantine (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    source_key TEXT NOT NULL CHECK (length(source_key) BETWEEN 1 AND 100),
    event_key TEXT NOT NULL CHECK (length(event_key) BETWEEN 1 AND 240),
    stored_request_hash TEXT NOT NULL CHECK (length(stored_request_hash) = 64),
    received_request_hash TEXT NOT NULL CHECK (length(received_request_hash) = 64),
    reason_code TEXT NOT NULL CHECK (length(reason_code) BETWEEN 1 AND 100),
    received_payload JSONB NOT NULL CHECK (jsonb_typeof(received_payload) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
