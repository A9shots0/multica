CREATE UNIQUE INDEX CONCURRENTLY external_incident_source_credential_key
    ON external_incident_source_binding (credential_hash);
