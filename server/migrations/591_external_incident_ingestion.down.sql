DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM external_incident_source_binding)
       OR EXISTS (SELECT 1 FROM external_incident_occurrence)
       OR EXISTS (SELECT 1 FROM external_incident_episode)
       OR EXISTS (SELECT 1 FROM external_incident_quarantine)
       OR EXISTS (SELECT 1 FROM issue WHERE origin_type = 'external_incident') THEN
        RAISE EXCEPTION 'cannot roll back external incident ingestion while captured bindings, receipts, episodes, quarantine records, or issues still exist';
    END IF;
END $$;

DROP TABLE IF EXISTS external_incident_quarantine;
DROP TABLE IF EXISTS external_incident_episode;
DROP TABLE IF EXISTS external_incident_occurrence;
DROP TABLE IF EXISTS external_incident_source_binding;
