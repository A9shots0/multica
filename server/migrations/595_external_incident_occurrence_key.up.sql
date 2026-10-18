CREATE UNIQUE INDEX CONCURRENTLY external_incident_occurrence_key ON external_incident_occurrence (workspace_id, source_key, event_key);
