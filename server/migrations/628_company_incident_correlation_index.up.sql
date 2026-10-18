CREATE UNIQUE INDEX CONCURRENTLY company_incident_correlation_key ON company_incident (workspace_id, correlation_key, generation);
