-- Controller-only durable system-graph evidence -------------------------

-- Serializes one workspace's system-graph append history. The lock is taken
-- inside the caller's transaction and is never exposed to agent-facing code.
-- name: LockSystemGraphEvidenceWorkspace :exec
SELECT pg_advisory_xact_lock(hashtextextended(@workspace_id::text, 519914));

-- name: LockSystemGraphEvidenceWorkspaceOwner :one
SELECT id FROM workspace WHERE id = @workspace_id FOR KEY SHARE;

-- name: LockSystemGraphEvidenceIssueOwner :one
SELECT id FROM issue
WHERE workspace_id = @workspace_id AND id = @issue_id
FOR KEY SHARE;

-- name: LockCurrentIssueOutcomeSnapshot :one
SELECT picture.workspace_id,
       picture.issue_id,
       picture.revision AS store_revision,
       picture.scope_revision,
       picture.authority_revision,
       outcome.id AS outcome_id,
       outcome.state AS outcome_state
FROM issue_current_picture AS picture
JOIN issue_workflow_outcome AS outcome
  ON outcome.workspace_id = picture.workspace_id
 AND outcome.issue_id = picture.issue_id
 AND outcome.superseded_at IS NULL
WHERE picture.workspace_id = @workspace_id
  AND picture.issue_id = @issue_id
  AND picture.scope_revision IS NOT NULL
  AND picture.authority_revision IS NOT NULL
ORDER BY outcome.revision DESC, outcome.updated_at DESC, outcome.id DESC
LIMIT 1
FOR SHARE OF picture, outcome;

-- name: CreateIssueWorkflowConsequenceReceipt :one
INSERT INTO issue_workflow_consequence_receipt (
    id, workspace_id, source_issue_id, affected_issue_id, idempotency_key,
    receipt_id, controller_authority_receipt_id, controller_id, graph_version,
    source_outcome_id, affected_outcome_id, source_scope_revision,
    affected_scope_revision, source_authority_revision,
    affected_authority_revision, relationship_kind, payload_sha256, receipt,
    committed_at
) VALUES (
    @id, @workspace_id, @source_issue_id, @affected_issue_id, @idempotency_key,
    @receipt_id, @controller_authority_receipt_id, @controller_id,
    @graph_version, @source_outcome_id, @affected_outcome_id,
    @source_scope_revision, @affected_scope_revision,
    @source_authority_revision, @affected_authority_revision,
    @relationship_kind, @payload_sha256, @receipt, @committed_at
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetIssueWorkflowConsequenceReceiptByIdempotency :one
SELECT * FROM issue_workflow_consequence_receipt
WHERE workspace_id = @workspace_id AND idempotency_key = @idempotency_key;

-- name: ListIssueWorkflowConsequenceReceipts :many
SELECT * FROM issue_workflow_consequence_receipt
WHERE workspace_id = @workspace_id
  AND (source_issue_id = @issue_id OR affected_issue_id = @issue_id)
ORDER BY committed_at ASC, id ASC;

-- name: ListWorkspaceIssueWorkflowConsequenceReceipts :many
SELECT * FROM issue_workflow_consequence_receipt
WHERE workspace_id = @workspace_id
ORDER BY committed_at DESC, id DESC
LIMIT @result_limit;

-- name: ListCurrentWorkflowEvidenceRefs :many
SELECT id
FROM issue_workflow_evidence
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = ANY(@evidence_ids::uuid[])
  AND superseded_at IS NULL
  AND observed_at <= @now
  AND (valid_until IS NULL OR valid_until > @now)
  AND jsonb_array_length(contradictions) = 0
ORDER BY id
FOR SHARE;

-- name: CreateIssueWorkflowRegressionChangeRecord :one
INSERT INTO issue_workflow_regression_change_record (
    id, workspace_id, source_issue_id, registry_version, proposal_id,
    obligation_id, record_hash, previous_record_hash, record, applied_at
) VALUES (
    @id, @workspace_id, @source_issue_id, @registry_version, @proposal_id,
    @obligation_id, @record_hash, @previous_record_hash, @record, @applied_at
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListIssueWorkflowRegressionChangeRecords :many
SELECT * FROM issue_workflow_regression_change_record
WHERE workspace_id = @workspace_id
ORDER BY registry_version ASC;

-- name: CreateIssueWorkflowRegressionRejectedAttempt :one
INSERT INTO issue_workflow_regression_rejected_attempt (
    id, workspace_id, source_issue_id, sequence, registry_version,
    proposal_id, obligation_id, submission_hash, attempt_hash,
    previous_attempt_hash, attempt, attempted_at
) VALUES (
    @id, @workspace_id, @source_issue_id, @sequence, @registry_version,
    @proposal_id, @obligation_id, @submission_hash, @attempt_hash,
    @previous_attempt_hash, @attempt, @attempted_at
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListIssueWorkflowRegressionRejectedAttempts :many
SELECT * FROM issue_workflow_regression_rejected_attempt
WHERE workspace_id = @workspace_id
ORDER BY sequence ASC;
