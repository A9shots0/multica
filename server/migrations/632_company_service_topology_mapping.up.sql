ALTER TABLE company_service
    ADD COLUMN system_node_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN journey_ids TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE company_service
    ADD CONSTRAINT company_service_system_node_ids_nonempty
        CHECK (array_position(system_node_ids, '') IS NULL),
    ADD CONSTRAINT company_service_journey_ids_nonempty
        CHECK (array_position(journey_ids, '') IS NULL);
