ALTER TABLE company_service
    DROP CONSTRAINT IF EXISTS company_service_journey_ids_nonempty,
    DROP CONSTRAINT IF EXISTS company_service_system_node_ids_nonempty,
    DROP COLUMN IF EXISTS journey_ids,
    DROP COLUMN IF EXISTS system_node_ids;
