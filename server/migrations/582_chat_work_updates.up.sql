-- Work notifications are saved conversation entries, independent of a model run.
-- Source identity lives in metadata; task_id remains NULL so a worker event can
-- never masquerade as, reanchor, or complete a direct-chat turn.
ALTER TABLE chat_message ADD COLUMN work_issue_id UUID;
ALTER TABLE chat_message ADD COLUMN work_update JSONB;
