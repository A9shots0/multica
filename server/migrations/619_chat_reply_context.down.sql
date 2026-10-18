ALTER TABLE chat_message
    DROP COLUMN IF EXISTS reply_context,
    DROP COLUMN IF EXISTS reply_to_message_id;
