ALTER TABLE chat_message ADD COLUMN work_follow_up_task_id uuid;
-- Keep history available without flooding existing chats with new reports.
ALTER TABLE chat_message ADD COLUMN work_follow_up_eligible boolean NOT NULL DEFAULT false;
ALTER TABLE chat_message ALTER COLUMN work_follow_up_eligible SET DEFAULT true;
