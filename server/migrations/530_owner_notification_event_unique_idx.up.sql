CREATE UNIQUE INDEX CONCURRENTLY owner_notification_event_unique_idx ON owner_notification (workspace_id, delivery_mode, event_key);
