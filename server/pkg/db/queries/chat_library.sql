-- name: GetChatLibrary :one
SELECT * FROM chat_library WHERE workspace_id=$1 AND owner_id=$2;

-- name: EnsureChatLibrary :exec
INSERT INTO chat_library (workspace_id,owner_id) VALUES ($1,$2) ON CONFLICT DO NOTHING;

-- name: LockChatLibrary :one
SELECT * FROM chat_library WHERE workspace_id=$1 AND owner_id=$2 FOR UPDATE;

-- name: SaveChatLibrary :one
UPDATE chat_library SET preferences=$3,revision=revision+1
WHERE workspace_id=$1 AND owner_id=$2 RETURNING *;
