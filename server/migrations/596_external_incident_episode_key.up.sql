CREATE UNIQUE INDEX CONCURRENTLY external_incident_episode_key ON external_incident_episode (workspace_id, source_key, finding_key, generation);
