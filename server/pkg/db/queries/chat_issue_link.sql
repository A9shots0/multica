-- name: LinkChatIssue :one
INSERT INTO chat_issue_link (workspace_id, chat_session_id, issue_id, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (workspace_id, chat_session_id, issue_id)
DO UPDATE SET source = EXCLUDED.source
RETURNING *;

-- name: UnlinkChatIssue :exec
DELETE FROM chat_issue_link
WHERE workspace_id = $1 AND chat_session_id = $2 AND issue_id = $3;

-- name: ListIssuesForChat :many
SELECT i.id, i.number, i.title, i.status, l.source, l.created_at
FROM chat_issue_link AS l
JOIN issue AS i ON i.id = l.issue_id AND i.workspace_id = l.workspace_id
WHERE l.workspace_id = $1 AND l.chat_session_id = $2
ORDER BY l.created_at DESC, i.number;

-- name: ListChatsForIssue :many
SELECT c.id, c.title, c.status, c.updated_at, l.source, l.created_at
FROM chat_issue_link AS l
JOIN chat_session AS c ON c.id = l.chat_session_id AND c.workspace_id = l.workspace_id
WHERE l.workspace_id = $1 AND l.issue_id = $2
ORDER BY l.created_at DESC, c.updated_at DESC;
