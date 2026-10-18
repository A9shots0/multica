CREATE UNIQUE INDEX CONCURRENTLY issue_external_incident_origin_key ON issue (workspace_id, origin_type, origin_id) WHERE origin_type = 'external_incident';
