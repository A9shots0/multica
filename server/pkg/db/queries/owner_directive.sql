-- name: CreateIssueOwnerDirective :one
INSERT INTO issue_owner_directive (
    id, workspace_id, issue_id, directive_type, expected_issue_revision,
    content, state, actor_id, comment_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListIssueOwnerDirectives :many
SELECT * FROM issue_owner_directive
WHERE workspace_id = $1 AND issue_id = $2
ORDER BY created_at DESC, id DESC;

-- name: GetCurrentIssueOwnerAcceptance :one
WITH latest_directive AS (
    SELECT id, actor_id, directive_type, state, created_at
    FROM issue_owner_directive AS directive
    WHERE directive.workspace_id = $1 AND directive.issue_id = $2
    ORDER BY directive.created_at DESC, directive.id DESC
    LIMIT 1
), latest_outcome AS (
    SELECT id, revision, state, completion_evidence_id, updated_at
    FROM issue_workflow_outcome AS outcome
    WHERE outcome.workspace_id = $1 AND outcome.issue_id = $2 AND outcome.superseded_at IS NULL
    ORDER BY outcome.revision DESC
    LIMIT 1
)
SELECT d.id AS receipt_id, d.actor_id, o.id AS outcome_id, o.revision AS outcome_revision
FROM latest_directive d
CROSS JOIN latest_outcome o
WHERE d.directive_type = 'accept_result'
  AND d.state = 'applied'
  AND o.state = 'satisfied'
  AND o.completion_evidence_id IS NOT NULL
  AND d.created_at >= o.updated_at;
