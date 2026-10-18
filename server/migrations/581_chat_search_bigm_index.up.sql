-- GIN bigram indexes on chat session title and chat message content, for
-- LIKE '%keyword%' search — same mechanism as project/issue search (032, 039).
-- Skips gracefully when pg_bigm is not installed, matching that precedent.
DO $$
BEGIN
  CREATE INDEX idx_chat_session_title_bigm ON chat_session USING gin (LOWER(title) gin_bigm_ops);
  CREATE INDEX idx_chat_message_content_bigm ON chat_message USING gin (LOWER(content) gin_bigm_ops);
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'skipping bigram indexes on chat (pg_bigm not installed)';
END
$$;
