-- The issue admission lock prevents teardown but permits concurrent workflow
-- writers. Serialize revisions of one external provider/work stream so exact
-- retries and distinct successors have one deterministic order.
-- name: LockExternalWorkReceiptStream :exec
WITH receipt_identity AS (
    SELECT sqlc.arg('workspace_id')::uuid AS workspace_id,
           sqlc.arg('issue_id')::uuid AS issue_id,
           sqlc.arg('provider')::text AS provider,
           sqlc.arg('work_id')::text AS work_id
)
SELECT pg_advisory_xact_lock(hashtextextended(
    workspace_id::text || ':' || issue_id::text || ':' || provider || ':' || work_id,
    19483273
))
FROM receipt_identity;

-- name: CreateExternalWorkReceipt :one
INSERT INTO issue_external_work_receipt (
    id, workspace_id, issue_id, idempotency_key, request_hash, contract_version,
    provider, work_id, run_id, receipt_revision, supersedes_receipt_revision,
    actor_type, actor_id, credential_kind, issue_revision,
    authority_claim_state, decision_proposal_state, raw_receipt
) VALUES (
    @id, @workspace_id, @issue_id, @idempotency_key, @request_hash, '0.2',
    @provider, @work_id, @run_id, @receipt_revision,
    sqlc.narg('supersedes_receipt_revision'), @actor_type, @actor_id,
    @credential_kind, @issue_revision, @authority_claim_state,
    @decision_proposal_state, @raw_receipt
)
ON CONFLICT (workspace_id, issue_id, idempotency_key) DO UPDATE
SET request_hash = issue_external_work_receipt.request_hash
WHERE issue_external_work_receipt.request_hash = EXCLUDED.request_hash
  AND issue_external_work_receipt.provider = EXCLUDED.provider
  AND issue_external_work_receipt.work_id = EXCLUDED.work_id
  AND issue_external_work_receipt.run_id = EXCLUDED.run_id
  AND issue_external_work_receipt.receipt_revision = EXCLUDED.receipt_revision
  AND issue_external_work_receipt.supersedes_receipt_revision IS NOT DISTINCT FROM EXCLUDED.supersedes_receipt_revision
  AND issue_external_work_receipt.actor_type = EXCLUDED.actor_type
  AND issue_external_work_receipt.actor_id = EXCLUDED.actor_id
  AND issue_external_work_receipt.credential_kind = EXCLUDED.credential_kind
  AND issue_external_work_receipt.issue_revision = EXCLUDED.issue_revision
  AND issue_external_work_receipt.raw_receipt = EXCLUDED.raw_receipt
RETURNING *;

-- name: GetExternalWorkReceiptByIdempotencyKey :one
SELECT * FROM issue_external_work_receipt
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND idempotency_key = @idempotency_key;

-- name: GetLatestExternalWorkReceiptForWork :one
SELECT * FROM issue_external_work_receipt
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND provider = @provider
  AND work_id = @work_id
ORDER BY receipt_revision DESC
LIMIT 1
FOR UPDATE;

-- name: ListExternalWorkReceiptsForIssue :many
SELECT * FROM issue_external_work_receipt
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND evidence_id IS NOT NULL
  AND execution_id IS NOT NULL
ORDER BY admitted_at DESC, id DESC
LIMIT @page_size;

-- name: HasAdmittedExternalWorkForIssue :one
-- Context brokerage is available only to work that has already been linked to
-- this exact issue through a completed receipt admission.  The caller cannot
-- use a guessed work_id to browse another project's shared context.
SELECT EXISTS (
    SELECT 1
    FROM issue_external_work_receipt
    WHERE workspace_id = @workspace_id
      AND issue_id = @issue_id
      AND work_id = @work_id
      AND evidence_id IS NOT NULL
      AND execution_id IS NOT NULL
);

-- name: SetExternalWorkReceiptAdmissionResult :one
UPDATE issue_external_work_receipt
SET evidence_id = @evidence_id,
    execution_id = @execution_id,
    execution_revision = @execution_revision,
    current_picture_revision = @current_picture_revision,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND evidence_id IS NULL
  AND execution_id IS NULL
  AND execution_revision IS NULL
  AND current_picture_revision IS NULL
RETURNING *;
