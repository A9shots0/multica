-- name: ListInboxItems :many
SELECT i.*,
       iss.status AS issue_status,
       iss.priority AS issue_priority
FROM inbox_item i
LEFT JOIN issue iss ON iss.id = i.issue_id
LEFT JOIN issue_status iss_status ON iss_status.workspace_id = iss.workspace_id AND iss_status.key = iss.status
WHERE i.workspace_id = $1 AND i.recipient_type = $2 AND i.recipient_id = $3 AND i.archived = false
  AND (
    i.type = 'task_failed'
    OR i.type = 'workflow_owner_decision'
    OR i.details->>'owner_relevant' = 'true'
    OR i.type = 'autopilot_quota_exceeded'
    OR i.type IN ('quick_create_done', 'quick_create_failed', 'quick_create_unconfirmed')
    OR (i.type = 'task_completed' AND i.details->>'independently_verified' = 'true')
    OR i.severity = 'action_required'
  )
  AND (
    i.details->>'recovery_owner_wait' IS DISTINCT FROM 'true'
    OR (
      COALESCE(iss_status.category, iss.status) = 'blocked'
      AND (
        iss.metadata->>'recovery.wait_kind' = 'owner'
        OR iss.metadata->>'recovery.wait_kind' LIKE 'owner\_%' ESCAPE '\'
      )
      AND btrim(COALESCE(iss.metadata->>'recovery.wait_reason', '')) <> ''
      AND NOT EXISTS (
        SELECT 1 FROM issue_current_picture picture
        WHERE picture.workspace_id = iss.workspace_id AND picture.issue_id = iss.id
      )
    )
  )
ORDER BY i.created_at DESC;

-- name: ListArchivedInboxItems :many
-- Archived counterpart of ListInboxItems, backing the inbox's "Archived"
-- sub-view (MUL-3736).
--
-- An issue whose group still has an active row is excluded: archiving is
-- issue-level, so a NEW notification on an already-archived issue leaves the
-- old archived rows in place alongside the fresh active one. The issue belongs
-- in the main inbox at that point, and the two lists must stay mutually
-- exclusive per issue group â€” otherwise the same issue renders in both. The
-- exclusion lives here rather than in the client so neither list depends on
-- the other's cache being loaded. Items without an issue_id group on their own
-- id and can never have an active sibling, hence the IS NULL short-circuit.
--
-- LIMIT applies to ISSUE GROUPS, not raw notification rows. Applying it after
-- the final SELECT lets one noisy issue consume all 200 rows and hide other
-- archived issues, which makes both the visible list and its count too small.
-- The response stays bounded at two rows per group: the newest row the UI
-- renders plus, when different, the newest row carrying a comment anchor. The
-- client already merges those two rows while deduplicating, preserving direct
-- comment landing without sending every historical notification in the group.
-- Keep the materialized working set narrow: full row data is joined only for
-- the final selected ids, not copied for every archived notification scanned.
WITH visible AS MATERIALIZED (
  SELECT i.id, CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END AS group_id, i.created_at, i.details, i.archived
  FROM inbox_item i
  LEFT JOIN issue iss ON iss.id = i.issue_id
  LEFT JOIN issue_status iss_status ON iss_status.workspace_id = iss.workspace_id AND iss_status.key = iss.status
  WHERE i.workspace_id = $1 AND i.recipient_type = $2 AND i.recipient_id = $3
  AND (
    i.type = 'task_failed'
    OR i.type = 'workflow_owner_decision'
    OR i.details->>'owner_relevant' = 'true'
    OR i.type = 'autopilot_quota_exceeded'
    OR i.type IN ('quick_create_done', 'quick_create_failed', 'quick_create_unconfirmed')
    OR (i.type = 'task_completed' AND i.details->>'independently_verified' = 'true')
    OR i.severity = 'action_required'
  )
  AND (
    i.details->>'recovery_owner_wait' IS DISTINCT FROM 'true'
    OR (
      COALESCE(iss_status.category, iss.status) = 'blocked'
      AND (
        iss.metadata->>'recovery.wait_kind' = 'owner'
        OR iss.metadata->>'recovery.wait_kind' LIKE 'owner\_%' ESCAPE '\'
      )
      AND btrim(COALESCE(iss.metadata->>'recovery.wait_reason', '')) <> ''
      AND NOT EXISTS (
        SELECT 1 FROM issue_current_picture picture
        WHERE picture.workspace_id = iss.workspace_id AND picture.issue_id = iss.id
      )
    )
  )

), eligible_archived AS (
  SELECT archived.* FROM visible archived
  WHERE archived.archived = true AND NOT EXISTS (
    SELECT 1 FROM visible active WHERE active.group_id = archived.group_id AND active.archived = false
  )
), newest_groups AS (
  SELECT DISTINCT ON (group_id) group_id, id AS newest_id, created_at AS newest_created_at
  FROM eligible_archived ORDER BY group_id, created_at DESC, id DESC
), limited_groups AS (
  SELECT group_id, newest_id FROM newest_groups
  ORDER BY newest_created_at DESC, newest_id DESC LIMIT 200
), comment_anchors AS (
  SELECT DISTINCT ON (archived.group_id) archived.group_id, archived.id
  FROM eligible_archived archived JOIN limited_groups selected USING (group_id)
  WHERE NULLIF(archived.details->>'comment_id', '') IS NOT NULL
  ORDER BY archived.group_id, archived.created_at DESC, archived.id DESC
), selected_ids AS (
  SELECT newest_id AS id FROM limited_groups UNION SELECT id FROM comment_anchors
)
SELECT i.*, iss.status AS issue_status, iss.priority AS issue_priority
FROM inbox_item i JOIN selected_ids selected ON selected.id = i.id
LEFT JOIN issue iss ON iss.id = i.issue_id
ORDER BY i.created_at DESC, i.id DESC;

-- name: GetInboxItem :one
SELECT * FROM inbox_item
WHERE id = $1;

-- name: GetInboxItemInWorkspace :one
SELECT * FROM inbox_item
WHERE id = $1 AND workspace_id = $2;

-- name: CreateInboxItem :one
INSERT INTO inbox_item (
    workspace_id, recipient_type, recipient_id,
    type, severity, issue_id, title, body,
    actor_type, actor_id, details, id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, COALESCE(sqlc.narg('id')::uuid, gen_random_uuid()))
RETURNING *;

-- name: UpsertExternalWorkInboxItem :one
-- One outside work stream owns one Inbox row. A new receipt reopens and
-- refreshes that row; replaying the same receipt preserves the owner's read
-- and archive state. This is an observation surface, never a control grant.
INSERT INTO inbox_item (
    id, workspace_id, recipient_type, recipient_id,
    type, severity, issue_id, title, body,
    actor_type, details
) VALUES (
    sqlc.arg('id'), sqlc.arg('workspace_id'), 'member', sqlc.arg('recipient_id'),
    'external_work_update', sqlc.arg('severity'), sqlc.arg('issue_id'),
    sqlc.arg('title'), sqlc.arg('body'), 'system', sqlc.arg('details')
)
ON CONFLICT (id) DO UPDATE SET
    recipient_id = EXCLUDED.recipient_id,
    severity = EXCLUDED.severity,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    details = EXCLUDED.details,
    read = CASE
      WHEN inbox_item.recipient_id IS DISTINCT FROM EXCLUDED.recipient_id
        OR inbox_item.severity IS DISTINCT FROM EXCLUDED.severity
        OR inbox_item.title IS DISTINCT FROM EXCLUDED.title
        OR inbox_item.body IS DISTINCT FROM EXCLUDED.body
        OR inbox_item.details IS DISTINCT FROM EXCLUDED.details THEN false
      ELSE inbox_item.read
    END,
    archived = CASE
      WHEN inbox_item.recipient_id IS DISTINCT FROM EXCLUDED.recipient_id
        OR inbox_item.severity IS DISTINCT FROM EXCLUDED.severity
        OR inbox_item.title IS DISTINCT FROM EXCLUDED.title
        OR inbox_item.body IS DISTINCT FROM EXCLUDED.body
        OR inbox_item.details IS DISTINCT FROM EXCLUDED.details THEN false
      ELSE inbox_item.archived
    END,
    created_at = CASE
      WHEN inbox_item.recipient_id IS DISTINCT FROM EXCLUDED.recipient_id
        OR inbox_item.severity IS DISTINCT FROM EXCLUDED.severity
        OR inbox_item.title IS DISTINCT FROM EXCLUDED.title
        OR inbox_item.body IS DISTINCT FROM EXCLUDED.body
        OR inbox_item.details IS DISTINCT FROM EXCLUDED.details THEN now()
      ELSE inbox_item.created_at
    END
WHERE inbox_item.workspace_id = EXCLUDED.workspace_id
  AND inbox_item.issue_id = EXCLUDED.issue_id
  AND inbox_item.type = 'external_work_update'
RETURNING *;

-- name: UpsertRecoveryOwnerWaitInboxItem :one
-- Compatibility projection for recovery bookkeeping that predates normalized
-- workflow decisions. The stable id makes repeated metadata writes and deploy
-- backfills converge on one row. A changed question/consequence reopens the
-- row; an exact replay preserves its read/archive state.
INSERT INTO inbox_item (
    workspace_id, recipient_type, recipient_id,
    type, severity, issue_id, title, body,
    actor_type, details, id
) VALUES (
    sqlc.arg('workspace_id'), 'member', sqlc.arg('recipient_id'),
    'workflow_owner_decision', 'action_required', sqlc.arg('issue_id'),
    sqlc.arg('title'), sqlc.arg('body'), 'system', sqlc.arg('details'),
    md5('multica-recovery-owner-wait-inbox:' || sqlc.arg('issue_id')::uuid::text)::uuid
)
ON CONFLICT (id) DO UPDATE SET
    recipient_id = EXCLUDED.recipient_id,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    details = EXCLUDED.details,
    read = CASE
      WHEN inbox_item.body IS DISTINCT FROM EXCLUDED.body OR inbox_item.details IS DISTINCT FROM EXCLUDED.details THEN false
      ELSE inbox_item.read
    END,
    archived = CASE
      WHEN inbox_item.body IS DISTINCT FROM EXCLUDED.body OR inbox_item.details IS DISTINCT FROM EXCLUDED.details THEN false
      ELSE inbox_item.archived
    END
RETURNING *;

-- name: MarkInboxRead :one
UPDATE inbox_item SET read = true
WHERE id = $1
RETURNING *;

-- name: MarkInboxUnread :one
-- Exact inverse of MarkInboxRead, and item-level for the same reason it is:
-- the inbox renders one row per issue carrying that group's NEWEST item, and
-- the group's read state is that item's read state. Flipping the whole group
-- unread would resurrect older siblings the user already dealt with and
-- inflate CountUnreadInbox (which counts raw rows), while changing nothing the
-- UI shows.
UPDATE inbox_item SET read = false
WHERE id = $1
RETURNING *;

-- name: ArchiveInboxItem :one
UPDATE inbox_item SET archived = true
WHERE id = $1
RETURNING *;

-- name: CloseInboxItemForOwnerDirective :one
-- Recording a direction from Inbox settles only the exact owner request that
-- carried it. Keep this in the same transaction as the directive/comment so a
-- response can never look sent while the actionable row remains open.
UPDATE inbox_item
SET read = true, archived = true
WHERE id = sqlc.arg(inbox_item_id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND recipient_type = 'member'
  AND recipient_id = sqlc.arg(owner_id)
  AND issue_id = sqlc.arg(issue_id)
  AND archived = false
RETURNING *;

-- name: ArchiveInboxByIssue :execrows
UPDATE inbox_item SET archived = true
WHERE workspace_id = $1 AND recipient_type = $2 AND recipient_id = $3 AND issue_id = $4 AND archived = false;

-- name: UnarchiveInboxItem :one
-- Deliberately does not touch `read`: unarchiving restores an item to the main
-- inbox in the exact read/unread state it was archived in, so restoring an
-- unread item legitimately raises the unread badge again (MUL-3736).
UPDATE inbox_item SET archived = false
WHERE id = $1
RETURNING *;

-- name: UnarchiveInboxByIssue :execrows
-- Issue-level restore, mirroring ArchiveInboxByIssue: archiving one item
-- archives every sibling for the same issue, so unarchiving must bring the
-- whole group back. Leaves `read` untouched for the same reason as above.
UPDATE inbox_item SET archived = false
WHERE workspace_id = $1 AND recipient_type = $2 AND recipient_id = $3 AND issue_id = $4 AND archived = true;

-- name: ArchiveInboxByIssueAndType :many
UPDATE inbox_item SET archived = true
WHERE workspace_id = $1 AND issue_id = $2 AND type = $3 AND archived = false
RETURNING recipient_type, recipient_id;

-- name: ArchiveInboxByDecision :exec
-- A response settles only this owner's exact request, not sibling questions.
UPDATE inbox_item SET archived = true
WHERE workspace_id = sqlc.arg(workspace_id) AND issue_id = sqlc.arg(issue_id)
  AND recipient_type = 'member' AND recipient_id = sqlc.arg(owner_id)
  AND type = 'workflow_owner_decision'
  AND details->>'decision_id' = sqlc.arg(decision_id)::text AND archived = false;

-- name: CountUnreadInbox :one
-- Match the visible groups: only failed attempts collapse by issue. Owner
-- decisions remain independent, and the newest visible row owns read state.
SELECT count(*) FROM (
 SELECT DISTINCT ON (CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END) i.read
 FROM inbox_item i
LEFT JOIN issue iss ON iss.id = i.issue_id
LEFT JOIN issue_status iss_status ON iss_status.workspace_id = iss.workspace_id AND iss_status.key = iss.status
WHERE i.workspace_id = $1 AND i.recipient_type = $2 AND i.recipient_id = $3 AND i.archived = false
  AND (
    i.type = 'task_failed'
    OR i.type = 'workflow_owner_decision'
    OR i.details->>'owner_relevant' = 'true'
    OR i.type = 'autopilot_quota_exceeded'
    OR i.type IN ('quick_create_done', 'quick_create_failed', 'quick_create_unconfirmed')
    OR (i.type = 'task_completed' AND i.details->>'independently_verified' = 'true')
    OR i.severity = 'action_required'
  )
  AND (
    i.details->>'recovery_owner_wait' IS DISTINCT FROM 'true'
    OR (
      COALESCE(iss_status.category, iss.status) = 'blocked'
      AND (
        iss.metadata->>'recovery.wait_kind' = 'owner'
        OR iss.metadata->>'recovery.wait_kind' LIKE 'owner\_%' ESCAPE '\'
      )
      AND btrim(COALESCE(iss.metadata->>'recovery.wait_reason', '')) <> ''
      AND NOT EXISTS (
        SELECT 1 FROM issue_current_picture picture
        WHERE picture.workspace_id = iss.workspace_id AND picture.issue_id = iss.id
      )
    )
  )

 ORDER BY CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END, i.created_at DESC, i.id DESC
) newest WHERE newest.read = false;

-- name: CountUnreadInboxByWorkspace :many
-- Per-workspace unread inbox counts for a recipient member, matching the
-- inbox UI's deduplicated view: notifications are grouped per issue
-- (Linear-style, one row per issue) and an issue counts as unread only when
-- its NEWEST non-archived item is unread. Opening an issue marks just that
-- newest item read, so counting raw unread rows would keep older siblings
-- alive and light the switcher dot for a workspace whose inbox the user sees
-- as empty (MUL-3695). Items without an issue group on their own id. The
-- member join keeps counts scoped to workspaces the user still belongs to,
-- so a stale item left behind in a workspace the user has since left cannot
-- light the dot.
SELECT newest.workspace_id, count(*) AS count
FROM (
    SELECT DISTINCT ON (i.workspace_id, CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END) i.workspace_id, i.read
    FROM inbox_item i
    JOIN member m ON m.workspace_id = i.workspace_id AND m.user_id = i.recipient_id
    LEFT JOIN issue iss ON iss.id = i.issue_id
    LEFT JOIN issue_status iss_status ON iss_status.workspace_id = iss.workspace_id AND iss_status.key = iss.status
    WHERE i.recipient_type = 'member'
      AND i.recipient_id = $1
      AND i.archived = false
      AND (
        i.type = 'task_failed'
        OR i.type = 'workflow_owner_decision'
        OR i.details->>'owner_relevant' = 'true'
        OR i.type = 'autopilot_quota_exceeded'
        OR i.type IN ('quick_create_done', 'quick_create_failed', 'quick_create_unconfirmed')
        OR (i.type = 'task_completed' AND i.details->>'independently_verified' = 'true')
        OR i.severity = 'action_required'
      )
      AND (
        i.details->>'recovery_owner_wait' IS DISTINCT FROM 'true'
        OR (
          COALESCE(iss_status.category, iss.status) = 'blocked'
          AND (
            iss.metadata->>'recovery.wait_kind' = 'owner'
            OR iss.metadata->>'recovery.wait_kind' LIKE 'owner\_%' ESCAPE '\'
          )
          AND btrim(COALESCE(iss.metadata->>'recovery.wait_reason', '')) <> ''
          AND NOT EXISTS (
            SELECT 1 FROM issue_current_picture picture
            WHERE picture.workspace_id = iss.workspace_id AND picture.issue_id = iss.id
          )
        )
      )
    ORDER BY i.workspace_id, CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END, i.created_at DESC, i.id DESC
) newest
WHERE newest.read = false
GROUP BY newest.workspace_id;

-- name: MarkAllInboxRead :execrows
UPDATE inbox_item SET read = true
WHERE workspace_id = $1 AND recipient_type = 'member' AND recipient_id = $2 AND archived = false AND read = false;

-- name: ArchiveAllInbox :execrows
UPDATE inbox_item SET archived = true
WHERE workspace_id = $1 AND recipient_type = 'member' AND recipient_id = $2 AND archived = false;

-- name: ArchiveAllReadInbox :execrows
-- "Read" is the state of the one issue row the inbox renders: the newest
-- active notification in that issue group. Archive every row in groups whose
-- newest row is read, and leave an unread group wholly untouched. Updating raw
-- read rows instead makes an older unread sibling reappear after the newest
-- row is archived (and can archive an older read sibling under an unread row).
WITH newest_groups AS (
    SELECT DISTINCT ON (CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END)
           CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END AS group_id,
           i.read
    FROM inbox_item i
    WHERE i.workspace_id = $1
      AND i.recipient_type = 'member'
      AND i.recipient_id = $2
      AND i.archived = false
    ORDER BY CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END, i.created_at DESC, i.id DESC
), read_groups AS (
    SELECT group_id
    FROM newest_groups
    WHERE read = true
)
UPDATE inbox_item i SET archived = true
FROM read_groups selected
WHERE i.workspace_id = $1
  AND i.recipient_type = 'member'
  AND i.recipient_id = $2
  AND i.archived = false
  AND CASE WHEN i.type = 'task_failed' AND i.issue_id IS NOT NULL THEN i.issue_id ELSE i.id END = selected.group_id;

-- name: ArchiveCompletedInbox :execrows
UPDATE inbox_item i SET archived = true
WHERE i.workspace_id = $1 AND i.recipient_type = 'member' AND i.recipient_id = $2 AND i.archived = false
  AND i.issue_id IN (
    SELECT id FROM issue
    WHERE workspace_id = $1
      AND status = ANY(sqlc.arg('terminal_status_keys')::text[])
  );
-- name: ListOpenOrdinaryTaskQuestions :many
-- A question guards only its still-current ordinary attempt. Invalid reports
-- remain durable history; they never fence a separately authorized recovery.
SELECT notification.* FROM inbox_item notification
JOIN issue i ON i.id = notification.issue_id AND i.workspace_id = notification.workspace_id
JOIN agent_task_queue source ON source.id::text = notification.details->>'source_task_id'
  AND source.issue_id = i.id
JOIN agent a ON a.id = source.agent_id AND a.workspace_id = i.workspace_id
WHERE notification.workspace_id = @workspace_id AND notification.issue_id = ANY(@issue_ids::uuid[])
  AND notification.type = 'workflow_owner_decision'
  AND notification.details->>'question_scope' = 'ordinary_task'
  AND notification.details->>'intake_state' = 'pending'
  AND source.status IN ('running', 'completed')
  AND i.assignee_type = 'agent' AND i.assignee_id = source.agent_id
  AND source.accountable_user_id = notification.recipient_id
  AND EXISTS (SELECT 1 FROM member m WHERE m.workspace_id = i.workspace_id AND m.user_id = notification.recipient_id)
  AND NOT EXISTS (SELECT 1 FROM agent_task_queue newer WHERE newer.issue_id = i.id AND (newer.created_at,newer.id) > (source.created_at,source.id))
  AND NOT EXISTS (SELECT 1 FROM issue_workflow_plan plan WHERE plan.issue_id = i.id AND plan.workspace_id = i.workspace_id)
  AND NOT EXISTS (SELECT 1 FROM issue_current_picture picture WHERE picture.issue_id = i.id AND picture.workspace_id = i.workspace_id)
ORDER BY notification.created_at DESC, notification.id DESC;
