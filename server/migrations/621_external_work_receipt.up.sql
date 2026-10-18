CREATE TABLE issue_external_work_receipt (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    contract_version TEXT NOT NULL,
    provider TEXT NOT NULL,
    work_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    receipt_revision BIGINT NOT NULL,
    supersedes_receipt_revision BIGINT,
    actor_type TEXT NOT NULL,
    actor_id UUID NOT NULL,
    credential_kind TEXT NOT NULL,
    issue_revision BIGINT NOT NULL,
    authority_claim_state TEXT NOT NULL,
    decision_proposal_state TEXT NOT NULL,
    raw_receipt JSONB NOT NULL,
    evidence_id UUID,
    execution_id UUID,
    execution_revision BIGINT,
    current_picture_revision BIGINT,
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (receipt_revision >= 1),
    CHECK (contract_version = '0.2'),
    CHECK (
        (receipt_revision = 1 AND supersedes_receipt_revision IS NULL)
        OR (receipt_revision > 1 AND supersedes_receipt_revision = receipt_revision - 1)
    ),
    CHECK (actor_type IN ('member', 'plugin')),
    CHECK (credential_kind IN ('user_oauth', 'personal_access_token', 'plugin_installation', 'plugin_invocation')),
    CHECK (
        (actor_type = 'member' AND credential_kind IN ('user_oauth', 'personal_access_token', 'plugin_invocation'))
        OR (actor_type = 'plugin' AND credential_kind IN ('plugin_installation', 'plugin_invocation'))
    ),
    CHECK (authority_claim_state IN ('not_required_for_observation', 'matched_native_authority', 'unverified')),
    CHECK (decision_proposal_state IN ('none', 'recorded_non_actionable'))
);
