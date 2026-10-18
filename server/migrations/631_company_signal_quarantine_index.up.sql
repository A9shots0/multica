CREATE UNIQUE INDEX CONCURRENTLY company_signal_quarantine_key ON company_signal_quarantine (workspace_id, source_key, event_id, received_request_hash);
