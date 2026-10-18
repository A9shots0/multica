ALTER TABLE owner_notification_attempt
    DROP CONSTRAINT IF EXISTS owner_notification_attempt_notification_id_fkey,
    DROP CONSTRAINT IF EXISTS owner_notification_attempt_notification_id_request_key_key;
