-- Accepted scope ---------------------------------------------------------

-- Every v2 writer must call this in its transaction before admitting a
-- command or concern record. Workspace/issue deletion takes FOR UPDATE on the
-- same owner rows, so a writer either commits before the sweep or observes the
-- missing owner after it; it cannot create an orphan behind the sweep.
-- name: LockIssueWorkflowV2Admission :one
SELECT issue.id
FROM workspace
JOIN issue ON issue.workspace_id = workspace.id
WHERE workspace.id = @workspace_id AND issue.id = @issue_id
FOR KEY SHARE OF workspace, issue;

-- Legacy controllers already hold plan/task/issue locks. Never wait for an
-- archival owner lock here: a waiter could invert workspace -> issue -> plan.
-- Failure aborts the caller's transaction, including the legacy state change.
-- name: LockLegacyProjectionWorkspaceAdmission :one
SELECT id FROM workspace WHERE id = @workspace_id FOR KEY SHARE NOWAIT;

-- name: LockLegacyProjectionIssueAdmission :one
SELECT id FROM issue
WHERE workspace_id = @workspace_id AND id = @issue_id
FOR KEY SHARE NOWAIT;

-- name: LockWorkflowReleaseWorkspaceAdmission :one
SELECT id FROM workspace WHERE id = @workspace_id FOR KEY SHARE;

-- Release admission uses the established archive-safe lock order. Each actor
-- is locked in deterministic UUID order before the issue row is touched.
-- name: LockWorkflowReleaseAdmissionActor :one
SELECT agent.id, agent.runtime_id
FROM agent
WHERE agent.workspace_id = @workspace_id AND agent.id = @agent_id
  AND agent.archived_at IS NULL
  AND agent.runtime_id IS NOT NULL
FOR UPDATE OF agent;

-- name: LockWorkflowReleaseIssueAdmission :one
SELECT id FROM issue
WHERE workspace_id = @workspace_id AND id = @issue_id
FOR KEY SHARE;

-- Runtime rows are acquired only after the issue owner row. This matches the
-- global workspace -> agent -> issue -> runtime order used by archival and
-- task admission and closes a runtime-rebind check/use race.
-- name: LockWorkflowReleaseAdmissionRuntime :one
SELECT id FROM agent_runtime
WHERE workspace_id = @workspace_id AND id = @runtime_id
FOR UPDATE;

-- name: GetWorkflowRuntimeDaemonIdentity :one
SELECT daemon_id FROM agent_runtime
WHERE workspace_id = @workspace_id AND id = @runtime_id;

-- Authoritative concern heads -------------------------------------------

-- name: InitializeIssueWorkflowConcernHead :one
INSERT INTO issue_workflow_concern_head (
    workspace_id, issue_id, concern_type, revision,
    source_record_id, source_record_revision, state
) SELECT
    @workspace_id, @issue_id, @concern_type, 1,
    @source_record_id, 1, @state
WHERE @expected_revision::bigint = 0
  AND @source_record_revision::bigint = 1
ON CONFLICT (workspace_id, issue_id, concern_type) DO UPDATE
SET state = issue_workflow_concern_head.state
WHERE issue_workflow_concern_head.revision = 1
  AND issue_workflow_concern_head.source_record_id = EXCLUDED.source_record_id
  AND issue_workflow_concern_head.source_record_revision = 1
  AND issue_workflow_concern_head.state = EXCLUDED.state
RETURNING *;

-- name: ListIssueWorkflowConcernHeads :many
SELECT * FROM issue_workflow_concern_head
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY concern_type;

-- name: LockIssueWorkflowConcernHead :one
SELECT * FROM issue_workflow_concern_head
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND concern_type = @concern_type
FOR UPDATE;

-- Release admission freezes every authority-bearing source selected by the
-- locked concern heads. These records are locked independently so a revoked
-- grant or superseded review/candidate cannot race action admission.
-- name: LockIssueWorkflowScopeRecord :one
SELECT * FROM issue_workflow_scope
WHERE workspace_id=sqlc.arg('workspace_id') AND issue_id=sqlc.arg('issue_id') AND id=sqlc.arg('id')
FOR UPDATE;

-- name: LockIssueWorkflowAuthorityRecord :one
SELECT * FROM issue_workflow_authority
WHERE workspace_id=sqlc.arg('workspace_id') AND issue_id=sqlc.arg('issue_id') AND id=sqlc.arg('id')
FOR UPDATE;

-- name: LockIssueWorkflowExecutionRecord :one
SELECT * FROM issue_workflow_execution
WHERE workspace_id=sqlc.arg('workspace_id') AND issue_id=sqlc.arg('issue_id') AND id=sqlc.arg('id')
FOR UPDATE;

-- name: LockIssueWorkflowReviewRecord :one
SELECT * FROM issue_workflow_review
WHERE workspace_id=sqlc.arg('workspace_id') AND issue_id=sqlc.arg('issue_id') AND id=sqlc.arg('id')
FOR UPDATE;

-- name: LockIssueWorkflowCandidateRecord :one
SELECT * FROM issue_workflow_candidate
WHERE workspace_id=sqlc.arg('workspace_id') AND issue_id=sqlc.arg('issue_id') AND id=sqlc.arg('id')
FOR UPDATE;

-- name: LockIssueWorkflowEvidenceRecord :one
SELECT * FROM issue_workflow_evidence
WHERE workspace_id=sqlc.arg('workspace_id') AND issue_id=sqlc.arg('issue_id') AND id=sqlc.arg('id')
FOR UPDATE;

-- name: GetIssueWorkflowConcernHead :one
SELECT * FROM issue_workflow_concern_head
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND concern_type = @concern_type;

-- name: AdvanceIssueWorkflowConcernHeadCAS :one
UPDATE issue_workflow_concern_head
SET revision = revision + 1,
    source_record_id = @source_record_id,
    source_record_revision = @source_record_revision,
    state = @state,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND concern_type = @concern_type
  AND revision = @expected_revision
  AND @source_record_revision::bigint > 0
RETURNING *;

-- name: CreateIssueWorkflowScope :one
INSERT INTO issue_workflow_scope (
    id, workspace_id, issue_id, revision, acceptance_version, state,
    parent_scope_id, owner_evidence_id, created_by, statement, impact,
    requires_deployment, source_plan_revision
) VALUES (
    @id, @workspace_id, @issue_id, @revision, @acceptance_version, @state,
    sqlc.narg('parent_scope_id'), @owner_evidence_id, @created_by, @statement, @impact,
    sqlc.narg('requires_deployment'), sqlc.narg('source_plan_revision')
)
RETURNING *;

-- Authoritative business outcome ----------------------------------------

-- name: CreateIssueWorkflowOutcome :one
INSERT INTO issue_workflow_outcome (
    id, workspace_id, issue_id, state, acceptance_version,
    completion_evidence_id, terminal_reason
) VALUES (
    @id, @workspace_id, @issue_id, @state, @acceptance_version,
    sqlc.narg('completion_evidence_id'), @terminal_reason
)
RETURNING *;

-- name: GetLatestIssueWorkflowOutcome :one
SELECT * FROM issue_workflow_outcome
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND superseded_at IS NULL
ORDER BY revision DESC
LIMIT 1;

-- name: UpdateIssueWorkflowOutcomeCAS :one
UPDATE issue_workflow_outcome
SET state = @state,
    acceptance_version = @acceptance_version,
    completion_evidence_id = sqlc.narg('completion_evidence_id'),
    terminal_reason = @terminal_reason,
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND superseded_at IS NULL
RETURNING *;

-- name: GetIssueWorkflowScope :one
SELECT * FROM issue_workflow_scope
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- name: GetLatestIssueWorkflowScope :one
SELECT * FROM issue_workflow_scope
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND superseded_at IS NULL
ORDER BY revision DESC
LIMIT 1;

-- name: SupersedeIssueWorkflowScope :one
UPDATE issue_workflow_scope
SET superseded_by = @superseded_by, superseded_at = @superseded_at
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND superseded_at IS NULL
RETURNING *;

-- Authority --------------------------------------------------------------

-- name: CreateIssueWorkflowAuthority :one
INSERT INTO issue_workflow_authority (
    id, workspace_id, issue_id, revision, scope_id, scope_revision,
    owner_evidence_id, state, envelope, valid_until, created_by
) VALUES (
    @id, @workspace_id, @issue_id, @revision, @scope_id, @scope_revision,
    @owner_evidence_id, @state, @envelope, sqlc.narg('valid_until'), @created_by
)
RETURNING *;

-- name: GetIssueWorkflowAuthority :one
SELECT * FROM issue_workflow_authority
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- name: GetLatestIssueWorkflowAuthority :one
SELECT * FROM issue_workflow_authority
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND superseded_at IS NULL
  AND revoked_at IS NULL
ORDER BY revision DESC
LIMIT 1;

-- name: RevokeIssueWorkflowAuthority :one
UPDATE issue_workflow_authority
SET state = @state,
    revoked_by = @revoked_by,
    revoked_at = @revoked_at,
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND revoked_at IS NULL
RETURNING *;

-- Execution operations ---------------------------------------------------

-- name: PutIssueWorkflowExecutionCheckpointSnapshot :one
INSERT INTO issue_workflow_execution_checkpoint_snapshot (
    id, workspace_id, issue_id, execution_id, execution_revision,
    task_id, actor_id, scope_id, scope_revision, authority_id,
    authority_revision, branch_key, execution_state, checkpoint,
    snapshot_document, previous_revision, previous_snapshot_sha256,
    chain_anchor, snapshot_sha256
) VALUES (
    @id, @workspace_id, @issue_id, @execution_id, @execution_revision,
    sqlc.narg('task_id'), sqlc.narg('actor_id'), @scope_id, @scope_revision,
    sqlc.narg('authority_id'), sqlc.narg('authority_revision'), @branch_key,
    @execution_state, @checkpoint, @snapshot_document,
    sqlc.narg('previous_revision'), sqlc.narg('previous_snapshot_sha256'),
    @chain_anchor, @snapshot_sha256
)
ON CONFLICT (workspace_id, issue_id, execution_id, execution_revision)
DO NOTHING
RETURNING *;

-- name: GetLatestIssueWorkflowExecutionCheckpointSnapshot :one
SELECT * FROM issue_workflow_execution_checkpoint_snapshot
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND execution_id = @execution_id
ORDER BY execution_revision DESC
LIMIT 1;

-- name: GetIssueWorkflowExecutionCheckpointSnapshot :one
SELECT * FROM issue_workflow_execution_checkpoint_snapshot
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND execution_id = @execution_id
  AND execution_revision = @execution_revision;

-- name: CreateIssueWorkflowExecution :one
INSERT INTO issue_workflow_execution (
    id, workspace_id, issue_id, branch_key, operation_type, state,
    scope_id, scope_revision, authority_id, authority_revision,
    candidate_id, target_key, actor_type, actor_id, task_id,
    checkpoint, expected_event, wait_reason, lease_token,
    lease_expires_at, next_action_at, attempt_count
) VALUES (
    @id, @workspace_id, @issue_id, @branch_key, @operation_type, @state,
    @scope_id, @scope_revision, sqlc.narg('authority_id'),
    sqlc.narg('authority_revision'), sqlc.narg('candidate_id'),
    sqlc.narg('target_key'), sqlc.narg('actor_type'), sqlc.narg('actor_id'),
    sqlc.narg('task_id'), @checkpoint, @expected_event, @wait_reason,
    sqlc.narg('lease_token'), sqlc.narg('lease_expires_at'),
    sqlc.narg('next_action_at'), @attempt_count
)
RETURNING *;

-- name: GetIssueWorkflowExecution :one
SELECT * FROM issue_workflow_execution
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- Exactly two rows are sufficient for the checkpoint boundary to reject an
-- ambiguous execution binding. The task/actor predicates prevent a nearby
-- workflow on the same issue from donating its memory to this continuation.
-- name: ListTaskContinuationExecutionCandidates :many
WITH RECURSIVE task_lineage(task_id) AS (
    SELECT @task_id::uuid
    UNION
    SELECT COALESCE(task.retry_of_task_id, task.continuation_of_task_id, task.parent_task_id)
    FROM agent_task_queue task
    JOIN task_lineage lineage ON lineage.task_id = task.id
    WHERE COALESCE(task.retry_of_task_id, task.continuation_of_task_id, task.parent_task_id) IS NOT NULL
)
SELECT execution.* FROM issue_workflow_execution execution
JOIN issue_current_picture picture
  ON picture.workspace_id = execution.workspace_id
 AND picture.issue_id = execution.issue_id
 AND picture.scope_revision = execution.scope_revision
WHERE execution.workspace_id = @workspace_id
  AND execution.issue_id = @issue_id
  AND execution.task_id IN (SELECT task_id FROM task_lineage)
  AND execution.actor_id = @actor_id
  AND execution.superseded_at IS NULL
  AND execution.state NOT IN ('completed', 'failed', 'cancelled', 'superseded')
ORDER BY execution.created_at ASC, execution.id ASC
LIMIT 2;

-- name: ListActiveIssueWorkflowExecutions :many
SELECT * FROM issue_workflow_execution
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND superseded_at IS NULL
  AND state NOT IN ('completed', 'failed', 'cancelled', 'superseded')
ORDER BY created_at ASC, id ASC;

-- name: UpdateIssueWorkflowExecutionCAS :one
UPDATE issue_workflow_execution
SET state = @state,
    authority_id = sqlc.narg('authority_id'),
    authority_revision = sqlc.narg('authority_revision'),
    candidate_id = sqlc.narg('candidate_id'),
    target_key = sqlc.narg('target_key'),
    actor_type = sqlc.narg('actor_type'),
    actor_id = sqlc.narg('actor_id'),
    task_id = sqlc.narg('task_id'),
    checkpoint = @checkpoint,
    expected_event = @expected_event,
    wait_reason = @wait_reason,
    lease_token = sqlc.narg('lease_token'),
    lease_expires_at = sqlc.narg('lease_expires_at'),
    next_action_at = sqlc.narg('next_action_at'),
    attempt_count = @attempt_count,
    terminal_reason = @terminal_reason,
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND superseded_at IS NULL
RETURNING *;

-- Owner decisions --------------------------------------------------------

-- name: CreateIssueWorkflowDecision :one
INSERT INTO issue_workflow_decision (
    id, workspace_id, issue_id, scope_id, scope_revision,
    authority_id, authority_revision, candidate_id, target_key,
    decision_type, state, resolution_state, question, recommendation, consequence,
    options, response_schema, authority_gap, requester_id,
    accountable_owner_id, suspended_operation_id, suspended_execution_state,
    expires_at
) VALUES (
    @id, @workspace_id, @issue_id, @scope_id, @scope_revision,
    sqlc.narg('authority_id'), sqlc.narg('authority_revision'),
    sqlc.narg('candidate_id'), sqlc.narg('target_key'), @decision_type,
    @state, @resolution_state, @question, @recommendation, @consequence, @options,
    @response_schema, @authority_gap, @requester_id, @accountable_owner_id,
    @suspended_operation_id, @suspended_execution_state,
    sqlc.narg('expires_at')
)
RETURNING *;

-- name: GetIssueWorkflowDecision :one
SELECT * FROM issue_workflow_decision
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- name: ListActiveIssueWorkflowDecisions :many
SELECT * FROM issue_workflow_decision
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND (state = 'unanswered' OR resolution_state = 'resuming')
  AND superseded_at IS NULL
ORDER BY created_at ASC, id ASC;

-- name: UpdateIssueWorkflowDecisionStateCAS :one
UPDATE issue_workflow_decision
SET state = @state,
    resolution_state = @resolution_state,
    resolved_at = sqlc.narg('resolved_at'),
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND superseded_at IS NULL
RETURNING *;

-- name: CreateIssueWorkflowDecisionResponse :one
INSERT INTO issue_workflow_decision_response (
    id, workspace_id, issue_id, decision_id, decision_revision,
    response_type, actor_id, idempotency_key, request_hash, selected_options,
    content, scope_change
) VALUES (
    @id, @workspace_id, @issue_id, @decision_id, @decision_revision,
    @response_type, @actor_id, @idempotency_key, @request_hash, @selected_options,
    @content, @scope_change
)
ON CONFLICT (workspace_id, decision_id, idempotency_key) DO UPDATE
SET request_hash = issue_workflow_decision_response.request_hash
WHERE issue_workflow_decision_response.request_hash = EXCLUDED.request_hash
  AND issue_workflow_decision_response.issue_id = EXCLUDED.issue_id
  AND issue_workflow_decision_response.decision_revision = EXCLUDED.decision_revision
  AND issue_workflow_decision_response.response_type = EXCLUDED.response_type
  AND issue_workflow_decision_response.actor_id = EXCLUDED.actor_id
  AND issue_workflow_decision_response.selected_options = EXCLUDED.selected_options
  AND issue_workflow_decision_response.content = EXCLUDED.content
  AND issue_workflow_decision_response.scope_change = EXCLUDED.scope_change
RETURNING *;

-- name: ListIssueWorkflowDecisionResponses :many
SELECT * FROM issue_workflow_decision_response
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND decision_id = @decision_id
ORDER BY created_at ASC, id ASC;

-- Candidates and independent review -------------------------------------

-- name: CreateIssueWorkflowCandidate :one
INSERT INTO issue_workflow_candidate (
    id, workspace_id, issue_id, scope_id, scope_revision, candidate_type,
    state, source_revision, artifact, build_contract, created_by
) VALUES (
    @id, @workspace_id, @issue_id, @scope_id, @scope_revision,
    @candidate_type, @state, @source_revision, @artifact, @build_contract,
    @created_by
)
RETURNING *;

-- name: GetIssueWorkflowCandidate :one
SELECT * FROM issue_workflow_candidate
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- name: ListIssueWorkflowCandidates :many
SELECT * FROM issue_workflow_candidate
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY created_at DESC, id DESC;

-- name: SupersedeIssueWorkflowCandidateCAS :one
UPDATE issue_workflow_candidate
SET superseded_by = @superseded_by, superseded_at = @superseded_at,
    updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND superseded_at IS NULL
RETURNING *;

-- name: UpdateIssueWorkflowExecutionCorrectionCandidateCAS :one
UPDATE issue_workflow_execution
SET state = @state, candidate_id = @candidate_id, actor_type = @actor_type,
    actor_id = @actor_id, task_id = NULL, revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id
  AND revision = @expected_revision AND state = 'correction'
  AND superseded_at IS NULL
RETURNING *;

-- name: LockIssueWorkflowPlanForV2 :one
SELECT * FROM issue_workflow_plan
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
FOR UPDATE;

-- A correction result is authority-bearing only after the exact task is
-- durably terminal and its deterministic correction action is settled. Every
-- immutable failed-review binding is rechecked here before the result can
-- create a replacement candidate.
-- name: LockWorkflowCorrectionResultAuthority :one
SELECT task.id AS task_id, task.result, task.agent_id AS implementer_id,
       task.runtime_id AS implementer_runtime_id, implementer_runtime.daemon_id,
       task.work_dir, task.durable_work_dir, task.branch_name,
       task.workflow_action_id, task.workflow_operation_id AS operation_id,
       task.workflow_branch_key AS branch_key, receipt.action_key,
       (receipt.payload->>'correction_candidate_id')::uuid AS correction_candidate_id,
       (receipt.payload->>'correction_candidate_revision')::bigint AS correction_candidate_revision,
       candidate.id AS rejected_candidate_id, candidate.revision AS rejected_candidate_revision,
       candidate.scope_id, candidate.scope_revision, workflow_scope.acceptance_version,
       review.id AS failed_review_id, review.reviewer_id,
       reviewer_agent.runtime_id AS reviewer_runtime_id,
       review.correction_count, review.max_corrections,
       execution.revision AS execution_revision
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.workspace_id = @workspace_id
 AND receipt.issue_id = @issue_id AND receipt.bound_task_id = task.id
JOIN issue_workflow_candidate AS candidate
  ON candidate.workspace_id = receipt.workspace_id AND candidate.issue_id = receipt.issue_id
 AND candidate.id::text = receipt.payload->>'candidate_id'
 AND candidate.revision = (receipt.payload->>'candidate_revision')::bigint
 AND candidate.created_by = task.agent_id
JOIN issue_workflow_scope AS workflow_scope
  ON workflow_scope.workspace_id = candidate.workspace_id AND workflow_scope.issue_id = candidate.issue_id
 AND workflow_scope.id = candidate.scope_id AND workflow_scope.revision = candidate.scope_revision
JOIN issue_workflow_review AS review
  ON review.workspace_id = receipt.workspace_id AND review.issue_id = receipt.issue_id
 AND review.id::text = receipt.payload->>'review_id'
 AND review.candidate_id = candidate.id AND review.candidate_revision = candidate.revision
 AND review.scope_id = candidate.scope_id AND review.scope_revision = candidate.scope_revision
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = receipt.workspace_id AND execution.issue_id = receipt.issue_id
 AND execution.id = receipt.operation_id AND execution.branch_key = task.workflow_branch_key
JOIN agent AS implementer_agent
  ON implementer_agent.id = task.agent_id AND implementer_agent.workspace_id = receipt.workspace_id
 AND implementer_agent.runtime_id = task.runtime_id AND implementer_agent.archived_at IS NULL
JOIN agent_runtime AS implementer_runtime
  ON implementer_runtime.id = task.runtime_id AND implementer_runtime.workspace_id = receipt.workspace_id
JOIN agent AS reviewer_agent
  ON reviewer_agent.id = review.reviewer_id AND reviewer_agent.workspace_id = receipt.workspace_id
 AND reviewer_agent.archived_at IS NULL AND reviewer_agent.id <> task.agent_id
JOIN agent_runtime AS reviewer_runtime
  ON reviewer_runtime.id = reviewer_agent.runtime_id AND reviewer_runtime.workspace_id = receipt.workspace_id
JOIN issue_workflow_concern_head AS execution_head
  ON execution_head.workspace_id = execution.workspace_id AND execution_head.issue_id = execution.issue_id
 AND execution_head.concern_type = 'execution' AND execution_head.source_record_id = execution.id
 AND execution_head.source_record_revision = execution.revision AND execution_head.state = 'correction'
JOIN issue_workflow_concern_head AS review_head
  ON review_head.workspace_id = review.workspace_id AND review_head.issue_id = review.issue_id
 AND review_head.concern_type = 'review' AND review_head.source_record_id = review.id
 AND review_head.source_record_revision = 1 AND review_head.state = 'failed'
WHERE task.id = @task_id AND task.issue_id = @issue_id
  AND task.status = 'completed'
  AND task.workflow_operation_id = receipt.operation_id
  AND receipt.action_type = 'review_correction' AND receipt.state = 'settled'
  AND receipt.payload->>'branch_key' = task.workflow_branch_key
  AND receipt.payload->>'correction_candidate_revision' = '1'
  AND receipt.payload->>'correction_worker_id' = task.agent_id::text
  AND receipt.payload->>'reviewer_id' = review.reviewer_id::text
  AND receipt.payload->'correction_count' = to_jsonb(review.correction_count)
  AND receipt.payload->'max_corrections' = to_jsonb(review.max_corrections)
  AND candidate.superseded_at IS NULL AND review.superseded_at IS NULL
  AND execution.state = 'correction'
FOR UPDATE OF task, receipt, candidate, review, execution, implementer_agent, implementer_runtime, reviewer_agent, reviewer_runtime;

-- name: LockWorkflowReleaseBuildAuthority :one
SELECT task.id AS task_id, task.result, task.agent_id AS release_operator_id,
       task.runtime_id AS release_operator_runtime_id, receipt.id AS action_id,
       receipt.action_key, receipt.operation_id, task.workflow_branch_key AS branch_key,
       release.id AS release_id, release.revision AS release_revision,
       release.candidate_id, release.candidate_revision, release.scope_id,
       release.scope_revision, release.acceptance_version, release.authority_id,
       release.authority_revision, release.target_key, release.target_generation,
       release.expected_target_release_id, release.expected_target_generation,
       release.planned_verifier_id, release.build_manifest_digest,
       release.artifact_digest, release.runtime_contract_digest
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.workspace_id = @workspace_id
 AND receipt.issue_id = @issue_id AND receipt.bound_task_id = task.id
JOIN issue_workflow_release AS release
  ON release.workspace_id = receipt.workspace_id AND release.issue_id = receipt.issue_id
 AND release.id::text = receipt.payload->>'release_id'
 AND release.revision = (receipt.payload->>'release_revision')::bigint
JOIN issue_workflow_concern_head AS release_head
  ON release_head.workspace_id = release.workspace_id AND release_head.issue_id = release.issue_id
 AND release_head.concern_type = 'release' AND release_head.source_record_id = release.id
 AND release_head.source_record_revision = release.revision AND release_head.state = 'pending'
JOIN agent AS release_operator
  ON release_operator.id = task.agent_id AND release_operator.workspace_id = receipt.workspace_id
 AND release_operator.id = release.release_operator_id AND release_operator.archived_at IS NULL
JOIN agent_runtime AS release_runtime
  ON release_runtime.id = task.runtime_id AND release_runtime.workspace_id = receipt.workspace_id
 AND release_runtime.id = release_operator.runtime_id
WHERE task.id = @task_id AND task.issue_id = @issue_id AND task.status = 'completed'
  AND receipt.action_type = 'prepare_release' AND receipt.state = 'settled'
  AND task.workflow_operation_id = receipt.operation_id
  AND receipt.payload->>'branch_key' = task.workflow_branch_key
  AND receipt.payload->>'release_operator_id' = task.agent_id::text
  AND receipt.payload->>'planned_verifier_id' = release.planned_verifier_id::text
  AND receipt.payload->>'candidate_id' = release.candidate_id::text
  AND (receipt.payload->>'candidate_revision')::bigint = release.candidate_revision
  AND receipt.payload->>'scope_id' = release.scope_id::text
  AND (receipt.payload->>'scope_revision')::bigint = release.scope_revision
  AND (receipt.payload->>'acceptance_version')::bigint = release.acceptance_version
  AND receipt.payload->>'target_key' = release.target_key
  AND (receipt.payload->>'desired_target_generation')::bigint = release.target_generation
  AND release.attempt_contract_version = 3 AND release.state = 'pending'
  AND release.superseded_at IS NULL
FOR UPDATE OF task, receipt, release, release_operator, release_runtime;

-- name: GetWorkflowCorrectionResultRoute :one
SELECT task.id AS task_id, task.agent_id, receipt.workspace_id, receipt.issue_id, task.result
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.issue_id = task.issue_id
 AND receipt.bound_task_id = task.id
WHERE task.id = @task_id AND task.status = 'completed'
  AND receipt.action_type = 'review_correction' AND receipt.state = 'settled';

-- name: GetWorkflowTaskResultRoute :one
SELECT task.id AS task_id, task.agent_id, receipt.workspace_id, receipt.issue_id,
       receipt.action_type, receipt.id AS action_id, receipt.action_key,
       receipt.operation_id, receipt.effect_key, receipt.payload, receipt.reconciliation_state, task.result,
       task.runtime_id, runtime.daemon_id, task.work_dir, task.durable_work_dir, task.branch_name
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.issue_id = task.issue_id
 AND receipt.bound_task_id = task.id
JOIN agent_runtime AS runtime
  ON runtime.id = task.runtime_id AND runtime.workspace_id = receipt.workspace_id
WHERE task.id = @task_id AND task.status = 'completed'
  AND receipt.action_type IN ('review_correction', 'review_candidate', 'verify_runtime_exact_release', 'verify_business_exact_release', 'monitor_exact_release', 'verify_restoration_exact_release')
  AND receipt.state = 'settled';

-- name: LockWorkflowReleaseVerificationResultAuthority :one
SELECT task.id AS task_id, task.result, task.agent_id AS verifier_id,
       receipt.id AS workflow_action_id, receipt.action_key, receipt.operation_id,
       receipt.payload
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.workspace_id = @workspace_id
 AND receipt.issue_id = @issue_id AND receipt.bound_task_id = task.id
JOIN agent AS verifier
  ON verifier.id = task.agent_id AND verifier.workspace_id = receipt.workspace_id
 AND verifier.archived_at IS NULL
JOIN agent_runtime AS runtime
  ON runtime.id = task.runtime_id AND runtime.id = verifier.runtime_id
 AND runtime.workspace_id = receipt.workspace_id
WHERE task.id = @task_id AND task.issue_id = @issue_id AND task.status = 'completed'
  AND receipt.action_type IN ('verify_runtime_exact_release','verify_business_exact_release','monitor_exact_release','verify_restoration_exact_release')
  AND receipt.state = 'settled' AND task.workflow_operation_id = receipt.operation_id
  AND receipt.payload->>'planned_verifier_id' = task.agent_id::text
FOR UPDATE OF task, receipt, verifier, runtime;

-- name: LockWorkflowReviewResultAuthority :one
SELECT task.id AS task_id, task.result, task.agent_id AS reviewer_id,
       receipt.id AS workflow_action_id, receipt.action_key,
       receipt.operation_id, task.workflow_branch_key AS branch_key,
       candidate.id AS candidate_id, candidate.revision AS candidate_revision,
       candidate.scope_id, candidate.scope_revision, workflow_scope.acceptance_version,
       review.id AS review_id, 1::bigint AS review_revision,
       review.correction_count, review.max_corrections
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.workspace_id = @workspace_id
 AND receipt.issue_id = @issue_id AND receipt.bound_task_id = task.id
JOIN issue_workflow_candidate AS candidate
  ON candidate.workspace_id = receipt.workspace_id AND candidate.issue_id = receipt.issue_id
 AND candidate.id::text = receipt.payload->>'candidate_id'
 AND candidate.revision = (receipt.payload->>'candidate_revision')::bigint
JOIN issue_workflow_scope AS workflow_scope
  ON workflow_scope.workspace_id = candidate.workspace_id AND workflow_scope.issue_id = candidate.issue_id
 AND workflow_scope.id = candidate.scope_id AND workflow_scope.revision = candidate.scope_revision
JOIN issue_workflow_review AS review
  ON review.workspace_id = candidate.workspace_id AND review.issue_id = candidate.issue_id
 AND review.id::text = receipt.payload->>'review_id'
 AND (receipt.payload->>'review_revision')::bigint = 1
 AND review.candidate_id = candidate.id AND review.candidate_revision = candidate.revision
 AND review.scope_id = candidate.scope_id AND review.scope_revision = candidate.scope_revision
 AND review.reviewer_id = task.agent_id
JOIN issue_workflow_execution AS execution
  ON execution.workspace_id = receipt.workspace_id AND execution.issue_id = receipt.issue_id
 AND execution.id = receipt.operation_id AND execution.branch_key = task.workflow_branch_key
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
JOIN agent AS reviewer_agent
  ON reviewer_agent.id = task.agent_id AND reviewer_agent.workspace_id = receipt.workspace_id
 AND reviewer_agent.archived_at IS NULL AND reviewer_agent.id <> candidate.created_by
JOIN agent_runtime AS reviewer_runtime
  ON reviewer_runtime.id = task.runtime_id AND reviewer_runtime.id = reviewer_agent.runtime_id
 AND reviewer_runtime.workspace_id = receipt.workspace_id
WHERE task.id = @task_id AND task.issue_id = @issue_id AND task.status = 'completed'
  AND receipt.action_type = 'review_candidate' AND receipt.state = 'settled'
  AND task.workflow_operation_id = receipt.operation_id
  AND receipt.payload->>'branch_key' = task.workflow_branch_key
  AND receipt.payload->>'reviewer_id' = task.agent_id::text
  AND receipt.payload->>'reviewer_runtime_id' = task.runtime_id::text
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
  AND execution.state = 'in_review' AND execution.actor_type = 'reviewer'
  AND execution.actor_id = task.agent_id AND execution.candidate_id = candidate.id
  AND candidate.superseded_at IS NULL AND review.superseded_at IS NULL
FOR UPDATE OF task, receipt, candidate, review, execution, reviewer_agent, reviewer_runtime;

-- name: ListPendingWorkflowCorrectionResults :many
SELECT task.id AS task_id
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.issue_id = task.issue_id
 AND receipt.bound_task_id = task.id
WHERE task.status = 'completed' AND receipt.action_type = 'review_correction'
  AND receipt.state = 'settled'
  AND NOT EXISTS (
    SELECT 1 FROM issue_workflow_command AS command
    WHERE command.workspace_id = receipt.workspace_id AND command.issue_id = receipt.issue_id
      AND command.command_key = 'task/' || task.id::text || '/correction-result'
      AND command.state = 'processed'
  )
ORDER BY task.completed_at, task.id
LIMIT @batch_size;

-- name: ListPendingWorkflowTaskResults :many
SELECT task.id AS task_id
FROM agent_task_queue AS task
JOIN issue_workflow_action_receipt AS receipt
  ON receipt.id = task.workflow_action_id AND receipt.issue_id = task.issue_id
 AND receipt.bound_task_id = task.id
WHERE task.status = 'completed'
  AND receipt.action_type IN ('review_correction', 'review_candidate', 'verify_runtime_exact_release', 'verify_business_exact_release', 'monitor_exact_release', 'verify_restoration_exact_release')
  AND receipt.state = 'settled'
  AND receipt.reconciliation_state <> 'failed'
  AND NOT EXISTS (
    SELECT 1 FROM issue_workflow_command AS command
    WHERE command.workspace_id = receipt.workspace_id AND command.issue_id = receipt.issue_id
      AND command.command_key = 'task/' || task.id::text ||
        CASE receipt.action_type WHEN 'review_correction' THEN '/correction-result'
          WHEN 'review_candidate' THEN '/review-result'
          WHEN 'verify_runtime_exact_release' THEN '/runtime-verification-result'
          WHEN 'verify_business_exact_release' THEN '/business-verification-result'
          WHEN 'monitor_exact_release' THEN '/production-regression-result'
          ELSE '/restoration-verification-result' END
      AND command.state = 'processed'
  )
ORDER BY task.completed_at, task.id
LIMIT @batch_size;

-- name: MarkWorkflowTaskResultReconciliationFailed :one
UPDATE issue_workflow_action_receipt AS receipt
SET reconciliation_state = 'failed',
    last_error = @last_error,
    revision = receipt.revision + 1,
    updated_at = now()
FROM agent_task_queue AS task
WHERE task.id = @task_id
  AND task.workflow_action_id = receipt.id
  AND task.issue_id = receipt.issue_id
  AND receipt.bound_task_id = task.id
  AND receipt.workspace_id = @workspace_id
  AND receipt.issue_id = @issue_id
  AND task.status = 'completed'
  AND receipt.action_type IN ('review_correction', 'review_candidate', 'verify_runtime_exact_release', 'verify_business_exact_release')
  AND receipt.state = 'settled'
  AND receipt.reconciliation_state <> 'failed'
  AND NOT EXISTS (
    SELECT 1 FROM issue_workflow_command AS command
    WHERE command.workspace_id = receipt.workspace_id AND command.issue_id = receipt.issue_id
      AND command.command_key = 'task/' || task.id::text ||
        CASE receipt.action_type WHEN 'review_correction' THEN '/correction-result'
          WHEN 'review_candidate' THEN '/review-result'
          WHEN 'verify_runtime_exact_release' THEN '/runtime-verification-result'
          ELSE '/business-verification-result' END
      AND command.state = 'processed'
  )
RETURNING receipt.*;

-- name: CreateIssueWorkflowReview :one
INSERT INTO issue_workflow_review (
    id, workspace_id, issue_id, candidate_id, candidate_revision,
    scope_id, scope_revision, reviewer_id, outcome, criteria, coverage,
    evidence_refs, gaps, correction_packet, correction_count, max_corrections
) VALUES (
    @id, @workspace_id, @issue_id, @candidate_id, @candidate_revision,
    @scope_id, @scope_revision, @reviewer_id, @outcome, @criteria,
    @coverage, @evidence_refs, @gaps, @correction_packet,
    @correction_count, @max_corrections
)
RETURNING *;

-- name: ListIssueWorkflowReviewsForCandidate :many
SELECT * FROM issue_workflow_review
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND candidate_id = @candidate_id
  AND candidate_revision = @candidate_revision
  AND superseded_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: GetIssueWorkflowReview :one
SELECT * FROM issue_workflow_review
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- Release generations ----------------------------------------------------

-- name: CreateIssueWorkflowRelease :one
INSERT INTO issue_workflow_release (
    id, workspace_id, issue_id, candidate_id, candidate_revision,
    scope_id, scope_revision, authority_id, authority_revision,
    target_key, target_generation, state, facets, recovery_policy,
    exposed_population
) VALUES (
    @id, @workspace_id, @issue_id, @candidate_id, @candidate_revision,
    @scope_id, @scope_revision, @authority_id, @authority_revision,
    @target_key, @target_generation, @state, @facets, @recovery_policy,
    @exposed_population
)
RETURNING *;

-- A release attempt is immutable with respect to the candidate, target,
-- predecessor head, operator/verifier, and recovery identity. State/facets may
-- advance through the existing revision CAS, but these bindings may not drift.
-- name: CreateIssueWorkflowReleaseAttempt :one
INSERT INTO issue_workflow_release (
    id, workspace_id, issue_id, candidate_id, candidate_revision,
    scope_id, scope_revision, authority_id, authority_revision,
    target_key, target_generation, state, facets, recovery_policy,
    exposed_population, attempt_contract_version, acceptance_version, operation_id, branch_key,
    expected_target_release_id, expected_target_generation, release_kind,
    recovery_of_release_id, release_operator_id, planned_verifier_id,
    build_manifest_digest, artifact_digest, runtime_contract_digest,
    authority_capability, authority_evidence_id, recovery_policy_revision
) VALUES (
    @id, @workspace_id, @issue_id, @candidate_id, @candidate_revision,
    @scope_id, @scope_revision, @authority_id, @authority_revision,
    @target_key, @target_generation, @state, @facets, @recovery_policy,
    @exposed_population, 2, @acceptance_version, @operation_id, @branch_key,
    sqlc.narg('expected_target_release_id'), @expected_target_generation,
    @release_kind, sqlc.narg('recovery_of_release_id'), @release_operator_id,
    @planned_verifier_id, @build_manifest_digest, @artifact_digest,
    @runtime_contract_digest, @authority_capability,
    @authority_evidence_id, @recovery_policy_revision
)
RETURNING *;

-- name: GetIssueWorkflowRelease :one
SELECT * FROM issue_workflow_release
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- name: CreateIssueWorkflowRecoveryReleaseAttempt :one
INSERT INTO issue_workflow_release (
    id, workspace_id, issue_id, candidate_id, candidate_revision,
    scope_id, scope_revision, authority_id, authority_revision,
    target_key, target_generation, state, facets, recovery_policy,
    exposed_population, attempt_contract_version, acceptance_version,
    operation_id, branch_key, expected_target_release_id,
    expected_target_generation, release_kind, recovery_of_release_id,
    release_operator_id, planned_verifier_id, build_manifest_digest,
    artifact_digest, runtime_contract_digest, authority_capability,
    authority_evidence_id, recovery_policy_revision,
    authority_envelope_digest, owner_evidence_revision,
    exposure_policy_digest, rollback_target_release_id,
    irreversible_effects_digest, reviewed_review_id,
    reviewed_review_revision
) VALUES (
    @id, @workspace_id, @issue_id, @candidate_id, @candidate_revision,
    @scope_id, @scope_revision, @authority_id, @authority_revision,
    @target_key, @target_generation, 'rolling_back', @facets,
    @recovery_policy, '{}'::jsonb, 3, @acceptance_version,
    @operation_id, @branch_key, @expected_target_release_id,
    @expected_target_generation, 'recovery', @recovery_of_release_id,
    @release_operator_id, @planned_verifier_id, @build_manifest_digest,
    @artifact_digest, @runtime_contract_digest, @authority_capability,
    @authority_evidence_id, @recovery_policy_revision,
    @authority_envelope_digest, @owner_evidence_revision,
    @exposure_policy_digest, @rollback_target_release_id,
    @irreversible_effects_digest, @reviewed_review_id,
    @reviewed_review_revision
)
RETURNING *;

-- name: GetIssueWorkflowReleaseByTargetGeneration :one
SELECT * FROM issue_workflow_release
WHERE workspace_id = @workspace_id
  AND target_key = @target_key
  AND target_generation = @target_generation;

-- name: GetWorkspaceWorkflowCandidate :one
SELECT * FROM issue_workflow_candidate
WHERE workspace_id = @workspace_id AND id = @id;

-- name: GetWorkspaceWorkflowReview :one
SELECT * FROM issue_workflow_review
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListWorkspaceWorkflowReleaseEvidence :many
SELECT * FROM issue_workflow_evidence
WHERE workspace_id = @workspace_id
  AND concern_type = 'release'
  AND concern_id = @release_id
  AND superseded_at IS NULL
ORDER BY observed_at DESC, id DESC;

-- name: ListIssueWorkflowReleases :many
SELECT * FROM issue_workflow_release
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY created_at DESC, id DESC;

-- name: UpdateIssueWorkflowReleaseCAS :one
UPDATE issue_workflow_release
SET state = @state,
    facets = @facets,
    exposed_population = @exposed_population,
    started_at = sqlc.narg('started_at'),
    deployed_at = sqlc.narg('deployed_at'),
    restored_at = sqlc.narg('restored_at'),
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND superseded_at IS NULL
RETURNING *;

-- name: RebindPendingIssueWorkflowReleaseCandidateCAS :one
UPDATE issue_workflow_release
SET candidate_id = @candidate_id,
    candidate_revision = @candidate_revision,
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND candidate_id = @expected_candidate_id
  AND candidate_revision = @expected_candidate_revision
  AND state = 'pending'
  AND target_key = ''
  AND target_generation = 0
  AND started_at IS NULL
  AND deployed_at IS NULL
  AND restored_at IS NULL
  AND superseded_at IS NULL
RETURNING *;

-- name: AdmitPendingIssueWorkflowReleaseAttemptCAS :one
UPDATE issue_workflow_release
SET candidate_id = @candidate_id,
    candidate_revision = @candidate_revision,
    scope_id = @scope_id,
    scope_revision = @scope_revision,
    authority_id = @authority_id,
    authority_revision = @authority_revision,
    target_key = @target_key,
    target_generation = @target_generation,
    facets = @facets,
    recovery_policy = @recovery_policy,
    exposed_population = @exposed_population,
    attempt_contract_version = 3,
    acceptance_version = @acceptance_version,
    operation_id = @operation_id,
    branch_key = @branch_key,
    expected_target_release_id = sqlc.narg('expected_target_release_id'),
    expected_target_generation = @expected_target_generation,
    release_kind = 'forward',
    recovery_of_release_id = NULL,
    release_operator_id = @release_operator_id,
    planned_verifier_id = @planned_verifier_id,
    build_manifest_digest = @build_manifest_digest,
    artifact_digest = @artifact_digest,
    runtime_contract_digest = @runtime_contract_digest,
    authority_capability = @authority_capability,
    authority_evidence_id = @authority_evidence_id,
    recovery_policy_revision = @recovery_policy_revision,
    authority_envelope_digest = @authority_envelope_digest,
    owner_evidence_revision = @owner_evidence_revision,
    exposure_policy_digest = @exposure_policy_digest,
    rollback_target_release_id = sqlc.narg('rollback_target_release_id'),
    irreversible_effects_digest = @irreversible_effects_digest,
    reviewed_review_id = @reviewed_review_id,
    reviewed_review_revision = @reviewed_review_revision,
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND state = 'pending'
  AND attempt_contract_version = 1
  AND target_key = ''
  AND target_generation = 0
  AND started_at IS NULL
  AND deployed_at IS NULL
  AND restored_at IS NULL
  AND superseded_at IS NULL
RETURNING *;

-- Evidence and incidents -------------------------------------------------

-- name: CreateIssueWorkflowEvidence :one
INSERT INTO issue_workflow_evidence (
    id, workspace_id, issue_id, concern_type, concern_id,
    concern_revision, evidence_type, level, source, producer_type,
    producer_id, candidate_id, target_key, observed_at, valid_until,
    coverage, limitations, contradictions, payload
) VALUES (
    @id, @workspace_id, @issue_id, @concern_type, sqlc.narg('concern_id'),
    sqlc.narg('concern_revision'), @evidence_type, @level, @source,
    @producer_type, sqlc.narg('producer_id'), sqlc.narg('candidate_id'),
    sqlc.narg('target_key'), @observed_at, sqlc.narg('valid_until'),
    @coverage, @limitations, @contradictions, @payload
)
RETURNING *;

-- name: ListIssueWorkflowEvidence :many
SELECT * FROM issue_workflow_evidence
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND superseded_at IS NULL
ORDER BY observed_at DESC, id DESC;

-- name: CreateIssueWorkflowIncident :one
INSERT INTO issue_workflow_incident (
    id, workspace_id, issue_id, release_id, target_key, incident_type,
    severity, state, consequence, recovery
) VALUES (
    @id, @workspace_id, @issue_id, sqlc.narg('release_id'),
    sqlc.narg('target_key'), @incident_type, @severity, @state,
    @consequence, @recovery
)
RETURNING *;

-- name: CreateIssueWorkflowReleaseIncident :one
INSERT INTO issue_workflow_incident (
    id, workspace_id, issue_id, release_id, release_revision, candidate_id,
    candidate_revision, target_key, target_generation, detection_evidence_id,
    recovery_action_id, compatibility_state, regression_fingerprint,
    recovery_release_id, incident_type, severity, state,
    consequence, recovery
) SELECT
    @id, @workspace_id, @issue_id, @release_id, @release_revision,
    @candidate_id, @candidate_revision, @target_key, @target_generation,
    @detection_evidence_id, sqlc.narg('recovery_action_id'),
    @compatibility_state, @regression_fingerprint,
    sqlc.narg('recovery_release_id'), @incident_type, @severity, @state,
    @consequence, @recovery
WHERE @release_id::uuid IS NOT NULL
  AND @release_revision::bigint IS NOT NULL
  AND @candidate_id::uuid IS NOT NULL
  AND @candidate_revision::bigint IS NOT NULL
  AND @target_key::text IS NOT NULL
  AND btrim(@target_key::text) <> ''
  AND @target_generation::bigint > 0
  AND @detection_evidence_id::uuid IS NOT NULL
  AND btrim(@regression_fingerprint::text) <> ''
RETURNING *;

-- name: ListActiveIssueWorkflowIncidents :many
SELECT * FROM issue_workflow_incident
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND state NOT IN ('resolved', 'cancelled', 'superseded')
  AND superseded_at IS NULL
ORDER BY created_at ASC, id ASC;

-- name: UpdateIssueWorkflowIncidentCAS :one
UPDATE issue_workflow_incident
SET state = @state,
    consequence = @consequence,
    recovery = @recovery,
    acknowledged_at = sqlc.narg('acknowledged_at'),
    resolved_at = sqlc.narg('resolved_at'),
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND superseded_at IS NULL
RETURNING *;

-- name: LockIssueWorkflowRecoveryIncident :one
SELECT * FROM issue_workflow_incident
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
  AND recovery_release_id = @recovery_release_id
  AND regression_fingerprint IS NOT NULL
  AND state NOT IN ('resolved', 'cancelled', 'superseded')
  AND superseded_at IS NULL
FOR UPDATE;

-- Causal commands and promised actions ----------------------------------

-- name: LockWorkflowOwnerCommandAuthority :one
SELECT EXISTS (
    SELECT 1 FROM member
    WHERE workspace_id = @workspace_id AND user_id = @actor_id AND role = 'owner'
    FOR UPDATE
) AS authorized;

-- name: LockWorkflowReviewerCommandAuthority :one
WITH locked_agent AS MATERIALIZED (
    SELECT reviewer_agent.id FROM agent AS reviewer_agent
    WHERE reviewer_agent.id = @actor_id AND reviewer_agent.workspace_id = @workspace_id AND reviewer_agent.archived_at IS NULL
    FOR UPDATE OF reviewer_agent
), locked_task AS MATERIALIZED (
	SELECT task.id, task.agent_id, task.issue_id, task.workflow_action_id,
	       task.workflow_operation_id, task.workflow_branch_key
    FROM agent_task_queue AS task
    JOIN locked_agent ON locked_agent.id = task.agent_id
    WHERE task.id = @task_id AND task.issue_id = @issue_id AND task.status = 'running'
    FOR UPDATE OF task
), locked_action AS MATERIALIZED (
	SELECT receipt.id
	FROM issue_workflow_action_receipt AS receipt
	JOIN locked_task ON locked_task.workflow_action_id = receipt.id
	WHERE receipt.workspace_id = @workspace_id
	  AND receipt.issue_id = @issue_id
	  AND receipt.action_type = 'review_candidate'
	  AND receipt.operation_id = @operation_id
	  AND receipt.state = 'settled'
	  AND receipt.bound_task_id = locked_task.id
	  AND locked_task.workflow_operation_id = receipt.operation_id
	  AND locked_task.workflow_branch_key = receipt.payload->>'branch_key'
	  AND receipt.payload->>'reviewer_id' = @actor_id::text
	  AND receipt.payload->>'candidate_id' = @candidate_id::text
      AND (receipt.payload->>'scope_revision')::bigint = @bound_scope_revision::bigint
    FOR UPDATE OF receipt
)
SELECT EXISTS (SELECT 1 FROM locked_action) AS authorized;

-- name: LockWorkflowFinalizerCommandAuthority :one
WITH locked_action AS MATERIALIZED (
    SELECT id FROM issue_workflow_action_receipt
    WHERE id = @action_id AND workspace_id = @workspace_id AND issue_id = @issue_id
      AND action_key = @action_key AND action_type = 'finalize_non_live'
      AND operation_id = @operation_id AND state = 'leased'
      AND lease_token = @lease_token AND lease_expires_at > @now
      AND payload->>'operation_id' = @operation_id::text
      AND payload->>'candidate_id' = @candidate_id::text
      AND payload->>'requires_deployment' = 'false'
      AND (payload->>'scope_revision')::bigint = sqlc.arg(scope_revision)::bigint
      AND (payload->>'acceptance_version')::bigint = sqlc.arg(acceptance_version)::bigint
      AND (payload->>'source_plan_revision')::bigint = sqlc.arg(source_plan_revision)::bigint
      AND payload->'expected' = sqlc.arg(expected_revisions)::jsonb
    FOR UPDATE
)
SELECT EXISTS (SELECT 1 FROM locked_action) AS authorized;

-- name: CreateIssueWorkflowCommand :one
INSERT INTO issue_workflow_command (
    id, workspace_id, issue_id, command_key, request_hash, command_type, actor_type,
    actor_id, scope_id, scope_revision, authority_id, authority_revision,
    candidate_id, target_key, expected_revisions, payload
) VALUES (
    @id, @workspace_id, @issue_id, @command_key, @request_hash, @command_type, @actor_type,
    sqlc.narg('actor_id'), sqlc.narg('scope_id'), sqlc.narg('scope_revision'),
    sqlc.narg('authority_id'), sqlc.narg('authority_revision'),
    sqlc.narg('candidate_id'), sqlc.narg('target_key'),
    @expected_revisions, @payload
)
ON CONFLICT (workspace_id, issue_id, command_key) DO UPDATE
SET request_hash = issue_workflow_command.request_hash
WHERE issue_workflow_command.request_hash = EXCLUDED.request_hash
  AND issue_workflow_command.issue_id = EXCLUDED.issue_id
  AND issue_workflow_command.command_type = EXCLUDED.command_type
  AND issue_workflow_command.actor_type = EXCLUDED.actor_type
  AND issue_workflow_command.actor_id IS NOT DISTINCT FROM EXCLUDED.actor_id
  AND issue_workflow_command.scope_id IS NOT DISTINCT FROM EXCLUDED.scope_id
  AND issue_workflow_command.scope_revision IS NOT DISTINCT FROM EXCLUDED.scope_revision
  AND issue_workflow_command.authority_id IS NOT DISTINCT FROM EXCLUDED.authority_id
  AND issue_workflow_command.authority_revision IS NOT DISTINCT FROM EXCLUDED.authority_revision
  AND issue_workflow_command.candidate_id IS NOT DISTINCT FROM EXCLUDED.candidate_id
  AND issue_workflow_command.target_key IS NOT DISTINCT FROM EXCLUDED.target_key
  AND issue_workflow_command.expected_revisions = EXCLUDED.expected_revisions
  AND issue_workflow_command.payload = EXCLUDED.payload
RETURNING *;

-- name: GetIssueWorkflowCommandByKey :one
SELECT * FROM issue_workflow_command
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND command_key = @command_key;

-- name: SetIssueWorkflowCommandResult :one
UPDATE issue_workflow_command
SET state = @state,
    result = @result,
    error = @error,
    processed_at = @processed_at,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND state = @expected_state
RETURNING *;

-- name: CreateIssueWorkflowActionReceipt :one
INSERT INTO issue_workflow_action_receipt (
    id, workspace_id, issue_id, action_key, request_hash, command_id, action_type,
    operation_id, decision_id, release_id, notification_id, target_key,
    payload, not_before, max_attempts
) VALUES (
    @id, @workspace_id, @issue_id, @action_key, @request_hash, @command_id, @action_type,
    sqlc.narg('operation_id'), sqlc.narg('decision_id'),
    sqlc.narg('release_id'), sqlc.narg('notification_id'),
    sqlc.narg('target_key'), @payload, @not_before, @max_attempts
)
ON CONFLICT (workspace_id, action_key) DO UPDATE
SET request_hash = issue_workflow_action_receipt.request_hash
WHERE issue_workflow_action_receipt.request_hash = EXCLUDED.request_hash
  AND issue_workflow_action_receipt.issue_id = EXCLUDED.issue_id
  AND issue_workflow_action_receipt.command_id = EXCLUDED.command_id
  AND issue_workflow_action_receipt.action_type = EXCLUDED.action_type
  AND issue_workflow_action_receipt.operation_id IS NOT DISTINCT FROM EXCLUDED.operation_id
  AND issue_workflow_action_receipt.decision_id IS NOT DISTINCT FROM EXCLUDED.decision_id
  AND issue_workflow_action_receipt.release_id IS NOT DISTINCT FROM EXCLUDED.release_id
  AND issue_workflow_action_receipt.notification_id IS NOT DISTINCT FROM EXCLUDED.notification_id
  AND issue_workflow_action_receipt.target_key IS NOT DISTINCT FROM EXCLUDED.target_key
  AND issue_workflow_action_receipt.payload = EXCLUDED.payload
  AND issue_workflow_action_receipt.not_before = EXCLUDED.not_before
  AND issue_workflow_action_receipt.max_attempts = EXCLUDED.max_attempts
RETURNING *;

-- name: GetIssueWorkflowActionReceiptByKey :one
SELECT * FROM issue_workflow_action_receipt
WHERE workspace_id = @workspace_id AND action_key = @action_key;

-- Owner attention --------------------------------------------------------

-- name: CreateIssueWorkflowAttention :one
INSERT INTO issue_workflow_attention (
    id, workspace_id, issue_id, attention_type, decision_id, incident_id,
    consequence, recommendation, context, next_action, read_state,
    acknowledged, resolution_state, action_receipt_id, snoozed_until
) VALUES (
    @id, @workspace_id, @issue_id, @attention_type,
    sqlc.narg('decision_id'), sqlc.narg('incident_id'), @consequence,
    @recommendation, @context, @next_action, @read_state, @acknowledged,
    @resolution_state, sqlc.narg('action_receipt_id'),
    sqlc.narg('snoozed_until')
)
RETURNING *;

-- name: ListActiveIssueWorkflowAttention :many
SELECT * FROM issue_workflow_attention
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND resolution_state IN ('open', 'resuming', 'snoozed')
  AND superseded_at IS NULL
ORDER BY created_at ASC, id ASC;

-- name: GetIssueWorkflowAttention :one
SELECT * FROM issue_workflow_attention
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND id = @id;

-- name: UpdateIssueWorkflowAttentionCAS :one
UPDATE issue_workflow_attention
SET read_state = @read_state,
    acknowledged = @acknowledged,
    resolution_state = @resolution_state,
    action_receipt_id = sqlc.narg('action_receipt_id'),
    snoozed_until = sqlc.narg('snoozed_until'),
    resolved_at = sqlc.narg('resolved_at'),
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
  AND superseded_at IS NULL
RETURNING *;

-- Shared release target head --------------------------------------------

-- The issue admission transaction already locks the workspace first. This
-- target-scoped advisory lock then serializes release attempts from different
-- issues without requiring a pre-existing target-head row.
-- name: LockIssueWorkflowTarget :exec
WITH target_identity AS (
    SELECT sqlc.arg('workspace_id')::uuid AS workspace_id,
           sqlc.arg('target_key')::text AS target_key
)
SELECT pg_advisory_xact_lock(hashtextextended(workspace_id::text || ':' || target_key, 19483271))
FROM target_identity;

-- name: LockIssueWorkflowTargetHead :one
SELECT * FROM issue_workflow_target_head
WHERE workspace_id = @workspace_id AND target_key = @target_key
FOR UPDATE;

-- name: GetIssueWorkflowTargetHead :one
SELECT * FROM issue_workflow_target_head
WHERE workspace_id = @workspace_id AND target_key = @target_key;

-- name: AdvanceIssueWorkflowTargetHeadCAS :one
INSERT INTO issue_workflow_target_head (
    workspace_id, target_key, generation, release_id, issue_id,
    candidate_id, state
) SELECT
    @workspace_id, @target_key, @generation, @release_id, @issue_id,
    @candidate_id, @state
WHERE @expected_generation::bigint = 0
   OR EXISTS (
       SELECT 1 FROM issue_workflow_target_head
       WHERE workspace_id = @workspace_id AND target_key = @target_key
   )
ON CONFLICT (workspace_id, target_key) DO UPDATE SET
    generation = EXCLUDED.generation,
    release_id = EXCLUDED.release_id,
    issue_id = EXCLUDED.issue_id,
    candidate_id = EXCLUDED.candidate_id,
    state = EXCLUDED.state,
    revision = issue_workflow_target_head.revision + 1,
    updated_at = now()
WHERE issue_workflow_target_head.generation = @expected_generation::bigint
  AND EXCLUDED.generation > issue_workflow_target_head.generation
RETURNING *;

-- name: UpdateIssueWorkflowTargetHeadStateCAS :one
UPDATE issue_workflow_target_head
SET state = @state, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND target_key = @target_key
  AND release_id = @release_id AND issue_id = @issue_id
  AND candidate_id = @candidate_id AND generation = @generation
  AND revision = @expected_revision
RETURNING *;

-- name: ListIssueWorkflowTargetHeadsForIssue :many
SELECT head.*
FROM issue_workflow_target_head AS head
WHERE head.workspace_id = @workspace_id
  AND EXISTS (
      SELECT 1
      FROM issue_workflow_release AS release
      WHERE release.workspace_id = head.workspace_id
        AND release.issue_id = @issue_id
        AND release.target_key = head.target_key
  )
ORDER BY head.target_key ASC;

-- Budgets ----------------------------------------------------------------

-- name: CreateIssueWorkflowBudget :one
INSERT INTO issue_workflow_budget (
    id, workspace_id, issue_id, scope_id, concern_type, concern_id,
    state, limits, consumption
) VALUES (
    @id, @workspace_id, @issue_id, @scope_id, @concern_type,
    sqlc.narg('concern_id'), @state, @limits, @consumption
)
RETURNING *;

-- name: ListIssueWorkflowBudgets :many
SELECT * FROM issue_workflow_budget
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY created_at ASC, id ASC;

-- name: UpdateIssueWorkflowBudgetCAS :one
UPDATE issue_workflow_budget
SET state = @state,
    consumption = @consumption,
    exhausted_at = sqlc.narg('exhausted_at'),
    revision = revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND issue_id = @issue_id
  AND id = @id
  AND revision = @expected_revision
RETURNING *;

-- Rebuildable Current Picture -------------------------------------------

-- name: GetIssueCurrentPicture :one
SELECT * FROM issue_current_picture
WHERE workspace_id = @workspace_id AND issue_id = @issue_id;

-- name: ListIssueCurrentPicturesByIssueIDs :many
SELECT workspace_id, issue_id, contract_version, revision, scope_revision,
       authority_revision, outcome_state, outcome_open, execution_state,
       release_state, health_state, attention_state, current_milestone,
       latest_meaningful_change, next_automatic_action, active_actor,
       wait_reason, candidate, target, evidence_summary, unresolved_decisions,
         unresolved_incidents, unresolved_dependencies, terminal_reason,
         next_actor, document, generated_at
FROM issue_current_picture
WHERE workspace_id = @workspace_id
  AND issue_id = ANY(@issue_ids::uuid[])
ORDER BY issue_id;

-- name: UpsertIssueCurrentPicture :one
INSERT INTO issue_current_picture (
    workspace_id, issue_id, contract_version, revision, scope_revision,
    authority_revision, outcome_state, outcome_open, execution_state,
    release_state, health_state, attention_state, current_milestone,
    latest_meaningful_change, next_automatic_action, active_actor,
    wait_reason, candidate, target, evidence_summary, authority_summary,
    unresolved_decisions, unresolved_incidents, unresolved_dependencies,
    budget_summary, supersession_summary, terminal_reason, next_actor,
    source_revisions, document, generated_at
) SELECT
    @workspace_id, @issue_id, @contract_version, @revision,
    sqlc.narg('scope_revision'), sqlc.narg('authority_revision'),
    @outcome_state, @outcome_open, @execution_state, @release_state,
    @health_state, @attention_state, @current_milestone,
    @latest_meaningful_change, @next_automatic_action, @active_actor,
    @wait_reason, @candidate, @target, @evidence_summary,
    @authority_summary, @unresolved_decisions, @unresolved_incidents,
    @unresolved_dependencies, @budget_summary, @supersession_summary,
    @terminal_reason, @next_actor, @source_revisions, @document, @generated_at
WHERE @expected_revision::bigint = 0
   OR EXISTS (
       SELECT 1 FROM issue_current_picture
       WHERE workspace_id = @workspace_id AND issue_id = @issue_id
   )
ON CONFLICT (workspace_id, issue_id) DO UPDATE SET
    contract_version = EXCLUDED.contract_version,
    revision = EXCLUDED.revision,
    scope_revision = EXCLUDED.scope_revision,
    authority_revision = EXCLUDED.authority_revision,
    outcome_state = EXCLUDED.outcome_state,
    outcome_open = EXCLUDED.outcome_open,
    execution_state = EXCLUDED.execution_state,
    release_state = EXCLUDED.release_state,
    health_state = EXCLUDED.health_state,
    attention_state = EXCLUDED.attention_state,
    current_milestone = EXCLUDED.current_milestone,
    latest_meaningful_change = EXCLUDED.latest_meaningful_change,
    next_automatic_action = EXCLUDED.next_automatic_action,
    active_actor = EXCLUDED.active_actor,
    wait_reason = EXCLUDED.wait_reason,
    candidate = EXCLUDED.candidate,
    target = EXCLUDED.target,
    evidence_summary = EXCLUDED.evidence_summary,
    authority_summary = EXCLUDED.authority_summary,
    unresolved_decisions = EXCLUDED.unresolved_decisions,
    unresolved_incidents = EXCLUDED.unresolved_incidents,
    unresolved_dependencies = EXCLUDED.unresolved_dependencies,
    budget_summary = EXCLUDED.budget_summary,
    supersession_summary = EXCLUDED.supersession_summary,
    terminal_reason = EXCLUDED.terminal_reason,
    next_actor = EXCLUDED.next_actor,
    source_revisions = EXCLUDED.source_revisions,
    document = EXCLUDED.document,
    generated_at = EXCLUDED.generated_at,
    updated_at = now()
WHERE issue_current_picture.revision = @expected_revision
  AND EXCLUDED.revision > issue_current_picture.revision
RETURNING *;

-- name: DeleteIssueCurrentPictureForRebuild :execrows
DELETE FROM issue_current_picture
WHERE workspace_id = @workspace_id AND issue_id = @issue_id;

-- name: ListIssueWorkflowProjectionInputs :many
SELECT 'outcome'::text AS record_type, issue_workflow_outcome.id, issue_workflow_outcome.revision, issue_workflow_outcome.created_at, to_jsonb(issue_workflow_outcome) AS payload
FROM issue_workflow_outcome
WHERE issue_workflow_outcome.workspace_id = $1 AND issue_workflow_outcome.issue_id = $2
UNION ALL
SELECT 'scope'::text, issue_workflow_scope.id, issue_workflow_scope.revision, issue_workflow_scope.created_at, to_jsonb(issue_workflow_scope)
FROM issue_workflow_scope
WHERE issue_workflow_scope.workspace_id = $1 AND issue_workflow_scope.issue_id = $2
UNION ALL
SELECT 'authority'::text, issue_workflow_authority.id, issue_workflow_authority.revision, issue_workflow_authority.created_at, to_jsonb(issue_workflow_authority)
FROM issue_workflow_authority
WHERE issue_workflow_authority.workspace_id = $1 AND issue_workflow_authority.issue_id = $2
UNION ALL
SELECT 'execution'::text, issue_workflow_execution.id, issue_workflow_execution.revision, issue_workflow_execution.created_at, to_jsonb(issue_workflow_execution)
FROM issue_workflow_execution
WHERE issue_workflow_execution.workspace_id = $1 AND issue_workflow_execution.issue_id = $2
UNION ALL
SELECT 'decision'::text, issue_workflow_decision.id, issue_workflow_decision.revision, issue_workflow_decision.created_at, to_jsonb(issue_workflow_decision)
FROM issue_workflow_decision
WHERE issue_workflow_decision.workspace_id = $1 AND issue_workflow_decision.issue_id = $2
UNION ALL
SELECT 'candidate'::text, issue_workflow_candidate.id, issue_workflow_candidate.revision, issue_workflow_candidate.created_at, to_jsonb(issue_workflow_candidate)
FROM issue_workflow_candidate
WHERE issue_workflow_candidate.workspace_id = $1 AND issue_workflow_candidate.issue_id = $2
UNION ALL
SELECT 'release'::text, issue_workflow_release.id, issue_workflow_release.revision, issue_workflow_release.created_at, to_jsonb(issue_workflow_release)
FROM issue_workflow_release
WHERE issue_workflow_release.workspace_id = $1 AND issue_workflow_release.issue_id = $2
UNION ALL
SELECT 'incident'::text, issue_workflow_incident.id, issue_workflow_incident.revision, issue_workflow_incident.created_at, to_jsonb(issue_workflow_incident)
FROM issue_workflow_incident
WHERE issue_workflow_incident.workspace_id = $1 AND issue_workflow_incident.issue_id = $2
UNION ALL
SELECT 'decision_response'::text, issue_workflow_decision_response.id, 1::bigint, issue_workflow_decision_response.created_at, to_jsonb(issue_workflow_decision_response)
FROM issue_workflow_decision_response
WHERE issue_workflow_decision_response.workspace_id = $1 AND issue_workflow_decision_response.issue_id = $2
UNION ALL
SELECT 'review'::text, issue_workflow_review.id, 1::bigint, issue_workflow_review.created_at, to_jsonb(issue_workflow_review)
FROM issue_workflow_review
WHERE issue_workflow_review.workspace_id = $1 AND issue_workflow_review.issue_id = $2
UNION ALL
SELECT 'evidence'::text, issue_workflow_evidence.id, 1::bigint, issue_workflow_evidence.created_at, to_jsonb(issue_workflow_evidence)
FROM issue_workflow_evidence
WHERE issue_workflow_evidence.workspace_id = $1 AND issue_workflow_evidence.issue_id = $2
UNION ALL
SELECT 'action_receipt'::text, issue_workflow_action_receipt.id, issue_workflow_action_receipt.revision, issue_workflow_action_receipt.created_at, to_jsonb(issue_workflow_action_receipt)
FROM issue_workflow_action_receipt
WHERE issue_workflow_action_receipt.workspace_id = $1 AND issue_workflow_action_receipt.issue_id = $2
UNION ALL
SELECT 'attention'::text, issue_workflow_attention.id, issue_workflow_attention.revision, issue_workflow_attention.created_at, to_jsonb(issue_workflow_attention)
FROM issue_workflow_attention
WHERE issue_workflow_attention.workspace_id = $1 AND issue_workflow_attention.issue_id = $2
UNION ALL
SELECT 'budget'::text, issue_workflow_budget.id, issue_workflow_budget.revision, issue_workflow_budget.created_at, to_jsonb(issue_workflow_budget)
FROM issue_workflow_budget
WHERE issue_workflow_budget.workspace_id = $1 AND issue_workflow_budget.issue_id = $2
UNION ALL
SELECT 'target_head'::text, issue_workflow_target_head.release_id, issue_workflow_target_head.revision, issue_workflow_target_head.updated_at, to_jsonb(issue_workflow_target_head)
FROM issue_workflow_target_head
WHERE issue_workflow_target_head.workspace_id = $1
  AND EXISTS (
      SELECT 1 FROM issue_workflow_release
      WHERE issue_workflow_release.workspace_id = $1
        AND issue_workflow_release.issue_id = $2
        AND issue_workflow_release.target_key = issue_workflow_target_head.target_key
  )
UNION ALL
SELECT 'consequence'::text, consequence.id, 1::bigint, consequence.created_at, to_jsonb(consequence)
FROM issue_workflow_consequence_receipt AS consequence
WHERE consequence.workspace_id = $1
  AND (consequence.source_issue_id = $2 OR consequence.affected_issue_id = $2)
UNION ALL
SELECT 'regression_change'::text, regression.id, regression.registry_version, regression.created_at, to_jsonb(regression)
FROM issue_workflow_regression_change_record AS regression
WHERE regression.workspace_id = $1 AND regression.source_issue_id = $2
UNION ALL
SELECT 'regression_rejected'::text, rejected.id, rejected.sequence, rejected.created_at, to_jsonb(rejected)
FROM issue_workflow_regression_rejected_attempt AS rejected
WHERE rejected.workspace_id = $1 AND rejected.source_issue_id = $2
ORDER BY created_at ASC, record_type ASC, id ASC;

-- This statement is the issue-delete transaction contract for callers that do
-- not use DeleteIssue. It must run in the same transaction and after locking
-- the workspace-scoped issue row against concurrent workflow admission.
-- name: DeleteIssueWorkflowV2RecordsByIssue :exec
WITH deleted_picture AS (
    DELETE FROM issue_current_picture WHERE workspace_id = $1 AND issue_id = $2
), deleted_concern_head AS (
    DELETE FROM issue_workflow_concern_head WHERE workspace_id = $1 AND issue_id = $2
), deleted_target_head AS (
    DELETE FROM issue_workflow_target_head WHERE workspace_id = $1 AND issue_id = $2
), deleted_attention AS (
    DELETE FROM issue_workflow_attention WHERE workspace_id = $1 AND issue_id = $2
), deleted_budget AS (
    DELETE FROM issue_workflow_budget WHERE workspace_id = $1 AND issue_id = $2
), deleted_action AS (
    DELETE FROM issue_workflow_action_receipt WHERE workspace_id = $1 AND issue_id = $2
), deleted_command AS (
    DELETE FROM issue_workflow_command WHERE workspace_id = $1 AND issue_id = $2
), deleted_incident AS (
    DELETE FROM issue_workflow_incident WHERE workspace_id = $1 AND issue_id = $2
), deleted_evidence AS (
    DELETE FROM issue_workflow_evidence WHERE workspace_id = $1 AND issue_id = $2
), deleted_release AS (
    DELETE FROM issue_workflow_release WHERE workspace_id = $1 AND issue_id = $2
), deleted_review AS (
    DELETE FROM issue_workflow_review WHERE workspace_id = $1 AND issue_id = $2
), deleted_candidate AS (
    DELETE FROM issue_workflow_candidate WHERE workspace_id = $1 AND issue_id = $2
), deleted_response AS (
    DELETE FROM issue_workflow_decision_response WHERE workspace_id = $1 AND issue_id = $2
), deleted_decision AS (
    DELETE FROM issue_workflow_decision WHERE workspace_id = $1 AND issue_id = $2
), deleted_task_context_manifest AS (
    DELETE FROM agent_task_context_manifest WHERE workspace_id = $1 AND issue_id = $2
), deleted_task_usage_sample AS (
    DELETE FROM agent_task_usage_sample WHERE workspace_id = $1 AND issue_id = $2
), deleted_task_loop_observation AS (
    DELETE FROM agent_task_loop_observation WHERE workspace_id = $1 AND issue_id = $2
), deleted_task_continuation_receipt AS (
    DELETE FROM agent_task_continuation_receipt WHERE workspace_id = $1 AND issue_id = $2
), deleted_execution_checkpoint_snapshot AS (
    DELETE FROM issue_workflow_execution_checkpoint_snapshot WHERE workspace_id = $1 AND issue_id = $2
), deleted_execution AS (
    DELETE FROM issue_workflow_execution WHERE workspace_id = $1 AND issue_id = $2
), deleted_authority AS (
    DELETE FROM issue_workflow_authority WHERE workspace_id = $1 AND issue_id = $2
), deleted_outcome AS (
    DELETE FROM issue_workflow_outcome WHERE workspace_id = $1 AND issue_id = $2
)
DELETE FROM issue_workflow_scope AS scope_row
WHERE scope_row.workspace_id = $1 AND scope_row.issue_id = $2;

-- This statement is the workspace-delete transaction contract for callers
-- that do not use DeleteWorkspaceIssueRoots/DeleteWorkspaceSquadsAndSkills.
-- The owning transaction must hold the workspace row lock before this sweep.
-- name: DeleteIssueWorkflowV2RecordsByWorkspace :exec
WITH deleted_picture AS (
    DELETE FROM issue_current_picture AS picture_row WHERE picture_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_concern_head AS (
    DELETE FROM issue_workflow_concern_head AS concern_head_row WHERE concern_head_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_target_head AS (
    DELETE FROM issue_workflow_target_head AS target_head_row WHERE target_head_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_attention AS (
    DELETE FROM issue_workflow_attention AS attention_row WHERE attention_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_budget AS (
    DELETE FROM issue_workflow_budget AS budget_row WHERE budget_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_action AS (
    DELETE FROM issue_workflow_action_receipt AS action_row WHERE action_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_command AS (
    DELETE FROM issue_workflow_command AS command_row WHERE command_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_incident AS (
    DELETE FROM issue_workflow_incident AS incident_row WHERE incident_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_evidence AS (
    DELETE FROM issue_workflow_evidence AS evidence_row WHERE evidence_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_release AS (
    DELETE FROM issue_workflow_release AS release_row WHERE release_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_review AS (
    DELETE FROM issue_workflow_review AS review_row WHERE review_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_candidate AS (
    DELETE FROM issue_workflow_candidate AS candidate_row WHERE candidate_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_response AS (
    DELETE FROM issue_workflow_decision_response AS response_row WHERE response_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_decision AS (
    DELETE FROM issue_workflow_decision AS decision_row WHERE decision_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_task_context_manifest AS (
    DELETE FROM agent_task_context_manifest AS manifest_row WHERE manifest_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_task_usage_sample AS (
    DELETE FROM agent_task_usage_sample AS usage_row WHERE usage_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_task_loop_observation AS (
    DELETE FROM agent_task_loop_observation AS loop_row WHERE loop_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_task_continuation_receipt AS (
    DELETE FROM agent_task_continuation_receipt AS receipt_row WHERE receipt_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_execution_checkpoint_snapshot AS (
    DELETE FROM issue_workflow_execution_checkpoint_snapshot AS snapshot_row WHERE snapshot_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_execution AS (
    DELETE FROM issue_workflow_execution AS execution_row WHERE execution_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_authority AS (
    DELETE FROM issue_workflow_authority AS authority_row WHERE authority_row.workspace_id = sqlc.arg('target_workspace_id')
), deleted_outcome AS (
    DELETE FROM issue_workflow_outcome AS outcome_row WHERE outcome_row.workspace_id = sqlc.arg('target_workspace_id')
)
DELETE FROM issue_workflow_scope AS scope_row WHERE scope_row.workspace_id = sqlc.arg('target_workspace_id');

-- name: DeleteSquadWorkflowTemplateScoped :execrows
DELETE FROM squad_workflow_template
WHERE workspace_id = @workspace_id AND squad_id = @squad_id;

-- name: ListIssueWorkflowMeaningfulActivity :many
WITH meaningful AS (
    SELECT 'outcome'::text AS record_type, o.id, o.revision, o.created_at, to_jsonb(o) AS payload FROM issue_workflow_outcome o WHERE o.workspace_id = @workspace_id AND o.issue_id = @issue_id
    UNION ALL SELECT 'scope', s.id, s.revision, s.created_at, to_jsonb(s) FROM issue_workflow_scope s WHERE s.workspace_id = @workspace_id AND s.issue_id = @issue_id
    UNION ALL SELECT 'authority', a.id, a.revision, a.created_at, to_jsonb(a) FROM issue_workflow_authority a WHERE a.workspace_id = @workspace_id AND a.issue_id = @issue_id
    UNION ALL SELECT 'execution', x.id, x.revision, x.created_at, to_jsonb(x) FROM issue_workflow_execution x WHERE x.workspace_id = @workspace_id AND x.issue_id = @issue_id
    UNION ALL SELECT 'decision', d.id, d.revision, d.created_at, to_jsonb(d) FROM issue_workflow_decision d WHERE d.workspace_id = @workspace_id AND d.issue_id = @issue_id
    UNION ALL SELECT 'decision_response', dr.id, 1::bigint, dr.created_at, to_jsonb(dr) FROM issue_workflow_decision_response dr WHERE dr.workspace_id = @workspace_id AND dr.issue_id = @issue_id
    UNION ALL SELECT 'candidate', c.id, c.revision, c.created_at, to_jsonb(c) FROM issue_workflow_candidate c WHERE c.workspace_id = @workspace_id AND c.issue_id = @issue_id
    UNION ALL SELECT 'review', rv.id, 1::bigint, rv.created_at, to_jsonb(rv) FROM issue_workflow_review rv WHERE rv.workspace_id = @workspace_id AND rv.issue_id = @issue_id
    UNION ALL SELECT 'release', rl.id, rl.revision, rl.created_at, to_jsonb(rl) FROM issue_workflow_release rl WHERE rl.workspace_id = @workspace_id AND rl.issue_id = @issue_id
    UNION ALL SELECT 'evidence', ev.id, 1::bigint, ev.created_at, to_jsonb(ev) FROM issue_workflow_evidence ev WHERE ev.workspace_id = @workspace_id AND ev.issue_id = @issue_id
    UNION ALL SELECT 'incident', i.id, i.revision, i.created_at, to_jsonb(i) FROM issue_workflow_incident i WHERE i.workspace_id = @workspace_id AND i.issue_id = @issue_id
    UNION ALL SELECT 'consequence', consequence.id, 1::bigint, consequence.created_at, to_jsonb(consequence) FROM issue_workflow_consequence_receipt consequence WHERE consequence.workspace_id = @workspace_id AND (consequence.source_issue_id = @issue_id OR consequence.affected_issue_id = @issue_id)
    UNION ALL SELECT 'regression_change', regression.id, regression.registry_version, regression.created_at, to_jsonb(regression) FROM issue_workflow_regression_change_record regression WHERE regression.workspace_id = @workspace_id AND regression.source_issue_id = @issue_id
)
SELECT record_type, id, revision, created_at, payload
FROM meaningful
WHERE sqlc.narg('cursor_time')::timestamptz IS NULL
   OR (created_at, record_type, id, revision) < (
       sqlc.narg('cursor_time')::timestamptz,
       sqlc.narg('cursor_type')::text,
       sqlc.narg('cursor_id')::uuid,
       sqlc.narg('cursor_revision')::bigint
   )
ORDER BY created_at DESC, record_type DESC, id DESC, revision DESC
LIMIT @page_size;
