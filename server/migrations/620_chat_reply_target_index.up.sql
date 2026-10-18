CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_message_reply_to_message
    ON chat_message (reply_to_message_id)
    WHERE reply_to_message_id IS NOT NULL;
