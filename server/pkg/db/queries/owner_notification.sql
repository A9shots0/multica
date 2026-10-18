-- name: UpsertOwnerNotificationBinding :one
INSERT INTO owner_notification_binding (
    workspace_id, recipient_id, destination_key, destination_fingerprint,
    delivery_mode, enabled
) VALUES (
    @workspace_id, @recipient_id, @destination_key, @destination_fingerprint,
    @delivery_mode, @enabled
)
ON CONFLICT (workspace_id) DO UPDATE
SET recipient_id = EXCLUDED.recipient_id,
    destination_key = EXCLUDED.destination_key,
    destination_fingerprint = EXCLUDED.destination_fingerprint,
    delivery_mode = EXCLUDED.delivery_mode,
    enabled = EXCLUDED.enabled,
    updated_at = now()
RETURNING *;

-- name: GetOwnerNotificationBinding :one
SELECT * FROM owner_notification_binding
WHERE workspace_id = @workspace_id AND enabled = TRUE;

-- name: AdmitOwnerNotification :one
INSERT INTO owner_notification (
    id, workspace_id, issue_id, recipient_id, delivery_mode, event_type,
    event_key, source_type, source_id, source_revision, destination_key,
    message, action_url, payload
) VALUES (
    @id, @workspace_id, @issue_id, @recipient_id, @delivery_mode, @event_type,
    @event_key, @source_type, @source_id, @source_revision, @destination_key,
    @message, @action_url, @payload
)
ON CONFLICT (workspace_id, delivery_mode, event_key) DO NOTHING
RETURNING *;

-- name: GetOwnerNotification :one
SELECT * FROM owner_notification
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListOwnerNotificationsByIssue :many
SELECT * FROM owner_notification
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY admitted_at DESC, id DESC;

-- name: ClaimNextOwnerNotification :one
WITH candidate AS (
    SELECT id
    FROM owner_notification
    WHERE owner_notification.delivery_mode = sqlc.arg('requested_delivery_mode')
      AND owner_notification.state IN ('admitted', 'retryable')
      AND (owner_notification.next_attempt_at IS NULL OR owner_notification.next_attempt_at <= now())
      AND (owner_notification.lease_expires_at IS NULL OR owner_notification.lease_expires_at <= now())
    ORDER BY owner_notification.admitted_at ASC, owner_notification.id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE owner_notification AS notification
SET state = 'claimed',
    lease_token = @lease_token,
    lease_expires_at = @lease_expires_at,
    attempt_count = notification.attempt_count + 1,
    attempted_at = now(),
    updated_at = now()
FROM candidate
WHERE notification.id = candidate.id
RETURNING notification.*;

-- name: RecoverExpiredOwnerNotificationClaims :many
UPDATE owner_notification
SET state = 'ambiguous', provider_state = 'claim_expired_unknown_send_state',
    lease_token = NULL, lease_expires_at = NULL, next_attempt_at = NULL,
    updated_at = now()
WHERE state = 'claimed' AND lease_expires_at <= now()
RETURNING *;

-- name: CreateOwnerNotificationAttempt :one
INSERT INTO owner_notification_attempt (
    id, notification_id, request_key, state, provider_message_id,
    provider_state, error_code, metadata
) VALUES (
    @id, @notification_id, @request_key, @state,
    sqlc.narg('provider_message_id'), sqlc.narg('provider_state'),
    sqlc.narg('error_code'), @metadata
)
ON CONFLICT (notification_id, request_key) DO UPDATE
SET request_key = owner_notification_attempt.request_key
WHERE owner_notification_attempt.state = EXCLUDED.state
RETURNING *;

-- name: CompleteOwnerNotificationDryRun :one
UPDATE owner_notification
SET state = 'delivered', provider_state = 'dry_run_delivered',
    delivered_at = now(), lease_token = NULL, lease_expires_at = NULL,
    next_attempt_at = NULL, updated_at = now()
WHERE id = @id AND state = 'claimed' AND lease_token = @lease_token
RETURNING *;

-- name: RecordOwnerNotificationProviderAccepted :one
UPDATE owner_notification
SET state = 'provider_accepted', provider_message_id = @provider_message_id,
    provider_state = @provider_state, provider_accepted_at = now(),
    lease_token = NULL, lease_expires_at = NULL, next_attempt_at = @next_attempt_at,
    updated_at = now()
WHERE id = @id AND state = 'claimed' AND lease_token = @lease_token
RETURNING *;

-- name: ClaimNextOwnerNotificationDeliveryCheck :one
WITH candidate AS (
    SELECT id
    FROM owner_notification
    WHERE owner_notification.delivery_mode = sqlc.arg('requested_delivery_mode')
      AND owner_notification.state = 'provider_accepted'
      AND owner_notification.provider_message_id IS NOT NULL
      AND owner_notification.next_attempt_at <= now()
      AND (owner_notification.lease_expires_at IS NULL OR owner_notification.lease_expires_at <= now())
    ORDER BY owner_notification.next_attempt_at ASC, owner_notification.id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE owner_notification AS notification
SET lease_token = @lease_token, lease_expires_at = @lease_expires_at,
    delivery_check_count = notification.delivery_check_count + 1,
    updated_at = now()
FROM candidate
WHERE notification.id = candidate.id
RETURNING notification.*;

-- name: DeferOwnerNotificationDeliveryCheck :one
UPDATE owner_notification
SET provider_state = @provider_state, next_attempt_at = @next_attempt_at,
    lease_token = NULL, lease_expires_at = NULL, updated_at = now()
WHERE id = @id AND state = 'provider_accepted' AND lease_token = @lease_token
RETURNING *;

-- name: FailOwnerNotificationDeliveryCheck :one
UPDATE owner_notification
SET state = 'failed', provider_state = @provider_state,
    lease_token = NULL, lease_expires_at = NULL, next_attempt_at = NULL,
    updated_at = now()
WHERE id = @id AND state = 'provider_accepted' AND lease_token = @lease_token
RETURNING *;

-- name: StopOwnerNotificationDeliveryChecks :one
UPDATE owner_notification
SET state = 'ambiguous', provider_state = @provider_state,
    lease_token = NULL, lease_expires_at = NULL, next_attempt_at = NULL,
    updated_at = now()
WHERE id = @id AND state = 'provider_accepted' AND lease_token = @lease_token
RETURNING *;

-- name: RecordOwnerNotificationFailure :one
UPDATE owner_notification
SET state = @state, provider_state = @provider_state,
    lease_token = NULL, lease_expires_at = NULL,
    next_attempt_at = sqlc.narg('next_attempt_at'), updated_at = now()
WHERE id = @id AND state = 'claimed' AND lease_token = @lease_token
  AND @state IN ('ambiguous', 'retryable', 'rejected', 'failed')
RETURNING *;

-- name: MarkOwnerNotificationDelivered :one
UPDATE owner_notification
SET state = 'delivered', provider_state = @provider_state,
    delivered_at = now(), lease_token = NULL, lease_expires_at = NULL,
    next_attempt_at = NULL, updated_at = now()
WHERE id = @id AND state = 'provider_accepted' AND lease_token = @lease_token
RETURNING *;

-- name: AcknowledgeOwnerNotification :one
UPDATE owner_notification
SET state = 'acknowledged', acknowledged_at = now(), updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id
  AND state IN ('provider_accepted', 'delivered')
RETURNING *;

-- name: ListOwnerNotificationAttempts :many
SELECT * FROM owner_notification_attempt
WHERE notification_id = @notification_id
ORDER BY created_at ASC, id ASC;
