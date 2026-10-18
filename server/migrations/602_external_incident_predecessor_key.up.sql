CREATE UNIQUE INDEX CONCURRENTLY external_incident_predecessor_key
ON external_incident_occurrence (workspace_id, source_key, predecessor_event_key)
WHERE predecessor_event_key IS NOT NULL AND state <> 'rejected';
