INSERT INTO inbox_item (
    workspace_id, recipient_type, recipient_id,
    type, severity, issue_id, title, body,
    actor_type, details, id
)
SELECT
    issue.workspace_id,
    'member',
    recipient.user_id,
    'workflow_owner_decision',
    'action_required',
    issue.id,
    issue.title,
    issue.metadata->>'recovery.wait_reason',
    'system',
    jsonb_build_object(
        'owner_relevant', 'true',
        'owner_category', 'needs_you',
        'required_action', 'respond',
        'recovery_owner_wait', 'true',
        'recovery_wait_kind', issue.metadata->>'recovery.wait_kind',
        'question', issue.metadata->>'recovery.wait_reason',
        'consequence', COALESCE(
            NULLIF(btrim(issue.metadata->>'recovery.next_action'), ''),
            'This issue stays paused until the owner answers; the existing work remains attached to the same issue.'
        )
    ),
    md5('multica-recovery-owner-wait-inbox:' || issue.id::text)::uuid
FROM issue
LEFT JOIN issue_status status_entry
  ON status_entry.workspace_id = issue.workspace_id
 AND status_entry.key = issue.status
LEFT JOIN agent creator_agent
  ON issue.creator_type = 'agent'
 AND creator_agent.id = issue.creator_id
 AND creator_agent.workspace_id = issue.workspace_id
JOIN member recipient
  ON recipient.workspace_id = issue.workspace_id
 AND recipient.user_id = CASE
    WHEN issue.creator_type = 'member' THEN issue.creator_id
    WHEN issue.creator_type = 'agent' THEN creator_agent.owner_id
    ELSE NULL
 END
WHERE issue.creator_id IS NOT NULL
  AND COALESCE(status_entry.category, issue.status) = 'blocked'
  AND (
    issue.metadata->>'recovery.wait_kind' = 'owner'
    OR issue.metadata->>'recovery.wait_kind' LIKE 'owner\_%' ESCAPE '\'
  )
  AND btrim(COALESCE(issue.metadata->>'recovery.wait_reason', '')) <> ''
  AND NOT EXISTS (
    SELECT 1 FROM issue_current_picture picture
    WHERE picture.workspace_id = issue.workspace_id
      AND picture.issue_id = issue.id
  )
ON CONFLICT (id) DO UPDATE SET
    recipient_id = EXCLUDED.recipient_id,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    details = EXCLUDED.details;
