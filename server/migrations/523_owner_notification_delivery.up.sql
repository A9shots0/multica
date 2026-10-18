CREATE TABLE owner_notification_binding (
    workspace_id UUID NOT NULL,
    recipient_id UUID NOT NULL,
    destination_key TEXT NOT NULL CHECK (length(btrim(destination_key)) BETWEEN 1 AND 128),
    destination_fingerprint TEXT NOT NULL CHECK (length(btrim(destination_fingerprint)) BETWEEN 16 AND 128),
    delivery_mode TEXT NOT NULL DEFAULT 'dry_run' CHECK (delivery_mode IN ('dry_run', 'live_test', 'live')),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE owner_notification (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    recipient_id UUID NOT NULL,
    delivery_mode TEXT NOT NULL CHECK (delivery_mode IN ('dry_run', 'live_test', 'live')),
    event_type TEXT NOT NULL CHECK (event_type IN (
        'owner_decision', 'important_incident', 'production_regression',
        'rollback_started', 'verified_completion'
    )),
    event_key TEXT NOT NULL CHECK (length(btrim(event_key)) BETWEEN 1 AND 512),
    source_type TEXT NOT NULL CHECK (length(btrim(source_type)) BETWEEN 1 AND 64),
    source_id TEXT NOT NULL CHECK (length(btrim(source_id)) BETWEEN 1 AND 256),
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    state TEXT NOT NULL DEFAULT 'admitted' CHECK (state IN (
        'admitted', 'claimed', 'provider_accepted', 'delivered', 'ambiguous',
        'retryable', 'rejected', 'failed', 'acknowledged', 'resolved'
    )),
    destination_key TEXT NOT NULL CHECK (length(btrim(destination_key)) BETWEEN 1 AND 128),
    message TEXT NOT NULL CHECK (length(btrim(message)) BETWEEN 1 AND 1000),
    action_url TEXT NOT NULL CHECK (length(btrim(action_url)) BETWEEN 1 AND 2000),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    delivery_check_count INTEGER NOT NULL DEFAULT 0 CHECK (delivery_check_count >= 0),
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ,
    provider_message_id TEXT,
    provider_state TEXT,
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempted_at TIMESTAMPTZ,
    provider_accepted_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE owner_notification_attempt (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL,
    request_key TEXT NOT NULL CHECK (length(btrim(request_key)) BETWEEN 1 AND 512),
    state TEXT NOT NULL CHECK (state IN (
        'started', 'dry_run_delivered', 'provider_accepted', 'delivered',
        'ambiguous', 'retryable', 'rejected', 'failed'
    )),
    provider_message_id TEXT,
    provider_state TEXT,
    error_code TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
