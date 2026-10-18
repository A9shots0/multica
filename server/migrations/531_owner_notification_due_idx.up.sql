CREATE INDEX CONCURRENTLY owner_notification_due_idx ON owner_notification (delivery_mode, next_attempt_at, admitted_at) WHERE state IN ('admitted', 'retryable', 'ambiguous');
