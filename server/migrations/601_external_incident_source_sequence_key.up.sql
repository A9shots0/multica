CREATE UNIQUE INDEX CONCURRENTLY external_incident_source_sequence_key
ON external_incident_occurrence (workspace_id, source_key, finding_key, source_sequence);
