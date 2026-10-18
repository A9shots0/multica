ALTER TABLE owner_notification_binding
    DROP CONSTRAINT IF EXISTS owner_notification_binding_workspace_id_fkey,
    DROP CONSTRAINT IF EXISTS owner_notification_binding_recipient_id_fkey;
