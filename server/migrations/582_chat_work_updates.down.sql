DELETE FROM chat_message WHERE message_kind = 'work_update';
ALTER TABLE chat_message DROP COLUMN work_update;
ALTER TABLE chat_message DROP COLUMN work_issue_id;
