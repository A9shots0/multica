CREATE UNIQUE INDEX CONCURRENTLY company_signal_event_key ON company_signal_event (workspace_id, source_key, event_id);
