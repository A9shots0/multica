CREATE INDEX CONCURRENTLY company_signal_incident_idx ON company_signal_event (workspace_id, incident_id, observed_at DESC);
