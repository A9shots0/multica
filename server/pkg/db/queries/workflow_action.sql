-- Durable workflow action execution. Global selection is read-only; every
-- mutation is exact and revision/token fenced by its owning transaction.

-- name: GetWorkflowTaskCompletionCandidate :one
SELECT task.id, task.agent_id, task.runtime_id, task.issue_id, task.status, task.result, task.completed_at,
       task.workflow_action_id, task.workflow_operation_id, task.workflow_branch_key,
       receipt.workspace_id
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.issue_id = task.issue_id
 AND receipt.bound_task_id = task.id
WHERE task.id = @task_id;

-- name: CompleteWorkflowTaskExact :one
UPDATE agent_task_queue
SET status = 'completed', completed_at = now(), result = @result
WHERE id = @task_id AND status = 'running'
  AND agent_id = @agent_id AND runtime_id = @runtime_id
  AND issue_id = @issue_id AND workflow_action_id = @workflow_action_id
  AND workflow_operation_id = @workflow_operation_id
  AND workflow_branch_key = @workflow_branch_key
RETURNING id, agent_id, runtime_id, issue_id, status, result, completed_at,
          workflow_action_id, workflow_operation_id, workflow_branch_key;

-- name: ListDueIssueWorkflowActions :many
SELECT * FROM issue_workflow_action_receipt
WHERE action_type IN ('review_correction', 'review_candidate', 'finalize_non_live', 'resume_exact_operation', 'replan_exact_operation', 'deploy_exact_release', 'expose_exact_release', 'verify_runtime_exact_release', 'verify_business_exact_release', 'monitor_exact_release', 'rollback_exact_release', 'verify_restoration_exact_release')
  AND (
    (state IN ('admitted', 'retry_wait', 'reconciling') AND not_before <= @now)
    OR (state = 'leased' AND lease_expires_at <= @now)
  )
ORDER BY CASE WHEN state = 'leased' THEN lease_expires_at ELSE not_before END, created_at, id
LIMIT @batch_size;

-- name: ClaimIssueWorkflowActionExact :one
UPDATE issue_workflow_action_receipt AS receipt
SET state = 'leased', lease_token = @lease_token, lease_expires_at = @lease_expires_at,
    lease_count = lease_count + 1, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision
  AND action_type IN ('review_correction', 'review_candidate', 'finalize_non_live', 'resume_exact_operation', 'replan_exact_operation', 'owner_decision_requested', 'deploy_exact_release', 'expose_exact_release', 'verify_runtime_exact_release', 'verify_business_exact_release', 'monitor_exact_release', 'rollback_exact_release', 'verify_restoration_exact_release')
  AND @lease_expires_at > CAST(@now AS timestamptz)
  AND ((state IN ('admitted', 'retry_wait', 'reconciling') AND not_before <= CAST(@now AS timestamptz))
       OR (state = 'leased' AND lease_expires_at <= CAST(@now AS timestamptz)))
RETURNING *;

-- name: DeferIssueWorkflowActionExact :one
UPDATE issue_workflow_action_receipt
SET state = 'retry_wait', not_before = @not_before, last_error = @last_error,
    lease_token = NULL, lease_expires_at = NULL, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND state = 'leased' AND lease_token = @lease_token
  AND lease_expires_at > @now
RETURNING *;

-- name: BeginIssueWorkflowActionAttempt :one
UPDATE issue_workflow_action_receipt
SET effect_attempt_count = effect_attempt_count + 1, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND state = 'leased' AND lease_token = @lease_token
  AND lease_expires_at > @now AND effect_attempt_count < max_attempts
RETURNING *;

-- name: RenewIssueWorkflowActionLease :one
UPDATE issue_workflow_action_receipt
SET lease_expires_at = @lease_expires_at, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND state = 'leased' AND lease_token = @lease_token
  AND lease_expires_at > @now AND @lease_expires_at > @now AND @lease_expires_at > lease_expires_at
RETURNING *;

-- name: RecordIssueWorkflowActionObservationFailure :one
UPDATE issue_workflow_action_receipt
SET observation_failure_count = observation_failure_count + 1,
    state = CASE WHEN observation_failure_count + 1 >= max_attempts THEN 'dead_letter' ELSE 'reconciling' END,
    reconciliation_state = CASE WHEN observation_failure_count + 1 >= max_attempts THEN 'failed' ELSE 'unknown' END,
    settled_at = CASE WHEN observation_failure_count + 1 >= max_attempts THEN @now ELSE settled_at END,
    last_observed_at = @now,
    not_before = @not_before, last_error = @last_error, lease_token = NULL,
    lease_expires_at = NULL, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND state = 'leased' AND lease_token = @lease_token
  AND lease_expires_at > @now
RETURNING *;

-- name: SettleIssueWorkflowAction :one
UPDATE issue_workflow_action_receipt
SET state = 'settled', reconciliation_state = 'found', last_observed_at = @now,
    effect_result = @effect_result, committed_at = COALESCE(committed_at, @now), settled_at = @now,
    last_error = '', lease_token = NULL, lease_expires_at = NULL,
    revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND state = 'leased' AND lease_token = @lease_token
  AND lease_expires_at > @now
RETURNING *;

-- name: DeadLetterIssueWorkflowActionExact :one
UPDATE issue_workflow_action_receipt
SET state = 'dead_letter', reconciliation_state = 'failed', last_observed_at = @now,
    last_error = @last_error, settled_at = @now, lease_token = NULL, lease_expires_at = NULL,
    revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND state = 'leased' AND lease_token = @lease_token
  AND lease_expires_at > @now AND (effect_attempt_count >= max_attempts OR observation_failure_count >= max_attempts)
RETURNING *;

-- name: GetWorkflowCorrectionTaskSpec :one
SELECT receipt.*, candidate.created_by AS agent_id, execution.branch_key,
       agent.runtime_id, agent.workspace_id AS agent_workspace_id,
       runtime.workspace_id AS runtime_workspace_id
FROM issue_workflow_action_receipt AS receipt
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = receipt.workspace_id AND execution.issue_id = receipt.issue_id
 AND execution.id = receipt.operation_id
JOIN issue_workflow_candidate AS candidate
  ON candidate.workspace_id = receipt.workspace_id AND candidate.issue_id = receipt.issue_id
 AND candidate.id::text = receipt.payload->>'candidate_id'
 AND candidate.created_by::text = receipt.payload->>'correction_worker_id'
 AND candidate.scope_id = execution.scope_id
 AND candidate.scope_revision = execution.scope_revision
 AND execution.branch_key = receipt.payload->>'branch_key'
JOIN issue_workflow_scope AS workflow_scope
  ON workflow_scope.workspace_id = receipt.workspace_id AND workflow_scope.issue_id = receipt.issue_id
 AND workflow_scope.id = candidate.scope_id AND workflow_scope.revision = candidate.scope_revision
JOIN issue_workflow_review AS review
  ON review.workspace_id = receipt.workspace_id AND review.issue_id = receipt.issue_id
 AND review.id::text = receipt.payload->>'review_id'
 AND review.candidate_id = candidate.id AND review.candidate_revision = candidate.revision
 AND review.scope_id = workflow_scope.id AND review.scope_revision = workflow_scope.revision
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = receipt.workspace_id AND execution_head.issue_id = receipt.issue_id
 AND execution_head.concern_type = 'execution' AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision
JOIN issue_workflow_concern_head AS review_head
  ON review_head.workspace_id = receipt.workspace_id AND review_head.issue_id = receipt.issue_id
 AND review_head.concern_type = 'review' AND review_head.source_record_id = review.id
 AND review_head.source_record_revision = 1
JOIN agent ON agent.id = candidate.created_by AND agent.workspace_id = receipt.workspace_id
JOIN agent_runtime AS runtime ON runtime.id = agent.runtime_id AND runtime.workspace_id = receipt.workspace_id
WHERE receipt.id = @id AND receipt.workspace_id = @workspace_id AND receipt.issue_id = @issue_id
  AND receipt.action_type = 'review_correction'
  AND receipt.payload->>'operation_id' = receipt.operation_id::text
  AND jsonb_typeof(receipt.payload->'operation_id') = 'string'
  AND jsonb_typeof(receipt.payload->'branch_key') = 'string'
  AND jsonb_typeof(receipt.payload->'candidate_id') = 'string'
  AND jsonb_typeof(receipt.payload->'correction_worker_id') = 'string'
  AND jsonb_typeof(receipt.payload->'candidate_revision') = 'number'
  AND jsonb_typeof(receipt.payload->'scope_id') = 'string'
  AND jsonb_typeof(receipt.payload->'scope_revision') = 'number'
  AND jsonb_typeof(receipt.payload->'acceptance_version') = 'number'
  AND jsonb_typeof(receipt.payload->'review_id') = 'string'
  AND jsonb_typeof(receipt.payload->'review_revision') = 'number'
  AND receipt.payload->'candidate_revision' = to_jsonb(candidate.revision)
  AND receipt.payload->>'scope_id' = workflow_scope.id::text
  AND receipt.payload->'scope_revision' = to_jsonb(workflow_scope.revision)
  AND receipt.payload->'acceptance_version' = to_jsonb(workflow_scope.acceptance_version)
  AND receipt.payload->'review_revision' = '1'::jsonb
  AND execution.state = 'correction' AND execution_head.state = 'correction'
  AND review.outcome = 'failed' AND review.superseded_at IS NULL AND review_head.state = 'failed'
  AND candidate.superseded_at IS NULL AND agent.archived_at IS NULL;

-- These four locks are deliberately separate so the caller enforces the
-- established task-owner order: workspace -> agent -> issue -> runtime. The
-- agent FOR SHARE lock conflicts with ArchiveAgent's ordinary UPDATE and also
-- revalidates the exact active runtime binding before a correction task exists.
-- name: LockWorkflowCorrectionTaskWorkspace :one
SELECT id FROM workspace WHERE id = @workspace_id FOR KEY SHARE;

-- name: LockWorkflowCorrectionTaskAgent :one
SELECT id FROM agent
WHERE id = @agent_id AND workspace_id = @workspace_id
  AND runtime_id = @runtime_id AND archived_at IS NULL
FOR SHARE;

-- name: LockWorkflowCorrectionTaskIssue :one
SELECT id FROM issue
WHERE id = @issue_id AND workspace_id = @workspace_id
FOR KEY SHARE;

-- name: LockWorkflowCorrectionTaskRuntime :one
SELECT id FROM agent_runtime
WHERE id = @runtime_id AND workspace_id = @workspace_id
FOR KEY SHARE;

-- name: LockWorkflowCorrectionTaskEffect :one
SELECT receipt.*, candidate.created_by AS agent_id, execution.branch_key,
       agent.runtime_id, agent.workspace_id AS agent_workspace_id,
       runtime.workspace_id AS runtime_workspace_id
FROM issue_workflow_action_receipt AS receipt
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = receipt.workspace_id AND execution.issue_id = receipt.issue_id
 AND execution.id = receipt.operation_id
JOIN issue_workflow_candidate AS candidate
  ON candidate.workspace_id = receipt.workspace_id AND candidate.issue_id = receipt.issue_id
 AND candidate.id::text = receipt.payload->>'candidate_id'
 AND candidate.created_by::text = receipt.payload->>'correction_worker_id'
 AND candidate.scope_id = execution.scope_id
 AND candidate.scope_revision = execution.scope_revision
 AND execution.branch_key = receipt.payload->>'branch_key'
JOIN issue_workflow_scope AS workflow_scope
  ON workflow_scope.workspace_id = receipt.workspace_id AND workflow_scope.issue_id = receipt.issue_id
 AND workflow_scope.id = candidate.scope_id AND workflow_scope.revision = candidate.scope_revision
JOIN issue_workflow_review AS review
  ON review.workspace_id = receipt.workspace_id AND review.issue_id = receipt.issue_id
 AND review.id::text = receipt.payload->>'review_id'
 AND review.candidate_id = candidate.id AND review.candidate_revision = candidate.revision
 AND review.scope_id = workflow_scope.id AND review.scope_revision = workflow_scope.revision
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = receipt.workspace_id AND execution_head.issue_id = receipt.issue_id
 AND execution_head.concern_type = 'execution' AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision
JOIN issue_workflow_concern_head AS review_head
  ON review_head.workspace_id = receipt.workspace_id AND review_head.issue_id = receipt.issue_id
 AND review_head.concern_type = 'review' AND review_head.source_record_id = review.id
 AND review_head.source_record_revision = 1
JOIN agent ON agent.id = candidate.created_by AND agent.workspace_id = receipt.workspace_id
JOIN agent_runtime AS runtime ON runtime.id = agent.runtime_id AND runtime.workspace_id = receipt.workspace_id
WHERE receipt.id = @id AND receipt.workspace_id = @workspace_id AND receipt.issue_id = @issue_id
  AND receipt.action_type = 'review_correction' AND receipt.revision = @expected_revision
  AND receipt.state = 'leased' AND receipt.lease_token = @lease_token
  AND receipt.lease_expires_at > @now
  AND receipt.payload->>'operation_id' = receipt.operation_id::text
  AND jsonb_typeof(receipt.payload->'operation_id') = 'string'
  AND jsonb_typeof(receipt.payload->'branch_key') = 'string'
  AND jsonb_typeof(receipt.payload->'candidate_id') = 'string'
  AND jsonb_typeof(receipt.payload->'correction_worker_id') = 'string'
  AND jsonb_typeof(receipt.payload->'candidate_revision') = 'number'
  AND jsonb_typeof(receipt.payload->'scope_id') = 'string'
  AND jsonb_typeof(receipt.payload->'scope_revision') = 'number'
  AND jsonb_typeof(receipt.payload->'acceptance_version') = 'number'
  AND jsonb_typeof(receipt.payload->'review_id') = 'string'
  AND jsonb_typeof(receipt.payload->'review_revision') = 'number'
  AND receipt.payload->'candidate_revision' = to_jsonb(candidate.revision)
  AND receipt.payload->>'scope_id' = workflow_scope.id::text
  AND receipt.payload->'scope_revision' = to_jsonb(workflow_scope.revision)
  AND receipt.payload->'acceptance_version' = to_jsonb(workflow_scope.acceptance_version)
  AND receipt.payload->'review_revision' = '1'::jsonb
  AND execution.state = 'correction' AND execution_head.state = 'correction'
  AND review.outcome = 'failed' AND review.superseded_at IS NULL AND review_head.state = 'failed'
  AND candidate.superseded_at IS NULL AND agent.archived_at IS NULL
FOR UPDATE OF receipt;

-- name: GetWorkflowCorrectionTaskEffect :one
SELECT id, agent_id, runtime_id, issue_id, status, context, workflow_action_id, workflow_operation_id, workflow_branch_key
FROM agent_task_queue WHERE id = @id AND workflow_action_id = @workflow_action_id;

-- name: GetWorkflowReviewTaskSpec :one
SELECT receipt.*, reviewer.id AS agent_id, reviewer.runtime_id,
       execution.branch_key, candidate.scope_id, candidate.scope_revision
FROM issue_workflow_action_receipt AS receipt
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = receipt.workspace_id AND execution.issue_id = receipt.issue_id
 AND execution.id = receipt.operation_id
JOIN issue_workflow_candidate AS candidate
  ON candidate.workspace_id = receipt.workspace_id AND candidate.issue_id = receipt.issue_id
 AND candidate.id::text = receipt.payload->>'candidate_id'
 AND candidate.revision = (receipt.payload->>'candidate_revision')::bigint
JOIN issue_workflow_review AS review
  ON review.workspace_id = receipt.workspace_id AND review.issue_id = receipt.issue_id
 AND review.id::text = receipt.payload->>'review_id'
 AND (receipt.payload->>'review_revision')::bigint = 1
 AND review.candidate_id = candidate.id AND review.candidate_revision = candidate.revision
 AND review.reviewer_id::text = receipt.payload->>'reviewer_id'
JOIN issue_workflow_scope AS workflow_scope
  ON workflow_scope.workspace_id = candidate.workspace_id AND workflow_scope.issue_id = candidate.issue_id
 AND workflow_scope.id = candidate.scope_id AND workflow_scope.revision = candidate.scope_revision
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = execution.workspace_id AND execution_head.issue_id = execution.issue_id
 AND execution_head.concern_type = 'execution' AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision AND execution_head.state = 'in_review'
JOIN issue_workflow_concern_head AS review_head
  ON review_head.workspace_id = review.workspace_id AND review_head.issue_id = review.issue_id
 AND review_head.concern_type = 'review' AND review_head.source_record_id = review.id
 AND review_head.source_record_revision = 1 AND review_head.state = 'in_progress'
JOIN issue_workflow_concern_head AS evidence_head
  ON evidence_head.workspace_id = receipt.workspace_id AND evidence_head.issue_id = receipt.issue_id
 AND evidence_head.concern_type = 'evidence' AND evidence_head.state = 'current'
JOIN agent AS reviewer
  ON reviewer.id = review.reviewer_id AND reviewer.workspace_id = receipt.workspace_id
 AND reviewer.archived_at IS NULL AND reviewer.id <> candidate.created_by
JOIN agent_runtime AS runtime
  ON runtime.id = reviewer.runtime_id AND runtime.workspace_id = receipt.workspace_id
WHERE receipt.id = @id AND receipt.workspace_id = @workspace_id AND receipt.issue_id = @issue_id
  AND receipt.action_type = 'review_candidate'
  AND receipt.payload->>'operation_id' = receipt.operation_id::text
  AND receipt.payload->>'branch_key' = execution.branch_key
  AND execution.state = 'in_review' AND execution.actor_type = 'reviewer'
  AND execution.actor_id = reviewer.id AND execution.candidate_id = candidate.id
  AND receipt.payload->>'reviewer_runtime_id' = reviewer.runtime_id::text
  AND receipt.payload->>'scope_id' = candidate.scope_id::text
  AND (receipt.payload->>'scope_revision')::bigint = candidate.scope_revision
  AND (receipt.payload->>'acceptance_version')::bigint = workflow_scope.acceptance_version
  AND jsonb_typeof(receipt.payload->'implementation_evidence_refs') = 'array'
  AND jsonb_array_length(receipt.payload->'implementation_evidence_refs') > 0
  AND evidence_head.source_record_id::text IN (SELECT jsonb_array_elements_text(receipt.payload->'implementation_evidence_refs'))
  AND NOT EXISTS (
    SELECT 1 FROM jsonb_array_elements_text(receipt.payload->'implementation_evidence_refs') AS ref(value)
    WHERE NOT EXISTS (
      SELECT 1 FROM issue_workflow_evidence AS evidence
      WHERE evidence.workspace_id = receipt.workspace_id AND evidence.issue_id = receipt.issue_id
        AND evidence.id::text = ref.value AND evidence.evidence_type = 'implementation'
        AND evidence.candidate_id = candidate.id AND evidence.producer_id = candidate.created_by
        AND evidence.superseded_at IS NULL
        AND (evidence.payload->>'candidate_revision')::bigint = candidate.revision
        AND evidence.payload->>'scope_id' = candidate.scope_id::text
        AND (evidence.payload->>'scope_revision')::bigint = candidate.scope_revision
        AND (evidence.payload->>'acceptance_version')::bigint = workflow_scope.acceptance_version
        AND evidence.payload->>'freshness' = 'current'
    )
  )
  AND candidate.superseded_at IS NULL AND review.superseded_at IS NULL;

-- name: LockWorkflowReviewTaskEffect :one
SELECT receipt.*, reviewer.id AS agent_id, reviewer.runtime_id,
       execution.branch_key, candidate.scope_id, candidate.scope_revision
FROM issue_workflow_action_receipt AS receipt
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = receipt.workspace_id AND execution.issue_id = receipt.issue_id
 AND execution.id = receipt.operation_id
JOIN issue_workflow_candidate AS candidate
  ON candidate.workspace_id = receipt.workspace_id AND candidate.issue_id = receipt.issue_id
 AND candidate.id::text = receipt.payload->>'candidate_id'
 AND candidate.revision = (receipt.payload->>'candidate_revision')::bigint
JOIN issue_workflow_review AS review
  ON review.workspace_id = receipt.workspace_id AND review.issue_id = receipt.issue_id
 AND review.id::text = receipt.payload->>'review_id'
 AND (receipt.payload->>'review_revision')::bigint = 1
 AND review.candidate_id = candidate.id AND review.candidate_revision = candidate.revision
 AND review.reviewer_id::text = receipt.payload->>'reviewer_id'
JOIN issue_workflow_scope AS workflow_scope
  ON workflow_scope.workspace_id = candidate.workspace_id AND workflow_scope.issue_id = candidate.issue_id
 AND workflow_scope.id = candidate.scope_id AND workflow_scope.revision = candidate.scope_revision
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = execution.workspace_id AND execution_head.issue_id = execution.issue_id
 AND execution_head.concern_type = 'execution' AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision AND execution_head.state = 'in_review'
JOIN issue_workflow_concern_head AS review_head
  ON review_head.workspace_id = review.workspace_id AND review_head.issue_id = review.issue_id
 AND review_head.concern_type = 'review' AND review_head.source_record_id = review.id
 AND review_head.source_record_revision = 1 AND review_head.state = 'in_progress'
JOIN issue_workflow_concern_head AS evidence_head
  ON evidence_head.workspace_id = receipt.workspace_id AND evidence_head.issue_id = receipt.issue_id
 AND evidence_head.concern_type = 'evidence' AND evidence_head.state = 'current'
JOIN agent AS reviewer
  ON reviewer.id = review.reviewer_id AND reviewer.workspace_id = receipt.workspace_id
 AND reviewer.archived_at IS NULL AND reviewer.id <> candidate.created_by
JOIN agent_runtime AS runtime
  ON runtime.id = reviewer.runtime_id AND runtime.workspace_id = receipt.workspace_id
WHERE receipt.id = @id AND receipt.workspace_id = @workspace_id AND receipt.issue_id = @issue_id
  AND receipt.revision = @expected_revision AND receipt.state = 'leased'
  AND receipt.lease_token = @lease_token AND receipt.lease_expires_at > @now
  AND receipt.action_type = 'review_candidate'
  AND receipt.payload->>'operation_id' = receipt.operation_id::text
  AND receipt.payload->>'branch_key' = execution.branch_key
  AND execution.state = 'in_review' AND execution.actor_type = 'reviewer'
  AND execution.actor_id = reviewer.id AND execution.candidate_id = candidate.id
  AND receipt.payload->>'reviewer_runtime_id' = reviewer.runtime_id::text
  AND receipt.payload->>'scope_id' = candidate.scope_id::text
  AND (receipt.payload->>'scope_revision')::bigint = candidate.scope_revision
  AND (receipt.payload->>'acceptance_version')::bigint = workflow_scope.acceptance_version
  AND jsonb_typeof(receipt.payload->'implementation_evidence_refs') = 'array'
  AND jsonb_array_length(receipt.payload->'implementation_evidence_refs') > 0
  AND evidence_head.source_record_id::text IN (SELECT jsonb_array_elements_text(receipt.payload->'implementation_evidence_refs'))
  AND NOT EXISTS (
    SELECT 1 FROM jsonb_array_elements_text(receipt.payload->'implementation_evidence_refs') AS ref(value)
    WHERE NOT EXISTS (
      SELECT 1 FROM issue_workflow_evidence AS evidence
      WHERE evidence.workspace_id = receipt.workspace_id AND evidence.issue_id = receipt.issue_id
        AND evidence.id::text = ref.value AND evidence.evidence_type = 'implementation'
        AND evidence.candidate_id = candidate.id AND evidence.producer_id = candidate.created_by
        AND evidence.superseded_at IS NULL
        AND (evidence.payload->>'candidate_revision')::bigint = candidate.revision
        AND evidence.payload->>'scope_id' = candidate.scope_id::text
        AND (evidence.payload->>'scope_revision')::bigint = candidate.scope_revision
        AND (evidence.payload->>'acceptance_version')::bigint = workflow_scope.acceptance_version
        AND evidence.payload->>'freshness' = 'current'
    )
  )
  AND candidate.superseded_at IS NULL AND review.superseded_at IS NULL
FOR UPDATE OF receipt;

-- name: CreateWorkflowCorrectionTask :one
INSERT INTO agent_task_queue (
  id, agent_id, runtime_id, issue_id, status, priority, context,
  originator_source, trigger_evidence_kind, trigger_evidence_ref_id,
  workflow_action_id, workflow_operation_id, workflow_branch_key
) VALUES (
  @id, @agent_id, @runtime_id, @issue_id, 'queued', @priority, @context,
  'workflow_action', 'workflow_action', @workflow_action_id,
  @workflow_action_id, @workflow_operation_id, @workflow_branch_key
)
ON CONFLICT (workflow_action_id) WHERE workflow_action_id IS NOT NULL DO UPDATE
SET workflow_action_id = agent_task_queue.workflow_action_id
WHERE agent_task_queue.id = EXCLUDED.id
  AND agent_task_queue.agent_id = EXCLUDED.agent_id
  AND agent_task_queue.runtime_id = EXCLUDED.runtime_id
  AND agent_task_queue.issue_id = EXCLUDED.issue_id
  AND agent_task_queue.context = EXCLUDED.context
  AND agent_task_queue.workflow_operation_id = EXCLUDED.workflow_operation_id
  AND agent_task_queue.workflow_branch_key = EXCLUDED.workflow_branch_key
RETURNING id, agent_id, runtime_id, issue_id, status, context, workflow_action_id, workflow_operation_id, workflow_branch_key;

-- name: GetWorkflowReleaseVerificationTaskSpec :one
SELECT receipt.*, verifier.id AS agent_id, verifier.runtime_id, release.branch_key,
       release.candidate_id, release.candidate_revision, release.scope_id,
       release.scope_revision, release.acceptance_version, release.target_key,
       release.target_generation
FROM issue_workflow_action_receipt AS receipt
JOIN issue_workflow_release AS release
  ON release.workspace_id = receipt.workspace_id AND release.issue_id = receipt.issue_id
 AND release.id = receipt.release_id AND release.superseded_at IS NULL
JOIN issue_workflow_concern_head AS release_head
  ON release_head.workspace_id = release.workspace_id AND release_head.issue_id = release.issue_id
 AND release_head.concern_type = 'release' AND release_head.source_record_id = release.id
 AND release_head.source_record_revision = release.revision AND release_head.state = release.state
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = release.workspace_id AND execution.issue_id = release.issue_id
 AND execution.id = release.operation_id AND execution.superseded_at IS NULL
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = execution.workspace_id AND execution_head.issue_id = execution.issue_id
 AND execution_head.concern_type = 'execution' AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision AND execution_head.state = execution.state
JOIN agent AS verifier
  ON verifier.id = release.planned_verifier_id AND verifier.workspace_id = release.workspace_id
 AND verifier.archived_at IS NULL AND verifier.id <> release.release_operator_id
JOIN agent_runtime AS runtime
  ON runtime.id = verifier.runtime_id AND runtime.workspace_id = release.workspace_id
WHERE receipt.id = @id AND receipt.workspace_id = @workspace_id AND receipt.issue_id = @issue_id
  AND receipt.action_type IN ('verify_runtime_exact_release','verify_business_exact_release','monitor_exact_release','verify_restoration_exact_release')
  AND receipt.operation_id = release.operation_id
  AND receipt.payload->>'operation_id' = release.operation_id::text
  AND receipt.payload->>'branch_key' = release.branch_key
  AND receipt.payload->>'release_id' = release.id::text
  AND (receipt.payload->>'release_revision')::bigint = release.revision
  AND receipt.payload->>'candidate_id' = release.candidate_id::text
  AND (receipt.payload->>'candidate_revision')::bigint = release.candidate_revision
  AND receipt.payload->>'scope_id' = release.scope_id::text
  AND (receipt.payload->>'scope_revision')::bigint = release.scope_revision
  AND (receipt.payload->>'acceptance_version')::bigint = release.acceptance_version
  AND receipt.payload->>'target_key' = release.target_key
  AND (receipt.payload->>'desired_target_generation')::bigint = release.target_generation
  AND receipt.payload->>'planned_verifier_id' = verifier.id::text
  AND ((receipt.action_type = 'verify_runtime_exact_release' AND release.state = 'exposed')
    OR (receipt.action_type = 'verify_business_exact_release' AND release.state = 'production_verifying')
    OR (receipt.action_type = 'monitor_exact_release' AND release.state = 'live')
    OR (receipt.action_type = 'verify_restoration_exact_release' AND release.state = 'rolled_back'));

-- name: LockWorkflowReleaseVerificationTaskEffect :one
SELECT receipt.*, verifier.id AS agent_id, verifier.runtime_id, release.branch_key
FROM issue_workflow_action_receipt AS receipt
JOIN issue_workflow_release AS release
  ON release.workspace_id = receipt.workspace_id AND release.issue_id = receipt.issue_id
 AND release.id = receipt.release_id AND release.superseded_at IS NULL
JOIN issue_workflow_concern_head AS release_head
  ON release_head.workspace_id = release.workspace_id AND release_head.issue_id = release.issue_id
 AND release_head.concern_type = 'release' AND release_head.source_record_id = release.id
 AND release_head.source_record_revision = release.revision AND release_head.state = release.state
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = release.workspace_id AND execution.issue_id = release.issue_id
 AND execution.id = release.operation_id AND execution.superseded_at IS NULL
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = execution.workspace_id AND execution_head.issue_id = execution.issue_id
 AND execution_head.concern_type = 'execution' AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision AND execution_head.state = execution.state
JOIN agent AS verifier
  ON verifier.id = release.planned_verifier_id AND verifier.workspace_id = release.workspace_id
 AND verifier.archived_at IS NULL AND verifier.id <> release.release_operator_id
JOIN agent_runtime AS runtime
  ON runtime.id = verifier.runtime_id AND runtime.workspace_id = release.workspace_id
WHERE receipt.id = @id AND receipt.workspace_id = @workspace_id AND receipt.issue_id = @issue_id
  AND receipt.revision = @expected_revision AND receipt.state = 'leased'
  AND receipt.lease_token = @lease_token AND receipt.lease_expires_at > @now
  AND receipt.action_type IN ('verify_runtime_exact_release','verify_business_exact_release','monitor_exact_release','verify_restoration_exact_release')
  AND receipt.operation_id = release.operation_id
  AND receipt.payload->>'operation_id' = release.operation_id::text
  AND receipt.payload->>'branch_key' = release.branch_key
  AND receipt.payload->>'release_id' = release.id::text
  AND (receipt.payload->>'release_revision')::bigint = release.revision
  AND receipt.payload->>'candidate_id' = release.candidate_id::text
  AND (receipt.payload->>'candidate_revision')::bigint = release.candidate_revision
  AND receipt.payload->>'scope_id' = release.scope_id::text
  AND (receipt.payload->>'scope_revision')::bigint = release.scope_revision
  AND (receipt.payload->>'acceptance_version')::bigint = release.acceptance_version
  AND receipt.payload->>'target_key' = release.target_key
  AND (receipt.payload->>'desired_target_generation')::bigint = release.target_generation
  AND receipt.payload->>'planned_verifier_id' = verifier.id::text
  AND ((receipt.action_type = 'verify_runtime_exact_release' AND release.state = 'exposed')
    OR (receipt.action_type = 'verify_business_exact_release' AND release.state = 'production_verifying')
    OR (receipt.action_type = 'monitor_exact_release' AND release.state = 'live')
    OR (receipt.action_type = 'verify_restoration_exact_release' AND release.state = 'rolled_back'))
FOR UPDATE OF receipt;

-- name: BindWorkflowActionTaskExact :one
UPDATE issue_workflow_action_receipt AS receipt
SET bound_task_id = @task_id, reconciliation_state = 'found', last_observed_at = @now,
    effect_result = jsonb_build_object('task_id', @task_id::text), committed_at = COALESCE(committed_at, @now),
    revision = revision + 1, updated_at = now()
WHERE receipt.workspace_id = @workspace_id AND receipt.issue_id = @issue_id AND receipt.id = @id
  AND receipt.revision = @expected_revision AND receipt.state = 'leased' AND receipt.lease_token = @lease_token
  AND receipt.lease_expires_at > @now AND (receipt.bound_task_id IS NULL OR receipt.bound_task_id = @task_id)
  AND EXISTS (
    SELECT 1 FROM agent_task_queue AS task
    WHERE task.id = @task_id AND task.workflow_action_id = receipt.id
      AND task.workflow_operation_id = receipt.operation_id
      AND task.workflow_branch_key = @workflow_branch_key
      AND task.agent_id = @agent_id AND task.runtime_id = @runtime_id
      AND task.issue_id = receipt.issue_id AND task.status = 'queued'
      AND task.context = @context
  )
RETURNING receipt.*;
