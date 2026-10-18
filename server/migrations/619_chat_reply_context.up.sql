-- Exact conversation reply identity. Relationships are application-enforced:
-- repository policy forbids database foreign keys, and the immutable snapshot
-- must survive if retention later removes the referenced event.
ALTER TABLE chat_message
    ADD COLUMN IF NOT EXISTS reply_to_message_id UUID,
    ADD COLUMN IF NOT EXISTS reply_context JSONB;
