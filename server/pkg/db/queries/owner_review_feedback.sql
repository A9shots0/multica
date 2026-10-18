-- name: CreateOwnerReviewFeedback :one
INSERT INTO owner_review_feedback (
    id, workspace_id, user_id, issue_id, inbox_item_id,
    source_attachment_id, source_filename, source_content_type,
    marked_screenshot_attachment_id, marks, note
) VALUES (
    sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(issue_id), sqlc.narg(inbox_item_id),
    sqlc.arg(source_attachment_id), sqlc.arg(source_filename), sqlc.arg(source_content_type),
    sqlc.arg(marked_screenshot_attachment_id), sqlc.arg(marks), sqlc.arg(note)
)
RETURNING *;

-- name: ListOwnerReviewFeedbackByIssue :many
SELECT *
FROM owner_review_feedback
WHERE workspace_id = sqlc.arg(workspace_id)
  AND issue_id = sqlc.arg(issue_id)
ORDER BY created_at ASC, id ASC;
