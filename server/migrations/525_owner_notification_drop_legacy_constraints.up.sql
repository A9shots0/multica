ALTER TABLE owner_notification
    DROP CONSTRAINT IF EXISTS owner_notification_workspace_id_fkey,
    DROP CONSTRAINT IF EXISTS owner_notification_issue_id_fkey,
    DROP CONSTRAINT IF EXISTS owner_notification_recipient_id_fkey,
    DROP CONSTRAINT IF EXISTS owner_notification_workspace_id_delivery_mode_event_key_key;
