CREATE UNIQUE INDEX CONCURRENTLY external_incident_quarantine_key ON external_incident_quarantine (workspace_id, source_key, event_key, received_request_hash);
